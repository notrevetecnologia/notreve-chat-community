package store

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func testLGPDKey(t *testing.T) {
	t.Helper()
	t.Setenv("AGENTE_IA_SECRETS_KEY", base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
}

func TestLGPDSensitiveEncryptionRoundTrip(t *testing.T) {
	testLGPDKey(t)
	store := New(nil)
	if !store.LGPDEncryptionEnabled() {
		t.Fatal("expected encryption to be enabled with a valid key")
	}
	plaintext := "CPF: 12312312312"
	ciphertext, nonce, err := store.EncryptLGPDSensitive(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if len(nonce) != 12 {
		t.Fatalf("nonce length = %d, want 12", len(nonce))
	}
	if strings.Contains(string(ciphertext), plaintext) {
		t.Fatal("ciphertext must not contain plaintext")
	}
	got, err := store.DecryptLGPDSensitive(ciphertext, nonce)
	if err != nil || got != plaintext {
		t.Fatalf("decrypted = %q, err = %v", got, err)
	}
}

func TestLGPDSensitiveEncryptionDisabledWithoutKey(t *testing.T) {
	t.Setenv("AGENTE_IA_SECRETS_KEY", "")
	store := New(nil)
	if store.LGPDEncryptionEnabled() {
		t.Fatal("encryption must be disabled when the key is absent")
	}
	if _, _, err := store.EncryptLGPDSensitive("secret"); !errors.Is(err, ErrLGPDEncryptionUnavailable) {
		t.Fatalf("error = %v, want unavailable", err)
	}
}

func TestLGPDSensitiveEncryptionRejectsTampering(t *testing.T) {
	testLGPDKey(t)
	store := New(nil)
	ciphertext, nonce, err := store.EncryptLGPDSensitive("secret")
	if err != nil {
		t.Fatal(err)
	}
	ciphertext[0] ^= 0x01
	if _, err := store.DecryptLGPDSensitive(ciphertext, nonce); !errors.Is(err, ErrLGPDDecryptionFailed) {
		t.Fatalf("tampered ciphertext error = %v", err)
	}
}

func TestMaskLGPDSensitiveDocument(t *testing.T) {
	if got := MaskLGPDSensitiveDocument("123"); got != "***" {
		t.Fatalf("short doc = %q, want ***", got)
	}
	if got := MaskLGPDSensitiveDocument("1234"); got != "****" {
		t.Fatalf("exact length = %q, want ****", got)
	}
	if got := MaskLGPDSensitiveDocument("12312312312"); got != "*******2312" {
		t.Fatalf("masked = %q, want *******2312", got)
	}
	if got := MaskLGPDSensitiveDocument("  "); got != "" {
		t.Fatalf("blank = %q, want empty", got)
	}
}
