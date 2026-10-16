# Produção — Checklist e Operação

Guia para subir o Notreve Chat Community em produção com segurança.

## Requisitos mínimos
- **Servidor VPS/dedicado com Ubuntu 24.04 ou 26.04** e Docker + Docker Compose
  (o instalador instala o Docker se faltar). É o ambiente **suportado**.
  - Rodar em máquina local (`localhost`, Windows/macOS) é **por sua conta e
    risco** — não é suportado.
- PostgreSQL, Redis e RabbitMQ (o app funciona mesmo sem Redis/Rabbit — fallback em memória).
- DNS/domínio apontando pro servidor (recomendado) e **HTTPS**.

## Recomendado (antes de produzir)
1. **Reverse proxy com HTTPS** (Nginx/Caddy/Traefik) na frente da porta 8080.
   - O painel e o webhook devem ser acessíveis por HTTPS.
   - Ex.: `https://atendimento.seuclientedominio.com` → `http://127.0.0.1:8080`.
2. **Configurar a URL pública do painel** no painel (Config → WhatsApp → "URL pública do painel")
   = a URL HTTPS que a API WhatsApp vai chamar como webhook.
3. **Limitar os números** (Config → WhatsApp → "Responder somente para estes números"):
   - Em produção, coloque APENAS os números reais da empresa (vazio = responde pra qualquer um — perigoso).
4. **Usuários/perfis**:
   - `super_admin` = operação (Notreve) — configura servidores WhatsApp e módulos.
   - `admin` = cliente (dono do sistema) — configura IA (inclui STT), equipe, campanhas.
   - `supervisor` / `atendente` = equipe do cliente.
   - Troque as senhas padrão antes de produzir.
5. **Backup do banco**: agende dump do PostgreSQL (volume `pgdata`) diário.

## Passos de produção (ordem)
1. `docker compose up -d --build` no servidor.
2. Login como `super_admin` → Config → WhatsApp:
   - Base URL da API + token admin (servidores WhatsApp).
   - URL pública do painel (HTTPS).
3. Canais → "🔗 Conectar" cada número (QR) → confere conectado.
4. Config → IA padrão:
   - Provedor de chat (multimodal se quiser ver foto/PDF).
   - Transcrição de áudio (STT): Groq `https://api.groq.com/openai/v1` + `whisper-large-v3` ou OpenAI `whisper-1`.
5. Agentes → selecione a IA + base de conhecimento (links do site).
6. Teste: mande mensagem, áudio, foto, PDF, transferência e encerramento.

## Fuso horário
- Todos os horários são **America/Sao_Paulo** (containers com `TZ`). Não mude.

## Operação
- Logs: `docker compose logs app --tail`.
- Reinício: `docker compose up -d --build app`.
- Migrations aplicam sozinhas no start (crie sempre uma nova `migrations/NNN_*.sql`).

## Segurança
- Login tem proteção anti brute-force (10 tentativas/min por IP).
- Senhas com bcrypt. Segredos (API keys/tokens) vivem no banco/painel — **nunca** no repositório.
- O painel é interno: restrinja o acesso por firewall/IP se possível.
- A assinatura HMAC do webhook recebido da Notreve WhatsApp API é opcional:
  requisições sem `x-hmac-signature` são aceitas; uma assinatura só é validada
  quando o cabeçalho é enviado. Não ativar exigência obrigatória sem alterar o
  contrato da API e testar uma entrega real.

## Limites/anti-bloqueio do WhatsApp
- A IA segue a allowlist de números.
- Campanhas: intervalo, limite diário e modo teste — use com cuidado (risco de bloqueio do número).
- Números novos: faça "warm-up" (sem envio em massa na 1ª semana).
