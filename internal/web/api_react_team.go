package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

type reactTeamSector struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	HorarioAbertura   string `json:"horario_abertura"`
	HorarioFechamento string `json:"horario_fechamento"`
	DiasSemana        string `json:"dias_semana"`
	Cor               string `json:"cor"`
	Active            bool   `json:"active"`
	MemberCount       int    `json:"member_count"`
	OnlineCount       int    `json:"online_count"`
	CreatedAt         string `json:"created_at"`
}

type reactTeamMember struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Username   string `json:"username,omitempty"`
	Role       string `json:"role,omitempty"`
	Active     bool   `json:"active"`
	Online     bool   `json:"online"`
	SetorIDs   []int  `json:"setor_ids"`
	Capacidade int    `json:"capacidade"`
	CreatedAt  string `json:"created_at"`
}

type reactTeamData struct {
	Sectors []reactTeamSector `json:"sectors"`
	Members []reactTeamMember `json:"members"`
	Stats   map[string]int    `json:"stats"`
}

type reactSectorInput struct {
	Name              *string `json:"name"`
	Description       *string `json:"description"`
	HorarioAbertura   *string `json:"horario_abertura"`
	HorarioFechamento *string `json:"horario_fechamento"`
	DiasSemana        *string `json:"dias_semana"`
	Cor               *string `json:"cor"`
	Active            *bool   `json:"active"`
}

func reactTeamAccessAllowed(s *Server, r *http.Request, manageSectors bool) bool {
	if !s.reactModuleEnabled(r.Context(), "equipe") {
		return false
	}
	user := currentUser(r)
	if manageSectors {
		return userHasPerm(user, permSetoresGerenciar)
	}
	return userHasPerm(user, permSetoresGerenciar) || userHasPerm(user, permEquipeGerenciar)
}

func requireReactTeamAccess(s *Server, w http.ResponseWriter, r *http.Request, manageSectors bool) bool {
	if _, err := store.TenantIDRequired(r.Context()); err != nil {
		writeReactError(w, http.StatusUnauthorized, "tenant_required", "contexto da empresa não identificado")
		return false
	}
	if !s.reactModuleEnabled(r.Context(), "equipe") {
		writeReactError(w, http.StatusForbidden, "module_disabled", "módulo desativado")
		return false
	}
	if !reactTeamAccessAllowed(s, r, manageSectors) {
		writeReactError(w, http.StatusForbidden, "permission_denied", "sem permissão para acessar")
		return false
	}
	return true
}

func reactSectorInScope(actor *store.Usuario, ownSectors map[int]bool, sectorID int) bool {
	if actor == nil {
		return false
	}
	if actor.Role != "supervisor" {
		return true
	}
	return ownSectors[sectorID]
}

func (s *Server) reactSupervisorSectors(ctx context.Context, actor *store.Usuario) ([]int, map[int]bool) {
	if actor == nil || actor.Role != "supervisor" {
		return nil, map[int]bool{}
	}
	ids := s.userSetores(ctx, actor)
	scope := make(map[int]bool, len(ids))
	for _, id := range ids {
		scope[id] = true
	}
	return ids, scope
}

func (s *Server) requireReactSectorScope(w http.ResponseWriter, r *http.Request, sectorID int) bool {
	actor := currentUser(r)
	_, scope := s.reactSupervisorSectors(r.Context(), actor)
	if reactSectorInScope(actor, scope, sectorID) {
		return true
	}
	writeReactError(w, http.StatusForbidden, "sector_forbidden", "você só pode gerenciar setores dos quais participa")
	return false
}

func (s *Server) linkReactSupervisorSector(ctx context.Context, actor *store.Usuario, sectorID int) {
	if actor == nil || actor.Role != "supervisor" || actor.AtendenteID == nil {
		return
	}
	attendant, err := s.st.GetAtendente(ctx, *actor.AtendenteID)
	if err != nil {
		Logger.Printf("setor: supervisor %d não pôde ser vinculado ao setor %d: %v", actor.ID, sectorID, err)
		return
	}
	if containsInt(attendant.SetorIDs, sectorID) {
		return
	}
	attendant.SetorIDs = append(attendant.SetorIDs, sectorID)
	if err := s.st.UpdateAtendente(ctx, attendant); err != nil {
		Logger.Printf("setor: vínculo do supervisor %d ao setor %d falhou: %v", actor.ID, sectorID, err)
	}
}

func isReactSectorUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func reactTeamSectorFromStore(sector store.Setor, memberCount, onlineCount int) reactTeamSector {
	return reactTeamSector{
		ID: sector.ID, Name: sector.Name, Description: sector.Description,
		HorarioAbertura: sector.HorarioAbertura, HorarioFechamento: sector.HorarioFechamento,
		DiasSemana: sector.DiasSemana, Cor: sector.Cor, Active: sector.Active,
		MemberCount: memberCount, OnlineCount: onlineCount, CreatedAt: sector.CreatedAt.Format(time.RFC3339),
	}
}

func reactTeamMemberFromStore(member store.Atendente) reactTeamMember {
	return reactTeamMember{
		ID: member.ID, Name: member.Name, Username: member.Username, Role: member.Role,
		Active: member.Active, Online: member.Online, SetorIDs: append([]int{}, member.SetorIDs...),
		Capacidade: member.Capacidade, CreatedAt: member.CreatedAt.Format(time.RFC3339),
	}
}

func (s *Server) loadReactTeamData(r *http.Request) (reactTeamData, error) {
	sectors, err := s.st.ListSetores(r.Context())
	if err != nil {
		return reactTeamData{}, err
	}
	members, err := s.st.ListAtendentes(r.Context())
	if err != nil {
		return reactTeamData{}, err
	}
	online, err := s.st.SetoresOnline(r.Context())
	if err != nil {
		return reactTeamData{}, err
	}
	memberCounts := make(map[int]int, len(sectors))
	for _, member := range members {
		for _, sectorID := range member.SetorIDs {
			memberCounts[sectorID]++
		}
	}
	actor := currentUser(r)
	ownSectors, sectorScope := s.reactSupervisorSectors(r.Context(), actor)
	isSupervisor := actor != nil && actor.Role == "supervisor"
	data := reactTeamData{
		Sectors: make([]reactTeamSector, 0, len(sectors)),
		Members: make([]reactTeamMember, 0, len(members)),
		Stats: map[string]int{
			"sectors":        len(sectors),
			"active_sectors": 0,
			"members":        len(members),
			"online_members": 0,
		},
	}
	for _, sector := range sectors {
		if isSupervisor && !sectorScope[sector.ID] {
			continue
		}
		if sector.Active {
			data.Stats["active_sectors"]++
		}
		data.Sectors = append(data.Sectors, reactTeamSectorFromStore(sector, memberCounts[sector.ID], online[sector.ID]))
	}
	for _, member := range members {
		if isSupervisor && !hasSectorIntersection(member.SetorIDs, ownSectors) {
			continue
		}
		if member.Online {
			data.Stats["online_members"]++
		}
		data.Members = append(data.Members, reactTeamMemberFromStore(member))
	}
	return data, nil
}

func (s *Server) handleReactTeam(w http.ResponseWriter, r *http.Request) {
	if !requireReactTeamAccess(s, w, r, false) {
		return
	}
	page, err := parseReactPage(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	data, err := s.loadReactTeamData(r)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar a equipe")
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	role := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("role")))
	filtered := make([]reactTeamMember, 0, len(data.Members))
	for _, member := range data.Members {
		if role != "" && strings.ToLower(member.Role) != role {
			continue
		}
		if status == "online" && !member.Online {
			continue
		}
		if status == "ativos" && !member.Active {
			continue
		}
		if status == "inativos" && member.Active {
			continue
		}
		if !reactContainsFold([]string{member.Name, member.Username, member.Role}, query) {
			continue
		}
		filtered = append(filtered, member)
	}
	pageItems, err := reactPageSlice(filtered, page.Limit, page.Offset)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_cursor", err.Error())
		return
	}
	total := len(filtered)
	data.Members = pageItems
	meta := reactPageMeta(total, page.Limit, page.Offset, reactQueryFilter(map[string]string{"q": query, "status": status, "role": role}))
	meta["scope"] = "tenant"
	writeReactResponse(w, http.StatusOK, data, meta)
}

func (s *Server) handleReactSectors(w http.ResponseWriter, r *http.Request) {
	if !requireReactTeamAccess(s, w, r, false) {
		return
	}
	data, err := s.loadReactTeamData(r)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os setores")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"items": data.Sectors}, map[string]string{"scope": "tenant"})
}

func decodeReactSectorInput(r *http.Request) (reactSectorInput, error) {
	var input reactSectorInput
	if err := json.NewDecoder(io.LimitReader(r.Body, 32*1024+1)).Decode(&input); err != nil {
		return input, errors.New("JSON inválido")
	}
	return input, nil
}

func normalizeReactSectorDays(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "0,1,2,3,4,5,6", nil
	}
	seen := map[int]bool{}
	for _, part := range strings.Split(value, ",") {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || id < 0 || id > 6 {
			return "", errors.New("dias_semana deve conter apenas números de 0 a 6")
		}
		seen[id] = true
	}
	ids := make([]int, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	values := make([]string, 0, len(ids))
	for _, id := range ids {
		values = append(values, strconv.Itoa(id))
	}
	return strings.Join(values, ","), nil
}

func normalizeReactSectorTime(value, fallback string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	if _, err := time.Parse("15:04", value); err != nil {
		return "", errors.New("horário deve estar no formato HH:MM")
	}
	return value, nil
}

func validateReactSector(sector store.Setor) error {
	name := strings.TrimSpace(sector.Name)
	if name == "" || len([]rune(name)) > 120 {
		return errors.New("nome deve ter entre 1 e 120 caracteres")
	}
	if len([]rune(sector.Description)) > 500 {
		return errors.New("descrição deve ter no máximo 500 caracteres")
	}
	if !reactHexColorPattern.MatchString(sector.Cor) {
		return errors.New("cor deve estar no formato hexadecimal")
	}
	if _, err := normalizeReactSectorTime(sector.HorarioAbertura, "00:00"); err != nil {
		return err
	}
	if _, err := normalizeReactSectorTime(sector.HorarioFechamento, "23:59"); err != nil {
		return err
	}
	if _, err := normalizeReactSectorDays(sector.DiasSemana); err != nil {
		return err
	}
	return nil
}

func applyReactSectorInput(sector *store.Setor, input reactSectorInput, creating bool) error {
	if input.Name != nil {
		sector.Name = strings.TrimSpace(*input.Name)
	}
	if input.Description != nil {
		sector.Description = strings.TrimSpace(*input.Description)
	}
	if input.HorarioAbertura != nil {
		sector.HorarioAbertura = strings.TrimSpace(*input.HorarioAbertura)
	}
	if input.HorarioFechamento != nil {
		sector.HorarioFechamento = strings.TrimSpace(*input.HorarioFechamento)
	}
	if input.DiasSemana != nil {
		sector.DiasSemana = strings.TrimSpace(*input.DiasSemana)
	}
	if input.Cor != nil {
		sector.Cor = strings.TrimSpace(*input.Cor)
	}
	if input.Active != nil {
		sector.Active = *input.Active
	} else if creating {
		sector.Active = true
	}
	if sector.HorarioAbertura == "" {
		sector.HorarioAbertura = "00:00"
	}
	if sector.HorarioFechamento == "" {
		sector.HorarioFechamento = "23:59"
	}
	if sector.DiasSemana == "" {
		sector.DiasSemana = "0,1,2,3,4,5,6"
	}
	if sector.Cor == "" {
		sector.Cor = "#0ea5e9"
	}
	days, err := normalizeReactSectorDays(sector.DiasSemana)
	if err != nil {
		return err
	}
	sector.DiasSemana = days
	return validateReactSector(*sector)
}

func parseReactSectorID(r *http.Request) (int, error) {
	value := strings.TrimSpace(r.PathValue("id"))
	if value == "" {
		return 0, errors.New("id inválido")
	}
	id, err := strconv.Atoi(value)
	if err != nil || id < 1 {
		return 0, errors.New("id inválido")
	}
	return id, nil
}

func (s *Server) handleReactSectorCreate(w http.ResponseWriter, r *http.Request) {
	if !requireReactTeamAccess(s, w, r, true) {
		return
	}
	input, err := decodeReactSectorInput(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	sector := store.Setor{}
	if err := applyReactSectorInput(&sector, input, true); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_sector", err.Error())
		return
	}
	id, err := s.st.CreateSetor(r.Context(), sector)
	if err != nil {
		if isReactSectorUniqueViolation(err) {
			writeReactError(w, http.StatusConflict, "sector_exists", "já existe um setor com esse nome")
		} else {
			writeReactError(w, http.StatusInternalServerError, "create_failed", "não foi possível criar o setor")
		}
		return
	}
	s.linkReactSupervisorSector(r.Context(), currentUser(r), id)
	created, err := s.st.GetSetor(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "setor criado, mas não foi possível carregá-lo")
		return
	}
	writeReactResponse(w, http.StatusCreated, reactTeamSectorFromStore(created, 0, 0), nil)
}

func (s *Server) handleReactSectorUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireReactTeamAccess(s, w, r, true) {
		return
	}
	id, err := parseReactSectorID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	sector, err := s.st.GetSetor(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "setor não encontrado")
		return
	}
	if !s.requireReactSectorScope(w, r, sector.ID) {
		return
	}
	input, err := decodeReactSectorInput(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := applyReactSectorInput(&sector, input, false); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_sector", err.Error())
		return
	}
	if err := s.st.UpdateSetor(r.Context(), sector); err != nil {
		if isReactSectorUniqueViolation(err) {
			writeReactError(w, http.StatusConflict, "sector_exists", "já existe um setor com esse nome")
		} else {
			writeReactError(w, http.StatusInternalServerError, "sector_update_failed", "não foi possível atualizar o setor")
		}
		return
	}
	updated, err := s.st.GetSetor(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "setor atualizado, mas não foi possível carregá-lo")
		return
	}
	writeReactResponse(w, http.StatusOK, reactTeamSectorFromStore(updated, 0, 0), nil)
}

func (s *Server) handleReactSectorToggle(w http.ResponseWriter, r *http.Request) {
	if !requireReactTeamAccess(s, w, r, true) {
		return
	}
	id, err := parseReactSectorID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	sector, err := s.st.GetSetor(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "setor não encontrado")
		return
	}
	if !s.requireReactSectorScope(w, r, sector.ID) {
		return
	}
	sector.Active = !sector.Active
	if err := s.st.UpdateSetor(r.Context(), sector); err != nil {
		writeReactError(w, http.StatusInternalServerError, "update_failed", "não foi possível alterar o estado do setor")
		return
	}
	writeReactResponse(w, http.StatusOK, reactTeamSectorFromStore(sector, 0, 0), nil)
}

func (s *Server) handleReactSectorDelete(w http.ResponseWriter, r *http.Request) {
	if !requireReactTeamAccess(s, w, r, true) {
		return
	}
	id, err := parseReactSectorID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	if _, err := s.st.GetSetor(r.Context(), id); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "setor não encontrado")
		return
	}
	if !s.requireReactSectorScope(w, r, id) {
		return
	}
	if err := s.st.DeleteSetor(r.Context(), id); err != nil {
		writeReactError(w, http.StatusConflict, "delete_failed", "não foi possível excluir o setor")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]int{"deleted": id}, nil)
}
