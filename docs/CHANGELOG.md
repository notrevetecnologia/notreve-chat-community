# Changelog

## 2026-09-15 — relatórios acionáveis com exportação CSV

- Filtros reais na tela **Relatórios** (período hoje/7d/30d/custom, canal, setor
  e atendente) com paginação padrão na lista de atendimentos avaliados.
- Novas métricas: **tempo médio de espera por humano**, **tempo médio de
  atendimento**, **CSAT** (notas ≥ 4), **taxa de encerramento** e **resolução
  IA × humano**, além das já existentes (volume/dia/atendente/setor, resposta,
  transferência, NPS, abandono).
- **`GET /api/react/reports/export`**: exporta CSV (`text/csv`) com o resumo do
  período e uma linha por atendimento, com escaping via `encoding/csv` (sem
  dependências novas). Mesma guarda `relatorios_ver` + módulo `relatorios`.
- UI teal + dark mode, botão **Exportar CSV** e testes das agregações e do CSV.

## 2026-09-03 — segurança: anti-SSRF e erros não silenciados

- **Anti-SSRF no fetch de conhecimento e de mídia**: o download de URLs de
  conhecimento (`agent.FetchLinks`) e o download de mídia via mediaLink no
  webhook WhatsApp (`channel.downloadMediaURL`) agora usam um cliente HTTP que
  bloqueia endereços privados/loopback/link-local (local, .internal, .lan) e
  **não segue redirecionamentos** — os caminhos de tools/webhooks já eram
  protegidos; estes dois ficaram para trás.
- **Erros de caminho crítico não são mais engolidos**:
  - `channel.Process`: a transição para `humano_aguardando` e o setor da
    transferência agora checam/logam erro; o webhook de transferência e o
    `out.Status` só refletem o novo estado quando a gravação no banco teve
    sucesso (evita conversa aparentemente na fila humana sem estar lá).
  - `channel.Notreve.handleCall`: registrar as mensagens da chamada no
    histórico agora loga falha em vez de silenciar.
- `go build`, `go vet`, `go test ./...` e `npm run build` passaram.

## 2026-09-02 — rejeição automática de chamadas Notreve

- A conexão WhatsApp passou a manter `CallOffer` na inscrição de eventos,
  inclusive ao reaplicar a configuração de uma sessão já conectada.
- O atendimento rejeita a chamada via `POST /call/reject` (`call_from` e
  `call_id`) antes de enviar o texto configurado para chamadas.
- O monitor de canais reaplica a inscrição para sessões ativas quando a URL
  pública do webhook está configurada.

## 2026-09-02 — webhook Notreve aceita entrega sem HMAC

- Corrigido o guard do webhook WhatsApp: a assinatura HMAC da Notreve API é
  opcional. Eventos sem `x-hmac-signature` seguem para o processamento; quando
  o cabeçalho existe, a assinatura inválida continua sendo rejeitada.

## 2026-08-24 — OTP/2FA no guard de identidade + templates de conectores

- **Identificação por código de verificação (OTP/2FA)** no guard de tools: a tool
  exige o código que o cliente recebeu; o guard envia `identifier` + `code` ao
  verifier externo e autoriza com o `client_id`. O argumento reservado `_otp_code`
  nunca vai para a API externa. Prompt orienta a IA a pedir o código.
- **Templates de conectores** (migrados do roteiro) definitivamente incluídos:
  MKAUTH, WHMCS e ASAAS com parâmetros + sugestão de guard.
- Campo `DetectarBots` adicionado aos Settings para controle de detecção de bots.
- Campo `BotPatterns` configurado para padrões de texto de bot.
- `go build`, `go vet`, `go test ./...` e `npm run build` passaram.

## 2026-08-24 — white-label por empresa e auditoria de isolamento multi-tenant

- **White-label por empresa**: o cliente (empresa/tenant) configura nome, logo,
  favicon e as 5 cores (primária, secundária, destaque, fundo, carregamento) e a
  marca é aplicada em **login, shell React e webchat público**, com fallback
  seguro para a marca Notreve. Só o **admin da empresa** edita (não só
  super_admin). Segurança do branding: URLs http(s) com whitelist de extensão de
  imagem, cores `#RRGGBB`, nome sanitizado e sem HTML/JS/CSS arbitrário (nada de
  XSS). Sem tokens/segredos.
- **Auditoria de isolamento multi-tenant**: revisadas as queries/handlers/jobs de
  negócio. O `store` já está fortemente escopado por `tenant_id`; corrigida 1
  inconsistência (`ListWaitingConversations` subquery sem tenant). 4 pontos vão
  para revisão manual (nenhum vazamento crítico de dados cruzados). RLS
  permanece **não ativado** (exige staging).
- `go build`, `go vet`, `go test ./...` e `npm run build` passaram.

## 2026-08-24 — templates de conectores (MKAUTH, WHMCS, ASAAS)

- **Catálogo de conectores** com templates reais: **MKAUTH** (consultar
  cliente/fatura, abrir chamado), **WHMCS** (consultar cliente, 2ª via de fatura,
  abrir ticket) e **ASAAS** (consultar cliente, criar cobrança boleto/Pix). Cada
  template traz método, risco, escopos, limites, **schema de parâmetros** (a IA
  preenche) e a **sugestão de guard de identidade** (phone/challenge/resolve).
- Ao escolher um template, o formulário **pré-preenche** política + parâmetros +
  identificação; o card exibe o selo de identificação exigida. O admin só informa
  URL + token + os campos do verifier.
- `go build`, `go vet`, `go test ./...` e `npm run build` passaram.

## 2026-08-24 — correção do white-label (marca da plataforma vs marca da empresa)

- **Recorte corrigido**: 
  - **Painel/login** = marca **da plataforma (Notreve)**, controlada pelo
    **super_admin** (aba "Marca da plataforma" no control plane) — os clientes
    (empresas/admin) veem a marca Notreve no painel.
  - **Superfícies do cliente final** (widget webchat, mensagens ao cliente) =
    marca **da empresa** (admin configura), com fallback para a Notreve.
- Nova tabela `platform_branding` (migration 050) para a marca global da
  plataforma + endpoints `GET/PUT /api/react/platform/branding` (super_admin).
- Marca da empresa (admin) continua em `settings` por tenant e é usada no widget
  público; o painel não é mais retematizado pela empresa.
- `go build`, `go vet`, `go test ./...` e `npm run build` passaram.

## 2026-08-24 — white-label por empresa e auditoria de isolamento multi-tenant

- **White-label por empresa**: o cliente (empresa/tenant) configura nome, logo,
  favicon e as 5 cores (primária, secundária, destaque, fundo, carregamento) e a
  marca é aplicada em **login, shell React e webchat público**, com fallback
  seguro para a marca Notreve. Só o **admin da empresa** edita (não só
  super_admin). Segurança do branding: URLs http(s) com whitelist de extensão de
  imagem, cores `#RRGGBB`, nome sanitizado e sem HTML/JS/CSS arbitrário (nada de
  XSS). Sem tokens/segredos.
- **Auditoria de isolamento multi-tenant**: revisadas as queries/handlers/jobs de
  negócio. O `store` já está fortemente escopado por `tenant_id`; corrigida 1
  inconsistência (`ListWaitingConversations` subquery sem tenant). 4 pontos vão
  para revisão manual (nenhum vazamento crítico de dados cruzados). RLS
  permanece **não ativado** (exige staging).
- `go build`, `go vet`, `go test ./...` e `npm run build` passaram.

## 2026-08-23 — detecção de "outro bot" no atendimento (segurança)

- Nova configuração **Detectar outros bots** (em Configurações do atendimento,
  desligada por padrão). Quando ligada, a plataforma detecta um provável outro
  bot por **padrão de texto** (lista editável `bot_patterns`) ou **rajada**
  (6+ mensagens em 60s) e **transfere a conversa para atendimento humano** com a
  nota "⚠️ Possível bot detectado" — em vez de a IA responder a outro bot. Não
  encerra (evita loop de "encerrar → bot responde de novo").
- Opt-in: com a configuração desligada, o comportamento do atendimento livre
  permanece idêntico. Migration `048_bot_detection.sql`.
- **Estado `bot_blocked`** (migration 049): ao detectar, a conversa é **marcada
  como bloqueada** e novas mensagens são **ignoradas** (sem gastar a fila humana
  nem a IA) até um atendente **desbloquear** no Inbox (badge "🤖 Bot suspeito" +
  botão "Desbloquear bot"). Padrões padrão conservadores aplicados quando a lista
  está vazia.
- `go build`, `go vet`, `go test ./...` e `npm run build` passaram.

## 2026-08-23 — guard de identidade de tools (genérico e customizável)

- Uma tool agora pode **exigir identificação do cliente** em três modos, todos
  aditivos/opt-in: `phone` (allowlist ou verifier), `challenge` (perguntas
  mascaradas: nascimento, nome da mãe, parte do documento...) e `resolve`
  (buscar o cliente no sistema externo e injetar o `client_id`). O **verifier
  externo** é configurado por tool (`identity_verify_url`+método+param+match+
  client_id_field). Vale para MKAUTH/WHMCS/ASAAS/ERP e afins.
- Os argumentos reservados `_identity`/`_challenge_answers` alimentam o guard e
  **nunca são enviados** à API externa. Dados pessoais não vão para logs.
- Migration `047_tool_identity_guard.sql`. Config de desafios e verifier no
  painel (Inteligência → Tools).
- `go build`, `go vet`, `go test ./...` e `npm run build` passaram.

## 2026-08-23 — Chat Flow em canvas e regra de identificação de tools

- **Chat Flow em canvas (drag-and-drop)** — o editor deixou de ser
  uma lista linear e virou um canvas de nós conectáveis (@xyflow/react): paleta
  de blocos (`message`/`collect`/`condition`/`intent`/`tool`/`transfer`/`end`),
  arestas para `next`/`then`/`else`/`branches`/`default`, configuração por nó
  (incluindo mensagens ricas) e persistência compatível com o backend. Menu
  lateral e vínculo por agente mantidos.
- **Regra de identificação por telefone em tools (motor de regras)** — uma tool
  pode exigir a identificação do cliente por telefone e só executar se o número
  estiver numa lista de permitidos (ex.: consultar boleto só do próprio dono).
  É aditiva/opt-in: tools sem `requires_identity` continuam idênticas. Inclui
  migration 046, campos na política, gate no executor, injeção do telefone da
  conversa no contexto e config no painel. O telefone nunca vai para logs.
- `go build`, `go vet`, `go test ./...` e `npm run build` passaram.

## 2026-08-23 — correção: respostas de botões/listas do WhatsApp

- Corrigido o reconhecimento de respostas **de botões, listas e fluxos nativos**
  do WhatsApp (`buttonsResponseMessage`, `listResponseMessage`,
  `templateButtonReplyMessage`, `interactiveResponseMessage`). Antes o app só
  lia texto puro, então a nota da pesquisa de satisfação (enviada por botões)
  não era identificada. Agora `extractText` extrai o texto/opção selecionada e
  o processador reconhece a nota/opção normalmente.

## 2026-08-23 — remoção do módulo de pagamentos

- Removido o **módulo de pagamentos** (cobrança/Pix por provedor) do Agente IA:
  código (`internal/payments`, `store/pagamentos.go`, `api_react_pagamentos.go`),
  permissão `pagamentos_gerenciar`, rotas do painel, item de menu, tela React,
  migration `041_pagamentos_conversacionais.sql` e limite de plano relacionado.
- Motivo: o produto é atendimento (billing é externo). Pagamentos
  **conversacionais no WhatsApp** (tipo banco digital — abrir conta, saldo,
  extrato) ficam para um **projeto separado**.
- `go build`, `go vet`, `go test ./...` e `npm run build` passaram.

## 2026-08-23 — mensagens ricas (botões, lista, enquete e mais)

- **Mensagens ricas** em todos os fluxos do bot: a IA, o menu de 1º contato, o
  chat flow e a pesquisa de satisfação agora podem enviar **botões**, **lista**,
  **enquete**, **contato**, **localização** e **figurinha** no WhatsApp.
- A IA emite um marcador (`[OPCOES:...]`, `[LISTA:...]`, `[ENQUETE:...]`,
  `[CONTATO:...]`, `[LOCALIZACAO:...]`, `[STICKER:...]`) quando é pertinente
  oferecer opções. O menu de 1º contato do agente vira botões automaticamente.
- **Fallback garantido**: se o canal não suportar o tipo (webchat/telegram) ou a
  API falhar, o envio cai para **texto com opções numeradas** — o atendimento
  nunca quebra. A pesquisa de satisfação usa botões (1..5); a nota chega como
  texto e é lida normalmente.
- Novos endpoints na camada de canal: `/chat/send/buttons|list|poll|contact|
  location|sticker`, com fallback.
- `go build`, `go vet`, `go test ./...` e `npm run build` passaram. Envios reais
  (texto, botões, lista, enquete, contato e localização) validados em produção.

## 2026-08-23 — monitoramento de canais e log de entregas de webhooks

- **Monitoramento de canais**: um processo de fundo verifica periodicamente a
  saúde de cada canal WhatsApp (`/session/status`) e Telegram (`getWebhookInfo`),
  grava `health_status`/`last_checked_at`/`last_error` em `channels` (migration
  045) e exibe o estado no painel de canais. Ao detectar uma transição real de
  estado, enfileira o evento `channel.status_changed` na outbox (alerta de
  desconexão). Webchat não é monitorado (passivo).
- **Log de entregas de webhooks**: novo endpoint
  `GET /api/react/webhooks/deliveries` e tela nas Integrações (seção Webhooks)
  para listar/filtrar as entregas da outbox (evento, status, tentativas, erro,
  próxima tentativa/entrega) — sem expor payload nem segredos. O worker de
  entrega (já ativo em produção) passa a ser visível no painel.
- **Publicação automática dos eventos internos**: o fluxo do Inbox agora
  enfileira na outbox `message.received`, `message.sent` (IA e humano),
  `conversation.created`, `conversation.updated`, `conversation.closed`,
  `agent.transferred` e `satisfaction.completed`. A publicação é best-effort
  (um erro é só registrado em log e nunca interrompe o atendimento) e usa chaves
  de idempotência para não duplicar entregas em retries.
- `go build`, `go vet`, `go test ./...` e `npm run build` passaram.

## 2026-08-22 — fases 1–5 integradas

- Login público, logout e setup passaram a entrar pelo React; as telas Go de
  login/setup/agente foram aposentadas como páginas e ficaram somente APIs de
  compatibilidade.
- A resolução de tenant passou a ocorrer pelo host do painel no login, sessão
  e APIs React. Hosts não cadastrados não escolhem silenciosamente o tenant 1.
- O menu ganhou a Base de conhecimento com texto, PDF, áudio e site; modelos,
  webhook e números permitidos também ficaram editáveis nas configurações sem
  expor credenciais.
- Inbox, canais e contatos passaram a usar contratos React para mídia, ações,
  status, conexão, sincronização, verificação e abertura de conversa.
- Tools receberam catálogo, risco, confirmação, escopos, allowlist de host,
  limites, timeout, validação anti-SSRF e auditoria sem valores sensíveis.
- `npm run build`, `go test ./...`, `go vet ./...` e `go build ./...` passaram.

## 2026-08-22 — migração administrativa React

- Adicionada gestão de usuários no React com perfis, setores, senha, estado e
  permissões granulares respeitando a hierarquia do tenant.
- Adicionadas configurações operacionais no React e redirecionamento das telas
  administrativas legadas para a SPA nativa.
- Campanhas ganharam grupos de canais CRUD e anexo de mídia opcional de até
  16 MB.

## 2026-08-21 — revisão completa

- A IA agora preserva até 48 mensagens reais da conversa (ignorando eventos
  técnicos), relaciona respostas curtas ao turno anterior e recebe instruções
  explícitas para não repetir perguntas, saudações ou respostas genéricas.
- O processamento de mensagens foi serializado por atendimento para evitar que
  mensagens simultâneas gerem respostas com versões diferentes do histórico.
- Corrigida a sincronização do painel quando um cliente encerra e avalia pelo WhatsApp: a conversa selecionada agora recarrega no estado encerrado e a lista reage a mudanças em qualquer ticket, não apenas no primeiro.
- Relatórios agora exibem média geral, IA e atendimento humano, ranking de atendentes, desempenho por setor, distribuição separada das notas e nota individual por protocolo.
- Renovada a versão dos assets do painel para invalidar o cache anterior e entregar imediatamente o novo JavaScript e o novo layout aos navegadores.
- Corrigida a largura flexível do conteúdo no celular, eliminando a rolagem horizontal da página e reorganizando o cabeçalho e os indicadores do relatório em telas estreitas.
- Adicionado encerramento real pelo cliente no Webchat, com confirmação,
  avaliação dinâmica, repetição para nota inválida e início de novo protocolo.
- Pedidos explícitos como “quero encerrar o atendimento” agora encerram a
  conversa em qualquer canal processado e seguem o mesmo fluxo de avaliação.
- Corrigido o modal de edição de canais para carregar os dados do card.
- Removidos URL e token da criação/edição de WhatsApp; o provisionamento continua
  automático pelo botão Conectar e QR Code.
- Corrigida a renderização do widget webchat quando o CSS contém percentuais (`%`), evitando erro de `fmt.Fprintf`/`go vet`.
- Fechado acesso direto a mensagens e mídias por ID: a API agora valida conversa, papel, setor e atribuição antes de exibir ou alterar dados.
- Impedido que uma mensagem de resposta use como `reply_to` uma mensagem de outra conversa.
- Aceite de atendimento passou a usar operação atômica, evitando que dois atendentes assumam o mesmo ticket simultaneamente.
- Adicionados limites rígidos para payloads de webhook, uploads de mídia, respostas de integrações, API WhatsApp e provedores LLM.
- Webhooks com `UserID` desconhecido deixaram de cair no canal ativo por fallback, reduzindo a possibilidade de forjar eventos em instâncias erradas.
- Renovado o TTL da sessão também no Redis durante atividade do usuário.
- Cookies de sessão receberam `Secure` quando HTTPS está ativo e `SameSite=Lax`; POSTs autenticados validam `Origin`/`Referer` quando enviados pelo navegador.
- Chaves de IA, tokens de canal, token administrativo do WhatsApp e headers sensíveis de integrações deixaram de ser enviados ao JavaScript/HTML do painel. Edições vazias preservam o segredo já salvo.
- O marcador interno `[NOME:...]` é removido antes de salvar a resposta no histórico.
- Falhas de escrita de destinatários de campanha agora abortam a transação em vez de serem ignoradas.
- Agendamentos de campanha passam a interpretar o horário explicitamente em `America/Sao_Paulo`.
- Adicionados testes de escopo de conversas e proteção de origem.
- Aplicado refinamento visual responsivo com foco acessível, estados de interação, cartões, tabela, modal e suporte a redução de movimento, preservando IDs e hooks existentes.
- Corrigida a montagem de URLs de mídia no painel, validando MIME e escapando atributos antes de inserir conteúdo no DOM.
- Impedido o nascimento de tickets duplicados para o mesmo cliente e tipo de canal:
  a busca agora normaliza telefones/JIDs legados, reutiliza o atendimento ativo
  mesmo após troca de instância e usa lock transacional para corridas de webhook.
  Tickets encerrados iniciam um novo protocolo; avaliações ainda pendentes
  continuam no protocolo original.
# Próxima rodada

- Painel React agora usa `/` como endereço canônico; `/react` continua compatível.
- Entrada de autenticação canônica em `/auth`, com login refinado e recuperação de acesso orientada ao administrador.
- API pública v1 para abrir/reusar conversa, enviar texto, consultar e encerrar atendimento.
- Chaves de API por tenant com escopos, revogação, rate limit e idempotência.
- Fundação de webhooks com deliveries, HMAC, retry/backoff e proteção anti-SSRF; publicação automática de eventos permanece na fila de ativação.
- Integrações ganhou gerenciamento visual das chaves públicas.
