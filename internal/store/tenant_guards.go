package store

import (
	"context"
	"fmt"
)

func (s *Store) validateConversationReferences(ctx context.Context, tenantID int, c Conversation) error {
	if c.ChannelID != nil {
		var exists bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM channels WHERE tenant_id=$1 AND id=$2)`, tenantID, *c.ChannelID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("channel does not belong to tenant")
		}
	}
	if c.AgentID != nil {
		var exists bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM agents WHERE tenant_id=$1 AND id=$2)`, tenantID, *c.AgentID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("agent does not belong to tenant")
		}
	}
	if c.SetorID != nil {
		if err := s.validateSetorReference(ctx, tenantID, c.SetorID); err != nil {
			return err
		}
	}
	if c.AtendenteID != nil {
		if err := s.validateAtendenteReference(ctx, tenantID, c.AtendenteID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) validateSetorReference(ctx context.Context, tenantID int, setorID *int) error {
	if setorID == nil {
		return nil
	}
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM setores WHERE tenant_id=$1 AND id=$2)`, tenantID, *setorID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("sector does not belong to tenant")
	}
	return nil
}

func (s *Store) validateSetorReferences(ctx context.Context, tenantID int, setorIDs []int) error {
	ids := uniqueInts(setorIDs)
	if len(ids) == 0 {
		return nil
	}
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM setores WHERE tenant_id=$1 AND id=ANY($2)`, tenantID, ids).Scan(&count); err != nil {
		return err
	}
	if count != len(ids) {
		return fmt.Errorf("sector does not belong to tenant")
	}
	return nil
}

func (s *Store) validateProviderReference(ctx context.Context, tenantID int, providerID *int) error {
	if providerID == nil {
		return nil
	}
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM providers WHERE tenant_id=$1 AND id=$2)`, tenantID, *providerID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("provider does not belong to tenant")
	}
	return nil
}

func (s *Store) validateIntegrationReferences(ctx context.Context, tenantID int, integrationIDs []int) error {
	ids := uniqueInts(integrationIDs)
	if len(ids) == 0 {
		return nil
	}
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM integrations WHERE tenant_id=$1 AND id=ANY($2)`, tenantID, ids).Scan(&count); err != nil {
		return err
	}
	if count != len(ids) {
		return fmt.Errorf("tool does not belong to tenant")
	}
	return nil
}

func (s *Store) validateAtendenteProfileReference(ctx context.Context, tenantID int, atendenteID *int) error {
	if atendenteID == nil {
		return nil
	}
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM atendentes WHERE tenant_id=$1 AND id=$2)`, tenantID, *atendenteID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("attendant profile does not belong to tenant")
	}
	return nil
}

func (s *Store) validateAtendenteReference(ctx context.Context, tenantID int, userID *int) error {
	if userID == nil {
		return nil
	}
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM usuarios u
		LEFT JOIN atendentes a ON a.tenant_id=u.tenant_id AND a.id=u.atendente_id
		WHERE u.tenant_id=$1 AND u.id=$2 AND u.active
		  AND (u.role IN ('admin','super_admin') OR (u.atendente_id IS NOT NULL AND a.active))
	)`, tenantID, *userID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("attendant does not belong to tenant")
	}
	return nil
}

func (s *Store) validateChannelReference(ctx context.Context, tenantID, channelID int) error {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM channels WHERE tenant_id=$1 AND id=$2)`, tenantID, channelID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("channel does not belong to tenant")
	}
	return nil
}

func (s *Store) validateChannelReferences(ctx context.Context, tenantID int, channelIDs []int) error {
	ids := uniqueInts(channelIDs)
	if len(ids) == 0 {
		return nil
	}
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM channels WHERE tenant_id=$1 AND id=ANY($2)`, tenantID, ids).Scan(&count); err != nil {
		return err
	}
	if count != len(ids) {
		return fmt.Errorf("channel does not belong to tenant")
	}
	return nil
}

func (s *Store) validateContactReference(ctx context.Context, tenantID, contactID int) error {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM contatos WHERE tenant_id=$1 AND id=$2)`, tenantID, contactID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("contact does not belong to tenant")
	}
	return nil
}

func tenantJobID(ctx context.Context) (int, error) {
	return TenantIDRequired(ctx)
}

func uniqueInts(values []int) []int {
	seen := make(map[int]struct{}, len(values))
	result := make([]int, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
