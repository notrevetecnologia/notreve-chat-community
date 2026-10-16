package store

import (
	"context"
)

// ConversationLLMUsage resumo de tokens de uma conversa.
type ConversationLLMUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// RecordConversationLLMUsage registra o consumo de uma chamada de IA numa
// conversa e atualiza os contadores da conversa (para o admin acompanhar gasto).
func (s *Store) RecordConversationLLMUsage(ctx context.Context, conversationID int, agentID int, model string, promptTokens, completionTokens int) error {
	if promptTokens < 0 {
		promptTokens = 0
	}
	if completionTokens < 0 {
		completionTokens = 0
	}
	tenant := TenantID(ctx)
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO conversation_llm_usage (tenant_id, conversation_id, agent_id, model, prompt_tokens, completion_tokens) VALUES ($1,$2,$3,$4,$5,$6)`,
		tenant, conversationID, agentID, model, promptTokens, completionTokens); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE conversations SET tokens_prompt = tokens_prompt + $1, tokens_completion = tokens_completion + $2 WHERE tenant_id=$3 AND id=$4`,
		promptTokens, completionTokens, tenant, conversationID)
	return err
}

// ConversationLLMUsage retorna o total de tokens gastos numa conversa.
func (s *Store) ConversationLLMUsage(ctx context.Context, conversationID int) (ConversationLLMUsage, error) {
	var u ConversationLLMUsage
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(tokens_prompt,0), COALESCE(tokens_completion,0) FROM conversations WHERE tenant_id=$1 AND id=$2`,
		TenantID(ctx), conversationID).Scan(&u.PromptTokens, &u.CompletionTokens)
	return u, err
}
