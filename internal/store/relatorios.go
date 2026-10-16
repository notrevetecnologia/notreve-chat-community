package store

import (
	"context"
	"time"
)

// RelatorioResumoPeriodo agrega os indicadores do período.
func (s *Store) RelatorioResumo(ctx context.Context, desde time.Time) (RelatorioResumo, error) {
	var r RelatorioResumo
	err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) AS conversas,
			COUNT(*) FILTER (WHERE status IN ('ia','humano_atendendo','humano_aguardando')) AS ativas,
			COUNT(*) FILTER (WHERE status='humano_aguardando') AS aguardando,
			COUNT(*) FILTER (WHERE status='closed') AS encerradas,
			(SELECT COUNT(*) FROM messages m WHERE m.tenant_id = $2 AND m.created_at >= $1) AS mensagens,
			(SELECT COUNT(*) FROM messages m WHERE m.tenant_id = $2 AND m.created_at >= $1 AND m.source='customer') AS msg_cliente,
			(SELECT COUNT(*) FROM messages m WHERE m.tenant_id = $2 AND m.created_at >= $1 AND m.source='ai') AS msg_ia,
			(SELECT COUNT(*) FROM messages m WHERE m.tenant_id = $2 AND m.created_at >= $1 AND m.source='human') AS msg_humano,
			(SELECT COUNT(*) FROM contatos c WHERE c.tenant_id = $2 AND c.created_at >= $1) AS contatos_novos,
			COALESCE((SELECT ROUND(AVG(nota),1)::float8 FROM (
				SELECT nota_ia::numeric AS nota FROM avaliacoes WHERE tenant_id = $2 AND nota_ia IS NOT NULL AND created_at >= $1
				UNION ALL
				SELECT nota_operador::numeric FROM avaliacoes WHERE tenant_id = $2 AND nota_operador IS NOT NULL AND created_at >= $1
			) notas),0) AS sat_geral,
			COALESCE((SELECT ROUND(AVG(nota_ia),1)::float8 FROM avaliacoes WHERE tenant_id = $2 AND nota_ia IS NOT NULL AND created_at >= $1),0) AS sat_ia,
			COALESCE((SELECT ROUND(AVG(nota_operador),1)::float8 FROM avaliacoes WHERE tenant_id = $2 AND nota_operador IS NOT NULL AND created_at >= $1),0) AS sat_op,
			(SELECT COUNT(*) FROM avaliacoes WHERE tenant_id = $2 AND created_at >= $1) AS avaliacoes,
			(SELECT COUNT(nota_ia) FROM avaliacoes WHERE tenant_id = $2 AND created_at >= $1) AS avaliacoes_ia,
			(SELECT COUNT(nota_operador) FROM avaliacoes WHERE tenant_id = $2 AND created_at >= $1) AS avaliacoes_op,
			(SELECT COUNT(*) FROM campanhas WHERE tenant_id = $2 AND created_at >= $1) AS campanhas,
			COALESCE((SELECT SUM(total) FROM campanhas WHERE tenant_id = $2 AND created_at >= $1),0) AS camp_total,
			COALESCE((SELECT SUM(enviadas) FROM campanhas WHERE tenant_id = $2 AND created_at >= $1),0) AS camp_enviadas,
			COALESCE((SELECT SUM(falhas) FROM campanhas WHERE tenant_id = $2 AND created_at >= $1),0) AS camp_falhas
		FROM conversations WHERE tenant_id = $2 AND created_at >= $1`, desde, TenantID(ctx)).
		Scan(&r.Conversas, &r.Ativas, &r.Aguardando, &r.Encerradas, &r.Mensagens, &r.MsgCliente,
			&r.MsgIA, &r.MsgHumano, &r.ContatosNovos, &r.SatGeral, &r.SatIA, &r.SatOp, &r.Avaliacoes,
			&r.AvaliacoesIA, &r.AvaliacoesOp,
			&r.Campanhas, &r.CampTotal, &r.CampEnviadas, &r.CampFalhas)
	return r, err
}

// RelatorioPorAtendente agrega conversas e mensagens humanas por atendente.
func (s *Store) RelatorioPorAtendente(ctx context.Context, desde time.Time) ([]RelAtendente, error) {
	rows, err := s.pool.Query(ctx, `
		WITH mensagens AS (
			SELECT conversation_id, COUNT(*) AS total
			FROM messages
			WHERE tenant_id=$2 AND source='human' AND created_at >= $1
			GROUP BY conversation_id
		)
		SELECT u.name,
			COUNT(DISTINCT c.id) FILTER (WHERE c.created_at >= $1) AS conversas,
			COALESCE(SUM(m.total),0) AS msg_humanas,
			COUNT(a.nota_operador) AS avaliacoes,
			COALESCE(ROUND(AVG(a.nota_operador),1)::float8,0) AS nota_operador
		FROM conversations c
		JOIN usuarios u ON u.id = c.atendente_id AND u.tenant_id=$2
		LEFT JOIN mensagens m ON m.conversation_id = c.id
		LEFT JOIN avaliacoes a ON a.conversation_id = c.id AND a.tenant_id=$2 AND a.created_at >= $1
		WHERE c.tenant_id=$2 AND (c.created_at >= $1 OR a.id IS NOT NULL OR m.conversation_id IS NOT NULL)
		GROUP BY u.id, u.name
		ORDER BY nota_operador DESC, avaliacoes DESC, conversas DESC, u.name ASC`, desde, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RelAtendente
	for rows.Next() {
		var r RelAtendente
		if err := rows.Scan(&r.Nome, &r.Conversas, &r.MsgHumanas, &r.Avaliacoes, &r.NotaOperador); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) RelatorioPorSetor(ctx context.Context, desde time.Time) ([]RelSetor, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT COALESCE(s.name, 'Sem setor') AS nome,
			COUNT(DISTINCT c.id) FILTER (WHERE c.created_at >= $1) AS conversas,
			COUNT(a.id) AS avaliacoes,
			COALESCE(ROUND((SUM(COALESCE(a.nota_ia,0) + COALESCE(a.nota_operador,0))::numeric /
				NULLIF(COUNT(a.nota_ia) + COUNT(a.nota_operador),0)),1)::float8,0) AS nota_geral,
			COALESCE(ROUND(AVG(a.nota_ia),1)::float8,0) AS nota_ia,
			COALESCE(ROUND(AVG(a.nota_operador),1)::float8,0) AS nota_humano
		FROM conversations c
		LEFT JOIN setores s ON s.id = c.setor_id AND s.tenant_id=$2
		LEFT JOIN avaliacoes a ON a.conversation_id = c.id AND a.tenant_id=$2 AND a.created_at >= $1
		WHERE c.tenant_id=$2 AND (c.created_at >= $1 OR a.id IS NOT NULL)
		GROUP BY s.id, s.name
		ORDER BY nota_geral DESC, avaliacoes DESC, conversas DESC, nome ASC`, desde, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RelSetor
	for rows.Next() {
		var r RelSetor
		if err := rows.Scan(&r.Nome, &r.Conversas, &r.Avaliacoes, &r.NotaGeral, &r.NotaIA, &r.NotaHumano); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) RelatorioAvaliacoesPorAtendimento(ctx context.Context, desde time.Time) ([]RelAvaliacaoAtendimento, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT a.conversation_id, COALESCE(u.name, 'Sem atendente'), COALESCE(s.name, 'Sem setor'), c.channel,
			COALESCE(a.nota_ia,0), COALESCE(a.nota_operador,0),
			ROUND(((COALESCE(a.nota_ia,0) + COALESCE(a.nota_operador,0))::numeric /
				NULLIF((a.nota_ia IS NOT NULL)::int + (a.nota_operador IS NOT NULL)::int,0)),1)::float8 AS nota_geral,
			a.created_at
		FROM avaliacoes a
		JOIN conversations c ON c.id = a.conversation_id AND c.tenant_id=$2
		LEFT JOIN usuarios u ON u.id = c.atendente_id AND u.tenant_id=$2
		LEFT JOIN setores s ON s.id = c.setor_id AND s.tenant_id=$2
		WHERE a.tenant_id=$2 AND a.created_at >= $1 AND (a.nota_ia IS NOT NULL OR a.nota_operador IS NOT NULL)
		ORDER BY a.created_at DESC
		LIMIT 200`, desde, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RelAvaliacaoAtendimento
	for rows.Next() {
		var r RelAvaliacaoAtendimento
		if err := rows.Scan(&r.ConversationID, &r.Atendente, &r.Setor, &r.Canal, &r.NotaIA, &r.NotaOperador, &r.NotaGeral, &r.AvaliadoEm); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RelatorioPorDia retorna conversas e mensagens por dia no período (últimos 30 dias).
func (s *Store) RelatorioPorDia(ctx context.Context, dias int) ([]RelDia, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT to_char(d.dia, 'DD/MM') AS dia,
			COALESCE(conv.total,0) AS conversas,
			COALESCE(msg.total,0) AS mensagens
		FROM generate_series(date_trunc('day', now()) - make_interval(days => $1 - 1), date_trunc('day', now()), '1 day') AS d(dia)
		LEFT JOIN (SELECT date_trunc('day', created_at) AS dia, COUNT(*) AS total FROM conversations WHERE tenant_id=$2 GROUP BY 1) conv ON conv.dia = d.dia
		LEFT JOIN (SELECT date_trunc('day', created_at) AS dia, COUNT(*) AS total FROM messages WHERE tenant_id=$2 GROUP BY 1) msg ON msg.dia = d.dia
		ORDER BY d.dia ASC`, dias, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RelDia
	for rows.Next() {
		var r RelDia
		if err := rows.Scan(&r.Dia, &r.Conversas, &r.Mensagens); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RelatorioSatisfacao devolve a distribuição de notas (1-5) combinando IA e operador.
func (s *Store) RelatorioSatisfacao(ctx context.Context, desde time.Time) ([]RelNota, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT nota,
			COUNT(*) FILTER (WHERE origem='ia') AS ia,
			COUNT(*) FILTER (WHERE origem='operador') AS operador,
			COUNT(*) AS total
		FROM (
			SELECT nota_ia AS nota, 'ia' AS origem FROM avaliacoes WHERE tenant_id=$2 AND created_at >= $1 AND nota_ia IS NOT NULL
			UNION ALL
			SELECT nota_operador, 'operador' FROM avaliacoes WHERE tenant_id=$2 AND created_at >= $1 AND nota_operador IS NOT NULL
		) notas
		GROUP BY nota ORDER BY nota DESC`, desde, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RelNota
	for rows.Next() {
		var n RelNota
		if err := rows.Scan(&n.Nota, &n.IA, &n.Operador, &n.Total); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

type RelIndicadoresAvancados struct {
	TMASegundos      float64
	TMAConversas     int
	Transferidas     int
	ConversasTotal   int
	TransferenciaPct float64
	NPS              float64
	NPSPromotores    int
	NPSDetratores    int
	NPSTotal         int
	Abandonadas      int
	AbandonoPct      float64
}

func (s *Store) RelatorioIndicadoresAvancados(ctx context.Context, desde time.Time) (RelIndicadoresAvancados, error) {
	var r RelIndicadoresAvancados
	tenant := TenantID(ctx)

	err := s.pool.QueryRow(ctx, `
		WITH cust AS (
			SELECT DISTINCT ON (conversation_id) conversation_id, created_at AS t
			FROM messages
			WHERE tenant_id=$1 AND created_at >= $2 AND source IN ('customer','user')
			ORDER BY conversation_id, created_at
		),
		resp AS (
			SELECT DISTINCT ON (conversation_id) conversation_id, created_at AS t
			FROM messages
			WHERE tenant_id=$1 AND created_at >= $2 AND source IN ('ai','assistant','human','operator')
			ORDER BY conversation_id, created_at
		)
		SELECT COUNT(*), COALESCE(AVG(EXTRACT(EPOCH FROM (r.t - c.t)))::float8,0)
		FROM cust c JOIN resp r ON r.conversation_id=c.conversation_id AND r.t > c.t`, tenant, desde).
		Scan(&r.TMAConversas, &r.TMASegundos)
	if err != nil {
		return r, err
	}

	err = s.pool.QueryRow(ctx, `
		SELECT COUNT(*),
			COUNT(*) FILTER (WHERE c.status IN ('humano_aguardando','humano_atendendo')
				OR c.atendente_id IS NOT NULL
				OR EXISTS (SELECT 1 FROM messages m WHERE m.conversation_id=c.id AND m.tenant_id=$1 AND m.source IN ('human','operator')))
		FROM conversations c WHERE c.tenant_id=$1 AND c.created_at >= $2`, tenant, desde).
		Scan(&r.ConversasTotal, &r.Transferidas)
	if err != nil {
		return r, err
	}

	err = s.pool.QueryRow(ctx, `
		WITH notas AS (
			SELECT nota_ia AS n FROM avaliacoes WHERE tenant_id=$1 AND created_at >= $2 AND nota_ia IS NOT NULL
			UNION ALL
			SELECT nota_operador FROM avaliacoes WHERE tenant_id=$1 AND created_at >= $2 AND nota_operador IS NOT NULL
		)
		SELECT COUNT(*) FILTER (WHERE n>=4), COUNT(*) FILTER (WHERE n<=2), COUNT(*)
		FROM notas`, tenant, desde).
		Scan(&r.NPSPromotores, &r.NPSDetratores, &r.NPSTotal)
	if err != nil {
		return r, err
	}

	err = s.pool.QueryRow(ctx, `
		SELECT COUNT(*)
			FILTER (WHERE c.status='humano_aguardando'
				OR (c.status='closed' AND c.satisfacao IN ('sem_resposta','aguardando','aguardando_operador')
					AND c.atendente_id IS NULL
					AND NOT EXISTS (SELECT 1 FROM messages m WHERE m.conversation_id=c.id AND m.tenant_id=$1 AND m.source IN ('human','operator'))))
		FROM conversations c WHERE c.tenant_id=$1 AND c.created_at >= $2`, tenant, desde).
		Scan(&r.Abandonadas)
	if err != nil {
		return r, err
	}

	if r.ConversasTotal > 0 {
		r.TransferenciaPct = float64(r.Transferidas) / float64(r.ConversasTotal) * 100
		r.AbandonoPct = float64(r.Abandonadas) / float64(r.ConversasTotal) * 100
	}
	if r.NPSTotal > 0 {
		r.NPS = (float64(r.NPSPromotores) - float64(r.NPSDetratores)) / float64(r.NPSTotal) * 100
	}
	return r, nil
}
