package agendamento

import (
	"testing"
	"time"
)

func TestNextRunOnce(t *testing.T) {
	from := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	next, err := NextRun(TipoOnce, "2030-01-02T15:04:05", from)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	want := time.Date(2030, 1, 2, 18, 4, 5, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("once = %s, esperado %s", next, want)
	}
}

func TestNextRunOncePast(t *testing.T) {
	from := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	next, err := NextRun(TipoOnce, "2020-01-02 15:04:05", from)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !next.Before(from) {
		t.Fatalf("once no passado deveria ser anterior a from: %s", next)
	}
}

func TestNextRunInterval(t *testing.T) {
	from := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	next, err := NextRun(TipoInterval, "300", from)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got := next.Sub(from); got != 300*time.Second {
		t.Fatalf("intervalo = %s, esperado 5m", got)
	}
	if _, err := NextRun(TipoInterval, "5", from); err == nil {
		t.Fatal("intervalo abaixo do mínimo deveria falhar")
	}
	if _, err := NextRun(TipoInterval, "abc", from); err == nil {
		t.Fatal("intervalo não numérico deveria falhar")
	}
}

func TestNextRunCronDaily(t *testing.T) {
	from := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	next, err := NextRun(TipoCron, "0 9 * * *", from)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	want := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("cron = %s, esperado %s", next, want)
	}
}

func TestNextRunCronStep(t *testing.T) {
	from := time.Date(2026, 1, 1, 12, 7, 0, 0, time.UTC)
	next, err := NextRun(TipoCron, "*/15 * * * *", from)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	want := time.Date(2026, 1, 1, 12, 15, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("cron step = %s, esperado %s", next, want)
	}
}

func TestNextRunCronWeekday(t *testing.T) {
	from := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	next, err := NextRun(TipoCron, "30 8 * * 1", from)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if next.Weekday() != time.Monday {
		t.Fatalf("cron deveria cair numa segunda, veio %s", next.Weekday())
	}
}

func TestNextRunInvalid(t *testing.T) {
	from := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	if _, err := NextRun(TipoCron, "0 9 * *", from); err == nil {
		t.Fatal("cron com poucos campos deveria falhar")
	}
	if _, err := NextRun(TipoCron, "90 9 * * *", from); err == nil {
		t.Fatal("cron com minuto inválido deveria falhar")
	}
	if _, err := NextRun("desconhecido", "x", from); err == nil {
		t.Fatal("tipo desconhecido deveria falhar")
	}
}

func TestValidTipoEAcao(t *testing.T) {
	for _, tipo := range []string{"once", "interval", "cron"} {
		if !ValidTipo(tipo) {
			t.Fatalf("tipo %q deveria ser válido", tipo)
		}
	}
	if ValidTipo("qualquer") {
		t.Fatal("tipo inválido aceito")
	}
	for _, acao := range []string{"enviar_mensagem", "rodar_ferramenta", "callback_agente"} {
		if !ValidAcao(acao) {
			t.Fatalf("ação %q deveria ser válida", acao)
		}
	}
	if ValidAcao("voar") {
		t.Fatal("ação inválida aceita")
	}
}
