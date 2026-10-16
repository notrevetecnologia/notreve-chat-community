package store

import (
	"context"
	"time"
)

type DashboardConversations struct {
	IA         int
	Aguardando int
	Atendendo  int
	Encerradas int
	Novas      int
	Total      int
}

type DashboardQueue struct {
	Aguardando          int
	EsperaMaxSegundos   float64
	EsperaMediaSegundos float64
}

type DashboardResolution struct {
	Total        int
	IAResolvidas int
	Transferidas int
}

type DashboardSatisfaction struct {
	Avaliacoes int
	Media      float64
}

type DashboardVolumeDia struct {
	Dia       time.Time
	Recebidas int
	Enviadas  int
}

type DashboardSetor struct {
	ID         *int
	Nome       string
	Conversas  int
	Aguardando int
}

type DashboardAtendente struct {
	ID         int
	Nome       string
	Ativas     int
	Aguardando int
}

func (s *Store) DashboardConversations(ctx context.Context, desde time.Time, user *Usuario, setorIDs []int, modoAberto, verIA bool) (DashboardConversations, error) {
	var out DashboardConversations
	query := `SELECT
		COUNT(*) FILTER (WHERE c.status='ia'),
		COUNT(*) FILTER (WHERE c.status='humano_aguardando'),
		COUNT(*) FILTER (WHERE c.status='humano_atendendo'),
		COUNT(*) FILTER (WHERE c.status='closed'),
		COUNT(*) FILTER (WHERE c.created_at >= $2),
		COUNT(*)
		FROM conversations c WHERE c.tenant_id=$1`
	args := []interface{}{TenantID(ctx), desde}
	query, args = conversationScopeFilter(query, args, user, setorIDs, modoAberto, verIA)
	err := s.pool.QueryRow(ctx, query, args...).Scan(&out.IA, &out.Aguardando, &out.Atendendo, &out.Encerradas, &out.Novas, &out.Total)
	return out, err
}

func (s *Store) DashboardQueue(ctx context.Context, user *Usuario, setorIDs []int, modoAberto, verIA bool) (DashboardQueue, error) {
	var out DashboardQueue
	query := `SELECT COUNT(*),
		COALESCE(MAX(EXTRACT(EPOCH FROM (now() - wait_start)))::float8, 0),
		COALESCE(AVG(EXTRACT(EPOCH FROM (now() - wait_start)))::float8, 0)
		FROM (
			SELECT c.id,
				COALESCE(
					(SELECT MIN(m.created_at) FROM messages m
						WHERE m.tenant_id=c.tenant_id AND m.conversation_id=c.id AND m.source IN ('customer','user')
						AND m.created_at > COALESCE(
							(SELECT MAX(m2.created_at) FROM messages m2
								WHERE m2.tenant_id=c.tenant_id AND m2.conversation_id=c.id AND m2.source IN ('ai','assistant','human','operator','system')),
							'epoch'::timestamptz)),
					c.updated_at
				) AS wait_start
			FROM conversations c
			WHERE c.tenant_id=$1 AND c.status='humano_aguardando'`
	args := []interface{}{TenantID(ctx)}
	query, args = conversationScopeFilter(query, args, user, setorIDs, modoAberto, verIA)
	query += `) q`
	err := s.pool.QueryRow(ctx, query, args...).Scan(&out.Aguardando, &out.EsperaMaxSegundos, &out.EsperaMediaSegundos)
	return out, err
}

func (s *Store) DashboardResolution(ctx context.Context, desde time.Time, user *Usuario, setorIDs []int, modoAberto, verIA bool) (DashboardResolution, error) {
	var out DashboardResolution
	query := `SELECT
		COUNT(*) FILTER (WHERE c.created_at >= $2),
		COUNT(*) FILTER (WHERE c.created_at >= $2 AND c.status='closed' AND c.atendente_id IS NULL
			AND NOT EXISTS (SELECT 1 FROM messages m WHERE m.tenant_id=c.tenant_id AND m.conversation_id=c.id AND m.source IN ('human','operator'))),
		COUNT(*) FILTER (WHERE c.created_at >= $2 AND (c.atendente_id IS NOT NULL OR c.status IN ('humano_aguardando','humano_atendendo')
			OR EXISTS (SELECT 1 FROM messages m WHERE m.tenant_id=c.tenant_id AND m.conversation_id=c.id AND m.source IN ('human','operator'))))
		FROM conversations c WHERE c.tenant_id=$1`
	args := []interface{}{TenantID(ctx), desde}
	query, args = conversationScopeFilter(query, args, user, setorIDs, modoAberto, verIA)
	err := s.pool.QueryRow(ctx, query, args...).Scan(&out.Total, &out.IAResolvidas, &out.Transferidas)
	return out, err
}

func (s *Store) DashboardSatisfaction(ctx context.Context, desde time.Time, user *Usuario, setorIDs []int, modoAberto, verIA bool) (DashboardSatisfaction, error) {
	var out DashboardSatisfaction
	query := `SELECT COUNT(*), COALESCE(AVG(notas.nota)::float8, 0) FROM (
		SELECT a.nota_ia::numeric AS nota, a.conversation_id FROM avaliacoes a
			WHERE a.tenant_id=$1 AND a.created_at >= $2 AND a.nota_ia IS NOT NULL
		UNION ALL
		SELECT a.nota_operador::numeric, a.conversation_id FROM avaliacoes a
			WHERE a.tenant_id=$1 AND a.created_at >= $2 AND a.nota_operador IS NOT NULL
	) notas
	JOIN conversations c ON c.id=notas.conversation_id AND c.tenant_id=$1 WHERE TRUE`
	args := []interface{}{TenantID(ctx), desde}
	query, args = conversationScopeFilter(query, args, user, setorIDs, modoAberto, verIA)
	err := s.pool.QueryRow(ctx, query, args...).Scan(&out.Avaliacoes, &out.Media)
	return out, err
}

func (s *Store) DashboardVolume(ctx context.Context, desde time.Time, user *Usuario, setorIDs []int, modoAberto, verIA bool) ([]DashboardVolumeDia, error) {
	query := `SELECT date_trunc('day', m.created_at) AS dia,
		COUNT(*) FILTER (WHERE m.source IN ('customer','user')),
		COUNT(*) FILTER (WHERE m.source IN ('ai','assistant','human','operator'))
		FROM messages m JOIN conversations c ON c.id=m.conversation_id AND c.tenant_id=$1
		WHERE m.tenant_id=$1 AND m.created_at >= $2`
	args := []interface{}{TenantID(ctx), desde}
	query, args = conversationScopeFilter(query, args, user, setorIDs, modoAberto, verIA)
	query += ` GROUP BY 1 ORDER BY 1 ASC`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DashboardVolumeDia
	for rows.Next() {
		var item DashboardVolumeDia
		if err := rows.Scan(&item.Dia, &item.Recebidas, &item.Enviadas); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) DashboardSetores(ctx context.Context, desde time.Time, user *Usuario, setorIDs []int, modoAberto, verIA bool) ([]DashboardSetor, error) {
	query := `SELECT c.setor_id, COALESCE(s.name, 'Sem setor'),
		COUNT(*) FILTER (WHERE c.created_at >= $2),
		COUNT(*) FILTER (WHERE c.status='humano_aguardando')
		FROM conversations c LEFT JOIN setores s ON s.id=c.setor_id AND s.tenant_id=$1
		WHERE c.tenant_id=$1`
	args := []interface{}{TenantID(ctx), desde}
	query, args = conversationScopeFilter(query, args, user, setorIDs, modoAberto, verIA)
	query += ` GROUP BY c.setor_id, s.name ORDER BY 3 DESC, 2 ASC LIMIT 6`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DashboardSetor
	for rows.Next() {
		var item DashboardSetor
		if err := rows.Scan(&item.ID, &item.Nome, &item.Conversas, &item.Aguardando); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) DashboardAtendentes(ctx context.Context, user *Usuario, setorIDs []int, modoAberto, verIA bool) ([]DashboardAtendente, error) {
	query := `SELECT u.id, u.name,
		COUNT(*) FILTER (WHERE c.status IN ('humano_aguardando','humano_atendendo')),
		COUNT(*) FILTER (WHERE c.status='humano_aguardando')
		FROM conversations c JOIN usuarios u ON u.id=c.atendente_id AND u.tenant_id=$1
		WHERE c.tenant_id=$1`
	args := []interface{}{TenantID(ctx)}
	query, args = conversationScopeFilter(query, args, user, setorIDs, modoAberto, verIA)
	query += ` GROUP BY u.id, u.name ORDER BY 3 DESC, u.name ASC LIMIT 8`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DashboardAtendente
	for rows.Next() {
		var item DashboardAtendente
		if err := rows.Scan(&item.ID, &item.Nome, &item.Ativas, &item.Aguardando); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
