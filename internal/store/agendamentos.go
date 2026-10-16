package store

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const agendamentoCols = `id, tenant_id, nome, descricao, tipo, quando, acao, payload, status,
	max_execucoes, execucoes, proxima_execucao, ultima_execucao, resultado, erro,
	canal_id, contato_id, telefone, agente_id, criado_por, created_at, updated_at`

type agendamentoScanner interface {
	Scan(dest ...interface{}) error
}

func scanAgendamento(scanner agendamentoScanner) (Agendamento, error) {
	var a Agendamento
	var payload []byte
	err := scanner.Scan(&a.ID, &a.TenantID, &a.Nome, &a.Descricao, &a.Tipo, &a.Quando, &a.Acao, &payload, &a.Status,
		&a.MaxExecucoes, &a.Execucoes, &a.ProximaExecucao, &a.UltimaExecucao, &a.Resultado, &a.Erro,
		&a.CanalID, &a.ContatoID, &a.Telefone, &a.AgenteID, &a.CriadoPor, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return a, err
	}
	if len(payload) == 0 {
		payload = []byte("{}")
	}
	a.Payload = payload
	return a, nil
}

func agendamentoFilterClause(args []interface{}, status, tipo, acao, q string) (string, []interface{}) {
	query := ""
	if status != "" {
		args = append(args, status)
		query += ` AND status=$` + strconv.Itoa(len(args))
	}
	if tipo != "" {
		args = append(args, tipo)
		query += ` AND tipo=$` + strconv.Itoa(len(args))
	}
	if acao != "" {
		args = append(args, acao)
		query += ` AND acao=$` + strconv.Itoa(len(args))
	}
	if q != "" {
		args = append(args, "%"+q+"%")
		query += ` AND (lower(nome) LIKE lower($` + strconv.Itoa(len(args)) + `) OR telefone LIKE $` + strconv.Itoa(len(args)) + `)`
	}
	return query, args
}

func (s *Store) ListAgendamentosFilteredForTenant(ctx context.Context, status, tipo, acao, q string, limit, offset int) ([]Agendamento, error) {
	args := []interface{}{TenantID(ctx)}
	filter, args := agendamentoFilterClause(args, status, tipo, acao, q)
	query := `SELECT ` + agendamentoCols + ` FROM agendamentos WHERE tenant_id=$1` + filter
	args = append(args, limit, offset)
	query += ` ORDER BY created_at DESC, id DESC LIMIT $` + strconv.Itoa(len(args)-1) + ` OFFSET $` + strconv.Itoa(len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Agendamento
	for rows.Next() {
		a, err := scanAgendamento(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) CountAgendamentosForTenant(ctx context.Context, status, tipo, acao, q string) (int, error) {
	args := []interface{}{TenantID(ctx)}
	filter, args := agendamentoFilterClause(args, status, tipo, acao, q)
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM agendamentos WHERE tenant_id=$1`+filter, args...).Scan(&n)
	return n, err
}

func (s *Store) GetAgendamentoForTenant(ctx context.Context, id int) (Agendamento, error) {
	return scanAgendamento(s.pool.QueryRow(ctx, `SELECT `+agendamentoCols+` FROM agendamentos WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id))
}

func (s *Store) validateAgendamentoReferences(ctx context.Context, tenantID int, a Agendamento) error {
	if a.CanalID != nil {
		if err := s.validateChannelReference(ctx, tenantID, *a.CanalID); err != nil {
			return err
		}
	}
	if a.ContatoID != nil {
		if err := s.validateContactReference(ctx, tenantID, *a.ContatoID); err != nil {
			return err
		}
	}
	if a.AgenteID != nil {
		if err := s.validateAgentReferences(ctx, tenantID, []int{*a.AgenteID}); err != nil {
			return err
		}
	}
	return nil
}

func normalizeAgendamentoPayload(payload []byte) []byte {
	if len(strings.TrimSpace(string(payload))) == 0 {
		return []byte("{}")
	}
	return payload
}

func (s *Store) CreateAgendamentoForTenant(ctx context.Context, a Agendamento) (int, error) {
	tenantID := TenantID(ctx)
	if err := s.validateAgendamentoReferences(ctx, tenantID, a); err != nil {
		return 0, err
	}
	var id int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO agendamentos (tenant_id, nome, descricao, tipo, quando, acao, payload, status,
			max_execucoes, execucoes, proxima_execucao, canal_id, contato_id, telefone, agente_id, criado_por)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16) RETURNING id`,
		tenantID, a.Nome, a.Descricao, a.Tipo, a.Quando, a.Acao, normalizeAgendamentoPayload(a.Payload), a.Status,
		a.MaxExecucoes, a.Execucoes, a.ProximaExecucao, a.CanalID, a.ContatoID, a.Telefone, a.AgenteID, a.CriadoPor).Scan(&id)
	return id, err
}

func (s *Store) UpdateAgendamentoForTenant(ctx context.Context, a Agendamento) error {
	tenantID := TenantID(ctx)
	if err := s.validateAgendamentoReferences(ctx, tenantID, a); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE agendamentos SET nome=$1, descricao=$2, tipo=$3, quando=$4, acao=$5, payload=$6, status=$7,
			max_execucoes=$8, proxima_execucao=$9, canal_id=$10, contato_id=$11, telefone=$12, agente_id=$13, updated_at=now()
		 WHERE tenant_id=$14 AND id=$15`,
		a.Nome, a.Descricao, a.Tipo, a.Quando, a.Acao, normalizeAgendamentoPayload(a.Payload), a.Status,
		a.MaxExecucoes, a.ProximaExecucao, a.CanalID, a.ContatoID, a.Telefone, a.AgenteID, tenantID, a.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) SetAgendamentoStatusForTenant(ctx context.Context, id int, status, resultado, erro string, execucoes int, ultima time.Time, proxima *time.Time) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE agendamentos SET status=$1, resultado=$2, erro=$3, execucoes=$4, ultima_execucao=$5, proxima_execucao=$6, updated_at=now()
		 WHERE tenant_id=$7 AND id=$8`,
		status, resultado, erro, execucoes, ultima, proxima, TenantID(ctx), id)
	return err
}

func (s *Store) SetAgendamentoProximaForTenant(ctx context.Context, id int, proxima time.Time) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE agendamentos SET proxima_execucao=$1, updated_at=now() WHERE tenant_id=$2 AND id=$3 AND status='pendente'`,
		proxima, TenantID(ctx), id)
	return err
}

func (s *Store) DeleteAgendamentoForTenant(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM agendamentos WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id)
	return err
}

// ListDueAgendamentosForTenant devolve pendentes cuja próxima execução já
// venceu (ou nunca foi calculada), na ordem de vencimento.
func (s *Store) ListDueAgendamentosForTenant(ctx context.Context, now time.Time, limit int) ([]Agendamento, error) {
	if limit < 1 || limit > 100 {
		limit = 25
	}
	rows, err := s.pool.Query(ctx,
		`SELECT `+agendamentoCols+` FROM agendamentos
		 WHERE tenant_id=$1 AND status='pendente' AND (proxima_execucao IS NULL OR proxima_execucao <= $2)
		 ORDER BY proxima_execucao ASC NULLS FIRST, id ASC LIMIT $3`, TenantID(ctx), now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Agendamento
	for rows.Next() {
		a, err := scanAgendamento(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ListAgendamentosSemProximaForTenant devolve pendentes sem próxima execução
// calculada (ex.: registros legados), usados no re-agendamento do boot.
func (s *Store) ListAgendamentosSemProximaForTenant(ctx context.Context, limit int) ([]Agendamento, error) {
	if limit < 1 || limit > 500 {
		limit = 200
	}
	rows, err := s.pool.Query(ctx,
		`SELECT `+agendamentoCols+` FROM agendamentos
		 WHERE tenant_id=$1 AND status='pendente' AND proxima_execucao IS NULL
		 ORDER BY id ASC LIMIT $2`, TenantID(ctx), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Agendamento
	for rows.Next() {
		a, err := scanAgendamento(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ClaimAgendamentoForTenant marca a tarefa como executando de forma atômica,
// evitando disparo duplicado entre ticks ou instâncias.
func (s *Store) ClaimAgendamentoForTenant(ctx context.Context, id int, now time.Time) (bool, error) {
	tag, err := s.pool.Exec(ctx,
		`UPDATE agendamentos SET status='executando', updated_at=now()
		 WHERE tenant_id=$1 AND id=$2 AND status='pendente'`, TenantID(ctx), id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// RequeueStaleExecutandoForTenant devolve para pendente as tarefas presas em
// `executando` (ex.: queda do app durante a execução), para que o boot as
// reagende em vez de deixá-las paradas.
func (s *Store) RequeueStaleExecutandoForTenant(ctx context.Context, olderThan time.Time) (int64, error) {
	tag, err := s.pool.Exec(ctx,
		`UPDATE agendamentos SET status='pendente', proxima_execucao=NULL, updated_at=now()
		 WHERE tenant_id=$1 AND status='executando' AND updated_at < $2`, TenantID(ctx), olderThan)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
