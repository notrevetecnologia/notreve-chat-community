package web

import (
	"encoding/base64"
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/authn"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
	"github.com/skip2/go-qrcode"
)

func totpQRDataURI(uri string) string {
	if strings.TrimSpace(uri) == "" {
		return ""
	}
	png, err := qrcode.Encode(uri, qrcode.Medium, 256)
	if err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
}

var mfaPageTemplate = template.Must(template.New("mfa").Parse(`<!doctype html>
<html lang="pt-BR">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}} · {{.BrandName}}</title>
  {{if .FaviconURL}}<link rel="icon" href="{{.FaviconURL}}">{{end}}
  <link rel="stylesheet" href="/static/mfa.css?v=20260915-2">
  <style>
    :root {
      --brand-primary: {{.PrimaryColor}};
      --teal-hover: {{.SecondaryColor}};
      --mint: {{.AccentColor}};
      --paper: {{.BackgroundColor}};
    }
  </style>
</head>
<body class="auth-page">
  <section class="auth-card">
    <div class="auth-brand">
      {{if .LogoURL}}<img src="{{.LogoURL}}" alt="">{{else}}<span class="auth-brand-mark">{{.BrandInitial}}</span>{{end}}
      <strong>{{.BrandName}}</strong>
    </div>
    <div class="auth-heading">
      <h1>{{.Title}}</h1>
      {{if .Secret}}<p>Escaneie o QR Code ou digite a chave no seu app autenticador e informe o código gerado.</p>{{else}}<p>Abra seu app autenticador e informe o código atual para continuar.</p>{{end}}
    </div>

    {{if .Message}}<div class="auth-error" role="alert">{{.Message}}</div>{{end}}

    {{if .Secret}}
    <div class="auth-qr">{{if .QR}}<img src="{{.QR}}" alt="QR Code para configurar o autenticador">{{end}}</div>
    <div class="auth-secret"><span>Chave de configuração</span><code>{{.Secret}}</code></div>
    <p class="auth-hint">No celular: abra o autenticador, aponte para o QR Code ou digite a chave e informe o código gerado.</p>
    {{end}}

    <form class="auth-form" method="post" action="/mfa/confirm">
      <label for="mfa-code">Código de 6 dígitos</label>
      <input id="mfa-code" name="code" inputmode="numeric" autocomplete="one-time-code" pattern="[0-9]{6}" maxlength="6" minlength="6" placeholder="000000" required autofocus>
      <button class="auth-submit" type="submit">{{if .Secret}}Ativar proteção{{else}}Continuar{{end}}</button>
    </form>

    <p class="auth-footnote"><a class="auth-link" href="/logout">Sair da conta</a> · Sua segurança vem primeiro.</p>
  </section>
</body>
</body>
</html>`))

type mfaPageData struct {
	Title           string
	Message         string
	Secret          string
	URI             string
	QR              string
	BrandName       string
	BrandInitial    string
	LogoURL         string
	FaviconURL      string
	PrimaryColor    string
	SecondaryColor  string
	AccentColor     string
	BackgroundColor string
}

func (s *Server) mfaPageData(r *http.Request, title, message, secret, uri string) mfaPageData {
	branding := s.brandingForRequest(r)
	initial := "N"
	if trimmed := strings.TrimSpace(branding.Name); trimmed != "" {
		initial = strings.ToUpper(string([]rune(trimmed)[0]))
	}
	return mfaPageData{
		Title:           title,
		Message:         message,
		Secret:          secret,
		URI:             uri,
		QR:              totpQRDataURI(uri),
		BrandName:       branding.Name,
		BrandInitial:    initial,
		LogoURL:         branding.LogoURL,
		FaviconURL:      branding.FaviconURL,
		PrimaryColor:    branding.PrimaryColor,
		SecondaryColor:  branding.SecondaryColor,
		AccentColor:     branding.AccentColor,
		BackgroundColor: branding.BackgroundColor,
	}
}

func renderMFAPage(w http.ResponseWriter, data mfaPageData) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := mfaPageTemplate.Execute(w, data); err != nil {
		http.Error(w, "não foi possível renderizar a autenticação em duas etapas", http.StatusInternalServerError)
	}
}

func (s *Server) mfaSession(r *http.Request) (sessionIdentity, string, bool) {
	if s.sessions == nil {
		return sessionIdentity{}, "", false
	}
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return sessionIdentity{}, "", false
	}
	identity, ok := s.sessions.Identity(r.Context(), cookie.Value)
	if !ok {
		return sessionIdentity{}, "", false
	}
	return identity, cookie.Value, true
}

func (s *Server) ensureMFAState(r *http.Request, identity sessionIdentity) (store.MFAState, error) {
	state, err := s.st.GetMFAStateByUserID(r.Context(), identity.UserID)
	if err != nil {
		return store.MFAState{}, err
	}
	if state.Enabled || strings.TrimSpace(state.Secret) != "" {
		return state, nil
	}
	secret, err := authn.GenerateTOTPSecret()
	if err != nil {
		return store.MFAState{}, err
	}
	if err := s.st.SetPendingMFASecret(r.Context(), identity.UserID, secret); err != nil {
		return store.MFAState{}, err
	}
	state.Secret = secret
	return state, nil
}

func (s *Server) handleMFASetupPage(w http.ResponseWriter, r *http.Request) {
	identity, _, ok := s.mfaSession(r)
	if !ok {
		http.Redirect(w, r, "/auth", http.StatusFound)
		return
	}
	state, err := s.ensureMFAState(r, identity)
	if err != nil {
		http.Error(w, "não foi possível preparar a autenticação em duas etapas", http.StatusServiceUnavailable)
		return
	}
	if state.Enabled {
		http.Redirect(w, r, "/mfa/challenge", http.StatusFound)
		return
	}
	u, err := s.st.GetUserByIDGlobal(r.Context(), identity.UserID)
	if err != nil {
		http.Error(w, "usuário não encontrado", http.StatusUnauthorized)
		return
	}
	renderMFAPage(w, s.mfaPageData(r, "Ative a autenticação em duas etapas", "", state.Secret, authn.TOTPURI("Notreve Chat Community", u.Username, state.Secret)))
}

func (s *Server) handleMFAChallengePage(w http.ResponseWriter, r *http.Request) {
	identity, _, ok := s.mfaSession(r)
	if !ok {
		http.Redirect(w, r, "/auth", http.StatusFound)
		return
	}
	state, err := s.st.GetMFAStateByUserID(r.Context(), identity.UserID)
	if err != nil {
		http.Error(w, "não foi possível validar a autenticação em duas etapas", http.StatusServiceUnavailable)
		return
	}
	if !state.Enabled {
		http.Redirect(w, r, "/mfa/setup", http.StatusFound)
		return
	}
	renderMFAPage(w, s.mfaPageData(r, "Confirme sua autenticação em duas etapas", "", "", ""))
}

func (s *Server) handleMFASetupAPI(w http.ResponseWriter, r *http.Request) {
	identity, _, ok := s.mfaSession(r)
	if !ok {
		writeReactError(w, http.StatusUnauthorized, "mfa_session_required", "sessão de autenticação inválida")
		return
	}
	state, err := s.ensureMFAState(r, identity)
	if err != nil {
		writeReactError(w, http.StatusServiceUnavailable, "mfa_setup_failed", "não foi possível preparar o MFA")
		return
	}
	if state.Enabled {
		writeReactResponse(w, http.StatusOK, map[string]interface{}{"enabled": true}, nil)
		return
	}
	u, err := s.st.GetUserByIDGlobal(r.Context(), identity.UserID)
	if err != nil {
		writeReactError(w, http.StatusUnauthorized, "user_not_found", "usuário não encontrado")
		return
	}
	uri := authn.TOTPURI("Notreve Chat Community", u.Username, state.Secret)
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"enabled": false, "secret": state.Secret, "uri": uri, "qr": totpQRDataURI(uri)}, nil)
}

func mfaCodeFromRequest(r *http.Request) string {
	if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		var input struct {
			Code string `json:"code"`
		}
		if json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&input) == nil {
			return strings.TrimSpace(input.Code)
		}
		return ""
	}
	if r.ParseForm() != nil {
		return ""
	}
	return strings.TrimSpace(r.FormValue("code"))
}

func (s *Server) handleMFAConfirm(w http.ResponseWriter, r *http.Request) {
	identity, oldToken, ok := s.mfaSession(r)
	if !ok {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeReactError(w, http.StatusUnauthorized, "mfa_session_required", "sessão de autenticação inválida")
		} else {
			http.Redirect(w, r, "/auth", http.StatusFound)
		}
		return
	}
	state, err := s.st.GetMFAStateByUserID(r.Context(), identity.UserID)
	if err != nil || strings.TrimSpace(state.Secret) == "" {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeReactError(w, http.StatusServiceUnavailable, "mfa_unavailable", "MFA indisponível")
		} else {
			http.Error(w, "MFA indisponível", http.StatusServiceUnavailable)
		}
		return
	}
	if !authn.VerifyTOTP(state.Secret, mfaCodeFromRequest(r), time.Now()) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeReactError(w, http.StatusUnauthorized, "invalid_mfa_code", "código inválido")
		} else {
			page := s.mfaPageData(r, "Confirme sua autenticação em duas etapas", "Código inválido. Tente novamente.", "", "")
			if !state.Enabled {
				if u, userErr := s.st.GetUserByIDGlobal(r.Context(), identity.UserID); userErr == nil {
					page = s.mfaPageData(r, "Ative a autenticação em duas etapas", "Código inválido. Tente novamente.", state.Secret, authn.TOTPURI("Notreve Chat Community", u.Username, state.Secret))
				}
			}
			renderMFAPage(w, page)
		}
		return
	}
	if !state.Enabled {
		if err := s.st.EnableMFA(r.Context(), identity.UserID); err != nil {
			writeReactError(w, http.StatusServiceUnavailable, "mfa_enable_failed", "não foi possível ativar o MFA")
			return
		}
	}
	s.sessions.Delete(r.Context(), oldToken)
	newToken := newSessionToken()
	s.sessions.SetIdentity(r.Context(), newToken, identity.UserID, identity.TenantID, identity.Role, true)
	setSessionCookie(w, r, newToken, 0)
	setCSRFCookie(w, r, newSessionToken())
	tenantCtx := store.WithTenantID(r.Context(), identity.TenantID)
	_ = s.st.RecordAudit(tenantCtx, identity.UserID, "login_mfa", "autenticacao", nil, clientIP(r), nil)
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeReactResponse(w, http.StatusOK, map[string]bool{"verified": true}, nil)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) handleMFAStatus(w http.ResponseWriter, r *http.Request) {
	identity, _, ok := s.mfaSession(r)
	if !ok {
		writeReactError(w, http.StatusUnauthorized, "mfa_session_required", "sessão de autenticação inválida")
		return
	}
	state, err := s.st.GetMFAStateByUserID(r.Context(), identity.UserID)
	if err != nil {
		writeReactError(w, http.StatusServiceUnavailable, "mfa_unavailable", "não foi possível consultar a autenticação em duas etapas")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{
		"enabled":  state.Enabled,
		"required": identity.Role == "super_admin",
	}, nil)
}

func (s *Server) handleMFADisable(w http.ResponseWriter, r *http.Request) {
	identity, oldToken, ok := s.mfaSession(r)
	if !ok {
		writeReactError(w, http.StatusUnauthorized, "mfa_session_required", "sessão de autenticação inválida")
		return
	}
	if identity.Role == "super_admin" {
		writeReactError(w, http.StatusForbidden, "mfa_required", "o super_admin não pode desativar a autenticação em duas etapas")
		return
	}
	state, err := s.st.GetMFAStateByUserID(r.Context(), identity.UserID)
	if err != nil || !state.Enabled || strings.TrimSpace(state.Secret) == "" {
		writeReactError(w, http.StatusConflict, "mfa_not_enabled", "a autenticação em duas etapas não está ativa")
		return
	}
	if !authn.VerifyTOTP(state.Secret, mfaCodeFromRequest(r), time.Now()) {
		writeReactError(w, http.StatusUnauthorized, "invalid_mfa_code", "código inválido")
		return
	}
	if err := s.st.DisableMFA(r.Context(), identity.UserID); err != nil {
		writeReactError(w, http.StatusServiceUnavailable, "mfa_disable_failed", "não foi possível desativar o MFA")
		return
	}
	s.sessions.Delete(r.Context(), oldToken)
	newToken := newSessionToken()
	s.sessions.SetIdentity(r.Context(), newToken, identity.UserID, identity.TenantID, identity.Role, true)
	setSessionCookie(w, r, newToken, 0)
	setCSRFCookie(w, r, newSessionToken())
	tenantCtx := store.WithTenantID(r.Context(), identity.TenantID)
	_ = s.st.RecordAudit(tenantCtx, identity.UserID, "mfa_disable", "autenticacao", nil, clientIP(r), nil)
	writeReactResponse(w, http.StatusOK, map[string]bool{"enabled": false}, nil)
}
