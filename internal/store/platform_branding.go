package store

import (
	"context"
	"time"
)

// PlatformBranding é a identidade visual global da plataforma (Notreve,
// super_admin). É independente de tenant e aparece no painel/login.
type PlatformBranding struct {
	Name            string    `json:"name"`
	LogoURL         string    `json:"logo_url"`
	FaviconURL      string    `json:"favicon_url"`
	PrimaryColor    string    `json:"primary_color"`
	SecondaryColor  string    `json:"secondary_color"`
	AccentColor     string    `json:"accent_color"`
	BackgroundColor string    `json:"background_color"`
	LoadingColor    string    `json:"loading_color"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (s *Store) GetPlatformBranding(ctx context.Context) (PlatformBranding, error) {
	var b PlatformBranding
	err := s.pool.QueryRow(ctx, `SELECT name, logo_url, favicon_url, primary_color, secondary_color, accent_color, background_color, loading_color, updated_at
		FROM platform_branding WHERE id = 1`).Scan(&b.Name, &b.LogoURL, &b.FaviconURL, &b.PrimaryColor, &b.SecondaryColor,
		&b.AccentColor, &b.BackgroundColor, &b.LoadingColor, &b.UpdatedAt)
	return b, err
}

func (s *Store) SetPlatformBranding(ctx context.Context, b PlatformBranding) error {
	_, err := s.pool.Exec(ctx, `UPDATE platform_branding SET
		name=$1, logo_url=$2, favicon_url=$3, primary_color=$4, secondary_color=$5, accent_color=$6,
		background_color=$7, loading_color=$8, updated_at=now() WHERE id = 1`,
		b.Name, b.LogoURL, b.FaviconURL, b.PrimaryColor, b.SecondaryColor, b.AccentColor,
		b.BackgroundColor, b.LoadingColor)
	return err
}
