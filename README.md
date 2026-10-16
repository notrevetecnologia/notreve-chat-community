# Notreve Chat Community

Plataforma **multi-tenant** de atendimento inteligente no WhatsApp, que une
**IA + time humano** num só produto. A IA (LLM compatível com OpenAI) atende
24/7 com a base de conhecimento do seu agente; quando o cliente **pede um
humano** ou um setor, a conversa vai para a **fila humana** (com distribuição
automática), e o time assume no **Inbox** com contexto.

> A IA **nunca transfere por conta própria** — só quando o cliente solicita um
> humano (`[HUMANO]`) ou um setor (`[SETOR:Nome]`), ou em falha técnica.

## Integrações e pré-requisitos

O **Notreve Chat Community** é self-hosted, mas depende de dois serviços
externos para operar de ponta a ponta:

- **WhatsApp — Notreve WhatsApp API (obrigatório para o canal WhatsApp).** Toda a
  infraestrutura do WhatsApp é administrada pela Notreve; você só conecta o
  número por QR Code. É preciso **contratar o serviço** em
  [whatsapp.notreve.com.br](https://whatsapp.notreve.com.br). Os canais Webchat e
  Telegram não dependem dele.
- **IA — qualquer provedor compatível com OpenAI (recomendado: Notreve IA).**
  Você pode usar OpenAI, Gemini, DeepSeek, um modelo local etc., informando
  `base_url` + modelo + chave. Para simplicidade, **recomendamos a
  [Notreve IA](https://ia.notreve.com.br)**: basta o token, que a Notreve gerencia
  endpoint, modelos e transcrição.

## O que faz

- **Agentes de IA** — instruções, base de conhecimento (texto, PDF, site, áudio)
  e **tools** que chamam APIs externas, com resposta real ao cliente.
- **Provedores de IA** — qualquer API compatível com OpenAI (OpenAI, Gemini,
  DeepSeek, local) via `base_url` + `modelo` + chave, com capacidade
  texto/multimodal e transcrição (STT).
- **Tools / function calling** — a IA chama endpoints externos (ex.: consultar
  saldo, status de pedido) e responde com dados reais; com catálogo, política de
  risco, escopos, allowlist de host, timeout e **auditoria**.
- **Multiatendimento** — Inbox, filas, setores, distribuição automática,
  transferência, follow-up, pesquisa de satisfação.
- **Mensagens ricas** — botões, lista, enquete, contato, localização e
  figurinha, com **fallback** para texto numerado quando o canal não suporta.
- **Campanhas** — envio em massa com agendamento, intervalo, limite diário,
  cooldown e modo teste (proteção anti-bloqueio no WhatsApp).
- **Canais** — WhatsApp (API Notreve), **Web Chat** (widget no site) e Telegram,
  via um `Adapter`/`Registry` plugável.
- **API pública & webhooks** — conversa (abrir/reusar/texto/consultar/encerrar),
  chaves por tenant com escopos, e webhooks de saída com outbox + retry/backoff.

## Stack

| Camada | Tecnologia |
|---|---|
| Backend | Go (pgx/v5), sem framework web |
| Banco | PostgreSQL 16, migrations em `migrations/*.sql` |
| Frontend | React + TypeScript (Vite) em `web/react` |
| Cache/Sessões | Redis (opcional, fallback em memória) |
| Mensageria | RabbitMQ (opcional, eventos) |
| Deploy | Docker Compose (app + postgres + redis + rabbitmq + caddy) |

## Ambiente recomendado

O projeto foi **projetado para rodar em um servidor VPS/dedicado** com
**Ubuntu 24.04 ou 26.04** e **Docker** (o instalador cuida do resto). Este é o
ambiente **suportado** para produção.

> Instalar em **máquina local** (`localhost`, Windows/macOS/desktop) é possível,
> mas **por sua conta e risco** — não é um ambiente suportado/oficial: a
> resolução de tenant é por domínio e várias funcionalidades dependem de
> HTTPS, domínio e portas públicas.

## Como rodar (Docker Compose)

Pré-requisitos: Docker + Docker Compose.

```bash
git clone https://github.com/notrevetecnologia/notreve-chat-community.git
cd notreve-agente-ia
cp .env.example .env   # defina os segredos (veja abaixo)
docker compose up -d --build
```

Acesse `http://localhost:8080`.

Variáveis mínimas no `.env` (veja `.env.example` para todas):
- `AGENTE_IA_PASS` — senha do super administrador.
- `AGENTE_IA_SECRETS_KEY` — chave mestra (32 bytes em base64) para criptografar
  credenciais internas.
- `AGENTE_IA_DB` / `AGENTE_IA_ADMIN_DB` — conexões PostgreSQL (runtime e
  administração/migrations). O runtime usa uma role **sem** SUPERUSER/BYPASSRLS.
- `WHATSAPP_HMAC_KEY` — chave HMAC (opcional) para validar webhooks recebidos.

> **Segurança:** nunca commite `.env` (está no `.gitignore`). Credenciais de
> canais/provedores/tools são armazenadas **criptografadas no banco** e nunca
> retornadas ao painel (a UI só mostra `configured`).

## Como rodar (local, sem Docker)

```bash
# 1. Banco
createdb agente_ia

# 2. Config
cp .env.example .env

# 3. Migrations + app
go run ./cmd/migrate
go run .
```

Acesse `http://127.0.0.1:8080`.

## Configurar o bot (passo a passo rápido)

Veja o guia completo em [`docs/SETUP.md`](docs/SETUP.md). Resumo:

1. **Canais → Conectar** o WhatsApp (QR/Notreve), Web Chat ou Telegram.
2. **Inteligência → Provedores de IA** — informe `base_url`, modelo e chave.
3. **Inteligência → Agentes** — instruções + base de conhecimento (texto/PDF/site/áudio).
4. **Inteligência → Tools** (opcional) — conecte APIs externas.
5. **Operação → Equipe e setores** — atendentes e setores (horário Brasília).
6. **Configurações** — allowlist de números (segurança), distribuição, follow-up.
7. **Inbox** — teste o ciclo completo (IA → pedido de humano → fila → humano).

## Testes

```bash
go build -buildvcs=false ./...
go vet ./...
go test ./...
cd web/react && npm run build
```

## Estrutura

```
main.go                       # entrada / rotas raiz (webhook, webchat, painel)
cmd/migrate/                  # runner de migrations
internal/web                  # painel HTTP, sessões, permissões, jobs
internal/channel              # canais (notreve/webchat/telegram), roteador
internal/agent                # motor de IA/prompt, conhecimento, mensagens ricas
internal/llm                  # client OpenAI-compat (chat + STT)
internal/tools (+policy)      # executor de + política (risco/escopo/auditoria)
internal/store                # acesso a dados (SQL)
internal/db                   # runner de migrations
internal/webhooks             # assinatura/webhooks de saída
internal/webhookoutbox        # worker de entrega (retry/backoff)
internal/infra                # Redis/RabbitMQ (opcionais)
web/react                     # frontend React/TypeScript (Vite)
migrations/                   # schema PostgreSQL
docs/                         # arquitetura, setup, roteiro, operação
```

## Documentação

- [`docs/ARQUITETURA.md`](docs/ARQUITETURA.md) — visão geral e fluxos.
- [`docs/SETUP.md`](docs/SETUP.md) — guia passo a passo (1 → 10).
- [`docs/ARQUITETURA_MULTI_TENANT.md`](docs/ARQUITETURA_MULTI_TENANT.md) —
  isolamento multi-tenant.
- [`docs/PRODUCAO.md`](docs/PRODUCAO.md) — operação/produção, backup.
- [`docs/API_PUBLICA.md`](docs/API_PUBLICA.md) — API pública e webhooks.
- [`docs/CHANGELOG.md`](docs/CHANGELOG.md) — histórico de alterações.
- [`docs/ROADMAP.md`](docs/ROADMAP.md) — fases e próximas features.
- [`docs/PENDENCIAS.md`](docs/PENDENCIAS.md) — riscos conhecidos e pendências.

## Comunidade

- [`CONTRIBUTING.md`](CONTRIBUTING.md) — como contribuir (gate obrigatório e regras).
- [`SECURITY.md`](SECURITY.md) — como reportar vulnerabilidades de forma privada.

## Licença

[MIT](LICENSE). Veja o arquivo `LICENSE`.
