package web

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
	"github.com/notrevetecnologia/notreve-chat-community/internal/webhooks"
)

type reactContact struct {
	ID           int               `json:"id"`
	Name         string            `json:"name"`
	Phone        string            `json:"phone"`
	Email        string            `json:"email"`
	Notes        string            `json:"notes"`
	TemWhatsapp  bool              `json:"tem_whatsapp"`
	Tags         []string          `json:"tags"`
	CamposExtras map[string]string `json:"campos_extras"`
	SegmentIDs   []int             `json:"segment_ids"`
	Segments     []string          `json:"segments"`
	CreatedAt    string            `json:"created_at"`
	UpdatedAt    string            `json:"updated_at"`
}

type reactContactInput struct {
	Name         *string            `json:"name"`
	Phone        *string            `json:"phone"`
	Email        *string            `json:"email"`
	Notes        *string            `json:"notes"`
	TemWhatsapp  *bool              `json:"tem_whatsapp"`
	Tags         *[]string          `json:"tags"`
	CamposExtras *map[string]string `json:"campos_extras"`
	SegmentIDs   *[]int             `json:"segment_ids"`
	Action       *string            `json:"action"`
	ContactID    *int               `json:"contact_id"`
}

type reactSegment struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count,omitempty"`
}

func (s *Server) reactContactFromStore(ctx context.Context, contact store.Contato) reactContact {
	ids, _ := s.st.SegmentosOfContatoForTenant(ctx, contact.ID)
	labels := make([]string, 0, len(ids))
	if segmentos, err := s.st.ListSegmentosForTenant(ctx); err == nil {
		byID := make(map[int]string, len(segmentos))
		for _, segmento := range segmentos {
			byID[segmento.ID] = segmento.Name
		}
		for _, id := range ids {
			if name := byID[id]; name != "" {
				labels = append(labels, name)
			}
		}
	}
	return reactContact{
		ID: contact.ID, Name: contact.Name, Phone: contact.Phone, Email: contact.Email,
		Notes: contact.Notes, TemWhatsapp: contact.TemWhatsapp, Tags: parseContactTags(contact.Tags),
		CamposExtras: contactCamposExtras(contact), SegmentIDs: ids,
		Segments: labels, CreatedAt: contact.CreatedAt.Format(time.RFC3339), UpdatedAt: contact.UpdatedAt.Format(time.RFC3339),
	}
}

func parseContactTags(raw string) []string {
	out := make([]string, 0)
	seen := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		tag := strings.TrimSpace(part)
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, tag)
	}
	return out
}

func contactCamposExtras(contact store.Contato) map[string]string {
	if contact.CamposExtras == nil {
		return map[string]string{}
	}
	return contact.CamposExtras
}

const (
	maxContactTags       = 30
	maxContactTagLength  = 40
	maxContactExtraKeys  = 50
	maxContactExtraKey   = 60
	maxContactExtraValue = 500
)

func normalizeContactTags(tags []string) ([]string, error) {
	if len(tags) > maxContactTags {
		return nil, errors.New("máximo de " + strconv.Itoa(maxContactTags) + " tags por contato")
	}
	out := make([]string, 0, len(tags))
	seen := map[string]bool{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if len([]rune(tag)) > maxContactTagLength {
			return nil, errors.New("cada tag deve ter no máximo " + strconv.Itoa(maxContactTagLength) + " caracteres")
		}
		key := strings.ToLower(tag)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, tag)
	}
	return out, nil
}

func normalizeContactCamposExtras(campos map[string]string) (map[string]string, error) {
	if len(campos) > maxContactExtraKeys {
		return nil, errors.New("máximo de " + strconv.Itoa(maxContactExtraKeys) + " campos extras por contato")
	}
	out := make(map[string]string, len(campos))
	for key, value := range campos {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if len([]rune(key)) > maxContactExtraKey {
			return nil, errors.New("cada campo extra deve ter chave com no máximo " + strconv.Itoa(maxContactExtraKey) + " caracteres")
		}
		value = strings.TrimSpace(value)
		if len([]rune(value)) > maxContactExtraValue {
			return nil, errors.New("cada campo extra deve ter valor com no máximo " + strconv.Itoa(maxContactExtraValue) + " caracteres")
		}
		out[key] = value
	}
	return out, nil
}

func decodeReactContactInput(r *http.Request) (reactContactInput, error) {
	var input reactContactInput
	if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024+1)).Decode(&input); err != nil {
		return input, errors.New("JSON inválido")
	}
	return input, nil
}

func validateContactName(value string) error {
	if len([]rune(strings.TrimSpace(value))) > 160 {
		return errors.New("nome deve ter no máximo 160 caracteres")
	}
	return nil
}

func validateContactPhone(value string) error {
	value = strings.TrimSpace(value)
	if value == "" || len([]rune(value)) > 40 {
		return errors.New("telefone é obrigatório e deve ser válido")
	}
	return nil
}

func normalizeSegmentIDs(ids []int) ([]int, error) {
	seen := map[int]bool{}
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		if id < 1 {
			return nil, errors.New("segmento inválido")
		}
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (s *Server) reactContactOptions(ctx context.Context) ([]reactSegment, error) {
	segments, err := s.st.ListSegmentosForTenant(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]reactSegment, 0, len(segments))
	for _, segment := range segments {
		out = append(out, reactSegment{ID: segment.ID, Name: segment.Name, Count: segment.Count})
	}
	return out, nil
}

func (s *Server) handleReactContacts(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	limit, err := parseReactLimit(query.Get("limit"))
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_limit", err.Error())
		return
	}
	offset, err := decodeReactCursor(query.Get("cursor"))
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_cursor", err.Error())
		return
	}
	search := strings.TrimSpace(query.Get("q"))
	segmentID, err := optionalPositiveInt(query.Get("segment_id"))
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_segment", "segmento inválido")
		return
	}
	segmentFilter := 0
	if segmentID != nil {
		segmentFilter = *segmentID
	}
	var temWhatsapp *bool
	if raw := strings.TrimSpace(query.Get("tem_whatsapp")); raw != "" {
		value := parseBoolValue(raw)
		temWhatsapp = &value
	}
	total, err := s.st.CountContatosForTenant(r.Context(), search, segmentFilter, temWhatsapp)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível contar os contatos")
		return
	}
	contacts, err := s.st.ListContatosFilteredForTenant(r.Context(), search, segmentFilter, temWhatsapp, limit, offset)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os contatos")
		return
	}
	items := make([]reactContact, 0, len(contacts))
	for _, contact := range contacts {
		items = append(items, s.reactContactFromStore(r.Context(), contact))
	}
	segments, err := s.reactContactOptions(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os segmentos")
		return
	}
	meta := reactPageMeta(total, limit, offset, reactQueryFilter(map[string]string{
		"q": search, "segment_id": query.Get("segment_id"), "tem_whatsapp": query.Get("tem_whatsapp"),
	}))
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"items": items, "segments": segments}, meta)
}

func (s *Server) handleReactContactCreate(w http.ResponseWriter, r *http.Request) {
	input, err := decodeReactContactInput(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if input.Action != nil && strings.TrimSpace(*input.Action) != "" {
		s.handleReactContactAction(w, r, input)
		return
	}
	if input.Phone == nil || validateContactPhone(*input.Phone) != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_phone", "telefone é obrigatório e deve ser válido")
		return
	}
	name, email, notes := "", "", ""
	if input.Name != nil {
		name = strings.TrimSpace(*input.Name)
	}
	if input.Email != nil {
		email = strings.TrimSpace(*input.Email)
	}
	if input.Notes != nil {
		notes = *input.Notes
	}
	if err := validateContactName(name); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_name", err.Error())
		return
	}
	segmentIDs := []int{}
	if input.SegmentIDs != nil {
		segmentIDs, err = normalizeSegmentIDs(*input.SegmentIDs)
		if err != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_segments", err.Error())
			return
		}
	}
	temWhatsapp := false
	if input.TemWhatsapp != nil {
		temWhatsapp = *input.TemWhatsapp
	}
	tags := []string{}
	if input.Tags != nil {
		tags, err = normalizeContactTags(*input.Tags)
		if err != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_tags", err.Error())
			return
		}
	}
	camposExtras := map[string]string{}
	if input.CamposExtras != nil {
		camposExtras, err = normalizeContactCamposExtras(*input.CamposExtras)
		if err != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_campos_extras", err.Error())
			return
		}
	}
	id, err := s.st.CreateContatoForTenant(r.Context(), store.Contato{
		Name: name, Phone: strings.TrimSpace(*input.Phone), Email: email, Notes: notes,
		TemWhatsapp: temWhatsapp, Tags: strings.Join(tags, ","), CamposExtras: camposExtras,
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeReactError(w, http.StatusConflict, "contact_exists", "já existe um contato com esse telefone nesta empresa")
			return
		}
		writeReactError(w, http.StatusInternalServerError, "create_failed", "não foi possível criar o contato")
		return
	}
	if err := s.st.SetContatoSegmentosForTenant(r.Context(), id, segmentIDs); err != nil {
		writeReactError(w, http.StatusInternalServerError, "segments_failed", "contato criado, mas não foi possível salvar os segmentos")
		return
	}
	created, err := s.st.GetContatoForTenant(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "contato criado, mas não foi possível carregá-lo")
		return
	}
	writeReactResponse(w, http.StatusCreated, s.reactContactFromStore(r.Context(), created), nil)
}

func (s *Server) handleReactContactUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactConversationID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	contact, err := s.st.GetContatoForTenant(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "contato não encontrado")
		return
	}
	input, err := decodeReactContactInput(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if input.Name != nil {
		contact.Name = strings.TrimSpace(*input.Name)
	}
	if input.Phone != nil {
		contact.Phone = strings.TrimSpace(*input.Phone)
	}
	if input.Email != nil {
		contact.Email = strings.TrimSpace(*input.Email)
	}
	if input.Notes != nil {
		contact.Notes = *input.Notes
	}
	if input.TemWhatsapp != nil {
		contact.TemWhatsapp = *input.TemWhatsapp
	}
	if input.Tags != nil {
		tags, validationErr := normalizeContactTags(*input.Tags)
		if validationErr != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_tags", validationErr.Error())
			return
		}
		contact.Tags = strings.Join(tags, ",")
	}
	if input.CamposExtras != nil {
		camposExtras, validationErr := normalizeContactCamposExtras(*input.CamposExtras)
		if validationErr != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_campos_extras", validationErr.Error())
			return
		}
		contact.CamposExtras = camposExtras
	}
	if err := validateContactName(contact.Name); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_name", err.Error())
		return
	}
	if err := validateContactPhone(contact.Phone); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_phone", err.Error())
		return
	}
	if err := s.st.UpdateContatoForTenant(r.Context(), contact); err != nil {
		if isUniqueViolation(err) {
			writeReactError(w, http.StatusConflict, "contact_exists", "já existe um contato com esse telefone nesta empresa")
			return
		}
		writeReactError(w, http.StatusInternalServerError, "update_failed", "não foi possível atualizar o contato")
		return
	}
	if input.SegmentIDs != nil {
		ids, validationErr := normalizeSegmentIDs(*input.SegmentIDs)
		if validationErr != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_segments", validationErr.Error())
			return
		}
		if err := s.st.SetContatoSegmentosForTenant(r.Context(), id, ids); err != nil {
			writeReactError(w, http.StatusInternalServerError, "segments_failed", "não foi possível salvar os segmentos")
			return
		}
	}
	updated, err := s.st.GetContatoForTenant(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "contato atualizado, mas não foi possível carregá-lo")
		return
	}
	writeReactResponse(w, http.StatusOK, s.reactContactFromStore(r.Context(), updated), nil)
}

func (s *Server) handleReactContactDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactConversationID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	if _, err := s.st.GetContatoForTenant(r.Context(), id); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "contato não encontrado")
		return
	}
	if err := s.st.DeleteContatoForTenant(r.Context(), id); err != nil {
		writeReactError(w, http.StatusInternalServerError, "delete_failed", "não foi possível excluir o contato")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"deleted": id}, nil)
}

func (s *Server) handleReactContactAction(w http.ResponseWriter, r *http.Request, input reactContactInput) {
	action := strings.ToLower(strings.TrimSpace(*input.Action))
	switch action {
	case "verify", "verificar":
		phone := ""
		var contact *store.Contato
		if input.ContactID != nil && *input.ContactID > 0 {
			loaded, err := s.st.GetContatoForTenant(r.Context(), *input.ContactID)
			if err != nil {
				status := http.StatusInternalServerError
				if errors.Is(err, pgx.ErrNoRows) {
					status = http.StatusNotFound
				}
				writeReactError(w, status, "not_found", "contato não encontrado")
				return
			}
			contact = &loaded
			phone = loaded.Phone
		}
		if input.Phone != nil && strings.TrimSpace(*input.Phone) != "" {
			phone = strings.TrimSpace(*input.Phone)
		}
		if err := validateContactPhone(phone); err != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_phone", err.Error())
			return
		}
		if s.sender == nil {
			writeReactError(w, http.StatusServiceUnavailable, "whatsapp_unavailable", "WhatsApp não está disponível para verificar agora")
			return
		}
		ok, err := s.sender.CheckWhatsApp(r.Context(), phone)
		if err != nil {
			writeReactError(w, http.StatusBadGateway, "whatsapp_check_failed", "não foi possível verificar o número agora")
			return
		}
		data := map[string]interface{}{"ok": true, "tem_whatsapp": ok, "phone": phone}
		if contact != nil {
			contact.TemWhatsapp = ok
			if err := s.st.UpdateContatoForTenant(r.Context(), *contact); err != nil {
				writeReactError(w, http.StatusInternalServerError, "update_failed", "número verificado, mas não foi possível atualizar o contato")
				return
			}
			data["contact"] = s.reactContactFromStore(r.Context(), *contact)
		}
		writeReactResponse(w, http.StatusOK, data, nil)
	case "sync", "sincronizar":
		if s.sender == nil {
			writeReactError(w, http.StatusServiceUnavailable, "whatsapp_unavailable", "WhatsApp não está disponível para sincronizar agora")
			return
		}
		contacts, err := s.sender.SyncContacts(r.Context())
		if err != nil {
			writeReactError(w, http.StatusBadGateway, "sync_failed", "não foi possível sincronizar os contatos agora")
			return
		}
		imported := 0
		for _, contact := range contacts {
			if strings.TrimSpace(contact.Phone) == "" {
				continue
			}
			if _, err := s.st.UpsertContatoGetIDForTenant(r.Context(), contact); err != nil {
				writeReactError(w, http.StatusInternalServerError, "sync_failed", "a sincronização foi interrompida ao salvar um contato")
				return
			}
			imported++
		}
		writeReactResponse(w, http.StatusOK, map[string]interface{}{"ok": true, "imported": imported, "total": len(contacts)}, nil)
	case "conversation", "conversa":
		if !userHasPerm(currentUser(r), permConversasResponder) || !s.reactModuleEnabled(r.Context(), "conversas") {
			writeReactError(w, http.StatusForbidden, "permission_denied", "sem permissão para iniciar conversas")
			return
		}
		if input.ContactID == nil || *input.ContactID < 1 {
			writeReactError(w, http.StatusBadRequest, "missing_contact", "informe o contato")
			return
		}
		contact, err := s.st.GetContatoForTenant(r.Context(), *input.ContactID)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, pgx.ErrNoRows) {
				status = http.StatusNotFound
			}
			writeReactError(w, status, "not_found", "contato não encontrado")
			return
		}
		conversation, err := s.st.GetOpenConversationByPhone(r.Context(), contact.Phone)
		if errors.Is(err, pgx.ErrNoRows) {
			id, createErr := s.st.CreateConversation(r.Context(), store.Conversation{
				Channel: "whatsapp", CustomerName: contact.Name, CustomerPhone: contact.Phone, Status: "humano_aguardando",
			})
			if createErr != nil {
				writeReactError(w, http.StatusInternalServerError, "conversation_failed", "não foi possível iniciar a conversa")
				return
			}
			conversation, err = s.st.GetConversation(r.Context(), id)
		}
		if err != nil {
			writeReactError(w, http.StatusInternalServerError, "conversation_failed", "não foi possível carregar a conversa")
			return
		}
		if user := currentUser(r); user != nil && conversation.AtendenteID == nil {
			uid := user.ID
			if err := s.st.SetConversationAtendente(r.Context(), conversation.ID, &uid); err == nil {
				_ = s.st.SetConversationStatus(r.Context(), conversation.ID, "humano_atendendo")
				conversation, _ = s.st.GetConversation(r.Context(), conversation.ID)
				s.enqueueEvent(r.Context(), webhooks.EventConversationUpdated, "conversation.updated:"+strconv.Itoa(conversation.ID)+":"+strconv.FormatInt(time.Now().UnixNano(), 10), map[string]interface{}{
					"conversation": store.ConversationWebhookPayload(conversation), "status": conversation.Status,
				})
			}
		}
		writeReactResponse(w, http.StatusOK, map[string]interface{}{"ok": true, "conversation": conversation}, nil)
	default:
		writeReactError(w, http.StatusBadRequest, "invalid_action", "ação de contato inválida")
	}
}

const maxImportCSVRows = 5000

func sniffCSVDelimiter(data []byte) rune {
	var firstLine string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			firstLine = line
			break
		}
	}
	if strings.Count(firstLine, ";") > strings.Count(firstLine, ",") {
		return ';'
	}
	return ','
}

func normalizeCSVHeaderKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "\ufeff")
	aliases := map[string]string{
		"e-mail": "email", "correio": "email",
		"celular": "telefone", "numero": "telefone", "whatsapp": "telefone", "fone": "telefone", "tel": "telefone",
		"observacao": "observacoes", "observa\u00e7\u00e3o": "observacoes", "nota": "observacoes",
	}
	if canonical, ok := aliases[value]; ok {
		return canonical
	}
	return value
}

func pickCSVHeader(header map[string]int, key string) int {
	if idx, ok := header[key]; ok {
		return idx
	}
	return -1
}

func csvColumn(record []string, idx int) string {
	if idx < 0 || idx >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[idx])
}

func parseBoolValue(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "sim", "yes", "s":
		return true
	default:
		return false
	}
}

func isBlankCSVRecord(record []string) bool {
	for _, field := range record {
		if strings.TrimSpace(field) != "" {
			return false
		}
	}
	return true
}

func (s *Server) handleReactContactImport(w http.ResponseWriter, r *http.Request) {
	if _, err := store.TenantIDRequired(r.Context()); err != nil {
		writeReactError(w, http.StatusUnauthorized, "tenant_required", "contexto da empresa não identificado")
		return
	}
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_upload", "arquivo de importação inválido")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_upload", "envie um arquivo CSV no campo file")
		return
	}
	defer file.Close()
	segmentID := 0
	if value := strings.TrimSpace(r.FormValue("segment_id")); value != "" {
		segmentID, err = strconv.Atoi(value)
		if err != nil || segmentID < 1 {
			writeReactError(w, http.StatusBadRequest, "invalid_segment", "segmento de destino inválido")
			return
		}
	}
	data, err := io.ReadAll(io.LimitReader(file, 16<<20))
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_upload", "não foi possível ler o arquivo")
		return
	}
	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = sniffCSVDelimiter(data)
	reader.FieldsPerRecord = -1
	headerRow, err := reader.Read()
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_csv", "CSV vazio ou sem cabeçalho")
		return
	}
	headers := make(map[string]int, len(headerRow))
	for i, raw := range headerRow {
		headers[normalizeCSVHeaderKey(raw)] = i
	}
	phoneIdx := pickCSVHeader(headers, "telefone")
	if phoneIdx < 0 {
		writeReactError(w, http.StatusBadRequest, "invalid_csv", "o cabeçalho do CSV precisa de uma coluna de telefone (phone/telefone)")
		return
	}
	nameIdx := pickCSVHeader(headers, "nome")
	if nameIdx < 0 {
		nameIdx = pickCSVHeader(headers, "name")
	}
	emailIdx := pickCSVHeader(headers, "email")
	notesIdx := pickCSVHeader(headers, "observacoes")
	if notesIdx < 0 {
		notesIdx = pickCSVHeader(headers, "notes")
	}
	temWhatsappIdx := pickCSVHeader(headers, "tem_whatsapp")
	if temWhatsappIdx < 0 {
		temWhatsappIdx = pickCSVHeader(headers, "whatsapp_verificado")
	}

	var records [][]string
	for {
		record, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			continue
		}
		if isBlankCSVRecord(record) {
			continue
		}
		records = append(records, record)
	}
	if len(records) > maxImportCSVRows {
		writeReactError(w, http.StatusBadRequest, "limit_exceeded", "o arquivo excede o limite de "+strconv.Itoa(maxImportCSVRows)+" contatos por importação")
		return
	}

	imported, duplicated, failed, total := 0, 0, 0, len(records)
	seen := map[string]bool{}
	var importedIDs []int
	for _, record := range records {
		phone := store.NormalizePhoneWithDDI(csvColumn(record, phoneIdx))
		if phone == "" || len(phone) < 10 || len(phone) > 13 {
			failed++
			continue
		}
		if seen[phone] {
			duplicated++
			continue
		}
		seen[phone] = true
		name := csvColumn(record, nameIdx)
		if len([]rune(name)) > 160 {
			name = string([]rune(name)[:160])
		}
		temWhatsapp := false
		if temWhatsappIdx >= 0 {
			temWhatsapp = parseBoolValue(csvColumn(record, temWhatsappIdx))
		}
		contact := store.Contato{Name: name, Phone: phone, Email: csvColumn(record, emailIdx), Notes: csvColumn(record, notesIdx), TemWhatsapp: temWhatsapp}
		id, inserted, importErr := s.st.ImportContatoForTenant(r.Context(), contact)
		if importErr != nil {
			failed++
			continue
		}
		if inserted {
			imported++
		} else {
			duplicated++
		}
		importedIDs = append(importedIDs, id)
	}
	if segmentID > 0 {
		for _, id := range importedIDs {
			_ = s.st.AddContatoSegmentoForTenant(r.Context(), id, segmentID)
		}
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{
		"ok": true, "imported": imported, "duplicated": duplicated, "errors": failed, "total": total,
		"segment_id": segmentID,
	}, nil)
}
