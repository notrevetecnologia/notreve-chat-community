package web

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/agendamento"
	"github.com/notrevetecnologia/notreve-chat-community/internal/agent"
	"github.com/notrevetecnologia/notreve-chat-community/internal/channel"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

type agendamentoRepo struct {
	st *store.Store
}

func (r agendamentoRepo) ListDue(ctx context.Context, now time.Time, limit int) ([]store.Agendamento, error) {
	return r.st.ListDueAgendamentosForTenant(ctx, now, limit)
}

func (r agendamentoRepo) Claim(ctx context.Context, id int, now time.Time) (bool, error) {
	return r.st.ClaimAgendamentoForTenant(ctx, id, now)
}

func (r agendamentoRepo) Finish(ctx context.Context, id int, outcome agendamento.Outcome, now time.Time) error {
	return r.st.SetAgendamentoStatusForTenant(ctx, id, outcome.Status, outcome.Resultado, outcome.Erro, outcome.Execucoes, now, outcome.Proxima)
}

// agendamentosJob dispara os agendamentos vencidos do tenant do contexto.
func (s *Server) agendamentosJob(ctx context.Context) {
	if s.scheduler == nil {
		return
	}
	s.scheduler.RunOnce(ctx)
}

// reagendarAgendamentos recalcula a próxima execução de pendentes sem data
// (registros legados ou criados antes do primeiro tick) no boot do app.
func (s *Server) reagendarAgendamentos(ctx context.Context) {
	if _, err := s.st.RequeueStaleExecutandoForTenant(ctx, time.Now().Add(-10*time.Minute)); err != nil {
		Logger.Printf("agendamentos: recuperando execuções interrompidas: %v", err)
	}
	pending, err := s.st.ListAgendamentosSemProximaForTenant(ctx, 200)
	if err != nil {
		Logger.Printf("agendamentos: listando para reagendar: %v", err)
		return
	}
	for _, task := range pending {
		next, err := agendamento.NextRun(task.Tipo, task.Quando, time.Now())
		if err != nil {
			_ = s.st.SetAgendamentoStatusForTenant(ctx, task.ID, agendamento.StatusFalhou, task.Resultado, "falha ao reagendar: "+err.Error(), task.Execucoes, time.Now(), nil)
			continue
		}
		_ = s.st.SetAgendamentoProximaForTenant(ctx, task.ID, next)
	}
}

func agendamentoPayloadString(payload map[string]interface{}, key string) string {
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", value))
}

func agendamentoTruncate(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func (s *Server) agendamentoEnviarMensagem(ctx context.Context, a store.Agendamento, payload map[string]interface{}) (string, error) {
	texto := agendamentoPayloadString(payload, "texto")
	if texto == "" {
		return "", errors.New("payload.texto ausente")
	}
	if a.CanalID == nil {
		return "", errors.New("canal_id é obrigatório para enviar mensagem")
	}
	if strings.TrimSpace(a.Telefone) == "" {
		return "", errors.New("telefone do destinatário ausente")
	}
	ch, err := s.st.GetChannelByIDSecure(ctx, *a.CanalID)
	if err != nil {
		return "", err
	}
	adapter := s.channels.For(ch.Type)
	if adapter == nil {
		return "", fmt.Errorf("canal %q sem adaptador de saída", ch.Type)
	}
	if err := adapter.Send(ctx, ch, a.Telefone, channel.Outbound{Text: texto}); err != nil {
		return "", err
	}
	s.persistScheduledOutbound(ctx, a, ch.Type, texto)
	return fmt.Sprintf("mensagem enviada para %s", a.Telefone), nil
}

func (s *Server) persistScheduledOutbound(ctx context.Context, a store.Agendamento, channelType, texto string) {
	if a.CanalID == nil || strings.TrimSpace(a.Telefone) == "" {
		return
	}
	convID, _, err := s.st.FindOrCreateConversationForChannel(ctx, store.Conversation{
		AgentID: a.AgenteID, ChannelID: a.CanalID, Channel: channelType,
		CustomerPhone: a.Telefone, Status: "ia",
	})
	if err != nil {
		return
	}
	_ = s.st.AddMessage(ctx, store.Message{ConversationID: convID, Role: "assistant", Source: "system", Content: texto})
}

func (s *Server) agendamentoRodarFerramenta(ctx context.Context, a store.Agendamento, payload map[string]interface{}) (string, error) {
	nome := agendamentoPayloadString(payload, "nome_ferramenta")
	if nome == "" {
		return "", errors.New("payload.nome_ferramenta ausente")
	}
	args := map[string]interface{}{}
	if raw, ok := payload["argumentos"].(map[string]interface{}); ok {
		args = raw
	}
	result, err := s.eng.RunScheduledTool(ctx, nome, args, a.Telefone)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("ferramenta %s: %s", nome, agendamentoTruncate(result, 500)), nil
}

func (s *Server) chatflowTarget(ctx context.Context, channelID *int) agent.ChatflowTarget {
	target := agent.ChatflowTarget{Mode: "ai"}
	if channelID == nil || *channelID <= 0 {
		return target
	}
	channel, err := s.st.GetChannelByID(ctx, *channelID)
	if err != nil {
		return target
	}
	target.Mode = channel.Mode
	target.ChatflowID = channel.ChatflowID
	return target
}

func (s *Server) agendamentoCallbackAgente(ctx context.Context, a store.Agendamento, payload map[string]interface{}) (string, error) {
	prompt := agendamentoPayloadString(payload, "prompt")
	if prompt == "" {
		return "", errors.New("payload.prompt ausente")
	}
	if strings.TrimSpace(a.Telefone) == "" {
		return "", errors.New("telefone do destinatário ausente")
	}
	convID, conv, err := s.ensureAgendamentoConversation(ctx, a)
	if err != nil {
		return "", err
	}
	agentObj, err := s.agentForScheduledConversation(ctx, a, conv)
	if err != nil {
		return "", err
	}
	if err := s.st.AddMessage(ctx, store.Message{ConversationID: convID, Role: "user", Source: "system", Content: "[agendamento] " + prompt}); err != nil {
		return "", err
	}
	reply, err := s.eng.HandleUserMessage(ctx, convID, agentObj, conv.Channel, s.chatflowTarget(ctx, conv.ChannelID), agent.Media{})
	if err != nil {
		return "", err
	}
	text := strings.TrimSpace(reply.Text)
	if text == "" && reply.Rich != nil {
		text = reply.Rich.Fallback
	}
	if text == "" {
		return "agente não gerou resposta", nil
	}
	if err := s.sendConversationText(ctx, conv, text, ""); err != nil {
		return "", err
	}
	_ = s.st.AddMessage(ctx, store.Message{ConversationID: convID, Role: "assistant", Source: "ai", Content: text})
	return "resposta enviada: " + agendamentoTruncate(text, 200), nil
}

func (s *Server) ensureAgendamentoConversation(ctx context.Context, a store.Agendamento) (int, store.Conversation, error) {
	if a.CanalID != nil {
		ch, err := s.st.GetChannelByIDSecure(ctx, *a.CanalID)
		if err != nil {
			return 0, store.Conversation{}, err
		}
		convID, _, err := s.st.FindOrCreateConversationForChannel(ctx, store.Conversation{
			AgentID: a.AgenteID, ChannelID: a.CanalID, Channel: ch.Type,
			CustomerPhone: a.Telefone, Status: "ia",
		})
		if err != nil {
			return 0, store.Conversation{}, err
		}
		conv, err := s.st.GetConversation(ctx, convID)
		return convID, conv, err
	}
	conv, err := s.st.GetOpenConversationByPhone(ctx, a.Telefone)
	if err != nil {
		return 0, store.Conversation{}, err
	}
	return conv.ID, conv, nil
}

func (s *Server) agentForScheduledConversation(ctx context.Context, a store.Agendamento, conv store.Conversation) (store.Agent, error) {
	if a.AgenteID != nil {
		if agentObj, err := s.st.GetAgent(ctx, *a.AgenteID); err == nil {
			return agentObj, nil
		}
	}
	if conv.AgentID != nil {
		if agentObj, err := s.st.GetAgent(ctx, *conv.AgentID); err == nil {
			return agentObj, nil
		}
	}
	return s.agentForConv(ctx, conv)
}
