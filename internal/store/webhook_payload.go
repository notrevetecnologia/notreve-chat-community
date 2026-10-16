package store

// ConversationWebhookPayload monta o payload público de um evento de conversa,
// sem dados internos (tenant_id, IDs internos de agente/atendente, tokens ou
// mídia). O endpoint do cliente também recebe esses campos nos eventos da API
// pública (conversa/mensagem/encerramento).
func ConversationWebhookPayload(c Conversation) map[string]interface{} {
	payload := map[string]interface{}{
		"id":            c.ID,
		"status":        c.Status,
		"channel":       c.Channel,
		"customer_id":   c.CustomerPhone,
		"customer_name": c.CustomerName,
		"origem":        c.Origem,
		"created_at":    c.CreatedAt,
		"updated_at":    c.UpdatedAt,
	}
	if c.SetorName != "" {
		payload["setor"] = c.SetorName
	}
	if c.AtendenteName != "" {
		payload["atendente"] = c.AtendenteName
	}
	return payload
}

// MessageWebhookPayload monta o payload público de uma mensagem, reproduzindo o
// que a API pública expõe: papel, fonte, conteúdo e metadados de mídia — sem o
// base64 da mídia nem os dados de contato do cliente.
func MessageWebhookPayload(m Message) map[string]interface{} {
	return map[string]interface{}{
		"id":           m.ID,
		"conversation": m.ConversationID,
		"role":         m.Role,
		"source":       m.Source,
		"content":      m.Content,
		"media_type":   m.MediaType,
		"media_name":   m.MediaName,
		"created_at":   m.CreatedAt,
	}
}
