package agendamento

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

// ActionHandlers implementa cada ação disparada por um agendamento. São
// injetadas para manter o scheduler desacoplado dos canais, das tools e do
// motor de IA.
type ActionHandlers struct {
	EnviarMensagem  func(ctx context.Context, a store.Agendamento, payload map[string]interface{}) (string, error)
	RodarFerramenta func(ctx context.Context, a store.Agendamento, payload map[string]interface{}) (string, error)
	CallbackAgente  func(ctx context.Context, a store.Agendamento, payload map[string]interface{}) (string, error)
}

type Outcome struct {
	Status    string
	Resultado string
	Erro      string
	Execucoes int
	Proxima   *time.Time
}

func DecodePayload(raw []byte) map[string]interface{} {
	payload := map[string]interface{}{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &payload)
	}
	if payload == nil {
		payload = map[string]interface{}{}
	}
	return payload
}

func truncate(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

// RunOne executa a ação e decide o próximo estado sem tocar no banco. Erros e
// panics são convertidos em status `falhou` — o agendador nunca é derrubado.
func RunOne(ctx context.Context, a store.Agendamento, handlers ActionHandlers, now time.Time) (outcome Outcome) {
	if now.IsZero() {
		now = time.Now()
	}
	outcome.Execucoes = a.Execucoes + 1
	defer func() {
		if recovered := recover(); recovered != nil {
			outcome.Status = StatusFalhou
			outcome.Erro = truncate(fmt.Sprintf("panic: %v", recovered), 2000)
			outcome.Proxima = nil
		}
	}()
	resultado, err := dispatch(ctx, a, DecodePayload(a.Payload), handlers)
	if err != nil {
		outcome.Status = StatusFalhou
		outcome.Erro = truncate(err.Error(), 2000)
		outcome.Proxima = nil
		return outcome
	}
	outcome.Resultado = truncate(resultado, 5000)
	outcome.Erro = ""
	if NormalizeTipo(a.Tipo) == TipoOnce {
		outcome.Status = StatusConcluida
		return outcome
	}
	if a.MaxExecucoes != nil && outcome.Execucoes >= *a.MaxExecucoes {
		outcome.Status = StatusConcluida
		return outcome
	}
	next, err := NextRun(a.Tipo, a.Quando, now)
	if err != nil {
		outcome.Status = StatusFalhou
		outcome.Erro = truncate(err.Error(), 2000)
		outcome.Proxima = nil
		return outcome
	}
	outcome.Status = StatusPendente
	outcome.Proxima = &next
	return outcome
}

func dispatch(ctx context.Context, a store.Agendamento, payload map[string]interface{}, handlers ActionHandlers) (string, error) {
	switch NormalizeAcao(a.Acao) {
	case AcaoEnviarMensagem:
		if handlers.EnviarMensagem == nil {
			return "", errors.New("ação enviar_mensagem indisponível")
		}
		return handlers.EnviarMensagem(ctx, a, payload)
	case AcaoRodarFerramenta:
		if handlers.RodarFerramenta == nil {
			return "", errors.New("ação rodar_ferramenta indisponível")
		}
		return handlers.RodarFerramenta(ctx, a, payload)
	case AcaoCallbackAgente:
		if handlers.CallbackAgente == nil {
			return "", errors.New("ação callback_agente indisponível")
		}
		return handlers.CallbackAgente(ctx, a, payload)
	}
	return "", fmt.Errorf("ação não suportada: %s", a.Acao)
}

type Repository interface {
	ListDue(ctx context.Context, now time.Time, limit int) ([]store.Agendamento, error)
	Claim(ctx context.Context, id int, now time.Time) (bool, error)
	Finish(ctx context.Context, id int, outcome Outcome, now time.Time) error
}

type Scheduler struct {
	repo     Repository
	handlers ActionHandlers
	now      func() time.Time
	logf     func(format string, args ...interface{})
	limit    int
}

func NewScheduler(repo Repository, handlers ActionHandlers) *Scheduler {
	return &Scheduler{repo: repo, handlers: handlers, now: time.Now, limit: 25}
}

func (s *Scheduler) WithLogger(logf func(format string, args ...interface{})) *Scheduler {
	s.logf = logf
	return s
}

func (s *Scheduler) WithClock(now func() time.Time) *Scheduler {
	if now != nil {
		s.now = now
	}
	return s
}

// RunOnce processa os agendamentos vencidos do tenant do contexto. Toda falha é
// registrada em log; a função nunca devolve erro nem interrompe o processo.
func (s *Scheduler) RunOnce(ctx context.Context) {
	now := s.now()
	due, err := s.repo.ListDue(ctx, now, s.limit)
	if err != nil {
		s.log("agendamentos: listando pendentes: %v", err)
		return
	}
	for _, task := range due {
		claimed, err := s.repo.Claim(ctx, task.ID, now)
		if err != nil {
			s.log("agendamento %d: reservando execução: %v", task.ID, err)
			continue
		}
		if !claimed {
			continue
		}
		outcome := RunOne(ctx, task, s.handlers, now)
		if err := s.repo.Finish(ctx, task.ID, outcome, now); err != nil {
			s.log("agendamento %d: persistindo resultado: %v", task.ID, err)
		}
	}
}

func (s *Scheduler) log(format string, args ...interface{}) {
	if s.logf != nil {
		s.logf(format, args...)
	}
}
