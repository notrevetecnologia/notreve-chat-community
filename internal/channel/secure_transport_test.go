package channel

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestIsBlockedHostChannel(t *testing.T) {
	blocked := []string{
		"localhost", "127.0.0.1", "10.0.0.5", "192.168.1.10",
		"172.16.0.1", "169.254.169.254", "0.0.0.0", "::1",
	}
	allowed := []string{
		"example.com", "8.8.8.8", "1.1.1.1",
	}
	for _, h := range blocked {
		if !isBlockedHost(h) {
			t.Errorf("%q deveria ser bloqueado", h)
		}
	}
	for _, h := range allowed {
		if isBlockedHost(h) {
			t.Errorf("%q não deveria ser bloqueado", h)
		}
	}
}

func TestSafeDialContextRejectsPrivate(t *testing.T) {
	_, err := safeDialContext(context.Background(), "tcp", "127.0.0.1:8080")
	if err == nil {
		t.Fatal("dial para loopback deveria ser rejeitado")
	}
}

func TestSafeDialContextResolvesPublic(t *testing.T) {
	if testing.Short() {
		t.Skip("requer DNS externo")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := safeDialContext(ctx, "tcp", net.JoinHostPort("8.8.8.8", "53"))
	if err != nil {
		t.Fatalf("dial público falhou: %v", err)
	}
	_ = conn.Close()
}
