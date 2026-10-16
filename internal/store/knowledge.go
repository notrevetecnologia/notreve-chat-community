package store

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type KnowledgeSource struct {
	ID           int64                  `json:"id"`
	TenantID     int                    `json:"tenant_id"`
	AgentID      int                    `json:"agent_id"`
	Kind         string                 `json:"kind"`
	Name         string                 `json:"name"`
	SourceURL    string                 `json:"source_url"`
	MIMEType     string                 `json:"mime_type"`
	Content      string                 `json:"content,omitempty"`
	ContentChars int                    `json:"content_chars"`
	Status       string                 `json:"status"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Metadata     map[string]interface{} `json:"metadata"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

type KnowledgeUsage struct {
	Sources int `json:"sources"`
	Chars   int `json:"chars"`
}

const knowledgeSourceColumns = `id, tenant_id, agent_id, kind, name, source_url, mime_type, content, content_chars, status, error_message, metadata, created_at, updated_at`

func scanKnowledgeSource(row interface{ Scan(...interface{}) error }) (KnowledgeSource, error) {
	var source KnowledgeSource
	var metadata []byte
	err := row.Scan(&source.ID, &source.TenantID, &source.AgentID, &source.Kind, &source.Name, &source.SourceURL,
		&source.MIMEType, &source.Content, &source.ContentChars, &source.Status, &source.ErrorMessage, &metadata,
		&source.CreatedAt, &source.UpdatedAt)
	if err != nil {
		return source, err
	}
	source.Metadata = map[string]interface{}{}
	if len(metadata) > 0 {
		_ = json.Unmarshal(metadata, &source.Metadata)
	}
	return source, nil
}

func (s *Store) ListKnowledgeSources(ctx context.Context, agentID int) ([]KnowledgeSource, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+knowledgeSourceColumns+` FROM knowledge_sources WHERE tenant_id=$1 AND agent_id=$2 ORDER BY updated_at DESC, id DESC`, TenantID(ctx), agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]KnowledgeSource, 0)
	for rows.Next() {
		item, err := scanKnowledgeSource(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetKnowledgeSource(ctx context.Context, id int64) (KnowledgeSource, error) {
	return scanKnowledgeSource(s.pool.QueryRow(ctx, `SELECT `+knowledgeSourceColumns+` FROM knowledge_sources WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id))
}

func (s *Store) CreateKnowledgeSource(ctx context.Context, source KnowledgeSource) (int64, error) {
	metadata, err := json.Marshal(source.Metadata)
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.pool.QueryRow(ctx, `INSERT INTO knowledge_sources (tenant_id, agent_id, kind, name, source_url, mime_type, content, content_chars, status, error_message, metadata) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id`,
		TenantID(ctx), source.AgentID, source.Kind, source.Name, source.SourceURL, source.MIMEType, source.Content, source.ContentChars,
		source.Status, source.ErrorMessage, metadata).Scan(&id)
	return id, err
}

func (s *Store) UpdateKnowledgeSourceResult(ctx context.Context, id int64, status, content, errorMessage string, contentChars int, metadata map[string]interface{}) error {
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `UPDATE knowledge_sources SET status=$1, content=$2, content_chars=$3, error_message=$4, metadata=$5, updated_at=now() WHERE tenant_id=$6 AND id=$7`,
		status, content, contentChars, errorMessage, encoded, TenantID(ctx), id)
	return err
}

func (s *Store) DeleteKnowledgeSource(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM knowledge_sources WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id)
	return err
}

func (s *Store) KnowledgeUsage(ctx context.Context, agentID int) (KnowledgeUsage, error) {
	var usage KnowledgeUsage
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*)::int, COALESCE(SUM(content_chars),0)::int FROM knowledge_sources WHERE tenant_id=$1 AND agent_id=$2 AND status='ready'`, TenantID(ctx), agentID).Scan(&usage.Sources, &usage.Chars)
	return usage, err
}

const (
	maxKnowledgeQueryTerms = 10
	maxKnowledgeChunkLimit = 20
)

type KnowledgeChunk struct {
	SourceID   int64   `json:"source_id"`
	SourceName string  `json:"source_name"`
	Content    string  `json:"content"`
	Ord        int     `json:"ord"`
	Rank       float64 `json:"rank"`
}

// knowledgeQueryTerms extrai os termos úteis da mensagem do cliente para montar
// a consulta de full-text. Termos curtos e sem letra/dígito são descartados.
func knowledgeQueryTerms(query string) []string {
	fields := strings.Fields(query)
	terms := make([]string, 0, len(fields))
	for _, field := range fields {
		if !strings.ContainsFunc(field, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) {
			continue
		}
		terms = append(terms, field)
		if len(terms) >= maxKnowledgeQueryTerms {
			break
		}
	}
	return terms
}

// SearchKnowledgeChunks busca os trechos mais relevantes da base do agente com
// o full-text do Postgres (ts_rank), escopado por tenant e por agente. Os termos
// entram como parâmetros e são combinados com OR para não exigir que todos
// apareçam no mesmo trecho.
func (s *Store) SearchKnowledgeChunks(ctx context.Context, agentID int, query string, limit int) ([]KnowledgeChunk, error) {
	terms := knowledgeQueryTerms(query)
	if len(terms) == 0 || limit <= 0 {
		return nil, nil
	}
	if limit > maxKnowledgeChunkLimit {
		limit = maxKnowledgeChunkLimit
	}

	args := make([]interface{}, 0, len(terms)+3)
	args = append(args, TenantID(ctx), agentID)
	termExprs := make([]string, 0, len(terms))
	rankExprs := make([]string, 0, len(terms))
	for _, term := range terms {
		args = append(args, term)
		placeholder := "$" + strconv.Itoa(len(args))
		expr := "plainto_tsquery('portuguese', " + placeholder + ")"
		termExprs = append(termExprs, expr)
		rankExprs = append(rankExprs, "ts_rank(c.search, "+expr+")")
	}
	args = append(args, limit)
	limitPlaceholder := "$" + strconv.Itoa(len(args))

	statement := `SELECT c.source_id, s.name, c.content, c.ord, (` + strings.Join(rankExprs, " + ") + `)::float8 AS rank
		FROM knowledge_chunks c
		JOIN knowledge_sources s ON s.id = c.source_id
		WHERE c.tenant_id = $1 AND c.agent_id = $2 AND s.status = 'ready'
		  AND c.search @@ (` + strings.Join(termExprs, " || ") + `)
		ORDER BY rank DESC, c.ord ASC
		LIMIT ` + limitPlaceholder

	rows, err := s.pool.Query(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	chunks := make([]KnowledgeChunk, 0, limit)
	for rows.Next() {
		var chunk KnowledgeChunk
		if err := rows.Scan(&chunk.SourceID, &chunk.SourceName, &chunk.Content, &chunk.Ord, &chunk.Rank); err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}
	return chunks, rows.Err()
}

// HasKnowledgeChunks diz se o agente já teve a base indexada em trechos. É usado
// pelo motor para distinguir "nenhum trecho relevante" de "índice indisponível".
func (s *Store) HasKnowledgeChunks(ctx context.Context, agentID int) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM knowledge_chunks WHERE tenant_id=$1 AND agent_id=$2)`, TenantID(ctx), agentID).Scan(&exists)
	return exists, err
}
