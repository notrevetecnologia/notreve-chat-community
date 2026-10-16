package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

const (
	ResourceMaxCanaisWhatsApp = "max_canais_whatsapp"
	ResourceMaxCanaisTelegram = "max_canais_telegram"
	ResourceMaxCanaisWebChat  = "max_canais_webchat"
)

type PlatformChannelTypeLimits struct {
	WhatsApp int `json:"whatsapp"`
	Telegram int `json:"telegram"`
	WebChat  int `json:"webchat"`
}

type TenantChannelTypeLimitError struct {
	Type  string
	Limit int
}

func channelTypeResourceKey(channelType string) string {
	switch strings.ToLower(strings.TrimSpace(channelType)) {
	case "whatsapp":
		return ResourceMaxCanaisWhatsApp
	case "telegram":
		return ResourceMaxCanaisTelegram
	case "webchat":
		return ResourceMaxCanaisWebChat
	default:
		return ""
	}
}

func (e *TenantChannelTypeLimitError) Error() string {
	return fmt.Sprintf("tenant %s channel limit reached (%d)", e.Type, e.Limit)
}

func supportedChannelType(channelType string) bool {
	switch strings.ToLower(strings.TrimSpace(channelType)) {
	case "whatsapp", "telegram", "webchat":
		return true
	default:
		return false
	}
}

func validatePlatformChannelTypeLimits(limits PlatformChannelTypeLimits) error {
	if limits.WhatsApp < 0 || limits.Telegram < 0 || limits.WebChat < 0 {
		return ErrInvalidTenantPlanLimit
	}
	return nil
}

func (s *Store) GetPlatformChannelTypeLimits(ctx context.Context, tenantID int) (PlatformChannelTypeLimits, error) {
	var limits PlatformChannelTypeLimits
	err := s.pool.QueryRow(ctx, `SELECT max_canais_whatsapp, max_canais_telegram, max_canais_webchat
		FROM tenant_limits WHERE tenant_id=$1`, tenantID).Scan(&limits.WhatsApp, &limits.Telegram, &limits.WebChat)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlatformChannelTypeLimits{}, ErrPlatformTenantNotFound
	}
	return limits, err
}

func (s *Store) UpdatePlatformChannelTypeLimits(ctx context.Context, tenantID int, limits PlatformChannelTypeLimits) error {
	if err := validatePlatformChannelTypeLimits(limits); err != nil {
		return err
	}
	// max_conexoes_whatsapp é somente o espelho transacional usado pelo allocator.
	// A única quota de produto é max_canais_whatsapp.
	tag, err := s.pool.Exec(ctx, `UPDATE tenant_limits SET
		max_canais_whatsapp=$1, max_conexoes_whatsapp=$1,
		max_canais_telegram=$2, max_canais_webchat=$3, updated_at=now()
		WHERE tenant_id=$4`, limits.WhatsApp, limits.Telegram, limits.WebChat, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrPlatformTenantNotFound
	}
	return nil
}

func (s *Store) tenantChannelTypeLimit(ctx context.Context, tenantID int, channelType string) (int, error) {
	var limit int
	var query string
	switch strings.ToLower(strings.TrimSpace(channelType)) {
	case "whatsapp":
		query = `SELECT max_canais_whatsapp FROM tenant_limits WHERE tenant_id=$1`
	case "telegram":
		query = `SELECT max_canais_telegram FROM tenant_limits WHERE tenant_id=$1`
	case "webchat":
		query = `SELECT max_canais_webchat FROM tenant_limits WHERE tenant_id=$1`
	default:
		return 0, fmt.Errorf("unsupported channel type: %s", channelType)
	}
	if err := s.pool.QueryRow(ctx, query, tenantID).Scan(&limit); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrTenantPlanLimitsUnavailable
		}
		return 0, err
	}
	return limit, nil
}

// EffectiveChannelTypeLimit preserva o nome consumido pelos handlers, mas as
// quotas por canal são deliberadamente administradas por empresa e não fazem
// parte do overlay de planos. Isso evita duas fontes de verdade para WhatsApp.
func (s *Store) EffectiveChannelTypeLimit(ctx context.Context, channelType string) (int, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return 0, err
	}
	return s.tenantChannelTypeLimit(ctx, tenantID, channelType)
}

func enforceChannelCountLimit(kind string, count, limit int) error {
	if limit < 0 {
		return ErrInvalidTenantPlanLimit
	}
	if limit > 0 && count >= limit {
		return &TenantChannelTypeLimitError{Type: kind, Limit: limit}
	}
	return nil
}

// EnsureCanCreateChannelType aplica o teto total do plano e a quota específica
// administrada para o tipo. A capacidade física do WhatsApp é separada e fica
// exclusivamente em whatsapp_servers.max_connections.
func (s *Store) EnsureCanCreateChannelType(ctx context.Context, channelType string) error {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return err
	}
	channelType = strings.ToLower(strings.TrimSpace(channelType))
	if !supportedChannelType(channelType) {
		return fmt.Errorf("unsupported channel type: %s", channelType)
	}
	resources, err := s.resolveEffectiveResourceMap(ctx, tenantID)
	if err != nil {
		return err
	}
	typeLimit, err := s.EffectiveChannelTypeLimit(ctx, channelType)
	if err != nil {
		return err
	}
	var totalCount, typeCount int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*), COUNT(*) FILTER (WHERE type=$2)
		FROM channels WHERE tenant_id=$1`, tenantID, channelType).Scan(&totalCount, &typeCount); err != nil {
		return err
	}
	if err := enforceChannelCountLimit("total", totalCount, int(resources[ResourceMaxCanais])); err != nil {
		return err
	}
	return enforceChannelCountLimit(channelType, typeCount, typeLimit)
}
