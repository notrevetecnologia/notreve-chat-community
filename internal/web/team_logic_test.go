package web

import (
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func TestReactUserTargetHierarchy(t *testing.T) {
	atendente := &store.Usuario{ID: 1, Role: "atendente"}
	outroAtendente := &store.Usuario{ID: 9, Role: "atendente"}
	supervisor := &store.Usuario{ID: 2, Role: "supervisor"}
	outroSupervisor := &store.Usuario{ID: 8, Role: "supervisor"}
	admin := &store.Usuario{ID: 3, Role: "admin"}
	root := &store.Usuario{ID: 4, Role: "super_admin"}

	if err := validateReactUserTarget(root, admin); err != nil {
		t.Fatalf("super_admin deveria administrar admin: %v", err)
	}
	if err := validateReactUserTarget(admin, root); err == nil {
		t.Fatal("admin não deveria administrar super_admin")
	}
	if err := validateReactUserTarget(admin, admin); err != nil {
		t.Fatalf("admin deveria administrar outro admin: %v", err)
	}
	if err := validateReactUserTarget(admin, supervisor); err != nil {
		t.Fatalf("admin deveria administrar supervisor: %v", err)
	}
	if err := validateReactUserTarget(supervisor, atendente); err != nil {
		t.Fatalf("supervisor deveria administrar atendente: %v", err)
	}
	if err := validateReactUserTarget(supervisor, outroSupervisor); err == nil {
		t.Fatal("supervisor não deveria administrar outro supervisor")
	}
	if err := validateReactUserTarget(supervisor, admin); err == nil {
		t.Fatal("supervisor não deveria administrar admin")
	}
	if err := validateReactUserTarget(atendente, outroAtendente); err != nil {
		t.Fatalf("validação de alvo não deve bloquear sozinha o nível igual: %v", err)
	}
	self := &store.Usuario{ID: 5, Role: "supervisor"}
	if err := validateReactUserTarget(self, self); err != nil {
		t.Fatalf("usuário deveria poder editar o próprio perfil: %v", err)
	}
	if err := validateReactUserTarget(nil, atendente); err == nil {
		t.Fatal("ator ausente deveria ser rejeitado")
	}
}

func TestReactRoleAssignmentHierarchy(t *testing.T) {
	supervisor := &store.Usuario{ID: 2, Role: "supervisor"}
	admin := &store.Usuario{ID: 3, Role: "admin"}
	root := &store.Usuario{ID: 4, Role: "super_admin"}

	if err := validateReactRoleAssignment(root, nil, "super_admin"); err != nil {
		t.Fatalf("super_admin deveria atribuir qualquer perfil: %v", err)
	}
	if err := validateReactRoleAssignment(admin, nil, "admin"); err != nil {
		t.Fatalf("admin deveria atribuir admin: %v", err)
	}
	if err := validateReactRoleAssignment(admin, nil, "supervisor"); err != nil {
		t.Fatalf("admin deveria atribuir supervisor: %v", err)
	}
	if err := validateReactRoleAssignment(admin, nil, "super_admin"); err == nil {
		t.Fatal("admin não deveria atribuir super_admin")
	}
	if err := validateReactRoleAssignment(supervisor, nil, "atendente"); err != nil {
		t.Fatalf("supervisor deveria atribuir atendente: %v", err)
	}
	if err := validateReactRoleAssignment(supervisor, nil, "supervisor"); err == nil {
		t.Fatal("supervisor não deveria atribuir supervisor")
	}
	if err := validateReactRoleAssignment(supervisor, nil, "admin"); err == nil {
		t.Fatal("supervisor não deveria atribuir admin")
	}

	target := &store.Usuario{ID: 10, Role: "atendente"}
	if err := validateReactRoleAssignment(supervisor, target, "atendente"); err != nil {
		t.Fatalf("supervisor deveria manter atendente: %v", err)
	}
	if err := validateReactRoleAssignment(supervisor, target, "supervisor"); err == nil {
		t.Fatal("supervisor não deveria promover atendente a supervisor")
	}

	if err := validateReactRoleAssignment(supervisor, supervisor, "supervisor"); err != nil {
		t.Fatalf("no-op no próprio perfil deveria ser aceito: %v", err)
	}
	if err := validateReactRoleAssignment(supervisor, supervisor, "admin"); err == nil {
		t.Fatal("ninguém deveria se autopromover")
	}
	if err := validateReactRoleAssignment(admin, admin, "super_admin"); err == nil {
		t.Fatal("ninguém deveria se autopromover a super_admin")
	}
	if err := validateReactRoleAssignment(nil, nil, "atendente"); err == nil {
		t.Fatal("ator ausente deveria ser rejeitado")
	}
}

func TestReactUserVisibleHierarchy(t *testing.T) {
	atendente := &store.Usuario{ID: 1, Role: "atendente"}
	supervisor := &store.Usuario{ID: 2, Role: "supervisor"}
	outroSupervisor := store.Usuario{ID: 6, Role: "supervisor"}
	admin := store.Usuario{ID: 3, Role: "admin"}
	root := store.Usuario{ID: 4, Role: "super_admin"}
	s := &Server{}

	if reactUserVisible(nil, s, nil, *atendente) {
		t.Fatal("ator ausente não deveria ver usuários")
	}
	if !reactUserVisible(nil, s, atendente, *atendente) {
		t.Fatal("usuário deveria ver a si mesmo")
	}
	if !reactUserVisible(nil, s, &admin, admin) || !reactUserVisible(nil, s, &admin, *atendente) {
		t.Fatal("admin deveria ver usuários da própria empresa")
	}
	if reactUserVisible(nil, s, &admin, root) {
		t.Fatal("admin não deveria ver super_admin")
	}
	if !reactUserVisible(nil, s, &root, admin) {
		t.Fatal("super_admin deveria ver qualquer usuário")
	}
	if reactUserVisible(nil, s, supervisor, outroSupervisor) {
		t.Fatal("supervisor não deveria ver outro supervisor")
	}
	if reactUserVisible(nil, s, supervisor, admin) {
		t.Fatal("supervisor não deveria ver admin")
	}
	if reactUserVisible(nil, s, atendente, outroSupervisor) {
		t.Fatal("atendente não deveria ver outros usuários")
	}
}

func TestReactSectorInScope(t *testing.T) {
	own := map[int]bool{5: true, 7: true}
	if !reactSectorInScope(&store.Usuario{Role: "admin"}, nil, 99) {
		t.Fatal("admin deveria administrar qualquer setor")
	}
	if !reactSectorInScope(&store.Usuario{Role: "super_admin"}, nil, 99) {
		t.Fatal("super_admin deveria administrar qualquer setor")
	}
	if !reactSectorInScope(&store.Usuario{Role: "supervisor"}, own, 5) {
		t.Fatal("supervisor deveria administrar o próprio setor")
	}
	if reactSectorInScope(&store.Usuario{Role: "supervisor"}, own, 6) {
		t.Fatal("supervisor não deveria administrar setor alheio")
	}
	if reactSectorInScope(nil, own, 5) {
		t.Fatal("ator ausente deveria ser rejeitado")
	}
}

func TestReactUserRolesForScopesSupervisor(t *testing.T) {
	keys := func(actor *store.Usuario) []string {
		roles := reactUserRolesFor(actor)
		out := make([]string, 0, len(roles))
		for _, role := range roles {
			out = append(out, role.Key)
		}
		return out
	}

	supervisor := keys(&store.Usuario{Role: "supervisor"})
	if len(supervisor) != 1 || supervisor[0] != "atendente" {
		t.Fatalf("supervisor deveria atribuir apenas atendente: %v", supervisor)
	}
	admin := keys(&store.Usuario{Role: "admin"})
	if len(admin) != 3 || admin[0] != "admin" || admin[1] != "supervisor" || admin[2] != "atendente" {
		t.Fatalf("admin deveria atribuir admin/supervisor/atendente: %v", admin)
	}
	root := keys(&store.Usuario{Role: "super_admin"})
	if len(root) != 4 {
		t.Fatalf("super_admin deveria ver todos os perfis: %v", root)
	}
}
