package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/notrevetecnologia/notreve-chat-community/internal/agent"
	"github.com/notrevetecnologia/notreve-chat-community/internal/applog"
	"github.com/notrevetecnologia/notreve-chat-community/internal/channel"
	"github.com/notrevetecnologia/notreve-chat-community/internal/config"
	"github.com/notrevetecnologia/notreve-chat-community/internal/db"
	"github.com/notrevetecnologia/notreve-chat-community/internal/infra"
	"github.com/notrevetecnologia/notreve-chat-community/internal/mcp"
	"github.com/notrevetecnologia/notreve-chat-community/internal/securesecret"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
	"github.com/notrevetecnologia/notreve-chat-community/internal/web"
	"github.com/notrevetecnologia/notreve-chat-community/internal/webhookoutbox"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	cfg := config.Load()
	if err := securesecret.ValidateKey(); err != nil {
		log.Fatalf("proteção de segredos: %v", err)
	}
	whatsappHMACKey := strings.TrimSpace(os.Getenv("WHATSAPP_HMAC_KEY"))
	if whatsappHMACKey == "" {
		whatsappHMACKey = strings.TrimSpace(os.Getenv("NOTREVE_HMAC_KEY"))
	}
	if whatsappHMACKey == "" {
		log.Fatal("WHATSAPP_HMAC_KEY é obrigatório")
	}

	ctx := context.Background()

	// O processo web usa exclusivamente a role runtime. Migrations, preflight
	// e criação/endurecimento dessa role são responsabilidade do job migrate,
	// que termina antes do app iniciar.
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("banco de dados runtime: %v", err)
	}
	defer pool.Close()

	inf := infra.Connect(ctx, cfg.RedisURL, cfg.RabbitURL)
	defer inf.Close()

	st := store.New(pool)
	applog.SetDefault(applog.New(st))
	eng := agent.NewEngine(st)
	proc := channel.NewProcessor(st, eng, inf)

	ensureAdmin(store.WithTenantID(ctx, 1), st, cfg.AdminPass)

	resetMFAUser := strings.TrimSpace(os.Getenv("AGENTE_IA_RESET_MFA"))
	if resetMFAUser != "" {
		if err := st.ResetMFAForUsername(ctx, resetMFAUser); err != nil {
			log.Printf("reset de MFA para %q: %v", resetMFAUser, err)
		} else {
			log.Printf("MFA desativado para o usuário %q (AGENTE_IA_RESET_MFA)", resetMFAUser)
		}
	}

	webroot := os.Getenv("AGENTE_IA_WEBROOT")
	if webroot == "" {
		webroot = filepath.Join(".", "web")
	}

	whatsapp := channel.NewNotreve(proc, st)
	wc := channel.NewWebChat(proc, st)
	tg := channel.NewTelegram(proc, st)

	srv, err := web.NewServer(st, eng, cfg.AdminPass, webroot, whatsapp, inf, channel.NewRegistry())
	if err != nil {
		log.Fatalf("servidor: %v", err)
	}
	eng.SetMediaSender(srv)
	srv.Start(ctx)

	wbWorker, err := webhookoutbox.NewWorker(webhookoutbox.Config{
		Store:          st,
		LeaseDuration:  2 * time.Minute,
		RequestTimeout: 15 * time.Second,
		InitialBackoff: 2 * time.Second,
		MaxBackoff:     30 * time.Second,
	})
	if err != nil {
		log.Fatalf("worker de webhooks: %v", err)
	}
	workerCtx, cancelWorker := context.WithCancel(ctx)
	defer cancelWorker()
	go runWebhookOutboxWorker(workerCtx, wbWorker, st)

	chanMon := channel.NewMonitor(st)
	monitorCtx, cancelMonitor := context.WithCancel(ctx)
	defer cancelMonitor()
	go runChannelMonitor(monitorCtx, chanMon, st)

	logCtx, cancelLogs := context.WithCancel(ctx)
	defer cancelLogs()
	go runLogRetention(logCtx, st)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		readyCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(readyCtx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"unavailable","database":"down"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"ready","database":"ok"}`))
	})
	mux.Handle("/notreve/webhook", requireWebhookHMAC(whatsappHMACKey, whatsapp))
	mux.Handle("/telegram/webhook/{token}", tg)
	mux.Handle("GET /webchat/{key}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wc.ServeWidget(w, r, r.PathValue("key"))
	}))
	mux.Handle("GET /webchat/{key}/embed.js", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wc.ServeEmbed(w, r, r.PathValue("key"))
	}))
	mux.Handle("POST /webchat/{key}/message", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wc.HandleMessage(w, r, r.PathValue("key"))
	}))
	mux.Handle("GET /webchat/{key}/messages", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wc.HandleMessages(w, r, r.PathValue("key"))
	}))
	mux.Handle("POST /webchat/{key}/end", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wc.HandleEnd(w, r, r.PathValue("key"))
	}))
	mux.Handle("GET /webchat/{key}/config", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wc.HandleConfig(w, r, r.PathValue("key"))
	}))
	mux.Handle("POST /login", http.HandlerFunc(srv.HandleLoginPost))
	mux.Handle("/", srv.SaaSGuard(srv.Routes()))

	httpServer := &http.Server{Addr: cfg.HTTPAddr, Handler: globalSecurityHeaders(mux)}
	shutdownCtx, stopSignals := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	go func() {
		<-shutdownCtx.Done()
		_ = mcp.Shutdown()
		graceful, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(graceful)
	}()

	log.Printf("Notreve Chat Community rodando em http://%s", cfg.HTTPAddr)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func requireWebhookHMAC(key string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const maxBody = 25 << 20
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
		if err != nil || len(body) > maxBody {
			http.Error(w, "payload inválido", http.StatusBadRequest)
			return
		}
		signature := strings.TrimSpace(r.Header.Get("x-hmac-signature"))
		// A assinatura da Notreve WhatsApp API é opcional. Quando a API não
		// envia o cabeçalho, o webhook deve continuar sendo processado; exigir
		// HMAC aqui fazia todas as mensagens sem assinatura morrerem em 401.
		if signature == "" || strings.TrimSpace(key) == "" {
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
			return
		}
		mac := hmac.New(sha256.New, []byte(key))
		_, _ = mac.Write(body)
		expected := mac.Sum(nil)
		valid := false
		if decoded, err := hex.DecodeString(strings.ToLower(signature)); err == nil {
			valid = hmac.Equal(decoded, expected)
		}
		if !valid {
			if decoded, err := base64.StdEncoding.DecodeString(signature); err == nil {
				valid = hmac.Equal(decoded, expected)
			}
		}
		if !valid {
			http.Error(w, "assinatura inválida", http.StatusUnauthorized)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}

func globalSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if r.TLS != nil || strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/") && strings.TrimSpace(r.Header.Get("X-API-Key")) != "" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"code":"unsupported_auth_header","message":"use Authorization: Bearer <chave>"}}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func runWebhookOutboxWorker(ctx context.Context, wb *webhookoutbox.Worker, st *store.Store) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tenantIDs, err := st.ListActiveTenantIDs(ctx)
			if err != nil {
				log.Printf("worker de webhooks: listando empresas: %v", err)
				continue
			}
			for _, tenantID := range tenantIDs {
				tenantCtx := store.WithTenantID(ctx, tenantID)
				if _, err := wb.RunOnce(tenantCtx); err != nil {
					log.Printf("worker de webhooks [tenant %d]: %v", tenantID, err)
				}
			}
		}
	}
}

func runChannelMonitor(ctx context.Context, mon *channel.Monitor, st *store.Store) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tenantIDs, err := st.ListActiveTenantIDs(ctx)
			if err != nil {
				log.Printf("monitor de canais: listando empresas: %v", err)
				continue
			}
			for _, tenantID := range tenantIDs {
				tenantCtx := store.WithTenantID(ctx, tenantID)
				if _, err := mon.Run(tenantCtx); err != nil {
					log.Printf("monitor de canais [tenant %d]: %v", tenantID, err)
				}
			}
		}
	}
}

func runLogRetention(ctx context.Context, st *store.Store) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tenantIDs, err := st.ListActiveTenantIDs(ctx)
			if err != nil {
				log.Printf("retenção de logs: listando empresas: %v", err)
				continue
			}
			for _, tenantID := range tenantIDs {
				tenantCtx := store.WithTenantID(ctx, tenantID)
				settings, err := st.GetSettings(tenantCtx)
				if err != nil {
					continue
				}
				days := settings.LogsRetencaoDias
				if days < 1 {
					days = 30
				}
				removed, err := st.DeleteOldLogEntriesForTenant(tenantCtx, time.Now().AddDate(0, 0, -days))
				if err != nil {
					log.Printf("retenção de logs [tenant %d]: %v", tenantID, err)
					continue
				}
				if removed > 0 {
					log.Printf("retenção de logs [tenant %d]: %d entradas removidas", tenantID, removed)
				}
			}
		}
	}
}

func ensureAdmin(ctx context.Context, st *store.Store, pass string) {
	if strings.TrimSpace(pass) == "" {
		log.Printf("usuário admin inicial não criado: defina AGENTE_IA_PASS antes do primeiro start")
		return
	}
	n, err := st.CountUsuarios(ctx)
	if err != nil {
		log.Printf("verificando usuários: %v", err)
		return
	}
	if n > 0 {
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("gerando senha admin: %v", err)
		return
	}
	if _, err := st.CreateUsuario(ctx, store.Usuario{
		Name: "Administrador da plataforma", Username: "super_admin", PasswordHash: string(hash), Role: "super_admin", Active: true,
	}); err != nil {
		log.Printf("criando admin: %v", err)
		return
	}
	log.Printf("Super administrador criado (usuario: super_admin, senha: configurada em AGENTE_IA_PASS)")
}
