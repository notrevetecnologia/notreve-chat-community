package web

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/channel"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func newInstanceToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Server) webhookURLFor(r *http.Request) string {
	scheme := "https"
	forwarded := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]))
	if forwarded == "http" || forwarded == "https" {
		scheme = forwarded
	} else if r.TLS == nil && (strings.HasPrefix(r.Host, "localhost") || strings.HasPrefix(r.Host, "127.0.0.1") || strings.HasPrefix(r.Host, "[::1]")) {
		scheme = "http"
	}
	return scheme + "://" + r.Host + "/notreve/webhook"
}

func (s *Server) handleCanalConectar(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeJSON(w, map[string]interface{}{"error": "id inválido"})
		return
	}
	ctx := r.Context()
	// O primeiro clique ainda está provisionando a instância e o browser pode
	// repetir a requisição. Serializa por canal para não criar nomes duplicados.
	s.channelConnectMu.Lock()
	defer s.channelConnectMu.Unlock()

	c, err := s.st.GetChannelByIDSecure(ctx, id)
	if err != nil || c.Type != "whatsapp" {
		writeJSON(w, map[string]interface{}{"error": "canal não encontrado"})
		return
	}
	tenantID, err := store.TenantIDRequired(ctx)
	if err != nil {
		writeJSON(w, map[string]interface{}{"error": "empresa inválida"})
		return
	}
	credentials, err := s.st.WhatsAppServerAdminCredentialsForChannelSecure(ctx, tenantID, c.ID)
	if err != nil || strings.TrimSpace(credentials.BaseURL) == "" || strings.TrimSpace(credentials.Token) == "" {
		writeJSON(w, map[string]interface{}{"error": "infraestrutura WhatsApp indisponível"})
		return
	}
	base := strings.TrimRight(credentials.BaseURL, "/")
	evts := []string{"Message", "ReadReceipt", "ChatPresence", "CallOffer"}

	if strings.TrimSpace(c.Token) != "" {
		connected, loggedIn, _, _, instanceID, _ := channel.InstanciaStatus(ctx, base, c.Token)
		if instanceID != "" && instanceID != c.InstanceID {
			_ = s.st.SetChannelInstanceID(ctx, c.ID, instanceID)
			c.InstanceID = instanceID
		}
		if connected && loggedIn {
			if err := channel.InstanciaSetWebhook(ctx, base, c.Token, s.webhookURLFor(r), evts); err != nil {
				writeJSON(w, map[string]interface{}{"error": "webhook: " + err.Error()})
				return
			}
			_ = channel.InstanciaSetMediaDelivery(ctx, base, c.Token)
			s.syncReactChannelHealth(ctx, c.ID, "connected", "")
			writeJSON(w, map[string]interface{}{"ok": true, "already": true, "connected": true, "loggedIn": true})
			return
		}
	}

	if strings.TrimSpace(c.Token) == "" {
		instanceToken := newInstanceToken()
		existing, found, findErr := channel.InstanciaEncontrarPorNome(ctx, base, credentials.Token, strings.TrimSpace(c.Name))
		if findErr != nil {
			writeJSON(w, map[string]interface{}{"error": "não foi possível verificar a instância existente: " + findErr.Error()})
			return
		}
		if found {
			if existing.Token == "" {
				writeJSON(w, map[string]interface{}{"error": "já existe uma instância com este nome, mas a API não devolveu o token"})
				return
			}
			c.BaseURL = base
			c.Token = existing.Token
			c.InstanceID = existing.ID
			if err := s.st.UpdateChannelSecure(ctx, c); err != nil {
				writeJSON(w, map[string]interface{}{"error": "instância existente localizada, mas a credencial não pôde ser persistida"})
				return
			}
		} else {
			createdID, err := channel.InstanciaCriarID(ctx, base, credentials.Token, c.Name, instanceToken, s.webhookURLFor(r), evts)
			if err != nil {
				writeJSON(w, map[string]interface{}{"error": "falha ao criar instância: " + err.Error()})
				return
			}
			c.BaseURL = base
			c.Token = instanceToken
			c.InstanceID = createdID
			if err := s.st.UpdateChannelSecure(ctx, c); err != nil {
				writeJSON(w, map[string]interface{}{"error": "instância criada, mas a credencial não pôde ser persistida"})
				return
			}
		}
	}

	if err := channel.InstanciaSetWebhook(ctx, base, c.Token, s.webhookURLFor(r), evts); err != nil {
		writeJSON(w, map[string]interface{}{"error": "webhook: " + err.Error()})
		return
	}
	_ = channel.InstanciaSetMediaDelivery(ctx, base, c.Token)

	if lim, err := s.st.GetLimites(ctx); err == nil && lim.CampanhaLimiteDiario > 0 {
		day := lim.CampanhaLimiteDiario * 10
		if day < 500 {
			day = 500
		}
		_ = channel.InstanciaSetLimits(ctx, base, c.Token, 100, day)
	}

	if err := channel.InstanciaConectar(ctx, base, c.Token, []string{"Message", "ReadReceipt", "ChatPresence", "CallOffer"}, true); err != nil {
		writeJSON(w, map[string]interface{}{"error": "falha ao conectar: " + err.Error()})
		return
	}

	var qr string
	var passkey bool
	for i := 0; i < 8; i++ {
		qr, passkey, err = channel.InstanciaQR(ctx, base, c.Token)
		if err == nil && qr != "" {
			break
		}
		time.Sleep(800 * time.Millisecond)
	}
	writeJSON(w, map[string]interface{}{"qr": qr, "passkey": passkey, "ok": true})
}

func (s *Server) handleCanalStatusAPI(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeJSON(w, map[string]interface{}{"error": "id inválido"})
		return
	}
	ctx := r.Context()
	c, err := s.st.GetChannelByIDSecure(ctx, id)
	if err != nil || c.Type != "whatsapp" || strings.TrimSpace(c.Token) == "" || strings.TrimSpace(c.BaseURL) == "" {
		writeJSON(w, map[string]interface{}{"connected": false, "provisioned": false})
		return
	}
	base := strings.TrimRight(c.BaseURL, "/")
	connected, loggedIn, jid, name, instanceID, statusErr := channel.InstanciaStatus(ctx, base, c.Token)
	if instanceID != "" && instanceID != c.InstanceID {
		_ = s.st.SetChannelInstanceID(ctx, c.ID, instanceID)
	}
	resp := map[string]interface{}{
		"connected": connected, "loggedIn": loggedIn, "jid": jid, "name": name,
		"provisioned": true,
	}
	if statusErr != nil {
		resp["error"] = statusErr.Error()
	}
	if connected && loggedIn && jid != "" {
		phone := jid
		if i := strings.IndexAny(phone, ":@"); i > 0 {
			phone = phone[:i]
		}
		if av, err := channel.InstanciaAvatar(ctx, base, c.Token, phone); err == nil && av != "" {
			resp["avatar"] = av
		}
	}
	writeJSON(w, resp)
}

func (s *Server) handleCanalDesconectar(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeJSON(w, map[string]interface{}{"error": "id inválido"})
		return
	}
	ctx := r.Context()
	c, err := s.st.GetChannelByIDSecure(ctx, id)
	if err != nil || c.Type != "whatsapp" || strings.TrimSpace(c.Token) == "" || strings.TrimSpace(c.BaseURL) == "" {
		writeJSON(w, map[string]interface{}{"error": "canal não encontrado"})
		return
	}
	base := strings.TrimRight(c.BaseURL, "/")
	connected, loggedIn, _, _, _, statusErr := channel.InstanciaStatus(ctx, base, c.Token)
	if statusErr == nil && !connected && !loggedIn {
		s.syncReactChannelHealth(ctx, c.ID, "disconnected", "")
		writeJSON(w, map[string]interface{}{"ok": true, "already": true, "connected": false, "loggedIn": false})
		return
	}
	var serr error
	if r.FormValue("action") == "logout" {
		serr = channel.InstanciaLogout(ctx, base, c.Token)
	} else {
		serr = channel.InstanciaDesconectar(ctx, base, c.Token)
	}
	if serr != nil {
		writeJSON(w, map[string]interface{}{"error": serr.Error()})
		return
	}
	s.syncReactChannelHealth(ctx, c.ID, "disconnected", "")
	writeJSON(w, map[string]interface{}{"ok": true, "connected": false, "loggedIn": false})
}
