# API pública e webhooks

Este documento descreve o contrato **disponível hoje** para integrar um sistema
externo à Notreve. A chave sempre opera dentro de uma única empresa (tenant) e
nunca acessa dados de outra.

## Antes de começar

1. O super_admin habilita o módulo **API e webhooks** para a empresa.
2. Um administrador da empresa cria uma chave em **API e webhooks → Chaves**.
3. Guarde o segredo exibido na criação: ele não é mostrado outra vez.
4. Configure a URL base da sua instalação. Nos exemplos, ela é
   `https://SEU_DOMINIO`.

> O módulo **API** permite que sistemas externos usem a API da Notreve. O
> módulo **Tools para agentes** permite que o agente execute ações em serviços
> externos durante um atendimento. Uma chave da API não dá permissão automática
> para uma tool, e uma tool não expõe a API pública.

## Autenticação e escopos

Envie a chave como Bearer token. `X-API-Key` é aceito por compatibilidade, mas
`Authorization` é a forma recomendada.

```http
Authorization: Bearer nrv_...
```

| Escopo | Permite |
| --- | --- |
| `conversations:read` | Consultar uma conversa pertencente à empresa. |
| `conversations:write` | Abrir/reutilizar conversa, enviar texto e encerrar. |
| `conversations:*` | Leitura e escrita de conversas. |
| `*` | Todos os escopos liberados para a chave. Use somente quando necessário. |

Chaves inativas, revogadas, sem escopo ou de empresas sem o módulo API recebem
`401` ou `403`. Há limite temporário por chave; quando ele é atingido, a
resposta é `429` com `Retry-After: 60`.

## Convenções

- Todas as respostas são JSON: `{ "data": ... }` em sucesso ou
  `{ "error": { "code": "...", "message": "..." } }` em erro.
- O corpo JSON pode ter até 64 KB; uma mensagem de texto pode ter até
  16.000 caracteres.
- Toda escrita exige `Idempotency-Key` (1 a 200 caracteres, alfanuméricos e
  `._:-`).
- A mesma chave com o mesmo conteúdo devolve a primeira resposta por até
  24 horas e inclui `Idempotent-Replayed: true`.
- Reutilizar uma chave com conteúdo diferente retorna
  `409 idempotency_conflict`. Se a chamada original ainda estiver em execução,
  retorna `409 idempotency_in_progress` e `Retry-After: 1`.
- Respostas `5xx` também são preservadas para a mesma chave: não repita uma
  operação com nova chave sem investigar se ela pode ter tido efeito.

## Rotas disponíveis

| Método e rota | Escopo | Função |
| --- | --- | --- |
| `POST /api/v1/conversations` | `conversations:write` | Abre ou reutiliza atendimento ativo. |
| `GET /api/v1/conversations/{id}` | `conversations:read` | Consulta uma conversa da empresa. |
| `POST /api/v1/conversations/{id}/messages` | `conversations:write` | Envia texto e registra a mensagem. |
| `POST /api/v1/conversations/{id}/close` | `conversations:write` | Encerra o atendimento. |

O prefixo legado `/api/public/v1` aceita as mesmas rotas. Para novas
integrações, use `/api/v1`.

### Abrir ou reutilizar uma conversa

Uma empresa não recebe tickets duplicados para o mesmo canal e identificador de
cliente enquanto existir um atendimento aberto. A resposta traz `reused: true`
quando o atendimento já existia. Depois do encerramento, uma nova chamada pode
criar outro ticket.

`channel_id` é obrigatório e precisa apontar para um canal ativo da própria
empresa. Hoje são aceitos canais `whatsapp` e `webchat`.

```bash
curl -X POST "$NOTREVE_URL/api/v1/conversations" \
  -H "Authorization: Bearer $NOTREVE_API_KEY" \
  -H "Idempotency-Key: crm-ordem-8472-open" \
  -H "Content-Type: application/json" \
  -d '{
    "channel_id": 12,
    "customer_id": "5511999999999",
    "customer_name": "Ana Silva",
    "origin": "https://loja.exemplo.com.br"
  }'
```

`customer_id` é obrigatório (máximo de 160 caracteres). Os aliases
`phone` e `identifier` são aceitos, mas informe somente um identificador.
`origin`, quando usado, precisa ser URL HTTP(S) válida; a Notreve guarda apenas
esquema e host. Para webchat, o identificador é normalizado internamente.

Exemplo de sucesso:

```json
{
  "data": {
    "conversation": {
      "id": 418,
      "hash": "...",
      "channel": "whatsapp",
      "channel_name": "Comercial",
      "customer_id": "5511999999999",
      "customer_name": "Ana Silva",
      "origin": "https://loja.exemplo.com.br",
      "status": "ia",
      "created_at": "2026-08-22T12:00:00Z",
      "updated_at": "2026-08-22T12:00:00Z"
    },
    "reused": false
  }
}
```

### Consultar uma conversa

```bash
curl "$NOTREVE_URL/api/v1/conversations/418" \
  -H "Authorization: Bearer $NOTREVE_API_KEY"
```

Somente conversas da empresa associada à chave são retornadas. Uma conversa
inexistente, de outra empresa ou ligada a canal indisponível é tratada como
`404 conversation_not_found`.

### Enviar uma mensagem

O envio disponível é apenas de texto. Em WhatsApp, a API tenta a entrega no
canal; em webchat, a mensagem fica disponível na conversa. O campo
`delivered` descreve a aceitação pelo fluxo atual, não substitui um futuro
evento de confirmação de entrega do provedor.

```bash
curl -X POST "$NOTREVE_URL/api/v1/conversations/418/messages" \
  -H "Authorization: Bearer $NOTREVE_API_KEY" \
  -H "Idempotency-Key: crm-ordem-8472-message-1" \
  -H "Content-Type: application/json" \
  -d '{"content":"Olá, Ana. Seu pedido está em separação."}'
```

Não envie mensagens para tickets encerrados: a resposta será
`409 conversation_closed`. Se o canal não estiver disponível, os erros usuais
são `403 channel_inactive`, `422 unsupported_channel`,
`503 channel_unavailable` ou `502 delivery_failed`.

### Encerrar uma conversa

```bash
curl -X POST "$NOTREVE_URL/api/v1/conversations/418/close" \
  -H "Authorization: Bearer $NOTREVE_API_KEY" \
  -H "Idempotency-Key: crm-ordem-8472-close" \
  -H "Content-Type: application/json" \
  -d '{"reason":"Pedido concluído no ERP"}'
```

`reason` é opcional e tem até 240 caracteres. Se a avaliação estiver habilitada
para o canal, a resposta informa `satisfaction_status` e
`satisfaction_delivered`; o envio da pesquisa não impede o encerramento.

## Erros para tratar

| HTTP | Código frequente | Ação recomendada |
| --- | --- | --- |
| `400` | `invalid_json`, `missing_channel_id`, `invalid_idempotency_key` | Corrija a requisição; não repita cegamente. |
| `401` | `missing_credentials`, `invalid_credentials` | Revise ou rotacione a chave. |
| `403` | `scope_forbidden`, `module_disabled`, `channel_inactive` | Ajuste escopo, módulo ou canal. |
| `404` | `conversation_not_found`, `channel_not_found` | Não tente acessar recurso de outra empresa. |
| `409` | `idempotency_conflict`, `idempotency_in_progress`, `conversation_closed` | Respeite a chave e `Retry-After`; abra novo ticket se necessário. |
| `413` | `payload_too_large`, `content_too_large` | Reduza o conteúdo. |
| `429` | `rate_limited` | Aguarde `Retry-After`. |
| `5xx` | falha temporária ou de entrega | Consulte o efeito antes de usar outra chave. |

## Webhooks de saída

O painel já permite que empresas autorizadas configurem assinaturas, e os
eventos públicos (`conversation.created`, `conversation.updated`,
`conversation.closed`, `message.received`, `message.sent`, `contact.created`,
`contact.updated`, `agent.transferred`, `satisfaction.requested`,
`satisfaction.completed`, `campaign.completed` e `channel.status_changed`) são
gravados em uma outbox transacional e **entregues pelo worker que roda em
produção**. As URLs são validadas como HTTPS públicas e os segredos opcionais são
cifrados no servidor.

- POST JSON para URL HTTPS pública, sem credenciais na URL e sem hosts privados;
- envelope `{ id, event, version, timestamp, data }`;
- cabeçalhos `X-Notreve-Webhook-Id`, `X-Notreve-Webhook-Event`,
  `X-Notreve-Webhook-Timestamp`, `X-Notreve-Webhook-Signature` e
  `X-Notreve-Webhook-Attempt`;
- assinatura HMAC-SHA256 no formato `v1=<hex>`, calculada sobre
  `timestamp + "." + corpo bruto` (quando a assinatura tem segredo configurado);
- retentativas para falha de transporte, `408`, `425`, `429` e `5xx`;
  o consumidor deve responder `2xx` rapidamente e deduplicar por `id`.

> O painel exibe o log de entregas em **Integrações → Webhooks → Ver entregas**.
> Pela API pública, **o envio disponível hoje é apenas texto** — mídia e
> mensagens ricas pela API pública ainda não estão expostas.
