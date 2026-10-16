# Contribuindo

Obrigado por querer contribuir com o **Notreve Chat Community**! Este guia resume o
fluxo e as regras para que suas mudanças sejam aceitas com rapidez e qualidade.

> Leia o [`AGENTS.md`](AGENTS.md) primeiro — ele contém as regras obrigatórias
> que os agentes de IA (e contribuidores) devem seguir neste repositório.

## Começando

1. Faça um **fork** do repositório e crie uma branch a partir do `main`.
2. Rode o projeto localmente e o gate completo antes de começar.
3. Mantenha commits pequenos e descritivos, em **pt-BR**.

## O que é o projeto (recorte)

Plataforma **multi-tenant** de atendimento com IA + time humano no WhatsApp,
que une: **API/integração**, **multiatendimento/inbox**, **agentes de IA/tools**
e **campanhas**. Backend em Go (sem framework, pgx), frontend React/TypeScript
(Vite) em `web/react`, PostgreSQL com migrations.

## Fluxo de trabalho

1. **Entenda o estado atual** (banco + código) antes de mexer.
2. **Mudança mínima**, seguindo os padrões existentes — não refatore/“melhore” o
   que não está relacionado.
3. Rode o **gate obrigatório** (veja abaixo).
4. **Commit claro (pt-BR)** e push da branch, depois abra um *Pull Request*.

## Gate obrigatório (antes de entregar)

```bash
go build -buildvcs=false ./...
go vet ./...
go test ./...
cd web/react && npm run build
```

> Se este é o seu primeiro build, instale as dependências com `npm ci` em
> `web/react` e o Go 1.26+ (veja `go.mod`).

### Regras que não devem ser quebradas

- **Migrações são imutáveis.** Nunca edite as antigas; crie
  `migrations/NNN_*.sql` novas.
- **Nunca commite segredos** (API keys, tokens, senhas). Eles vivem no banco
  (configurados pelo painel) ou em variáveis de ambiente. APIs nunca devolvem
  chave/token (só `configured`).
- **Hammer of the product:** a IA **nunca transfere por conta própria** — só se
  o cliente pedir um humano (`[HUMANO]`) ou setor (`[SETOR:Nome]`), ou em falha.
- **Nunca torne o HMAC do webhook obrigatório** sem confirmar mudança no
  contrato da API; a assinatura é opcional (sem cabeçalho → aceita; cabeçalho
  inválido → rejeita).
- Não facilite **envio em massa** sem as proteções de intervalo, limite diário,
  cooldown e modo teste.
- **Não ative RLS/schema** que quebre as queries do store sem validação em
  staging.

## Como contribuir por área

### Novos canais

Implemente `channel.Adapter` (`Send`/`SendMedia`) e registre no
`channel.Registry`. O núcleo (IA/webhook/painel) fica intocado — **não reescreva
o centro**.

### Novas mensagens ricas

O motor (`internal/agent`) pode produzir `agent.RichMessage` (botões, lista,
enquete, contato, localização, figurinha). A entrega é no `channel`, sempre com
**fallback para texto numerado** caso o canal não suporte.

### Tools / integrações

Use o catálogo + política (risco, escopos, confirmação), allowlist de host,
anti-SSRF, timeout e auditoria. Para uma tool que exige identificação, use o
guard com modos `phone`/`challenge`/`resolve` via verifier externo.

## Documentação

- Atualize `docs/CHANGELOG.md` quando algo relevante mudar.
- Novas features/ideias entram em `docs/ROADMAP.md` — fique apenas o que é
  prioridade das fases.
- Mantenha `docs/ARQUITETURA.md` atualizado quando a arquitetura mudar.

## Pull Request

- Descreva o **problema** que resolve e a **solução**.
- Referencie testes que validam a mudança.
- Confirme que o gate passou (cole a saída do `go test`/`npm run build`).

## Segurança

Reporte falhas de segurança de forma **privada** — veja o
[`SECURITY.md`](SECURITY.md).
