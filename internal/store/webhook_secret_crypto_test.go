package store

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func testWebhookSecretKey(t *testing.T) {
	t.Helper()
	t.Setenv("AGENTE_IA_SECRETS_KEY", base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
}

func TestWebhookSigningSecretEncryptionRoundTrip(t *testing.T) {
	testWebhookSecretKey(t)
	store := New(nil)
	secret := "whsec_internal_only_123"
	ciphertext, nonce, err := store.encryptWebhookSigningSecret(secret)
	if err != nil {
		t.Fatal(err)
	}
	if len(nonce) != 12 {
		t.Fatalf("nonce length = %d, want 12", len(nonce))
	}
	if strings.Contains(string(ciphertext), secret) {
		t.Fatal("ciphertext must not contain plaintext secret")
	}
	got, err := store.decryptWebhookSigningSecret(ciphertext, nonce)
	if err != nil || got != secret {
		t.Fatalf("decrypted secret = %q, err = %v", got, err)
	}
}

func TestWebhookSigningSecretEncryptionRejectsUnavailableKey(t *testing.T) {
	for _, key := range []string{"", "not-base64", base64.StdEncoding.EncodeToString([]byte("short"))} {
		t.Run(key, func(t *testing.T) {
			t.Setenv("AGENTE_IA_SECRETS_KEY", key)
			store := New(nil)
			if _, _, err := store.encryptWebhookSigningSecret("secret"); !errors.Is(err, ErrWebhookSecretStorageUnavailable) {
				t.Fatalf("key %q error = %v, want unavailable storage", key, err)
			}
		})
	}
}

func TestWebhookSigningSecretEncryptionRejectsTampering(t *testing.T) {
	testWebhookSecretKey(t)
	store := New(nil)
	ciphertext, nonce, err := store.encryptWebhookSigningSecret("secret")
	if err != nil {
		t.Fatal(err)
	}
	ciphertext[0] ^= 0x01
	if _, err := store.decryptWebhookSigningSecret(ciphertext, nonce); !errors.Is(err, ErrWebhookSigningSecretUnavailable) {
		t.Fatalf("tampered ciphertext error = %v", err)
	}
}

func TestWebhookSigningSecretRequiresTenantContext(t *testing.T) {
	testWebhookSecretKey(t)
	store := New(nil)
	if _, err := store.WebhookSigningSecret(context.Background(), 1); !errors.Is(err, ErrTenantContextRequired) {
		t.Fatalf("missing tenant context error = %v", err)
	}
}
