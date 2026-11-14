# Permissões — matriz papel × área

Fonte da verdade da divisão de acesso entre papéis. Toda tela, item de menu ou
função nova **precisa declarar o papel** que pode usá-la e aparecer na seção
correspondente desta matriz. Se algo não se encaixa aqui, o desenho está errado.

- Código de referência: [`internal/web/perms.go`](../internal/web/perms.go),
  [`internal/web/server.go`](../internal/web/server.go) (registro de rotas),
  [`web/react/src/app/navigation.ts`](../web/react/src/app/navigation.ts) e
  [`web/react/src/app/app-shell.tsx`](../web/react/src/app/app-shell.tsx).
- Papéis: `super_admin` · `admin` · `supervisor` · `atendente` (`usuarios.role`).
- Permissões granulares: `usuarios.permissions` (CSV). Vazio = padrão do papel
  (`roleDefaultPerms` em `perms.go`).

## Regra de ouro

| Papel | Quem é | Escopo | NUNCA faz |
|-------|--------|--------|-----------|
| `super_admin` | Dono do sistema / plataforma | Servidores, tenants, planos, white-label global, marca do sistema | Operar dados de cliente (conversas, canais, agentes, LGPD) |
| `admin` | Dono da empresa contratante | Tudo dentro da empresa dele: atendimento, canais, agentes, LGPD, equipe | Administrar outras empresas ou infraestrutura compartilhada |
| `supervisor` | Funcionário | Setores, campanhas, relatórios, agentes (sem provedores) | Plataforma, equipe/permissões, config, canais, integrações |
| `atendente` | Funcionário | Conversas, contatos e respostas rápidas | Qualquer configuração |

**O grupo de menu `Plataforma` é exclusivo do `super_admin`.** Nenhum outro
papel pode ver itens nele. O `super_admin`, por sua vez, só vê o grupo
`Plataforma` (control plane) — o resto do painel é da empresa.

## Permissões granulares (catálogo)

| Módulo | Permissão | O que libera |
|--------|-----------|--------------|
| Atendimento | `conversas_ver` | Ver conversas e a visão geral |
| Atendimento | `conversas_responder` | Responder, assumir, digitar, reagir |
| Atendimento | `conversas_transferir` | Transferir de setor/atendente |
| Atendimento | `conversas_encerrar` | Encerrar/reabrir/devolver à IA |
| Atendimento | `conversas_ver_ia` | Ver a fila da IA (conversas em `ia`) |
| Atendimento | `enviar_assinatura` | Enviar mensagem com assinatura (opt-in) |
| Contatos | `contatos_gerenciar` | Gerenciar contatos |
| Agentes de IA | `agentes_gerenciar` | Agentes, base de conhecimento e skills |
| Agentes de IA | `ias_gerenciar` | Provedores de IA |
| Agentes de IA | `aprendizado_ver` | Aprendizado e falhas do agente |
| Agentes de IA | `chatflows_gerenciar` | Chat flows |
| Setores e Equipe | `setores_gerenciar` | Gerenciar setores |
| Setores e Equipe | `equipe_gerenciar` | Gerenciar equipe e permissões (usuários) |
| Canais | `canais_gerenciar` | Gerenciar canais |
| Integrações | `integracoes_gerenciar` | Integrações, API, webhooks e MCP |
| Mensagens | `respostas_gerenciar` | Respostas rápidas |
| Mensagens | `mensagens_gerenciar` | Mensagens padrão do sistema |
| Campanhas | `campanhas_ver` | Ver campanhas e grupos |
| Campanhas | `campanhas_gerenciar` | Criar/operar campanhas |
| Campanhas | `grupos_canais_gerenciar` | Grupos de canais |
| Agendamentos | `agendamentos_gerenciar` | Criar/editar/disparar/cancelar agendamentos |
| Relatórios | `relatorios_ver` | Ver relatórios |
| Configurações | `config_gerenciar` | Configurações do atendimento e LGPD |
| Configurações | `logs_ver` | Ver o painel de logs estruturados do sistema |
| Configurações | `plataforma_gerenciar` | Plataforma multi-tenant — **apenas `super_admin`** |

### Padrão por papel

- `super_admin`: **todas** as permissões (inclusive `plataforma_gerenciar`).
- `admin`: todas **exceto** `plataforma_gerenciar`.
- `supervisor`: `conversas_ver`, `conversas_responder`,
  `conversas_transferir`, `conversas_encerrar`, `conversas_ver_ia`,
  `enviar_assinatura`, `contatos_gerenciar`, `respostas_gerenciar`,
  `mensagens_gerenciar`, `agentes_gerenciar`, `aprendizado_ver`,
  `setores_gerenciar`, `campanhas_ver`, `campanhas_gerenciar`,
  `grupos_canais_gerenciar`, `relatorios_ver`.
- `atendente`: `conversas_ver`, `conversas_responder`,
  `conversas_transferir`, `conversas_encerrar`, `conversas_ver_ia`,
  `contatos_gerenciar`, `respostas_gerenciar`, `enviar_assinatura`.

> **Regra da fila `ia`**: ver conversas em estado `ia` (a fila da IA) é
> controlado pela permissão granular `conversas_ver_ia`, não pelo papel. Sem
> ela, o atendente não vê conversas em `ia` (nem no filtro "IA" do Inbox, que
> fica oculto). Com ela, vê a fila `ia` do seu escopo (atribuídas a ele e não
> atribuídas do setor). Admin/super_admin continuam vendo tudo.

## Matriz do menu (item por item)

Grupo · item · módulo · permissão → quem vê.

| Grupo | Item (`navigation.ts`) | Módulo | Permissão | admin | supervisor | atendente | super_admin |
|-------|------------------------|--------|-----------|:-----:|:----------:|:---------:|:-----------:|
| Atendimento | Visão geral | conversas | `conversas_ver` | Sim | Sim | Sim | — |
| Atendimento | Caixa de entrada | conversas | `conversas_ver` | Sim | Sim | Sim | — |
| Atendimento | Relatórios | relatorios | `relatorios_ver` | Sim | Sim | — | — |
| Atendimento | Assistente de configuração | config | `config_gerenciar` | Sim | — | — | — |
| Relacionamento | Contatos | contatos | `contatos_gerenciar` | Sim | Sim | Sim | — |
| Relacionamento | Campanhas e segmentos | campanhas | `campanhas_ver` | Sim | Sim | — | — |
| Inteligência | Agentes | ia | `agentes_gerenciar` | Sim | Sim | — | — |
| Inteligência | Base de conhecimento | ia | `agentes_gerenciar` | Sim | Sim | — | — |
| Inteligência | Provedores de IA | ia | `ias_gerenciar` | Sim | — | — | — |
| Inteligência | Aprendizado | ia | `aprendizado_ver` | Sim | Sim | — | — |
| Inteligência | Skills | ia | `agentes_gerenciar` | Sim | Sim | — | — |
| Inteligência | Chat Flow | chatflows | `chatflows_gerenciar` | Sim | — | — | — |
| Canais | Canais | canais | `canais_gerenciar` | Sim | — | — | — |
| Automação | Integrações e API | tools **ou** api | `integracoes_gerenciar` | Sim | — | — | — |
| Operação | Equipe e setores | equipe | `setores_gerenciar` **ou** `equipe_gerenciar` | Sim | Sim | — | — |
| Operação | Agendamentos | agendamentos | `agendamentos_gerenciar` | Sim | — | — | — |
| Operação | Biblioteca de mensagens | mensagens | `respostas_gerenciar` **ou** `mensagens_gerenciar` | Sim | Sim | Sim | — |
| Administração | Configurações do atendimento | config | `config_gerenciar` | Sim | — | — | — |
| Administração | Logs | config | `logs_ver` | Sim | — | — | — |
| Administração | LGPD e privacidade | lgpd | `config_gerenciar` | Sim | — | — | — |
| Plataforma | Plataforma e white-label | config | `plataforma_gerenciar` | — | — | — | Sim |
| Plataforma | Planos e limites | config | `plataforma_gerenciar` | — | — | — | Sim |

> O `super_admin` só enxerga o grupo **Plataforma** (`app-shell.tsx` filtra por
> `item.group === 'plataforma'`). O `admin` enxerga o grupo **Administração**
> (configuração da própria empresa) — antes esses itens apareciam por engano em
> "Plataforma".

## Matriz de rotas da API React (função por função)

> **`super_admin` é bloqueado no middleware** (`superAdminReactPathAllowed`) de
> toda API de tenant: só passa `/api/react/session`, `/api/react/bootstrap`,
> `/api/react/platform`, `/api/react/platform/*` e `/api/react/branding`. Por
> isso a coluna dele é `—` nas rotas da empresa, mesmo tendo todas as
> permissões.

> **Modo de atendimento**: decidido por **canal** (`channels.mode` +
> `channels.chatflow_id`), controlado por `canais_gerenciar` no editor do canal.
> O agente não define mais o modo (é só a persona de IA). O vínculo fluxo↔canal
> também é ajustável na tela **Chat Flow** (`chatflows_gerenciar`), gravando as
> mesmas colunas via `POST /api/react/chatflows/{id}/channels`; a lista de flows
> disponíveis vem de `/api/react/chatflows`.

> **Dashboard (`GET /api/react/dashboard`)**: exige `conversas_ver` e módulo
> `conversas`. Dentro do payload, a seção `satisfaction`/`sectors` só vem com
> `relatorios_ver` e a seção `failures` só com `aprendizado_ver`; o operador sem
> essas permissões recebe `permissions.reports`/`permissions.failures` = `false` e
> a UI esconde os blocos. As métricas de conversa respeitam o mesmo escopo por
> papel da listagem (atendente/supervisor), e a fila da IA segue `conversas_ver_ia`.

| Rota | Permissão efetiva | Módulo | super_admin | admin | supervisor | atendente |
|------|-------------------|--------|:-----------:|:-----:|:----------:|:---------:|
| `GET /api/react/session` · `/bootstrap` | autenticado | — | Sim | Sim | Sim | Sim |
| `GET /api/react/overview` | `conversas_ver` | conversas | — | Sim | Sim | Sim |
| `GET /api/react/dashboard` | `conversas_ver` | conversas | — | Sim | Sim | Sim |
| `GET/PUT /api/react/branding` | `super_admin` (legado) | — | Sim | — | — | — |
| `GET/PUT /api/react/platform/branding` | `super_admin` | — | Sim | — | — | — |
| `GET/POST/PATCH/DELETE /api/react/channels` | `canais_gerenciar` | canais | — | Sim | — | — |
| `GET/POST/PATCH/DELETE /api/react/contacts` | `contatos_gerenciar` | contatos | — | Sim | Sim | Sim |
| `… /api/react/agents` (+ toggle/refetch/test/channels) | `agentes_gerenciar` | ia | — | Sim | Sim | — |
| `… /api/react/chatflows` | `chatflows_gerenciar` | chatflows | — | Sim | — | — |
| `… /api/react/providers` (+ test/models) | `ias_gerenciar` | ia | — | Sim | — | — |
| `… /api/react/integrations` | `integracoes_gerenciar` | tools | — | Sim | — | — |
| `… /api/react/api-keys` · `/api-docs` | `integracoes_gerenciar` | api | — | Sim | — | — |
| `… /api/react/skills` | `agentes_gerenciar` | ia | — | Sim | Sim | — |
| `… /api/react/mcp` | `integracoes_gerenciar` | tools | — | Sim | — | — |
| `… /api/react/webhooks` | `integracoes_gerenciar` | api | — | Sim | — | — |
| `GET /api/react/learning` · `agent-failures` | `aprendizado_ver` | ia | — | Sim | Sim | — |
| `GET/POST/DELETE /api/react/knowledge` | `agentes_gerenciar` | ia | — | Sim | Sim | — |
| `… /api/react/quick-replies` | `respostas_gerenciar` | mensagens | — | Sim | Sim | Sim |
| `… /api/react/message-templates` | `mensagens_gerenciar` | mensagens | — | Sim | Sim | — |
| `… /api/react/platform/*` (tenants, planos, servidores, admins, switch) | `super_admin` | — | Sim | — | — | — |
| `GET /api/react/team` · `/sectors` | `setores_gerenciar` **ou** `equipe_gerenciar` | equipe | — | Sim | Sim | — |
| `POST/PATCH/DELETE /api/react/sectors` (+ toggle) | `setores_gerenciar` | equipe | — | Sim | Sim | — |
| `… /api/react/users` | `equipe_gerenciar` | equipe | — | Sim | — | — |
| `GET/PATCH /api/react/settings` | `config_gerenciar` | config | — | Sim | — | — |
| `GET /api/react/logs` · `/logs/stats` | `logs_ver` | config | — | Sim | — | — |
| `… /api/react/lgpd/*` | `config_gerenciar` | lgpd | — | Sim | — | — |
| `… /api/react/campaigns` · `/segments` | `campanhas_ver` / `campanhas_gerenciar` | campanhas | — | Sim | Sim | — |
| `… /api/react/campaign-groups` | `grupos_canais_gerenciar` | campanhas | — | Sim | Sim | — |
| `… /api/react/schedules` (+ `/{id}/trigger`) | `agendamentos_gerenciar` | agendamentos | — | Sim | — | — |
| `GET /api/react/reports` · `/reports/export` | `relatorios_ver` | relatorios | — | Sim | Sim | — |
| `GET /api/react/conversations` · `/{id}` | `conversas_ver` | conversas | — | Sim | Sim | Sim |
| `POST /api/react/conversations/{id}/messages` | `conversas_responder` | conversas | — | Sim | Sim | Sim |
| `POST /api/react/conversations/{id}/actions` | por ação (`responder`/`transferir`/`encerrar`) | conversas | — | Sim | Sim | Sim |
| `POST /api/react/conversations/{id}/bot-block` | `conversas_responder` | conversas | — | Sim | Sim | Sim |

## Funções dentro das páginas

| Página | Função | Permissão |
|--------|--------|-----------|
| Inbox | Responder, assumir, digitar, reagir | `conversas_responder` |
| Inbox | Ver a fila da IA (filtro "IA" e conversas em `ia`) | `conversas_ver_ia` |
| Inbox | Transferir setor/atendente | `conversas_transferir` |
| Inbox | Encerrar/reabrir/devolver à IA | `conversas_encerrar` |
| Settings | Configurações operacionais do atendimento | `config_gerenciar` |
| Settings | `default_model`/`stt_model`/`numeros_permitidos` | `super_admin` (campos técnicos) |
| Logs | Ver/filtrar/expandir logs estruturados e estatísticas | `logs_ver` |
| LGPD | Consentimentos, política, auditoria, documento do contato | `config_gerenciar` |
| Plataforma | Empresas, servidores WhatsApp, marca global, admins, assumir contexto | `super_admin` |
| Planos | CRUD de planos e limites | `super_admin` |
| Equipe | CRUD de setores | `setores_gerenciar` (supervisor só nos setores dele) |
| Equipe | CRUD de usuários e permissões | `equipe_gerenciar` (supervisor só abaixo dele e nos setores dele) |
| Agentes | Editar agente, testar (dry-run), canais, skills, conhecimento | `agentes_gerenciar` |
| Provedores | Cadastrar/testar provedor de IA | `ias_gerenciar` |
| Canais | Conectar/editar canais (WhatsApp/Telegram/Webchat) | `canais_gerenciar` |
| Campanhas | Ver | `campanhas_ver` |
| Campanhas | Criar/iniciar/pausar/agendar | `campanhas_gerenciar` |
| Campanhas | Grupos de canais | `grupos_canais_gerenciar` |
| Agendamentos | Ver/criar/editar/disparar/cancelar agendamentos | `agendamentos_gerenciar` |
| Integrações | Tools, API keys, webhooks, MCP | `integracoes_gerenciar` |
| Relatórios | Ver relatórios | `relatorios_ver` |
| Relatórios | Exportar CSV | `relatorios_ver` |

## Como decidir o papel de uma tela/função nova

1. **É da plataforma inteira?** (servidores, tenants, planos, marca do sistema,
   white-label global) → `super_admin`, grupo **Plataforma**, guarda
   `requireSuperAdmin`. Não pode operar dado de cliente.
2. **É configuração da empresa?** (atendimento, canais, agentes, LGPD, equipe,
   integrações, provedores) → `admin`, permissão `*_gerenciar` do módulo.
3. **É operação de time?** (setores, campanhas, relatórios, aprendizado) →
   `supervisor`, com a permissão granular específica.
4. **É atendimento?** (conversas, contatos, respostas rápidas) → `atendente`.
5. **Declare no código**:
   - Adicione a permissão em `permCatalog` (`perms.go`) se for nova.
   - Atualize `roleDefaultPerms` para os papéis que devem recebê-la.
   - Registre a rota com `requireReactPerm(perm, requireReactModule(modulo, …))`
     (ou `requireSuperAdmin` no control plane). Não confie só no handler.
   - Adicione o item em `navigation.ts` com `module` + `requiredPermission`
     (ou `requiredAnyPermission`) e o grupo certo.
   - Atualize **esta matriz** na mesma mudança.
6. **Regra de grupo**: itens de `super_admin` **só** no grupo `plataforma`.
   Itens de configuração da empresa ficam em `administracao`/`operacao`. Nunca
   misture os dois.

## Guardas no backend

- `requireSuperAdmin` — control plane. Exige `role == "super_admin"` (a
  permissão `plataforma_gerenciar` sozinha não basta) e aplica acesso global
  durante a operação.
- `requireReactPerm(perm, next)` — permissão granular via `userHasPerm`
  (`super_admin` tem bypass).
- `requireReactModule(modulo, next)` — módulo contratado habilitado.
- Handlers com guarda interna: `requireReactUserAccess`, `requireReactTeamAccess`,
  `requireReactSettingsAccess`, `requireReactLGPD`, `requireReactKnowledgeAccess`,
  `canEditReactBranding`. Mantidas por compatibilidade; as rotas também recebem
  guarda no registro.

## Hierarquia e escopo (Equipe e setores)

- **Ninguém se autopromove**: alterar o próprio `role` é bloqueado
  (`validateReactRoleAssignment`). Editar o próprio nome/senha continua permitido.
- **Nível acima do seu**: ninguém cria ou edita usuário com papel superior ao seu
  (`roleLevel`). `super_admin` só é atribuído por `super_admin`.
- **Supervisor**: administra apenas usuários de nível **inferior** (atendente) e
  que compartilham setor com ele (`reactUserVisible`). Não administra outro
  supervisor nem admin.
- **Admin**: administra todos da empresa, exceto `super_admin`.
- **Setores**: `supervisor` só edita/ativa/exclui setores dos quais participa;
  `admin`/`super_admin` administram qualquer setor. A listagem de setores da
  página também é escopada para o supervisor. Ao criar um setor, o supervisor é
  vinculado a ele para poder gerenciá-lo.
- **Concessão de permissão**: só é possível conceder permissões que o próprio
  ator possui (exceto `super_admin`); `plataforma_gerenciar` é exclusiva de
  `super_admin`.
- **Edição de setores de um usuário**: o supervisor pode manter vínculos já
  existentes fora do escopo dele, mas não pode adicionar novos
  (`normalizeReactUserSectors`).

## Pendências conhecidas (revisar, não é regressão)

- `/api/react/branding` (legado) é restrito a `super_admin` no handler; a marca
  do **tenant** é configurada no canal Webchat.
- `default_model`/`stt_model`/`numeros_permitidos` em Settings são campos
  técnicos restritos a `super_admin` e hoje não são renderizados na UI do
  cliente.
- O item "Assistente de configuração" (`wizard`, permissão `config_gerenciar`,
  só `admin`) ainda vive no grupo **Atendimento**. É config da empresa e é
  candidato a migrar para **Administração** numa próxima passada; não foi
  movido aqui para manter a mudança mínima.
