package web

import (
	"net/http"
	"strings"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
	"golang.org/x/crypto/bcrypt"
)

func (s *Server) HandleLoginPost(w http.ResponseWriter, r *http.Request) {
	if !strictSameOriginRequest(r) {
		s.renderLogin(w, r, "Origem da autenticação não permitida")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderLogin(w, r, "Erro de formulário")
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	ip := clientIP(r)
	if s.loginRateLimited(r.Context(), ip, username) {
		s.renderLogin(w, r, "Muitas tentativas. Aguarde um minuto e tente de novo.")
		return
	}

	tenantID, tenantStatus, tenantErr := s.st.ResolveTenantForHost(r.Context(), r.Host)
	if tenantErr != nil {
		s.renderLogin(w, r, "Não foi possível identificar a empresa deste domínio.")
		return
	}
	if tenantStatus != "active" && tenantStatus != "trial" {
		s.renderLogin(w, r, "Esta empresa está temporariamente indisponível.")
		return
	}
	tenantCtx := store.WithTenantID(r.Context(), tenantID)

	u, err := s.st.GetUsuarioByUsername(tenantCtx, username)
	if err != nil || !u.Active || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		s.recordLoginFailure(r.Context(), ip, username)
		s.renderLogin(w, r, "Usuário ou senha incorretos")
		return
	}

	if u.Role == "atendente" && u.AtendenteID != nil {
		if attendant, attendantErr := s.st.GetAtendente(tenantCtx, *u.AtendenteID); attendantErr == nil {
			if !dentroHorarioAtendente(attendant.HorarioAbertura, attendant.HorarioFechamento, attendant.DiasSemana, nowBR()) {
				s.renderLogin(w, r, "Fora do seu horário de trabalho. Tente novamente dentro do horário permitido.")
				return
			}
		}
	}

	s.clearUserLoginFailures(r.Context(), username)

	state, stateErr := s.st.GetMFAStateByUserID(r.Context(), u.ID)
	if stateErr != nil {
		s.renderLogin(w, r, "Não foi possível validar a autenticação em duas etapas")
		return
	}

	token := newSessionToken()
	mfaVerified := !state.Enabled && u.Role != "super_admin"
	s.sessions.SetIdentity(r.Context(), token, u.ID, u.TenantID, u.Role, mfaVerified)
	setSessionCookie(w, r, token, 0)
	setCSRFCookie(w, r, newSessionToken())

	if state.Enabled {
		http.Redirect(w, r, "/mfa/challenge", http.StatusFound)
		return
	}
	if u.Role == "super_admin" {
		http.Redirect(w, r, "/mfa/setup", http.StatusFound)
		return
	}

	_ = s.st.SetAtendenteOnline(tenantCtx, u.AtendenteID, true)
	_ = s.st.RecordAudit(tenantCtx, u.ID, "login", "autenticacao", nil, ip, nil)
	http.Redirect(w, r, "/", http.StatusFound)
}
