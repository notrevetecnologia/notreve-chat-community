package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

type reactLGPDConsent struct {
	ID             int       `json:"id"`
	ContatoID      *int      `json:"contato_id"`
	ConversationID *int      `json:"conversation_id"`
	Scope          string    `json:"scope"`
	BaseLegal      string    `json:"base_legal"`
	Aceito         bool      `json:"aceito"`
	Canal          string    `json:"canal"`
	Token          string    `json:"token"`
	IP             string    `json:"ip"`
	Data           time.Time `json:"data"`
}

type reactLGPDConsentInput struct {
	ContatoID      *int    `json:"contato_id"`
	ConversationID *int    `json:"conversation_id"`
	Scope          *string `json:"scope"`
	BaseLegal      *string `json:"base_legal"`
	Aceito         *bool   `json:"aceito"`
	Canal          *string `json:"canal"`
	Token          *string `json:"token"`
}

type reactLGPDPolitica struct {
	ID             int        `json:"id"`
	Version        int        `json:"version"`
	Tipo           string     `json:"tipo"`
	Texto          string     `json:"texto"`
	URL            string     `json:"url"`
	VigenciaInicio time.Time  `json:"vigencia_inicio"`
	VigenciaFim    *time.Time `json:"vigencia_fim"`
	Active         bool       `json:"active"`
	CreatedAt      time.Time  `json:"created_at"`
}

type reactLGPDPoliticaInput struct {
	Tipo  *string `json:"tipo"`
	Texto *string `json:"texto"`
	URL   *string `json:"url"`
}

func requireReactLGPD(s *Server, w http.ResponseWriter, r *http.Request) bool {
	if _, err := store.TenantIDRequired(r.Context()); err != nil {
		writeReactError(w, http.StatusUnauthorized, "tenant_required", "contexto da empresa não identificado")
		return false
	}
	if !s.reactModuleEnabled(r.Context(), "lgpd") {
		writeReactError(w, http.StatusForbidden, "module_disabled", "módulo desativado")
		return false
	}
	if !userHasPerm(currentUser(r), permConfigGerenciar) {
		writeReactError(w, http.StatusForbidden, "permission_denied", "sem permissão para acessar a LGPD")
		return false
	}
	return true
}

func reactLGPDConsentFromStore(c store.LGPDConsent) reactLGPDConsent {
	return reactLGPDConsent{
		ID: c.ID, ContatoID: c.ContatoID, ConversationID: c.ConversationID, Scope: c.Scope,
		BaseLegal: c.BaseLegal, Aceito: c.Aceito, Canal: c.Canal, Token: c.Token, IP: c.IP, Data: c.Data,
	}
}

func reactLGPDPoliticaFromStore(p store.LGPDPolitica) reactLGPDPolitica {
	return reactLGPDPolitica{
		ID: p.ID, Version: p.Version, Tipo: p.Tipo, Texto: p.Texto, URL: p.URL,
		VigenciaInicio: p.VigenciaInicio, VigenciaFim: p.VigenciaFim, Active: p.Active, CreatedAt: p.CreatedAt,
	}
}

func (s *Server) handleReactLGPDSummary(w http.ResponseWriter, r *http.Request) {
	if !requireReactLGPD(s, w, r) {
		return
	}
	consents, err := s.st.CountConsents(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível contar os consentimentos")
		return
	}
	audits, err := s.st.CountAuditLogs(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível contar a auditoria")
		return
	}
	politica, err := s.st.GetCurrentPolitica(r.Context())
	currentPolitica := reactLGPDPolitica{}
	politicaFound := true
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar a política")
			return
		}
		politicaFound = false
	} else {
		currentPolitica = reactLGPDPoliticaFromStore(politica)
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{
		"consents":     consents,
		"audit":        audits,
		"politica":     currentPolitica,
		"has_politica": politicaFound,
		"encrypted":    s.st.LGPDEncryptionEnabled(),
	}, nil)
}

func (s *Server) handleReactLGPDConsents(w http.ResponseWriter, r *http.Request) {
	if !requireReactLGPD(s, w, r) {
		return
	}
	limit, err := parseReactLimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_limit", err.Error())
		return
	}
	offset, err := decodeReactCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_cursor", err.Error())
		return
	}
	scope := strings.TrimSpace(r.URL.Query().Get("scope"))
	consents, total, err := s.st.ListConsents(r.Context(), scope, limit, offset)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os consentimentos")
		return
	}
	items := make([]reactLGPDConsent, 0, len(consents))
	for _, consent := range consents {
		items = append(items, reactLGPDConsentFromStore(consent))
	}
	meta := reactPageMeta(total, limit, offset, reactQueryFilter(map[string]string{"scope": scope}))
	writeReactResponse(w, http.StatusOK, items, meta)
}

func (s *Server) handleReactLGPDConsentCreate(w http.ResponseWriter, r *http.Request) {
	if !requireReactLGPD(s, w, r) {
		return
	}
	var input reactLGPDConsentInput
	if err := json.NewDecoder(io.LimitReader(r.Body, 16*1024+1)).Decode(&input); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido")
		return
	}
	scope := "contato"
	if input.Scope != nil && strings.TrimSpace(*input.Scope) != "" {
		scope = strings.TrimSpace(*input.Scope)
	}
	if len([]rune(scope)) > 40 {
		writeReactError(w, http.StatusBadRequest, "invalid_scope", "escopo inválido")
		return
	}
	baseLegal := "consentimento"
	if input.BaseLegal != nil && strings.TrimSpace(*input.BaseLegal) != "" {
		baseLegal = strings.TrimSpace(*input.BaseLegal)
	}
	aceito := true
	if input.Aceito != nil {
		aceito = *input.Aceito
	}
	canal := ""
	if input.Canal != nil {
		canal = strings.TrimSpace(*input.Canal)
	}
	token := ""
	if input.Token != nil {
		token = strings.TrimSpace(*input.Token)
	}
	id, err := s.st.RecordConsent(r.Context(), store.LGPDConsent{
		ContatoID: input.ContatoID, ConversationID: input.ConversationID, Scope: scope,
		BaseLegal: baseLegal, Aceito: aceito, Canal: canal, Token: token, IP: clientIP(r),
	})
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "create_failed", "não foi possível registrar o consentimento")
		return
	}
	writeReactResponse(w, http.StatusCreated, map[string]interface{}{"id": id}, nil)
}

func (s *Server) handleReactLGPDPoliticaCurrent(w http.ResponseWriter, r *http.Request) {
	if !requireReactLGPD(s, w, r) {
		return
	}
	politica, err := s.st.GetCurrentPolitica(r.Context())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeReactResponse(w, http.StatusOK, nil, nil)
			return
		}
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar a política")
		return
	}
	writeReactResponse(w, http.StatusOK, reactLGPDPoliticaFromStore(politica), nil)
}

func (s *Server) handleReactLGPDPoliticaList(w http.ResponseWriter, r *http.Request) {
	if !requireReactLGPD(s, w, r) {
		return
	}
	politicas, err := s.st.ListPoliticas(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar o histórico")
		return
	}
	items := make([]reactLGPDPolitica, 0, len(politicas))
	for _, politica := range politicas {
		items = append(items, reactLGPDPoliticaFromStore(politica))
	}
	writeReactResponse(w, http.StatusOK, items, nil)
}

func (s *Server) handleReactLGPDPoliticaCreate(w http.ResponseWriter, r *http.Request) {
	if !requireReactLGPD(s, w, r) {
		return
	}
	var input reactLGPDPoliticaInput
	if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024+1)).Decode(&input); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido")
		return
	}
	tipo := "privacidade"
	if input.Tipo != nil && strings.TrimSpace(*input.Tipo) != "" {
		tipo = strings.TrimSpace(*input.Tipo)
	}
	if tipo != "privacidade" && tipo != "termos" {
		writeReactError(w, http.StatusBadRequest, "invalid_tipo", "tipo deve ser privacidade ou termos")
		return
	}
	texto := ""
	if input.Texto != nil {
		texto = strings.TrimSpace(*input.Texto)
	}
	url := ""
	if input.URL != nil {
		url = strings.TrimSpace(*input.URL)
	}
	if texto == "" && url == "" {
		writeReactError(w, http.StatusBadRequest, "invalid_politica", "informe o texto ou a URL da política")
		return
	}
	politicas, err := s.st.ListPoliticas(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível calcular a versão")
		return
	}
	version := 1
	for _, politica := range politicas {
		if politica.Version >= version {
			version = politica.Version + 1
		}
	}
	id, err := s.st.CreatePolitica(r.Context(), store.LGPDPolitica{
		Version: version, Tipo: tipo, Texto: texto, URL: url, VigenciaInicio: time.Now().UTC(),
	})
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "create_failed", "não foi possível publicar a política")
		return
	}
	created, err := s.st.GetCurrentPolitica(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "política publicada, mas não foi possível recarregá-la")
		return
	}
	if err := s.st.RecordAudit(r.Context(), currentUser(r).ID, "politica_publicada", "lgpd_politica", &id, clientIP(r), map[string]interface{}{"version": version, "tipo": tipo}); err != nil {
		writeReactError(w, http.StatusInternalServerError, "audit_failed", "política publicada, mas não foi possível registrar na auditoria")
		return
	}
	writeReactResponse(w, http.StatusCreated, reactLGPDPoliticaFromStore(created), nil)
}

func (s *Server) handleReactLGPDAudit(w http.ResponseWriter, r *http.Request) {
	if !requireReactLGPD(s, w, r) {
		return
	}
	limit, err := parseReactLimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_limit", err.Error())
		return
	}
	offset, err := decodeReactCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_cursor", err.Error())
		return
	}
	acao := strings.TrimSpace(r.URL.Query().Get("acao"))
	recurso := strings.TrimSpace(r.URL.Query().Get("recurso"))
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	logs, total, err := s.st.ListAudit(r.Context(), acao, recurso, search, limit, offset)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar a auditoria")
		return
	}
	items := make([]store.AuditLog, 0, len(logs))
	for _, entry := range logs {
		items = append(items, entry)
	}
	meta := reactPageMeta(total, limit, offset, reactQueryFilter(map[string]string{"acao": acao, "recurso": recurso, "q": search}))
	writeReactResponse(w, http.StatusOK, items, meta)
}

func (s *Server) handleReactLGPDContactDocumentGet(w http.ResponseWriter, r *http.Request) {
	if !requireReactLGPD(s, w, r) {
		return
	}
	id, err := strconv.Atoi(strings.TrimSpace(r.PathValue("id")))
	if err != nil || id < 1 {
		writeReactError(w, http.StatusBadRequest, "invalid_id", "contato inválido")
		return
	}
	documento, err := s.st.GetContatoDocumento(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeReactError(w, http.StatusNotFound, "not_found", "contato não encontrado")
			return
		}
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível ler o documento")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{
		"contato_id": id, "mascarado": store.MaskLGPDSensitiveDocument(documento), "encrypted": s.st.LGPDEncryptionEnabled(),
	}, nil)
}

func (s *Server) handleReactLGPDContactDocumentSet(w http.ResponseWriter, r *http.Request) {
	if !requireReactLGPD(s, w, r) {
		return
	}
	id, err := strconv.Atoi(strings.TrimSpace(r.PathValue("id")))
	if err != nil || id < 1 {
		writeReactError(w, http.StatusBadRequest, "invalid_id", "contato inválido")
		return
	}
	var input struct {
		Documento string `json:"documento"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4*1024+1)).Decode(&input); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido")
		return
	}
	documento := strings.TrimSpace(input.Documento)
	if documento == "" || len([]rune(documento)) > 40 {
		writeReactError(w, http.StatusBadRequest, "invalid_documento", "documento deve ter entre 1 e 40 caracteres")
		return
	}
	if err := s.st.SetContatoDocumento(r.Context(), id, documento); err != nil {
		writeReactError(w, http.StatusInternalServerError, "save_failed", "não foi possível salvar o documento")
		return
	}
	user := currentUser(r)
	userID := 0
	if user != nil {
		userID = user.ID
	}
	if err := s.st.RecordAudit(r.Context(), userID, "documento_sensivel_atualizado", "contatos", &id, clientIP(r), nil); err != nil {
		writeReactError(w, http.StatusInternalServerError, "audit_failed", "documento salvo, mas não foi possível registrar na auditoria")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{
		"contato_id": id, "mascarado": store.MaskLGPDSensitiveDocument(documento), "encrypted": s.st.LGPDEncryptionEnabled(),
	}, nil)
}
