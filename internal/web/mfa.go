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

func totpQRDataURI(uri string) template.URL {
	if strings.TrimSpace(uri) == "" {
		return template.URL("")
	}
	png, err := qrcode.Encode(uri, qrcode.Medium, 256)
	if err != nil {
		return template.URL("")
	}
	return template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(png))
}

var mfaPageTemplate = template.Must(template.New("mfa").Parse(`<!doctype html>
<html lang="pt-BR">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}} · {{.BrandName}}</title>
  {{if .FaviconURL}}<link rel="icon" href="{{.FaviconURL}}">{{end}}
  <link rel="stylesheet" href="/static/mfa.css?v=20261114-1">
  <style>
    :root {
      --brand-primary: {{.PrimaryColor}};
      --teal-hover: {{.SecondaryColor}};
      --mint: {{.AccentColor}};
      --paper: {{.BackgroundColor}};
    }
    * { box-sizing: border-box; }
    html, body { min-height: 100%; margin: 0; }
    .auth-page {
      display: grid;
      min-height: 100vh;
      place-items: center;
      padding: 24px;
      color: #071c1c;
      background: var(--paper, #f5faf8);
      font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      -webkit-font-smoothing: antialiased;
    }
    .auth-card {
      width: min(100%, 440px);
      padding: 32px;
      border: 1px solid #e3ebe9;
      border-radius: 18px;
      background: #fff;
      box-shadow: 0 18px 50px rgba(8, 43, 41, 0.08);
    }
    .auth-brand { display: flex; align-items: center; gap: 11px; margin-bottom: 26px; color: #071c1c; }
    .auth-brand img { width: 38px; height: 38px; object-fit: contain; border: 1px solid #e3ebe9; border-radius: 11px; background: #fff; }
    .auth-brand-mark { display: grid; width: 38px; height: 38px; flex: 0 0 auto; place-items: center; border-radius: 11px; background: var(--brand-primary, #12948e); box-shadow: inset 0 -5px 0 rgba(0, 0, 0, 0.12); color: #fff; font-size: 17px; font-weight: 800; }
    .auth-brand strong { max-width: 280px; overflow: hidden; font-size: 15px; font-weight: 700; text-overflow: ellipsis; white-space: nowrap; }
    .auth-heading { margin-bottom: 6px; }
    .auth-heading h1 { margin: 0 0 6px; color: #071c1c; font-size: 24px; font-weight: 800; }
    .auth-heading p { margin: 0; color: #5d706e; font-size: 13px; line-height: 1.55; }
    .auth-form { display: grid; gap: 6px; margin-top: 22px; }
    .auth-form label { margin-top: 10px; color: #5d706e; font-size: 12px; font-weight: 700; }
    .auth-form input { width: 100%; min-height: 46px; padding: 12px 14px; border: 1px solid #e3ebe9; border-radius: 10px; color: #071c1c; background: #fff; font: inherit; font-size: 16px; letter-spacing: 0.16em; text-align: center; outline: none; }
    .auth-form input:focus, .auth-form input:focus-visible { border-color: var(--brand-primary, #12948e); box-shadow: 0 0 0 4px rgba(18, 148, 142, 0.15); }
    .auth-error { margin: 14px 0 0; padding: 10px 12px; border: 1px solid #f2b4b4; border-radius: 10px; color: #a52c2c; background: #fff5f5; font-size: 12px; line-height: 1.45; }
    .auth-submit { display: flex; align-items: center; justify-content: center; min-height: 46px; margin-top: 16px; padding: 12px 16px; border: 0; border-radius: 10px; color: #fff; background: var(--brand-primary, #12948e); box-shadow: 0 8px 20px rgba(18, 148, 142, 0.22); cursor: pointer; font: inherit; font-size: 14px; font-weight: 700; }
    .auth-submit:hover { background: var(--teal-hover, #0e7c77); }
    .auth-footnote { margin: 20px 0 0; color: #8aa09d; font-size: 12px; text-align: center; }
    .auth-link { color: var(--brand-primary, #12948e); font-weight: 700; text-decoration: none; }
    .auth-link:hover { text-decoration: underline; }
    .auth-qr { display: flex; justify-content: center; margin: 22px 0 14px; }
    .auth-qr img { width: 200px; height: 200px; padding: 10px; border: 1px solid #e3ebe9; border-radius: 16px; background: #fff; }
    .auth-secret { display: flex; flex-direction: column; gap: 6px; padding: 12px 14px; border: 1px dashed #e3ebe9; border-radius: 12px; background: #f8fbfa; }
    .auth-secret span { color: #5d706e; font-size: 12px; }
    .auth-secret code { color: #071c1c; font-size: 15px; font-weight: 700; letter-spacing: 0.06em; word-break: break-all; }
    .auth-hint { margin: 12px 0 0; color: #5d706e; font-size: 12px; line-height: 1.55; }
    @media (max-width: 520px) { .auth-card { padding: 26px 20px; } }
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
</html>`))

type mfaPageData struct {
	Title           string
	Message         string
	Secret          string
	URI             string
	QR              template.URL
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
