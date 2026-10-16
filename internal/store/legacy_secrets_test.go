package store

import (
	"errors"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/securesecret"
)

func TestMigrateLegacySecretEncryptsPlaintext(t *testing.T) {
	testLGPDKey(t)

	migrated, err := migrateLegacySecret("providers.api_key", "token-legado")
	if err != nil {
		t.Fatalf("migrateLegacySecret() = %v", err)
	}
	if !securesecret.IsEncrypted(migrated) {
		t.Fatal("credencial legada deveria ser cifrada")
	}
	plain, err := securesecret.Decrypt("providers.api_key", migrated)
	if err != nil || plain != "token-legado" {
		t.Fatalf("credencial migrada = %q, %v", plain, err)
	}
}

func TestMigrateLegacySecretKeepsEncryptedAndEmptyValues(t *testing.T) {
	testLGPDKey(t)
	ciphertext, err := securesecret.Encrypt("providers.api_key", "token-existente")
	if err != nil {
		t.Fatal(err)
	}

	for name, value := range map[string]string{"encrypted": ciphertext, "empty": ""} {
		t.Run(name, func(t *testing.T) {
			got, err := migrateLegacySecret("providers.api_key", value)
			if err != nil {
				t.Fatalf("migrateLegacySecret() = %v", err)
			}
			if got != value {
				t.Fatalf("migrateLegacySecret() = %q, want %q", got, value)
			}
		})
	}
}

func TestMigrateLegacySecretRequiresKey(t *testing.T) {
	t.Setenv("AGENTE_IA_SECRETS_KEY", "")
	if _, err := migrateLegacySecret("providers.api_key", "token-legado"); !errors.Is(err, securesecret.ErrKeyUnavailable) {
		t.Fatalf("erro de chave = %v", err)
	}
}
