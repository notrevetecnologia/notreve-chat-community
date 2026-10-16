package store

import (
	"context"
	"encoding/json"
	"strings"
)

type PlatformAuditEntry struct {
	ActorID        *int
	ActorName      string
	Action         string
	Path           string
	TargetTenantID *int
	HTTPStatus     int
	IP             string
	Metadata       map[string]interface{}
}

func (s *Store) RecordPlatformAudit(ctx context.Context, entry PlatformAuditEntry) error {
	ctx = WithGlobalDataAccess(ctx)
	entry.ActorName = strings.TrimSpace(entry.ActorName)
	entry.Action = strings.TrimSpace(entry.Action)
	entry.Path = strings.TrimSpace(entry.Path)
	entry.IP = strings.TrimSpace(entry.IP)
	if entry.Action == "" || entry.Path == "" {
		return nil
	}
	metadata := []byte(`{}`)
	if len(entry.Metadata) > 0 {
		if encoded, err := json.Marshal(entry.Metadata); err == nil {
			metadata = encoded
		}
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO platform_audit_log
		(actor_id, actor_name, action, path, target_tenant_id, http_status, ip, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb)`,
		entry.ActorID, entry.ActorName, entry.Action, entry.Path, entry.TargetTenantID,
		entry.HTTPStatus, entry.IP, metadata)
	return err
}
