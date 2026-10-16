package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

const (
	maxSkillNameChars        = 100
	maxSkillDescriptionChars = 250
	maxSkillCategoryChars    = 50
	maxSkillInstructionChars = 20000
)

type reactSkill struct {
	ID                int    `json:"id"`
	Nome              string `json:"nome"`
	Descricao         string `json:"descricao"`
	InstrucaoCompleta string `json:"instrucao_completa"`
	Categoria         string `json:"categoria"`
	Ativa             bool   `json:"ativa"`
	AgentIDs          []int  `json:"agent_ids"`
}

type reactSkillAgent struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type reactSkillInput struct {
	Nome              string `json:"nome"`
	Descricao         string `json:"descricao"`
	InstrucaoCompleta string `json:"instrucao_completa"`
	Categoria         string `json:"categoria"`
	Ativa             *bool  `json:"ativa"`
	AgentIDs          []int  `json:"agent_ids"`
}

type reactSkillsView struct {
	Items      []reactSkill      `json:"items"`
	Agents     []reactSkillAgent `json:"agents"`
	Categories []string          `json:"categories"`
}

func requireReactSkillsAccess(s *Server, w http.ResponseWriter, r *http.Request) bool {
	if _, err := store.TenantIDRequired(r.Context()); err != nil {
		writeReactError(w, http.StatusUnauthorized, "tenant_required", "contexto da empresa não identificado")
		return false
	}
	if !s.reactModuleEnabled(r.Context(), "ia") {
		writeReactError(w, http.StatusForbidden, "module_disabled", "módulo desativado")
		return false
	}
	if !userHasPerm(currentUser(r), permAgentesGerenciar) {
		writeReactError(w, http.StatusForbidden, "permission_denied", "sem permissão para gerenciar skills")
		return false
	}
	return true
}

func reactSkillFromStore(skill store.Skill) reactSkill {
	agentIDs := skill.AgentIDs
	if agentIDs == nil {
		agentIDs = []int{}
	}
	return reactSkill{
		ID: skill.ID, Nome: skill.Nome, Descricao: skill.Descricao,
		InstrucaoCompleta: skill.InstrucaoCompleta, Categoria: skill.Categoria,
		Ativa: skill.Ativa, AgentIDs: agentIDs,
	}
}

func reactSkillAgentsFromStore(agents []store.Agent) []reactSkillAgent {
	items := make([]reactSkillAgent, 0, len(agents))
	for _, agent := range agents {
		items = append(items, reactSkillAgent{ID: agent.ID, Name: agent.Name, Active: agent.Active})
	}
	return items
}

func parseReactSkillID(r *http.Request) (int, error) {
	id, err := strconv.Atoi(strings.TrimSpace(r.PathValue("id")))
	if err != nil || id < 1 {
		return 0, errors.New("id inválido")
	}
	return id, nil
}

func (s *Server) handleReactSkills(w http.ResponseWriter, r *http.Request) {
	if !requireReactSkillsAccess(s, w, r) {
		return
	}
	page, err := parseReactPage(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	if err := s.st.EnsureDefaultSkills(r.Context()); err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível preparar as skills padrão")
		return
	}
	skills, err := s.st.ListSkills(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível listar as skills")
		return
	}
	agents, err := s.st.ListAgents(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os agentes")
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	view := reactSkillsView{Items: make([]reactSkill, 0, len(skills)), Agents: reactSkillAgentsFromStore(agents), Categories: []string{}}
	seen := map[string]bool{}
	for _, skill := range skills {
		if skill.Categoria != "" && !seen[skill.Categoria] {
			seen[skill.Categoria] = true
			view.Categories = append(view.Categories, skill.Categoria)
		}
		if category != "" && skill.Categoria != category {
			continue
		}
		if status == "ativas" && !skill.Ativa {
			continue
		}
		if status == "inativas" && skill.Ativa {
			continue
		}
		if !reactContainsFold([]string{skill.Nome, skill.Descricao, skill.Categoria, skill.InstrucaoCompleta}, query) {
			continue
		}
		view.Items = append(view.Items, reactSkillFromStore(skill))
	}
	pageItems, err := reactPageSlice(view.Items, page.Limit, page.Offset)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_cursor", err.Error())
		return
	}
	total := len(view.Items)
	view.Items = pageItems
	meta := reactPageMeta(total, page.Limit, page.Offset, reactQueryFilter(map[string]string{"q": query, "category": category, "status": status}))
	writeReactResponse(w, http.StatusOK, view, meta)
}

func decodeReactSkillInput(r *http.Request) (reactSkillInput, error) {
	var input reactSkillInput
	if err := json.NewDecoder(io.LimitReader(r.Body, 256<<10)).Decode(&input); err != nil {
		return input, errors.New("JSON inválido")
	}
	return input, nil
}

func validateReactSkillInput(input reactSkillInput) (reactSkillInput, error) {
	input.Nome = strings.TrimSpace(input.Nome)
	input.Descricao = strings.TrimSpace(input.Descricao)
	input.InstrucaoCompleta = strings.TrimSpace(input.InstrucaoCompleta)
	input.Categoria = strings.TrimSpace(input.Categoria)
	if input.Nome == "" {
		return input, errors.New("informe o nome da skill")
	}
	if len([]rune(input.Nome)) > maxSkillNameChars {
		return input, errors.New("o nome da skill deve ter no máximo 100 caracteres")
	}
	if len([]rune(input.Descricao)) > maxSkillDescriptionChars {
		return input, errors.New("a descrição deve ter no máximo 250 caracteres")
	}
	if input.Descricao == "" {
		input.Descricao = input.Nome
	}
	if input.InstrucaoCompleta == "" {
		return input, errors.New("a instrução completa da skill é obrigatória")
	}
	if len([]rune(input.InstrucaoCompleta)) > maxSkillInstructionChars {
		return input, errors.New("a instrução completa deve ter no máximo 20000 caracteres")
	}
	if input.Categoria == "" {
		input.Categoria = "geral"
	}
	if len([]rune(input.Categoria)) > maxSkillCategoryChars {
		return input, errors.New("a categoria deve ter no máximo 50 caracteres")
	}
	return input, nil
}

func (s *Server) handleReactSkillCreate(w http.ResponseWriter, r *http.Request) {
	if !requireReactSkillsAccess(s, w, r) {
		return
	}
	input, err := decodeReactSkillInput(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	input, err = validateReactSkillInput(input)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_skill", err.Error())
		return
	}
	ativa := true
	if input.Ativa != nil {
		ativa = *input.Ativa
	}
	skill := store.Skill{Nome: input.Nome, Descricao: input.Descricao, InstrucaoCompleta: input.InstrucaoCompleta, Categoria: input.Categoria, Ativa: ativa, AgentIDs: input.AgentIDs}
	id, err := s.st.CreateSkill(r.Context(), skill)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			writeReactError(w, http.StatusConflict, "duplicate_skill", "já existe uma skill com esse nome")
			return
		}
		writeReactError(w, http.StatusInternalServerError, "create_failed", "não foi possível criar a skill")
		return
	}
	created, err := s.st.GetSkill(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "skill criada, mas não foi possível carregá-la")
		return
	}
	writeReactResponse(w, http.StatusCreated, reactSkillFromStore(created), nil)
}

func (s *Server) handleReactSkillUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireReactSkillsAccess(s, w, r) {
		return
	}
	id, err := parseReactSkillID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	current, err := s.st.GetSkill(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "skill não encontrada")
		return
	}
	input, err := decodeReactSkillInput(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	input, err = validateReactSkillInput(input)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_skill", err.Error())
		return
	}
	ativa := current.Ativa
	if input.Ativa != nil {
		ativa = *input.Ativa
	}
	skill := store.Skill{ID: id, Nome: input.Nome, Descricao: input.Descricao, InstrucaoCompleta: input.InstrucaoCompleta, Categoria: input.Categoria, Ativa: ativa, AgentIDs: input.AgentIDs}
	if err := s.st.UpdateSkill(r.Context(), skill); err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			writeReactError(w, http.StatusConflict, "duplicate_skill", "já existe uma skill com esse nome")
			return
		}
		writeReactError(w, http.StatusInternalServerError, "update_failed", "não foi possível atualizar a skill")
		return
	}
	updated, err := s.st.GetSkill(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "skill atualizada, mas não foi possível carregá-la")
		return
	}
	writeReactResponse(w, http.StatusOK, reactSkillFromStore(updated), nil)
}

func (s *Server) handleReactSkillDelete(w http.ResponseWriter, r *http.Request) {
	if !requireReactSkillsAccess(s, w, r) {
		return
	}
	id, err := parseReactSkillID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	if _, err := s.st.GetSkill(r.Context(), id); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "skill não encontrada")
		return
	}
	if err := s.st.DeleteSkill(r.Context(), id); err != nil {
		writeReactError(w, http.StatusInternalServerError, "delete_failed", "não foi possível excluir a skill")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]int{"deleted": id}, nil)
}

func (s *Server) handleReactSkillSeed(w http.ResponseWriter, r *http.Request) {
	if !requireReactSkillsAccess(s, w, r) {
		return
	}
	if err := s.st.EnsureDefaultSkills(r.Context()); err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível restaurar as skills padrão")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]bool{"ok": true}, nil)
}
