package store

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

var ErrUserPasswordChangeFailed = errors.New("user password change failed")

func (s *Store) UserMustChangePassword(ctx context.Context, userID int) (bool, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return false, err
	}
	var mustChange bool
	err = s.pool.QueryRow(ctx,
		`SELECT must_change_password FROM usuarios WHERE tenant_id=$1 AND id=$2 AND active`,
		tenantID, userID,
	).Scan(&mustChange)
	return mustChange, err
}

func (s *Store) ChangeOwnPassword(ctx context.Context, userID int, passwordHash string) error {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return err
	}
	passwordHash = strings.TrimSpace(passwordHash)
	if passwordHash == "" {
		return ErrUserPasswordChangeFailed
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE usuarios SET password_hash=$1 WHERE tenant_id=$2 AND id=$3 AND active`,
			passwordHash, tenantID, userID,
		)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrUserPasswordChangeFailed
		}
		_, err = tx.Exec(ctx,
			`UPDATE usuarios SET must_change_password=false WHERE tenant_id=$1 AND id=$2`,
			tenantID, userID,
		)
		return err
	})
}
