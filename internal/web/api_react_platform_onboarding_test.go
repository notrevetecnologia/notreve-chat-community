package web

import (
	"errors"
	"strings"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
	"golang.org/x/crypto/bcrypt"
)

func TestValidateReactPlatformTemporaryPassword(t *testing.T) {
	if err := validateReactPlatformTemporaryPassword("  senha-segura  "); err != nil {
		t.Fatalf("senha válida rejeitada: %v", err)
	}
	if err := validateReactPlatformTemporaryPassword("12345"); err == nil {
		t.Fatal("senha curta deveria ser rejeitada")
	}
	if err := validateReactPlatformTemporaryPassword(strings.Repeat("a", 73)); err == nil {
		t.Fatal("senha acima do limite bcrypt deveria ser rejeitada")
	}
}

func TestOnlyReactPlatformTenantAdminRequiresExactlyOneAdmin(t *testing.T) {
	admin := store.Usuario{ID: 12, TenantID: 8, Role: "admin"}
	got, err := onlyReactPlatformTenantAdmin([]store.Usuario{{ID: 2, Role: "atendente"}, admin})
	if err != nil || got.ID != admin.ID {
		t.Fatalf("admin único não selecionado corretamente: %#v, %v", got, err)
	}
	for _, users := range [][]store.Usuario{
		nil,
		{{ID: 1, Role: "supervisor"}},
		{admin, {ID: 13, TenantID: 8, Role: "admin"}},
	} {
		if _, err := onlyReactPlatformTenantAdmin(users); !errors.Is(err, errReactPlatformTenantAdminCount) {
			t.Fatalf("estado de admins inválido deveria ser rejeitado: %v", err)
		}
	}
}

func TestResetReactPlatformTenantAdminPasswordOnlyChangesHash(t *testing.T) {
	original := store.Usuario{
		ID:           12,
		TenantID:     8,
		Name:         "Dona da empresa",
		Username:     "dona",
		PasswordHash: "hash-antigo",
		Role:         "admin",
		Permissions:  platformAdminPermissions(),
		AtendenteID:  intPtr(44),
		Active:       true,
	}

	updated, err := resetReactPlatformTenantAdminPassword(original, "  nova-senha-temporaria  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(updated.PasswordHash), []byte("nova-senha-temporaria")); err != nil {
		t.Fatalf("hash bcrypt não valida a senha temporária: %v", err)
	}
	if updated.PasswordHash == original.PasswordHash {
		t.Fatal("o hash de senha deveria mudar")
	}
	if updated.ID != original.ID || updated.TenantID != original.TenantID || updated.Name != original.Name || updated.Username != original.Username || updated.Role != "admin" || updated.Permissions != original.Permissions || updated.Active != original.Active || updated.AtendenteID == nil || *updated.AtendenteID != 44 {
		t.Fatalf("redefinição alterou dados além da senha: %#v", updated)
	}
	if userHasPerm(&updated, permPlataformaGerenciar) {
		t.Fatal("admin do tenant não pode receber acesso ao control plane")
	}
}
