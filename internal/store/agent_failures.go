package store

import (
	"context"
	"strings"
	"time"
)

const (
	FailureKindLLM      = "llm"
	FailureKindTool     = "tool"
	FailureKindChannel  = "channel"
	FailureKindPipeline = "pipeline"
	FailureKindOther    = "outros"
)

type AgentFailure struct {
	ID             int        `json:"id"`
	TenantID       int        `json:"tenant_id"`
	ConversationID *int       `json:"conversation_id,omitempty"`
	Channel        string     `json:"channel"`
	Phone          string     `json:"phone,omitempty"`
	Origem         string     `json:"origem"`
	Tipo           string     `json:"tipo"`
	ExceptionClass string     `json:"exception_class"`
	ExceptionCode  string     `json:"exception_code,omitempty"`
	MensagemErro   string     `json:"mensagem_erro"`
	Traceback      string     `json:"traceback,omitempty"`
	Tentativas     int        `json:"tentativas"`
	CreatedAt      time.Time  `json:"created_at"`
	ResolvidoEm    *time.Time `json:"resolvido_em,omitempty"`
	ResolvidoPor   string     `json:"resolvido_por,omitempty"`
	ResolucaoNota  string     `json:"resolucao_nota,omitempty"`
}

func ClassifyAgentFailure(err error) string {
	if err == nil {
		return FailureKindOther
	}
	message := strings.ToLower(err.Error())
	if containsAny(message, "tool", "ferramenta") {
		return FailureKindTool
	}
	if containsAny(message, "canal", "channel", "envio", "send", "whatsapp", "telegram") {
		return FailureKindChannel
	}
	if containsAny(message, "llm", "modelo", "model", "provedor", "provider", "openai", "anthropic",
		"base_url", "token", "http", "timeout", "conex", "connection", "rate", "api key") {
		return FailureKindLLM
	}
	return FailureKindPipeline
}

func containsAny(text string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func (s *Store) RecordAgentFailure(ctx context.Context, failure AgentFailure) error {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return err
	}
	if strings.TrimSpace(failure.Tipo) == "" {
		failure.Tipo = FailureKindOther
	}
	if strings.TrimSpace(failure.Origem) == "" {
		failure.Origem = "pipeline"
	}
	if failure.Tentativas < 0 {
		failure.Tentativas = 1
	}
	if failure.Tentativas == 0 {
		failure.Tentativas = 1
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO agent_failures
		(tenant_id, conversation_id, channel, phone, origem, tipo, exception_class, exception_code, mensagem_erro, traceback, tentativas)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, failure.ConversationID, failure.Channel, failure.Phone, failure.Origem,
		failure.Tipo, failure.ExceptionClass, failure.ExceptionCode, failure.MensagemErro,
		failure.Traceback, failure.Tentativas)
	return err
}

func (s *Store) ListAgentFailures(ctx context.Context, tipo string, onlyOpen bool, limit int) ([]AgentFailure, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return nil, err
	}
	if limit < 1 || limit > 500 {
		limit = 100
	}
	tipo = strings.TrimSpace(tipo)
	rows, err := s.pool.Query(ctx, `SELECT id, tenant_id, conversation_id, channel, phone, origem, tipo,
		exception_class, exception_code, mensagem_erro, traceback, tentativas, created_at, resolvido_em, resolvido_por, resolucao_nota
		FROM agent_failures
		WHERE tenant_id=$1 AND ($2='' OR tipo=$2) AND (NOT $3 OR resolvido_em IS NULL)
		ORDER BY created_at DESC LIMIT $4`, tenantID, tipo, onlyOpen, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AgentFailure
	for rows.Next() {
		var f AgentFailure
		if err := rows.Scan(&f.ID, &f.TenantID, &f.ConversationID, &f.Channel, &f.Phone, &f.Origem, &f.Tipo,
			&f.ExceptionClass, &f.ExceptionCode, &f.MensagemErro, &f.Traceback, &f.Tentativas,
			&f.CreatedAt, &f.ResolvidoEm, &f.ResolvidoPor, &f.ResolucaoNota); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) ListAgentFailuresPage(ctx context.Context, tipo string, onlyOpen bool, q string, limit, offset int) ([]AgentFailure, int, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return nil, 0, err
	}
	tipo = strings.TrimSpace(tipo)
	q = strings.TrimSpace(q)
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM agent_failures
		WHERE tenant_id=$1 AND ($2='' OR tipo=$2) AND (NOT $3 OR resolvido_em IS NULL)
		  AND ($4='' OR lower(mensagem_erro) LIKE lower('%'||$4||'%') OR lower(exception_class) LIKE lower('%'||$4||'%') OR phone LIKE '%'||$4||'%' OR lower(channel) LIKE lower('%'||$4||'%'))`,
		tenantID, tipo, onlyOpen, q).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id, tenant_id, conversation_id, channel, phone, origem, tipo,
		exception_class, exception_code, mensagem_erro, traceback, tentativas, created_at, resolvido_em, resolvido_por, resolucao_nota
		FROM agent_failures
		WHERE tenant_id=$1 AND ($2='' OR tipo=$2) AND (NOT $3 OR resolvido_em IS NULL)
		  AND ($4='' OR lower(mensagem_erro) LIKE lower('%'||$4||'%') OR lower(exception_class) LIKE lower('%'||$4||'%') OR phone LIKE '%'||$4||'%' OR lower(channel) LIKE lower('%'||$4||'%'))
		ORDER BY created_at DESC LIMIT $5 OFFSET $6`, tenantID, tipo, onlyOpen, q, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []AgentFailure
	for rows.Next() {
		var f AgentFailure
		if err := rows.Scan(&f.ID, &f.TenantID, &f.ConversationID, &f.Channel, &f.Phone, &f.Origem, &f.Tipo,
			&f.ExceptionClass, &f.ExceptionCode, &f.MensagemErro, &f.Traceback, &f.Tentativas,
			&f.CreatedAt, &f.ResolvidoEm, &f.ResolvidoPor, &f.ResolucaoNota); err != nil {
			return nil, 0, err
		}
		out = append(out, f)
	}
	return out, total, rows.Err()
}

func (s *Store) CountOpenAgentFailures(ctx context.Context) (int, error) {	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return 0, err
	}
	var count int
	err = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM agent_failures WHERE tenant_id=$1 AND resolvido_em IS NULL`, tenantID).Scan(&count)
	return count, err
}

func (s *Store) CountAgentFailuresByType(ctx context.Context, since time.Time) (map[string]int, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT tipo, COUNT(*) FROM agent_failures
		WHERE tenant_id=$1 AND created_at >= $2 GROUP BY tipo`, tenantID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var tipo string
		var count int
		if err := rows.Scan(&tipo, &count); err != nil {
			return nil, err
		}
		out[tipo] = count
	}
	return out, rows.Err()
}

func (s *Store) MarkAgentFailureResolved(ctx context.Context, id int, by, nota string) (bool, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return false, err
	}
	tag, err := s.pool.Exec(ctx, `UPDATE agent_failures SET resolvido_em=now(), resolvido_por=$3, resolucao_nota=$4
		WHERE tenant_id=$1 AND id=$2 AND resolvido_em IS NULL`, tenantID, id, by, nota)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
