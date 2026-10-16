package store

import "testing"

func TestNormalizeAPIKeyScopes(t *testing.T) {
	got := normalizeAPIKeyScopes([]string{" conversations:read ", "", "conversations:read", "contacts:write"})
	want := []string{"conversations:read", "contacts:write"}
	if len(got) != len(want) {
		t.Fatalf("scopes length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("scope[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPublicAPISecretPrefixAndHash(t *testing.T) {
	secret, err := newPublicAPISecret()
	if err != nil {
		t.Fatal(err)
	}
	prefix, ok := publicAPIKeyPrefix(secret)
	if !ok || len(prefix) != 16 {
		t.Fatalf("prefix = %q, valid = %v", prefix, ok)
	}
	if !sameSecret(secret, secret) || sameSecret(secret, secret+"x") {
		t.Fatal("secret hash comparison failed")
	}
	if len(hashPublicSecret(secret)) != 32 {
		t.Fatal("secret hash must be 32 bytes")
	}
}

func TestPublicAPIKeyPrefixRejectsInvalidSecret(t *testing.T) {
	if prefix, ok := publicAPIKeyPrefix("plain-secret"); ok || prefix != "" {
		t.Fatalf("invalid prefix accepted: %q", prefix)
	}
}
