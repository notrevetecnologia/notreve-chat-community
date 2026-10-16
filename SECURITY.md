# Security Policy

## Versões suportadas

Este projeto ainda está em fase ativa de desenvolvimento. Apenas a versão mais
recente do branch `main` recebe correções de segurança. Versões/commits
anteriores não são mantidos por enquanto.

## Reportando uma vulnerabilidade

A segurança é importante para nós. Se você encontrou uma falha de segurança,
**não abra uma issue pública** — reporte de forma privada para que possamos
corrigir antes de divulgar.

Como reportar:

1. **Marque a issue como privada** ou, se preferir, envie um e-mail privado.
2. **Não inclua o exploit completo** em texto público; descreva o suficiente
   para que a equipe entenda e reproduza.
3. Inclua: descrição da falha, o componente afetado (ex.: `internal/tools`,
   `internal/web`, canal), passos de reprodução, impacto e versão afetada.
4. Informe se preferencialmente pode ser divulgada publicamente após a correção.

## O que não deve ser reportado

- Erros `go vet`/lint ou build que já são cobertos pelo CI.
- Questões de configuração/operação documentadas em `docs/`.
- Vulnerabilidades em dependências de terceiros que já tenham correção
  conhecida (felicitamos se você encaminhar o CVE relevante).

## Política de divulgação responsável

- Trabalhamos com **divulgação coordenada**: esperamos o prazo razoável para a
  correção antes de qualquer divulgação pública.
- Reconhecemos o relato no `docs/CHANGELOG.md` (sem expor detalhes do exploit
  ou a identidade do reporter sem consentimento).

## Boas práticas de segurança no projeto

- **Segredos nunca ficam no repositório.** Credenciais de canais, provedores e
  tools são armazenadas **criptografadas no banco** e nunca retornadas ao painel
  (a UI só mostra `configured`).
- **`AGENTE_IA_SECRETS_KEY`** é a chave mestra (32 bytes base64, gerenciada no
  servidor) para derivar, por domínio, as chaves de segredos internos, LGPD,
  credenciais e assinaturas de webhooks.
- A role de runtime do PostgreSQL é **sem SUPERUSER/BYPASSRLS**; a conexão
  administrativa é usada apenas nas migrations/bootstrap.
- Recomenda-se manter a **allowlist de números** preenchida em produção.

## Ambiente de produção

Ao operar em produção:

- Gire `AGENTE_IA_SECRETS_KEY` e `WHATSAPP_HMAC_KEY` e faça backup seguro.
- Mantenha `.env` **fora** do controle de versão (já está em `.gitignore`).
- Ative HTTPS (o `Caddyfile` de exemplo já força TLS).
- Não ative RLS/schema que quebre as queries do store sem validação em staging.
