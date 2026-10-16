package web

import (
	"net/http"
	"strings"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

// Permissões granulares.
const (
	permConversasVer          = "conversas_ver"
	permConversasResponder    = "conversas_responder"
	permConversasTransferir   = "conversas_transferir"
	permConversasEncerrar     = "conversas_encerrar"
	permConversasVerIA        = "conversas_ver_ia"
	permEnviarAssinatura      = "enviar_assinatura"
	permContatosGerenciar     = "contatos_gerenciar"
	permRespostasGerenciar    = "respostas_gerenciar"
	permMensagensGerenciar    = "mensagens_gerenciar"
	permAgentesGerenciar      = "agentes_gerenciar"
	permIAsGerenciar          = "ias_gerenciar"
	permAprendizadoVer        = "aprendizado_ver"
	permChatflowsGerenciar    = "chatflows_gerenciar"
	permSetoresGerenciar      = "setores_gerenciar"
	permEquipeGerenciar       = "equipe_gerenciar"
	permCanaisGerenciar       = "canais_gerenciar"
	permIntegracoesGerenciar  = "integracoes_gerenciar"
	permConfigGerenciar       = "config_gerenciar"
	permCampanhasVer          = "campanhas_ver"
	permCampanhasGerenciar    = "campanhas_gerenciar"
	permGruposCanaisGerenciar = "grupos_canais_gerenciar"
	permAgendamentosGerenciar = "agendamentos_gerenciar"
	permRelatoriosVer         = "relatorios_ver"
	permLogsVer               = "logs_ver"
	permPlataformaGerenciar   = "plataforma_gerenciar"
)

// Permissões padrão por perfil (usadas quando o usuário não tem lista salva).
// Assinatura (enviar_assinatura) vem LIGADA por padrão — o admin pode desligar por usuário.
var roleDefaultPerms = map[string][]string{
	// O super_admin é o dono da plataforma: recebe também a permissão de
	// plataforma (planos, limites, tenants, white-label e provedores globais).
	"super_admin": allPermsList(),
	"admin":       allPermsExcept(permPlataformaGerenciar),
	"supervisor": {
		permConversasVer, permConversasResponder, permConversasTransferir, permConversasEncerrar, permConversasVerIA,
		permContatosGerenciar, permRespostasGerenciar, permMensagensGerenciar,
		permAgentesGerenciar, permAprendizadoVer, permSetoresGerenciar,
		permCampanhasVer, permCampanhasGerenciar, permGruposCanaisGerenciar,
		permRelatoriosVer, permEnviarAssinatura,
	},
	"atendente": {
		permConversasVer, permConversasResponder, permConversasTransferir, permConversasEncerrar, permConversasVerIA,
		permContatosGerenciar, permRespostasGerenciar, permEnviarAssinatura,
	},
}

func allPermsList() []string {
	var out []string
	for _, g := range permCatalog {
		for _, p := range g.Perms {
			out = append(out, p.Key)
		}
	}
	return out
}

func allPermsExcept(excluded ...string) []string {
	ex := map[string]bool{}
	for _, e := range excluded {
		ex[e] = true
	}
	var out []string
	for _, p := range allPermsList() {
		if !ex[p] {
			out = append(out, p)
		}
	}
	return out
}

type permDef struct {
	Key   string
	Label string
}

// permCatalog agrupa as permissões por módulo (usado no cadastro de usuários).
var permCatalog = []struct {
	Module string
	Label  string
	Perms  []permDef
}{
	{"conversas", "Atendimento", []permDef{
		{permConversasVer, "Ver conversas"},
		{permConversasResponder, "Responder mensagens"},
		{permConversasTransferir, "Transferir de setor"},
		{permConversasEncerrar, "Encerrar/reabrir atendimento"},
		{permConversasVerIA, "Ver a fila da IA / conversas em ia"},
		{permEnviarAssinatura, "Enviar mensagens com assinatura (nome do atendente)"},
	}},
	{"contatos", "Contatos", []permDef{{permContatosGerenciar, "Gerenciar contatos"}}},
	{"ia", "Agentes de IA", []permDef{
		{permAgentesGerenciar, "Gerenciar agentes de IA"},
		{permIAsGerenciar, "Gerenciar provedores de IA"},
		{permAprendizadoVer, "Ver aprendizado da IA"},
		{permChatflowsGerenciar, "Gerenciar chat flows"},
	}},
	{"equipe", "Setores e Equipe", []permDef{
		{permSetoresGerenciar, "Gerenciar setores"},
		{permEquipeGerenciar, "Gerenciar equipe e permissões"},
	}},
	{"canais", "Canais", []permDef{{permCanaisGerenciar, "Gerenciar canais"}}},
	{"integracoes", "Integrações", []permDef{{permIntegracoesGerenciar, "Gerenciar integrações"}}},
	{"mensagens", "Mensagens", []permDef{
		{permRespostasGerenciar, "Respostas rápidas"},
		{permMensagensGerenciar, "Mensagens padrão do sistema"},
	}},
	{"campanhas", "Campanhas", []permDef{
		{permCampanhasVer, "Ver campanhas e grupos"},
		{permCampanhasGerenciar, "Criar e operar campanhas (iniciar, pausar, agendar)"},
		{permGruposCanaisGerenciar, "Gerenciar grupos de canais (números do disparo)"},
	}},
	{"agendamentos", "Agendamentos", []permDef{
		{permAgendamentosGerenciar, "Gerenciar agendamentos (criar, editar, disparar e cancelar)"},
	}},
	{"relatorios", "Relatórios", []permDef{
		{permRelatoriosVer, "Ver relatórios"},
	}},
	{"config", "Configurações", []permDef{
		{permConfigGerenciar, "Configurações e módulos"},
		{permLogsVer, "Ver logs do sistema"},
		{permPlataformaGerenciar, "Plataforma (multi-tenant) — apenas super_admin"},
	}},
}

func roleLevel(role string) int {
	switch role {
	case "super_admin":
		return 4
	case "admin":
		return 3
	case "supervisor":
		return 2
	default:
		return 1
	}
}

// userPermSet resolve as permissões efetivas do usuário: a lista salva
// (completa, definida no cadastro) ou, se vazia, o padrão do perfil.
func userPermSet(u *store.Usuario) map[string]bool {
	set := map[string]bool{}
	if u == nil {
		return set
	}
	if strings.TrimSpace(u.Permissions) != "" {
		for _, p := range strings.Split(u.Permissions, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				set[p] = true
			}
		}
		return set
	}
	for _, p := range roleDefaultPerms[u.Role] {
		set[p] = true
	}
	return set
}

func userHasPerm(u *store.Usuario, perm string) bool {
	if u == nil {
		return false
	}
	if u.Role == "super_admin" {
		return true
	}
	return userPermSet(u)[perm]
}

// permEnabled é como userHasPerm mas SEM o atalho do super_admin — usado para
// funcionalidades opt-in (ex.: assinatura) que devem respeitar a permissão real.
func permEnabled(u *store.Usuario, perm string) bool {
	if u == nil {
		return false
	}
	return userPermSet(u)[perm]
}

// requirePerm exige uma permissão granular específica.
func (s *Server) requirePerm(perm string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !userHasPerm(currentUser(r), perm) {
			http.Error(w, "Sem permissão para acessar", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireSuperAdmin protege o control plane. Ter a permissão granular de
// plataforma não permite que um admin de empresa administre outras empresas
// ou a infraestrutura compartilhada. O bypass RLS é aplicado somente depois
// desta checagem e só durante a operação global explícita.
func (s *Server) requireSuperAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := currentUser(r)
		if u == nil || u.Role != "super_admin" {
			http.Error(w, "Acesso restrito ao super_admin", http.StatusForbidden)
			return
		}
		ctx := store.WithGlobalDataAccess(r.Context())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
