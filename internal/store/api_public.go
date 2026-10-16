package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrInvalidPublicAPIKey = errors.New("invalid public api key")

type APIKey struct {
	ID         int64      `json:"id"`
	TenantID   int        `json:"tenant_id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Scopes     []string   `json:"scopes"`
	Active     bool       `json:"active"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type APIKeyCredential struct {
	APIKey APIKey `json:"api_key"`
	Secret string `json:"secret"`
}

func normalizeAPIKeyScopes(scopes []string) []string {
	seen := make(map[string]struct{}, len(scopes))
	result := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if len(scope) > 100 {
			continue
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		result = append(result, scope)
	}
	return result
}

func hashPublicSecret(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

func newPublicAPISecret() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "nrv_" + base64.RawURLEncoding.EncodeToString(raw), nil
}

func publicAPIKeyPrefix(secret string) (string, bool) {
	secret = strings.TrimSpace(secret)
	if !strings.HasPrefix(secret, "nrv_") || len(secret) < 16 {
		return "", false
	}
	return secret[:16], true
}

func scanAPIKey(row interface{ Scan(...interface{}) error }) (APIKey, error) {
	var key APIKey
	err := row.Scan(&key.ID, &key.TenantID, &key.Name, &key.Prefix, &key.Scopes, &key.Active,
		&key.LastUsedAt, &key.RevokedAt, &key.CreatedAt, &key.UpdatedAt)
	return key, err
}

const apiKeyColumns = `id, tenant_id, name, prefix, scopes, active, last_used_at, revoked_at, created_at, updated_at`

func (s *Store) CreateAPIKey(ctx context.Context, name string, scopes []string) (APIKeyCredential, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return APIKeyCredential{}, err
	}
	if err := s.EnsureCanCreateAPIKey(ctx); err != nil {
		return APIKeyCredential{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return APIKeyCredential{}, errors.New("api key name required")
	}
	scopes = normalizeAPIKeyScopes(scopes)
	for attempt := 0; attempt < 3; attempt++ {
		secret, err := newPublicAPISecret()
		if err != nil {
			return APIKeyCredential{}, err
		}
		prefix, _ := publicAPIKeyPrefix(secret)
		var key APIKey
		err = s.pool.QueryRow(ctx, `INSERT INTO public_api_keys
			(tenant_id, name, prefix, secret_hash, scopes)
			VALUES ($1,$2,$3,$4,$5)
			RETURNING `+apiKeyColumns, tenantID, name, prefix, hashPublicSecret(secret), scopes).Scan(
			&key.ID, &key.TenantID, &key.Name, &key.Prefix, &key.Scopes, &key.Active,
			&key.LastUsedAt, &key.RevokedAt, &key.CreatedAt, &key.UpdatedAt)
		if err == nil {
			return APIKeyCredential{APIKey: key, Secret: secret}, nil
		}
		if !isUniqueViolation(err) {
			return APIKeyCredential{}, err
		}
	}
	return APIKeyCredential{}, errors.New("could not allocate api key prefix")
}

func (s *Store) ListAPIKeys(ctx context.Context) ([]APIKey, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT `+apiKeyColumns+` FROM public_api_keys
		WHERE tenant_id=$1 ORDER BY id DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := make([]APIKey, 0)
	for rows.Next() {
		key, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (s *Store) GetAPIKey(ctx context.Context, id int64) (APIKey, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return APIKey{}, err
	}
	return scanAPIKey(s.pool.QueryRow(ctx, `SELECT `+apiKeyColumns+` FROM public_api_keys
		WHERE tenant_id=$1 AND id=$2`, tenantID, id))
}

func (s *Store) SetAPIKeyActive(ctx context.Context, id int64, active bool) error {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return err
	}
	current, err := s.GetAPIKey(ctx, id)
	if err != nil {
		return err
	}
	if active && !current.Active {
		if err := s.EnsureCanCreateAPIKey(ctx); err != nil {
			return err
		}
	}
	tag, err := s.pool.Exec(ctx, `UPDATE public_api_keys
		SET active=$1, revoked_at=CASE WHEN $1 THEN NULL ELSE COALESCE(revoked_at, now()) END, updated_at=now()
		WHERE tenant_id=$2 AND id=$3`, active, tenantID, id)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}

func (s *Store) DeleteAPIKey(ctx context.Context, id int64) error {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM public_api_keys WHERE tenant_id=$1 AND id=$2`, tenantID, id)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}

func (s *Store) AuthenticateAPIKey(ctx context.Context, secret string) (APIKey, error) {
	prefix, ok := publicAPIKeyPrefix(secret)
	if !ok {
		return APIKey{}, ErrInvalidPublicAPIKey
	}
	lookupCtx := WithGlobalDataAccess(ctx)
	var key APIKey
	err := s.pool.QueryRow(lookupCtx, `SELECT `+apiKeyColumns+`
		FROM public_api_keys
		WHERE prefix=$1 AND secret_hash=$2 AND active=true AND revoked_at IS NULL`, prefix, hashPublicSecret(secret)).Scan(
		&key.ID, &key.TenantID, &key.Name, &key.Prefix, &key.Scopes, &key.Active,
		&key.LastUsedAt, &key.RevokedAt, &key.CreatedAt, &key.UpdatedAt)
	if err != nil {
		return APIKey{}, ErrInvalidPublicAPIKey
	}
	tenantCtx := WithTenantID(ctx, key.TenantID)
	_, _ = s.pool.Exec(tenantCtx, `UPDATE public_api_keys SET last_used_at=now(), updated_at=now()
		WHERE tenant_id=$1 AND id=$2 AND active=true`, key.TenantID, key.ID)
	return key, nil
}

func isUniqueViolation(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "duplicate key") || strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}

func sameSecret(left, right string) bool {
	return subtle.ConstantTimeCompare(hashPublicSecret(left), hashPublicSecret(right)) == 1
}
