package web

import (
	"context"
	"testing"
	"time"
)

func TestSessionWireRoundTrip(t *testing.T) {
	started := time.Unix(1735689600, 123456789)
	value := encodeSessionValue(77, 9, "super_admin", true, started)
	entry, decoded, ok := decodeSessionValue(value)
	if !ok {
		t.Fatal("sessão codificada deveria ser válida")
	}
	if entry.userID != 77 || entry.tenantID != 9 || entry.role != "super_admin" || !entry.mfaVerified {
		t.Fatalf("identidade decodificada inesperada: %+v", entry)
	}
	if !decoded.Equal(started) {
		t.Fatalf("started = %v, esperado %v", decoded, started)
	}
}

func TestOldSessionWireIsRejected(t *testing.T) {
	for _, value := range []string{"77", "77|123", "77|9|admin|123"} {
		if _, _, ok := decodeSessionValue(value); ok {
			t.Fatalf("formato antigo aceito: %q", value)
		}
	}
}

func TestInvalidSessionWire(t *testing.T) {
	for _, value := range []string{"", "0|9|admin|true|1", "77|0|admin|true|1", "77|9||true|1", "77|9|admin|x|1", "77|9|admin|true|0"} {
		if _, _, ok := decodeSessionValue(value); ok {
			t.Fatalf("sessão inválida aceita: %q", value)
		}
	}
}

func TestLegacySessionSetCannotAuthenticate(t *testing.T) {
	store := newSessionStore(nil, time.Minute)
	store.Set(context.Background(), "legacy", 77)
	if _, ok := store.Identity(context.Background(), "legacy"); ok {
		t.Fatal("sessão legada sem tenant e role não pode autenticar")
	}
	if _, ok := store.Get(context.Background(), "legacy"); ok {
		t.Fatal("Get não pode aceitar sessão legada incompleta")
	}
}

func TestCanonicalSessionIdentityAuthenticates(t *testing.T) {
	store := newSessionStore(nil, time.Minute)
	store.SetIdentity(context.Background(), "canonical", 77, 9, "admin", true)
	identity, ok := store.Identity(context.Background(), "canonical")
	if !ok {
		t.Fatal("sessão canônica deveria autenticar")
	}
	if identity.UserID != 77 || identity.TenantID != 9 || identity.Role != "admin" || !identity.MFAVerified {
		t.Fatalf("identidade inesperada: %+v", identity)
	}
}
