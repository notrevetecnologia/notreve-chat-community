package channel

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/agent"
	"github.com/notrevetecnologia/notreve-chat-community/internal/infra"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
	"github.com/notrevetecnologia/notreve-chat-community/internal/webhooks"
)

type Processor struct {
	st        *store.Store
	eng       *agent.Engine
	inf       *infra.Infra
	summarize func(ctx context.Context, convID int, a store.Agent) (string, error)
	// O webhook da API pode ser reenviado em paralelo. O lock cobre apenas a
	// janela check+insert do dedupe; a chamada à IA continua concorrente.
	dedupMu sync.Mutex
	convMu  sync.Mutex
	conv    map[int]*conversationLock
}

type conversationLock struct {
	mu   sync.Mutex
	refs int
}

var errConversationPhoneInvalid = errors.New("telefone inválido para abrir conversa")

func NewProcessor(st *store.Store, eng *agent.Engine, inf *infra.Infra) *Processor {
	p := &Processor{st: st, eng: eng, inf: inf, conv: map[int]*conversationLock{}}
	if eng != nil {
		p.summarize = eng.SummarizeConversation
	}
	return p
}

func (p *Processor) lockConversation(id int) func() {
	p.convMu.Lock()
	if p.conv == nil {
		p.conv = map[int]*conversationLock{}
	}
	entry := p.conv[id]
	if entry == nil {
		entry = &conversationLock{}
		p.conv[id] = entry
	}
	entry.refs++
	p.convMu.Unlock()

	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		p.convMu.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(p.conv, id)
		}
		p.convMu.Unlock()
	}
}

type Inbound struct {
	Message string
	Phone   string
	Name    string
	AgentID *int
	// TenantID e ChannelID são preenchidos pelo adaptador do canal antes do
	// processamento. O tenant mantém o fluxo isolado e o channel_id registra
	// qual instância recebeu a última mensagem.
	TenantID  int
	ChannelID *int
	WAMsgID   string
	Channel   string
	Origem    string

	MediaType    string
	MediaB64     string
	MediaMime    string
	MediaName    string
	MediaCaption string

	Lat         float64
	Lng         float64
	ContactName string
	ContactTel  string
}

type Result struct {
	ConversationID int
	Status         string
	SetorName      string
	Reply          string
	Fallback       string
	Rich           *agent.RichMessage
	UsedTools      []string
	Handled        bool // false quando a conversa está em modo humano (só registrou)
}

// Process roteia uma mensagem de entrada: cria/acha a conversa, roda a IA
// (ou apenas registra se estiver em modo humano) e devolve a resposta.
func (p *Processor) Process(ctx context.Context, in Inbound) (Result, error) {
	tenantID, err := store.TenantIDRequired(ctx)
	if err != nil {
		return Result{}, err
	}
	if in.TenantID > 0 && in.TenantID != tenantID {
		return Result{}, fmt.Errorf("inbound tenant does not match context")
	}
	in.TenantID = tenantID
	agentID := in.AgentID
	if agentID == nil && in.ChannelID != nil {
		if channelAgentID := p.channelAgentID(ctx, *in.ChannelID); channelAgentID != nil {
			if a, err := p.st.GetAgentForTenant(ctx, in.TenantID, *channelAgentID); err == nil && a.Active {
				agentID = channelAgentID
			}
		}
	}

	convID, created, err := p.findOrCreateConversation(ctx, in, agentID)
	if err != nil {
		if errors.Is(err, errConversationPhoneInvalid) {
			return Result{}, nil
		}
		return Result{}, err
	}
	releaseConversation := p.lockConversation(convID)
	defer releaseConversation()

	conv, err := p.st.GetConversation(ctx, convID)
	if err != nil {
		return Result{}, err
	}

	if created {
		publishWebhook(ctx, p.st, webhooks.EventConversationCreated, "conversation.created:"+strconv.Itoa(convID), map[string]interface{}{
			"conversation": store.ConversationWebhookPayload(conv),
		})
	}

	out := Result{ConversationID: convID, Status: conv.Status}
	// O padrão seguro é registrar falhas e encaminhar para a equipe. A empresa
	// pode desligar qualquer uma das duas ações nas configurações operacionais.
	registrarFalhas := true
	transferirFalhas := true
	if settings, settingsErr := p.st.GetSettings(ctx); settingsErr == nil {
		registrarFalhas = settings.RegistrarFalhasAprendizado
		transferirFalhas = settings.TransferirFalhasHumano
	}

	inbound := store.Message{ConversationID: convID, Role: "user", Content: in.Message, Source: "customer", WAMsgID: in.WAMsgID,
		MediaType: in.MediaType, MediaB64: in.MediaB64, MediaMime: in.MediaMime, MediaName: in.MediaName,
		Lat: in.Lat, Lng: in.Lng, ContactName: in.ContactName, ContactTel: in.ContactTel}
	// Dedup antes de gravar: o lock impede dois retries simultâneos dentro
	// deste processo de ambos passarem pelo check e gerarem duas respostas.
	if in.WAMsgID != "" {
		p.dedupMu.Lock()
		exists, err := p.messageExists(ctx, in)
		if err == nil && !exists {
			err = p.addMessage(ctx, in.TenantID, inbound)
		}
		p.dedupMu.Unlock()
		if err != nil {
			return Result{}, err
		}
		if exists {
			out.Handled = false
			return out, nil
		}
	} else if err := p.addMessage(ctx, in.TenantID, inbound); err != nil {
		return Result{}, err
	}
	inbound.ID = 0
	inbound.CreatedAt = time.Now().UTC()
	publishWebhook(ctx, p.st, webhooks.EventMessageReceived, "message.received:"+strconv.Itoa(convID)+":"+receivedMessageKey(in.WAMsgID), map[string]interface{}{
		"conversation": store.ConversationWebhookPayload(conv),
		"message":      store.MessageWebhookPayload(inbound),
	})
	_ = p.st.TouchConversation(ctx, convID)
	// Cliente voltou a falar: libera novo follow-up se ele ficar sem responder de novo.
	_ = p.st.ResetConversationFollowup(ctx, convID)

	if conv.BotBlocked {
		out.Status = conv.Status
		out.Handled = false
		return out, nil
	}
	if p.botDetected(ctx, in, convID) {
		p.blockBot(ctx, convID)
		out.Status = conv.Status
		out.Handled = false
		return out, nil
	}

	if conv.Status == "closed" && (conv.Satisfacao == "aguardando" || conv.Satisfacao == "aguardando_operador") {
		out.Handled = true
		hasAI, hasHuman, participantErr := p.st.SatisfactionParticipants(ctx, convID)
		if participantErr != nil {
			Logger.Printf("erro identificando participantes da avaliação: %v", participantErr)
			hasAI = true
			hasHuman = conv.AtendenteID != nil
		}
		target := satisfactionTarget(hasAI, hasHuman)
		if conv.Satisfacao == "aguardando_operador" {
			target = "operador"
		}

		rating, valid := agent.ParseSatisfactionRating(in.Message)
		if !valid {
			out.Status = "closed"
			out.Reply = satisfactionRetry(target)
			if err := p.addMessage(ctx, in.TenantID, store.Message{ConversationID: convID, Role: "assistant", Source: "system", Content: out.Reply}); err != nil {
				Logger.Printf("erro registrando repetição da avaliação: %v", err)
			}
			return out, nil
		}

		evaluation := store.Avaliacao{ConversationID: convID}
		nextState := "finalizada"
		if target == "operador" {
			evaluation.NotaOperador = &rating
		} else {
			evaluation.NotaIA = &rating
			if hasHuman {
				nextState = "aguardando_operador"
			}
		}
		if err := p.st.SaveAvaliacaoAndSetSatisfacao(ctx, evaluation, nextState); err != nil {
			Logger.Printf("erro registrando avaliação: %v", err)
			out.Status = "closed"
			out.Reply = "Não consegui registrar sua nota agora. Por favor, envie novamente um número de 1 a 5."
			return out, nil
		}
		publishWebhook(ctx, p.st, webhooks.EventSatisfactionCompleted, "satisfaction.completed:"+strconv.Itoa(convID)+":"+strconv.FormatInt(time.Now().UnixNano(), 10), map[string]interface{}{
			"conversation": store.ConversationWebhookPayload(conv),
			"target":       target,
			"nota":         rating,
		})

		if nextState == "aguardando_operador" {
			out.Reply = "Obrigado por avaliar nossa assistente virtual! Agora, " + lowerFirst(SatisfactionQuestion(ctx, p.st, "operador"))
		} else if hasAI && hasHuman {
			out.Reply = "Obrigado! As avaliações da assistente virtual e do atendente foram registradas. Protocolo " + ticketLabel(convID) + " encerrado."
		} else {
			out.Reply = "Obrigado! Sua avaliação foi registrada. Protocolo " + ticketLabel(convID) + " encerrado."
		}
		if err := p.addMessage(ctx, in.TenantID, store.Message{ConversationID: convID, Role: "assistant", Source: "system", Content: out.Reply}); err != nil {
			Logger.Printf("erro registrando resposta da avaliação: %v", err)
		}
		out.Status = "closed"
		return out, nil
	}

	if conv.Status != "closed" && isEndConversationRequest(in.Message) {
		closed, err := p.st.EncerrarConversationIfOpen(ctx, convID, "Encerrado pelo cliente", nil)
		if err != nil {
			return out, err
		}
		if closed {
			out.Reply, err = BeginSatisfaction(ctx, p.st, convID)
			if err != nil {
				return out, err
			}
			if rich := SatisfactionRich(out.Reply); rich != nil {
				out.Rich = rich
				out.Fallback = rich.Fallback
				out.Reply = rich.Fallback
			}
			publishWebhook(ctx, p.st, webhooks.EventConversationClosed, "conversation.closed:"+strconv.Itoa(convID), map[string]interface{}{
				"conversation": store.ConversationWebhookPayload(conv),
				"reason":       "Encerrado pelo cliente",
			})
		}
		out.Status = "closed"
		out.Handled = true
		return out, nil
	}

	// IA ajuda na fila: enquanto o cliente espera o humano (humano_aguardando),
	// a IA responde dúvidas sem tirá-lo da fila. Em humano_atendendo a IA fica
	// quieta (o humano está ativo na conversa).
	iaNaFila := false
	if conv.Status == "humano_aguardando" {
		if st, err := p.st.GetSettings(ctx); err == nil && st.IaNaFila {
			iaNaFila = true
		}
	}

	// A IA SÓ responde se a conversa estiver em modo "ia" (ou IA-na-fila).
	// Qualquer outro estado = só registra a mensagem pro humano atender.
	if conv.Status != "ia" && !iaNaFila {
		out.Status = conv.Status
		out.Handled = false
		return out, nil
	}
	out.Handled = true

	agentObj, agentErr := p.resolveAgent(ctx, conv)
	target := p.chatflowTarget(ctx, conv.ChannelID)
	var reply agent.Reply
	if agentErr != nil && target.Mode != "chatflow" {
		// Sem agente, o webhook não pode devolver 500 e deixar o cliente sem
		// retorno. A dúvida vira aprendizado e a fila humana é opcional.
		Logger.Printf("nenhum agente disponível (%s): %v", in.Phone, agentErr)
		p.recordFailure(ctx, store.AgentFailure{
			ConversationID: &convID, Channel: conv.Channel, Phone: in.Phone,
			Origem: "resolver_agente", Tipo: store.FailureKindPipeline,
			ExceptionClass: fmt.Sprintf("%T", agentErr), MensagemErro: agentErr.Error(),
		})
		reply = p.noAgentReply()
	} else {
		reply, err = p.eng.HandleUserMessage(ctx, convID, agentObj, conv.Channel, target, agent.Media{
			Type: in.MediaType, B64: in.MediaB64, Mime: in.MediaMime, Name: in.MediaName, Caption: in.MediaCaption,
		})
		if err != nil {
			// Segurança: nunca deixar o cliente sem atendimento.
			Logger.Printf("erro no motor de IA (%s): %v", in.Phone, err)
			p.recordFailure(ctx, store.AgentFailure{
				ConversationID: &convID, Channel: conv.Channel, Phone: in.Phone,
				Origem: "engine", Tipo: store.ClassifyAgentFailure(err),
				ExceptionClass: fmt.Sprintf("%T", err), MensagemErro: err.Error(),
			})
			reply = p.fallbackReply(ctx)
		}
	}
	if reply.RecordLearning && transferirFalhas {
		reply.NeedsHuman = true
	}
	// Remove o marcador interno antes de persistir a resposta, para que ele
	// nunca apareça no histórico do painel nem seja reenviado ao cliente.
	if i := strings.Index(reply.Text, "[NOME:"); i >= 0 {
		end := strings.Index(reply.Text[i:], "]")
		if end > 0 {
			nome := strings.TrimSpace(reply.Text[i+6 : i+end])
			reply.Text = strings.TrimSpace(reply.Text[:i] + reply.Text[i+end+1:])
			if nome != "" {
				_ = p.st.UpdateConversationName(ctx, convID, nome)
				if in.Phone != "" {
					_ = p.st.UpsertContato(ctx, store.Contato{Name: nome, Phone: in.Phone})
				}
			}
		}
	}

	// Sem conteúdo (ex.: IA não configurada) não há mensagem a enviar: não
	// persiste vazio nem publica evento, para nunca vazar texto interno ao
	// cliente. Transferência/aprendizado continuam valendo abaixo.
	if strings.TrimSpace(reply.Text) != "" || reply.Rich != nil {
		if err := p.addMessage(ctx, in.TenantID, store.Message{ConversationID: convID, Role: "assistant", Content: reply.Text, Source: "ai"}); err != nil {
			return out, err
		}
		publishWebhook(ctx, p.st, webhooks.EventMessageSent, "message.sent:"+strconv.Itoa(convID)+":ai:"+strconv.FormatInt(time.Now().UnixNano(), 10), map[string]interface{}{
			"conversation": store.ConversationWebhookPayload(conv),
			"message": store.MessageWebhookPayload(store.Message{
				ConversationID: convID, Role: "assistant", Source: "ai", Content: reply.Text, CreatedAt: time.Now().UTC(),
			}),
		})
	}

	if reply.NeedsHuman {
		if err := p.st.SetConversationStatus(ctx, convID, "humano_aguardando"); err != nil {
			Logger.Printf("erro ao marcar conversa %d como humano_aguardando: %v", convID, err)
		} else {
			publishWebhook(ctx, p.st, webhooks.EventAgentTransferred, "agent.transferred:"+strconv.Itoa(convID)+":"+strconv.FormatInt(time.Now().UnixNano(), 10), map[string]interface{}{
				"conversation": store.ConversationWebhookPayload(conv),
				"from_status":  conv.Status,
				"to_status":    "humano_aguardando",
				"reason":       "transferencia_solicitada",
			})
			out.Status = "humano_aguardando"
			if agentErr == nil {
				go p.generateHandoffSummary(store.WithTenantID(context.Background(), in.TenantID), convID, agentObj)
			}
		}
	}
	if reply.SetorID != nil {
		if err := p.st.SetConversationSetor(ctx, convID, reply.SetorID); err != nil {
			Logger.Printf("erro ao definir setor %d na conversa %d: %v", *reply.SetorID, convID, err)
		}
		if setor, err := p.st.GetSetor(ctx, *reply.SetorID); err == nil {
			out.SetorName = setor.Name
		}
	}

	// Aprendizado: só registra quando a IA falhou de verdade (fallback) ou não
	// produziu resposta. Transferência pedida pelo cliente NÃO entra aqui para
	// não poluir a fila de curadoria com casos que a IA resolveu por política.
	if registrarFalhas && (reply.RecordLearning || reply.Text == "") {
		_ = p.st.AddAprendizado(ctx, in.Message, in.Channel)
	}

	out.UsedTools = reply.UsedTools
	if reply.Rich != nil {
		out.Rich = reply.Rich
		out.Fallback = reply.Fallback
		out.Reply = reply.Fallback
	} else {
		out.Reply = reply.Text
	}
	return out, nil
}

func (p *Processor) findOrCreateConversation(ctx context.Context, in Inbound, agentID *int) (int, bool, error) {
	if in.Channel == "whatsapp" && !whatsappConversationPhoneValid(in.Phone) {
		Logger.Printf("conversa ignorada: telefone de WhatsApp inválido %q", in.Phone)
		return 0, false, errConversationPhoneInvalid
	}
	if in.Phone != "" {
		if in.TenantID > 0 && in.ChannelID != nil {
			conversation := store.Conversation{
				AgentID:       agentID,
				ChannelID:     in.ChannelID,
				Channel:       in.Channel,
				CustomerName:  in.Name,
				CustomerPhone: in.Phone,
				Status:        "ia",
				Origem:        in.Origem,
			}
			conversationID, reused, err := p.st.FindOrCreateConversationForChannel(ctx, conversation)
			if err != nil {
				return 0, false, err
			}
			if !reused {
				_ = p.addMessage(ctx, in.TenantID, store.Message{
					ConversationID: conversationID, Role: "system", Source: "system",
					Content: "🔖 Protocolo de atendimento " + ticketLabel(conversationID) + " aberto",
				})
			}
			return conversationID, !reused, nil
		} else {
			convs, err := p.st.ListConversations(ctx)
			if err != nil {
				return 0, false, err
			}
			for _, c := range convs {
				// reusa a conversa do mesmo canal+telefone que ainda está aberta
				// OU encerrada mas aguardando satisfação. Não depende do agente
				// (evita criar conversa duplicada em modo "ia" após reinício).
				aguardandoSatisf := c.Satisfacao == "aguardando" || c.Satisfacao == "aguardando_operador"
				reusable := c.Status != "closed" || (c.Status == "closed" && aguardandoSatisf)
				sameChannel := inboundMatchesConversation(c, in)
				if c.CustomerPhone == in.Phone && sameChannel && reusable {
					if strings.TrimSpace(in.Name) != "" || strings.TrimSpace(in.Origem) != "" {
						_ = p.st.UpdateConversationIdentity(ctx, c.ID, in.Name, in.Origem)
					}
					return c.ID, false, nil
				}
			}
		}
	}
	conversation := store.Conversation{
		AgentID:       agentID,
		ChannelID:     in.ChannelID,
		Channel:       in.Channel,
		CustomerName:  in.Name,
		CustomerPhone: in.Phone,
		Status:        "ia",
		Origem:        in.Origem,
	}
	var createdID int
	var err error
	if in.TenantID > 0 && in.ChannelID != nil {
		createdID, err = p.st.CreateConversationForChannel(ctx, conversation)
	} else {
		createdID, err = p.st.CreateConversation(ctx, conversation)
	}
	if err == nil {
		_ = p.addMessage(ctx, in.TenantID, store.Message{
			ConversationID: createdID, Role: "system", Source: "system",
			Content: "🔖 Protocolo de atendimento " + ticketLabel(createdID) + " aberto",
		})
	}
	return createdID, err == nil, err
}

func inboundMatchesConversation(c store.Conversation, in Inbound) bool {
	if c.Channel != in.Channel {
		return false
	}
	if in.ChannelID == nil {
		// Compatibilidade com registros antigos que ainda não têm o vínculo
		// explícito com o canal.
		return true
	}
	return c.ChannelID != nil && *c.ChannelID == *in.ChannelID
}

// ticketLabel formata o ID de atendimento.
func ticketLabel(id int) string {
	return fmt.Sprintf("#T-%05d", id)
}

func (p *Processor) fallbackReply(ctx context.Context) agent.Reply {
	return agent.Reply{Text: "Não consegui concluir essa resposta agora. Pode repetir sua última mensagem? Vou continuar o atendimento por aqui. 😊", RecordLearning: true}
}

func (p *Processor) noAgentReply() agent.Reply {
	// Sem agente/IA configurada não há o que responder ao cliente. A mensagem
	// interna fica no painel; aqui não vai texto para o cliente.
	return agent.Reply{RecordLearning: true}
}

func (p *Processor) recordFailure(ctx context.Context, failure store.AgentFailure) {
	if p.st == nil {
		return
	}
	if err := p.st.RecordAgentFailure(ctx, failure); err != nil {
		Logger.Printf("audit de falha não registrado: %v", err)
	}
}

func (p *Processor) generateHandoffSummary(ctx context.Context, convID int, a store.Agent) {
	if p.summarize == nil || p.st == nil {
		return
	}
	summary, err := p.summarize(ctx, convID, a)
	if err != nil {
		Logger.Printf("resumo da IA não gerado para a conversa %d: %v", convID, err)
		return
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return
	}
	if err := p.st.SetConversationAISummary(ctx, convID, summary); err != nil {
		Logger.Printf("resumo da IA não persistido para a conversa %d: %v", convID, err)
	}
}

func (p *Processor) resolveAgent(ctx context.Context, conv store.Conversation) (store.Agent, error) {
	tenantID := store.TenantID(ctx)
	if conv.AgentID != nil {
		if a, err := p.st.GetAgentForTenant(ctx, tenantID, *conv.AgentID); err == nil && a.Active {
			return a, nil
		}
	}
	if conv.ChannelID != nil {
		if channelAgentID := p.channelAgentID(ctx, *conv.ChannelID); channelAgentID != nil {
			if a, err := p.st.GetAgentForTenant(ctx, tenantID, *channelAgentID); err == nil && a.Active {
				return a, nil
			}
		}
	}
	return store.Agent{}, fmt.Errorf("nenhum agente ativo configurado para o canal %s", conv.Channel)
}

// channelAgentID devolve o agente vinculado explicitamente ao canal
// (coluna channels.agent_id), a única fonte de verdade do vínculo.
func (p *Processor) channelAgentID(ctx context.Context, channelID int) *int {
	if channelID <= 0 {
		return nil
	}
	channel, err := p.st.GetChannelByID(ctx, channelID)
	if err != nil {
		return nil
	}
	return channel.AgentID
}

// chatflowTarget devolve o modo de atendimento do canal (channels.mode) e o
// fluxo vinculado (channels.chatflow_id). Sem canal, cai em IA livre.
func (p *Processor) chatflowTarget(ctx context.Context, channelID *int) agent.ChatflowTarget {
	target := agent.ChatflowTarget{Mode: "ai"}
	if channelID == nil || *channelID <= 0 {
		return target
	}
	channel, err := p.st.GetChannelByID(ctx, *channelID)
	if err != nil {
		return target
	}
	target.Mode = channel.Mode
	target.ChatflowID = channel.ChatflowID
	return target
}

func (p *Processor) messageExists(ctx context.Context, in Inbound) (bool, error) {
	if in.TenantID > 0 {
		if in.ChannelID != nil {
			return p.st.MessageExistsWAMsgIDForChannel(ctx, in.TenantID, *in.ChannelID, in.WAMsgID)
		}
		return p.st.MessageExistsWAMsgIDForTenant(ctx, in.TenantID, in.WAMsgID)
	}
	return p.st.MessageExistsWAMsgID(ctx, in.WAMsgID)
}

func (p *Processor) addMessage(ctx context.Context, tenantID int, message store.Message) error {
	if tenantID > 0 {
		return p.st.AddMessageForTenant(ctx, tenantID, message)
	}
	return p.st.AddMessage(ctx, message)
}

func agentMatches(a, b *int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func satisfactionTarget(hasAI, hasHuman bool) string {
	if hasAI || !hasHuman {
		return "ia"
	}
	return "operador"
}

func BeginSatisfaction(ctx context.Context, st *store.Store, conversationID int) (string, error) {
	hasAI, hasHuman, err := st.SatisfactionParticipants(ctx, conversationID)
	if err != nil {
		return "", err
	}
	text := "Atendimento encerrado. Quando precisar, é só iniciar um novo atendimento. 😊"
	state := "finalizada"
	if hasAI || hasHuman {
		text = SatisfactionQuestion(ctx, st, satisfactionTarget(hasAI, hasHuman))
		state = "aguardando"
	}
	if err := st.SetConversationSatisfacao(ctx, conversationID, state); err != nil {
		return "", err
	}
	if err := st.AddMessageForTenant(ctx, store.TenantID(ctx), store.Message{ConversationID: conversationID, Role: "assistant", Source: "system", Content: text}); err != nil {
		return "", err
	}
	return text, nil
}

// SatisfactionRich monta a pergunta de satisfação como botões (1 a 5) para o
// WhatsApp. O botão tocado envia o texto do número, que o processador lê como
// nota; o fallback é o próprio texto (caso o canal não suporte botões ou a API
// falhe).
func SatisfactionRich(text string) *agent.RichMessage {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	rich := &agent.RichMessage{Kind: "buttons", Text: text, Buttons: []agent.OptionButton{
		{Type: "reply", Title: "1", ID: "1"},
		{Type: "reply", Title: "2", ID: "2"},
		{Type: "reply", Title: "3", ID: "3"},
		{Type: "reply", Title: "4", ID: "4"},
		{Type: "reply", Title: "5", ID: "5"},
	}}
	rich.Fallback = text
	return rich
}

func isEndConversationRequest(text string) bool {
	normalized := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(text)), " "))
	normalized = strings.Trim(normalized, " .,!?:;…")
	switch normalized {
	case "quero encerrar", "quero encerrar o atendimento", "quero finalizar", "quero finalizar o atendimento",
		"encerrar atendimento", "encerrar o atendimento", "finalizar atendimento", "finalizar o atendimento",
		"pode encerrar", "pode encerrar o atendimento", "pode finalizar", "pode finalizar o atendimento",
		"encerrar conversa", "encerrar a conversa", "finalizar conversa", "finalizar a conversa",
		"pode encerrar a conversa", "pode finalizar a conversa":
		return true
	default:
		return false
	}
}

func SatisfactionQuestion(ctx context.Context, st *store.Store, target string) string {
	key := "satisfacao_ia"
	text := "De 1 a 5, como você avalia o atendimento da nossa assistente virtual? Responda apenas com um número (1 = péssimo, 5 = excelente). 😊"
	if target == "operador" {
		key = "satisfacao_operador"
		text = "De 1 a 5, como você avalia o atendimento do nosso atendente? Responda apenas com um número (1 = péssimo, 5 = excelente). 😊"
	}
	if message, err := st.GetMensagemByChave(ctx, key); err == nil && strings.TrimSpace(message.Texto) != "" {
		return strings.TrimSpace(message.Texto)
	}
	return text
}

func satisfactionRetry(target string) string {
	name := "assistente virtual"
	if target == "operador" {
		name = "atendente"
	}
	return "Não consegui identificar a nota do " + name + ". Por favor, responda apenas com um número de 1 a 5."
}

func lowerFirst(text string) string {
	if text == "" {
		return ""
	}
	runes := []rune(text)
	runes[0] = []rune(strings.ToLower(string(runes[0])))[0]
	return string(runes)
}

// botDetected decide se a mensagem de entrada vem de um provável "outro bot"
// (automação), para segurança. Só age quando a configuração `detectar_bots` está
// ligada (opt-in). Detecta por padrão de texto (bot_patterns editável) e por
// rajada de mensagens curtas. Nunca muda o comportamento quando desligado.
func (p *Processor) botDetected(ctx context.Context, in Inbound, convID int) bool {
	settings, err := p.st.GetSettings(ctx)
	if err != nil || !settings.DetectarBots {
		return false
	}
	if botPatternMatch(in.Message, botPatternsOrDefault(settings.BotPatterns)) {
		Logger.Printf("monitor de bot [tenant %d]: padrão de bot detectado na mensagem de %q", store.TenantID(ctx), in.Phone)
		return true
	}
	if count, err := p.st.CountRecentMessagesByRole(ctx, convID, "user", 60); err == nil && count >= botBurstThreshold {
		Logger.Printf("monitor de bot [tenant %d]: rajada de mensagens detectada (%d em 60s) de %q", store.TenantID(ctx), count, in.Phone)
		return true
	}
	return false
}

// blockBot marca a conversa como provável outro bot e a ignora (não responde e
// não consome a fila humana), deixando um aviso no histórico para a equipe.
// Um atendente pode desbloquear no Inbox, voltando a conversa ao fluxo normal.
func (p *Processor) blockBot(ctx context.Context, convID int) {
	_ = p.st.SetConversationBotBlocked(ctx, convID, true)
	_ = p.st.AddMessage(ctx, store.Message{ConversationID: convID, Role: "system", Source: "system", Content: "⚠️ Possível bot detectado — conversa marcada como bloqueada. Desbloqueie no Inbox se for um cliente real."})
}

// botPatterns normaliza a lista editável de padrões (uma por linha ou vírgula).
func botPatterns(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		for _, part := range strings.Split(line, ",") {
			p := strings.ToLower(strings.TrimSpace(part))
			if p == "" || seen[p] {
				continue
			}
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// botPatternsOrDefault usa a lista editável; se vazia, aplica um conjunto
// padrão conservador (marcadores típicos de automação) para a detecção já
// funcionar "de cara" quando ligada, sem exigir configurar padrões.
func botPatternsOrDefault(raw string) []string {
	if patterns := botPatterns(raw); len(patterns) > 0 {
		return patterns
	}
	return strings.Split(defaultBotPatterns, ",")
}

// defaultBotPatterns são marcadores comuns de automação; editável no painel.
const defaultBotPatterns = "test,teste,bot,robô,robo,automação,automatizado,newsletter,unsubscribe,@g.us,broadcast"

// botPatternMatch devolve true se a mensagem contém algum padrão de bot.
func botPatternMatch(message string, patterns []string) bool {
	message = strings.ToLower(message)
	for _, pattern := range patterns {
		if strings.Contains(message, pattern) {
			return true
		}
	}
	return false
}

// botBurstThreshold é o número de mensagens em 60s que indica rajada (bot).
const botBurstThreshold = 6

// publishWebhook enfileira um evento de domínio na outbox de forma best-effort.
// Um erro de publicação é registrado em log e nunca interrompe o atendimento.
func publishWebhook(ctx context.Context, st *store.Store, event webhooks.Event, key string, payload interface{}) {
	if err := st.PublishOutboxEvent(ctx, event, key, payload); err != nil {
		Logger.Printf("webhook outbox: evento %s não publicado: %v", event, err)
	}
}

// receivedMessageKey é a chave de idempotência de um evento message.received.
// Usa o WAMsgID (estável por mensagem) quando disponível; senão, um valor único.
func receivedMessageKey(waMsgID string) string {
	if strings.TrimSpace(waMsgID) != "" {
		return waMsgID
	}
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}
