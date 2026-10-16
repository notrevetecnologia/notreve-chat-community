# Hierarquia de papéis e responsabilidades

Mapa de **quem faz o quê** no Notreve Chat Community: papéis, níveis,
permissões, quem configura cada área, quem gerencia quem e como isso é imposto
no código. Fonte da verdade deste documento: [`docs/PERMISSOES.md`](PERMISSOES.md)
e o código citado ao fim.

## 1. Papéis e níveis

| Papel | Nível | É quem |
|---|---|---|
| `super_admin` | 4 | Dono da plataforma (provedor). Control plane. |
| `admin` | 3 | Dono da empresa cliente. |
| `supervisor` | 2 | Funcionário do cliente — operação de time. |
| `atendente` | 1 | Funcionário do cliente — atendimento. |

Nível em `internal/web/perms.go:137` (`roleLevel`). Um papel só pode agir sobre
níveis **menores** que o seu.

## 2. Permissões por papel (padrão)

Definidas em `internal/web/perms.go:41` (`roleDefaultPerms`); podem ser
ajustadas por usuário (lista salva em `usuarios.permissions`). O `super_admin`
tem sempre todas (`userHasPerm` faz curto-circuito, `perms.go:176`).

| Área (permissão) | super_admin | admin | supervisor | atendente |
|---|:--:|:--:|:--:|:--:|
| Conversas (`conversas_*`, `conversas_ver_ia`) | ✔ | ✔ | ✔ | ✔ |
| Contatos (`contatos_gerenciar`) | ✔ | ✔ | ✔ | ✔ |
| Respostas rápidas (`respostas_gerenciar`) | ✔ | ✔ | ✔ | ✔ |
| Assinatura (`enviar_assinatura`) | ✔ | ✔ | ✔ | ✔ |
| Agentes de IA (`agentes_gerenciar`) | ✔ | ✔ | ✔ | ✘ |
| Aprendizado (`aprendizado_ver`) | ✔ | ✔ | ✔ | ✘ |
| Setores (`setores_gerenciar`) | ✔ | ✔ | ✔ | ✘ |
| Campanhas (`campanhas_*`, `grupos_canais_gerenciar`) | ✔ | ✔ | ✔ | ✘ |
| Relatórios (`relatorios_ver`) | ✔ | ✔ | ✔ | ✘ |
| Mensagens padrão (`mensagens_gerenciar`) | ✔ | ✔ | ✔ | ✘ |
| **Provedores de IA** (`ias_gerenciar`) | ✔ | ✔ | ✘ | ✘ |
| Chaves de API/webhooks (`integracoes_gerenciar`) | ✔ | ✔ | ✘ | ✘ |
| Chat flow (`chatflows_gerenciar`) | ✔ | ✔ | ✘ | ✘ |
| Canais (`canais_gerenciar`) | ✔ | ✔ | ✘ | ✘ |
| Equipe (`equipe_gerenciar`) | ✔ | ✔ | ✘ | ✘ |
| Configurações/Logs/LGPD (`config_gerenciar`, `logs_ver`) | ✔ | ✔ | ✘ | ✘ |
| Agendamentos (`agendamentos_gerenciar`) | ✔ | ✔ | ✘ | ✘ |
| **Plataforma** (`plataforma_gerenciar`) | ✔ | ✘ | ✘ | ✘ |

Resumo: **admin = tudo da empresa, menos Plataforma**; **supervisor = operação**
(setores, agentes, campanhas, relatórios) **sem** provedores/canais/config;
**atendente = só atendimento**.

## 3. Quem configura o quê

- **super_admin (Plataforma)** — grupo de menu só dele (`navigation.ts:31`):
  Empresas, Planos e limites, Infraestrutura WhatsApp, Marca da plataforma.
  Rotas `/api/react/platform/*` exigem `requireSuperAdmin` (`server.go:743`),
  que também aplica **bypass de RLS** só durante a operação global
  (`perms.go:206`). Pode **trocar de contexto** para uma empresa (com auditoria).
- **admin (Empresa)** — Canais, Provedores de IA, Agentes, Equipe e permissões,
  Integrações/API, Campanhas, LGPD, Configurações do atendimento. Nunca outra
  empresa.
- **supervisor (Operação)** — Setores, Agentes, Campanhas, Relatórios,
  Aprendizado.
- **atendente (Atendimento)** — Caixa de entrada, Contatos, respostas rápidas.

## 4. Quem manda em quem (gestão de equipe)

Regras em `internal/web/api_react_users.go`:
- **Papéis atribuíveis** (`reactUserRolesFor`, `:115`):
  - `super_admin` atribui qualquer papel.
  - `admin` atribui `admin`, `supervisor`, `atendente` (nunca `super_admin`).
  - `supervisor` atribui apenas `atendente`.
- **Editar/remover**: ninguém mexe em quem está no **mesmo nível ou acima**
  (`:349`, `:352`, `:380`).
- **Permissões concedíveis** (`:137`): o ator só concede o que ele próprio tem;
  `plataforma_gerenciar` só aparece para o `super_admin` (`:142`).

## 5. Como é imposto no código

Cada rota `/api/react/*` encadeia guardas:
```
authJSON → requireReactPerm(permissão) → requireReactModule(módulo)
```
(ex.: `server.go:668`). Guardas: `requireReactPerm` (`api_react.go:367`),
`requireReactModule` (`api_react.go:352`), `requireSuperAdmin`
(`perms.go:206`), `requireRole` (`server.go:921`). O menu reflete as permissões
(`navigation.ts`), mas a **autoridade é sempre no backend**.

**Isolamento:** `tenant_id` em todas as tabelas de negócio + políticas de RLS
registradas; o store filtra por `TenantID(ctx)`. O `super_admin` só cruza
tenants pela rota de plataforma/`context switch`, com auditoria.

## 6. Personalização (branding)

Responsabilidades separadas — **não** são a mesma coisa:

| Superfície | Quem configura | Onde | Endpoint |
|---|---|---|---|
| **Painel / login** (marca global) | `super_admin` | Marca da plataforma | `GET/PUT /api/react/platform/branding` |
| **Widget Webchat** (layout daquele canal) | quem tem `canais_gerenciar` (admin) | Canal → aba **Widget** | `widget_config` do canal |

- O painel usa **a marca da plataforma** (`platformBranding`,
  `api_react.go:183`) — o admin **não** altera a carcaça do painel. O endpoint
  legado `/api/react/branding` (marca por empresa) também é restrito ao
  `super_admin` (`canEditReactBranding`, `api_react.go:531`).
- O **Webchat** usa o `WidgetConfig` **do canal** (logo, título, cores, textos),
  em `internal/channel/webchat.go`. O fallback para a marca global só ocorre
  quando o campo do canal está vazio/no padrão (`overlayWidgetBrand`,
  `webchat.go:173`).

## 7. Primeira execução (instalação nova)

Nada vem pré-configurado (nenhum provedor, canal, agente ou empresa demo).

1. O bootstrap cria **um único usuário `super_admin`** (usuário `super_admin`,
   nome "Administrador da plataforma") — `main.go` (`ensureAdmin`).
2. **Wizard de 2 passos (super_admin):** cadastra a **API WhatsApp** (base_url +
   token admin do contrato) e cria a **primeira empresa + o admin dela**. Abre
   sozinho quando a instalação está vazia. `web/react/src/features/platform/PlatformSetup.tsx`.
3. **Wizard do admin da empresa** (provedor → agente → canal) abre sozinho no
   primeiro login quando a empresa ainda está sem provedor/canal/agente.

O `super_admin` vê apenas o grupo de menu **Plataforma** (`app-shell.tsx`);
o admin da empresa configura o próprio workspace.

## Referências no código
- Papéis/permissões: `internal/web/perms.go`
- Gestão de equipe: `internal/web/api_react_users.go`
- Registro de rotas e guardas: `internal/web/server.go`, `internal/web/api_react.go`
- Sessão e tenant por host: `internal/store/tenant.go`, `internal/web/saas_guard.go`
- Webchat/widget: `internal/channel/webchat.go`
- Menu: `web/react/src/app/navigation.ts`
