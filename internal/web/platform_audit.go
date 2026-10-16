package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

type auditResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *auditResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *auditResponseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(body)
}

func (w *auditResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func platformTargetTenantID(path string) *int {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] != "tenants" {
			continue
		}
		id, err := strconv.Atoi(parts[i+1])
		if err == nil && id > 0 {
			return &id
		}
	}
	return nil
}

func shouldAuditPlatformMutation(r *http.Request) bool {
	return unsafeHTTPMethod(r.Method) && (r.URL.Path == "/api/react/platform" || strings.HasPrefix(r.URL.Path, "/api/react/platform/"))
}

func (s *Server) auditPlatformMutation(r *http.Request, status int) {
	if s.sessions == nil {
		return
	}
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return
	}
	uid, ok := s.sessions.Get(r.Context(), cookie.Value)
	if !ok {
		return
	}
	user, err := s.st.GetUserByIDGlobal(r.Context(), uid)
	if err != nil || user.Role != "super_admin" {
		return
	}
	actorID := user.ID
	_ = s.st.RecordPlatformAudit(r.Context(), store.PlatformAuditEntry{
		ActorID:        &actorID,
		ActorName:      user.Name,
		Action:         r.Method,
		Path:           r.URL.Path,
		TargetTenantID: platformTargetTenantID(r.URL.Path),
		HTTPStatus:     status,
		IP:             clientIP(r),
	})
}
