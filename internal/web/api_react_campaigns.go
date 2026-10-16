package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

type reactCampaignItem struct {
	ID                int        `json:"id"`
	Name              string     `json:"name"`
	Message           string     `json:"message"`
	Canal             string     `json:"canal"`
	GrupoID           *int       `json:"grupo_id"`
	GrupoName         string     `json:"grupo_name"`
	ChannelID         *int       `json:"channel_id"`
	ChannelName       string     `json:"channel_name"`
	SegmentoID        *int       `json:"segmento_id"`
	SegmentoName      string     `json:"segmento_name"`
	Status            string     `json:"status"`
	ScheduledAt       *time.Time `json:"scheduled_at"`
	IntervaloSegundos int        `json:"intervalo_segundos"`
	LimiteDiario      int        `json:"limite_diario"`
	VariarMensagem    bool       `json:"variar_mensagem"`
	ModoTeste         bool       `json:"modo_teste"`
	MediaType         string     `json:"media_type"`
	MediaMime         string     `json:"media_mime"`
	MediaName         string     `json:"media_name"`
	HasMedia          bool       `json:"has_media"`
	Total             int        `json:"total"`
	Enviadas          int        `json:"enviadas"`
	Falhas            int        `json:"falhas"`
	Lidas             int        `json:"lidas"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	StartedAt         *time.Time `json:"started_at"`
	CompletedAt       *time.Time `json:"completed_at"`
}

type reactCampaignInput struct {
	Name              string `json:"name"`
	Message           string `json:"message"`
	GrupoID           *int   `json:"grupo_id"`
	ChannelID         *int   `json:"channel_id"`
	SegmentoID        *int   `json:"segmento_id"`
	ScheduledAt       string `json:"scheduled_at"`
	IntervaloSegundos int    `json:"intervalo_segundos"`
	LimiteDiario      int    `json:"limite_diario"`
	VariarMensagem    bool   `json:"variar_mensagem"`
	ModoTeste         bool   `json:"modo_teste"`
	MediaType         string `json:"media_type"`
	MediaB64          string `json:"media_b64"`
	MediaMime         string `json:"media_mime"`
	MediaName         string `json:"media_name"`
}

type reactCampaignStatusInput struct {
	Action  string `json:"action"`
	Confirm bool   `json:"confirm"`
}

type reactSegmentInput struct {
	Name              string  `json:"name"`
	Description       string  `json:"description"`
	FiltroTemWhatsapp *bool   `json:"filtro_tem_whatsapp"`
	FiltroCanal       string  `json:"filtro_canal"`
}

type reactCampaignGroupInput struct {
	Name       string `json:"name"`
	ChannelIDs []int  `json:"channel_ids"`
}

type reactCampaignChannel struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Active bool   `json:"active"`
}

type reactCampaignGroup struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	ChannelIDs   []int    `json:"channel_ids"`
	ChannelNames []string `json:"channel_names"`
}

func reactCampaignGroupFromStore(group store.GrupoCanal) reactCampaignGroup {
	ids := group.ChannelIDs
	if ids == nil {
		ids = []int{}
	}
	names := group.ChannelNames
	if names == nil {
		names = []string{}
	}
	return reactCampaignGroup{ID: group.ID, Name: group.Name, ChannelIDs: ids, ChannelNames: names}
}

func reactCampaignFromStore(c store.Campanha) reactCampaignItem {
	return reactCampaignItem{
		ID: c.ID, Name: c.Name, Message: c.Message, Canal: c.Canal,
		GrupoID: c.GrupoID, GrupoName: c.GrupoName, ChannelID: c.ChannelID, ChannelName: c.ChannelName,
		SegmentoID: c.SegmentoID, SegmentoName: c.SegmentoName, Status: c.Status, ScheduledAt: c.ScheduledAt,
		IntervaloSegundos: c.IntervaloSegundos, LimiteDiario: c.LimiteDiario, VariarMensagem: c.VariarMensagem,
		ModoTeste: c.ModoTeste, MediaType: c.MediaType, MediaMime: c.MediaMime, MediaName: c.MediaName,
		HasMedia: c.MediaB64 != "", Total: c.Total, Enviadas: c.Enviadas, Falhas: c.Falhas, Lidas: c.Lidas,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt, StartedAt: c.StartedAt, CompletedAt: c.CompletedAt,
	}
}

func parseReactCampaignID(r *http.Request) (int, error) {
	id, err := strconv.Atoi(strings.TrimSpace(r.PathValue("id")))
	if err != nil || id < 1 {
		return 0, errors.New("id inválido")
	}
	return id, nil
}

func parseReactCampaignTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		loc = time.Local
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04"} {
		var parsed time.Time
		if layout == time.RFC3339 {
			parsed, err = time.Parse(layout, value)
		} else {
			parsed, err = time.ParseInLocation(layout, value, loc)
		}
		if err == nil {
			return &parsed, nil
		}
	}
	return nil, errors.New("scheduled_at inválido")
}

func validateReactCampaignInput(input reactCampaignInput) error {
	if strings.TrimSpace(input.Name) == "" || len([]rune(input.Name)) > 120 {
		return errors.New("nome deve ter entre 1 e 120 caracteres")
	}
	if strings.TrimSpace(input.Message) == "" || len([]rune(input.Message)) > 20000 {
		return errors.New("mensagem deve ter entre 1 e 20000 caracteres")
	}
	if input.GrupoID != nil && *input.GrupoID < 1 {
		return errors.New("grupo inválido")
	}
	if input.ChannelID != nil && *input.ChannelID < 1 {
		return errors.New("canal inválido")
	}
	if input.GrupoID == nil && input.ChannelID == nil {
		return errors.New("selecione um canal ou grupo de canais")
	}
	if input.GrupoID != nil && input.ChannelID != nil {
		return errors.New("selecione canal ou grupo, não os dois")
	}
	if input.SegmentoID != nil && *input.SegmentoID < 1 {
		return errors.New("segmento inválido")
	}
	if input.MediaB64 != "" {
		if len(input.MediaB64) > 24<<20 {
			return errors.New("a mídia deve ter no máximo 16 MB")
		}
		if _, err := base64.StdEncoding.DecodeString(input.MediaB64); err != nil {
			return errors.New("mídia inválida")
		}
		if strings.TrimSpace(input.MediaName) == "" || strings.TrimSpace(input.MediaMime) == "" {
			return errors.New("os dados da mídia estão incompletos")
		}
		if strings.TrimSpace(input.MediaType) == "" {
			input.MediaType = mediaTypeFromName(input.MediaName, input.MediaMime)
		}
	}
	return nil
}

func (s *Server) reactCampaignOptions(ctx context.Context) (map[string]interface{}, error) {
	segments, err := s.st.ListSegmentos(ctx)
	if err != nil {
		return nil, err
	}
	groups, err := s.st.ListGruposCanais(ctx)
	if err != nil {
		return nil, err
	}
	channels, err := s.st.ListChannels(ctx)
	if err != nil {
		return nil, err
	}
	channelItems := make([]reactCampaignChannel, 0, len(channels))
	for _, channel := range channels {
		if channel.Type != "whatsapp" || !channel.Active || !channel.CanCampaign {
			continue
		}
		channelItems = append(channelItems, reactCampaignChannel{ID: channel.ID, Name: channel.Name, Type: channel.Type, Active: channel.Active})
	}
	groupItems := make([]reactCampaignGroup, 0, len(groups))
	for _, group := range groups {
		groupItems = append(groupItems, reactCampaignGroupFromStore(group))
	}
	return map[string]interface{}{
		"segments": segments,
		"groups":   groupItems,
		"channels": channelItems,
	}, nil
}

func campaignPeriodStart(period string) (time.Time, bool) {
	switch strings.ToLower(strings.TrimSpace(period)) {
	case "7", "7d", "semana", "week":
		return time.Now().AddDate(0, 0, -7), true
	case "30", "30d", "mes", "month":
		return time.Now().AddDate(0, 0, -30), true
	case "90", "90d":
		return time.Now().AddDate(0, 0, -90), true
	default:
		return time.Time{}, false
	}
}

func (s *Server) handleReactCampaigns(w http.ResponseWriter, r *http.Request) {
	page, err := parseReactPage(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	items, err := s.st.ListCampanhas(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar as campanhas")
		return
	}
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	period := strings.TrimSpace(r.URL.Query().Get("period"))
	periodStart, hasPeriod := campaignPeriodStart(period)
	filtered := make([]reactCampaignItem, 0, len(items))
	for _, item := range items {
		if status != "" && item.Status != status {
			continue
		}
		if hasPeriod && item.CreatedAt.Before(periodStart) {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(item.Name), query) && !strings.Contains(strings.ToLower(item.Message), query) && !strings.Contains(strings.ToLower(item.SegmentoName), query) {
			continue
		}
		filtered = append(filtered, reactCampaignFromStore(item))
	}
	pageItems, err := reactPageSlice(filtered, page.Limit, page.Offset)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_cursor", err.Error())
		return
	}
	options, err := s.reactCampaignOptions(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar as opções de campanha")
		return
	}
	data := map[string]interface{}{"items": pageItems, "options": options}
	meta := reactPageMeta(len(filtered), page.Limit, page.Offset, reactQueryFilter(map[string]string{"q": query, "status": status, "period": period}))
	writeReactResponse(w, http.StatusOK, data, meta)
}

func (s *Server) handleReactCampaignCreate(w http.ResponseWriter, r *http.Request) {
	var input reactCampaignInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<20)).Decode(&input); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido")
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Message = strings.TrimSpace(input.Message)
	input.MediaType = strings.TrimSpace(input.MediaType)
	input.MediaMime = strings.TrimSpace(input.MediaMime)
	input.MediaName = strings.TrimSpace(input.MediaName)
	if err := validateReactCampaignInput(input); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_campaign", err.Error())
		return
	}
	if input.MediaB64 != "" && input.MediaType == "" {
		input.MediaType = mediaTypeFromName(input.MediaName, input.MediaMime)
	}
	scheduled, err := parseReactCampaignTime(input.ScheduledAt)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_schedule", err.Error())
		return
	}
	if scheduled != nil && scheduled.Before(time.Now()) {
		writeReactError(w, http.StatusBadRequest, "invalid_schedule", "agende a campanha para um horário futuro")
		return
	}
	if input.ChannelID != nil {
		channel, channelErr := s.st.GetChannelByID(r.Context(), *input.ChannelID)
		if channelErr != nil || channel.Type != "whatsapp" || !channel.Active || !channel.CanCampaign {
			writeReactError(w, http.StatusBadRequest, "invalid_channel", "o canal selecionado não está disponível para campanhas")
			return
		}
	}
	if input.GrupoID != nil {
		ids, groupErr := s.st.GrupoChannelIDs(r.Context(), *input.GrupoID)
		if groupErr != nil || len(ids) == 0 {
			writeReactError(w, http.StatusBadRequest, "invalid_group", "o grupo selecionado não possui canais")
			return
		}
		validGroup := false
		for _, channelID := range ids {
			channel, channelErr := s.st.GetChannelByID(r.Context(), channelID)
			if channelErr == nil && channel.Type == "whatsapp" && channel.Active && channel.CanCampaign {
				validGroup = true
				break
			}
		}
		if !validGroup {
			writeReactError(w, http.StatusBadRequest, "invalid_group", "o grupo não possui canal WhatsApp disponível para campanhas")
			return
		}
	}
	segmentID := 0
	if input.SegmentoID != nil {
		segmentID = *input.SegmentoID
	}
	contacts, err := s.st.ContatosPorSegmento(r.Context(), segmentID)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "recipients_failed", "não foi possível carregar os contatos do público")
		return
	}
	validContacts := make([]store.Contato, 0, len(contacts))
	for _, contact := range contacts {
		if isDigits(strings.TrimSpace(contact.Phone)) && len(strings.TrimSpace(contact.Phone)) >= 11 {
			validContacts = append(validContacts, contact)
		}
	}
	if len(validContacts) == 0 {
		writeReactError(w, http.StatusBadRequest, "empty_audience", "o público selecionado não possui contatos com telefone válido")
		return
	}
	interval := input.IntervaloSegundos
	if interval < 5 {
		interval = 15
	}
	dailyLimit := input.LimiteDiario
	if dailyLimit < 0 {
		dailyLimit = 50
	}
	if dailyLimit == 0 {
		if limits, limitsErr := s.st.GetLimites(r.Context()); limitsErr == nil {
			dailyLimit = limits.CampanhaLimiteDiario
		}
	}
	status := "rascunho"
	if scheduled != nil {
		status = "agendada"
		if limits, limitsErr := s.st.GetLimites(r.Context()); limitsErr == nil {
			if usage, usageErr := s.st.CountCampanhasAtivas(r.Context()); usageErr == nil && !s.limiteCheck(r.Context(), currentUser(r), usage, limits.MaxCampanhasAtivas) {
				writeReactError(w, http.StatusConflict, "campaign_limit", "o limite de campanhas ativas foi atingido")
				return
			}
		}
	}
	var createdBy *int
	if user := currentUser(r); user != nil {
		createdBy = &user.ID
	}
	id, err := s.st.CreateCampanha(r.Context(), store.Campanha{
		Name: input.Name, Message: input.Message, Canal: "whatsapp", GrupoID: input.GrupoID, ChannelID: input.ChannelID,
		SegmentoID: input.SegmentoID, Status: status, ScheduledAt: scheduled, IntervaloSegundos: interval,
		LimiteDiario: dailyLimit, VariarMensagem: input.VariarMensagem, ModoTeste: input.ModoTeste,
		MediaType: input.MediaType, MediaB64: input.MediaB64, MediaMime: input.MediaMime, MediaName: input.MediaName, CriadoPor: createdBy,
	}, validContacts)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "create_failed", "não foi possível criar a campanha")
		return
	}
	created, err := s.st.GetCampanha(r.Context(), id)
	if err != nil {
		writeReactResponse(w, http.StatusCreated, map[string]interface{}{"id": id}, nil)
		return
	}
	writeReactResponse(w, http.StatusCreated, reactCampaignFromStore(created), nil)
}

func (s *Server) handleReactCampaignStatus(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactCampaignID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	var input reactCampaignStatusInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido")
		return
	}
	action := strings.ToLower(strings.TrimSpace(input.Action))
	current, err := s.st.GetCampanha(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeReactError(w, http.StatusNotFound, "not_found", "campanha não encontrada")
			return
		}
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar a campanha")
		return
	}
	if action != "iniciar" && action != "retomar" && action != "pausar" && action != "cancelar" {
		writeReactError(w, http.StatusBadRequest, "invalid_action", "ação de campanha inválida")
		return
	}
	if err := campaignTransitionAllowed(current.Status, action); err != nil {
		writeReactError(w, http.StatusConflict, "invalid_status", err.Error())
		return
	}
	switch action {
	case "iniciar", "retomar":
		if !input.Confirm {
			writeReactError(w, http.StatusBadRequest, "confirmation_required", "confirme explicitamente o início da campanha")
			return
		}
		if !s.campaignHasAvailableChannel(r.Context(), current) {
			writeReactError(w, http.StatusConflict, "no_channel", "nenhum canal WhatsApp ativo e liberado para campanha está disponível")
			return
		}
		if limits, limitsErr := s.st.GetLimites(r.Context()); limitsErr == nil {
			if usage, usageErr := s.st.CountCampanhasAtivas(r.Context()); usageErr == nil && !s.limiteCheck(r.Context(), currentUser(r), usage, limits.MaxCampanhasAtivas) {
				writeReactError(w, http.StatusConflict, "campaign_limit", "o limite de campanhas ativas foi atingido")
				return
			}
		}
		if err := s.st.SetCampanhaStarted(r.Context(), id); err != nil {
			writeReactError(w, http.StatusInternalServerError, "status_failed", "não foi possível iniciar a campanha")
			return
		}
		if err := s.st.UpdateCampanhaStatus(r.Context(), id, "rodando"); err != nil {
			writeReactError(w, http.StatusInternalServerError, "status_failed", "não foi possível atualizar o status")
			return
		}
	case "pausar":
		if err := s.st.UpdateCampanhaStatus(r.Context(), id, "pausada"); err != nil {
			writeReactError(w, http.StatusInternalServerError, "status_failed", "não foi possível pausar a campanha")
			return
		}
	case "cancelar":
		if !input.Confirm {
			writeReactError(w, http.StatusBadRequest, "confirmation_required", "confirme explicitamente o cancelamento da campanha")
			return
		}
		if err := s.st.UpdateCampanhaStatus(r.Context(), id, "cancelada"); err != nil {
			writeReactError(w, http.StatusInternalServerError, "status_failed", "não foi possível cancelar a campanha")
			return
		}
	default:
		writeReactError(w, http.StatusBadRequest, "invalid_action", "ação de campanha inválida")
		return
	}
	updated, err := s.st.GetCampanha(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar o novo status")
		return
	}
	writeReactResponse(w, http.StatusOK, reactCampaignFromStore(updated), nil)
}

func (s *Server) campaignHasAvailableChannel(ctx context.Context, camp store.Campanha) bool {
	eligible := func(channel store.Channel) bool {
		return channel.Type == "whatsapp" && channel.Active && channel.CanCampaign
	}
	if camp.GrupoID != nil {
		ids, err := s.st.GrupoChannelIDs(ctx, *camp.GrupoID)
		if err != nil {
			return false
		}
		for _, id := range ids {
			if channel, channelErr := s.st.GetChannelByID(ctx, id); channelErr == nil && eligible(channel) {
				return true
			}
		}
		return false
	}
	if camp.ChannelID != nil {
		if channel, err := s.st.GetChannelByID(ctx, *camp.ChannelID); err == nil && eligible(channel) {
			return true
		}
	}
	return false
}

func (s *Server) handleReactCampaignDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactCampaignID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	var input struct {
		Confirm bool `json:"confirm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || !input.Confirm {
		writeReactError(w, http.StatusBadRequest, "confirmation_required", "confirme explicitamente a exclusão da campanha")
		return
	}
	current, err := s.st.GetCampanha(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeReactError(w, http.StatusNotFound, "not_found", "campanha não encontrada")
			return
		}
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar a campanha")
		return
	}
	if err := campaignDeleteAllowed(current.Status); err != nil {
		writeReactError(w, http.StatusConflict, "invalid_status", err.Error())
		return
	}
	if err := s.st.DeleteCampanha(r.Context(), id); err != nil {
		writeReactError(w, http.StatusInternalServerError, "delete_failed", "não foi possível excluir a campanha")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"deleted": true, "id": id}, nil)
}

func (s *Server) handleReactCampaignEnvios(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactCampaignID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	if _, err := s.st.GetCampanha(r.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeReactError(w, http.StatusNotFound, "not_found", "campanha não encontrada")
			return
		}
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar a campanha")
		return
	}
	page, err := parseReactPage(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	items, total, err := s.st.ListCampanhaEnviosPage(r.Context(), id, status, page.Limit, page.Offset)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os envios")
		return
	}
	meta := reactPageMeta(total, page.Limit, page.Offset, reactQueryFilter(map[string]string{"status": status}))
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"items": items}, meta)
}

func (s *Server) handleReactSegments(w http.ResponseWriter, r *http.Request) {
	page, err := parseReactPage(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	segments, err := s.st.ListSegmentos(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os segmentos")
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	filtered := make([]store.Segmento, 0, len(segments))
	for _, segment := range segments {
		if !reactContainsFold([]string{segment.Name, segment.Description, segment.FilterCanal}, query) {
			continue
		}
		filtered = append(filtered, segment)
	}
	pageItems, err := reactPageSlice(filtered, page.Limit, page.Offset)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_cursor", err.Error())
		return
	}
	meta := reactPageMeta(len(filtered), page.Limit, page.Offset, reactQueryFilter(map[string]string{"q": query}))
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"items": pageItems}, meta)
}

func (s *Server) reactCampaignGroupOptions(ctx context.Context) (map[string]interface{}, error) {
	groups, err := s.st.ListGruposCanais(ctx)
	if err != nil {
		return nil, err
	}
	channels, err := s.st.ListChannels(ctx)
	if err != nil {
		return nil, err
	}
	channelItems := make([]reactCampaignChannel, 0, len(channels))
	for _, channel := range channels {
		if channel.Type == "whatsapp" {
			channelItems = append(channelItems, reactCampaignChannel{ID: channel.ID, Name: channel.Name, Type: channel.Type, Active: channel.Active})
		}
	}
	groupItems := make([]reactCampaignGroup, 0, len(groups))
	for _, group := range groups {
		groupItems = append(groupItems, reactCampaignGroupFromStore(group))
	}
	return map[string]interface{}{"items": groupItems, "channels": channelItems}, nil
}

func (s *Server) handleReactCampaignGroups(w http.ResponseWriter, r *http.Request) {
	page, err := parseReactPage(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	data, err := s.reactCampaignGroupOptions(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os grupos de canais")
		return
	}
	groups, _ := data["items"].([]reactCampaignGroup)
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	filtered := make([]reactCampaignGroup, 0, len(groups))
	for _, group := range groups {
		if !reactContainsFold(append([]string{group.Name}, group.ChannelNames...), query) {
			continue
		}
		filtered = append(filtered, group)
	}
	pageItems, err := reactPageSlice(filtered, page.Limit, page.Offset)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_cursor", err.Error())
		return
	}
	data["items"] = pageItems
	meta := reactPageMeta(len(filtered), page.Limit, page.Offset, reactQueryFilter(map[string]string{"q": query}))
	writeReactResponse(w, http.StatusOK, data, meta)
}

func (s *Server) validateReactCampaignGroup(ctx context.Context, input reactCampaignGroupInput) ([]int, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len([]rune(input.Name)) > 120 {
		return nil, errors.New("nome deve ter entre 1 e 120 caracteres")
	}
	if len(input.ChannelIDs) == 0 {
		return nil, errors.New("selecione pelo menos um canal WhatsApp")
	}
	seen := map[int]bool{}
	ids := make([]int, 0, len(input.ChannelIDs))
	for _, id := range input.ChannelIDs {
		if id < 1 || seen[id] {
			continue
		}
		channel, err := s.st.GetChannelByID(ctx, id)
		if err != nil || channel.Type != "whatsapp" {
			return nil, errors.New("canal WhatsApp inválido")
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, errors.New("selecione pelo menos um canal WhatsApp")
	}
	return ids, nil
}

func (s *Server) handleReactCampaignGroupCreate(w http.ResponseWriter, r *http.Request) {
	var input reactCampaignGroupInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&input); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido")
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	ids, err := s.validateReactCampaignGroup(r.Context(), input)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_group", err.Error())
		return
	}
	id, err := s.st.CreateGrupoCanais(r.Context(), input.Name, ids)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "create_failed", "não foi possível criar o grupo")
		return
	}
	writeReactResponse(w, http.StatusCreated, map[string]interface{}{"id": id, "name": input.Name, "channel_ids": ids}, nil)
}

func (s *Server) handleReactCampaignGroupUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactCampaignID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	var input reactCampaignGroupInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&input); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido")
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	ids, err := s.validateReactCampaignGroup(r.Context(), input)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_group", err.Error())
		return
	}
	if err := s.st.UpdateGrupoCanais(r.Context(), id, input.Name, ids); err != nil {
		writeReactError(w, http.StatusInternalServerError, "update_failed", "não foi possível atualizar o grupo")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"id": id, "name": input.Name, "channel_ids": ids}, nil)
}

func (s *Server) handleReactCampaignGroupDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactCampaignID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	var input struct {
		Confirm bool `json:"confirm"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8*1024)).Decode(&input); err != nil || !input.Confirm {
		writeReactError(w, http.StatusBadRequest, "confirmation_required", "confirme explicitamente a exclusão do grupo")
		return
	}
	if err := s.st.DeleteGrupoCanais(r.Context(), id); err != nil {
		writeReactError(w, http.StatusInternalServerError, "delete_failed", "não foi possível excluir o grupo")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"deleted": true, "id": id}, nil)
}

func (s *Server) handleReactSegmentCreate(w http.ResponseWriter, r *http.Request) {
	var input reactSegmentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido")
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.FiltroCanal = strings.TrimSpace(input.FiltroCanal)
	if input.Name == "" || len([]rune(input.Name)) > 100 {
		writeReactError(w, http.StatusBadRequest, "invalid_segment", "nome deve ter entre 1 e 100 caracteres")
		return
	}
	if len([]rune(input.Description)) > 500 {
		writeReactError(w, http.StatusBadRequest, "invalid_segment", "descrição deve ter no máximo 500 caracteres")
		return
	}
	if input.FiltroCanal != "" {
		switch input.FiltroCanal {
		case "whatsapp", "webchat", "instagram", "telegram":
		default:
			writeReactError(w, http.StatusBadRequest, "invalid_segment", "filtro de canal inválido")
			return
		}
	}
	id, err := s.st.CreateSegmentoComFiltros(r.Context(), input.Name, input.Description, input.FiltroTemWhatsapp, input.FiltroCanal)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "create_failed", "não foi possível criar o segmento")
		return
	}
	segments, err := s.st.ListSegmentos(r.Context())
	if err == nil {
		for _, segment := range segments {
			if segment.ID == id {
				writeReactResponse(w, http.StatusCreated, segment, nil)
				return
			}
		}
	}
	writeReactResponse(w, http.StatusCreated, map[string]interface{}{"id": id, "name": input.Name, "description": input.Description, "filter_tem_whatsapp": input.FiltroTemWhatsapp, "filter_canal": input.FiltroCanal}, nil)
}

func (s *Server) handleReactSegmentDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactCampaignID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	var input struct {
		Confirm bool `json:"confirm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || !input.Confirm {
		writeReactError(w, http.StatusBadRequest, "confirmation_required", "confirme explicitamente a exclusão do segmento")
		return
	}
	if err := s.st.DeleteSegmento(r.Context(), id); err != nil {
		writeReactError(w, http.StatusInternalServerError, "delete_failed", "não foi possível excluir o segmento")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"deleted": true, "id": id}, nil)
}
