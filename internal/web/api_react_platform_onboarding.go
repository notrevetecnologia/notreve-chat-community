package web

import (
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
	"golang.org/x/crypto/bcrypt"
)

// reactPlatformAdminInput representa a primeira credencial entregue ao dono
// da empresa. A senha temporária é aceita somente na criação: não é registrada
// em logs e nunca volta na resposta HTTP.
type reactPlatformAdminInput struct {
	Name              string `json:"name"`
	Username          string `json:"username"`
	TemporaryPassword string `json:"temporary_password"`
}

// reactPlatformAdminPasswordResetInput recebe uma nova senha temporária sem
// aceitar alterações de papel, permissões ou estado do administrador.
// A senha nunca é devolvida na resposta nem incluída em mensagens de erro.
type reactPlatformAdminPasswordResetInput struct {
	TemporaryPassword string `json:"temporary_password"`
}

// reactPlatformAdminView é deliberadamente menor que reactUserView. O control
// plane só precisa saber quem é o administrador de cada empresa; não recebe
// hash de senha, permissões internas ou dados de atendimento.
type reactPlatformAdminView struct {
	ID        int    `json:"id"`
	TenantID  int    `json:"tenant_id"`
	Name      string `json:"name"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	Active    bool   `json:"active"`
	CreatedAt string `json:"created_at"`
}

func reactPlatformAdminFromStore(user store.Usuario) reactPlatformAdminView {
	return reactPlatformAdminView{
		ID: user.ID, TenantID: user.TenantID, Name: user.Name, Username: user.Username,
		Role: user.Role, Active: user.Active, CreatedAt: user.CreatedAt.Format(time.RFC3339),
	}
}

func platformAdminTenantContext(r *http.Request, s *Server) (int, store.PlatformTenant, error) {
	tenantID, err := platformTenantID(r)
	if err != nil {
		return 0, store.PlatformTenant{}, err
	}
	tenant, err := s.st.GetPlatformTenant(r.Context(), tenantID)
	if err != nil {
		return 0, store.PlatformTenant{}, err
	}
	return tenantID, tenant, nil
}

func platformAdminPermissions() string {
	permissions := allPermsExcept(permPlataformaGerenciar)
	sort.Strings(permissions)
	return strings.Join(permissions, ",")
}

func decodeReactPlatformAdmin(r *http.Request) (reactPlatformAdminInput, error) {
	var input reactPlatformAdminInput
	if err := decodeReactAdminJSON(r, &input, 32*1024); err != nil {
		return input, err
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Username = strings.TrimSpace(input.Username)
	return input, nil
}

func decodeReactPlatformAdminPasswordReset(r *http.Request) (reactPlatformAdminPasswordResetInput, error) {
	var input reactPlatformAdminPasswordResetInput
	if err := decodeReactAdminJSON(r, &input, 32*1024); err != nil {
		return input, err
	}
	input.TemporaryPassword = strings.TrimSpace(input.TemporaryPassword)
	return input, nil
}

func validateReactPlatformTemporaryPassword(password string) error {
	password = strings.TrimSpace(password)
	if len([]rune(password)) < 6 {
		return errors.New("senha temporária deve ter pelo menos 6 caracteres")
	}
	// bcrypt só usa senhas com até 72 bytes. Validar antes evita responder
	// 500 a uma solicitação inválida e mantém o mesmo valor validado e salvo.
	if len([]byte(password)) > 72 {
		return errors.New("senha temporária deve ter no máximo 72 bytes")
	}
	return nil
}

func validateReactPlatformAdmin(input reactPlatformAdminInput) error {
	if input.Name == "" || len([]rune(input.Name)) > 160 {
		return errors.New("nome deve ter entre 1 e 160 caracteres")
	}
	if err := validateReactUsername(input.Username); err != nil {
		return err
	}
	return validateReactPlatformTemporaryPassword(input.TemporaryPassword)
}

var errReactPlatformTenantAdminCount = errors.New("a empresa deve possuir exatamente um administrador")

func onlyReactPlatformTenantAdmin(users []store.Usuario) (store.Usuario, error) {
	var admin store.Usuario
	count := 0
	for _, user := range users {
		if user.Role != "admin" {
			continue
		}
		admin = user
		count++
	}
	if count != 1 {
		return store.Usuario{}, errReactPlatformTenantAdminCount
	}
	return admin, nil
}

func resetReactPlatformTenantAdminPassword(admin store.Usuario, temporaryPassword string) (store.Usuario, error) {
	password := strings.TrimSpace(temporaryPassword)
	if err := validateReactPlatformTemporaryPassword(password); err != nil {
		return store.Usuario{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return store.Usuario{}, err
	}
	// O usuário vem do banco e somente o hash é trocado. Papel, permissões,
	// estado e vínculo com atendente permanecem intactos, sem dar acesso ao
	// control plane ao administrador do tenant.
	admin.PasswordHash = string(hash)
	return admin, nil
}

func (s *Server) handleReactPlatformTenantAdmins(w http.ResponseWriter, r *http.Request) {
	tenantID, _, err := platformAdminTenantContext(r, s)
	if err != nil {
		status := http.StatusBadRequest
		code, message := "invalid_tenant", "empresa inválida"
		if errors.Is(err, store.ErrPlatformTenantNotFound) || errors.Is(err, pgx.ErrNoRows) {
			status, code, message = http.StatusNotFound, "not_found", "empresa não encontrada"
		}
		writeReactError(w, status, code, message)
		return
	}

	tenantCtx := store.WithTenantID(r.Context(), tenantID)
	users, err := s.st.ListUsuarios(tenantCtx)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os administradores")
		return
	}

	admins := make([]reactPlatformAdminView, 0)
	for _, user := range users {
		if user.Role == "admin" {
			admins = append(admins, reactPlatformAdminFromStore(user))
		}
	}
	writeReactResponse(w, http.StatusOK, admins, map[string]interface{}{"tenant_id": tenantID})
}

func (s *Server) handleReactPlatformTenantAdminCreate(w http.ResponseWriter, r *http.Request) {
	tenantID, _, err := platformAdminTenantContext(r, s)
	if err != nil {
		status := http.StatusBadRequest
		code, message := "invalid_tenant", "empresa inválida"
		if errors.Is(err, store.ErrPlatformTenantNotFound) || errors.Is(err, pgx.ErrNoRows) {
			status, code, message = http.StatusNotFound, "not_found", "empresa não encontrada"
		}
		writeReactError(w, status, code, message)
		return
	}

	input, err := decodeReactPlatformAdmin(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := validateReactPlatformAdmin(input); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_admin", err.Error())
		return
	}

	tenantCtx := store.WithTenantID(r.Context(), tenantID)
	users, err := s.st.ListUsuarios(tenantCtx)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível verificar os administradores")
		return
	}
	for _, user := range users {
		if user.Role == "admin" {
			writeReactError(w, http.StatusConflict, "admin_already_exists", "esta empresa já possui administrador")
			return
		}
	}
	// A consulta é deliberadamente no contexto da empresa-alvo. O banco também
	// preserva sua própria restrição de unicidade como proteção contra corrida.
	if existing, lookupErr := s.st.GetUsuarioByUsername(tenantCtx, input.Username); lookupErr == nil && existing.ID > 0 {
		writeReactError(w, http.StatusConflict, "username_exists", "este usuário já está cadastrado nesta empresa")
		return
	} else if lookupErr != nil && !errors.Is(lookupErr, pgx.ErrNoRows) {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível validar o usuário")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(strings.TrimSpace(input.TemporaryPassword)), bcrypt.DefaultCost)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "password_failed", "não foi possível proteger a senha temporária")
		return
	}

	admin := store.Usuario{
		Name:         input.Name,
		Username:     input.Username,
		PasswordHash: string(hash),
		Role:         "admin",
		Permissions:  platformAdminPermissions(),
		Active:       true,
	}
	if err := s.ensureReactUserAttendant(tenantCtx, &admin, []int{}, []string{}, 0); err != nil {
		writeReactError(w, http.StatusInternalServerError, "attendant_failed", "não foi possível preparar o acesso da empresa")
		return
	}

	id, err := s.st.CreateUsuario(tenantCtx, admin)
	if err != nil {
		if admin.AtendenteID != nil {
			_ = s.st.DeleteAtendente(tenantCtx, *admin.AtendenteID)
		}
		if isReactSectorUniqueViolation(err) {
			writeReactError(w, http.StatusConflict, "username_exists", "este usuário já está indisponível")
			return
		}
		writeReactError(w, http.StatusInternalServerError, "create_failed", "não foi possível criar o administrador")
		return
	}

	created, err := s.st.GetUsuarioByID(tenantCtx, id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "administrador criado, mas não foi possível carregá-lo")
		return
	}
	writeReactResponse(w, http.StatusCreated, reactPlatformAdminFromStore(created), nil)
}

// handleReactPlatformTenantAdminPasswordReset redefine a senha temporária do
// único administrador de uma empresa. A rota que a registra deve ser sempre
// envolvida por authJSON(requireSuperAdmin(...)); este handler não torna o
// admin do tenant apto a acessar o control plane.
func (s *Server) handleReactPlatformTenantAdminPasswordReset(w http.ResponseWriter, r *http.Request) {
	tenantID, _, err := platformAdminTenantContext(r, s)
	if err != nil {
		status := http.StatusBadRequest
		code, message := "invalid_tenant", "empresa inválida"
		if errors.Is(err, store.ErrPlatformTenantNotFound) || errors.Is(err, pgx.ErrNoRows) {
			status, code, message = http.StatusNotFound, "not_found", "empresa não encontrada"
		}
		writeReactError(w, status, code, message)
		return
	}

	input, err := decodeReactPlatformAdminPasswordReset(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido")
		return
	}
	if err := validateReactPlatformTemporaryPassword(input.TemporaryPassword); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_password", err.Error())
		return
	}

	tenantCtx := store.WithTenantID(r.Context(), tenantID)
	users, err := s.st.ListUsuarios(tenantCtx)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível verificar o administrador")
		return
	}
	admin, err := onlyReactPlatformTenantAdmin(users)
	if err != nil {
		writeReactError(w, http.StatusConflict, "admin_state_invalid", "a empresa deve possuir exatamente um administrador")
		return
	}
	admin, err = resetReactPlatformTenantAdminPassword(admin, input.TemporaryPassword)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "password_failed", "não foi possível proteger a senha temporária")
		return
	}
	if err := s.st.UpdateUsuario(tenantCtx, admin); err != nil {
		writeReactError(w, http.StatusInternalServerError, "update_failed", "não foi possível redefinir a senha temporária")
		return
	}

	// 204 não serializa nenhum campo, impedindo a exposição acidental da senha
	// ou do hash em respostas futuras.
	w.WriteHeader(http.StatusNoContent)
}
