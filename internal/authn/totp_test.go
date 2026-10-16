package authn

import (
	"strings"
	"testing"
	"time"
)

const rfc6238SHA1Secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

func TestTOTPCodeRFC6238SixDigits(t *testing.T) {
	cases := []struct {
		unix int64
		want string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
		{20000000000, "353130"},
	}
	for _, tc := range cases {
		got, err := totpCode(rfc6238SHA1Secret, tc.unix/30)
		if err != nil {
			t.Fatalf("totpCode(%d) = %v", tc.unix, err)
		}
		if got != tc.want {
			t.Fatalf("totpCode(%d) = %s, want %s", tc.unix, got, tc.want)
		}
	}
}

func TestVerifyTOTPAcceptsClockSkewAndRejectsInvalid(t *testing.T) {
	now := time.Unix(1234567890, 0)
	previous, _ := totpCode(rfc6238SHA1Secret, now.Unix()/30-1)
	current, _ := totpCode(rfc6238SHA1Secret, now.Unix()/30)
	next, _ := totpCode(rfc6238SHA1Secret, now.Unix()/30+1)
	for _, code := range []string{previous, current, next} {
		if !VerifyTOTP(rfc6238SHA1Secret, code, now) {
			t.Fatalf("código dentro da janela foi rejeitado: %s", code)
		}
	}
	for _, code := range []string{"", "12345", "1234567", "abcdef", "999999"} {
		if VerifyTOTP(rfc6238SHA1Secret, code, now) {
			t.Fatalf("código inválido foi aceito: %q", code)
		}
	}
}

func TestGenerateTOTPSecretProducesValidBase32(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	if len(secret) != 32 || strings.Contains(secret, "=") {
		t.Fatalf("segredo TOTP inesperado: %q", secret)
	}
	if _, err := totpCode(secret, 1); err != nil {
		t.Fatalf("segredo gerado não é utilizável: %v", err)
	}
}
