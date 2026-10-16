package web

import "testing"

func permIn(target string, list []string) bool {
	for _, p := range list {
		if p == target {
			return true
		}
	}
	return false
}

// Nenhum papel que não seja super_admin pode gerenciar a plataforma.
func TestRoleDefaultsNeverGrantPlatformToNonSuperAdmin(t *testing.T) {
	for _, role := range []string{"admin", "supervisor", "atendente"} {
		if permIn(permPlataformaGerenciar, roleDefaultPerms[role]) {
			t.Fatalf("role %q não deveria ter permPlataformaGerenciar", role)
		}
	}
}

	// Atendente só atende: nunca gerencia agentes, IA, canais, equipe/config, campanhas, chatflows, relatórios.
func TestRoleDefaultsAtendenteHasNoOwnerActions(t *testing.T) {
	forbidden := []string{
		permAgentesGerenciar, permIAsGerenciar, permCanaisGerenciar, permEquipeGerenciar,
		permSetoresGerenciar, permIntegracoesGerenciar, permConfigGerenciar,
		permCampanhasGerenciar, permChatflowsGerenciar, permRelatoriosVer,
		permPlataformaGerenciar,
	}
	for _, p := range forbidden {
		if permIn(p, roleDefaultPerms["atendente"]) {
			t.Fatalf("atendente não deveria ter %q", p)
		}
	}
}

// Supervisor gerencia operação, mas não a plataforma nem ferramentas sensíveis de dono.
func TestRoleDefaultsSupervisorScoped(t *testing.T) {
	perms := roleDefaultPerms["supervisor"]
	if permIn("perm_inexistente", perms) {
		t.Fatal("sanidade")
	}
	if !permIn(permConversasResponder, perms) || !permIn(permCampanhasGerenciar, perms) {
		t.Fatal("supervisor deveria responder conversas e gerenciar campanhas")
	}
	if permIn(permPlataformaGerenciar, perms) || permIn(permIAsGerenciar, perms) {
		t.Fatal("supervisor não deveria gerenciar plataforma nem provedores de IA")
	}
}

// Admin gerencia o negócio (agentes, campanhas, canais), mas nunca a plataforma.
func TestRoleDefaultsAdminHasEverythingButPlatform(t *testing.T) {
	perms := roleDefaultPerms["admin"]
	if permIn(permPlataformaGerenciar, perms) {
		t.Fatal("admin não deveria gerenciar a plataforma")
	}
	if !permIn(permAgentesGerenciar, perms) || !permIn(permCampanhasGerenciar, perms) || !permIn(permCanaisGerenciar, perms) {
		t.Fatal("admin deveria gerenciar agentes, campanhas e canais")
	}
}

// Ver a fila da IA é granular e vem ligada por padrão para quem atende.
func TestRoleDefaultsConversasVerIA(t *testing.T) {
	for _, role := range []string{"admin", "supervisor", "atendente"} {
		if !permIn(permConversasVerIA, roleDefaultPerms[role]) {
			t.Fatalf("papel %q deveria ter %q por padrão", role, permConversasVerIA)
		}
	}
	if !permIn(permConversasEncerrar, roleDefaultPerms["atendente"]) || !permIn(permConversasTransferir, roleDefaultPerms["atendente"]) {
		t.Fatal("atendente deveria encerrar/transferir por padrão para resolver a própria fila")
	}
}

// Super_admin é o único com tudo.
func TestRoleDefaultsSuperAdminHasAll(t *testing.T) {
	perms := roleDefaultPerms["super_admin"]
	if !permIn(permPlataformaGerenciar, perms) || len(perms) != len(allPermsList()) {
		t.Fatal("super_admin deveria ter todas as permissões da plataforma")
	}
}

func TestRoleDefaultsAuditedScreens(t *testing.T) {
	for _, p := range []string{permIntegracoesGerenciar, permConfigGerenciar} {
		for _, role := range []string{"supervisor", "atendente"} {
			if permIn(p, roleDefaultPerms[role]) {
				t.Fatalf("papel %q não deveria ter %q", role, p)
			}
		}
		for _, role := range []string{"admin", "super_admin"} {
			if !permIn(p, roleDefaultPerms[role]) {
				t.Fatalf("papel %q deveria ter %q", role, p)
			}
		}
	}
	if !permIn(permRelatoriosVer, roleDefaultPerms["admin"]) || !permIn(permRelatoriosVer, roleDefaultPerms["supervisor"]) {
		t.Fatal("admin e supervisor deveriam ver relatórios")
	}
	if permIn(permRelatoriosVer, roleDefaultPerms["atendente"]) {
		t.Fatal("atendente não deveria ver relatórios")
	}
}
