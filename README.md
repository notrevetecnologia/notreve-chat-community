# Notreve Chat Community

[![Licença: MIT](https://img.shields.io/badge/licen%C3%A7a-MIT-2ea44f.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8.svg?logo=go&logoColor=white)](https://go.dev)
[![React](https://img.shields.io/badge/React-18-61DAFB.svg?logo=react&logoColor=white)](https://react.dev)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED.svg?logo=docker&logoColor=white)](https://docs.docker.com/compose/)

> **Atendimento no WhatsApp unindo IA + time humano num só produto** —
> self-hosted, **multi-tenant** e open source.
> Desenvolvido pela [Notreve Tecnologia LTDA](https://notreve.com.br).

A IA atende 24/7 com a base de conhecimento do seu agente. Quando o cliente
**pede um humano** ou um setor, a conversa entra na **fila humana** (com
distribuição automática) e o time assume no **Inbox**, com todo o contexto.

> **Princípio do produto:** a IA **nunca transfere por conta própria** — só
> quando o cliente pede um humano (`[HUMANO]`) ou um setor (`[SETOR:Nome]`), ou
> em falha técnica.

## Índice

- [Integrações](#integrações)
- [Recursos](#recursos)
- [Stack](#stack)
- [Requisitos](#requisitos)
- [Instalação](#instalação)
- [Primeiro acesso](#primeiro-acesso)
- [Gerenciar e atualizar](#gerenciar-e-atualizar)
- [Desenvolvimento](#desenvolvimento)
- [Documentação](#documentação)
- [Contribuindo](#contribuindo)
- [Licença](#licença)

## Integrações

O projeto é **self-hosted**, mas depende de dois serviços externos para operar
de ponta a ponta:

| Integração | Papel | Observação |
|---|---|---|
| **Notreve WhatsApp API** | Canal WhatsApp | **Obrigatório** para o canal WhatsApp. A infraestrutura é administrada pela Notreve; você conecta o número por QR Code. **Contrate em [whatsapp.notreve.com.br](https://whatsapp.notreve.com.br)**. Webchat e Telegram não dependem dele. |
| **Provedor de IA** | Responde ao cliente | Qualquer serviço **compatível com OpenAI** (OpenAI, Gemini, DeepSeek, local…). **Recomendado:** [Notreve IA](https://ia.notreve.com.br) — basta o token; a Notreve gerencia endpoint, modelos e transcrição. |

## Recursos

- **Agentes de IA** — instruções + base de conhecimento (texto, PDF, site, áudio)
  e **tools** que chamam APIs externas, respondendo com dados reais.
- **Function calling com segurança** — catálogo, política de risco, escopos,
  allowlist de host, anti-SSRF, timeout e **auditoria**.
- **Multiatendimento** — Inbox, filas, setores, distribuição automática,
  transferência, follow-up e pesquisa de satisfação.
- **Mensagens ricas** — botões, lista, enquete, contato, localização e figurinha,
  com **fallback** para texto numerado quando o canal não suporta.
- **Campanhas** — envio em massa com agendamento, intervalo, limite diário,
  cooldown e modo teste (proteção anti-bloqueio no WhatsApp).
- **Canais plugáveis** — WhatsApp (Notreve WhatsApp API), **Webchat** (widget no
  site) e **Telegram**, via `Adapter`/`Registry`.
- **API pública & webhooks** — abrir/reusar/texto/consultar/encerrar conversa,
  chaves por empresa com escopos e webhooks de saída com outbox + retry/backoff.
- **Multi-tenant com papéis** — `super_admin`, `admin`, `supervisor` e
  `atendente`, com isolamento por empresa e permissões granulares.
- **RAG por relevância** — conhecimento quebrado em trechos e buscado por
  relevância (top-K) em vez de despejar toda a base no prompt.
- **Skills e MCP** — pacotes de instruções (progressive disclosure) e cliente
  Model Context Protocol (stdio + HTTP JSON-RPC).

## Stack

| Camada | Tecnologia |
|---|---|
| Backend | Go (pgx/v5), sem framework web |
| Banco | PostgreSQL 16 (`migrations/*.sql`) |
| Frontend | React + TypeScript (Vite) em `web/react` |
| Cache/Sessão | Redis (opcional, fallback em memória) |
| Mensageria | RabbitMQ (opcional) |
| Deploy | Docker Compose (app + postgres + redis + rabbitmq + caddy) |

## Requisitos

- **Servidor VPS/dedicado com Ubuntu 24.04 ou 26.04** e **Docker** — ambiente
  **suportado** (o instalador instala o Docker se faltar).
- Para usar domínio com HTTPS: DNS apontando para o servidor e portas 80/443
  liberadas (o Caddy emite o certificado automaticamente).

> Instalar em **máquina local** (`localhost`, Windows/macOS) é possível, mas
> **por sua conta e risco** — não é um ambiente suportado: o sistema resolve o
> tenant pelo domínio e depende de HTTPS/domínio/portas públicas.

## Instalação

### Rápida (recomendada)

```bash
curl -sSL https://raw.githubusercontent.com/notrevetecnologia/notreve-chat-community/main/install.sh | sh
```

O instalador baixa a versão atual da branch `main`, **gera os segredos**, pergunta
o domínio (Enter = localhost), sobe os containers, espera o `/health` e mostra
**URL + usuário (`super_admin`) + senha** (uma única vez).

### Manual (a partir do código)

```bash
git clone https://github.com/notrevetecnologia/notreve-chat-community.git
cd notreve-chat-community
sudo ./install.sh                 # ou: sudo ./install.sh --domain painel.exemplo.com
```

Opções do instalador: `--dir`, `--domain`, `--update`, `--reset`, `--yes`,
`--version`, `-h`. Veja tudo em [`docs/QUICKSTART.md`](docs/QUICKSTART.md).

## Primeiro acesso

1. Entre em **`/`** com **`super_admin`** + a senha gerada.
2. Ative a **verificação em duas etapas (MFA)**.
3. Rode o **assistente de 2 passos**: cadastre a **API WhatsApp** (base URL +
   token admin) e crie a **empresa + o primeiro admin**.
4. Entre como o **admin da empresa** e rode o assistente
   **Provedor de IA → Agente → Canal**.

## Gerenciar e atualizar

O instalador cria o atalho **`ncc`**:

```bash
ncc up | down | restart | logs | update | backup | restore | doctor | version
```

Atualizar: `sudo ncc update` (faz backup do banco, baixa a versão atualizada,
aplica as migrations e sobe de novo).

## Desenvolvimento

```bash
# backend
go build -buildvcs=false ./...
go vet ./...
go test ./...

# frontend
cd web/react && npm ci && npm run build
```

Atalhos no `Makefile`: `make gate`, `make run`, `make migrate`, `make up`.

Estrutura (resumo):

```
main.go                  # entrada / rotas raiz (webhook, webchat, painel)
cmd/migrate/             # runner de migrations
internal/web             # painel HTTP, sessões, permissões, jobs
internal/channel         # canais (notreve/webchat/telegram) + Adapter/Registry
internal/agent           # motor de IA, conhecimento (RAG), chat flow, mensagens ricas
internal/llm             # client OpenAI-compat (chat + STT + imagem)
internal/tools (+policy) # executor de tools + política (risco/escopo/auditoria)
internal/store           # acesso a dados (SQL)
internal/webhooks        # assinatura e webhooks de saída
internal/webhookoutbox   # worker de entrega (retry/backoff)
web/react                # frontend React/TypeScript (Vite)
migrations/              # schema PostgreSQL
docs/                    # arquitetura, hierarquia, permissões, setup, API
```

Leia o [`AGENTS.md`](AGENTS.md) antes de mexer — é o mapa do projeto (regras,
arquitetura e o que não quebrar).

## Documentação

- [`docs/QUICKSTART.md`](docs/QUICKSTART.md) — do zero ao primeiro atendimento.
- [`docs/SETUP.md`](docs/SETUP.md) — configuração passo a passo (1 → 10).
- [`docs/ARQUITETURA.md`](docs/ARQUITETURA.md) — visão geral e fluxos.
- [`docs/HIERARQUIA.md`](docs/HIERARQUIA.md) — papéis, permissões e quem configura o quê.
- [`docs/PERMISSOES.md`](docs/PERMISSOES.md) — matriz de permissões.
- [`docs/ARQUITETURA_MULTI_TENANT.md`](docs/ARQUITETURA_MULTI_TENANT.md) — isolamento multi-tenant.
- [`docs/API_PUBLICA.md`](docs/API_PUBLICA.md) — API pública e webhooks.
- [`docs/PRODUCAO.md`](docs/PRODUCAO.md) — operação/produção e backup.

## Contribuindo

Contribuições são bem-vindas! Leia o [`CONTRIBUTING.md`](CONTRIBUTING.md)
(gate obrigatório e padrões) e o [`AGENTS.md`](AGENTS.md). Abra uma *issue*
usando os [templates](.github/ISSUE_TEMPLATE) e siga o
[pull request template](.github/PULL_REQUEST_TEMPLATE.md).

- **Segurança:** reporte vulnerabilidades de forma **privada** — veja o [`SECURITY.md`](SECURITY.md).
- **Conduta:** [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md).

## Licença

[MIT](LICENSE) © Notreve Tecnologia LTDA.
