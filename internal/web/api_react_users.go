package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
	"golang.org/x/crypto/bcrypt"
)

type reactUserView struct {
	ID                int      `json:"id"`
	Name              string   `json:"name"`
	Username          string   `json:"username"`
	Role              string   `json:"role"`
	Active            bool     `json:"active"`
	Online            bool     `json:"online"`
	AtendenteID       *int     `json:"atendente_id,omitempty"`
	SetorIDs          []int    `json:"setor_ids"`
	Habilidades       []string `json:"habilidades,omitempty"`
	Capacidade        int      `json:"capacidade"`
	Permissions       []string `json:"permissions"`
	PermissionsCustom bool     `json:"permissions_customized"`
	CreatedAt         string   `json:"created_at"`
}

type reactUserSectorOption struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type reactUserRoleOption struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type reactUserPermissionOption struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type reactUserPermissionGroup struct {
	Module      string                      `json:"module"`
	Label       string                      `json:"label"`
	Permissions []reactUserPermissionOption `json:"permissions"`
}

type reactUsersData struct {
	Items            []reactUserView            `json:"items"`
	Sectors          []reactUserSectorOption    `json:"sectors"`
	Roles            []reactUserRoleOption      `json:"roles"`
	PermissionGroups []reactUserPermissionGroup `json:"permission_groups"`
}

type reactUserInput struct {
	Name        *string   `json:"name"`
	Username    *string   `json:"username"`
	Password    *string   `json:"password"`
	Role        *string   `json:"role"`
	Permissions *[]string `json:"permissions"`
	SetorIDs    *[]int    `json:"setor_ids"`
	Habilidades *[]string `json:"habilidades"`
	Capacidade  *int      `json:"capacidade"`
	Active      *bool     `json:"active"`
}

func reactUserAccessAllowed(s *Server, r *http.Request, manage bool) bool {
	if !s.reactModuleEnabled(r.Context(), "equipe") {
		return false
	}
	if manage {
		return userHasPerm(currentUser(r), permEquipeGerenciar)
	}
	return userHasPerm(currentUser(r), permEquipeGerenciar)
}

func requireReactUserAccess(s *Server, w http.ResponseWriter, r *http.Request, manage bool) bool {
	if _, err := store.TenantIDRequired(r.Context()); err != nil {
		writeReactError(w, http.StatusUnauthorized, "tenant_required", "contexto da empresa não identificado")
		return false
	}
	if !s.reactModuleEnabled(r.Context(), "equipe") {
		writeReactError(w, http.StatusForbidden, "module_disabled", "módulo desativado")
		return false
	}
	if !reactUserAccessAllowed(s, r, manage) {
		writeReactError(w, http.StatusForbidden, "permission_denied", "sem permissão para administrar usuários")
		return false
	}
	return true
}

func reactUserRoleLabel(role string) string {
	switch role {
	case "super_admin":
		return "Super administrador"
	case "admin":
		return "Administrador"
	case "supervisor":
		return "Supervisor"
	case "atendente":
		return "Atendente"
	default:
		return "Operador"
	}
}

func reactUserRolesFor(actor *store.Usuario) []reactUserRoleOption {
	roles := []reactUserRoleOption{
		{Key: "super_admin", Label: reactUserRoleLabel("super_admin")},
		{Key: "admin", Label: reactUserRoleLabel("admin")},
		{Key: "supervisor", Label: reactUserRoleLabel("supervisor")},
		{Key: "atendente", Label: reactUserRoleLabel("atendente")},
	}
	out := make([]reactUserRoleOption, 0, len(roles))
	for _, role := range roles {
		if actor != nil && actor.Role != "super_admin" {
			if actor.Role == "supervisor" && roleLevel(role.Key) >= roleLevel(actor.Role) {
				continue
			}
			if roleLevel(actor.Role) < roleLevel(role.Key) {
				continue
			}
		}
		out = append(out, role)
	}
	return out
}

func reactUserPermissionGroupsFor(actor *store.Usuario) []reactUserPermissionGroup {
	groups := make([]reactUserPermissionGroup, 0, len(permCatalog))
	for _, group := range permCatalog {
		permissions := make([]reactUserPermissionOption, 0, len(group.Perms))
		for _, permission := range group.Perms {
			if permission.Key == permPlataformaGerenciar && (actor == nil || actor.Role != "super_admin") {
				continue
			}
			if actor != nil && actor.Role != "super_admin" && !userHasPerm(actor, permission.Key) {
				continue
			}
			permissions = append(permissions, reactUserPermissionOption{Key: permission.Key, Label: permission.Label})
		}
		if len(permissions) > 0 {
			groups = append(groups, reactUserPermissionGroup{Module: group.Module, Label: group.Label, Permissions: permissions})
		}
	}
	return groups
}

func reactTeamUserFromStore(ctx context.Context, s *Server, user store.Usuario) reactUserView {
	permissionSet := userPermSet(&user)
	permissions := make([]string, 0, len(permissionSet))
	for permission := range permissionSet {
		permissions = append(permissions, permission)
	}
	view := reactUserView{
		ID: user.ID, Name: user.Name, Username: user.Username, Role: user.Role, Active: user.Active,
		AtendenteID: user.AtendenteID, SetorIDs: []int{}, Habilidades: []string{}, Permissions: permissions,
		PermissionsCustom: strings.TrimSpace(user.Permissions) != "", CreatedAt: user.CreatedAt.Format(time.RFC3339),
	}
	if user.AtendenteID != nil {
		if attendant, err := s.st.GetAtendente(ctx, *user.AtendenteID); err == nil {
			view.Online = attendant.Online
			view.SetorIDs = append(view.SetorIDs, attendant.SetorIDs...)
			view.Capacidade = attendant.Capacidade
		}
		if skills, err := s.st.GetAtendenteHabilidades(ctx, *user.AtendenteID); err == nil {
			view.Habilidades = append(view.Habilidades, skills...)
		}
	}
	sort.Strings(view.Permissions)
	return view
}

func reactUserVisible(ctx context.Context, s *Server, actor *store.Usuario, target store.Usuario) bool {
	if actor == nil {
		return false
	}
	if actor.Role == "super_admin" || actor.Role == "admin" {
		return actor.Role == "super_admin" || target.Role != "super_admin"
	}
	if actor.ID == target.ID {
		return true
	}
	if actor.Role != "supervisor" || roleLevel(target.Role) >= roleLevel("supervisor") {
		return false
	}
	return hasSectorIntersection(s.userSetores(ctx, actor), s.userSetores(ctx, &target))
}

func (s *Server) loadReactUsersData(r *http.Request) (reactUsersData, error) {
	actor := currentUser(r)
	users, err := s.st.ListUsuarios(r.Context())
	if err != nil {
		return reactUsersData{}, err
	}
	sectors, err := s.st.ListSetores(r.Context())
	if err != nil {
		return reactUsersData{}, err
	}
	data := reactUsersData{
		Items:            make([]reactUserView, 0, len(users)),
		Sectors:          make([]reactUserSectorOption, 0, len(sectors)),
		Roles:            reactUserRolesFor(actor),
		PermissionGroups: reactUserPermissionGroupsFor(actor),
	}
	sectorScope := map[int]bool{}
	if actor != nil && actor.Role == "supervisor" {
		for _, id := range s.userSetores(r.Context(), actor) {
			sectorScope[id] = true
		}
	}
	for _, sector := range sectors {
		if actor != nil && actor.Role == "supervisor" && !sectorScope[sector.ID] {
			continue
		}
		data.Sectors = append(data.Sectors, reactUserSectorOption{ID: sector.ID, Name: sector.Name, Active: sector.Active})
	}
	for _, user := range users {
		if !reactUserVisible(r.Context(), s, actor, user) {
			continue
		}
		data.Items = append(data.Items, reactTeamUserFromStore(r.Context(), s, user))
	}
	return data, nil
}

func decodeReactUserInput(r *http.Request) (reactUserInput, error) {
	var input reactUserInput
	if err := json.NewDecoder(io.LimitReader(r.Body, 96*1024+1)).Decode(&input); err != nil {
		return input, errors.New("JSON inválido")
	}
	return input, nil
}

func normalizeReactUserRole(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "super_admin", "admin", "supervisor", "atendente":
		return value, nil
	default:
		return "", errors.New("perfil inválido")
	}
}

func validateReactUsername(value string) error {
	value = strings.TrimSpace(value)
	if len([]rune(value)) < 3 || len([]rune(value)) > 120 || strings.ContainsAny(value, " \t\r\n") {
		return errors.New("usuário deve ter entre 3 e 120 caracteres e não conter espaços")
	}
	return nil
}

func normalizeReactUserSectors(ctx context.Context, s *Server, actor *store.Usuario, values []int, existing []int) ([]int, error) {
	seen := map[int]bool{}
	for _, value := range values {
		if value < 1 {
			return nil, errors.New("setor inválido")
		}
		seen[value] = true
	}
	sectors, err := s.st.ListSetores(ctx)
	if err != nil {
		return nil, err
	}
	allowed := map[int]bool{}
	for _, sector := range sectors {
		allowed[sector.ID] = true
	}
	if actor != nil && actor.Role == "supervisor" {
		own := map[int]bool{}
		for _, id := range s.userSetores(ctx, actor) {
			own[id] = true
		}
		keep := map[int]bool{}
		for _, id := range existing {
			keep[id] = true
		}
		for id := range seen {
			if !own[id] && !keep[id] {
				return nil, errors.New("supervisor só pode vincular seus próprios setores")
			}
		}
	}
	out := make([]int, 0, len(seen))
	for id := range seen {
		if !allowed[id] {
			return nil, errors.New("setor não pertence a esta empresa")
		}
		out = append(out, id)
	}
	sort.Ints(out)
	return out, nil
}

func normalizeReactUserPermissions(actor *store.Usuario, role string, values []string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	allowed := map[string]bool{}
	for _, permission := range allPermsList() {
		allowed[permission] = true
	}
	seen := map[string]bool{}
	for _, permission := range values {
		permission = strings.TrimSpace(permission)
		if permission == "" {
			continue
		}
		if !allowed[permission] {
			return "", errors.New("permissão inválida")
		}
		if permission == permPlataformaGerenciar && role != "super_admin" {
			return "", errors.New("a permissão de plataforma é exclusiva do super_admin")
		}
		if actor != nil && actor.Role != "super_admin" && !userHasPerm(actor, permission) {
			return "", errors.New("você não pode conceder uma permissão que não possui")
		}
		seen[permission] = true
	}
	permissions := make([]string, 0, len(seen))
	for permission := range seen {
		permissions = append(permissions, permission)
	}
	sort.Strings(permissions)
	return strings.Join(permissions, ","), nil
}

func validateReactUserTarget(actor, target *store.Usuario) error {
	if actor == nil || target == nil {
		return errors.New("usuário não autenticado")
	}
	if actor.Role == "super_admin" {
		return nil
	}
	if target.Role == "super_admin" {
		return errors.New("somente super_admin pode administrar outro super_admin")
	}
	if actor.ID == target.ID {
		return nil
	}
	if roleLevel(actor.Role) < roleLevel(target.Role) {
		return errors.New("seu perfil não pode administrar este usuário")
	}
	if actor.Role == "supervisor" && roleLevel(target.Role) >= roleLevel(actor.Role) {
		return errors.New("supervisor só pode administrar usuários de nível inferior")
	}
	return nil
}

func validateReactRoleAssignment(actor, target *store.Usuario, role string) error {
	if actor == nil {
		return errors.New("usuário não autenticado")
	}
	if role == "super_admin" && actor.Role != "super_admin" {
		return errors.New("somente super_admin pode usar esse perfil")
	}
	if actor.Role == "super_admin" {
		return nil
	}
	if target != nil && target.ID == actor.ID {
		if role != actor.Role {
			return errors.New("você não pode alterar o próprio perfil")
		}
		return nil
	}
	if actor.Role == "supervisor" {
		if roleLevel(role) >= roleLevel("supervisor") {
			return errors.New("supervisor só pode atribuir perfil de nível inferior")
		}
		return nil
	}
	if roleLevel(actor.Role) < roleLevel(role) {
		return errors.New("seu perfil não pode atribuir esse perfil")
	}
	return nil
}

func validateReactUserInput(input reactUserInput, creating bool) error {
	if creating && (input.Name == nil || strings.TrimSpace(valueOrEmpty(input.Name)) == "") {
		return errors.New("nome é obrigatório")
	}
	if creating && input.Username == nil {
		return errors.New("usuário é obrigatório")
	}
	if input.Name != nil && (strings.TrimSpace(*input.Name) == "" || len([]rune(strings.TrimSpace(*input.Name))) > 160) {
		return errors.New("nome deve ter entre 1 e 160 caracteres")
	}
	if input.Username != nil {
		if err := validateReactUsername(*input.Username); err != nil {
			return err
		}
	}
	if creating && (input.Password == nil || len([]rune(strings.TrimSpace(*input.Password))) < 6) {
		return errors.New("senha deve ter pelo menos 6 caracteres")
	}
	if input.Password != nil && strings.TrimSpace(*input.Password) != "" && len([]rune(strings.TrimSpace(*input.Password))) < 6 {
		return errors.New("senha deve ter pelo menos 6 caracteres")
	}
	return nil
}

func normalizeReactUserHabilidades(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if len([]rune(value)) > 80 {
			value = string([]rune(value)[:80])
		}
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func (s *Server) ensureReactUserAttendant(ctx context.Context, user *store.Usuario, setorIDs []int, habilidades []string, capacidade int) error {
	if user.Role == "super_admin" {
		return nil
	}
	if user.AtendenteID == nil {
		if user.Active {
			if err := s.st.EnsureCanCreateActiveAttendant(ctx); err != nil {
				return err
			}
		}
		id, err := s.st.CreateAtendente(ctx, store.Atendente{Name: user.Name, Active: user.Active, Online: false, HorarioAbertura: "00:00", HorarioFechamento: "23:59", DiasSemana: "0,1,2,3,4,5,6", SetorIDs: setorIDs, Capacidade: capacidade})
		if err != nil {
			return err
		}
		user.AtendenteID = &id
		if err := s.st.SetAtendenteHabilidades(ctx, id, habilidades); err != nil {
			return err
		}
	}
	attendant, err := s.st.GetAtendente(ctx, *user.AtendenteID)
	if err != nil {
		return err
	}
	if !attendant.Active && user.Active {
		if err := s.st.EnsureCanCreateActiveAttendant(ctx); err != nil {
			return err
		}
	}
	attendant.Name = user.Name
	attendant.Active = user.Active
	if !user.Active {
		attendant.Online = false
	}
	attendant.SetorIDs = setorIDs
	attendant.Capacidade = capacidade
	if err := s.st.UpdateAtendente(ctx, attendant); err != nil {
		return err
	}
	return s.st.SetAtendenteHabilidades(ctx, *user.AtendenteID, habilidades)
}

func normalizeReactAtendenteCapacidade(value int) int {
	if value < 0 {
		return 0
	}
	if value > 999 {
		return 999
	}
	return value
}

func writeReactTeamLimitError(w http.ResponseWriter, err error) bool {
	if writeReactPlanLimitError(w, err) {
		return true
	}
	if errors.Is(err, store.ErrTenantAttendantLimitReached) {
		writeReactError(w, http.StatusConflict, "attendant_limit_reached", "limite de atendentes ativos do plano atingido")
		return true
	}
	return false
}

func (s *Server) handleReactUsers(w http.ResponseWriter, r *http.Request) {
	if !requireReactUserAccess(s, w, r, false) {
		return
	}
	page, err := parseReactPage(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	data, err := s.loadReactUsersData(r)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os usuários")
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	role := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("role")))
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	filtered := make([]reactUserView, 0, len(data.Items))
	for _, item := range data.Items {
		if role != "" && strings.ToLower(item.Role) != role {
			continue
		}
		if status == "ativos" && !item.Active {
			continue
		}
		if status == "inativos" && item.Active {
			continue
		}
		if !reactContainsFold([]string{item.Name, item.Username, item.Role}, query) {
			continue
		}
		filtered = append(filtered, item)
	}
	pageItems, err := reactPageSlice(filtered, page.Limit, page.Offset)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_cursor", err.Error())
		return
	}
	total := len(filtered)
	data.Items = pageItems
	meta := reactPageMeta(total, page.Limit, page.Offset, reactQueryFilter(map[string]string{"q": query, "role": role, "status": status}))
	meta["scope"] = "tenant"
	writeReactResponse(w, http.StatusOK, data, meta)
}

func (s *Server) handleReactUser(w http.ResponseWriter, r *http.Request) {
	if !requireReactUserAccess(s, w, r, false) {
		return
	}
	id, err := parseReactSectorID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	user, err := s.st.GetUsuarioByID(r.Context(), id)
	if err != nil || !reactUserVisible(r.Context(), s, currentUser(r), user) {
		writeReactError(w, http.StatusNotFound, "not_found", "usuário não encontrado")
		return
	}
	writeReactResponse(w, http.StatusOK, reactTeamUserFromStore(r.Context(), s, user), nil)
}

func (s *Server) handleReactUserCreate(w http.ResponseWriter, r *http.Request) {
	if !requireReactUserAccess(s, w, r, true) {
		return
	}
	input, err := decodeReactUserInput(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := validateReactUserInput(input, true); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_user", err.Error())
		return
	}
	actor := currentUser(r)
	role := "atendente"
	if input.Role != nil {
		role, err = normalizeReactUserRole(*input.Role)
		if err != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_role", err.Error())
			return
		}
	}
	if err := validateReactRoleAssignment(actor, nil, role); err != nil {
		writeReactError(w, http.StatusForbidden, "role_forbidden", err.Error())
		return
	}
	name := strings.TrimSpace(valueOrEmpty(input.Name))
	username := strings.TrimSpace(valueOrEmpty(input.Username))
	if existing, lookupErr := s.st.GetUsuarioByUsername(r.Context(), username); lookupErr == nil && existing.ID > 0 {
		writeReactError(w, http.StatusConflict, "username_exists", "esse usuário já está cadastrado")
		return
	}
	sectorIDs := []int{}
	if input.SetorIDs != nil {
		sectorIDs, err = normalizeReactUserSectors(r.Context(), s, actor, *input.SetorIDs, nil)
		if err != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_sectors", err.Error())
			return
		}
	}
	permissions := ""
	if input.Permissions != nil {
		permissions, err = normalizeReactUserPermissions(actor, role, *input.Permissions)
		if err != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_permissions", err.Error())
			return
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(strings.TrimSpace(*input.Password)), bcrypt.DefaultCost)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "password_failed", "não foi possível proteger a senha")
		return
	}
	active := true
	if input.Active != nil {
		active = *input.Active
	}
	if active {
		if err := s.st.EnsureCanCreateActiveUser(r.Context()); err != nil {
			if writeReactPlanLimitError(w, err) {
				return
			}
			writeReactError(w, http.StatusInternalServerError, "plan_check_failed", "não foi possível validar o limite do plano")
			return
		}
	}
	user := store.Usuario{Name: name, Username: username, PasswordHash: string(hash), Role: role, Permissions: permissions, Active: active}
	habilidades := []string{}
	if input.Habilidades != nil {
		habilidades = normalizeReactUserHabilidades(*input.Habilidades)
	}
	capacidade := 0
	if input.Capacidade != nil {
		capacidade = normalizeReactAtendenteCapacidade(*input.Capacidade)
	}
	if role != "super_admin" {
		if err := s.ensureReactUserAttendant(r.Context(), &user, sectorIDs, habilidades, capacidade); err != nil {
			if writeReactTeamLimitError(w, err) {
				return
			}
			writeReactError(w, http.StatusInternalServerError, "attendant_failed", "não foi possível preparar a equipe do usuário")
			return
		}
	}
	id, err := s.st.CreateUsuario(r.Context(), user)
	if err != nil {
		if user.AtendenteID != nil {
			_ = s.st.DeleteAtendente(r.Context(), *user.AtendenteID)
		}
		if isReactSectorUniqueViolation(err) {
			writeReactError(w, http.StatusConflict, "username_exists", "esse usuário já está cadastrado")
		} else {
			writeReactError(w, http.StatusInternalServerError, "create_failed", "não foi possível criar o usuário")
		}
		return
	}
	created, err := s.st.GetUsuarioByID(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "usuário criado, mas não foi possível carregá-lo")
		return
	}
	writeReactResponse(w, http.StatusCreated, reactTeamUserFromStore(r.Context(), s, created), nil)
}

func (s *Server) handleReactUserUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireReactUserAccess(s, w, r, true) {
		return
	}
	id, err := parseReactSectorID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	user, err := s.st.GetUsuarioByID(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusNotFound, "not_found", "usuário não encontrado")
		return
	}
	wasActive := user.Active
	actor := currentUser(r)
	if !reactUserVisible(r.Context(), s, actor, user) {
		writeReactError(w, http.StatusNotFound, "not_found", "usuário não encontrado")
		return
	}
	if err := validateReactUserTarget(actor, &user); err != nil {
		writeReactError(w, http.StatusForbidden, "user_forbidden", err.Error())
		return
	}
	input, err := decodeReactUserInput(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := validateReactUserInput(input, false); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_user", err.Error())
		return
	}
	if input.Name != nil {
		user.Name = strings.TrimSpace(*input.Name)
	}
	if input.Username != nil {
		user.Username = strings.TrimSpace(*input.Username)
		if existing, lookupErr := s.st.GetUsuarioByUsername(r.Context(), user.Username); lookupErr == nil && existing.ID != user.ID {
			writeReactError(w, http.StatusConflict, "username_exists", "esse usuário já está cadastrado")
			return
		}
	}
	role := user.Role
	if input.Role != nil {
		role, err = normalizeReactUserRole(*input.Role)
		if err != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_role", err.Error())
			return
		}
	}
	if err := validateReactRoleAssignment(actor, &user, role); err != nil {
		writeReactError(w, http.StatusForbidden, "role_forbidden", err.Error())
		return
	}
	sectorIDs := []int{}
	if user.AtendenteID != nil {
		if attendant, attendantErr := s.st.GetAtendente(r.Context(), *user.AtendenteID); attendantErr == nil {
			sectorIDs = attendant.SetorIDs
		}
	}
	if input.SetorIDs != nil {
		sectorIDs, err = normalizeReactUserSectors(r.Context(), s, actor, *input.SetorIDs, sectorIDs)
		if err != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_sectors", err.Error())
			return
		}
	}
	if input.Permissions != nil {
		user.Permissions, err = normalizeReactUserPermissions(actor, role, *input.Permissions)
		if err != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_permissions", err.Error())
			return
		}
	}
	if input.Password != nil && strings.TrimSpace(*input.Password) != "" {
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(strings.TrimSpace(*input.Password)), bcrypt.DefaultCost)
		if hashErr != nil {
			writeReactError(w, http.StatusInternalServerError, "password_failed", "não foi possível proteger a senha")
			return
		}
		user.PasswordHash = string(hash)
	}
	if input.Active != nil {
		if user.ID == actor.ID && !*input.Active {
			writeReactError(w, http.StatusBadRequest, "self_deactivate", "você não pode desativar seu próprio acesso")
			return
		}
		user.Active = *input.Active
	}
	if !wasActive && user.Active {
		if err := s.st.EnsureCanCreateActiveUser(r.Context()); err != nil {
			if writeReactPlanLimitError(w, err) {
				return
			}
			writeReactError(w, http.StatusInternalServerError, "plan_check_failed", "não foi possível validar o limite do plano")
			return
		}
	}
	oldAtendenteID := user.AtendenteID
	user.Role = role
	if role == "super_admin" {
		user.AtendenteID = nil
	}
	habilidades := []string{}
	capacidade := 0
	if user.AtendenteID != nil {
		if skills, skillsErr := s.st.GetAtendenteHabilidades(r.Context(), *user.AtendenteID); skillsErr == nil {
			habilidades = skills
		}
		if attendant, attendantErr := s.st.GetAtendente(r.Context(), *user.AtendenteID); attendantErr == nil {
			capacidade = attendant.Capacidade
		}
	}
	if input.Habilidades != nil {
		habilidades = normalizeReactUserHabilidades(*input.Habilidades)
	}
	if input.Capacidade != nil {
		capacidade = normalizeReactAtendenteCapacidade(*input.Capacidade)
	}
	if role != "super_admin" {
		if err := s.ensureReactUserAttendant(r.Context(), &user, sectorIDs, habilidades, capacidade); err != nil {
			if writeReactTeamLimitError(w, err) {
				return
			}
			writeReactError(w, http.StatusInternalServerError, "attendant_failed", "não foi possível sincronizar a equipe do usuário")
			return
		}
	}
	if err := s.st.UpdateUsuario(r.Context(), user); err != nil {
		if isReactSectorUniqueViolation(err) {
			writeReactError(w, http.StatusConflict, "username_exists", "esse usuário já está cadastrado")
		} else {
			writeReactError(w, http.StatusInternalServerError, "update_failed", "não foi possível atualizar o usuário")
		}
		return
	}
	if role == "super_admin" && oldAtendenteID != nil {
		_ = s.st.DeleteAtendente(r.Context(), *oldAtendenteID)
	}
	updated, err := s.st.GetUsuarioByID(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "usuário atualizado, mas não foi possível carregá-lo")
		return
	}
	writeReactResponse(w, http.StatusOK, reactTeamUserFromStore(r.Context(), s, updated), nil)
}

func (s *Server) handleReactUserToggle(w http.ResponseWriter, r *http.Request) {
	if !requireReactUserAccess(s, w, r, true) {
		return
	}
	id, err := parseReactSectorID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	user, err := s.st.GetUsuarioByID(r.Context(), id)
	if err != nil || !reactUserVisible(r.Context(), s, currentUser(r), user) {
		writeReactError(w, http.StatusNotFound, "not_found", "usuário não encontrado")
		return
	}
	if err := validateReactUserTarget(currentUser(r), &user); err != nil {
		writeReactError(w, http.StatusForbidden, "user_forbidden", err.Error())
		return
	}
	if user.ID == currentUser(r).ID && user.Active {
		writeReactError(w, http.StatusBadRequest, "self_deactivate", "você não pode desativar seu próprio acesso")
		return
	}
	if !user.Active {
		if err := s.st.EnsureCanCreateActiveUser(r.Context()); err != nil {
			if writeReactPlanLimitError(w, err) {
				return
			}
			writeReactError(w, http.StatusInternalServerError, "plan_check_failed", "não foi possível validar o limite do plano")
			return
		}
	}
	user.Active = !user.Active
	if user.AtendenteID != nil {
		if attendant, attendantErr := s.st.GetAtendente(r.Context(), *user.AtendenteID); attendantErr == nil {
			if !attendant.Active && user.Active {
				if err := s.st.EnsureCanCreateActiveAttendant(r.Context()); err != nil {
					if writeReactTeamLimitError(w, err) {
						return
					}
					writeReactError(w, http.StatusInternalServerError, "plan_check_failed", "não foi possível validar o limite do plano")
					return
				}
			}
		}
	}
	if err := s.st.UpdateUsuario(r.Context(), user); err != nil {
		writeReactError(w, http.StatusInternalServerError, "update_failed", "não foi possível alterar o estado do usuário")
		return
	}
	if user.AtendenteID != nil {
		if attendant, attendantErr := s.st.GetAtendente(r.Context(), *user.AtendenteID); attendantErr == nil {
			attendant.Active = user.Active
			if !user.Active {
				attendant.Online = false
			}
			if err := s.st.UpdateAtendente(r.Context(), attendant); err != nil {
				Logger.Printf("usuário %d: atendente %d não sincronizou o estado ativo: %v", user.ID, attendant.ID, err)
			}
		} else {
			Logger.Printf("usuário %d: atendente %d não pôde ser lido para sincronizar: %v", user.ID, *user.AtendenteID, attendantErr)
		}
	}
	writeReactResponse(w, http.StatusOK, reactTeamUserFromStore(r.Context(), s, user), nil)
}

func (s *Server) handleReactUserDelete(w http.ResponseWriter, r *http.Request) {
	if !requireReactUserAccess(s, w, r, true) {
		return
	}
	id, err := parseReactSectorID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	user, err := s.st.GetUsuarioByID(r.Context(), id)
	if err != nil || !reactUserVisible(r.Context(), s, currentUser(r), user) {
		writeReactError(w, http.StatusNotFound, "not_found", "usuário não encontrado")
		return
	}
	if err := validateReactUserTarget(currentUser(r), &user); err != nil {
		writeReactError(w, http.StatusForbidden, "user_forbidden", err.Error())
		return
	}
	if user.ID == currentUser(r).ID {
		writeReactError(w, http.StatusBadRequest, "self_delete", "você não pode excluir seu próprio acesso")
		return
	}
	if err := s.st.DeleteUsuario(r.Context(), id); err != nil {
		writeReactError(w, http.StatusInternalServerError, "delete_failed", "não foi possível excluir o usuário")
		return
	}
	if user.AtendenteID != nil {
		if err := s.st.DeleteAtendente(r.Context(), *user.AtendenteID); err != nil {
			Logger.Printf("usuário %d: atendente %d órfão após excluir o acesso: %v", user.ID, *user.AtendenteID, err)
		}
	}
	writeReactResponse(w, http.StatusOK, map[string]int{"deleted": id}, nil)
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
