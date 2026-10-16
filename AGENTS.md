# AGENTS.md — Regras para agentes de IA neste repositório

Este arquivo é lido por agentes de IA (opencode e outros) que trabalham no
**Notreve Chat Community**. Leia antes de alterar qualquer coisa.

## Leia primeiro (ordem sugerida)

1. [`README.md`](README.md) — o que é o projeto e as integrações.
2. Este `AGENTS.md` — regras, arquitetura e o que não quebrar.
3. [`docs/ARQUITETURA.md`](docs/ARQUITETURA.md) — visão geral e fluxos.
4. [`docs/HIERARQUIA.md`](docs/HIERARQUIA.md) — papéis, permissões e quem configura o quê.
5. [`docs/PERMISSOES.md`](docs/PERMISSOES.md) — matriz de permissões.
6. [`CONTRIBUTING.md`](CONTRIBUTING.md) — gate, regras e padrões de PR.

> Antes de varrer o código à toa, use estes documentos como mapa.

## O que é o projeto (visão)

Plataforma **multi-tenant** que unifica **4 frentes** em um só produto:

1. **APIs / integração (para o cliente integrar o próprio sistema/ERP)** — conectar
   a instância (QR), enviar mensagens e receber respostas via webhook pelo sistema
   do cliente, usando a API do Agente IA. API de conversa (abrir/reusar/texto/
   consultar/encerrar) + webhooks de saída. **Notreve WhatsApp API = B2B/atacado**
   (outro público); o Agente IA é self-service e expõe sua própria API/webhooks.
2. **Multiatendimento / chatbot** — Inbox onde a IA atende e **transfere para
   humano** (filas, setores, distribuição automática, avaliação).
3. **Agentes de IA / tools** — motores por agente (conhecimento + tools que
   chamam sistemas externos; catalogado, com risco/escopos/auditoria).
4. **Campanhas / envio em massa** — grupos de canais, listas de contatos,
   agendamento, limite/failover/modo teste.

É o **híbrido funcional** que aproveita os produtos-base: **Notreve WhatsApp API**
(devs), **Notreve IA** (plataforma de IA) e **Notreve Chat** (editado a partir de
base própria). Leia `docs/ESSENCIA.md` e `docs/ESPECIFICACAO.md`.

## Modelo de papéis (SaaS) — regra permanente, não esquecer

Este é um **SaaS multi-tenant**. A hierarquia é de **propriedade**, não só de
permissão. Grave isso antes de desenhar qualquer tela/função:

- `super_admin` — **dono do sistema/plataforma (Notreve)**. Control plane:
  empresas (tenants), planos, limites, módulos, **servidores WhatsApp**,
  white-label global, marca do sistema, saúde e consumo. **Não opera dados de
  cliente** (conversas, canais, agentes, LGPD) — isso é do tenant. Tem **MFA
  obrigatório**.
- `admin` — **dono da empresa contratante** (o cliente que paga mensalidade).
  Tudo dentro da própria empresa: atendimento, canais, agentes, provedores,
  equipe, LGPD, campanhas, relatórios. Nunca administra outras empresas nem
  infraestrutura compartilhada.
- `supervisor` — **funcionário do cliente**. Operação de time: setores,
  campanhas, relatórios, aprendizado e agentes (sem provedores).
- `atendente` — **funcionário do cliente**. Atendimento: conversas, contatos e
  respostas rápidas.

Regras: o grupo de menu **Plataforma é só do `super_admin`**; `admin` **nunca**
recebe `plataforma_gerenciar`; `super_admin` **nunca** ganha acesso implícito a
dados de tenant. Matriz completa em
[`docs/PERMISSOES.md`](docs/PERMISSOES.md). **Toda tela/função nova declara seu
papel** (permissão no catálogo, rota com guarda, item de menu e linha na matriz).
**Melhorias e telas do control plane (super_admin) são prioridade de produto** —
o dono do sistema precisa enxergar e operar tudo pela superfície de plataforma.

**Segurança (MFA/2FA) é de todos os papéis**, não só do `super_admin`: qualquer
usuário ativo pode **ativar/desativar** a autenticação em duas etapas. Quando
ativada, o login exige o código TOTP. Para o `super_admin` o MFA é obrigatório
(não pode desativar). Fluxo: `internal/web/mfa.go` + `internal/store/mfa.go`
(colunas `usuarios.totp_*`).

**Control plane coeso (não é painel separado)**: a área do `super_admin`
(`web/react/src/features/platform`) é parte do **mesmo sistema**, com o **mesmo
design system** (tokens teal, dark mode, tipografia, cards, abas). Regra: nada de
paleta própria (azul-acinzentada) nem estilos soltos — use as variáveis
(`--ink`, `--ink-soft`, `--muted`, `--line`, `--surface`, `--surface-soft`,
`--surface-muted`, `--brand-primary`, `--mint`, `--teal-*`). As abas e seções
devem ser **organizadas e legíveis**, sem agrupar tudo num bloco só. Ao criar
tela/função de plataforma, reaproveite os componentes e o visual das telas de
admin; se algo parecer "outro produto", está errado.

**Fluxo do super_admin: fácil e simples.** Cada tela tem **uma ação principal
clara**, poucos campos obrigatórios e **defaults sensatos**. O onboarding de um
cliente é **uma etapa só**: criar a empresa já cria o **primeiro administrador**
(o backend `POST /api/react/platform/tenants` aceita `admin` e provisiona os dois
atomicamente). Nunca obrigar o operador a caçar funções em telas densas: separar
por item de menu, com nomes diretos.

## Stack e estrutura

- Go 1.2x, PostgreSQL (pgx/v5), frontend **React/TypeScript (Vite)** em
  `web/react` (feature `wizard/` para a configuração guiada; notificações de
  Inbox em `shared/notifications.ts`). Backend sem framework web.
- `main.go` → `internal/web` (painel + API React), `internal/channel`
  (whatsapp/webchat/telegram + **Adapter/Registry**), `internal/agent` (motor de
  IA/prompt + `runtime_tools.go` — skills/MCP como tools de runtime — e
  `testrun.go` — dry-run do wizard), `internal/mcp` (cliente MCP: stdio + HTTP
  JSON-RPC; `initialize`/`tools/list`/`tools/call`), `internal/llm` (client
  OpenAI-compat + preset Notreve IA), `internal/tools`
  (+ `internal/tools/policy` — catálogo/risco/escopos/auditoria), `internal/store`
  (SQL; `skills.go` e `mcp.go` para skills/MCP), `internal/db` (migrations),
  `internal/agendamento` (scheduler de tarefas `once`/`interval`/`cron`),
  `internal/webhooks` (out) + `internal/webhookoutbox` (worker de delivery),
  `internal/infra`.
  **Não há módulo de pagamentos** (cobrança/billing é externo; pagamentos
  conversacionais tipo banco digital ficam num projeto separado).
  O painel expõe o **log de entregas** da outbox em
  `GET /api/react/webhooks/deliveries` (Integrações → Webhooks). A publicação
  automática dos eventos internos (mensagens IA/humano, conversas,
  transferências, avaliações) é feita via `store.PublishOutboxEvent` de forma
  **best-effort** — nunca pode interromper o atendimento.
- **Migrations**: `migrations/NNN_*.sql`, aplicadas em ordem pelo runner
  (`internal/db/migrate.go`) e rastreadas em `schema_migrations`. **Sempre crie
  uma nova migration; nunca edite as antigas.** Recentes: **064**
  `channels.agent_id` (vínculo canal→agente), **065** skills/MCP
  (`skills`/`agent_skills`/`mcp_servers`/`mcp_tools`/`agent_mcp_servers`),
  **066** `knowledge_chunks` (RAG por full-text) + `settings.knowledge_top_k` e
  **071** remoção do modelo legado `agents.canal_ids`/`agents.canais`/
  `agents.channels_managed` (com backfill para `channels.agent_id`),
  **073** `agendamentos` (tarefas `once`/`interval`/`cron` + scheduler) e
  **074** `log_entries` (logs estruturados por tenant com RLS) + config de
  geração de imagem e retenção de logs em `settings`.
- **Novo canal**: implemente `channel.Adapter` (`Send`/`SendMedia`) e registre
  no `channel.Registry` — IA/webhook/painel ficam intocados (não reescreva o núcleo).
- **Mensagens ricas**: o motor (`internal/agent`) pode produzir `agent.RichMessage`
  (botões/lista/enquete/contato/localização/figurinha) via marcador da IA, menu de
  1º contato, chat flow e satisfação. A entrega é em `channel` (Notreve), sempre
  **com fallback para texto numerado** caso o canal não suporte ou a API falhe —
  publicação rica nunca pode quebrar o atendimento.

## Regras de conduta (obrigatório)

1. **Só edite o que for pedido.** Não refatore por conta própria nem "aproveite
   pra melhorar" código não relacionado.
2. **NUNCA adicione comentários ao código** a menos que o usuário peça.
3. **NUNCA commite segredos** (API keys, tokens, senhas). Segredos vivem no banco
   (configurados pelo painel) ou em variáveis de ambiente — nunca no repo.
   APIs nunca devolvem chave/token (só `configured`).
4. **NUNCA invente preços/produtos da Notreve.** A IA usa a base de conhecimento
   do agente (`knowledge` + `knowledge_links`, PDF/site/áudio). Se faltar
   informação, diga que não tem e avise o usuário para preencher no painel.
5. **Migrações são imutáveis.** Para mudanças de schema, crie `migrations/NNN_*.sql`.
6. **Sempre rode `go build -buildvcs=false ./...`, `go vet ./...` e
   `go test ./...`** (e `npm run build`/`tsc --noEmit` ao tocar React) e corrija
   antes de entregar.
7. **Deploy** na VPS configurada pelo operador (`APP_DIR` e usuário fora do
   repo): `docker compose up -d --build` após subir os arquivos. Confira
   `docker compose logs app --tail`. **RV: nunca ativar RLS/schema que quebre as
   queries do store sem validação em staging.**
8. **Backup do Postgres**: na VPS há `backup-pg.sh` (dump diário 03:00 via cron,
   retenção 14 dias em `/root/pg-backups/`) e `restore-pg-test.sh` (valida o
   restore). Nunca remova o volume `pgdata` sem backup recente.
9. **Tudo deve funcionar.** Não entregue recurso "desligado", "desconectado" ou
   "sem função": se está no painel, precisa operar de ponta a ponta. Se depende
   de config do cliente, o fluxo tem que deixar isso claro e ser validável —
   nunca um botão morto.

## Regra de dependências e paralelismo (ANTES de qualquer mudança)

1. **Dependências do que mudou**: quem usa essa função/tabela/campo/endpoint?
   (ex.: mudar `Processor.Process` afeta `notreve.go`, `webchat.go`, `telegram.go`).
   Procure todos os chamadores com grep antes de assinar/renomear.
2. **Caminhos paralelos**: fluxos que compartilham o mesmo código — webhook
   WhatsApp, webchat, telegram, envio de atendente, campanhas, follow-up,
   IA-na-fila, satisfação, chat flow. Uma mudança num desses quase sempre afeta os outros.
3. **API externa (Notreve WhatsApp)**: o contrato novo está na coleção Postman do
   dono (webhook com `phone`/`phone_jid`/`is_from_me`/`message_type`/`media_link`/`event`,
   eventos `Delivery` e `CallOffer`, rejeição de chamada em `/call/reject` com
   `call_from` + `call_id`, mídia `base64|s3|both`, HMAC `x-hmac-signature`). A
   assinatura HMAC da API é **opcional**: sem o cabeçalho o webhook deve ser
   aceito; se o cabeçalho vier, uma assinatura inválida deve ser rejeitada.
   Nunca torne o HMAC obrigatório sem confirmar mudança no contrato da API.
   Verifique o endpoint/parâmetro antes de usar (fonte da verdade: a API real).
   A documentação viva da API Notreve WhatsApp (no painel do produto contratado)
   é a fonte de verdade para nomes e formatos. Em novas requisições, emita
   `snake_case` e não use aliases marcados como `deprecated`; mantenha suporte
   de leitura aos formatos legados somente quando necessário para compatibilidade.
4. **Provider de IA**: texto x multimodal x STT têm comportamentos diferentes —
   nunca assuma que todo provider "vê"/"ouve".

Depois de mudar, rode o build e, se afetar comportamento real, teste na VPS e
descreva o que foi verificado.

## Teste proativo (obrigatório — não esperar o usuário achar bug)

Sempre que tocar em fluxo real (webhook, chat, mídia, canais, avaliação, jobs),
**teste você mesmo de ponta a ponta** antes de entregar:

1. **Mensagens de todos os tipos**: texto, áudio, imagem, vídeo, PDF/documento,
   sticker, contato, localização — entrada e saída (inclui mídia via base64 e
   `mediaLink`/s3).
2. **Ciclo completo**: receber → IA responder → envio → painel exibir.
3. **Reinício de WhatsApp**: simule desconectar/reconectar e confira que os envios
   voltam e nada duplica.
4. **Reinício do app**: `docker compose restart app` e confira migrations/jobs.
5. **Páginas do painel**: todas retornam 200 e sem erro no log de render.
6. **Erros silenciosos**: procure `_ =` engolindo erro em caminho crítico e logue.

Se achar qualquer bug, **corrija antes de entregar** — não deixe pro usuário.

## QA como código (gate obrigatório)

Além do agente de QA do opencode, parte do QA vive **no próprio código** e roda
no `go test ./...`. Os invariantes ficam em
[`internal/web/qa_invariants_test.go`](internal/web/qa_invariants_test.go) e são
determinísticos (sem rede/DB): leem o código-fonte e travam regressões.

O que eles travam:

- **Rotas**: toda rota `/api/react/*` tem guarda (`requireReactPerm`/
  `requireSuperAdmin`/`requireReactModule` ou guarda interna conhecida); rotas
  abertas só `GET /api/react/session` e `GET /api/react/bootstrap`.
- **Control plane**: todo `/api/react/platform/*` usa `requireSuperAdmin`, tanto
  em `server.go` quanto no dispatch do `SaaSGuard` (`saas_guard.go`).
- **Permissões**: `permCatalog` sem chave duplicada/órfã; `roleDefaultPerms`
  referencia só permissões existentes; toda constante `perm*` está no catálogo e
  `admin` nunca tem `plataforma_gerenciar`.
- **Migrations**: nome `NNN_*.sql`, arquivo não vazio e número novo não
  duplicado (colisões históricas imutáveis são toleradas no baseline do teste —
  migrations **nunca** são renomeadas/editadas).
- **Wiring**: todo pacote interno é referenciado e não há handler órfão
  (definido e nunca chamado).
- **Menu**: todo item de `navigation.ts` tem `module` + permissão, usa permissão
  existente no catálogo e o grupo `plataforma` só tem `plataforma_gerenciar`.

Ao mexer em rota, permissão, menu, migration ou wiring, **estenda o invariante na
mesma mudança**. Se um invariante apontar problema real, **corrija o problema**
(não afrouxe o teste).

O **agente de QA** ([`.opencode/agent/qa.md`](.opencode/agent/qa.md)) é
obrigatório antes de entregar: ele mapeia dependências e caminhos paralelos
(webhook WhatsApp, webchat, telegram, envio de atendente, campanhas, follow-up,
IA-na-fila, satisfação, chat flow), revisa o diff, roda o gate
(`go build -buildvcs=false ./...`, `go vet ./...`, `go test ./...`,
`tsc --noEmit`), audita permissões e sugere melhorias com `arquivo:linha` e
`Status: PASS/FAIL`.

## Arquitetura rápida (leia `docs/ARQUITETURA.md`)

- **Canais**: tabela `channels` (whatsapp, webchat, telegram). Cada WhatsApp =
  número (base_url + token da API Notreve). `can_campaign` libera p/ campanha.
  `channel.Adapter` + `channel.Registry` roteiam o envio de IA/humano por canal.
  **Bot detection (segurança)**: config `detectar_bots` (off por padrão) —
  quando ligada, padrão de texto (`bot_patterns`) ou rajada (6+/60s) transfere a
  conversa para humano com nota, sem encerrar. **Saúde**: colunas
  `health_status`/`last_checked_at`/`last_error` gravadas pelo
  monitor de canais (`internal/channel/monitor.go`, verifica WhatsApp via
  `/session/status` e Telegram via `getWebhookInfo`). Em transição real de
  estado, enfileira `channel.status_changed` na outbox (alerta de desconexão).
- **Estados de conversa**: `ia` · `humano_aguardando` · `humano_atendendo` · `closed`.
  A IA SÓ responde em estado `ia`. Satisfação: `aguardando`/`aguardando_operador`/`finalizada`/`sem_resposta`.
- **Regra de transferência**: a IA NUNCA transfere por conta própria (nem por
  horário, nem por incerteza). Só transfere se o cliente PEDIR um humano
  (`[HUMANO]`) ou setor (`[SETOR:Nome]`), ou em falha/erro. (Regra nº 1 do produto.)
- **Tools**: catálogo + política (risco, escopos, confirmação), allowlist de host,
  anti-SSRF, timeout, auditoria. **Motor de regras (evolução)**: uma tool pode
  exigir identificação do cliente com modos **phone/challenge/resolve** via um
  **verifier externo** configurável (`identity_verify_url` + param/match/client_id).
  Os argumentos reservados `_identity`/`_challenge_answers` são extraídos pelo
  guard e **nunca** vão para a API externa. Aditivo/opt-in. Credenciais nunca no repo.
- **Provedores de IA (linguagem + STT)**: cadastrados **pelo admin (por empresa)**.
  O **super_admin NÃO cadastra provedor** (fica no control plane). **Sem provedor o
  bot não funciona.** Sempre exibir o **preset "Notreve IA (recomendado)"** como
  padrão (só o token; a Notreve gerencia endpoint/modelo/transcrição). IA própria:
  `base_url`+`model`+token com `capacidade` `texto|multimodal` (com/sem visão) e
  `safe_mode`. **Notreve IA é outro sistema** (limites/cobrança são dela) — aqui é
  só a integração/token.
- **Allowlist**: o webhook só responde a números listados em
  `settings.numeros_permitidos`. NÃO vale para campanhas (envio explícito do admin).
- **Permissões**: `usuarios.role` = super_admin | admin | supervisor | atendente.
  Permissões granulares em `usuarios.permissions`; vazio = padrão do perfil.
  Módulos em `modulos` + cache `Server.modules`. Rotas usam `requirePerm(...)`.
  **Matriz completa em [`docs/PERMISSOES.md`](docs/PERMISSOES.md).** A fila `ia`
  (conversas em estado `ia`) é controlada pela permissão granular
  `conversas_ver_ia` — nunca por papel hardcoded: sem ela o atendente não vê a
  fila da IA. Regras:
  (1) **toda tela/função nova deve declarar seu papel** (permissão no
  `permCatalog`, rota com `requireReactPerm`/`requireSuperAdmin`, item em
  `navigation.ts` e linha na matriz);
  (2) **o grupo de menu `Plataforma` é só `super_admin`** (tenants, planos,
  servidores, white-label global) — configurações da empresa (atendimento,
  LGPD) ficam em `administracao`/`operacao`;
  (3) `super_admin` não opera dados de cliente e `admin` nunca recebe
  `plataforma_gerenciar`. O agente de QA
  [`.opencode/agent/qa.md`](.opencode/agent/qa.md) audita isso.
- **Isolamento**: `tenant_id` em todas as tabelas de negócio + políticas de RLS
  registradas (migration 039, **não ativadas** até o store migrar p/ `WithTenantTx`).

## Wizard, canal→agente, RAG, MCP e skills (recente)

- **Wizard de configuração** (`web/react/src/features/wizard`): a ordem é
  **provedor de IA → agente → canal**, e o vínculo define **qual agente atende**
  cada canal. O passo "Testar" usa `POST /api/react/agents/{id}/test`
  (**dry-run**): roda o motor real com histórico efêmero, **sem persistir**
  conversa/mensagem/consumo. Não pule a ordem nem deixe o agente sem provedor.
- **Vínculo canal→agente**: `channels.agent_id` é a **única fonte de verdade**
  do vínculo (migration 064). O modelo legado (`agents.canal_ids`,
  `agents.canais`, `agents.channels_managed`) foi removido na migration 071.
  Resolução (`internal/channel/process.go`): agente da conversa →
  `channels.agent_id`. O vínculo é gravado **só pelo canal**
  (`PATCH/POST /api/react/channels` com `agent_id`); a aba Canais do agente é
  **somente leitura** (mostra onde o agente atende e atalha para Canais).
- **RAG por relevância**: o conhecimento é quebrado em trechos
  (`knowledge_chunks`, `tsvector`/GIN, migration 066; trigger + backfill) e o
  motor busca **top-K** relevantes por agente (`internal/agent/knowledge.go` →
  `store.SearchKnowledgeChunks`) em vez de despejar toda a base no prompt.
  `settings.knowledge_top_k` (default 6, faixa 1–20) controla o K.
- **MCP** (`internal/mcp`): cliente mínimo do Model Context Protocol — handshake
  `initialize`, `tools/list` e `tools/call` — em transporte **stdio** e **HTTP
  JSON-RPC**. Servidores/tools em `mcp_servers`/`mcp_tools` (migration 065),
  rotas `/api/react/mcp` (+ `/test`, `/sync`), vínculo por agente. As tools MCP
  entram no ciclo LLM como **runtime tools** (`internal/agent/runtime_tools.go`)
  e os segredos de `env`/`headers` são cifrados. **Limitações conhecidas**:
  sessão **não persistente** (cada chamada abre/fecha via `CallToolOnce`), sem
  SSE legado (só JSON-RPC) e sem policy/audit dedicados (diferente das tools de
  `integrations`).
- **Skills** (`skills`/`agent_skills`, migration 065): pacotes de instruções
  carregados sob demanda (**progressive disclosure**) pela meta-tool
  `invocar_skill`. O system prompt lista só nome+descrição; as instruções
  completas entram quando a IA invoca. `EnsureDefaultSkills` semeia 3 skills
  padrão (atendimento/vendas/suporte) — idempotente, não sobrescreve edições.
- **Chat Flow**: o fluxo **começa obrigatoriamente pelo nó `start`** (um por
  fluxo; o motor entra por ele) e tem o bloco **`queue`** para setor/fila. Demais
  blocos: `message`/`collect`/`condition`/`intent`/`tool`/`end`.
  O bloco legado `transfer` foi unificado em `queue` (migration 075 normaliza
  `chatflow_steps`): são a mesma função (enfileirar no setor e aguardar humano),
  então existe só um bloco na paleta.
  Blocos adicionais (todos funcionais no motor `internal/agent/chatflow.go`):
  - `menu` — menu de opções com uma saída por opção (`option-N`) + `default`;
    casa a resposta pelo número, rótulo ou valor; usa botões/lista quando o
    canal suporta e cai no texto numerado. Pausa o fluxo (estado em
    `conversations.chatflow_step`) e retoma na resposta.
  - `delay` — espera N segundos/minutos, **limitada a 30s** para não travar o
    pipeline (acima disso segue).
  - `set_variable` — define/atualiza variável com interpolação `{{var}}` e
    operações `set`/`append`/`prepend`/`add`.
  - `goto` — salta para o `ord` alvo com **limite de `maxChatflowJumps` (24)**
    saltos; destino inexistente é ignorado.
  - `business_hours` — ramifica `open`/`closed` pelo horário do setor ou por
    `abertura`/`fechamento`/`dias`, sempre em Brasília (`brazilNow`/`setorAberto`).
  - `tag` — aplica tag(s) ao contato da conversa (por telefone, sem duplicar).
  - `satisfaction` — dispara a pesquisa de satisfação reusando o estado/fluxo
    existente (`SatisfactionParticipants`/`satisfacao_*`), com botões 1–5 e
    desvio `skip` quando não há participantes.
  - `note` — só anotação no canvas; **não executa nada** (passa adiante).
  - `http` — requisição genérica pelo mesmo executor/política das tools
    (`internal/tools`), salvando o resultado em variável.
  Nunca gere fluxo sem `start`. A validação ao salvar (`internal/web/api_react_chatflow.go`)
  exige exatamente um `start`, ramos coerentes de `menu`/`condition`/`intent` e
  `goto` apontando para bloco existente.
  **Modo de atendimento (fonte única)**: `channels.mode` (`ai` | `chatflow`,
  default `ai`) e `channels.chatflow_id` são a única fonte de verdade do modo.
  `mode=ai` → o canal usa `channels.agent_id` (IA livre); `mode=chatflow` → o
  canal usa `channels.chatflow_id` (fluxo guiado), e o bloco de IA dentro do
  fluxo pode usar o agente do canal. O motor resolve via
  `store.ResolveActiveChatflow` (`internal/store/chatflow.go`), chamado em
  `internal/agent/chatflow.go` (`resolveChatflowFor`) a partir de
  `Engine.HandleUserMessage`, que recebe o `ChatflowTarget` (modo + flow do
  canal) montado em `internal/channel/process.go`. Regra: canal em `chatflow`
  com flow **ativo** → roda o flow; flow **inativo**, sem vínculo ou canal em
  `ai` → **IA livre**. A gravação é por `store.SetChannelChatflow` (usado em
  `POST /api/react/chatflows/{id}/channels`) e pelo editor do canal
  (`PATCH/POST /api/react/channels`). O modelo antigo `agents.chatflow_id` foi
  removido na migration 076. Não crie outra coluna/tabela para esse vínculo.
- **Notificações**: `web/react/src/shared/notifications.ts` (som via WebAudio,
  notificação desktop e badge no título) acionado pelo **polling** do Inbox
  (`InboxPage`, incremental por `updatedAt`).
- **Branding**: a marca da **plataforma** é só do `super_admin`
  (`/api/react/platform/branding`). A marca do **tenant/widget** é configurada
  no **canal Webchat** (aplicada ao widget público). `/api/react/branding`
  permanece restrito ao `super_admin` (fallback legado).
- **Provedor Notreve IA**: preset recomendado. O endpoint é fixo em
  `llm.NotreveBaseURL` (`https://api.ia.notreve.com.br/v1`), com modelos
  `notreve-v1-pro`/`notreve-vision`; o painel lista modelos
  (`POST /api/react/providers/models`), testa a conexão e salva o modelo
  escolhido. Só o token é do cliente.
- **Mídia**: áudio é servido **como bytes com `Content-Type` tocável**
  (`internal/web/saas_conversations.go`, `messageMediaContentType`) e o player
  usa a própria URL como `src`. **Limitação conhecida**: OGG/Opus não toca no
  Safari.
- **Conversa fantasma**: **chamada NÃO cria conversa** — `handleCall` só registra
  na conversa já aberta, se existir. Filtros reforçados de status/grupo/newsletter
  e telefone **E.164** validado (`whatsappConversationPhoneValid`); inbound
  inválido é ignorado sem abrir conversa.

## Geração de mídia e logs (recente)

- **Geração de imagem**: a runtime tool `gerar_imagem(prompt, aspect_ratio?)` só
  existe quando o agente tem provedor com chave e capacidade de imagem
  (`providers.capabilities` com `imagem`/`image`, ou `base_url` OpenRouter) e a
  conversa é real (dry-run do wizard não envia). O client
  (`internal/llm`) chama `/chat/completions` com `modalities: ["image","text"]` +
  `image_config.aspect_ratio` (compatível com OpenRouter). A imagem sai pelo
  caminho de mídia existente (`Server.SendGeneratedMedia`), é persistida como
  mensagem do assistente e é best-effort. Config em `settings.image_model` e
  `settings.image_aspect_ratio` (Configurações). Provedor sem suporte devolve
  erro claro.
- **Logs estruturados**: `internal/applog` grava no stdout e em `log_entries`
  (migration 074) de forma best-effort (nunca quebra a operação). Endpoints
  `GET /api/react/logs` (filtros `level`/`module`/`q`/`from`/`to` + paginação
  padrão) e `/logs/stats`; permissão granular **`logs_ver`**, módulo `config`,
  tela **Logs** no grupo Administração. Retenção por tenant via
  `settings.logs_retencao_dias` (default 30) em job periódico no `main.go`.

## Agendamentos (tarefas programadas)

- Tabela `agendamentos` (migration 073) + scheduler em `internal/agendamento`
  (`once`/`interval`/`cron`, próxima execução em Brasília). Ações
  `enviar_mensagem`, `rodar_ferramenta` e `callback_agente` são best-effort
  (falha marca `falhou` com mensagem, nunca derruba o app) e re-agendadas no
  boot. API `/api/react/schedules` (+ `/{id}/trigger`), permissão
  `agendamentos_gerenciar` e módulo `agendamentos`. A IA cria agendamentos pela
  runtime tool `agendar_tarefa`. UI em `web/react/src/features/schedules`.

## Dashboard (Visão geral)

- Endpoint **`GET /api/react/dashboard?period=hoje|7d|30d`** (default 7d), guarda
  `conversas_ver` + módulo `conversas`. O `/api/react/overview` legado permanece
  (polling de notificações do Inbox) — não remova.
- Agregações em SQL em `internal/store/dashboard.go`, escopadas por tenant e pelo
  papel (reusa `conversationScopeFilter`, a mesma regra de
  `ListConversationsForUser`). Sem N+1. Tempo de espera da fila é medido desde a
  primeira mensagem do cliente sem resposta (não por `updated_at`, que muda a
  cada mensagem).
- **Gating por seção**: `satisfaction`/`sectors` só com `relatorios_ver`;
  `failures` só com `aprendizado_ver` (`permissions` no payload + UI esconde).
- UI: `web/react/src/app/App.tsx` (`OverviewPage`) + CSS `.dashboard-*` em
  `styles/app-shell.css` (KPIs, mini-gráfico de volume sem lib, top setores,
  carga por atendente, alerta de falhas, filtro de período, loading/erro claros).
- Testes: `internal/web/api_react_dashboard_test.go` (período, volume, taxa,
  gating) e `internal/store/dashboard_test.go` (escopo por papel).

## Regras de negócio sensíveis (não quebrar)

- **O produto NÃO obriga IA.** Damos os blocos e o cliente monta: **só IA**,
  **chatbot tradicional** (setores/filas/mensagens/condições, sem IA) ou **híbrido**.
  Os blocos são os mesmos (`start` obrigatório → `message`/`collect`/`condition`/
  `intent`/`menu`/`tool`/`http`/`set_variable`/`delay`/`business_hours`/`tag`/
  `agent`(opcional)/`queue`/`satisfaction`/`goto`/`end`/`note`).
  **Setores e filas
  são blocos nativos** — nunca tornar a IA obrigatória; ela é uma opção (um
  bloco), não pré-requisito.

- **Segurança anti-bloqueio no WhatsApp**: campanhas usam intervalo entre
  envios, limite diário, cooldown por canal e failover. `modo_teste` marca como
  enviada SEM chamar a API. Nunca facilitar envio em massa sem essas proteções.
- **IA monitora espera** (`ia_monitora`) e **follow-up** só enviam via Sender
  (respeitam allowlist). **Distribuição automática** atribui `humano_aguardando`
  ao atendente online (setor + horário Brasília + menos carregado), com
  priorização por habilidade/nicho quando cadastrada.
- **Horários**: todo cálculo usa **Brasília** (`nowBR()` no web, `brazilNow()` no
  agent). Containers rodam `TZ=America/Sao_Paulo`.

## Fluxo para mudanças

1. Entenda o estado atual (banco + código) antes de mexer.
2. Faça a mudança mínima seguindo os padrões existentes.
3. Rode o gate completo (build/vet/test+npm). Teste na VPS se afetar comportamento real.
4. Commit claro (pt-BR) e push apenas quando o usuário pedir (ou ao final de uma tarefa acordada).

## Roadmap e ideias

Novas features/ideias entram no **docs/ROADMAP.md** (não fica tudo no código).
Implemente seguindo a prioridade das fases; se o usuário pedir algo novo, adicione
no roadmap e implemente quando ele confirmar. **Nada fora das 4 frentes vira
prioridade** — se não servir a elas, fica na fila.

## Produtos-base e referências

- **Produtos-base**: Notreve WhatsApp API (contrato do webhook/canal) e
  Notreve IA (provedor). Credenciais de integração nunca ficam no repositório.
- **`notreve-agent-engine` (Python)** — motor irmão de referência: RAG por
  relevância (chunks + full-text), cliente MCP
  (stdio/HTTP JSON-RPC), skills com progressive disclosure / meta-tool
  `invocar_skill` e as runtime tools. Ao evoluir o motor, verifique se o engine
  Python já resolveu antes de inventar.
