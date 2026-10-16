# Especificação comportamental — Notreve Chat Community

> Este é o desenho do que o sistema **tem que fazer** e **como tem que fazer**.
> É a âncora contra desvio: qualquer feature nova deve servir a estes pilares.
> Este documento vale mais que o código — se código e spec divergirem, a spec
> ganha (e o código é corrigido para voltar ao trilho).

## 1. O que é (e o que não é)

**É:** um painel que entrega **um agente de IA multicanal** (WhatsApp, Webchat,
Telegram) capaz de **atender sozinho** o cliente e, quando necessário,
**transferir para atendimento humano** (multiatendimento: filas, setores,
distribuição), reunindo o que a concorrência vende em dois produtos separados
("assistente IA" **ou** "multiatendimento").

**NÃO é:** uma ferramenta de "enviar lista e esquecer". É uma **operação de
atendimento** — o envio em massa (campanha) é suporte, não o produto.

## 2. Os pilares (não negociáveis)

| Pilar | O que faz | Por que é ouro |
|---|---|---|
| **Atendimento IA → Humano** | IA atende; transfere só se pedir humano/setor ou erro; humano assume no Inbox com contexto | O coração do produto |
| **Tools = motor de regras** | Combina regras (identificação + licença) para autorizar ações em sistemas externos (ex.: consultar boleto em um sistema externo) | É o motor de automação do agente: **N possibilidades** |
| **Follow-up** | Reengaja cliente que não respondeu/avaliou | Preenche a operação, captura avaliação |
| **Campanhas + Grupos de canais** | Disparo em massa por grupo de canais, com intervalo, limite, failover e **modo teste** | Ouro pra quem faz disparo |
| **Lista na hora da campanha** | Criar campanha **importando uma lista de contatos** no ato (ou por segmento) | Luxo operacional |

## 3. Como cada pilar se comporta

### 3.1 Atendimento (IA → humano)

- A IA apenas responde enquanto a conversa está em estado `ia`.
- **Nunca transfere sozinha** (nem por horário, nem por incerteza). Só transfere se:
  - o cliente pedir humano (`[HUMANO]`) ou setor (`[SETOR:Nome]`), ou
  - ocorrer **falha/erro** da IA.
- Ao transferir: a conversa entra na fila do setor → **distribuição automática**
  (atendente online, horário **Brasília**, menos carregado) → humano assume no
  **Inbox** com o histórico.
- Encerramento → **avaliação**; sem avaliação → **follow-up** pede depois.
- Se faltar informação, a IA diz o que sabe e orienta; **não inventa** preço/produto.

### 3.2 Tools (motor de regras)

Uma tool é uma **regra composta** que habilita uma ação em um sistema externo.
Exemplo (consultar fatura/boleto):

```
Regra de autorização (combinada):
  - identificação: telefone do cliente (e/ou CPF/CNPJ informado)
  - AND chave/licença: este cliente TEM permissão para consultar a própria fatura?
  - AND integração: WHMCS configurada (URL + credencial + escopo)
  -> se TUDO confirmar: chama a API da integração e devolve o boleto/fatura
  -> senão: nega educadamente e (opcional) transfere para humano
```

- Configurada no painel: nome, integração (URL/credencial), método, parâmetros,
  escopos, risco, allowlist de host, timeout, **validação anti-SSRF** e auditoria.
- A IA usa a tool quando a regra faz sentido, preenchendo os parâmetros.
- **Sem segredo no repo** (credenciais ficam no painel/banco). **Sem expor**
  a chave para o cliente na resposta **a menos que a regra autorize**.
- Novas possibilidades nascem ao combinar tools (consultar saldo, status,
  boleto, CRM...) — sem mudar o núcleo do agente.

### 3.3 Follow-up

- Dispara uma mensagem de reengajamento **via Sender** (respeita a allowlist)
  quando o cliente não respondeu/avaliou num prazo configurável.
- Sempre dentro das regras de envio (intervalo/limite) para não bloquear o número.

### 3.4 Campanhas + Grupos de canais

- **Grupos de canais**: agrupar vários números/instâncias por grupo (Failover
  automático entre canais do grupo).
- **Alvo**: (a) um **segmento**, ou (b) **lista importada na hora** (upload CSV
  de contatos no ato de criar a campanha), ou (c) contatos avulsos.
- **Agendamento** (horário Brasília), **intervalo entre envios**, **limite
  diário**, **cooldown** por canal e **failover** — para não bloquear o número.
- **Modo teste**: marca como enviado SEM chamar a API (testa o fluxo sem gastar).
- Acompanhamento: status por envio, pausa, cancelamento, estatísticas por canal.

## 3.1 Painel do admin (controle e relatórios)

O **admin** (dono da empresa) tem visão e controle da operação e do gasto:

- **Visão geral**: conversas ativas, aguardando humano, com IA, por canal; TMA,
  taxa de transferência, NPS, abandono de fila, satisfação.
- **Conversas**: busca/filtro (cliente, setor, status, canal, datas), export; abrir
  cada conversa com histórico, quem atendeu, **tokens gastos naquela conversa**,
  tools usadas e transferências.
- **Uso de IA**: **tokens por conversa**, por cliente e por agente (prompt +
  completion), chamadas; **custo estimado** quando o provedor expuser preço.
- **Equipe**: desempenho por atendente (conversas assumidas, TMA, satisfação).
- **Canais**: saúde (conectada/instância) e volume de mensagens.
- **Limites do plano vs uso**: barras de consumo (agentes, canais, mensagens/dia,
  créditos IA, etc.) — mostra o limite efetivo e o quanto já foi usado (enforcement,
  **não** cobrança; billing é externo).
- **Auditoria**: acessos e ações sensíveis (via `audit_log`).

> Controle do super_admin (plataforma): planos/limites, empresas + status, admins,
> servidores WhatsApp, uso por empresa, troca de contexto auditada (já existe).

## 4. Regras de segurança (não quebrar)

- **Allowlist**: o bot só responde a números permitidos (não vale para campanha,
  que é envio explícito do admin).
- **Anti-bloqueio**: nunca facilitar envio em massa sem intervalo/limite.
- **Dados sensíveis**: consentimento LGPD, criptografia em repouso, mascaramento,
  auditoria. Nunca logar/retornar dado sensível sem máscara.
- **Sem segredo no repo**; APIs nunca devolvem chave/token (só `configured`).

## 5. Checklist — isso ainda é o produto? (anti-desvio)

Antes de aceitar qualquer nova ideia/feature, responda:

- [ ] Serve ao **núcleo** (IA atende → transfere → humano assume com contexto)?
- [ ] Melhora **Tools** (mais possibilidades), **Follow-up** ou **Campanhas**
      (grupos/importação de lista)?
- [ ] Não quebra as regras de segurança (allowlist/anti-bloqueio/LGPD)?
- [ ] Se for só admin/visual, é suporte (não o produto)?

Se não passar, vai para a fila de ideias (`docs/ROADMAP.md`) — não entra no núcleo.

> A regra que resume tudo: **"IA atende primeiro; transfere só quando o cliente
> pedir humano/setor ou em erro; o humano assume no Inbox com contexto. Tools
> conectam sistemas externos por regras; follow-up reengaja; campanha dispara
> com segurança."**
