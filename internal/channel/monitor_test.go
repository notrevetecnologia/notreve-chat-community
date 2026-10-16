package channel

import (
	"errors"
	"testing"
)

func TestHealthTransition(t *testing.T) {
	cases := []struct {
		previous, current string
		want              bool
	}{
		{"connected", "disconnected", true},
		{"disconnected", "connected", true},
		{"unknown", "connected", false},
		{"connected", "connected", false},
		{"", "connected", false},
	}
	for _, c := range cases {
		if got := healthTransition(c.previous, c.current); got != c.want {
			t.Fatalf("healthTransition(%q, %q) = %v, want %v", c.previous, c.current, got, c.want)
		}
	}
}

func TestWhatsappHealthStatus(t *testing.T) {
	status, msg := whatsappHealthStatus(true, true, nil)
	if status != "connected" || msg != "" {
		t.Fatalf("whatsappHealthStatus(true, true, nil) = (%q, %q), want (\"connected\", \"\")", status, msg)
	}
	status, msg = whatsappHealthStatus(true, false, nil)
	if status != "disconnected" || msg != "" {
		t.Fatalf("whatsappHealthStatus(true, false, nil) = (%q, %q), want (\"disconnected\", \"\")", status, msg)
	}
	status, msg = whatsappHealthStatus(false, false, errors.New("boom"))
	if status != "disconnected" || msg == "" {
		t.Fatalf("whatsappHealthStatus(false, false, err) = (%q, %q), want (\"disconnected\", não-vazio)", status, msg)
	}
}

func TestTelegramHealthStatus(t *testing.T) {
	status, msg := telegramHealthStatus("https://example.com", nil)
	if status != "connected" || msg != "" {
		t.Fatalf("telegramHealthStatus(url, nil) = (%q, %q), want (\"connected\", \"\")", status, msg)
	}
	status, msg = telegramHealthStatus("", nil)
	if status != "disconnected" || msg != "" {
		t.Fatalf("telegramHealthStatus(\"\", nil) = (%q, %q), want (\"disconnected\", \"\")", status, msg)
	}
	status, msg = telegramHealthStatus("", errors.New("boom"))
	if status != "disconnected" || msg == "" {
		t.Fatalf("telegramHealthStatus(\"\", err) = (%q, %q), want (\"disconnected\", não-vazio)", status, msg)
	}
}
