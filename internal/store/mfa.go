package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/notrevetecnologia/notreve-chat-community/internal/securesecret"
)

type MFAState struct {
	Enabled     bool
	Secret      string
	ConfirmedAt *time.Time
}

const totpSecretScope = "usuarios.totp_secret"

func (s *Store) GetMFAStateByUserID(ctx context.Context, userID int) (MFAState, error) {
	ctx = WithGlobalDataAccess(ctx)
	var state MFAState
	var encrypted string
	err := s.pool.QueryRow(ctx, `SELECT totp_enabled, totp_secret, totp_confirmed_at
		FROM usuarios WHERE id=$1 AND active=true`, userID).
		Scan(&state.Enabled, &encrypted, &state.ConfirmedAt)
	if err != nil {
		return MFAState{}, err
	}
	if strings.TrimSpace(encrypted) != "" {
		state.Secret, err = securesecret.Decrypt(totpSecretScope, encrypted)
		if err != nil {
			return MFAState{}, err
		}
	}
	return state, nil
}

func (s *Store) SetPendingMFASecret(ctx context.Context, userID int, secret string) error {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return errors.New("segredo TOTP obrigatório")
	}
	encrypted, err := securesecret.Encrypt(totpSecretScope, secret)
	if err != nil {
		return err
	}
	ctx = WithGlobalDataAccess(ctx)
	tag, err := s.pool.Exec(ctx, `UPDATE usuarios
		SET totp_secret=$1, totp_enabled=false, totp_confirmed_at=NULL
		WHERE id=$2 AND active=true`, encrypted, userID)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}

func (s *Store) EnableMFA(ctx context.Context, userID int) error {
	ctx = WithGlobalDataAccess(ctx)
	tag, err := s.pool.Exec(ctx, `UPDATE usuarios
		SET totp_enabled=true, totp_confirmed_at=now(), credentials_changed_at=now()
		WHERE id=$1 AND active=true AND totp_secret <> ''`, userID)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}

func (s *Store) DisableMFA(ctx context.Context, userID int) error {
	ctx = WithGlobalDataAccess(ctx)
	tag, err := s.pool.Exec(ctx, `UPDATE usuarios
		SET totp_enabled=false, totp_secret='', totp_confirmed_at=NULL, credentials_changed_at=now()
		WHERE id=$1 AND active=true`, userID)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}

func (s *Store) ResetMFAForUsername(ctx context.Context, username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("usuário obrigatório")
	}
	ctx = WithGlobalDataAccess(ctx)
	tag, err := s.pool.Exec(ctx, `UPDATE usuarios
		SET totp_enabled=false, totp_secret='', totp_confirmed_at=NULL, credentials_changed_at=now()
		WHERE lower(username)=lower($1)`, username)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}
