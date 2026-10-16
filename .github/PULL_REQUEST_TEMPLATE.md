# Pull Request

<!-- Preencha em português (pt-BR). PRs fora do padrão podem ser fechados. -->

## O que este PR faz

Descreva o problema e a solução em poucas linhas.

## Tipo de mudança

- [ ] Correção de bug
- [ ] Nova funcionalidade
- [ ] Refatoração / melhoria interna
- [ ] Documentação
- [ ] Outro (descreva)

## Issue relacionada

Closes #<!-- número da issue -->

## Como testar

Passos para reproduzir/validar a mudança.

## Checklist obrigatório

- [ ] Rodei o gate e está tudo verde: `go build -buildvcs=false ./...`, `go vet ./...`, `go test ./...` e `npm run build` (em `web/react`).
- [ ] Não commitei segredos (tokens, senhas, chaves). Credenciais ficam no banco/painel ou em variáveis de ambiente.
- [ ] Se mexi em schema, criei uma **nova** migration `migrations/NNN_*.sql` (não editei as antigas).
- [ ] Se mexi em rota permissão/menu, atualizei os invariantes de QA (`internal/web/qa_invariants_test.go`) e a matriz de permissões.
- [ ] Li e segui o [`CONTRIBUTING.md`](../CONTRIBUTING.md) e o [`AGENTS.md`](../AGENTS.md).
- [ ] Atualizei a documentação afetada (README/docs) quando necessário.

## Capturas / evidências (opcional)

Cole prints ou saída de terminal que ajudem a revisar.
