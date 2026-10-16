package store

import (
	"context"
	"errors"
	"net"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/notrevetecnologia/notreve-chat-community/internal/tenantctx"
)

type resolvedTenantContextKey struct{}

type ResolvedTenant struct {
	ID     int
	Status string
}

// SystemTenantID é o tenant de sistema que abriga o super_admin. Não é uma
// empresa-cliente e não aparece no control plane.
const SystemTenantID = 1

var ErrTenantContextRequired = errors.New("tenant context required")
var ErrTenantNotFound = errors.New("tenant not found for host")

func tenantContextID(ctx context.Context) (int, bool) {
	return tenantctx.ID(ctx)
}

func WithTenantID(ctx context.Context, tenantID int) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return tenantctx.WithID(ctx, tenantID)
}

func WithGlobalDataAccess(ctx context.Context) context.Context {
	return tenantctx.WithRLSBypass(ctx)
}

func WithResolvedTenant(ctx context.Context, tenantID int, status string) context.Context {
	if ctx == nil || tenantID < 1 {
		return ctx
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		status = "active"
	}
	ctx = tenantctx.WithID(ctx, tenantID)
	return context.WithValue(ctx, resolvedTenantContextKey{}, ResolvedTenant{ID: tenantID, Status: status})
}

func resolvedTenant(ctx context.Context) (ResolvedTenant, bool) {
	if ctx == nil {
		return ResolvedTenant{}, false
	}
	resolved, ok := ctx.Value(resolvedTenantContextKey{}).(ResolvedTenant)
	return resolved, ok && resolved.ID > 0
}

func TenantID(ctx context.Context) int {
	if tenantID, ok := tenantContextID(ctx); ok {
		return tenantID
	}
	return 0
}

func TenantIDRequired(ctx context.Context) (int, error) {
	tenantID, ok := tenantContextID(ctx)
	if !ok || tenantID < 1 {
		return 0, ErrTenantContextRequired
	}
	return tenantID, nil
}

func normalizeTenantHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	return strings.TrimSuffix(host, ".")
}

func (s *Store) ResolveTenantForHost(ctx context.Context, host string) (int, string, error) {
	if forced, ok := resolvedTenant(ctx); ok {
		return forced.ID, forced.Status, nil
	}
	host = normalizeTenantHost(host)
	if host == "" {
		return 0, "", ErrTenantNotFound
	}
	var id int
	var status string
	err := s.pool.QueryRow(WithGlobalDataAccess(ctx), `SELECT t.id, t.status FROM tenant_domains d JOIN tenants t ON t.id=d.tenant_id WHERE lower(d.hostname)=lower($1) LIMIT 1`, host).Scan(&id, &status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, "", ErrTenantNotFound
		}
		return 0, "", err
	}
	return id, status, nil
}

func (s *Store) ListActiveTenantIDs(ctx context.Context) ([]int, error) {
	rows, err := s.pool.Query(WithGlobalDataAccess(ctx), `SELECT id FROM tenants WHERE status='active' ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]int, 0)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
