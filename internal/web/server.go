package web

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	stdmime "mime"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/agendamento"
	"github.com/notrevetecnologia/notreve-chat-community/internal/agent"
	"github.com/notrevetecnologia/notreve-chat-community/internal/channel"
	"github.com/notrevetecnologia/notreve-chat-community/internal/infra"
	"github.com/notrevetecnologia/notreve-chat-community/internal/llm"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
	"github.com/notrevetecnologia/notreve-chat-community/internal/webhooks"
)

const sessionCookie = "agente_ia_session"
const csrfCookie = "agente_ia_csrf"

type ctxKey int

const ctxUser ctxKey = 1

type sessionEntry struct {
	userID   int
	lastSeen time.Time
}

type Server struct {
	st       *store.Store
	eng      *agent.Engine
	pass     string
	tmpls    map[string]*template.Template
	webroot  string
	sender   channel.Sender
	sessions *sessionStore
	infra    *infra.Infra
	channels *channel.Registry

	scheduler           *agendamento.Scheduler
	agendamentoHandlers agendamento.ActionHandlers

	modulesMu sync.RWMutex
	modules   map[string]bool

	campDown   map[int]time.Time
	campDownMu sync.Mutex

	loginMu  sync.Mutex
	loginTry map[string][]time.Time

	// Impede duas requisições simultâneas de provisionarem a mesma instância
	// WhatsApp antes que o primeiro token seja salvo no banco.
	channelConnectMu sync.Mutex
}

// sessionTTL define quanto tempo uma sessão vale sem atividade.
const sessionTTL = 30 * time.Minute

func NewServer(st *store.Store, eng *agent.Engine, pass, webroot string, sender channel.Sender, inf *infra.Infra, reg *channel.Registry) (*Server, error) {
	if inf == nil {
		inf = &infra.Infra{}
	}
	s := &Server{st: st, eng: eng, pass: pass, webroot: webroot, sender: sender, infra: inf, campDown: map[int]time.Time{}, loginTry: map[string][]time.Time{}, channels: reg}
	if s.channels == nil {
		s.channels = channel.NewRegistry()
	}
	// Registra os adaptadores do núcleo. Canais extras/plugin registram depois.
	s.channels.Register(channel.NewWhatsAppAdapter(sender))
	s.channels.Register(channel.NewTelegramAdapter())
	s.channels.Register(channel.NewWebChatAdapter())
	s.sessions = newSessionStore(inf.Redis, sessionTTL)
	s.agendamentoHandlers = agendamento.ActionHandlers{
		EnviarMensagem:  s.agendamentoEnviarMensagem,
		RodarFerramenta: s.agendamentoRodarFerramenta,
		CallbackAgente:  s.agendamentoCallbackAgente,
	}
	s.scheduler = agendamento.NewScheduler(agendamentoRepo{st: st}, s.agendamentoHandlers).WithLogger(Logger.Printf)
	s.reloadModules(context.Background())
	if err := s.loadTemplates(); err != nil {
		return nil, err
	}
	return s, nil
}

// reloadModules carrega o estado dos módulos (ligado/desligado) em memória.
func (s *Server) reloadModules(ctx context.Context) {
	list, err := s.st.ListModules(ctx)
	if err != nil {
		return
	}
	m := map[string]bool{}
	for _, mod := range list {
		m[mod.Key] = mod.Enabled
	}
	s.modulesMu.Lock()
	s.modules = m
	s.modulesMu.Unlock()
}

func (s *Server) moduleEnabled(key string) bool {
	s.modulesMu.RLock()
	defer s.modulesMu.RUnlock()
	return s.modules[key]
}

// requireModule impede acesso direto a uma funcionalidade desativada mesmo
// quando o usuário conhece a URL e tem a permissão correspondente.
func (s *Server) requireModule(key string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.moduleEnabledForContext(r.Context(), key) {
			http.Error(w, "Módulo desativado", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) moduleEnabledForContext(ctx context.Context, key string) bool {
	enabled, err := s.st.ModuleEnabled(ctx, key)
	return err == nil && enabled
}

// Start inicia os processos de fundo (limpeza de sessões + monitoramento de espera).
func (s *Server) Start(ctx context.Context) {
	go func() {
		s.reconcileAllTenants(ctx)
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.sweepSessions(ctx)
				s.runTenantJobs(ctx)
			}
		}
	}()
	// Campanhas rodam mais frequentemente para respeitar o intervalo configurado.
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.forEachActiveTenant(ctx, s.campanhaJob)
			}
		}
	}()
	// A purga de mídia é pesada; roda a cada 6h e não a cada minuto.
	go func() {
		s.forEachActiveTenant(ctx, s.mediaPurgeJob)
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.forEachActiveTenant(ctx, s.mediaPurgeJob)
			}
		}
	}()
	// Agendamentos: re-agenda pendentes no boot e dispara os vencidos.
	go func() {
		s.forEachActiveTenant(ctx, s.reagendarAgendamentos)
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.forEachActiveTenant(ctx, s.agendamentosJob)
			}
		}
	}()
}

func (s *Server) forEachActiveTenant(ctx context.Context, job func(context.Context)) {
	tenantIDs, err := s.st.ListActiveTenantIDs(ctx)
	if err != nil {
		Logger.Printf("processos de fundo: erro ao listar empresas ativas: %v", err)
		return
	}
	for _, tenantID := range tenantIDs {
		job(store.WithTenantID(ctx, tenantID))
	}
}

func (s *Server) runTenantJobs(ctx context.Context) {
	s.forEachActiveTenant(ctx, func(tenantCtx context.Context) {
		s.patienceJob(tenantCtx)
		s.avatarJob(tenantCtx)
		s.satisfacaoTimeoutJob(tenantCtx)
		s.distributeJob(tenantCtx)
		s.followupJob(tenantCtx)
		s.watchdogJob(tenantCtx)
	})
}

func (s *Server) reconcileAllTenants(ctx context.Context) {
	s.forEachActiveTenant(ctx, func(tenantCtx context.Context) {
		_ = s.reconcileOnline(tenantCtx)
	})
}

// patienceJob: se o cliente espera resposta humana há mais de 3 minutos,
// a IA manda uma mensagem pra manter a paciência (só WhatsApp).
func (s *Server) patienceJob(ctx context.Context) {
	st, err := s.st.GetSettings(ctx)
	if err != nil || !st.IAMonitora || s.sender == nil {
		return
	}
	convs, err := s.st.ListWaitingConversations(ctx, 3, 15)
	if err != nil {
		return
	}
	for _, c := range convs {
		if c.Channel != "whatsapp" || c.CustomerPhone == "" {
			continue
		}
		agentObj, aerr := s.agentForConv(ctx, c)
		if aerr != nil {
			continue
		}
		msg := s.eng.HandlePatience(ctx, agentObj)
		if msg == "" {
			continue
		}
		if err := s.sendConversationText(ctx, c, msg, ""); err != nil {
			continue
		}
		_ = s.st.AddMessage(ctx, store.Message{
			ConversationID: c.ID, Role: "assistant", Source: "system", Content: "🤖 " + msg,
		})
	}
}
func (s *Server) agentForConv(ctx context.Context, conv store.Conversation) (store.Agent, error) {
	if conv.AgentID != nil {
		if a, err := s.st.GetAgent(ctx, *conv.AgentID); err == nil {
			return a, nil
		}
	}
	agents, err := s.st.ListAgents(ctx)
	if err != nil {
		return store.Agent{}, err
	}
	for _, a := range agents {
		if a.Active {
			return a, nil
		}
	}
	return store.Agent{}, errors.New("nenhum agente ativo")
}

// satisfacaoTimeoutJob encerra avaliações que o cliente não respondeu.
func (s *Server) satisfacaoTimeoutJob(ctx context.Context) {
	convs, err := s.st.ListSatisfacaoPendentes(ctx, 180)
	if err != nil {
		return
	}
	for _, c := range convs {
		_ = s.st.SetConversationSatisfacao(ctx, c.ID, "sem_resposta")
	}
}

// avatarJob preenche gradualmente as fotos de perfil das conversas sem foto.
func (s *Server) avatarJob(ctx context.Context) {
	if s.sender == nil {
		return
	}
	convs, err := s.st.ListConversationsSemAvatar(ctx, 5)
	if err != nil {
		Logger.Printf("avatarJob: erro ao listar: %v", err)
		return
	}
	if len(convs) > 0 {
		Logger.Printf("avatarJob: processando %d conversas", len(convs))
	}
	for _, c := range convs {
		if c.CustomerPhone == "" {
			continue
		}
		url, err := s.sender.GetAvatar(ctx, c.CustomerPhone)
		if err != nil {
			Logger.Printf("avatar de %s falhou: %v", c.CustomerPhone, err)
			_ = s.st.SetConversationAvatarTried(ctx, c.ID)
			continue
		}
		if url != "" {
			_ = s.st.SetConversationAvatar(ctx, c.ID, url)
		} else {
			_ = s.st.SetConversationAvatarTried(ctx, c.ID)
		}
	}
}

// sweepSessions remove sessões expiradas e derruba online de quem não está logado.
func (s *Server) sweepSessions(ctx context.Context) {
	s.sessions.sweep(time.Now())
	s.reconcileAllTenants(ctx)
}

// reconcileOnline desliga atendentes cujo usuário não tem sessão ativa.
func (s *Server) reconcileOnline(ctx context.Context) error {
	atendentes, err := s.st.ListAtendentes(ctx)
	if err != nil {
		return err
	}
	active := s.sessions.activeUsers()
	for _, a := range atendentes {
		online := false
		if a.Username != "" {
			if u, err := s.st.GetUsuarioByUsername(ctx, a.Username); err == nil && active[u.ID] {
				online = true
			}
		}
		if a.Online != online {
			_ = s.st.SetAtendenteOnline(ctx, &a.ID, online)
		}
	}
	return nil
}

func newSessionToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// currentUser retorna o usuário autenticado da requisição.
func currentUser(r *http.Request) *store.Usuario {
	if u, ok := r.Context().Value(ctxUser).(*store.Usuario); ok {
		return u
	}
	return nil
}

func (s *Server) loadTemplates() error {
	funcs := template.FuncMap{
		"paramsJSON": func(t store.Tool) string {
			if t.Parameters == nil {
				return ""
			}
			b, _ := json.Marshal(t.Parameters)
			return string(b)
		},
		"joinLinks": func(links []string) string {
			return strings.Join(links, "\n")
		},
		"hasInt": func(list []int, id int) bool {
			for _, v := range list {
				if v == id {
					return true
				}
			}
			return false
		},
		"canPerm": func(u *store.Usuario, perm string) bool {
			return userHasPerm(u, perm)
		},
		"moduleOn": func(key string) bool {
			return s.moduleEnabled(key)
		},
		"permGroups": func() []struct {
			Module string
			Label  string
			Perms  []permDef
		} {
			return permCatalog
		},
		"rolePerms": func(role string) []string {
			return roleDefaultPerms[role]
		},
		"toJSON": func(v interface{}) string {
			b, _ := json.Marshal(v)
			return string(b)
		},
		"percent": func(done, total int) int {
			if total <= 0 {
				return 0
			}
			p := done * 100 / total
			if p > 100 {
				p = 100
			}
			return p
		},
		"joinComma": func(items []string) string {
			if len(items) == 0 {
				return "nenhum"
			}
			return strings.Join(items, ", ")
		},
		"grupoData": func(g store.GrupoCanal) string {
			b, _ := json.Marshal(map[string]interface{}{
				"id": g.ID, "name": g.Name, "channel_ids": g.ChannelIDs,
			})
			return string(b)
		},
		"segmentoNome": func(segmentos []store.Segmento, id int) string {
			for _, sg := range segmentos {
				if sg.ID == id {
					return sg.Name
				}
			}
			return ""
		},
		"statusLabel": func(s string) string {
			switch s {
			case "ia":
				return "IA atendendo"
			case "humano_aguardando":
				return "Aguardando atendimento"
			case "humano_atendendo":
				return "Em atendimento"
			case "closed":
				return "Encerrada"
			}
			return s
		},
		"fmtTime": func(t time.Time) string {
			loc, err := time.LoadLocation("America/Sao_Paulo")
			if err != nil {
				return t.Format("02/01/2006 15:04")
			}
			return t.In(loc).Format("02/01/2006 15:04")
		},
		"diasList": func() []string {
			return []string{"Dom", "Seg", "Ter", "Qua", "Qui", "Sex", "Sáb"}
		},
		"hasDay": func(dias string, idx int) bool {
			for _, p := range strings.Split(dias, ",") {
				if v, err := strconv.Atoi(strings.TrimSpace(p)); err == nil && v == idx {
					return true
				}
			}
			return false
		},
		"initial": func(s string) string {
			s = strings.TrimSpace(s)
			if s == "" {
				return "?"
			}
			r := []rune(s)
			return strings.ToUpper(string(r[0]))
		},
		"protocolo": func(id int) string {
			return fmt.Sprintf("#T-%05d", id)
		},
		"channelIcon": func(ch string) string {
			switch ch {
			case "whatsapp":
				return "📱 WhatsApp"
			case "webchat":
				return "🌐 Web Chat"
			case "instagram":
				return "📸 Instagram"
			case "telegram":
				return "✈️ Telegram"
			}
			return ch
		},
		"originLabel": func(value string) string {
			u, err := url.Parse(strings.TrimSpace(value))
			if err == nil && u.Hostname() != "" {
				return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
			}
			return strings.TrimSpace(value)
		},
		"diasLabel": func(dias string) string {
			names := []string{"Dom", "Seg", "Ter", "Qua", "Qui", "Sex", "Sáb"}
			var out []string
			for _, p := range strings.Split(dias, ",") {
				if v, err := strconv.Atoi(strings.TrimSpace(p)); err == nil && v >= 0 && v <= 6 {
					out = append(out, names[v])
				}
			}
			if len(out) == 0 {
				return "todos os dias"
			}
			return strings.Join(out, ", ")
		},
		"setorData": func(s store.Setor) string {
			days := []int{}
			for _, p := range strings.Split(s.DiasSemana, ",") {
				if v, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
					days = append(days, v)
				}
			}
			b, _ := json.Marshal(map[string]interface{}{
				"id": s.ID, "name": s.Name, "desc": s.Description,
				"abre": s.HorarioAbertura, "fecha": s.HorarioFechamento,
				"dias": days, "active": s.Active, "cor": s.Cor,
			})
			return string(b)
		},
		"atendenteSetores": func(ids []int, setores []store.Setor) string {
			byID := map[int]string{}
			for _, st := range setores {
				byID[st.ID] = st.Name
			}
			var out []string
			for _, id := range ids {
				if n, ok := byID[id]; ok {
					out = append(out, n)
				}
			}
			if len(out) == 0 {
				return "nenhum setor"
			}
			return strings.Join(out, ", ")
		},
		"atendenteData": func(a store.Atendente) string {
			b, _ := json.Marshal(map[string]interface{}{
				"id": a.ID, "name": a.Name, "online": a.Online,
				"active": a.Active, "setores": a.SetorIDs, "username": a.Username,
				"role": a.Role, "perms": strings.Split(a.Permissions, ","),
				"abre": a.HorarioAbertura, "fecha": a.HorarioFechamento, "dias": a.DiasSemana,
			})
			return string(b)
		},
		"canalData": func(c store.Channel) string {
			wc := map[string]interface{}{}
			if c.WidgetConfig != "" {
				_ = json.Unmarshal([]byte(c.WidgetConfig), &wc)
			}
			var widget interface{}
			if c.Type == "webchat" {
				widget = wc
			}
			b, _ := json.Marshal(map[string]interface{}{
				"id": c.ID, "name": c.Name, "base": c.BaseURL,
				// Tokens de instância nunca devem ser enviados ao JavaScript do painel.
				// O backend preserva o valor atual quando o campo fica vazio.
				"token": "", "active": c.Active, "type": c.Type,
				"respond_grupos": c.RespondGrupos, "responder_chamadas": c.ResponderChamadas,
				"can_campaign": c.CanCampaign,
				"widget":       widget, "public_id": c.PublicID,
			})
			return string(b)
		},
		"integracaoData": func(i store.Integration) string {
			headers := map[string]string{}
			if err := json.Unmarshal([]byte(i.Headers), &headers); err == nil {
				for k := range headers {
					if sensitiveHeaderName(k) {
						headers[k] = ""
					}
				}
			}
			headersJSON, _ := json.Marshal(headers)
			b, _ := json.Marshal(map[string]interface{}{
				"id": i.ID, "name": i.Name, "desc": i.Description,
				"method": i.Method, "url": i.URL, "headers": string(headersJSON), "parameters": i.Parameters,
			})
			return string(b)
		},
		"respostaData": func(rr store.RespostaRapida) string {
			b, _ := json.Marshal(map[string]interface{}{
				"id": rr.ID, "atalho": rr.Atalho, "texto": rr.Texto,
			})
			return string(b)
		},
		"contatoData": func(c store.Contato) string {
			b, _ := json.Marshal(map[string]interface{}{
				"id": c.ID, "name": c.Name, "phone": c.Phone, "email": c.Email, "notes": c.Notes, "tem_whatsapp": c.TemWhatsapp,
			})
			return string(b)
		},
		"mediaDataURI": func(m store.Message) string {
			if m.MediaB64 == "" {
				return ""
			}
			mime := m.MediaMime
			if mime == "" {
				mime = "application/octet-stream"
			}
			return "data:" + mime + ";base64," + m.MediaB64
		},
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
	}
	login := filepath.Join(s.webroot, "templates", "login.html")
	t, err := template.New("login.html").Funcs(funcs).ParseFiles(login)
	if err != nil {
		return err
	}
	s.tmpls = map[string]*template.Template{"login": t}
	return nil
}

func (s *Server) brandingForRequest(r *http.Request) reactBranding {
	return s.platformBranding(r.Context())
}

func (s *Server) renderLogin(w http.ResponseWriter, r *http.Request, errMsg string) {
	t, terr := template.ParseFiles(filepath.Join(s.webroot, "templates", "login.html"))
	if terr != nil {
		http.Error(w, "erro template", http.StatusInternalServerError)
		return
	}
	t.ExecuteTemplate(w, "login.html", map[string]interface{}{
		"Error":    errMsg,
		"Branding": s.brandingForRequest(r),
	})
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	publicAPI := NewPublicAPI(s.st, s.sender)

	mux.Handle("POST /api/v1/conversations", http.HandlerFunc(publicAPI.HandlePublicConversationOpen))
	mux.Handle("GET /api/v1/conversations/{id}", http.HandlerFunc(publicAPI.HandlePublicConversation))
	mux.Handle("POST /api/v1/conversations/{id}/messages", http.HandlerFunc(publicAPI.HandlePublicMessage))
	mux.Handle("POST /api/v1/conversations/{id}/close", http.HandlerFunc(publicAPI.HandlePublicClose))
	mux.Handle("POST /api/public/v1/conversations", http.HandlerFunc(publicAPI.HandlePublicConversationOpen))
	mux.Handle("GET /api/public/v1/conversations/{id}", http.HandlerFunc(publicAPI.HandlePublicConversation))
	mux.Handle("POST /api/public/v1/conversations/{id}/messages", http.HandlerFunc(publicAPI.HandlePublicMessage))
	mux.Handle("POST /api/public/v1/conversations/{id}/close", http.HandlerFunc(publicAPI.HandlePublicClose))

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(s.webroot, "static")))))
	mux.Handle("GET /react/assets/", http.StripPrefix("/react/", http.FileServer(http.Dir(filepath.Join(s.webroot, "react", "dist")))))
	// A entrada de login é pública; o restante do painel continua protegido pelo middleware.
	mux.Handle("GET /react/auth", http.HandlerFunc(s.handleReactApp))
	mux.Handle("GET /react/auth/", http.HandlerFunc(s.handleReactApp))
	mux.Handle("GET /auth", http.HandlerFunc(s.handleReactApp))
	mux.Handle("GET /auth/", http.HandlerFunc(s.handleReactApp))
	mux.Handle("GET /react", s.auth(http.HandlerFunc(s.handleReactApp)))
	mux.Handle("GET /react/", s.auth(http.HandlerFunc(s.handleReactApp)))
	mux.Handle("/", s.auth(s.requireModule("conversas", http.HandlerFunc(s.handleReactApp))))

	mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/auth?view=login", http.StatusFound)
	})
	mux.Handle("GET /api/public/branding", http.HandlerFunc(s.handleReactPublicBranding))
	mux.Handle("GET /logout", s.auth(http.HandlerFunc(s.handleLogout)))

	mux.Handle("GET /api/react/session", s.authJSON(http.HandlerFunc(s.handleReactSession)))
	mux.Handle("GET /api/react/bootstrap", s.authJSON(http.HandlerFunc(s.handleReactBootstrap)))
	mux.Handle("GET /api/react/overview", s.authJSON(s.requireReactPerm(permConversasVer, s.requireReactModule("conversas", http.HandlerFunc(s.handleReactOverview)))))
	mux.Handle("GET /api/react/dashboard", s.authJSON(s.requireReactPerm(permConversasVer, s.requireReactModule("conversas", http.HandlerFunc(s.handleReactDashboard)))))
	mux.Handle("GET /api/react/branding", s.authJSON(http.HandlerFunc(s.handleReactBrandingGet)))
	mux.Handle("PUT /api/react/branding", s.authJSON(http.HandlerFunc(s.handleReactBranding)))
	mux.Handle("GET /api/react/platform/branding", s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleReactPlatformBrandingGet))))
	mux.Handle("PUT /api/react/platform/branding", s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleReactPlatformBranding))))
	mux.Handle("GET /api/react/channels", s.authJSON(s.requireReactPerm(permCanaisGerenciar, s.requireReactModule("canais", http.HandlerFunc(s.handleReactChannels)))))
	mux.Handle("POST /api/react/channels", s.authJSON(s.requireReactPerm(permCanaisGerenciar, s.requireReactModule("canais", http.HandlerFunc(s.handleReactChannelCreate)))))
	mux.Handle("PATCH /api/react/channels/{id}", s.authJSON(s.requireReactPerm(permCanaisGerenciar, s.requireReactModule("canais", http.HandlerFunc(s.handleReactChannelUpdate)))))
	mux.Handle("DELETE /api/react/channels/{id}", s.authJSON(s.requireReactPerm(permCanaisGerenciar, s.requireReactModule("canais", http.HandlerFunc(s.handleReactChannelDelete)))))
	mux.Handle("GET /api/react/contacts", s.authJSON(s.requireReactPerm(permContatosGerenciar, s.requireReactModule("contatos", http.HandlerFunc(s.handleReactContacts)))))
	mux.Handle("POST /api/react/contacts", s.authJSON(s.requireReactPerm(permContatosGerenciar, s.requireReactModule("contatos", http.HandlerFunc(s.handleReactContactCreate)))))
	mux.Handle("POST /api/react/contacts/import", s.authJSON(s.requireReactPerm(permContatosGerenciar, s.requireReactModule("contatos", http.HandlerFunc(s.handleReactContactImport)))))
	mux.Handle("PATCH /api/react/contacts/{id}", s.authJSON(s.requireReactPerm(permContatosGerenciar, s.requireReactModule("contatos", http.HandlerFunc(s.handleReactContactUpdate)))))
	mux.Handle("DELETE /api/react/contacts/{id}", s.authJSON(s.requireReactPerm(permContatosGerenciar, s.requireReactModule("contatos", http.HandlerFunc(s.handleReactContactDelete)))))
	mux.Handle("GET /api/react/agents", s.authJSON(s.requireReactPerm(permAgentesGerenciar, s.requireReactModule("ia", http.HandlerFunc(s.handleReactAgents)))))
	mux.Handle("POST /api/react/agents", s.authJSON(s.requireReactPerm(permAgentesGerenciar, s.requireReactModule("ia", http.HandlerFunc(s.handleReactAgentCreate)))))
	mux.Handle("PATCH /api/react/agents/{id}", s.authJSON(s.requireReactPerm(permAgentesGerenciar, s.requireReactModule("ia", http.HandlerFunc(s.handleReactAgentUpdate)))))
	mux.Handle("POST /api/react/agents/{id}/toggle", s.authJSON(s.requireReactPerm(permAgentesGerenciar, s.requireReactModule("ia", http.HandlerFunc(s.handleReactAgentToggle)))))
	mux.Handle("POST /api/react/agents/{id}/refetch", s.authJSON(s.requireReactPerm(permAgentesGerenciar, s.requireReactModule("ia", http.HandlerFunc(s.handleReactAgentRefetch)))))
	mux.Handle("POST /api/react/agents/{id}/test", s.authJSON(s.requireReactPerm(permAgentesGerenciar, s.requireReactModule("ia", http.HandlerFunc(s.handleReactAgentTest)))))
	mux.Handle("DELETE /api/react/agents/{id}", s.authJSON(s.requireReactPerm(permAgentesGerenciar, s.requireReactModule("ia", http.HandlerFunc(s.handleReactAgentDelete)))))
	mux.Handle("GET /api/react/chatflows", s.authJSON(s.requireReactPerm(permChatflowsGerenciar, s.requireReactModule("chatflows", http.HandlerFunc(s.handleReactChatflows)))))
	mux.Handle("POST /api/react/chatflows", s.authJSON(s.requireReactPerm(permChatflowsGerenciar, s.requireReactModule("chatflows", http.HandlerFunc(s.handleReactChatflowCreate)))))
	mux.Handle("GET /api/react/chatflows/{id}", s.authJSON(s.requireReactPerm(permChatflowsGerenciar, s.requireReactModule("chatflows", http.HandlerFunc(s.handleReactChatflowGet)))))
	mux.Handle("PATCH /api/react/chatflows/{id}", s.authJSON(s.requireReactPerm(permChatflowsGerenciar, s.requireReactModule("chatflows", http.HandlerFunc(s.handleReactChatflowUpdate)))))
	mux.Handle("POST /api/react/chatflows/{id}/activate", s.authJSON(s.requireReactPerm(permChatflowsGerenciar, s.requireReactModule("chatflows", http.HandlerFunc(s.handleReactChatflowActivate)))))
	mux.Handle("POST /api/react/chatflows/{id}/channels", s.authJSON(s.requireReactPerm(permChatflowsGerenciar, s.requireReactModule("chatflows", http.HandlerFunc(s.handleReactChatflowAttach)))))
	mux.Handle("DELETE /api/react/chatflows/{id}", s.authJSON(s.requireReactPerm(permChatflowsGerenciar, s.requireReactModule("chatflows", http.HandlerFunc(s.handleReactChatflowDelete)))))
	mux.Handle("GET /api/react/providers", s.authJSON(s.requireReactPerm(permIAsGerenciar, s.requireReactModule("ia", http.HandlerFunc(s.handleReactProviders)))))
	mux.Handle("GET /api/react/providers/{id}", s.authJSON(s.requireReactPerm(permIAsGerenciar, s.requireReactModule("ia", http.HandlerFunc(s.handleReactProvider)))))
	mux.Handle("POST /api/react/providers", s.authJSON(s.requireReactPerm(permIAsGerenciar, s.requireReactModule("ia", http.HandlerFunc(s.handleReactProviderCreate)))))
	mux.Handle("PATCH /api/react/providers/{id}", s.authJSON(s.requireReactPerm(permIAsGerenciar, s.requireReactModule("ia", http.HandlerFunc(s.handleReactProviderUpdate)))))
	mux.Handle("DELETE /api/react/providers/{id}", s.authJSON(s.requireReactPerm(permIAsGerenciar, s.requireReactModule("ia", http.HandlerFunc(s.handleReactProviderDelete)))))
	mux.Handle("POST /api/react/providers/{id}/test", s.authJSON(s.requireReactPerm(permIAsGerenciar, s.requireReactModule("ia", http.HandlerFunc(s.handleReactProviderTest)))))
	mux.Handle("POST /api/react/providers/models", s.authJSON(s.requireReactPerm(permIAsGerenciar, s.requireReactModule("ia", http.HandlerFunc(s.handleReactProviderModels)))))
	mux.Handle("POST /api/react/providers/test", s.authJSON(s.requireReactPerm(permIAsGerenciar, s.requireReactModule("ia", http.HandlerFunc(s.handleReactProviderProbe)))))
	mux.Handle("GET /api/react/integrations", s.authJSON(s.requireReactPerm(permIntegracoesGerenciar, s.requireReactModule("tools", http.HandlerFunc(s.handleReactIntegrations)))))
	mux.Handle("GET /api/react/integrations/{id}", s.authJSON(s.requireReactPerm(permIntegracoesGerenciar, s.requireReactModule("tools", http.HandlerFunc(s.handleReactIntegration)))))
	mux.Handle("POST /api/react/integrations", s.authJSON(s.requireReactPerm(permIntegracoesGerenciar, s.requireReactModule("tools", http.HandlerFunc(s.handleReactIntegrationCreate)))))
	mux.Handle("PATCH /api/react/integrations/{id}", s.authJSON(s.requireReactPerm(permIntegracoesGerenciar, s.requireReactModule("tools", http.HandlerFunc(s.handleReactIntegrationUpdate)))))
	mux.Handle("DELETE /api/react/integrations/{id}", s.authJSON(s.requireReactPerm(permIntegracoesGerenciar, s.requireReactModule("tools", http.HandlerFunc(s.handleReactIntegrationDelete)))))
	mux.Handle("GET /api/react/api-keys", s.authJSON(s.requireReactPerm(permIntegracoesGerenciar, s.requireReactModule("api", http.HandlerFunc(s.handleReactAPIKeys)))))
	mux.Handle("POST /api/react/api-keys", s.authJSON(s.requireReactPerm(permIntegracoesGerenciar, s.requireReactModule("api", http.HandlerFunc(s.handleReactAPIKeyCreate)))))
	mux.Handle("POST /api/react/api-keys/{id}/toggle", s.authJSON(s.requireReactPerm(permIntegracoesGerenciar, s.requireReactModule("api", http.HandlerFunc(s.handleReactAPIKeyToggle)))))
	mux.Handle("DELETE /api/react/api-keys/{id}", s.authJSON(s.requireReactPerm(permIntegracoesGerenciar, s.requireReactModule("api", http.HandlerFunc(s.handleReactAPIKeyDelete)))))
	mux.Handle("GET /api/react/api-docs", s.authJSON(s.requireReactPerm(permIntegracoesGerenciar, s.requireReactModule("api", http.HandlerFunc(s.handleReactAPIDocs)))))
	mux.Handle("GET /api/react/skills", s.authJSON(s.requireReactPerm(permAgentesGerenciar, s.requireReactModule("ia", http.HandlerFunc(s.handleReactSkills)))))
	mux.Handle("POST /api/react/skills", s.authJSON(s.requireReactPerm(permAgentesGerenciar, s.requireReactModule("ia", http.HandlerFunc(s.handleReactSkillCreate)))))
	mux.Handle("POST /api/react/skills/seed", s.authJSON(s.requireReactPerm(permAgentesGerenciar, s.requireReactModule("ia", http.HandlerFunc(s.handleReactSkillSeed)))))
	mux.Handle("PATCH /api/react/skills/{id}", s.authJSON(s.requireReactPerm(permAgentesGerenciar, s.requireReactModule("ia", http.HandlerFunc(s.handleReactSkillUpdate)))))
	mux.Handle("DELETE /api/react/skills/{id}", s.authJSON(s.requireReactPerm(permAgentesGerenciar, s.requireReactModule("ia", http.HandlerFunc(s.handleReactSkillDelete)))))
	mux.Handle("GET /api/react/mcp", s.authJSON(s.requireReactPerm(permIntegracoesGerenciar, s.requireReactModule("tools", http.HandlerFunc(s.handleReactMCPServers)))))
	mux.Handle("POST /api/react/mcp", s.authJSON(s.requireReactPerm(permIntegracoesGerenciar, s.requireReactModule("tools", http.HandlerFunc(s.handleReactMCPCreate)))))
	mux.Handle("PATCH /api/react/mcp/{id}", s.authJSON(s.requireReactPerm(permIntegracoesGerenciar, s.requireReactModule("tools", http.HandlerFunc(s.handleReactMCPUpdate)))))
	mux.Handle("DELETE /api/react/mcp/{id}", s.authJSON(s.requireReactPerm(permIntegracoesGerenciar, s.requireReactModule("tools", http.HandlerFunc(s.handleReactMCPDelete)))))
	mux.Handle("POST /api/react/mcp/{id}/test", s.authJSON(s.requireReactPerm(permIntegracoesGerenciar, s.requireReactModule("tools", http.HandlerFunc(s.handleReactMCPTest)))))
	mux.Handle("POST /api/react/mcp/{id}/sync", s.authJSON(s.requireReactPerm(permIntegracoesGerenciar, s.requireReactModule("tools", http.HandlerFunc(s.handleReactMCPSync)))))
	mux.Handle("GET /api/react/webhooks", s.authJSON(s.requireReactPerm(permIntegracoesGerenciar, s.requireReactModule("api", http.HandlerFunc(s.handleReactWebhookSubscriptions)))))
	mux.Handle("GET /api/react/webhooks/deliveries", s.authJSON(s.requireReactPerm(permIntegracoesGerenciar, s.requireReactModule("api", http.HandlerFunc(s.handleReactWebhookDeliveries)))))
	mux.Handle("POST /api/react/webhooks", s.authJSON(s.requireReactPerm(permIntegracoesGerenciar, s.requireReactModule("api", http.HandlerFunc(s.handleReactWebhookSubscriptionCreate)))))
	mux.Handle("PATCH /api/react/webhooks/{id}", s.authJSON(s.requireReactPerm(permIntegracoesGerenciar, s.requireReactModule("api", http.HandlerFunc(s.handleReactWebhookSubscriptionUpdate)))))
	mux.Handle("POST /api/react/webhooks/{id}/toggle", s.authJSON(s.requireReactPerm(permIntegracoesGerenciar, s.requireReactModule("api", http.HandlerFunc(s.handleReactWebhookSubscriptionToggle)))))
	mux.Handle("DELETE /api/react/webhooks/{id}", s.authJSON(s.requireReactPerm(permIntegracoesGerenciar, s.requireReactModule("api", http.HandlerFunc(s.handleReactWebhookSubscriptionDelete)))))
	mux.Handle("GET /api/react/learning", s.authJSON(s.requireReactPerm(permAprendizadoVer, s.requireReactModule("ia", http.HandlerFunc(s.handleReactLearning)))))
	mux.Handle("POST /api/react/learning/{id}/status", s.authJSON(s.requireReactPerm(permAprendizadoVer, s.requireReactModule("ia", http.HandlerFunc(s.handleReactLearningMark)))))
	mux.Handle("POST /api/react/learning/{id}/teach", s.authJSON(s.requireReactPerm(permAprendizadoVer,
		s.requireReactModule("ia", http.HandlerFunc(s.handleReactLearningTeach)))))
	mux.Handle("GET /api/react/agent-failures", s.authJSON(s.requireReactPerm(permAprendizadoVer,
		s.requireReactModule("ia", http.HandlerFunc(s.handleReactAgentFailures)))))
	mux.Handle("POST /api/react/agent-failures/{id}/resolve", s.authJSON(s.requireReactPerm(permAprendizadoVer,
		s.requireReactModule("ia", http.HandlerFunc(s.handleReactAgentFailureResolve)))))
	mux.Handle("GET /api/react/knowledge", s.authJSON(s.requireReactPerm(permAgentesGerenciar, s.requireReactModule("ia", http.HandlerFunc(s.handleReactKnowledge)))))
	mux.Handle("POST /api/react/knowledge", s.authJSON(s.requireReactPerm(permAgentesGerenciar, s.requireReactModule("ia", http.HandlerFunc(s.handleReactKnowledgeCreate)))))
	mux.Handle("DELETE /api/react/knowledge/{id}", s.authJSON(s.requireReactPerm(permAgentesGerenciar, s.requireReactModule("ia", http.HandlerFunc(s.handleReactKnowledgeDelete)))))
	mux.Handle("GET /api/react/quick-replies", s.authJSON(s.requireReactPerm(permRespostasGerenciar, s.requireReactModule("mensagens", http.HandlerFunc(s.handleReactQuickReplies)))))
	mux.Handle("POST /api/react/quick-replies", s.authJSON(s.requireReactPerm(permRespostasGerenciar, s.requireReactModule("mensagens", http.HandlerFunc(s.handleReactQuickReplies)))))
	mux.Handle("PATCH /api/react/quick-replies/{id}", s.authJSON(s.requireReactPerm(permRespostasGerenciar, s.requireReactModule("mensagens", http.HandlerFunc(s.handleReactQuickReplyUpdate)))))
	mux.Handle("DELETE /api/react/quick-replies/{id}", s.authJSON(s.requireReactPerm(permRespostasGerenciar, s.requireReactModule("mensagens", http.HandlerFunc(s.handleReactQuickReplyDelete)))))
	mux.Handle("GET /api/react/message-templates", s.authJSON(s.requireReactPerm(permMensagensGerenciar, s.requireReactModule("mensagens", http.HandlerFunc(s.handleReactMessageTemplates)))))
	mux.Handle("PATCH /api/react/message-templates/{id}", s.authJSON(s.requireReactPerm(permMensagensGerenciar, s.requireReactModule("mensagens", http.HandlerFunc(s.handleReactMessageTemplateUpdate)))))
	mux.Handle("GET /api/react/platform", s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleReactPlatform))))
	mux.Handle("PATCH /api/react/platform/limits", s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleReactPlatformLimitsUpdate))))
	mux.Handle("GET /api/react/platform/plans", s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleReactPlans))))
	mux.Handle("POST /api/react/platform/plans", s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleReactPlanCreate))))
	mux.Handle("PATCH /api/react/platform/plans/{id}", s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleReactPlanUpdate))))
	mux.Handle("DELETE /api/react/platform/plans/{id}", s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleReactPlanDelete))))
	mux.Handle("GET /api/react/platform/tenants/{id}/plan", s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleReactTenantPlan))))
	mux.Handle("PATCH /api/react/platform/tenants/{id}/plan", s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleReactTenantPlanUpdate))))
	mux.Handle("GET /api/react/platform/tenants", s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleReactPlatformTenants))))
	mux.Handle("GET /api/react/platform/tenants/{id}/usage", s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleReactPlatformTenantUsage))))
	mux.Handle("POST /api/react/platform/tenants", s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleReactPlatformTenantCreate))))
	mux.Handle("PATCH /api/react/platform/tenants/{id}", s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleReactPlatformTenantUpdate))))
	mux.Handle("GET /api/react/platform/tenants/{id}/admins", s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleReactPlatformTenantAdmins))))
	mux.Handle("POST /api/react/platform/tenants/{id}/admins", s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleReactPlatformTenantAdminCreate))))
	mux.Handle("POST /api/react/platform/tenants/{id}/admins/reset-password", s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleReactPlatformTenantAdminPasswordReset))))
	mux.Handle("GET /api/react/platform/whatsapp-servers", s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleReactPlatformWhatsAppServers))))
	mux.Handle("POST /api/react/platform/whatsapp-servers", s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleReactPlatformWhatsAppServerCreate))))
	mux.Handle("PATCH /api/react/platform/whatsapp-servers/{id}", s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleReactPlatformWhatsAppServerUpdate))))
	mux.Handle("POST /api/react/platform/tenants/{id}/switch", s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleReactPlatformContextSwitch))))
	mux.Handle("GET /api/react/platform/context-switches", s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleReactPlatformContextSwitchAudit))))
	mux.Handle("GET /api/react/team", s.authJSON(s.requireReactModule("equipe", http.HandlerFunc(s.handleReactTeam))))
	mux.Handle("GET /api/react/sectors", s.authJSON(s.requireReactModule("equipe", http.HandlerFunc(s.handleReactSectors))))
	mux.Handle("POST /api/react/sectors", s.authJSON(s.requireReactModule("equipe", http.HandlerFunc(s.handleReactSectorCreate))))
	mux.Handle("PATCH /api/react/sectors/{id}", s.authJSON(s.requireReactModule("equipe", http.HandlerFunc(s.handleReactSectorUpdate))))
	mux.Handle("POST /api/react/sectors/{id}/toggle", s.authJSON(s.requireReactModule("equipe", http.HandlerFunc(s.handleReactSectorToggle))))
	mux.Handle("DELETE /api/react/sectors/{id}", s.authJSON(s.requireReactModule("equipe", http.HandlerFunc(s.handleReactSectorDelete))))
	mux.Handle("GET /api/react/users", s.authJSON(s.requireReactPerm(permEquipeGerenciar, s.requireReactModule("equipe", http.HandlerFunc(s.handleReactUsers)))))
	mux.Handle("GET /api/react/users/{id}", s.authJSON(s.requireReactPerm(permEquipeGerenciar, s.requireReactModule("equipe", http.HandlerFunc(s.handleReactUser)))))
	mux.Handle("POST /api/react/users", s.authJSON(s.requireReactPerm(permEquipeGerenciar, s.requireReactModule("equipe", http.HandlerFunc(s.handleReactUserCreate)))))
	mux.Handle("PATCH /api/react/users/{id}", s.authJSON(s.requireReactPerm(permEquipeGerenciar, s.requireReactModule("equipe", http.HandlerFunc(s.handleReactUserUpdate)))))
	mux.Handle("POST /api/react/users/{id}/toggle", s.authJSON(s.requireReactPerm(permEquipeGerenciar, s.requireReactModule("equipe", http.HandlerFunc(s.handleReactUserToggle)))))
	mux.Handle("DELETE /api/react/users/{id}", s.authJSON(s.requireReactPerm(permEquipeGerenciar, s.requireReactModule("equipe", http.HandlerFunc(s.handleReactUserDelete)))))
	mux.Handle("GET /api/react/settings", s.authJSON(s.requireReactPerm(permConfigGerenciar, s.requireReactModule("config", http.HandlerFunc(s.handleReactSettings)))))
	mux.Handle("PATCH /api/react/settings", s.authJSON(s.requireReactPerm(permConfigGerenciar, s.requireReactModule("config", http.HandlerFunc(s.handleReactSettingsUpdate)))))
	mux.Handle("GET /api/react/logs", s.authJSON(s.requireReactPerm(permLogsVer, s.requireReactModule("config", http.HandlerFunc(s.handleReactLogs)))))
	mux.Handle("GET /api/react/logs/stats", s.authJSON(s.requireReactPerm(permLogsVer, s.requireReactModule("config", http.HandlerFunc(s.handleReactLogsStats)))))
	mux.Handle("GET /api/react/lgpd/summary", s.authJSON(s.requireReactPerm(permConfigGerenciar, s.requireReactModule("lgpd", http.HandlerFunc(s.handleReactLGPDSummary)))))
	mux.Handle("GET /api/react/lgpd/consents", s.authJSON(s.requireReactPerm(permConfigGerenciar, s.requireReactModule("lgpd", http.HandlerFunc(s.handleReactLGPDConsents)))))
	mux.Handle("POST /api/react/lgpd/consents", s.authJSON(s.requireReactPerm(permConfigGerenciar, s.requireReactModule("lgpd", http.HandlerFunc(s.handleReactLGPDConsentCreate)))))
	mux.Handle("GET /api/react/lgpd/politica", s.authJSON(s.requireReactPerm(permConfigGerenciar, s.requireReactModule("lgpd", http.HandlerFunc(s.handleReactLGPDPoliticaCurrent)))))
	mux.Handle("GET /api/react/lgpd/politicas", s.authJSON(s.requireReactPerm(permConfigGerenciar, s.requireReactModule("lgpd", http.HandlerFunc(s.handleReactLGPDPoliticaList)))))
	mux.Handle("POST /api/react/lgpd/politica", s.authJSON(s.requireReactPerm(permConfigGerenciar, s.requireReactModule("lgpd", http.HandlerFunc(s.handleReactLGPDPoliticaCreate)))))
	mux.Handle("GET /api/react/lgpd/audit", s.authJSON(s.requireReactPerm(permConfigGerenciar, s.requireReactModule("lgpd", http.HandlerFunc(s.handleReactLGPDAudit)))))
	mux.Handle("GET /api/react/lgpd/documento/{id}", s.authJSON(s.requireReactPerm(permConfigGerenciar, s.requireReactModule("lgpd", http.HandlerFunc(s.handleReactLGPDContactDocumentGet)))))
	mux.Handle("POST /api/react/lgpd/documento/{id}", s.authJSON(s.requireReactPerm(permConfigGerenciar, s.requireReactModule("lgpd", http.HandlerFunc(s.handleReactLGPDContactDocumentSet)))))
	mux.Handle("GET /api/react/campaigns", s.authJSON(s.requireReactPerm(permCampanhasVer, s.requireReactModule("campanhas", http.HandlerFunc(s.handleReactCampaigns)))))
	mux.Handle("POST /api/react/campaigns", s.authJSON(s.requireReactPerm(permCampanhasGerenciar, s.requireReactModule("campanhas", http.HandlerFunc(s.handleReactCampaignCreate)))))
	mux.Handle("POST /api/react/campaigns/{id}/status", s.authJSON(s.requireReactPerm(permCampanhasGerenciar, s.requireReactModule("campanhas", http.HandlerFunc(s.handleReactCampaignStatus)))))
	mux.Handle("DELETE /api/react/campaigns/{id}", s.authJSON(s.requireReactPerm(permCampanhasGerenciar, s.requireReactModule("campanhas", http.HandlerFunc(s.handleReactCampaignDelete)))))
	mux.Handle("GET /api/react/campaigns/{id}/envios", s.authJSON(s.requireReactPerm(permCampanhasVer, s.requireReactModule("campanhas", http.HandlerFunc(s.handleReactCampaignEnvios)))))
	mux.Handle("GET /api/react/campaign-groups", s.authJSON(s.requireReactPerm(permCampanhasVer, s.requireReactModule("campanhas", http.HandlerFunc(s.handleReactCampaignGroups)))))
	mux.Handle("POST /api/react/campaign-groups", s.authJSON(s.requireReactPerm(permGruposCanaisGerenciar, s.requireReactModule("campanhas", http.HandlerFunc(s.handleReactCampaignGroupCreate)))))
	mux.Handle("PATCH /api/react/campaign-groups/{id}", s.authJSON(s.requireReactPerm(permGruposCanaisGerenciar, s.requireReactModule("campanhas", http.HandlerFunc(s.handleReactCampaignGroupUpdate)))))
	mux.Handle("DELETE /api/react/campaign-groups/{id}", s.authJSON(s.requireReactPerm(permGruposCanaisGerenciar, s.requireReactModule("campanhas", http.HandlerFunc(s.handleReactCampaignGroupDelete)))))
	mux.Handle("GET /api/react/segments", s.authJSON(s.requireReactPerm(permCampanhasVer, s.requireReactModule("campanhas", http.HandlerFunc(s.handleReactSegments)))))
	mux.Handle("POST /api/react/segments", s.authJSON(s.requireReactPerm(permCampanhasGerenciar, s.requireReactModule("campanhas", http.HandlerFunc(s.handleReactSegmentCreate)))))
	mux.Handle("DELETE /api/react/segments/{id}", s.authJSON(s.requireReactPerm(permCampanhasGerenciar, s.requireReactModule("campanhas", http.HandlerFunc(s.handleReactSegmentDelete)))))
	mux.Handle("GET /api/react/schedules", s.authJSON(s.requireReactPerm(permAgendamentosGerenciar, s.requireReactModule("agendamentos", http.HandlerFunc(s.handleReactSchedules)))))
	mux.Handle("POST /api/react/schedules", s.authJSON(s.requireReactPerm(permAgendamentosGerenciar, s.requireReactModule("agendamentos", http.HandlerFunc(s.handleReactScheduleCreate)))))
	mux.Handle("PATCH /api/react/schedules/{id}", s.authJSON(s.requireReactPerm(permAgendamentosGerenciar, s.requireReactModule("agendamentos", http.HandlerFunc(s.handleReactScheduleUpdate)))))
	mux.Handle("DELETE /api/react/schedules/{id}", s.authJSON(s.requireReactPerm(permAgendamentosGerenciar, s.requireReactModule("agendamentos", http.HandlerFunc(s.handleReactScheduleDelete)))))
	mux.Handle("POST /api/react/schedules/{id}/trigger", s.authJSON(s.requireReactPerm(permAgendamentosGerenciar, s.requireReactModule("agendamentos", http.HandlerFunc(s.handleReactScheduleTrigger)))))
	mux.Handle("GET /api/react/reports", s.authJSON(s.requireReactPerm(permRelatoriosVer, s.requireReactModule("relatorios", http.HandlerFunc(s.handleReactReports)))))
	mux.Handle("GET /api/react/reports/export", s.authJSON(s.requireReactPerm(permRelatoriosVer, s.requireReactModule("relatorios", http.HandlerFunc(s.handleReactReportsExport)))))
	mux.Handle("GET /api/react/conversations", s.authJSON(s.requireReactPerm(permConversasVer, s.requireReactModule("conversas", http.HandlerFunc(s.handleReactConversations)))))
	mux.Handle("GET /api/react/conversations/{id}", s.authJSON(s.requireReactPerm(permConversasVer, s.requireReactModule("conversas", http.HandlerFunc(s.handleReactConversation)))))
	mux.Handle("POST /api/react/conversations/{id}/messages", s.authJSON(s.requireReactPerm(permConversasResponder, s.requireReactModule("conversas", http.HandlerFunc(s.handleReactSend)))))
	mux.Handle("POST /api/react/conversations/{id}/actions", s.authJSON(s.requireReactModule("conversas", http.HandlerFunc(s.handleReactAction))))
	mux.Handle("POST /api/react/conversations/{id}/bot-block", s.authJSON(s.requireReactPerm(permConversasResponder, s.requireReactModule("conversas", http.HandlerFunc(s.handleReactBotBlock)))))

	return mux
}

// ---------- Auth ----------

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && !sameOriginRequest(r) {
			http.Error(w, "origem não permitida", http.StatusForbidden)
			return
		}
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		uid, ok := s.sessions.Get(r.Context(), c.Value)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		_ = s.sessions.Touch(r.Context(), c.Value)
		tenantID, tenantStatus, tenantErr := s.st.ResolveTenantForHost(r.Context(), r.Host)
		if tenantErr != nil || (tenantStatus != "active" && tenantStatus != "trial") {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		tenantCtx := store.WithTenantID(r.Context(), tenantID)
		u, err := s.st.GetUsuarioByID(tenantCtx, uid)
		if err != nil || !u.Active {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		if u.Role == "super_admin" && !superAdminHTMLPathAllowed(r.URL.Path) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.Error(w, "o super_admin usa somente o control plane da plataforma", http.StatusForbidden)
			} else {
				http.Redirect(w, r, "/react?view=platform&tab=limits", http.StatusFound)
			}
			return
		}
		setSecurityHeaders(w)
		if _, err := r.Cookie(csrfCookie); err != nil {
			setCSRFCookie(w, r, newSessionToken())
		}
		ctx := store.WithTenantID(context.WithValue(tenantCtx, ctxUser, &u), u.TenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func superAdminHTMLPathAllowed(path string) bool {
	switch path {
	case "/", "/react", "/react/", "/logout", "/limites", "/limites/salvar":
		return true
	default:
		return false
	}
}

func requestScheme(r *http.Request) string {
	if r.TLS != nil || strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]), "https") {
		return "https"
	}
	return "http"
}

// sameOriginRequest bloqueia POSTs vindos de outra origem. Ele protege tanto
// os formulários HTML quanto as chamadas fetch sem exigir que cada template
// replique um token. Clientes não-browser que não enviam Origin/Referer seguem
// funcionando; quando o navegador envia um deles, a origem precisa coincidir.
func sameOriginRequest(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		origin = strings.TrimSpace(r.Header.Get("Referer"))
	}
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || strings.EqualFold(u.Host, r.Host) == false {
		return false
	}
	return strings.EqualFold(u.Scheme, requestScheme(r))
}

func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Referrer-Policy", "same-origin")
}

func setCSRFCookie(w http.ResponseWriter, r *http.Request, token string) {
	https := requestScheme(r) == "https"
	http.SetCookie(w, &http.Cookie{
		Name: csrfCookie, Value: token, Path: "/", HttpOnly: false,
		Secure: https, SameSite: http.SameSiteLaxMode,
	})
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", HttpOnly: true,
		Secure: requestScheme(r) == "https", SameSite: http.SameSiteLaxMode,
		MaxAge: maxAge,
	})
}

// requireRole exige que o usuário tenha nível >= minRole.
func (s *Server) requireRole(minRole string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := currentUser(r)
		if u == nil || roleLevel(u.Role) < roleLevel(minRole) {
			http.Error(w, "Sem permissão para acessar", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP resolve o IP real do cliente, respeitando proxy reverso (Cloudflare/Nginx).
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if i := strings.Index(fwd, ","); i >= 0 {
			fwd = fwd[:i]
		}
		if ip := strings.TrimSpace(fwd); ip != "" {
			return ip
		}
	}
	if cf := r.Header.Get("CF-Connecting-IP"); cf != "" {
		return cf
	}
	ip := r.RemoteAddr
	if i := strings.LastIndex(ip, ":"); i > 0 {
		ip = ip[:i]
	}
	return ip
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.sessions.Delete(r.Context(), c.Value)
	}
	if u := currentUser(r); u != nil {
		_ = s.st.SetAtendenteOnline(r.Context(), u.AtendenteID, false)
	}
	setSessionCookie(w, r, "", -1)
	http.SetCookie(w, &http.Cookie{Name: csrfCookie, Value: "", Path: "/", MaxAge: -1, Secure: requestScheme(r) == "https", SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/login", http.StatusFound)
}

// ---------- Pages ----------

// modoAberto diz se qualquer atendente pode atender qualquer conversa.
func (s *Server) modoAberto(ctx context.Context) bool {
	st, err := s.st.GetSettings(ctx)
	if err != nil {
		return false
	}
	return st.ModoAberto
}

// userSetores retorna os setores do usuário (se atendente vinculado).
func (s *Server) userSetores(ctx context.Context, user *store.Usuario) []int {
	if user == nil || user.AtendenteID == nil {
		return nil
	}
	atendentes, err := s.st.ListAtendentes(ctx)
	if err != nil {
		return nil
	}
	for _, a := range atendentes {
		if a.ID == *user.AtendenteID {
			return a.SetorIDs
		}
	}
	return nil
}

// canSee decide se um atendente pode ver a conversa.
func (s *Server) canSee(user *store.Usuario, conv store.Conversation, setorIDs []int, modoAberto bool) bool {
	if user == nil || user.Role == "admin" || user.Role == "super_admin" {
		return true
	}
	if user.Role == "supervisor" {
		if conv.AtendenteID != nil && *conv.AtendenteID == user.ID {
			return true
		}
		if conv.SetorID == nil {
			return false
		}
		return containsInt(setorIDs, *conv.SetorID)
	}
	if user.Role != "atendente" {
		return false
	}
	if conv.Status == "closed" {
		return false
	}
	if conv.Status == "ia" && !userHasPerm(user, permConversasVerIA) {
		return false
	}
	if conv.AtendenteID != nil {
		return *conv.AtendenteID == user.ID // só o dono da conversa vê
	}
	// não atribuída: modo aberto aceita qualquer setor; senão, setor do atendente ou sem setor
	if modoAberto {
		return true
	}
	if conv.SetorID == nil {
		return true // fallback: conversa sem setor não fica presa
	}
	for _, sid := range setorIDs {
		if sid == *conv.SetorID {
			return true
		}
	}
	return false
}

func containsInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// ---------- Agent CRUD ----------

// ---------- Setores & Atendentes ----------

// distributeJob atribui automaticamente conversas aguardando humano ao
// atendente online disponível (setor + horário + menos carregado).
func (s *Server) distributeJob(ctx context.Context) {
	st, err := s.st.GetSettings(ctx)
	if err != nil || !st.DistribuicaoAuto {
		return
	}
	convs, err := s.st.ListUnassignedWaitingConversations(ctx)
	if err != nil || len(convs) == 0 {
		return
	}
	candidates, err := s.st.ListAvailableAtendentes(ctx)
	if err != nil {
		return
	}
	s.loadAtendenteHabilidades(ctx, candidates)
	loads := initialAtendenteLoads(candidates)
	var fallback []store.AtendenteDisponivel
	fallbackLoaded := false
	now := nowBR()
	for _, c := range convs {
		lastMessage, _ := s.st.LastCustomerMessageForConversation(ctx, c.ID)
		required := store.RequiredSkillsForConversation(c, lastMessage, store.SkillsOfMerge(candidates))
		best := pickBestAtendente(c, candidates, st.ModoAberto, now, required, loads)
		usouFallback := false
		if best == nil {
			if !fallbackLoaded {
				fallback, _ = s.st.ListAtendentesFallback(ctx)
				s.loadAtendenteHabilidades(ctx, fallback)
				fallbackLoaded = true
			}
			if len(fallback) > 0 {
				required = store.RequiredSkillsForConversation(c, lastMessage, store.SkillsOfMerge(fallback))
				best = pickBestAtendente(c, fallback, st.ModoAberto, now, required, loads)
				usouFallback = best != nil
			}
		}
		if best == nil {
			continue
		}
		uid := best.UsuarioID
		if err := s.st.SetConversationAtendente(ctx, c.ID, &uid); err == nil {
			loads[uid] = effectiveAtendenteLoad(best, loads) + 1
			if usouFallback {
				Logger.Printf("distribuicao: conversa #T-%05d atribuida a %s (sem atendente online)%s", c.ID, best.Nome, skillSuffix(best.Habilidades, required))
			} else {
				Logger.Printf("distribuicao: conversa #T-%05d atribuida a %s%s", c.ID, best.Nome, skillSuffix(best.Habilidades, required))
			}
		}
	}
}

func (s *Server) loadAtendenteHabilidades(ctx context.Context, cands []store.AtendenteDisponivel) {
	habilidades, err := s.st.ListAtendentesHabilidades(ctx)
	if err != nil {
		return
	}
	for i := range cands {
		cands[i].Habilidades = habilidades[cands[i].AtendenteID]
	}
}

func initialAtendenteLoads(cands []store.AtendenteDisponivel) map[int]int {
	loads := make(map[int]int, len(cands))
	for i := range cands {
		loads[cands[i].UsuarioID] = cands[i].Carga
	}
	return loads
}

func effectiveAtendenteLoad(cd *store.AtendenteDisponivel, loads map[int]int) int {
	if load, ok := loads[cd.UsuarioID]; ok {
		return load
	}
	return cd.Carga
}

func skillSuffix(hab, required []string) string {
	if len(required) == 0 {
		return ""
	}
	labels := make([]string, 0, len(required))
	for _, skill := range required {
		labels = append(labels, skill)
	}
	return " [habilidade: " + strings.Join(labels, ", ") + "]"
}

func pickBestAtendente(c store.Conversation, cands []store.AtendenteDisponivel, modoAberto bool, now time.Time, required []string, loads map[int]int) *store.AtendenteDisponivel {
	eligible := make([]*store.AtendenteDisponivel, 0, len(cands))
	for i := range cands {
		cd := &cands[i]
		if !atendenteDisponivelParaConversa(c, cd, modoAberto, now, loads) {
			continue
		}
		eligible = append(eligible, cd)
	}
	if len(eligible) == 0 {
		return nil
	}
	if len(required) > 0 {
		withSkill := make([]*store.AtendenteDisponivel, 0, len(eligible))
		for _, cd := range eligible {
			if atendenteHasSkill(cd.Habilidades, required) {
				withSkill = append(withSkill, cd)
			}
		}
		if len(withSkill) > 0 {
			eligible = withSkill
		}
	}
	best := eligible[0]
	bestLoad := effectiveAtendenteLoad(best, loads)
	for _, cd := range eligible[1:] {
		load := effectiveAtendenteLoad(cd, loads)
		if load < bestLoad || (load == bestLoad && cd.UsuarioID < best.UsuarioID) {
			best = cd
			bestLoad = load
		}
	}
	return best
}

func atendenteDisponivelParaConversa(c store.Conversation, cd *store.AtendenteDisponivel, modoAberto bool, now time.Time, loads map[int]int) bool {
	if !dentroHorarioAtendente(cd.HorarioAbertura, cd.HorarioFechamento, cd.DiasSemana, now) {
		return false
	}
	if c.SetorID != nil && !modoAberto && !hasInt(cd.Setores, *c.SetorID) {
		return false
	}
	if cd.Capacidade > 0 && effectiveAtendenteLoad(cd, loads) >= cd.Capacidade {
		return false
	}
	return true
}

func atendenteHasSkill(hab, required []string) bool {
	for _, skill := range hab {
		for _, req := range required {
			if strings.EqualFold(strings.TrimSpace(skill), strings.TrimSpace(req)) {
				return true
			}
		}
	}
	return false
}

func hasInt(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// campanhaJob processa campanhas em lotes, distribuindo entre os canais do
// grupo com failover (canal que falha é pausado por uns minutos e testado de
// novo), cooldown por número e limite diário.
func (s *Server) campanhaJob(ctx context.Context) {
	if !s.moduleEnabled("campanhas") || s.sender == nil {
		return
	}
	campanhas, err := s.st.ListCampanhasToProcess(ctx)
	if err != nil || len(campanhas) == 0 {
		return
	}
	for _, camp := range campanhas {
		s.processCampanha(ctx, camp)
	}
}

func (s *Server) processCampanha(ctx context.Context, camp store.Campanha) {
	now := nowBR()

	// Agendada que chegou a hora -> roda.
	if camp.Status == "agendada" {
		if camp.ScheduledAt != nil && now.After(*camp.ScheduledAt) {
			_ = s.st.UpdateCampanhaStatus(ctx, camp.ID, "rodando")
			_ = s.st.SetCampanhaStarted(ctx, camp.ID)
		} else {
			return
		}
	}
	if camp.Status != "rodando" {
		return
	}

	// Canais do grupo (ou canal único).
	var chans []store.Channel
	if camp.GrupoID != nil {
		ids, _ := s.st.GrupoChannelIDs(ctx, *camp.GrupoID)
		for _, id := range ids {
			if c, err := s.st.GetChannelByID(ctx, id); err == nil && c.Active && c.CanCampaign && c.Type == "whatsapp" {
				chans = append(chans, c)
			}
		}
	} else if camp.ChannelID != nil {
		if c, err := s.st.GetChannelByID(ctx, *camp.ChannelID); err == nil && c.Active && c.CanCampaign {
			chans = append(chans, c)
		}
	}

	// Filtra canais que ainda estão em janela de retry (falharam há pouco).
	available := make([]store.Channel, 0, len(chans))
	for _, c := range chans {
		s.campDownMu.Lock()
		until, down := s.campDown[c.ID]
		s.campDownMu.Unlock()
		if down && now.Before(until) {
			continue
		}
		available = append(available, c)
	}

	if len(available) == 0 {
		Logger.Printf("campanha #%d: aguardando canal disponível (segue em andamento)", camp.ID)
		return
	}

	// Limite diário.
	if camp.LimiteDiario > 0 {
		sentToday, _ := s.st.CountEnviosSentToday(ctx, camp.ID)
		if sentToday >= camp.LimiteDiario {
			return // retoma automaticamente no dia seguinte
		}
	}

	pending, err := s.st.ListPendingEnvios(ctx, camp.ID, 20)
	if err != nil {
		return
	}
	if len(pending) == 0 {
		_ = s.st.MarkCampanhaCompleted(ctx, camp.ID)
		return
	}

	interval := camp.IntervaloSegundos
	if interval < 5 {
		interval = 5
	}
	volume, lastSent, _ := s.st.EnviosStatsByChannel(ctx, camp.ID)

	sent, failed := 0, 0
	for _, pe := range pending {
		// Canal com menor volume que já passou o cooldown.
		chosen := pickCampaignChannel(available, volume, lastSent, interval, now)
		if chosen == nil {
			break
		}

		msg := strings.ReplaceAll(camp.Message, "{nome}", pe.ContatoNome)
		var seed int64
		for _, r := range pe.Phone {
			seed = seed*31 + int64(r)
		}
		msg = channel.PerturbaTexto(msg, camp.VariarMensagem, seed)

		var sendErr error
		var waID string
		if camp.ModoTeste {
			Logger.Printf("campanha #%d [TESTE]: enviaria %s para %s via canal %q", camp.ID, camp.MediaTypeOrText(), pe.Phone, chosen.Name)
		} else if camp.MediaType != "" && camp.MediaB64 != "" {
			waID, sendErr = s.sender.SendMediaChannel(ctx, *chosen, pe.Phone, camp.MediaType, camp.MediaB64, camp.MediaMime, camp.MediaName, msg)
		} else {
			waID, sendErr = s.sender.SendTextChannel(ctx, *chosen, pe.Phone, msg)
		}

		if sendErr == nil {
			_ = s.st.MarkEnvioSent(ctx, pe.ID, chosen.ID, waID)
			sent++
			volume[chosen.ID]++
			lastSent[chosen.ID] = now
		} else {
			_ = s.st.MarkEnvioFailed(ctx, pe.ID, &chosen.ID, sendErr.Error())
			failed++
			s.campDownMu.Lock()
			s.campDown[chosen.ID] = now.Add(5 * time.Minute)
			s.campDownMu.Unlock()
			available = removeChannel(available, chosen.ID)
			if len(available) == 0 {
				break
			}
		}
	}
	if sent+failed > 0 {
		_ = s.st.IncrementCampanhaCounters(ctx, camp.ID, sent, failed)
	}
	if n, _ := s.st.CountPendingEnvios(ctx, camp.ID); n == 0 {
		_ = s.st.MarkCampanhaCompleted(ctx, camp.ID)
	}
}

// pickCampaignChannel escolhe o canal disponível com menor volume na campanha
// que já passou o cooldown (intervalo) desde o último envio.
func pickCampaignChannel(chans []store.Channel, volume map[int]int, lastSent map[int]time.Time, interval int, now time.Time) *store.Channel {
	var best *store.Channel
	bestVol := int(^uint(0) >> 1)
	for i := range chans {
		c := &chans[i]
		if ls, ok := lastSent[c.ID]; ok && now.Sub(ls) < time.Duration(interval)*time.Second {
			continue
		}
		v := volume[c.ID]
		if v < bestVol {
			bestVol = v
			best = c
		}
	}
	return best
}

func removeChannel(chans []store.Channel, id int) []store.Channel {
	out := chans[:0]
	for _, c := range chans {
		if c.ID != id {
			out = append(out, c)
		}
	}
	return out
}

// followupJob reativa contatos que pararam de responder há mais de
// followup_horas (recuperação de oportunidades).
func (s *Server) followupJob(ctx context.Context) {
	st, err := s.st.GetSettings(ctx)
	if err != nil || !st.FollowupAtivo || s.sender == nil {
		return
	}
	if st.FollowupHoras < 1 {
		st.FollowupHoras = 24
	}
	convs, err := s.st.ListFollowupCandidates(ctx, st.FollowupHoras)
	if err != nil || len(convs) == 0 {
		return
	}
	for _, c := range convs {
		if c.Channel != "whatsapp" || c.CustomerPhone == "" {
			continue
		}
		text := "Olá, tudo bem? 😊 Passando pra ver se você ainda precisa de ajuda com o que conversamos. Se precisar, é só me chamar por aqui!"
		if m, err := s.st.GetMensagemByChave(ctx, "followup"); err == nil && strings.TrimSpace(m.Texto) != "" {
			text = strings.TrimSpace(m.Texto)
		}
		text = strings.ReplaceAll(text, "{nome}", c.CustomerName)
		if err := s.sendConversationText(ctx, c, text, ""); err != nil {
			Logger.Printf("followup: erro enviando pra %s: %v", c.CustomerPhone, err)
			continue
		}
		_ = s.st.SetConversationFollowupSent(ctx, c.ID)
		Logger.Printf("followup: conversa #T-%05d enviado pra %s (%s)", c.ID, c.CustomerName, c.CustomerPhone)
	}
}

// watchdogJob cobre mensagens órfãs: se o pipeline travar silenciosamente
// (crash/deadlock/timeout) entre gravar a entrada do cliente e enviar a
// resposta, a conversa fica em 'ia' com a última mensagem do cliente. O
// watchdog responde um fallback, registra o aprendizado e, se configurado,
// devolve a conversa para a fila humana. Reaproveita o mesmo caminho de saída
// dos jobs de paciência/follow-up (sendConversationText).
func (s *Server) watchdogJob(ctx context.Context) {
	st, err := s.st.GetSettings(ctx)
	if err != nil || !st.WatchdogAtivo || s.sender == nil {
		return
	}
	minutos := st.WatchdogMinutos
	if minutos < 1 {
		minutos = 5
	}
	convs, err := s.st.ListOrphanConversations(ctx, minutos, 50)
	if err != nil || len(convs) == 0 {
		return
	}
	text := "Desculpe, tive um problema técnico ao processar sua mensagem. Já registrei o ocorrido. Pode repetir sua última mensagem, por favor? 😊"
	if m, merr := s.st.GetMensagemByChave(ctx, "watchdog_fallback"); merr == nil && strings.TrimSpace(m.Texto) != "" {
		text = strings.TrimSpace(m.Texto)
	}
	for _, c := range convs {
		if c.CustomerPhone == "" && c.Channel != "webchat" {
			continue
		}
		convID := c.ID
		if err := s.st.RecordAgentFailure(ctx, store.AgentFailure{
			ConversationID: &convID, Channel: c.Channel, Phone: c.CustomerPhone,
			Origem: "watchdog", Tipo: store.FailureKindPipeline,
			ExceptionClass: "OrphanMessage",
			MensagemErro:   fmt.Sprintf("mensagem do cliente sem resposta há mais de %d minutos", minutos),
		}); err != nil {
			Logger.Printf("watchdog: falha ao registrar audit da conversa #T-%05d: %v", c.ID, err)
		}
		if err := s.sendConversationText(ctx, c, text, ""); err != nil {
			Logger.Printf("watchdog: falha ao responder conversa #T-%05d (%s): %v", c.ID, c.CustomerPhone, err)
			_ = s.st.RecordAgentFailure(ctx, store.AgentFailure{
				ConversationID: &convID, Channel: c.Channel, Phone: c.CustomerPhone,
				Origem: "watchdog_envio", Tipo: store.FailureKindChannel,
				ExceptionClass: "FallbackSendFailed",
				MensagemErro:   err.Error(),
			})
			continue
		}
		_ = s.st.AddMessage(ctx, store.Message{ConversationID: c.ID, Role: "assistant", Source: "system", Content: text})
		if last, lerr := s.st.LastCustomerMessageForConversation(ctx, c.ID); lerr == nil && strings.TrimSpace(last) != "" {
			_ = s.st.AddAprendizado(ctx, last, c.Channel)
		}
		if st.TransferirFalhasHumano {
			_ = s.st.SetConversationStatus(ctx, c.ID, "humano_aguardando")
		}
		Logger.Printf("watchdog: conversa #T-%05d órfã há mais de %d min; fallback enviado para %s", c.ID, minutos, c.CustomerPhone)
	}
}

// mediaPurgeJob limpa o base64 de mídias antigas (histórico preservado),
// evitando acúmulo indefinido no banco. Só toca conversas encerradas/paradas.
func (s *Server) mediaPurgeJob(ctx context.Context) {
	st, err := s.st.GetSettings(ctx)
	if err != nil || !st.MediaPurgaAtivo {
		return
	}
	ttl := st.MediaTTLDias
	if ttl < 1 {
		ttl = 90
	}
	if n, err := s.st.PurgeOldMedia(ctx, ttl); err != nil {
		Logger.Printf("purga de mídia: erro: %v", err)
	} else if n > 0 {
		Logger.Printf("purga de mídia: %d mensagem(ns) com mídia antiga limpa", n)
	}
	if n, err := s.st.PurgeCampaignMedia(ctx, ttl); err != nil {
		Logger.Printf("purga de mídia de campanhas: erro: %v", err)
	} else if n > 0 {
		Logger.Printf("purga de mídia: %d campanha(s) antigas limpas", n)
	}
}

// nowBR retorna o horário atual em Brasília (UTC-3), independente do TZ do servidor.
func nowBR() time.Time {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return time.Now()
	}
	return time.Now().In(loc)
}

// dentroHorarioAtendente verifica se agora está dentro dos dias/horários do atendente.
func dentroHorarioAtendente(aberto, fecha, dias string, now time.Time) bool {
	inDay := false
	for _, d := range strings.Split(dias, ",") {
		if v, err := strconv.Atoi(strings.TrimSpace(d)); err == nil && v == int(now.Weekday()) {
			inDay = true
			break
		}
	}
	if !inDay {
		return false
	}
	ab, ok1 := toMinute(aberto)
	fe, ok2 := toMinute(fecha)
	if !ok1 || !ok2 {
		return true
	}
	cur := now.Hour()*60 + now.Minute()
	end := fe
	if fe == 1439 {
		end = 1440
	}
	if fe < ab {
		return cur >= ab || cur < end
	}
	return cur >= ab && cur < end
}

func toMinute(hhmm string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(hhmm), ":")
	if len(parts) != 2 {
		return 0, false
	}
	h, e1 := strconv.Atoi(parts[0])
	m, e2 := strconv.Atoi(parts[1])
	if e1 != nil || e2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

// ---------- IA (providers) ----------

func providerTestBaseURL(p store.Provider) string {
	if p.ProviderType == "notreve" {
		return llm.NotreveBaseURL
	}
	return strings.TrimRight(p.BaseURL, "/")
}

func testProvider(p store.Provider) (string, string) {
	baseURL := providerTestBaseURL(p)
	if baseURL == "" {
		return "false", "Sem base URL"
	}
	req, err := http.NewRequest(http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return "false", "Não foi possível montar a requisição para o provedor."
	}
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "false", "Não foi possível conectar ao provedor. Confira a URL e a rede."
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return "true", "Conectado"
	}
	if resp.StatusCode == http.StatusNotFound {
		return "false", "Endpoint de modelos não encontrado (HTTP 404). Confira a URL do provedor."
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "false", "Token inválido ou sem permissão (HTTP " + strconv.Itoa(resp.StatusCode) + "). Confira a credencial."
	}
	return "false", "HTTP " + strconv.Itoa(resp.StatusCode)
}

type providerModel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func listProviderModels(p store.Provider) ([]providerModel, error) {
	baseURL := providerTestBaseURL(p)
	if baseURL == "" {
		return nil, errors.New("sem base URL para consultar modelos")
	}
	if strings.TrimSpace(p.APIKey) == "" {
		return nil, errors.New("token do provedor não informado")
	}
	req, err := http.NewRequest(http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return nil, errors.New("não foi possível montar a requisição de modelos")
	}
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("não foi possível conectar ao provedor. Confira a URL e a rede")
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if readErr != nil {
		return nil, errors.New("não foi possível ler a resposta do provedor")
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, errors.New("token inválido ou sem permissão. Confira a credencial")
	case resp.StatusCode == http.StatusNotFound:
		return nil, errors.New("endpoint de modelos não encontrado (HTTP 404). Confira a URL do provedor")
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return nil, fmt.Errorf("o provedor respondeu HTTP %d ao listar modelos", resp.StatusCode)
	}
	return decodeProviderModels(body)
}

func decodeProviderModels(body []byte) ([]providerModel, error) {
	var payload struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
		Models []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, errors.New("resposta de modelos inválida do provedor")
	}
	entries := payload.Data
	if len(entries) == 0 {
		entries = payload.Models
	}
	seen := map[string]bool{}
	models := make([]providerModel, 0, len(entries))
	for _, entry := range entries {
		id := strings.TrimSpace(entry.ID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			name = id
		}
		models = append(models, providerModel{ID: id, Name: name})
	}
	if len(models) == 0 {
		return nil, errors.New("o provedor não retornou nenhum modelo disponível")
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models, nil
}

// ---------- Canais ----------

func channelFromForm(r *http.Request) store.Channel {
	var dominios []string
	for _, d := range strings.Split(r.FormValue("dominios"), "\n") {
		d = strings.TrimSpace(d)
		if d != "" {
			dominios = append(dominios, d)
		}
	}
	wc := map[string]interface{}{
		"titulo":          r.FormValue("wc_titulo"),
		"subtitulo":       r.FormValue("wc_subtitulo"),
		"boas_vindas":     r.FormValue("wc_boas_vindas"),
		"posicao":         r.FormValue("wc_posicao"),
		"apresentacao":    r.FormValue("wc_apresentacao"),
		"titulo_botao":    r.FormValue("wc_titulo_botao"),
		"cor_primaria":    r.FormValue("wc_cor_primaria"),
		"cor_botao":       r.FormValue("wc_cor_botao"),
		"cor_fundo":       r.FormValue("wc_cor_fundo"),
		"cor_texto":       r.FormValue("wc_cor_texto"),
		"cor_texto_botao": r.FormValue("wc_cor_texto_botao"),
		"icone_botao":     r.FormValue("wc_icone_botao"),
		"texto_input":     r.FormValue("wc_texto_input"),
		"texto_rodape":    r.FormValue("wc_texto_rodape"),
		"largura":         parseWidgetWidth(r.FormValue("wc_largura")),
		"dominios":        dominios,
	}
	wcJSON, _ := json.Marshal(wc)
	publicID := strings.TrimSpace(r.FormValue("public_id"))
	if r.FormValue("type") == "webchat" && publicID == "" {
		publicID = newPublicID()
	}
	return store.Channel{
		Type:              r.FormValue("type"),
		Name:              strings.TrimSpace(r.FormValue("name")),
		BaseURL:           strings.TrimSpace(r.FormValue("base_url")),
		Token:             strings.TrimSpace(r.FormValue("token")),
		Active:            r.FormValue("active") != "",
		RespondGrupos:     r.FormValue("respond_grupos") != "",
		ResponderChamadas: r.FormValue("responder_chamadas") != "",
		CanCampaign:       r.FormValue("can_campaign") != "",
		WidgetConfig:      string(wcJSON),
		PublicID:          publicID,
	}
}

func parseWidgetWidth(value string) int {
	width, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || width < 320 || width > 520 {
		return 360
	}
	return width
}

func newPublicID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ---------- Integrações ----------

func sensitiveHeaderName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.Contains(name, "authorization") || strings.Contains(name, "token") ||
		strings.Contains(name, "secret") || strings.Contains(name, "api-key") || strings.Contains(name, "apikey")
}

// preserveSensitiveHeaders permite editar uma integração sem obrigar o usuário
// a reenviar tokens já salvos. Campos sensíveis vazios no formulário mantêm o
// valor do banco; os valores nunca são renderizados no HTML.
func preserveSensitiveHeaders(currentJSON, submittedJSON string) string {
	current := map[string]string{}
	submitted := map[string]string{}
	if json.Unmarshal([]byte(currentJSON), &current) != nil || json.Unmarshal([]byte(submittedJSON), &submitted) != nil {
		return submittedJSON
	}
	for key, value := range current {
		if sensitiveHeaderName(key) && strings.TrimSpace(submitted[key]) == "" {
			submitted[key] = value
		}
	}
	b, err := json.Marshal(submitted)
	if err != nil {
		return submittedJSON
	}
	return string(b)
}

// ---------- Mensagens ----------

// ---------- Aprendizado ----------

// ---------- Respostas rápidas ----------

// ---------- Contatos ----------

// handleConversaNova abre uma conversa direto do chat: salva o contato
// (se não existir) e inicia o ticket atribuído ao usuário atual.
// ---------- API ----------

// signedText adiciona a assinatura do remetente conforme o canal, pro cliente
// saber com quem está falando.
func signedText(channel, senderName, text string) string {
	name := strings.TrimSpace(senderName)
	if name == "" {
		return text
	}
	switch channel {
	case "whatsapp":
		return "*" + name + ":*\n" + text
	case "webchat":
		return "**" + name + ":**\n" + text
	default:
		return name + ":\n" + text
	}
}

type channelEditSender interface {
	SendEditChannel(context.Context, store.Channel, string, string, string) error
}

type channelTypingSender interface {
	SendTypingChannel(context.Context, store.Channel, string, bool) error
}

type channelReactionSender interface {
	SendReactionChannel(context.Context, store.Channel, string, string, string) error
}

type channelRichSender interface {
	SendRichMessageChannel(context.Context, store.Channel, string, *agent.RichMessage) error
}

func (s *Server) conversationChannel(ctx context.Context, conv store.Conversation) (store.Channel, error) {
	if conv.ChannelID == nil {
		return store.Channel{}, errors.New("conversa sem canal específico")
	}
	return s.st.GetChannelByIDSecure(ctx, *conv.ChannelID)
}

// sendConversationText é o único caminho de texto usado pelo painel e pelos
// jobs internos. Conversas novas sempre têm ChannelID; o fallback global fica
// apenas para registros antigos sem vínculo explícito.
func (s *Server) sendConversationText(ctx context.Context, conv store.Conversation, text, replyTo string) error {
	if conv.ChannelID != nil {
		ch, err := s.conversationChannel(ctx, conv)
		if err != nil {
			return err
		}
		if adapter := s.channels.For(conv.Channel); adapter != nil {
			return adapter.Send(ctx, ch, conv.CustomerPhone, channel.Outbound{Text: text, ReplyTo: replyTo})
		}
		if conv.Channel == "whatsapp" {
			if sender, ok := s.sender.(interface {
				SendTextChannelReply(context.Context, store.Channel, string, string, string) error
			}); ok && replyTo != "" {
				return sender.SendTextChannelReply(ctx, ch, conv.CustomerPhone, text, replyTo)
			}
			if sender, ok := s.sender.(interface {
				SendTextChannel(context.Context, store.Channel, string, string) (string, error)
			}); ok {
				_, err := sender.SendTextChannel(ctx, ch, conv.CustomerPhone, text)
				return err
			}
		}
		return errors.New("adaptador de saída não configurado para o canal")
	}
	if conv.Channel == "webchat" {
		return nil
	}
	if (conv.Channel == "whatsapp" || conv.Channel == "") && s.sender != nil {
		if replyTo != "" {
			return s.sender.SendTextReply(ctx, conv.CustomerPhone, text, replyTo)
		}
		return s.sender.SendText(ctx, conv.CustomerPhone, text)
	}
	return errors.New("canal de saída não configurado")
}

func (s *Server) sendConversationMedia(ctx context.Context, conv store.Conversation, media channel.OutboundMedia) error {
	if conv.ChannelID != nil {
		ch, err := s.conversationChannel(ctx, conv)
		if err != nil {
			return err
		}
		if adapter := s.channels.For(conv.Channel); adapter != nil {
			return adapter.SendMedia(ctx, ch, conv.CustomerPhone, media)
		}
		if sender, ok := s.sender.(interface {
			SendMediaChannel(context.Context, store.Channel, string, string, string, string, string, string) (string, error)
		}); ok {
			_, err := sender.SendMediaChannel(ctx, ch, conv.CustomerPhone, media.MediaType, media.DataB64, media.Mime, media.FileName, media.Caption)
			return err
		}
		return errors.New("sender não suporta mídia pelo canal específico")
	}
	if s.sender != nil && (conv.Channel == "whatsapp" || conv.Channel == "") {
		return s.sender.SendMedia(ctx, conv.CustomerPhone, media.MediaType, media.DataB64, media.Mime, media.FileName, media.Caption)
	}
	return errors.New("canal de mídia não configurado")
}

func (s *Server) SendGeneratedMedia(ctx context.Context, convID int, media agent.GeneratedMedia) error {
	conv, err := s.st.GetConversation(ctx, convID)
	if err != nil {
		return err
	}
	message := store.Message{
		ConversationID: convID, Role: "assistant", Source: "ai", Content: media.Caption,
		MediaType: media.MediaType, MediaB64: media.DataB64, MediaMime: media.Mime, MediaName: media.FileName,
	}
	if err := s.st.AddMessage(ctx, message); err != nil {
		Logger.Printf("mídia gerada: falha ao registrar a mensagem (conversa %d): %v", convID, err)
	}
	outbound := channel.OutboundMedia{
		MediaType: media.MediaType, DataB64: media.DataB64, Mime: media.Mime, FileName: media.FileName, Caption: media.Caption,
	}
	if err := s.sendConversationMedia(ctx, conv, outbound); err != nil {
		if errors.Is(err, channel.ErrUnsupported) {
			Logger.Printf("mídia gerada: canal %q não suporta mídia (conversa %d); mensagem registrada no painel", conv.Channel, convID)
			return nil
		}
		return err
	}
	return nil
}

func (s *Server) sendConversationEdit(ctx context.Context, conv store.Conversation, phone, waMsgID, text string) error {
	if conv.ChannelID != nil {
		ch, err := s.conversationChannel(ctx, conv)
		if err != nil {
			return err
		}
		if sender, ok := s.sender.(channelEditSender); ok {
			return sender.SendEditChannel(ctx, ch, phone, waMsgID, text)
		}
		return errors.New("sender não suporta edição pelo canal específico")
	}
	if s.sender != nil {
		return s.sender.SendEdit(ctx, phone, waMsgID, text)
	}
	return errors.New("WhatsApp não configurado")
}

func (s *Server) sendConversationTyping(ctx context.Context, conv store.Conversation, typing bool) error {
	if conv.ChannelID != nil {
		ch, err := s.conversationChannel(ctx, conv)
		if err != nil {
			return err
		}
		if sender, ok := s.sender.(channelTypingSender); ok {
			return sender.SendTypingChannel(ctx, ch, conv.CustomerPhone, typing)
		}
		return errors.New("sender não suporta digitação pelo canal específico")
	}
	if s.sender != nil {
		return s.sender.SendTyping(ctx, conv.CustomerPhone, typing)
	}
	return errors.New("WhatsApp não configurado")
}

func (s *Server) sendConversationReaction(ctx context.Context, conv store.Conversation, waMsgID, emoji string) error {
	if conv.ChannelID != nil {
		ch, err := s.conversationChannel(ctx, conv)
		if err != nil {
			return err
		}
		if sender, ok := s.sender.(channelReactionSender); ok {
			return sender.SendReactionChannel(ctx, ch, conv.CustomerPhone, waMsgID, emoji)
		}
		return errors.New("sender não suporta reação pelo canal específico")
	}
	if s.sender != nil {
		return s.sender.SendReaction(ctx, conv.CustomerPhone, waMsgID, emoji)
	}
	return errors.New("WhatsApp não configurado")
}

// enqueueEvent enfileira um evento de domínio na outbox de forma best-effort.
// Um erro é registrado em log e nunca interrompe a ação do operador.
func (s *Server) enqueueEvent(ctx context.Context, event webhooks.Event, key string, payload interface{}) {
	if err := s.st.PublishOutboxEvent(ctx, event, key, payload); err != nil {
		Logger.Printf("webhook outbox: evento %s não publicado: %v", event, err)
	}
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}
	var in struct {
		Content string `json:"content"`
		ReplyTo *int   `json:"reply_to"` // id da mensagem no banco a responder
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.Content) == "" {
		http.Error(w, `{"error":"conteúdo vazio"}`, http.StatusBadRequest)
		return
	}
	in.Content = strings.TrimSpace(in.Content)

	ctx := r.Context()
	conv, err := s.st.GetConversation(ctx, id)
	if err != nil {
		http.Error(w, `{"error":"conversa não encontrada"}`, http.StatusNotFound)
		return
	}
	if !s.canActOn(ctx, currentUser(r), id) {
		http.Error(w, `{"error":"aceite o atendimento antes de responder"}`, http.StatusForbidden)
		return
	}

	// Atendente só responde após ter aceitado o atendimento.
	if u := currentUser(r); u != nil && u.Role == "atendente" {
		if conv.Status == "humano_aguardando" || conv.AtendenteID == nil || *conv.AtendenteID != u.ID {
			http.Error(w, `{"error":"aceite o atendimento antes de responder"}`, http.StatusForbidden)
			return
		}
	}

	if err := s.st.AddMessage(ctx, store.Message{ConversationID: id, Role: "assistant", Content: in.Content, Source: "human"}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = s.st.TouchConversation(ctx, id)
	// Ao responder como humano, marca como EM ATENDIMENTO e atribui ao atendente.
	if u := currentUser(r); u != nil {
		uid := u.ID
		_ = s.st.SetConversationAtendente(ctx, id, &uid)
	}
	_ = s.st.SetConversationStatus(ctx, id, "humano_atendendo")
	s.enqueueEvent(ctx, webhooks.EventMessageSent, "message.sent:"+strconv.Itoa(id)+":human:"+strconv.FormatInt(time.Now().UnixNano(), 10), map[string]interface{}{
		"conversation": store.ConversationWebhookPayload(conv),
		"message": store.MessageWebhookPayload(store.Message{
			ConversationID: id, Role: "assistant", Source: "human", Content: in.Content, CreatedAt: time.Now().UTC(),
		}),
	})
	s.enqueueEvent(ctx, webhooks.EventConversationUpdated, "conversation.updated:"+strconv.Itoa(id)+":"+strconv.FormatInt(time.Now().UnixNano(), 10), map[string]interface{}{
		"conversation": store.ConversationWebhookPayload(conv),
		"status":       "humano_atendendo",
	})

	// Entrega pela instância exata que recebeu a conversa.
	delivered := false
	replyToWAMsgID := ""
	if in.ReplyTo != nil {
		if msg, err := s.st.GetMessage(ctx, *in.ReplyTo); err == nil && msg.ConversationID == id {
			replyToWAMsgID = msg.WAMsgID
		}
	}
	outgoing := in.Content
	if u := currentUser(r); u != nil && permEnabled(u, permEnviarAssinatura) {
		outgoing = signedText(conv.Channel, u.Name, in.Content)
	}

	if conv.CustomerPhone != "" || conv.Channel == "webchat" {
		delivered = s.sendConversationText(ctx, conv, outgoing, replyToWAMsgID) == nil
	}

	writeJSON(w, map[string]interface{}{"ok": true, "delivered": delivered})
}

func (s *Server) handleMedia(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	conv, err := s.st.GetConversation(ctx, id)
	if err != nil {
		http.Error(w, `{"error":"conversa não encontrada"}`, http.StatusNotFound)
		return
	}
	if !s.canActOn(ctx, currentUser(r), id) {
		http.Error(w, `{"error":"aceite o atendimento antes de enviar mídia"}`, http.StatusForbidden)
		return
	}
	const maxMediaUpload = 50 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxMediaUpload)
	if err := r.ParseMultipartForm(maxMediaUpload); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, `{"error":"arquivo excede o limite de 50 MB"}`, http.StatusRequestEntityTooLarge)
			return
		}
		log.Printf("upload de mídia inválido: %v", err)
		http.Error(w, `{"error":"não foi possível processar o upload"}`, http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"error":"arquivo obrigatório"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil || len(data) == 0 {
		http.Error(w, `{"error":"arquivo vazio"}`, http.StatusBadRequest)
		return
	}

	mediaType, mime := detectMedia(header.Filename, header.Header.Get("Content-Type"))
	if mediaType == "" {
		http.Error(w, `{"error":"tipo de arquivo não suportado"}`, http.StatusBadRequest)
		return
	}

	caption := strings.TrimSpace(r.FormValue("caption"))
	display := caption
	if display == "" {
		display = header.Filename
	}

	delivered := false
	if conv.CustomerPhone != "" {
		err := s.sendConversationMedia(ctx, conv, channel.OutboundMedia{
			MediaType: mediaType,
			DataB64:   base64.StdEncoding.EncodeToString(data),
			Mime:      mime,
			FileName:  header.Filename,
			Caption:   caption,
		})
		if err != nil {
			// Não gravar nem retornar "Mensagem enviada" quando a Notreve
			// recusou o arquivo. Antes o painel persistia o cartão mesmo com
			// falha de entrega, mascarando o problema para o operador.
			Logger.Printf("falha enviando mídia na conversa #%d: %v", id, err)
			payload, _ := json.Marshal(map[string]string{"error": "não foi possível entregar o arquivo ao cliente: " + err.Error()})
			http.Error(w, string(payload), http.StatusBadGateway)
			return
		}
		delivered = true
	} else {
		payload, _ := json.Marshal(map[string]string{"error": "esta conversa não possui um destinatário WhatsApp válido"})
		http.Error(w, string(payload), http.StatusUnprocessableEntity)
		return
	}

	if err := s.st.AddMessage(ctx, store.Message{
		ConversationID: id, Role: "assistant", Content: "📎 " + display, Source: "human",
		MediaType: mediaType, MediaB64: base64.StdEncoding.EncodeToString(data), MediaMime: mime, MediaName: header.Filename,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = s.st.TouchConversation(ctx, id)
	if u := currentUser(r); u != nil {
		uid := u.ID
		_ = s.st.SetConversationAtendente(ctx, id, &uid)
	}
	_ = s.st.SetConversationStatus(ctx, id, "humano_atendendo")
	s.enqueueEvent(ctx, webhooks.EventMessageSent, "message.sent:"+strconv.Itoa(id)+":human:"+strconv.FormatInt(time.Now().UnixNano(), 10), map[string]interface{}{
		"conversation": store.ConversationWebhookPayload(conv),
		"message": store.MessageWebhookPayload(store.Message{
			ConversationID: id, Role: "assistant", Source: "human", Content: "📎 " + display,
			MediaType: mediaType, MediaName: header.Filename, CreatedAt: time.Now().UTC(),
		}),
	})
	s.enqueueEvent(ctx, webhooks.EventConversationUpdated, "conversation.updated:"+strconv.Itoa(id)+":"+strconv.FormatInt(time.Now().UnixNano(), 10), map[string]interface{}{
		"conversation": store.ConversationWebhookPayload(conv),
		"status":       "humano_atendendo",
	})

	writeJSON(w, map[string]interface{}{"ok": true, "delivered": delivered})
}

// handleMessageMediaAPI devolve a mídia de uma mensagem (base64) sob demanda.
func (s *Server) handleMessageMediaAPI(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"id inválido"}`, http.StatusBadRequest)
		return
	}
	m, err := s.st.GetMessage(r.Context(), id)
	if err != nil || m.MediaB64 == "" {
		http.Error(w, `{"error":"mídia não encontrada"}`, http.StatusNotFound)
		return
	}
	ctx := r.Context()
	conv, err := s.st.GetConversation(ctx, m.ConversationID)
	if err != nil || !s.canSee(currentUser(r), conv, s.userSetores(ctx, currentUser(r)), s.modoAberto(ctx)) {
		http.Error(w, `{"error":"mídia não encontrada"}`, http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]interface{}{
		"id": m.ID, "media_type": m.MediaType, "media_b64": m.MediaB64, "media_mime": m.MediaMime, "media_name": m.MediaName,
	})
}

func detectMedia(filename, contentType string) (mediaType, mediaMIME string) {
	ext := strings.ToLower(path.Ext(filename))
	baseMIME := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	resolvedMIME := strings.TrimSpace(contentType)
	if resolvedMIME == "" {
		resolvedMIME = stdmime.TypeByExtension(ext)
	}
	switch {
	case ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp":
		if resolvedMIME == "" {
			resolvedMIME = "application/octet-stream"
		}
		return "image", resolvedMIME
	case ext == ".mp3" || ext == ".ogg" || ext == ".opus" || ext == ".wav" || ext == ".m4a":
		if resolvedMIME == "" {
			resolvedMIME = "audio/ogg"
		}
		return "audio", resolvedMIME
	case ext == ".webm" && strings.HasPrefix(baseMIME, "audio/"):
		if resolvedMIME == "" {
			resolvedMIME = "audio/webm"
		}
		return "audio", resolvedMIME
	case ext == ".mp4" || ext == ".3gp" || ext == ".mov":
		if resolvedMIME == "" {
			resolvedMIME = "video/mp4"
		}
		return "video", resolvedMIME
	case ext == ".webm":
		if resolvedMIME == "" {
			resolvedMIME = "video/webm"
		}
		return "video", resolvedMIME
	case ext == ".pdf" || ext == ".doc" || ext == ".docx" || ext == ".xls" || ext == ".xlsx" ||
		ext == ".txt" || ext == ".csv" || ext == ".zip":
		if resolvedMIME == "" {
			resolvedMIME = "application/octet-stream"
		}
		return "document", resolvedMIME
	}
	return "", ""
}

func (s *Server) handleAccept(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}
	u := currentUser(r)
	if u == nil {
		http.Error(w, `{"error":"não autenticado"}`, http.StatusUnauthorized)
		return
	}
	ctx := r.Context()
	conv, err := s.st.GetConversation(ctx, id)
	if err != nil || !s.canSee(u, conv, s.userSetores(ctx, u), s.modoAberto(ctx)) {
		http.Error(w, `{"error":"conversa não encontrada"}`, http.StatusNotFound)
		return
	}
	if !conversationActionAllowed(conv.Status, convActionAccept) {
		http.Error(w, `{"error":"conversa não está disponível para assumir"}`, http.StatusConflict)
		return
	}
	if !s.canManageConversation(ctx, u, conv) {
		http.Error(w, `{"error":"sem permissão para assumir esta conversa"}`, http.StatusForbidden)
		return
	}
	claimed, err := s.st.ClaimConversation(ctx, id, u.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !claimed {
		http.Error(w, `{"error":"conversa já foi aceita por outro atendente"}`, http.StatusConflict)
		return
	}
	_ = s.st.AddMessage(ctx, store.Message{ConversationID: id, Role: "system", Source: "system", Content: "👤 " + u.Name + " iniciou o atendimento"})
	// Avisa o cliente que um operador assumiu.
	if conv.CustomerPhone != "" {
		_ = s.sendConversationText(ctx, conv, "👋 "+u.Name+" entrou no seu atendimento!", "")
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

// handleEncerrar encerra com motivo e inicia a pesquisa de satisfação.
func (s *Server) handleEncerrar(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}
	var in struct {
		Motivo     string `json:"motivo"`
		Silencioso bool   `json:"silencioso"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	conv, err := s.st.GetConversation(ctx, id)
	if err != nil {
		http.Error(w, `{"error":"conversa não encontrada"}`, http.StatusNotFound)
		return
	}
	if !s.canManageConversation(ctx, currentUser(r), conv) {
		http.Error(w, `{"error":"sem permissão para encerrar esta conversa"}`, http.StatusForbidden)
		return
	}
	if !conversationActionAllowed(conv.Status, convActionClose) {
		http.Error(w, `{"error":"esta conversa já está encerrada"}`, http.StatusConflict)
		return
	}
	var encerradoPor *int
	if u := currentUser(r); u != nil {
		encerradoPor = &u.ID
	}
	if in.Silencioso {
		// Encerramento silencioso: só admin/super_admin; cliente não recebe aviso nem satisfação.
		if u := currentUser(r); u == nil || (u.Role != "admin" && u.Role != "super_admin") {
			http.Error(w, `{"error":"só admin pode encerrar silenciosamente"}`, http.StatusForbidden)
			return
		}
		if err := s.st.EncerrarConversationSilencioso(ctx, id, strings.TrimSpace(in.Motivo), encerradoPor); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.enqueueEvent(ctx, webhooks.EventConversationClosed, "conversation.closed:"+strconv.Itoa(id), map[string]interface{}{
			"conversation": store.ConversationWebhookPayload(conv), "reason": strings.TrimSpace(in.Motivo),
		})
		writeJSON(w, map[string]string{"ok": "true", "silencioso": "true"})
		return
	}
	if err := s.st.EncerrarConversation(ctx, id, strings.TrimSpace(in.Motivo), encerradoPor); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.enqueueEvent(ctx, webhooks.EventConversationClosed, "conversation.closed:"+strconv.Itoa(id), map[string]interface{}{
		"conversation": store.ConversationWebhookPayload(conv), "reason": strings.TrimSpace(in.Motivo),
	})
	// Dispara a pesquisa de satisfação (WhatsApp).
	s.enviarSatisfacao(ctx, conv, id)
	writeJSON(w, map[string]string{"ok": "true"})
}

// enviarSatisfacao manda a mensagem de pesquisa de satisfação pro cliente.
func (s *Server) enviarSatisfacao(ctx context.Context, conv store.Conversation, convID int) {
	text, err := channel.BeginSatisfaction(ctx, s.st, convID)
	if err != nil {
		Logger.Printf("erro iniciando avaliação: %v", err)
		return
	}
	if (conv.Channel == "whatsapp" || conv.Channel == "") && s.sender != nil && conv.CustomerPhone != "" {
		if n, ok := s.sender.(*channel.Notreve); ok {
			if rich := channel.SatisfactionRich(text); rich != nil {
				if ch, chErr := s.conversationChannel(ctx, conv); chErr == nil {
					if err := n.SendRichMessageChannel(ctx, ch, conv.CustomerPhone, rich); err == nil {
						return
					}
					Logger.Printf("envio rico de avaliação pelo canal específico falhou; usando texto: %v", chErr)
				} else if conv.ChannelID == nil {
					if err := n.SendRichMessage(ctx, conv.CustomerPhone, rich); err == nil {
						return
					}
					Logger.Printf("envio rico de avaliação falhou, usando texto: %v", err)
				}
			}
		}
		if err := s.sendConversationText(ctx, conv, text, ""); err != nil {
			Logger.Printf("erro enviando pergunta de avaliação: %v", err)
		}
	}
}

// canActOn: só admin age sem aceitar; atendente/supervisor precisam ter o
// atendimento atribuído a eles (aceito).
func (s *Server) canActOn(ctx context.Context, user *store.Usuario, convID int) bool {
	if user == nil {
		return false
	}
	if user.Role == "admin" || user.Role == "super_admin" {
		return true
	}
	conv, err := s.st.GetConversation(ctx, convID)
	if err != nil {
		return false
	}
	if user.Role == "supervisor" {
		return s.canSee(user, conv, s.userSetores(ctx, user), s.modoAberto(ctx))
	}
	return conv.AtendenteID != nil && *conv.AtendenteID == user.ID
}

func (s *Server) canSeeTransferTarget(ctx context.Context, user *store.Usuario, target store.Usuario) bool {
	if user == nil || user.Role == "admin" || user.Role == "super_admin" {
		return true
	}
	if user.Role != "supervisor" {
		return true
	}
	if !target.Active || target.AtendenteID == nil {
		return false
	}
	return hasSectorIntersection(s.userSetores(ctx, user), s.userSetores(ctx, &target))
}

func hasSectorIntersection(left, right []int) bool {
	seen := make(map[int]bool, len(left))
	for _, id := range left {
		seen[id] = true
	}
	for _, id := range right {
		if seen[id] {
			return true
		}
	}
	return false
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}
	var in struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Status == "" {
		http.Error(w, `{"error":"status vazio"}`, http.StatusBadRequest)
		return
	}
	valid := map[string]bool{"ia": true, "humano_aguardando": true, "humano_atendendo": true, "closed": true}
	if !valid[in.Status] {
		http.Error(w, `{"error":"status inválido"}`, http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	conv, err := s.st.GetConversation(ctx, id)
	if err != nil {
		http.Error(w, `{"error":"conversa não encontrada"}`, http.StatusNotFound)
		return
	}
	if !conversationStatusTransitionAllowed(conv.Status, in.Status) {
		http.Error(w, `{"error":"transição de atendimento inválida"}`, http.StatusConflict)
		return
	}
	if !s.canManageConversation(ctx, currentUser(r), conv) {
		http.Error(w, `{"error":"sem permissão para alterar esta conversa"}`, http.StatusForbidden)
		return
	}
	if in.Status == "ia" {
		if err := s.st.SetConversationToAI(ctx, id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else if err := s.st.SetConversationStatus(ctx, id, in.Status); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if in.Status == "closed" {
		_ = s.st.AddMessage(ctx, store.Message{
			ConversationID: id, Role: "system", Source: "system",
			Content: fmt.Sprintf("🔖 Protocolo de atendimento #T-%05d encerrado", id),
		})
	}
	if conv, err := s.st.GetConversation(ctx, id); err == nil {
		s.enqueueEvent(ctx, webhooks.EventConversationUpdated, "conversation.updated:"+strconv.Itoa(id)+":"+strconv.FormatInt(time.Now().UnixNano(), 10), map[string]interface{}{
			"conversation": store.ConversationWebhookPayload(conv), "status": in.Status,
		})
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

// handleReturnToAI devolve um atendimento humano para a IA. A atribuição
// humana é removida junto com o status para que a próxima mensagem volte a
// ser processada pelo agente configurado para aquele canal.
func (s *Server) handleReturnToAI(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	conv, err := s.st.GetConversation(ctx, id)
	if err != nil {
		http.Error(w, `{"error":"conversa não encontrada"}`, http.StatusNotFound)
		return
	}
	if !conversationActionAllowed(conv.Status, convActionReturnToAI) {
		http.Error(w, `{"error":"esta conversa não pode ser devolvida para a IA"}`, http.StatusConflict)
		return
	}
	if !s.canManageConversation(ctx, currentUser(r), conv) {
		http.Error(w, `{"error":"sem permissão para alterar esta conversa"}`, http.StatusForbidden)
		return
	}
	if err := s.st.SetConversationToAI(ctx, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = s.st.AddMessage(ctx, store.Message{
		ConversationID: id, Role: "system", Source: "system",
		Content: "🔁 Atendimento devolvido para a IA",
	})
	if updated, err := s.st.GetConversation(ctx, id); err == nil {
		s.enqueueEvent(ctx, webhooks.EventAgentTransferred, "agent.transferred:"+strconv.Itoa(id)+":"+strconv.FormatInt(time.Now().UnixNano(), 10), map[string]interface{}{
			"conversation": store.ConversationWebhookPayload(updated), "destino": "IA", "to_status": "ia",
		})
		s.enqueueEvent(ctx, webhooks.EventConversationUpdated, "conversation.updated:"+strconv.Itoa(id)+":"+strconv.FormatInt(time.Now().UnixNano(), 10), map[string]interface{}{
			"conversation": store.ConversationWebhookPayload(updated), "status": "ia",
		})
	}
	writeJSON(w, map[string]interface{}{"ok": true, "status": "ia"})
}

func (s *Server) handleConversationSetor(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}
	var in struct {
		SetorID     int `json:"setor_id"`
		AtendenteID int `json:"atendente_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	conv, gerr := s.st.GetConversation(ctx, id)
	if gerr != nil {
		http.Error(w, `{"error":"conversa não encontrada"}`, http.StatusNotFound)
		return
	}
	if !conversationActionAllowed(conv.Status, convActionTransfer) {
		http.Error(w, `{"error":"não é possível transferir uma conversa encerrada"}`, http.StatusConflict)
		return
	}
	if !s.canManageConversation(ctx, currentUser(r), conv) {
		http.Error(w, `{"error":"sem permissão para transferir esta conversa"}`, http.StatusForbidden)
		return
	}
	user := currentUser(r)

	destino := ""
	if in.SetorID > 0 {
		sid := in.SetorID
		if user != nil && user.Role == "supervisor" && !containsInt(s.userSetores(ctx, user), sid) {
			http.Error(w, `{"error":"setor fora do escopo do supervisor"}`, http.StatusForbidden)
			return
		}
		if err := s.st.SetConversationSetor(ctx, id, &sid); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if setor, err := s.st.GetSetor(ctx, sid); err == nil {
			destino = "setor " + setor.Name
		} else {
			destino = "setor"
		}
		// Desvincula do atendente atual -> fica "pendente de aceite" na fila do setor.
		_ = s.st.SetConversationAtendente(ctx, id, nil)
	} else if in.AtendenteID > 0 {
		aid := in.AtendenteID
		target, err := s.st.GetUsuarioByID(ctx, aid)
		if err != nil || !reactTransferTargetUserAllowed(target) || !s.canSeeTransferTarget(ctx, user, target) {
			http.Error(w, `{"error":"destino fora do escopo permitido"}`, http.StatusForbidden)
			return
		}
		if err := s.st.SetConversationAtendente(ctx, id, &aid); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if u, err := s.st.GetUsuarioByID(ctx, aid); err == nil {
			destino = "operador " + u.Name
		} else {
			destino = "operador"
		}
	} else {
		http.Error(w, `{"error":"informe setor ou atendente"}`, http.StatusBadRequest)
		return
	}

	// Vai pra fila humana (pendente de aceite).
	_ = s.st.SetConversationStatus(ctx, id, "humano_aguardando")
	s.enqueueEvent(ctx, webhooks.EventAgentTransferred, "agent.transferred:"+strconv.Itoa(id)+":"+strconv.FormatInt(time.Now().UnixNano(), 10), map[string]interface{}{
		"conversation": store.ConversationWebhookPayload(conv), "destino": destino, "to_status": "humano_aguardando",
	})

	// Registra no chat e avisa o cliente de forma clara/direta.
	if in.AtendenteID > 0 {
		_ = s.st.AddMessage(ctx, store.Message{ConversationID: id, Role: "system", Source: "system", Content: "🔁 Transferido para " + destino})
		if conv.CustomerPhone != "" {
			_ = s.sendConversationText(ctx, conv, "🔁 Seu atendimento será feito pelo(a) "+destino+". Um instante 😊", "")
		}
	} else {
		_ = s.st.AddMessage(ctx, store.Message{ConversationID: id, Role: "system", Source: "system", Content: "🔁 Transferido para " + destino})
		if conv.CustomerPhone != "" {
			_ = s.sendConversationText(ctx, conv, "🔁 Você foi transferido para o "+destino+". Um atendente vai te atender em instantes 😊", "")
		}
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleTyping(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}
	if !s.canActOn(r.Context(), currentUser(r), id) {
		http.Error(w, `{"error":"aceite o atendimento antes de agir"}`, http.StatusForbidden)
		return
	}
	var in struct {
		Typing bool `json:"typing"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	conv, err := s.st.GetConversation(ctx, id)
	if err != nil {
		http.Error(w, `{"error":"conversa não encontrada"}`, http.StatusNotFound)
		return
	}
	if conv.CustomerPhone != "" {
		_ = s.sendConversationTyping(ctx, conv, in.Typing)
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleReaction(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}
	var in struct {
		MsgID int    `json:"msg_id"`
		Emoji string `json:"emoji"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Emoji == "" {
		http.Error(w, `{"error":"emoji vazio"}`, http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	conv, err := s.st.GetConversation(ctx, id)
	if err != nil {
		http.Error(w, `{"error":"conversa não encontrada"}`, http.StatusNotFound)
		return
	}
	if !s.canActOn(r.Context(), currentUser(r), id) {
		http.Error(w, `{"error":"aceite o atendimento antes de agir"}`, http.StatusForbidden)
		return
	}
	msg, err := s.st.GetMessage(ctx, in.MsgID)
	if err != nil || msg.ConversationID != id || msg.WAMsgID == "" {
		http.Error(w, `{"error":"mensagem sem ID no WhatsApp"}`, http.StatusBadRequest)
		return
	}
	ok := false
	if conv.CustomerPhone != "" {
		if err := s.sendConversationReaction(ctx, conv, msg.WAMsgID, in.Emoji); err == nil {
			ok = true
		}
	}
	writeJSON(w, map[string]interface{}{"ok": true, "reacted": ok})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// ---------- Quick Setup ----------

type setupStep struct {
	Num      int
	Nome     string
	Desc     string
	Link     string
	Done     bool
	Opcional bool
}
