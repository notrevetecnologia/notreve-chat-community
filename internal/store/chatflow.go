package store

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

const chatflowCols = `id, tenant_id, nome, version, ativo, config, updated_at`
const chatflowStepCols = `id, chatflow_id, ord, tipo, config, created_at`

func (s *Store) ListChatflows(ctx context.Context) ([]Chatflow, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+chatflowCols+` FROM chatflow WHERE tenant_id=$1 ORDER BY nome ASC`, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Chatflow
	for rows.Next() {
		c, err := scanChatflow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetChatflow(ctx context.Context, id int) (Chatflow, error) {
	return scanChatflow(s.pool.QueryRow(ctx, `SELECT `+chatflowCols+` FROM chatflow WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id))
}

func (s *Store) CreateChatflow(ctx context.Context, c Chatflow) (int, error) {
	version := c.Version
	if version == 0 {
		version = 1
		if err := s.pool.QueryRow(ctx, `SELECT COALESCE(MAX(version),0)+1 FROM chatflow WHERE tenant_id=$1`, TenantID(ctx)).Scan(&version); err != nil {
			return 0, err
		}
	}
	var id int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO chatflow (tenant_id, nome, version, ativo, config) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		TenantID(ctx), c.Nome, version, c.Ativo, c.Config).Scan(&id)
	return id, err
}

func (s *Store) UpdateChatflow(ctx context.Context, c Chatflow) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE chatflow SET nome=$1, version=$2, config=$3, updated_at=now() WHERE tenant_id=$5 AND id=$4`,
		c.Nome, c.Version, c.Config, c.ID, TenantID(ctx))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) ActivateChatflow(ctx context.Context, id int, ativo bool) error {
	tag, err := s.pool.Exec(ctx, `UPDATE chatflow SET ativo=$1, updated_at=now() WHERE tenant_id=$3 AND id=$2`, ativo, id, TenantID(ctx))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) DeleteChatflow(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM chatflow WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id)
	return err
}

func (s *Store) ListChatflowSteps(ctx context.Context, chatflowID int) ([]ChatflowStep, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+chatflowStepCols+` FROM chatflow_steps WHERE chatflow_id=$1 ORDER BY ord ASC`, chatflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChatflowStep
	for rows.Next() {
		step, err := scanChatflowStep(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, step)
	}
	return out, rows.Err()
}

func (s *Store) CreateChatflowStep(ctx context.Context, step ChatflowStep) (int, error) {
	if !s.chatflowExists(ctx, TenantID(ctx), step.ChatflowID) {
		return 0, pgx.ErrNoRows
	}
	var id int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO chatflow_steps (chatflow_id, ord, tipo, config) VALUES ($1,$2,$3,$4) RETURNING id`,
		step.ChatflowID, step.Ord, step.Tipo, step.Config).Scan(&id)
	return id, err
}

func (s *Store) UpdateChatflowStep(ctx context.Context, step ChatflowStep) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE chatflow_steps SET ord=$1, tipo=$2, config=$3
		 WHERE id=$4 AND chatflow_id=$5 AND chatflow_id IN (SELECT id FROM chatflow WHERE tenant_id=$6)`,
		step.Ord, step.Tipo, step.Config, step.ID, step.ChatflowID, TenantID(ctx))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) DeleteChatflowStep(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM chatflow_steps WHERE id=$1 AND chatflow_id IN (SELECT id FROM chatflow WHERE tenant_id=$2)`,
		id, TenantID(ctx))
	return err
}

func (s *Store) SetChannelChatflow(ctx context.Context, channelID int, chatflowID *int) error {
	if chatflowID != nil {
		if !s.chatflowExists(ctx, TenantID(ctx), *chatflowID) {
			return pgx.ErrNoRows
		}
	}
	mode := "ai"
	if chatflowID != nil {
		mode = "chatflow"
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE channels SET chatflow_id=$1, mode=$2 WHERE tenant_id=$4 AND id=$3`, chatflowID, mode, channelID, TenantID(ctx))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) ResolveActiveChatflow(ctx context.Context, chatflowID *int) (*Chatflow, []ChatflowStep, error) {
	if chatflowID == nil {
		return nil, nil, nil
	}
	flow, err := scanChatflow(s.pool.QueryRow(ctx,
		`SELECT `+chatflowCols+` FROM chatflow WHERE tenant_id=$1 AND id=$2 AND ativo=true`, TenantID(ctx), *chatflowID))
	if err == pgx.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	steps, err := s.ListChatflowSteps(ctx, flow.ID)
	if err != nil {
		return nil, nil, err
	}
	return &flow, steps, nil
}

func (s *Store) SetConversationChatflowState(ctx context.Context, convID int, chatflowID int, stepOrd *int, vars map[string]string) error {
	bytes, _ := json.Marshal(vars)
	_, err := s.pool.Exec(ctx,
		`UPDATE conversations SET chatflow_id=$1, chatflow_step=$2, chatflow_vars=$3, updated_at=now()
		 WHERE tenant_id=$4 AND id=$5`,
		chatflowID, stepOrd, bytes, TenantID(ctx), convID)
	return err
}

func (s *Store) GetConversationChatflowState(ctx context.Context, convID int) (ConversationChatflowState, error) {
	var state ConversationChatflowState
	var raw []byte
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(chatflow_id,0), chatflow_step, COALESCE(chatflow_vars,'{}')::text
		 FROM conversations WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), convID).
		Scan(&state.ChatflowID, &state.StepOrd, &raw)
	if err != nil {
		return state, err
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &state.Vars)
	}
	return state, nil
}

func (s *Store) ClearConversationChatflow(ctx context.Context, convID int) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE conversations SET chatflow_id=NULL, chatflow_step=NULL, chatflow_vars=NULL, updated_at=now()
		 WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), convID)
	return err
}

func (s *Store) chatflowExists(ctx context.Context, tenantID, id int) bool {
	var exists bool
	_ = s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM chatflow WHERE tenant_id=$1 AND id=$2)`, tenantID, id).Scan(&exists)
	return exists
}

func scanChatflow(row pgx.Row) (Chatflow, error) {
	var c Chatflow
	err := row.Scan(&c.ID, &c.TenantID, &c.Nome, &c.Version, &c.Ativo, &c.Config, &c.UpdatedAt)
	return c, err
}

func scanChatflowStep(row pgx.Row) (ChatflowStep, error) {
	var step ChatflowStep
	err := row.Scan(&step.ID, &step.ChatflowID, &step.Ord, &step.Tipo, &step.Config, &step.CreatedAt)
	return step, err
}
