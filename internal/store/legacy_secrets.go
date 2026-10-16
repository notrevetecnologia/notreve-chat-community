package store

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/notrevetecnologia/notreve-chat-community/internal/securesecret"
)

// MigrateLegacySecrets is intentionally called only by the migration job. It
// upgrades credentials stored by older versions without exposing or changing
// their logical values. Encrypted values are left untouched, so the operation
// is safe to repeat after an interrupted deployment.
func (s *Store) MigrateLegacySecrets(ctx context.Context) error {
	ctx = WithGlobalDataAccess(ctx)
	if err := securesecret.ValidateKey(); err != nil {
		return err
	}
	if err := s.migrateLegacySettings(ctx); err != nil {
		return fmt.Errorf("migrating settings secrets: %w", err)
	}
	if err := s.migrateLegacyProviders(ctx); err != nil {
		return fmt.Errorf("migrating provider secrets: %w", err)
	}
	if err := s.migrateLegacyChannels(ctx); err != nil {
		return fmt.Errorf("migrating channel secrets: %w", err)
	}
	if err := s.migrateLegacyIntegrations(ctx); err != nil {
		return fmt.Errorf("migrating integration secrets: %w", err)
	}
	if err := s.migrateLegacyWhatsAppServers(ctx); err != nil {
		return fmt.Errorf("migrating WhatsApp server secrets: %w", err)
	}
	if err := s.migrateLegacyMFASecrets(ctx); err != nil {
		return fmt.Errorf("migrating MFA secrets: %w", err)
	}
	return nil
}

func migrateLegacySecret(scope, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || securesecret.IsEncrypted(value) {
		return value, nil
	}
	return securesecret.Encrypt(scope, value)
}

func (s *Store) migrateLegacySettings(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `SELECT id, default_api_key, whatsapp_token, whatsapp_admin_token, stt_api_key FROM settings`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type row struct {
		id                            int
		defaultAPIKey, whatsappToken  string
		whatsappAdminToken, sttAPIKey string
	}
	var all []row
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.id, &item.defaultAPIKey, &item.whatsappToken, &item.whatsappAdminToken, &item.sttAPIKey); err != nil {
			return err
		}
		all = append(all, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, item := range all {
		values := []struct {
			scope string
			value string
		}{
			{scope: "settings.default_api_key", value: item.defaultAPIKey},
			{scope: "settings.whatsapp_token", value: item.whatsappToken},
			{scope: "settings.whatsapp_admin_token", value: item.whatsappAdminToken},
			{scope: "settings.stt_api_key", value: item.sttAPIKey},
		}
		changed := false
		for i := range values {
			migrated, err := migrateLegacySecret(values[i].scope, values[i].value)
			if err != nil {
				return err
			}
			if migrated != values[i].value {
				changed = true
			}
			values[i].value = migrated
		}
		if !changed {
			continue
		}
		if _, err := s.pool.Exec(ctx, `UPDATE settings SET default_api_key=$1, whatsapp_token=$2, whatsapp_admin_token=$3, stt_api_key=$4 WHERE id=$5`, values[0].value, values[1].value, values[2].value, values[3].value, item.id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) migrateLegacyProviders(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `SELECT id, api_key FROM providers`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type row struct {
		id  int
		key string
	}
	var all []row
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.id, &item.key); err != nil {
			return err
		}
		all = append(all, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, item := range all {
		migrated, err := migrateLegacySecret("providers.api_key", item.key)
		if err != nil {
			return err
		}
		if migrated == strings.TrimSpace(item.key) {
			continue
		}
		if _, err := s.pool.Exec(ctx, `UPDATE providers SET api_key=$1 WHERE id=$2`, migrated, item.id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) migrateLegacyChannels(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `SELECT id, token, token_hash FROM channels`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type row struct {
		id        int
		token     string
		tokenHash []byte
	}
	var all []row
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.id, &item.token, &item.tokenHash); err != nil {
			return err
		}
		all = append(all, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, item := range all {
		original := strings.TrimSpace(item.token)
		plain := original
		migrated, err := migrateLegacySecret("channels.token", original)
		if err != nil {
			return err
		}
		hash := item.tokenHash
		if securesecret.IsEncrypted(plain) && len(hash) == 0 && plain != "" {
			plain, err = securesecret.Decrypt("channels.token", plain)
			if err != nil {
				return err
			}
			hash = securesecret.LookupHash(plain)
		}
		if plain != "" && !securesecret.IsEncrypted(plain) {
			hash = securesecret.LookupHash(plain)
		}
		if migrated == original && bytes.Equal(hash, item.tokenHash) {
			continue
		}
		if _, err := s.pool.Exec(ctx, `UPDATE channels SET token=$1, token_hash=$2 WHERE id=$3`, migrated, hash, item.id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) migrateLegacyIntegrations(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `SELECT id, headers FROM integrations`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type row struct {
		id      int
		headers string
	}
	var all []row
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.id, &item.headers); err != nil {
			return err
		}
		all = append(all, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, item := range all {
		migrated, err := transformIntegrationHeaders(item.headers, false)
		if err != nil {
			return err
		}
		if migrated == item.headers {
			continue
		}
		if _, err := s.pool.Exec(ctx, `UPDATE integrations SET headers=$1 WHERE id=$2`, migrated, item.id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) migrateLegacyWhatsAppServers(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `SELECT id, token FROM whatsapp_servers`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type row struct {
		id    int64
		token string
	}
	var all []row
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.id, &item.token); err != nil {
			return err
		}
		all = append(all, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, item := range all {
		migrated, err := migrateLegacySecret("whatsapp_servers.token", item.token)
		if err != nil {
			return err
		}
		if migrated == strings.TrimSpace(item.token) {
			continue
		}
		if _, err := s.pool.Exec(ctx, `UPDATE whatsapp_servers SET token=$1 WHERE id=$2`, migrated, item.id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) migrateLegacyMFASecrets(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `SELECT id, totp_secret FROM usuarios WHERE totp_secret <> ''`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type row struct {
		id     int
		secret string
	}
	var all []row
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.id, &item.secret); err != nil {
			return err
		}
		all = append(all, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, item := range all {
		migrated, err := migrateLegacySecret("usuarios.totp_secret", item.secret)
		if err != nil {
			return err
		}
		if migrated == strings.TrimSpace(item.secret) {
			continue
		}
		if _, err := s.pool.Exec(ctx, `UPDATE usuarios SET totp_secret=$1 WHERE id=$2`, migrated, item.id); err != nil {
			return err
		}
	}
	return nil
}
