# Arquitetura — Notreve Chat Community

Visão geral do sistema para futuros agentes de IA e desenvolvedores.
Mantenha este documento atualizado quando a arquitetura mudar.

## Proposta

Painel interno (reseller) de atendimento WhatsApp com IA + humanos. A IA
(LLM compatível com OpenAI — hoje DeepSeek) atende 24/7 usando a base de
conhecimento configurada por agente; quando o cliente pede um humano, a conversa
vai para um atendente (com distribuição automática).

## Stack

| Camada | Tecnologia |
|---|---|
| Linguagem | Go |
| Banco | PostgreSQL 16 (pgx/v5), migrations em `migrations/*.sql` |
| Templates | `html/template` (sem framework) |
| Cache/Sessões | Redis (opcional, fallback em memória) |
| Mensageria | RabbitMQ (opcional, eventos) |
| Deploy | Docker Compose no diretório configurado da VPS, porta 8080 |

## Pacotes (`internal/`)

- `web` — painel HTTP, sessões, permissões, módulos, jobs de automação
  (distribuição, follow-up, paciência, satisfação, campanhas).
- `channel` — canais de entrada/saída:
  - `notreve.go` — webhook WhatsApp (API Notreve) + envio (texto/mídia/edição/reação).
  - `webchat.go` — widget de chat no site.
  - `process.go` — roteador: cria/acha conversa, decide IA vs humano.
- `agent` — motor de IA: monta o system prompt, chama o LLM, resolve tools.
  `runtime_tools.go` injeta skills e tools MCP no ciclo; `testrun.go` é o
  dry-run do wizard; `knowledge.go` faz a recuperação top-K do conhecimento.
- `mcp` — cliente Model Context Protocol (stdio + HTTP JSON-RPC): handshake,
  `tools/list`, `tools/call`.
- `store` — acesso a dados (SQL); `skills.go` e `mcp.go` cobrem skills/MCP.
- `db` — runner de migrations.
- `tools` — executor de ferramentas (integrações/API externas).
- `agendamento` — scheduler de tarefas (`once`/`interval`/`cron`): cálculo da
  próxima execução, disparo best-effort das ações e re-agendamento no boot.
- `applog` — logger estruturado: grava no stdout e, best-effort, em
  `log_entries` (nunca falha a operação). Retenção por tenant em job periódico.
- `infra` — Redis/RabbitMQ (opcionais, com fallback).

## Modelo de dados (principais tabelas)

- `settings` — IA padrão, WhatsApp, allowlist de números, flags de automação.
- `providers` — provedores de LLM (base_url, model, api_key).
- `agents` — agentes de IA (system_prompt, knowledge, knowledge_links,
  knowledge_content, provider_id, setores, tools).
- `channels` — canais: whatsapp (número: base_url+token) e webchat (public_id).
  `can_campaign` libera o número para campanhas; `agent_id` é a **única fonte de
  verdade** do vínculo canal→agente (migration 064). O modelo legado
  (`agents.canal_ids`/`agents.canais`/`agents.channels_managed`) foi removido na
  migration 071.
- `knowledge_chunks` — trechos do conhecimento com `tsvector`/GIN para busca por
  relevância (migration 066); `settings.knowledge_top_k` define o top-K.
- `skills`/`agent_skills` — pacotes de instruções (progressive disclosure)
  vinculáveis a agentes (migration 065).
- `mcp_servers`/`mcp_tools`/`agent_mcp_servers` — servidores MCP (stdio/HTTP) e
  suas tools expostas à IA (migration 065).
- `conversations` — atendimentos (status, setor, atendente, satisfação, followup).
- `messages` — histórico (role, source, wa_msg_id).
- `contatos` — base de contatos; `segmentos`/`contato_segmentos` = listas/público.
- `grupo_canais`/`grupo_canal` — grupos de números WhatsApp para campanhas.
- `campanhas`/`campanha_envios` — envio em massa (agendamento, failover, modo teste).
- `integrations` — ferramentas que a IA pode chamar (ex.: SMS — é tool, não canal).
- `setores`, `atendentes`, `atendente_setores`, `usuarios` — equipe e permissões.
- `mensagens` — textos padrão do sistema (transferência, follow-up, satisfação...).
- `respostas_rapidas`, `aprendizados`, `avaliacoes`, `modulos`.
- `log_entries` — logs estruturados por tenant (migration 074): `ts`, `level`,
  `module`, `message`, `extra` (JSONB), `traceback`, `request_id`; RLS por
  `tenant_id` e índices por `(tenant_id, ts/level/module)`.
- `settings.image_model` / `image_aspect_ratio` / `logs_retencao_dias`
  (migration 074) — config de geração de imagem e retenção de logs.

## Fluxos principais

### Entrada WhatsApp
1. Webhook → `channel.Notreve.ServeHTTP` (valida allowlist de números).
2. `Processor.Process` → acha/cria conversa (reusa a aberta do mesmo telefone).
3. Se status `ia` → IA responde (prompt com base de conhecimento + horários).
4. IA só transfere por pedido explícito (`[HUMANO]`/`[SETOR:...]`) ou falha.
5. Transferência → `humano_aguardando` → job de distribuição atribui ao atendente.

### Atendimento humano
- Atendente vê conversas, aceita (`aceitar`), responde (envia no WhatsApp),
  transfere de setor, encerra (inicia satisfação).

### Automações (jobs a cada 60s em `web.Server.Start`)
- `reconcileOnline` — online/offline por sessão.
- `distributeJob` — atribui `humano_aguardando` ao atendente disponível.
- `followupJob` — reativa contato que parou de responder (intervalo configurável).
- `patienceJob` — mensagem de paciência quando humano demora (ia_monitora).
- `satisfacaoTimeoutJob` — encerra satisfação sem resposta.
- `campanhaJob` — envia campanhas em lotes com failover entre canais.
- `agendamentosJob` — dispara os agendamentos vencidos (ver abaixo).

### Agendamentos (tarefas programadas)
- Tabela `agendamentos` (migration 073): `tenant_id`, `nome`, `tipo`, `quando`,
  `acao`, `payload` JSONB, `status`, `max_execucoes`, `execucoes`,
  `proxima_execucao`, `canal_id`, `telefone`, `agente_id`, `criado_por` e
  timestamps. RLS pelo mesmo `tenant_row_visible` das demais tabelas de negócio.
- **Tipos**: `once` (data/hora), `interval` (a cada N segundos, mínimo 10) e
  `cron` (expressão de 5 campos, interpretada em **Brasília**).
- **Ações** (best-effort, nunca derrubam o app):
  - `enviar_mensagem` — envia texto pelo `channel.Adapter`/`Registry` do canal.
  - `rodar_ferramenta` — executa uma integração do tenant (`internal/tools`).
  - `callback_agente` — grava um prompt como mensagem e reativa o `Engine`
    (follow-up); a resposta é enviada ao contato.
- **Scheduler** (`internal/agendamento`): um ticker de 20s roda por tenant ativo
  (`web.Server.Start`). `NextRun` calcula a próxima execução; `RunOne` executa a
  ação e decide o novo status (concluída, pendente ou falhou); panics viram
  `falhou` com mensagem. No boot, pendentes sem `proxima_execucao` são
  re-agendados. A execução é reservada com `ClaimAgendamentoForTenant` (atômico).
- **API**: `GET/POST /api/react/schedules`, `PATCH/DELETE
  /api/react/schedules/{id}` e `POST /api/react/schedules/{id}/trigger`
  ("disparar agora"), com paginação/filtros. Permissão granular
  `agendamentos_gerenciar` e módulo `agendamentos`.
- **Tool da IA**: `agendar_tarefa` (runtime tool do `internal/agent`) permite à
  IA criar agendamentos (ex.: follow-up) para a conversa atual.
- **UI**: `web/react/src/features/schedules` (menu **Operação → Agendamentos**).

### Dashboard (Visão geral)
- Endpoint `GET /api/react/dashboard?period=hoje|7d|30d` (default 7d), com guarda
  `conversas_ver` + módulo `conversas`. O `/api/react/overview` legado continua
  existindo para o polling de notificações (badge/som), sem alterações.
- Agregações **em SQL**, escopadas por tenant e pelo papel do usuário (mesma
  cláusula de `conversationScopeFilter` usada por `ListConversationsForUser`):
  - `store.DashboardConversations` — conversas por status (ia, aguardando,
    atendendo, encerradas), novas no período e total.
  - `store.DashboardQueue` — fila `humano_aguardando` + tempo de espera
    (maior/médio), medido desde a primeira mensagem do cliente ainda sem
    resposta (não usa `updated_at`, que muda a cada mensagem).
  - `store.DashboardResolution` — resolvidas pela IA vs transferidas para humano.
  - `store.DashboardSatisfaction` — média e nº de avaliações (CSAT).
  - `store.DashboardVolume` — mensagens recebidas/enviadas por dia.
  - `store.DashboardSetores` — top setores por conversas no período.
  - `store.DashboardAtendentes` — carga atual por atendente (ativas/aguardando).
  - `store.CountOpenAgentFailures`/`CountAgentFailuresByType` — falhas do agente.
- **Gating de seções**: `permissions.reports` (`relatorios_ver`) libera
  `satisfaction` e `sectors`; `permissions.failures` (`aprendizado_ver`) libera
  `failures`. A UI esconde o que o operador não pode ver.
- **UI** (`web/react/src/app/App.tsx` → `OverviewPage` + CSS em
  `styles/app-shell.css`): KPIs (novas, em aberto, fila/tempo, CSAT, resolução
  pela IA), mini-gráfico de volume (barras CSS, sem lib), top setores, carga por
  atendente, alerta de falhas em aberto com link para Aprendizado, filtro de
  período e estados de loading/erro explícitos. Dado ausente vira `—`.

### Relatórios (tela analítica acionável)
- Endpoint **`GET /api/react/reports`** (guarda `relatorios_ver` + módulo
  `relatorios`) com filtros **período** (`hoje`/`7d`/`30d`/`custom`), **canal**,
  **setor** (`sector_id`) e **atendente** (`attendant_id`); a lista de
  atendimentos avaliados usa a paginação padrão (`api_react_pagination.go`,
  `limit` + `cursor`).
- Agregações em `internal/web/api_react_reports.go` sobre as conversas do
  período e seus `messages`/`avaliacoes`: volume por dia, por atendente e por
  setor; **tempo médio de resposta**, **tempo médio de espera por humano**,
  **tempo médio de atendimento**, **CSAT** (notas ≥ 4), **NPS**, **taxa de
  transferência**, **taxa de encerramento** e **resolução IA × humano**.
- **Exportação CSV aditiva**: **`GET /api/react/reports/export`** (mesma guarda
  `relatorios_ver`) devolve `text/csv` com o resumo do período e a linha de cada
  atendimento (protocolo, canal, setor, atendente, status, mensagens, tempos e
  notas). Usa `encoding/csv` (sem dependências novas) e escapa vírgulas,
  aspas e quebras de linha.
- UI: `web/react/src/features/reports/ReportsPage.tsx` + `Reports.css` (teal +
  dark mode): filtros, KPIs, gráfico de volume e distribuição de notas em
  CSS/SVG inline, tabelas por atendente/setor, lista paginada de atendimentos e
  botão **Exportar CSV**. Estados de loading/erro explícitos; o botão de export
  só aparece com `relatorios_ver`.
- Testes: `internal/web/api_react_reports_test.go` (filtros, agregações de
  tempos/CSAT, linhas e escaping do CSV).

### Campanhas (envio em massa)
1. Criar: nome + mensagem (`{nome}`) + grupo de canais + segmento (público) +
   agendamento + intervalo + limite diário + modo teste.
2. Ao criar, congela o público em `campanha_envios` (status pendente).
3. `campanhaJob` processa em lotes: escolhe o canal menos carregado do grupo,
   respeita cooldown por número e limite diário; canal que falha entra em janela
   de retry (5 min) e os envios seguem pelos demais (failover); todos caíram →
   pausa automática; voltou → retoma. `modo_teste` não chama a API.
4. Mensagens variadas: `PerturbaTexto` insere caracteres de largura zero quando
   `variar_mensagem` está ligado (visual idêntico, bytes diferentes).

### Wizard de configuração
- Passo a passo no painel (`web/react/src/features/wizard`): **provedor de IA →
  agente → canal**, escolhendo qual agente atende cada canal.
- O passo "Testar" chama `POST /api/react/agents/{id}/test` (dry-run): roda o
  motor real com histórico efêmero, sem persistir conversa/mensagem/consumo.

### Recuperação de conhecimento (RAG)
- `knowledge_chunks` guarda trechos do conteúdo (trigger + backfill, migration
  066) com índice `tsvector`/GIN.
- O motor busca os `top-K` trechos mais relevantes para a mensagem
  (`store.SearchKnowledgeChunks`), em vez de enviar toda a base no prompt.
- `settings.knowledge_top_k` (default 6, 1–20) ajusta o K.

### MCP e Skills
- **MCP** (`internal/mcp`): handshake `initialize`, `tools/list` e `tools/call`
  sobre stdio ou HTTP JSON-RPC. Servidores/tools em `mcp_servers`/`mcp_tools`,
  rotas `/api/react/mcp` (+ `/test`, `/sync`), vínculo por agente. Cada tool
  entra no ciclo LLM como runtime tool; segredos de `env`/`headers` são
  cifrados. Sessão não é persistente, sem SSE legado e sem policy/audit.
- **Skills** (`skills`/`agent_skills`): o system prompt lista nome+descrição e a
  IA carrega as instruções completas sob demanda pela meta-tool `invocar_skill`
  (progressive disclosure). Seed padrão: atendimento, vendas, suporte.

### Chat Flow
- O fluxo começa sempre pelo nó **`start`** (um por fluxo; o motor entra por
  ele) e tem o bloco **`queue`** para setor/fila. Demais blocos:
  `message`/`collect`/`condition`/`intent`/`tool`/`end`. O bloco legado
  `transfer` foi unificado em `queue` (mesma função; migration 075 normaliza os
  passos existentes).
- **Blocos adicionais** (executados no motor `internal/agent/chatflow.go`):
  - `menu`: opções com saída própria (`option-N`) + `default`; casa resposta por
    número/rótulo/valor; usa botões/lista quando o canal suporta e cai no texto
    numerado; pausa e retoma pela resposta.
  - `delay`: espera limitada a 30s para não travar o pipeline.
  - `set_variable`: define/atualiza variável com `{{var}}` e operações
    `set`/`append`/`prepend`/`add`.
  - `goto`: salto com limite de 24 saltos (anti-loop); destino inexistente é
    ignorado.
  - `business_hours`: ramifica `open`/`closed` pelo setor ou por
    abertura/fechamento/dias em Brasília.
  - `tag`: aplica tags ao contato da conversa.
  - `satisfaction`: dispara a pesquisa de satisfação (reusa o fluxo existente),
    com botões 1–5 e desvio `skip` sem participantes.
  - `note`: anotação no canvas, sem execução.
  - `http`: requisição genérica pelo executor/política das tools.
  A validação ao salvar (`internal/web/api_react_chatflow.go`) exige um único
  `start`, ramos coerentes de `menu`/`condition`/`intent` e `goto` existente.
- **Modo de atendimento (fonte única)**: as colunas **`channels.mode`**
  (`ai` | `chatflow`, default `ai`) e **`channels.chatflow_id`** são a única
  fonte de verdade do modo. `mode=ai` usa `channels.agent_id` (IA livre);
  `mode=chatflow` usa `channels.chatflow_id` (fluxo guiado). O motor lê via
  `store.ResolveActiveChatflow` (`internal/store/chatflow.go`) e grava por
  `store.SetChannelChatflow`; o `ChatflowTarget` (modo + flow do canal) é
  montado em `internal/channel/process.go`.
- **Regra**: canal em `chatflow` com flow **ativo** roda o flow e a IA livre é
  ignorada (`internal/agent/chatflow.go` → `resolveChatflowFor`, chamado em
  `internal/agent/engine.go:HandleUserMessage`). Flow **inativo**, **sem
  vínculo** ou canal em `ai` = **IA livre**. Fora do estado `ia`
  (humano/fechado) o flow não roda.
- **Configuração (dois lados, mesma fonte)**:
  - **Canal → Conexão → "Modo de atendimento"**: escolhe `IA (conversa livre)`
    (com o agente) ou `Chat Flow` (com o fluxo); grava por
    `PATCH/POST /api/react/channels` (`internal/web/api_react_channels.go`).
  - **Chat Flow → "Onde este fluxo rodará"**: liga/desliga canais via
    `POST /api/react/chatflows/{id}/channels`
    (`internal/web/api_react_chatflow.go`), que usa o mesmo `SetChannelChatflow`.
  - Ambos leem o mesmo `mode`/`chatflow_id` devolvido em `/api/react/channels` e
    `/api/react/chatflows`; mudar de um lado reflete no outro. O agente deixou
    de ter modo de atendimento (só a persona de IA). O vínculo com o agente
    (`channels.agent_id`) também é escolhido no canal; o editor do agente não
    define canais (a aba Canais é somente leitura e atalha para Canais).

### Inbox, mídia e notificações
- Notificações em `web/react/src/shared/notifications.ts` (som/desktop/badge no
  título), acionadas pelo polling incremental do Inbox.
- Áudio é servido como bytes com `Content-Type` tocável
  (`internal/web/saas_conversations.go`); OGG/Opus não toca no Safari.
- Chamada **não cria conversa**: `handleCall` só registra na conversa aberta, se
  existir; telefone E.164 validado e filtros de status/grupo/newsletter.

## Canais e instâncias WhatsApp (API Notreve)

- A API é configurada só pelo **super_admin** (`settings.whatsapp_base_url` + `whatsapp_admin_token`). O admin vê apenas "API Não oficial (QR code)".
- Autenticação: instância → headers `token` e `Authorization` (compatibilidade
  com as duas formas publicadas pela API); admin → header `Authorization`.
- Fluxo no painel (Canais → Conectar): provisiona `POST /admin/users` (nome+token gerado+webhook+eventos), `POST /session/connect`, `GET /session/qr` (QR PNG base64), `GET /session/status`, `POST /session/disconnect|logout`, `POST /session/limits` (anti-ban), `POST /webhook`.
- Cliente em `internal/channel/instancia.go`; handlers em `internal/web/canais_api.go`.
- Sessão preserva QR-disconnect (reconecta sem QR); logout exige novo QR.
- Mídia recebida no webhook: habilitar `POST /session/media/config` (base64/s3) quando necessário.
- Chamadas: a conexão assina `CallOffer`; quando `responder_chamadas` está ativo,
  o webhook chama primeiro `POST /call/reject` com `call_from` + `call_id` e
  depois envia a mensagem configurada em `/chat/send/text`. Ao clicar em
  Conectar numa sessão já ativa, webhook e eventos são reaplicados para não
  perder a inscrição de chamadas; o monitor também reaplica essa configuração
  para sessões ativas quando a URL pública do webhook está configurada.
- HMAC do webhook da API Notreve é opcional. O endpoint aceita eventos sem
  `x-hmac-signature`; quando o cabeçalho estiver presente, valida a assinatura
  e rejeita somente o valor inválido. Essa regra é diferente dos webhooks de
  saída da API pública, que têm assinatura própria.

## Permissões e módulos

- Perfis: `super_admin` (plataforma/multi-tenant), `admin`, `supervisor`, `atendente`.
- Permissões granulares: `usuarios.permissions` (lista final); vazio = padrão do
  perfil (`roleDefaultPerms` em `internal/web/perms.go`).
- `requirePerm("x")` nas rotas; `canPerm .User "x"` nos templates.
- Módulos em `modulos` — ligados/desligados no painel; escondem itens de menu e
  bloqueiam a página (rota) correspondente.
- **Branding**: a marca da **plataforma** é só do `super_admin`
  (`/api/react/platform/branding`); a marca do **tenant/widget** é configurada
  no canal Webchat. `/api/react/branding` segue restrito ao `super_admin`.

## Provedores de IA

- Cadastrados pelo admin (por empresa); o preset recomendado é **Notreve IA**
  (`llm.NotreveBaseURL` = `https://api.ia.notreve.com.br/v1`, modelos
  `notreve-v1-pro`/`notreve-vision`). O painel lista modelos
  (`POST /api/react/providers/models`), testa a conexão e salva o modelo. IA
  própria é OpenAI-compat (`base_url`+`model`+token+capacidade).

## Geração de mídia (imagem)

- A tool de runtime `gerar_imagem(prompt, aspect_ratio?)` é exposta à IA quando o
  agente tem provedor com chave e capacidade de imagem
  (`providers.capabilities` contém `imagem`/`image`, ou `base_url` OpenRouter),
  em conversa real (não no dry-run do wizard).
- `llm.Client.GenerateImage` chama `/chat/completions` com
  `modalities: ["image","text"]` + `image_config.aspect_ratio` (compatível com
  OpenRouter); aceita imagem em data URI ou URL (baixa e converte para base64).
- A imagem é entregue pelo caminho de mídia existente (`Server.SendGeneratedMedia`
  → `sendConversationMedia`), é persistida como mensagem do assistente (aparece
  no painel/webchat) e é best-effort: falha de envio não quebra o atendimento.
- Config: `settings.image_model` (vazio = modelo do provedor) e
  `settings.image_aspect_ratio` (default `1:1`), editáveis em Configurações.
  Provedor sem suporte devolve erro claro ("não suporta geração de imagem").

## Logs estruturados

- `internal/applog` grava no stdout e em `log_entries` de forma best-effort
  (erro/panic do writer é ignorado). `applog.SetDefault` é ligado no `main.go`.
- Endpoints `GET /api/react/logs` (filtros `level`/`module`/`q`/`from`/`to` +
  paginação padrão) e `GET /api/react/logs/stats` (total, por nível, por módulo,
  erros na última hora). Permissão granular `logs_ver`, módulo `config`.
- Retenção: job periódico (6h) remove entradas mais antigas que
  `settings.logs_retencao_dias` (default 30) por tenant.
- UI: tela **Logs** (Administração) com filtros, paginação, estatísticas e
  detalhe expansível (traceback/extra).

## Deploy

- VPS: host configurado no ambiente de produção · Docker Compose em `APP_DIR`
  (app + postgres + redis + rabbitmq). Fuso: `TZ=America/Sao_Paulo`.
- Build/teste local: `go build -buildvcs=false ./...`.
- Migrations aplicam no start do app; nova migration = novo arquivo `.sql`.
