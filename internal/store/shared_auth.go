package store

import (
	"context"
	"time"
)

type SessionUserState struct {
	UserID               int
	TenantID             int
	Role                 string
	Active               bool
	TenantStatus         string
	MustChangePassword   bool
	MFAEnabled           bool
	CredentialsChangedAt time.Time
}

func (s *Store) GetUserByIDGlobal(ctx context.Context, userID int) (Usuario, error) {
	ctx = WithGlobalDataAccess(ctx)
	return scanUsuario(s.pool.QueryRow(ctx,
		`SELECT u.id, u.tenant_id, u.name, u.username, u.password_hash, u.role, u.permissions, u.atendente_id, u.active, u.created_at
		 FROM usuarios u
		 JOIN tenants t ON t.id=u.tenant_id
		 WHERE u.id=$1 AND (t.status IN ('active','trial') OR u.role='super_admin')
		 LIMIT 1`, userID))
}

func (s *Store) GetSessionUserStateByID(ctx context.Context, userID int) (SessionUserState, error) {
	ctx = WithGlobalDataAccess(ctx)
	var state SessionUserState
	err := s.pool.QueryRow(ctx,
		`SELECT u.id, u.tenant_id, u.role, u.active, t.status, u.must_change_password, u.totp_enabled, u.credentials_changed_at
		 FROM usuarios u
		 JOIN tenants t ON t.id=u.tenant_id
		 WHERE u.id=$1
		 LIMIT 1`, userID).Scan(
		&state.UserID,
		&state.TenantID,
		&state.Role,
		&state.Active,
		&state.TenantStatus,
		&state.MustChangePassword,
		&state.MFAEnabled,
		&state.CredentialsChangedAt,
	)
	return state, err
}
