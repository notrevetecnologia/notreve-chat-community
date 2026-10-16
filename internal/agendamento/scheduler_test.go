package agendamento

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func task(acao string, payload string) store.Agendamento {
	return store.Agendamento{
		ID: 1, Nome: "teste", Tipo: TipoOnce, Quando: "2030-01-01T00:00:00",
		Acao: acao, Payload: json.RawMessage(payload), Status: StatusPendente,
	}
}

func TestRunOneEnviarMensagem(t *testing.T) {
	called := false
	handlers := ActionHandlers{EnviarMensagem: func(_ context.Context, _ store.Agendamento, payload map[string]interface{}) (string, error) {
		called = true
		if payload["texto"] != "oi" {
			t.Fatalf("payload texto = %v", payload["texto"])
		}
		return "enviado", nil
	}}
	outcome := RunOne(context.Background(), task(AcaoEnviarMensagem, `{"texto":"oi"}`), handlers, time.Now())
	if !called {
		t.Fatal("handler de enviar_mensagem não chamado")
	}
	if outcome.Status != StatusConcluida {
		t.Fatalf("status = %q, esperado concluida", outcome.Status)
	}
	if outcome.Resultado != "enviado" {
		t.Fatalf("resultado = %q", outcome.Resultado)
	}
}

func TestRunOneRodarFerramenta(t *testing.T) {
	handlers := ActionHandlers{RodarFerramenta: func(_ context.Context, _ store.Agendamento, payload map[string]interface{}) (string, error) {
		return "ferramenta " + payload["nome_ferramenta"].(string), nil
	}}
	outcome := RunOne(context.Background(), task(AcaoRodarFerramenta, `{"nome_ferramenta":"consultar"}`), handlers, time.Now())
	if outcome.Status != StatusConcluida || outcome.Resultado != "ferramenta consultar" {
		t.Fatalf("outcome inesperado: %+v", outcome)
	}
}

func TestRunOneCallbackAgente(t *testing.T) {
	handlers := ActionHandlers{CallbackAgente: func(_ context.Context, _ store.Agendamento, payload map[string]interface{}) (string, error) {
		return "agente " + payload["prompt"].(string), nil
	}}
	outcome := RunOne(context.Background(), task(AcaoCallbackAgente, `{"prompt":"follow-up"}`), handlers, time.Now())
	if outcome.Status != StatusConcluida || outcome.Resultado != "agente follow-up" {
		t.Fatalf("outcome inesperado: %+v", outcome)
	}
}

func TestRunOneIntervalReschedules(t *testing.T) {
	handlers := ActionHandlers{EnviarMensagem: func(context.Context, store.Agendamento, map[string]interface{}) (string, error) {
		return "ok", nil
	}}
	a := task(AcaoEnviarMensagem, `{"texto":"x"}`)
	a.Tipo = TipoInterval
	a.Quando = "60"
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	outcome := RunOne(context.Background(), a, handlers, now)
	if outcome.Status != StatusPendente {
		t.Fatalf("status = %q, esperado pendente", outcome.Status)
	}
	if outcome.Proxima == nil || !outcome.Proxima.Equal(now.Add(time.Minute)) {
		t.Fatalf("próxima = %v", outcome.Proxima)
	}
	if outcome.Execucoes != 1 {
		t.Fatalf("execuções = %d, esperado 1", outcome.Execucoes)
	}
}

func TestRunOneMaxExecucoes(t *testing.T) {
	handlers := ActionHandlers{EnviarMensagem: func(context.Context, store.Agendamento, map[string]interface{}) (string, error) {
		return "ok", nil
	}}
	max := 1
	a := task(AcaoEnviarMensagem, `{"texto":"x"}`)
	a.Tipo = TipoInterval
	a.Quando = "60"
	a.MaxExecucoes = &max
	outcome := RunOne(context.Background(), a, handlers, time.Now())
	if outcome.Status != StatusConcluida {
		t.Fatalf("status = %q, esperado concluida ao atingir max", outcome.Status)
	}
	if outcome.Proxima != nil {
		t.Fatal("tarefa concluída não deveria ter próxima execução")
	}
}

func TestRunOneErrorMarksFalhou(t *testing.T) {
	handlers := ActionHandlers{EnviarMensagem: func(context.Context, store.Agendamento, map[string]interface{}) (string, error) {
		return "", errors.New("canal fora do ar")
	}}
	outcome := RunOne(context.Background(), task(AcaoEnviarMensagem, `{"texto":"x"}`), handlers, time.Now())
	if outcome.Status != StatusFalhou {
		t.Fatalf("status = %q, esperado falhou", outcome.Status)
	}
	if outcome.Erro != "canal fora do ar" {
		t.Fatalf("erro = %q", outcome.Erro)
	}
	if outcome.Proxima != nil {
		t.Fatal("tarefa falha não deveria ter próxima execução")
	}
}

func TestRunOnePanicIsBestEffort(t *testing.T) {
	handlers := ActionHandlers{EnviarMensagem: func(context.Context, store.Agendamento, map[string]interface{}) (string, error) {
		panic("boom")
	}}
	outcome := RunOne(context.Background(), task(AcaoEnviarMensagem, `{"texto":"x"}`), handlers, time.Now())
	if outcome.Status != StatusFalhou {
		t.Fatalf("status = %q, esperado falhou após panic", outcome.Status)
	}
}

func TestRunOneMissingHandler(t *testing.T) {
	outcome := RunOne(context.Background(), task(AcaoCallbackAgente, `{"prompt":"x"}`), ActionHandlers{}, time.Now())
	if outcome.Status != StatusFalhou {
		t.Fatalf("status = %q, esperado falhou sem handler", outcome.Status)
	}
}

type fakeRepo struct {
	due       []store.Agendamento
	listErr   error
	claimErr  error
	finishErr error
	claimed   map[int]bool
	finished  map[int]Outcome
}

func (r *fakeRepo) ListDue(context.Context, time.Time, int) ([]store.Agendamento, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.due, nil
}

func (r *fakeRepo) Claim(_ context.Context, id int, _ time.Time) (bool, error) {
	if r.claimErr != nil {
		return false, r.claimErr
	}
	if r.claimed == nil {
		r.claimed = map[int]bool{}
	}
	r.claimed[id] = true
	return true, nil
}

func (r *fakeRepo) Finish(_ context.Context, id int, outcome Outcome, _ time.Time) error {
	if r.finishErr != nil {
		return r.finishErr
	}
	if r.finished == nil {
		r.finished = map[int]Outcome{}
	}
	r.finished[id] = outcome
	return nil
}

func TestSchedulerRunOncePersistsOutcome(t *testing.T) {
	repo := &fakeRepo{due: []store.Agendamento{task(AcaoEnviarMensagem, `{"texto":"oi"}`)}}
	handlers := ActionHandlers{EnviarMensagem: func(context.Context, store.Agendamento, map[string]interface{}) (string, error) {
		return "ok", nil
	}}
	logs := 0
	scheduler := NewScheduler(repo, handlers).WithLogger(func(string, ...interface{}) { logs++ })
	scheduler.RunOnce(context.Background())
	if len(repo.finished) != 1 {
		t.Fatalf("esperava 1 finalização, obtido %d", len(repo.finished))
	}
	if repo.finished[1].Status != StatusConcluida {
		t.Fatalf("status persistido = %q", repo.finished[1].Status)
	}
	if logs != 0 {
		t.Fatalf("não deveria registrar log de erro, logs=%d", logs)
	}
}

func TestSchedulerRunOnceListErrorIsBestEffort(t *testing.T) {
	repo := &fakeRepo{listErr: errors.New("banco fora")}
	logs := 0
	scheduler := NewScheduler(repo, ActionHandlers{}).WithLogger(func(string, ...interface{}) { logs++ })
	scheduler.RunOnce(context.Background())
	if logs == 0 {
		t.Fatal("erro de listagem deveria ser registrado em log")
	}
}

func TestSchedulerRunOnceFinishErrorIsBestEffort(t *testing.T) {
	repo := &fakeRepo{
		due:       []store.Agendamento{task(AcaoEnviarMensagem, `{"texto":"oi"}`)},
		finishErr: errors.New("falha ao gravar"),
	}
	handlers := ActionHandlers{EnviarMensagem: func(context.Context, store.Agendamento, map[string]interface{}) (string, error) {
		return "ok", nil
	}}
	logs := 0
	scheduler := NewScheduler(repo, handlers).WithLogger(func(string, ...interface{}) { logs++ })
	scheduler.RunOnce(context.Background())
	if logs == 0 {
		t.Fatal("falha ao persistir deveria ser registrada em log")
	}
}
