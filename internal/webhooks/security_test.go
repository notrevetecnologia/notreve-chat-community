package webhooks

import (
	"testing"
	"time"
)

func TestSignAndVerifyHMACWithTimestamp(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	timestamp := time.Unix(1730000000, 0)
	body := []byte(`{"event":"message.received","data":{"id":7}}`)
	signature, err := Sign(secret, timestamp, body)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(secret, "1730000000", body, signature, timestamp.Add(30*time.Second), 2*time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := Verify(secret, "1730000000", []byte(`{"event":"tampered"}`), signature, timestamp, 2*time.Minute); err == nil {
		t.Fatal("payload alterado deveria invalidar a assinatura")
	}
	if err := Verify(secret, "1730000000", body, signature, timestamp.Add(3*time.Minute), 2*time.Minute); err == nil {
		t.Fatal("timestamp fora da janela deveria ser rejeitado")
	}
}

func TestBackoffDelayIsExponentialAndCapped(t *testing.T) {
	limits := Limits{InitialBackoff: 2 * time.Second, MaxBackoff: 5 * time.Second}
	values := []time.Duration{BackoffDelay(limits, 1), BackoffDelay(limits, 2), BackoffDelay(limits, 3), BackoffDelay(limits, 4)}
	want := []time.Duration{2 * time.Second, 4 * time.Second, 5 * time.Second, 5 * time.Second}
	for index := range want {
		if values[index] != want[index] {
			t.Fatalf("tentativa %d: esperado %s, recebido %s", index+1, want[index], values[index])
		}
	}
	if got := BackoffDelay(limits, 0); got != 0 {
		t.Fatalf("tentativa zero deveria retornar zero, recebeu %s", got)
	}
}

func TestValidateURLRequiresHTTPSAndBlocksSSRFTargets(t *testing.T) {
	if err := ValidateURL("https://api.example.com/hooks", nil); err != nil {
		t.Fatal(err)
	}
	if err := ValidateURL("http://api.example.com/hooks", nil); err == nil {
		t.Fatal("HTTP deveria ser rejeitado")
	}
	if err := ValidateURL("https://127.0.0.1/hooks", nil); err == nil {
		t.Fatal("loopback deveria ser rejeitado")
	}
	if err := ValidateURL("https://service.internal/hooks", nil); err == nil {
		t.Fatal("host interno deveria ser rejeitado")
	}
	if err := ValidateURL("https://hooks.example.com/v1", []string{"api.example.com"}); err == nil {
		t.Fatal("host fora da allowlist deveria ser rejeitado")
	}
	if err := ValidateURL("https://tenant.api.example.com/v1", []string{"*.api.example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateURL("https://user:password@api.example.com/v1", nil); err == nil {
		t.Fatal("credenciais na URL deveriam ser rejeitadas")
	}
}
