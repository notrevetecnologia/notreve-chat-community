# Setup — Passo a passo (1 → 10)

> Objetivo: você consegue, em poucos passos, ter um **agente de IA que atende
> sozinho e transfere para humano** operando no seu WhatsApp — tudo no **Inbox**.
> Siga na ordem. Cada passo indica onde fica (menu do painel).

## 1. Subir o painel

- Com Docker: `git clone` do repo no servidor, criar `.env` e
  `docker compose up -d --build`. Acesse `http://IP:8080`.
- Sem Docker: PostgreSQL + Go 1.22+, aplicar migrations e `go run .`.
- Login inicial: usuário `admin`, senha = `AGENTE_IA_PASS` do `.env`.

## 2. Conectar o canal

Menu **Canais**. É por aqui que o cliente chega.
- **WhatsApp** (Notreve WhatsApp API): a infraestrutura do WhatsApp é
  administrada pela Notreve — **contrate o serviço em
  [whatsapp.notreve.com.br](https://whatsapp.notreve.com.br)** e então informe
  `base_url` + token da API e conecte por QR (botão Conectar).
- **Webchat**: gera o widget para colocar no site (embutível).
- **Telegram**: token do bot → URL do webhook.
> Depois de conectar, teste mandando uma mensagem real no canal.

## 3. Configurar a IA (provedor)

Menu **Inteligência → Provedores de IA**.
- Qualquer API **compatível com OpenAI** funciona (OpenAI, Gemini, DeepSeek,
  local): informe `base_url`, **modelo** e **chave**.
- **Recomendado:** o preset **[Notreve IA](https://ia.notreve.com.br)** — só o
  token; a Notreve gerencia endpoint, modelos e transcrição. Dê um nome e
  **ative**. (Essa é a "cabeça" do bot.)

## 4. Criar o agente (conhecimento)

Menu **Inteligência → Agentes**. Crie um agente e descreva:
- **Instruções**: como agir (ex.: "atendente virtual da loja X, tom cordial,
  nunca inventa preço, usa a base de conhecimento, se souber o total do pedido
  informe").
- **Base de conhecimento**: o que dá informação real → **texto**, **PDF**,
  **site** (aprende sozinho) e **áudio**. É o que evita "alucinar" preço/produto.
- (Opcional) **Menu de opções no 1º contato**: lista numerada para guiar o
  cliente no início.
- **Temperatura**: menor = mais objetivo; maior = mais criativo.

## 5. Conectar sistemas externos (tools)

Menu **Inteligência → Tools** (se precisar).
- Uma **tool** deixa a IA chamar uma API sua e responder com **dados reais**
  (ex.: "qual meu saldo?", "status do pedido").
- Configure: nome, método, URL, credencial, parâmetros, escopos, risco e
  allowlist de host. **Sem secret no repo.**

## 6. Montar a equipe

Menu **Operação → Equipe e setores**.
- Crie **atendentes** e **setores** (ex.: Financeiro, Suporte, Vendas).
- Defina perfis/permissões (atendente, supervisor, admin) e **horário** de
  trabalho (Brasília). É isso que a distribuição automática usa.

## 7. Ajustar as regras (comportamento)

Menu **Configurações** (Operação/Atendimento).
- **Números permitidos** (allowlist): só esses são respondidos (segurança).
- **Distribuição automática**: quem assume uma conversa transferida
  (setor + atendente online + menos carregado).
- **IA na fila**: opção de a IA dar um "respiro" enquanto o humano ainda não
  assumiu.
- **Follow-up** e **satisfação**: reengajar quem não avaliou.
- (Campanhas: só se for disparo em massa — com limite diário e modo teste.)

## 8. Testar o fluxo de ponta a ponta

Vá ao **Inbox** (o coração do produto) e valide o ciclo completo:
1. Cliente manda mensagem → a **IA responde** (usando a base de conhecimento).
2. Peça "quero falar com um humano" → a conversa **cai para a fila** do setor.
3. **Assuma** no Inbox → responda como humano → confirme que a IA parou.
4. Encerre → **avaliação** de satisfação.

## 9. Ir ao vivo e acompanhar

- Confira **Relatórios** (média de resposta, transferência, notas).
- Veja **Aprendizado**: perguntas que a IA não respondeu → melhore a base de
  conhecimento.
- Ajuste instruções/tools conforme a operação.

## 10. Operação contínua

- **Inbox** é lido o dia todo: a IA atende o fluxo, você assume o que ela
  transferiu.
- **Campanhas** para reativar clientes, **Contatos/Segmentos** para organizar a
  base, **Pagamentos** (se usar) para cobrar dentro da conversa.

---

### Regra que nunca muda

> **IA atende primeiro; transfere só se o cliente pedir humano/setor ou em
> erro; o humano assume no Inbox com contexto.** Se você lembrar só disso, o
> produto está no caminho certo.
