package store

import (
	"context"
	"testing"
)

func TestTenantIDWithoutContextIsZero(t *testing.T) {
	if got := TenantID(context.Background()); got != 0 {
		t.Fatalf("TenantID() = %d, want 0", got)
	}
	if got := TenantID(WithTenantID(context.Background(), 0)); got != 0 {
		t.Fatalf("TenantID(0) = %d, want 0", got)
	}
}

func TestTenantIDRoundTripsInContext(t *testing.T) {
	if got := TenantID(WithTenantID(context.Background(), 7)); got != 7 {
		t.Fatalf("TenantID(7) = %d, want 7", got)
	}
}

func TestTenantIDRequiredRejectsMissingContext(t *testing.T) {
	if _, err := TenantIDRequired(context.Background()); err != ErrTenantContextRequired {
		t.Fatalf("TenantIDRequired(background) error = %v, want %v", err, ErrTenantContextRequired)
	}
	if _, err := TenantIDRequired(WithTenantID(context.Background(), 0)); err != ErrTenantContextRequired {
		t.Fatalf("TenantIDRequired(0) error = %v, want %v", err, ErrTenantContextRequired)
	}
}

func TestTenantIDRequiredRoundTripsInContext(t *testing.T) {
	if got, err := TenantIDRequired(WithTenantID(context.Background(), 7)); err != nil || got != 7 {
		t.Fatalf("TenantIDRequired(7) = (%d, %v), want (7, nil)", got, err)
	}
}

func TestNormalizeTenantHost(t *testing.T) {
	cases := map[string]string{
		"App.Dominio.com.BR":      "app.dominio.com.br",
		"app.dominio.com.br.":     "app.dominio.com.br",
		"app.dominio.com.br:443":  "app.dominio.com.br",
		"  app.dominio.com.br  ":  "app.dominio.com.br",
	}
	for input, want := range cases {
		if got := normalizeTenantHost(input); got != want {
			t.Fatalf("normalizeTenantHost(%q) = %q, want %q", input, got, want)
		}
	}
}
