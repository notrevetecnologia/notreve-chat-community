package channel

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestVerifyWebhookSignature(t *testing.T) {
	key := "chave-teste"
	body := []byte(`{"type":"Message","body":"olá"}`)
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write(body)
	sum := mac.Sum(nil)

	hexSig := hex.EncodeToString(sum)
	b64Sig := base64.StdEncoding.EncodeToString(sum)

	if !verifyWebhookSignature(key, body, hexSig) {
		t.Fatal("assinatura em hex deveria ser válida")
	}
	if !verifyWebhookSignature(key, body, b64Sig) {
		t.Fatal("assinatura em base64 deveria ser válida")
	}

	// Hex maiúsculo deve continuar válido (normalizamos para lowercase).
	if !verifyWebhookSignature(key, body, stringsUppercase(hexSig)) {
		t.Fatal("assinatura em hex maiúsculo deveria ser válida")
	}

	// Assinatura gerada com outra chave NÃO deve ser aceita.
	wrong := hmac.New(sha256.New, []byte("outra-chave"))
	wrong.Write(body)
	if verifyWebhookSignature(key, body, hex.EncodeToString(wrong.Sum(nil))) {
		t.Fatal("assinatura com chave errada não deveria ser aceita")
	}
	if verifyWebhookSignature(key, body, "") {
		t.Fatal("assinatura vazia não deveria ser aceita")
	}
	if verifyWebhookSignature("", body, hexSig) {
		t.Fatal("chave vazia não deveria aceitar")
	}
}

func stringsUppercase(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'a' && b[i] <= 'z' {
			b[i] -= 32
		}
	}
	return string(b)
}
