# Essência — Notreve Chat Community

> Este documento reordena o foco do produto. Antes de qualquer feature ou
> visual, o que importa é o problema que resolvemos e como o sistema age para
> resolvê-lo. As demais capacidades existem para servir a essa essência.

## O problema

Atendimento ao cliente no WhatsApp é, na prática, um dilema de "ou/ou":

- **"Assistente IA"**: um bot que responde sozinho 24/7, mas quando o cliente
  precisa de gente, trava. Não escala para a equipe.
- **"Multiatendimento"**: um painel de atendimento humano, mas sem um
  assistente bom pra atender primeiro — a equipe faz tudo, mesmo o que a IA
  poderia resolver.

O cliente não quer escolher. Ele quer **os dois no mesmo fluxo**: a IA atende
primeiro (com o conhecimento real da empresa e conectada a sistemas externos),
e **quando não resolver, cai para um humano** — sem perder contexto, no mesmo
painel.

Dificuldades reais que motivaram o produto:

- Conectar o atendimento a **recursos externos** (ex.: consultar saldo, status,
  CRM) — resolvido com **tools** (a IA chama APIs reais e responde com dados).
- Atender em **mais de um canal** (WhatsApp, Webchat, Telegram) mantendo o
  mesmo fluxo de IA + humano.
- Dar à IA **conhecimento de verdade** da empresa (não "alucinar" preço/producto).

## A essência (o que o sistema é)

**Um agente de IA multicanal que atende o cliente sozinho — usando o
conhecimento da empresa e ferramentas — e que transfere para um humano quando
preciso, tudo em um só painel.**

O coração do produto é o fluxo *IA → humano* bem-feito. Tudo o mais (contatos,
segmentos, campanhas, relatórios, planos, white-label, LGPD...) é suporte a esse
núcleo.

## As 4 frentes do produto

O Notreve Chat Community resolve 4 frentes em **uma só plataforma**:

1. **APIs / integração (para o CLIENTE integrar o próprio sistema)** — o cliente
   conecta a instância via **painel/QR**, e do sistema dele (ERP) consegue
   **enviar mensagens**, **receber respostas via webhook** e **gerenciar conexão**
   — usando a **API do Agente IA**, sem precisar da Notreve WhatsApp API. Inclui
   também a API de conversa (abrir/reusar, texto, consultar, encerrar) e webhooks.
2. **Multiatendimento / chatbot** — o Inbox IA→humano (filas, setores, distribuição,
   avaliação).
3. **Agentes de IA / tools** — conhecimento do agente + tools que chamam sistemas
   externos (catálogo, risco, escopos, auditoria).
4. **Campanhas / envio em massa** — grupos de canais (failover), listas de
   contatos (importação na hora), agendamento, limites e modo teste.

> **Posicionamento:** a **Notreve WhatsApp API** é o produto **B2B/atacado**
> (máquina bruta de WhatsApp para outros públicos/revendas). Já o
> **Notreve Chat Community** é a plataforma **self-service** (multiatendimento + IA +
> campanhas) cujo cliente pode **integrar o próprio ERP** via API/webhooks do
> painel, sem contratar a API de atacado. As duas convivem — públicos distintos.

### O cliente é livre — IA, chatbot tradicional ou híbrido

O produto **não obriga IA**. Damos os **blocos** e o cliente monta o atendimento
do jeito que quiser:

- **Só IA** — agente com conhecimento + tools (IA decide a conversa).
- **Chatbot tradicional** — fluxo determinístico (setores, filas, mensagens,
  condições) **sem IA**, como sistemas clássicos de chatbot.
- **Híbrido** — mistura os dois: alguns ramos com IA, outros com fluxo fixo; a IA
  pode também decidir o rumo quando o cliente não definir.

Os **blocos são os mesmos** para os três modos: `message`, `collect`, `condition`,
`tool`, **`agent`/IA (opcional)**, `transfer`/setor, `end`. **Setores e filas são
blocos nativos** — não dependem de IA (a IA é uma opção, não um pré-requisito).
Isso **quebra o tabu**: o cliente faz como ele quer. É o mesmo construtor universal
(documentado em `docs/ESPECIFICACAO.md` e `docs/ROADMAP.md`).

### Produtos-base (do dono) e referências

- **Notreve WhatsApp API** — B2B/atacado (máquina de WhatsApp; contrato novo no
  webhook: `phone`/`phoneJid`/`isFromMe`/`messageType`/`mediaLink`/`event`, `Delivery`,
  mídia `base64|s3|both`, HMAC `x-hmac-signature`).
- **Notreve IA** — plataforma de IA (provedor do motor).
- **Notreve Chat** — multiatendimento/branding.

## Como o sistema age (regras de ouro)

1. **IA atende primeiro.** Enquanto a conversa está em estado `ia`, a IA
   responde. Ela usa a **base de conhecimento** do agente (texto, PDF, site,
   áudio) e **tools** (chamadas a sistemas externos) para responder com dados
   reais.
2. **A IA nunca transfere por conta própria.** Nem por horário, nem por
   "acha que não sabe". A transferência só acontece se:
   - o cliente **pedir** um humano (`[HUMANO]`) ou um setor (`[SETOR:Nome]`), ou
   - ocorrer **falha/erro** no processamento da IA.
3. **Quando transfere, entrega com contexto.** A conversa vai para a fila do
   setor/humano (distribuição automática: atendente online, horário Brasília,
   menos carregado), e o humano assume no **Inbox**.
4. **Humano assume e pode devolver.** No estado humano, a equipe responde; a
   conversa pode voltar para a IA ou ser encerrada.
5. **Encerramento com satisfação.** Ao encerrar, o cliente avalia; se não
   avaliar, **follow-up** pergunta depois. Feedback alimenta relatórios e
   ensina o bot (aprendizado).
6. **Segurança de envio.** O bot só responde a números autorizados
   (allowlist). Campanhas têm intervalos, limites diários e modo teste para não
   bloquear o número.

## O fluxo, em uma linha

> Cliente escreve → **IA entende** (conhecimento + tools) e responde →
> pediu humano/erro? → **transfere e o humano atende** → encerra e avalia →
> **relatório/aprendizado** melhora a próxima resposta.

## O que cada área do painel sustenta

- **Atendimento**: Inbox (IA ↔ humano), avaliações, fila.
- **Relacionamento**: contatos, segmentos, campanhas, pagamentos.
- **Inteligência**: agentes, base de conhecimento, provedores de IA, tools,
  chat flow.
- **Canais**: WhatsApp, Webchat, Telegram.
- **Operação**: equipe, setores, relatórios, aprendizado, biblioteca de
  mensagens.
- **Plataforma**: configurações, planos/limites, white-label, LGPD, usuários.

> Qualquer nova idea entra aqui como suporte ao núcleo IA → humano. Se não
> melhora o atendimento nem a operação, fica na fila de ideias (ROADMAP).
