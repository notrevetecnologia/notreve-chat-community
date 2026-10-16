package web

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

const maxLogSearchChars = 200

func requireReactLogsAccess(s *Server, w http.ResponseWriter, r *http.Request) bool {
	if _, err := store.TenantIDRequired(r.Context()); err != nil {
		writeReactError(w, http.StatusUnauthorized, "tenant_required", "contexto da empresa não identificado")
		return false
	}
	if !s.reactModuleEnabled(r.Context(), "config") {
		writeReactError(w, http.StatusForbidden, "module_disabled", "módulo desativado")
		return false
	}
	if !userHasPerm(currentUser(r), permLogsVer) {
		writeReactError(w, http.StatusForbidden, "permission_denied", "sem permissão para ver os logs")
		return false
	}
	return true
}

func parseReactLogTime(raw string, endOfDay bool) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return &parsed, nil
	}
	if parsed, err := time.Parse("2006-01-02", raw); err == nil {
		if endOfDay {
			value := parsed.Add(24*time.Hour - time.Nanosecond)
			return &value, nil
		}
		return &parsed, nil
	}
	return nil, errors.New("data inválida; use AAAA-MM-DD ou RFC3339")
}

func reactLogFilter(r *http.Request) (store.LogFilter, error) {
	filter := store.LogFilter{
		Level:  strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("level"))),
		Module: strings.TrimSpace(r.URL.Query().Get("module")),
		Search: strings.TrimSpace(r.URL.Query().Get("q")),
	}
	if len([]rune(filter.Search)) > maxLogSearchChars {
		return filter, errors.New("a busca deve ter no máximo 200 caracteres")
	}
	from, err := parseReactLogTime(r.URL.Query().Get("from"), false)
	if err != nil {
		return filter, err
	}
	to, err := parseReactLogTime(r.URL.Query().Get("to"), true)
	if err != nil {
		return filter, err
	}
	filter.From, filter.To = from, to
	return filter, nil
}

func (s *Server) handleReactLogs(w http.ResponseWriter, r *http.Request) {
	if !requireReactLogsAccess(s, w, r) {
		return
	}
	page, err := parseReactPage(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	filter, err := reactLogFilter(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_filter", err.Error())
		return
	}
	items, err := s.st.ListLogEntriesForTenant(r.Context(), filter, page.Limit, page.Offset)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os logs")
		return
	}
	total, err := s.st.CountLogEntriesForTenant(r.Context(), filter)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível contar os logs")
		return
	}
	if items == nil {
		items = []store.LogEntry{}
	}
	meta := reactPageMeta(total, page.Limit, page.Offset, reactQueryFilter(map[string]string{
		"level": filter.Level, "module": filter.Module, "q": filter.Search,
		"from": r.URL.Query().Get("from"), "to": r.URL.Query().Get("to"),
	}))
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"items": items}, meta)
}

func (s *Server) handleReactLogsStats(w http.ResponseWriter, r *http.Request) {
	if !requireReactLogsAccess(s, w, r) {
		return
	}
	stats, err := s.st.LogStatsForTenant(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar as estatísticas de logs")
		return
	}
	writeReactResponse(w, http.StatusOK, stats, nil)
}
