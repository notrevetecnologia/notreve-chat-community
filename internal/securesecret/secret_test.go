package securesecret

import (
	"bytes"
	"encoding/base64"
	"errors"
	"testing"
)

func validTestKey() string {
	return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32))
}

func TestValidateKey(t *testing.T) {
	t.Setenv("AGENTE_IA_SECRETS_KEY", validTestKey())
	if err := ValidateKey(); err != nil {
		t.Fatalf("ValidateKey() = %v", err)
	}
}

func TestValidateKeyRejectsMissingAndInvalid(t *testing.T) {
	t.Setenv("AGENTE_IA_SECRETS_KEY", "")
	if err := ValidateKey(); !errors.Is(err, ErrKeyUnavailable) {
		t.Fatalf("missing key error = %v", err)
	}
	t.Setenv("AGENTE_IA_SECRETS_KEY", base64.StdEncoding.EncodeToString([]byte("short")))
	if err := ValidateKey(); err == nil {
		t.Fatal("invalid key was accepted")
	}
}

func TestEncryptDecryptRoundTripAndScopeBinding(t *testing.T) {
	t.Setenv("AGENTE_IA_SECRETS_KEY", validTestKey())
	ciphertext, err := Encrypt("channels.token", "segredo")
	if err != nil {
		t.Fatalf("Encrypt() = %v", err)
	}
	if ciphertext == "segredo" || !IsEncrypted(ciphertext) {
		t.Fatalf("ciphertext inválido: %q", ciphertext)
	}
	plain, err := Decrypt("channels.token", ciphertext)
	if err != nil || plain != "segredo" {
		t.Fatalf("Decrypt() = %q, %v", plain, err)
	}
	if _, err := Decrypt("providers.api_key", ciphertext); err == nil {
		t.Fatal("ciphertext transplantado para outro scope foi aceito")
	}
}

func TestDecryptRejectsPlaintext(t *testing.T) {
	t.Setenv("AGENTE_IA_SECRETS_KEY", validTestKey())
	if _, err := Decrypt("channels.token", "plaintext"); !errors.Is(err, ErrPlaintextSecret) {
		t.Fatalf("plaintext error = %v", err)
	}
}

func TestLookupHashDeterministic(t *testing.T) {
	a := LookupHash(" token ")
	b := LookupHash("token")
	c := LookupHash("outro")
	if !bytes.Equal(a, b) {
		t.Fatal("LookupHash deve normalizar espaços externos")
	}
	if bytes.Equal(a, c) {
		t.Fatal("LookupHash colidiu para entradas diferentes")
	}
}
