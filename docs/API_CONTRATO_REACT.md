# Contrato da API do painel React

## Escopo

A API do novo painel fica no namespace `/api/react`. As rotas HTML e os endpoints legados em `/api/conversas` continuam disponíveis e não são substituídos por este contrato.

Todas as respostas da API usam um envelope comum:

```json
{
  "data": {},
  "meta": {}
}
```

Em caso de erro:

```json
{
  "error": {
    "code": "permission_denied",
    "message": "sem permissão para acessar"
  }
}
```

Os códigos HTTP usados são `200` para sucesso, `400` para entrada inválida, `401` para sessão ausente ou expirada, `403` para origem, módulo ou permissão bloqueados, `404` para conversa inexistente ou fora do escopo do usuário, `409` para conflito produzido por uma ação existente e `500` para falha interna.

## Sessão e bootstrap

O painel usa a sessão HTTP existente, armazenada no cookie `agente_ia_session`. O login HTML continua em `/login`; esta primeira fundação não cria um segundo fluxo de login.

### `GET /api/react/session`

Retorna o usuário autenticado, as permissões efetivas, os módulos cadastrados e a versão do contrato.

Resposta:

```json
{
  "data": {
    "user": {
      "id": 1,
      "name": "Usuário exemplo",
      "username": "usuario_exemplo",
      "role": "admin",
      "atendente_id": 2,
      "active": true
    },
    "permissions": ["conversas_responder", "conversas_ver"],
    "modules": [
      {
        "key": "conversas",
        "name": "Conversas",
        "description": "Atendimento",
        "enabled": true,
        "sort": 1
      }
    ],
    "api_version": "1"
  }
}
```

### `GET /api/react/bootstrap`

Retorna os mesmos dados de sessão e acrescenta os estados válidos de conversa e os limites de paginação.

Resposta adicional:

```json
{
  "data": {
    "conversation_statuses": ["ia", "humano_aguardando", "humano_atendendo", "closed"],
    "pagination": {
      "default_limit": 25,
      "max_limit": 100,
      "cursor": "offset"
    }
  }
}
```

O objeto completo de `data` contém também `user`, `permissions`, `modules` e `api_version` mostrados no endpoint de sessão.

## Conversas

As rotas de conversa exigem o módulo `conversas` ativo e a permissão `conversas_ver`, exceto envio e ações, que exigem as permissões indicadas abaixo. O escopo aplicado é o mesmo do painel atual: atendentes não recebem conversas de IA ou encerradas e só veem conversas atribuídas, da fila permitida ou sem setor conforme a configuração.

### `GET /api/react/conversations`

Parâmetros opcionais:

| Parâmetro | Tipo | Regra |
| --- | --- | --- |
| `status` | string | `ia`, `humano_aguardando`, `humano_atendendo`, `closed` ou `all` |
| `q` | string | Busca por nome, telefone, canal, origem, hash, status, atendente ou setor |
| `search` | string | Alias de `q` |
| `limit` | inteiro | De `1` a `100`; padrão `25` |
| `cursor` | string | Cursor opaco retornado em `meta.next_cursor` |

Exemplo:

```http
GET /api/react/conversations?status=humano_aguardando&q=maria&limit=25
```

Resposta:

```json
{
  "data": [
    {
      "id": 9,
      "hash": "c_exemplo",
      "channel": "whatsapp",
      "customer_name": "Cliente exemplo",
      "customer_phone": "NUMERO_COM_DDI",
      "status": "humano_aguardando",
      "origem": "checkout.exemplo.com",
      "setor_id": 2,
      "setor_name": "Comercial",
      "atendente_id": null,
      "updated_at": "2026-08-21T15:10:00Z"
    }
  ],
  "meta": {
    "limit": 25,
    "total": 1,
    "has_more": false,
    "next_cursor": "",
    "cursor": "",
    "filters": {
      "status": "humano_aguardando",
      "search": "maria"
    }
  }
}
```

Para a próxima página, envie o cursor sem modificá-lo:

```http
GET /api/react/conversations?limit=25&cursor=MjU
```

O cursor atual representa uma posição sobre o resultado autorizado carregado pelo servidor. Ele não é um identificador permanente; alterações de mensagens entre requisições podem deslocar a página. A evolução recomendada é mover o filtro e a paginação para SQL usando `updated_at` e `id` como chave de ordenação.

### `GET /api/react/conversations/{id}`

Retorna a conversa e todas as mensagens autorizadas. O campo de mídia em base64 não é incluído; somente `has_media` e os metadados são retornados para evitar respostas grandes.

Resposta:

```json
{
  "data": {
    "conversation": {
      "id": 9,
      "channel": "webchat",
      "customer_name": "Cliente exemplo",
      "status": "humano_atendendo",
      "hash": "c_exemplo"
    },
    "messages": [
      {
        "id": 101,
        "conversation_id": 9,
        "role": "user",
        "content": "Olá",
        "source": "customer",
        "has_media": false,
        "created_at": "2026-08-21T15:11:00Z"
      }
    ]
  }
}
```

## Mensagens

### `POST /api/react/conversations/{id}/messages`

Exige o módulo `conversas` e a permissão `conversas_responder`. Reutiliza as regras existentes de aceite, atribuição, assinatura do operador e envio pelo canal.

Request:

```json
{
  "content": "Posso ajudar em mais alguma coisa?",
  "reply_to": 101
}
```

`reply_to` é opcional e deve ser o ID de uma mensagem da mesma conversa.

Resposta:

```json
{
  "data": {
    "ok": true,
    "delivered": true,
    "conversation": {
      "id": 9,
      "status": "humano_atendendo"
    }
  }
}
```

`delivered=false` significa que a mensagem foi registrada, mas o envio ao canal não foi confirmado pelo sender.

## Ações

### `POST /api/react/conversations/{id}/actions`

As ações usam a permissão específica e reaproveitam os handlers existentes.

| `action` | Permissão | Campos adicionais |
| --- | --- | --- |
| `accept` | `conversas_responder` | nenhum |
| `reopen` | `conversas_encerrar` | nenhum; reabre como `humano_atendendo` |
| `close` | `conversas_encerrar` | `motivo`, `silencioso` opcional |
| `transfer` | `conversas_transferir` | exatamente um entre `setor_id` e `atendente_id` |

Reabrir:

```json
{
  "action": "reopen"
}
```

Encerrar:

```json
{
  "action": "close",
  "motivo": "Atendimento concluído",
  "silencioso": false
}
```

Transferir para setor:

```json
{
  "action": "transfer",
  "setor_id": 2
}
```

Resposta:

```json
{
  "data": {
    "action": "transfer",
    "conversation": {
      "id": 9,
      "status": "humano_aguardando",
      "setor_id": 2
    },
    "result": {
      "ok": "true"
    }
  }
}
```

## Segurança e compatibilidade

- Respostas de autenticação e autorização são JSON, sem redirect para `/login`.
- Requisições mutáveis precisam respeitar a mesma origem do painel.
- O cookie de sessão permanece HttpOnly e o cookie CSRF continua sendo preparado para o frontend.
- O contrato não expõe `password_hash` nem `media_b64`.
- O endpoint de mídia legado continua sendo `/api/mensagens/{id}/media`; sua migração para o namespace React fica para a etapa de anexos.
- Não há login JSON nesta primeira etapa; o login existente continua sendo HTML.
- Nenhuma rota HTML existente foi removida ou alterada em comportamento.
