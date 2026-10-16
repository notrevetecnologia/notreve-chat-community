package web

import (
	"net/http"
	"os"

	"github.com/notrevetecnologia/notreve-chat-community/internal/webhooks"
)

type apiDocsAuth struct {
	BearerExample string `json:"bearer"`
	HeaderHint    string `json:"header_hint"`
	KeyPrefix     string `json:"key_prefix"`
}

type apiDocsScope struct {
	Scope string `json:"scope"`
	What  string `json:"what"`
}

type apiDocsBody struct {
	ContentType string `json:"content_type,omitempty"`
	Example     string `json:"example,omitempty"`
}

type apiDocsEndpoint struct {
	Method      string        `json:"method"`
	Path        string        `json:"path"`
	Scope       string        `json:"scope"`
	Summary     string        `json:"summary"`
	Description string        `json:"description"`
	Request     *apiDocsBody  `json:"request,omitempty"`
	Response    *apiDocsBody  `json:"response,omitempty"`
	Errors      []string      `json:"errors,omitempty"`
}

type apiDocsWebhookHeader struct {
	Name string `json:"name"`
	What string `json:"what"`
}

type apiDocsWebhooks struct {
	Catalog     []webhooks.EventDefinition `json:"catalog"`
	Envelope    string                     `json:"envelope"`
	Signature   string                     `json:"signature"`
	Headers     []apiDocsWebhookHeader     `json:"headers"`
	Retry       string                     `json:"retry"`
	Dedupe      string                     `json:"dedupe"`
	Active      bool                       `json:"active"`
	Provisional bool                       `json:"provisional"`
}

type apiDocsErrorRow struct {
	HTTP   string `json:"http"`
	Code   string `json:"code"`
	Action string `json:"action"`
}

type apiDocsLimits struct {
	MaxJSONBytes   string `json:"max_json_bytes"`
	MaxTextRunes   string `json:"max_text_runes"`
	MaxNameRunes   string `json:"max_name_runes"`
	MaxReasonRunes string `json:"max_reason_runes"`
	IdempotencyTTL string `json:"idempotency_ttl"`
	RateLimit      string `json:"rate_limit"`
}

type apiDocs struct {
	BasePath  string            `json:"base_path"`
	Auth      apiDocsAuth       `json:"auth"`
	Scopes    []apiDocsScope    `json:"scopes"`
	Endpoints []apiDocsEndpoint `json:"endpoints"`
	Webhooks  apiDocsWebhooks   `json:"webhooks"`
	Errors    []apiDocsErrorRow `json:"errors"`
	Limits    apiDocsLimits     `json:"limits"`
}

func (s *Server) handleReactAPIDocs(w http.ResponseWriter, r *http.Request) {
	docs := apiDocs{
		BasePath: os.Getenv("AGENTE_IA_PUBLIC_BASE_URL"),
		Auth: apiDocsAuth{
			BearerExample: "nrv_...",
			HeaderHint:    "Use exclusivamente Authorization: Bearer <chave>.",
			KeyPrefix:     "nrv_",
		},
		Scopes: []apiDocsScope{
			{Scope: "conversations:read", What: "Consultar uma conversa pertencente à empresa."},
			{Scope: "conversations:write", What: "Abrir/reutilizar conversa, enviar texto e encerrar."},
			{Scope: "conversations:*", What: "Leitura e escrita de conversas."},
			{Scope: "*", What: "Todos os escopos liberados para a chave. Use somente quando necessário."},
		},
		Endpoints: []apiDocsEndpoint{
			{
				Method: "POST", Path: "/api/v1/conversations", Scope: "conversations:write",
				Summary:     "Abrir ou reutilizar uma conversa",
				Description: "Cria um atendimento ou retoma o ativo existente para o mesmo canal e identificador do cliente. `channel_id` é obrigatório e deve apontar para um canal ativo (whatsapp ou webchat).",
				Request: &apiDocsBody{ContentType: "application/json", Example: `{
  "channel_id": 12,
  "customer_id": "5511999999999",
  "customer_name": "Ana Silva",
  "origin": "https://loja.exemplo.com.br"
}`},
				Response: &apiDocsBody{ContentType: "application/json", Example: `{
  "data": {
    "conversation": { "id": 418, "status": "ia", "channel": "whatsapp" },
    "reused": false
  }
}`},
				Errors: []string{"400 missing_channel_id", "404 channel_not_found", "403 channel_inactive", "422 unsupported_channel", "403 module_disabled"},
			},
			{
				Method: "GET", Path: "/api/v1/conversations/{id}", Scope: "conversations:read",
				Summary:     "Consultar uma conversa",
				Description: "Retorna a conversa apenas se pertencer à empresa da chave. Conversa inexistente, de outra empresa ou de canal indisponível vira 404.",
				Response:    &apiDocsBody{ContentType: "application/json", Example: `{ "data": { "conversation": { "id": 418, "status": "ia" } } }`},
				Errors:      []string{"404 conversation_not_found"},
			},
			{
				Method: "POST", Path: "/api/v1/conversations/{id}/messages", Scope: "conversations:write",
				Summary:     "Enviar uma mensagem de texto",
				Description: "Envia texto e registra a mensagem. Em WhatsApp, tenta a entrega no canal; em webchat, fica disponível na conversa. O campo `delivered` descreve a aceitação pelo fluxo atual.",
				Request:     &apiDocsBody{ContentType: "application/json", Example: `{ "content": "Olá, Ana. Seu pedido está em separação." }`},
				Response:    &apiDocsBody{ContentType: "application/json", Example: `{ "data": { "conversation": { "id": 418 }, "delivered": true } }`},
				Errors:      []string{"409 conversation_closed", "403 channel_inactive", "422 unsupported_channel", "503 channel_unavailable", "502 delivery_failed"},
			},
			{
				Method: "POST", Path: "/api/v1/conversations/{id}/close", Scope: "conversations:write",
				Summary:     "Encerrar uma conversa",
				Description: "Encerra o atendimento com motivo opcional (até 240 caracteres). Se a avaliação estiver habilitada, a resposta traz `satisfaction_status` e `satisfaction_delivered`.",
				Request:     &apiDocsBody{ContentType: "application/json", Example: `{ "reason": "Pedido concluído no ERP" }`},
				Response:    &apiDocsBody{ContentType: "application/json", Example: `{ "data": { "closed": true, "satisfaction_status": "aguardando" } }`},
				Errors:      []string{"400 invalid_reason", "404 conversation_not_found"},
			},
		},
		Webhooks: apiDocsWebhooks{
			Catalog:   webhooks.Catalog(),
			Envelope:  `{ "id": "wh_...", "event": "message.received", "version": 1, "timestamp": 1780000000, "data": { ... } }`,
			Signature: "HMAC-SHA256 no formato `v1=<hex>`, calculada sobre `timestamp + \".\" + corpo bruto`. Configure um segredo de assinatura na assinatura do webhook.",
			Headers: []apiDocsWebhookHeader{
				{Name: "X-Notreve-Webhook-Id", What: "Identificador único do evento (dedupe)."},
				{Name: "X-Notreve-Webhook-Event", What: "Tipo do evento (ex.: message.received)."},
				{Name: "X-Notreve-Webhook-Timestamp", What: "Timestamp Unix do envio."},
				{Name: "X-Notreve-Webhook-Signature", What: "Assinatura HMAC-SHA256 (quando há segredo)."},
				{Name: "X-Notreve-Webhook-Attempt", What: "Número da tentativa (1, 2, ...)."},
			},
			Retry:  "Em falha de transporte, 408, 425, 429 ou 5xx, com backoff exponencial. Responda 2xx rapidamente e deduplique por `id`.",
			Dedupe: "O consumidor deve deduplicar pelo `id` do envelope.",
			Active: true, Provisional: false,
		},
		Errors: []apiDocsErrorRow{
			{HTTP: "400", Code: "invalid_json, missing_channel_id, invalid_idempotency_key", Action: "Corrija a requisição; não repita cegamente."},
			{HTTP: "401", Code: "missing_credentials, invalid_credentials", Action: "Revise ou rotacione a chave."},
			{HTTP: "403", Code: "scope_forbidden, module_disabled, channel_inactive", Action: "Ajuste escopo, módulo ou canal."},
			{HTTP: "404", Code: "conversation_not_found, channel_not_found", Action: "Não acesse recursos de outra empresa."},
			{HTTP: "409", Code: "idempotency_conflict, idempotency_in_progress, conversation_closed", Action: "Respeite a chave e Retry-After; abra novo ticket se necessário."},
			{HTTP: "413", Code: "payload_too_large, content_too_large", Action: "Reduza o conteúdo."},
			{HTTP: "429", Code: "rate_limited", Action: "Aguarde Retry-After."},
			{HTTP: "5xx", Code: "falha temporária ou de entrega", Action: "Confirme o efeito antes de usar outra chave."},
		},
		Limits: apiDocsLimits{
			MaxJSONBytes:   "64 KB",
			MaxTextRunes:   "16.000 caracteres",
			MaxNameRunes:   "80 caracteres",
			MaxReasonRunes: "240 caracteres",
			IdempotencyTTL: "24 h",
			RateLimit:      "Temporário por chave; 429 + Retry-After: 60",
		},
	}
	writeReactResponse(w, http.StatusOK, docs, nil)
}
