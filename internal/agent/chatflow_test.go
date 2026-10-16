package agent

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCFStartOrdFindsExplicitStart(t *testing.T) {
	order := []int{1, 2, 3}
	byOrd := map[int]cfStep{
		1: {ord: 1, tipo: cfStepMessage},
		2: {ord: 2, tipo: cfStepStart},
		3: {ord: 3, tipo: cfStepEnd},
	}
	if got := cfStartOrd(order, byOrd); got != 2 {
		t.Fatalf("esperava start ord 2, obteve %d", got)
	}
}

func TestCFStartOrdFallsBackToFirstStep(t *testing.T) {
	order := []int{4, 7, 9}
	byOrd := map[int]cfStep{
		4: {ord: 4, tipo: cfStepMessage},
		7: {ord: 7, tipo: cfStepEnd},
	}
	if got := cfStartOrd(order, byOrd); got != 4 {
		t.Fatalf("esperava fallback para 4, obteve %d", got)
	}
}

func TestCFStartConfigNext(t *testing.T) {
	var cfg cfStartConfig
	if err := json.Unmarshal([]byte(`{"next":5}`), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Next == nil || *cfg.Next != 5 {
		t.Fatalf("esperava next 5, obteve %v", cfg.Next)
	}
}

func TestCFEvalConditionEqualityOnMessage(t *testing.T) {
	cfg := cfConditionConfig{Field: "text", Operator: "eq", Value: "1"}
	if !cfEvalCondition("1", nil, cfg) {
		t.Fatal("esperava igualdade com a mensagem")
	}
	if cfEvalCondition("2", nil, cfg) {
		t.Fatal("não esperava igualdade com a mensagem")
	}
}

func TestCFEvalConditionEqualityOnVariable(t *testing.T) {
	vars := map[string]string{"setor": "financeiro"}
	cfg := cfConditionConfig{Field: "setor", Operator: "eq", Value: "FINANCEIRO"}
	if !cfEvalCondition("", vars, cfg) {
		t.Fatal("esperava igualdade case-insensitive na variável")
	}
}

func TestCFEvalConditionContains(t *testing.T) {
	cfg := cfConditionConfig{Field: "text", Operator: "contains", Value: "pedido"}
	if !cfEvalCondition("Quero fazer um PEDIDO", nil, cfg) {
		t.Fatal("esperava contains na mensagem")
	}
	if cfEvalCondition("quero suporte", nil, cfg) {
		t.Fatal("não esperava contains")
	}
}

func TestCFNextAfterExplicitAndImplicit(t *testing.T) {
	order := []int{1, 2, 5, 8}
	if next, ok := cfNextAfter(order, 1, nil); !ok || next != 2 {
		t.Fatalf("esperava próximo implícito 2, obteve %d ok=%v", next, ok)
	}
	explicit := 5
	if next, ok := cfNextAfter(order, 1, &explicit); !ok || next != 5 {
		t.Fatalf("esperava próximo explícito 5, obteve %d ok=%v", next, ok)
	}
	explicit = 99
	if next, ok := cfNextAfter(order, 1, &explicit); !ok || next != 2 {
		t.Fatalf("próximo explícito inexistente deveria cair no implícito 2, obteve %d ok=%v", next, ok)
	}
	if _, ok := cfNextAfter(order, 8, nil); ok {
		t.Fatal("último passo não deveria ter próximo")
	}
}

func TestCFAppendTextJoinsWithNewline(t *testing.T) {
	var reply Reply
	cfAppendText(&reply, "  Olá  ")
	cfAppendText(&reply, "")
	cfAppendText(&reply, "Tudo bem?")
	want := "Olá\nTudo bem?"
	if reply.Text != want {
		t.Fatalf("esperava %q, obteve %q", want, reply.Text)
	}
}

func TestCFIntentFallbackPicksDefault(t *testing.T) {
	fallback := 4
	cfg := cfIntentConfig{Branches: []cfIntentBranch{{Label: "a", Next: intPtr(2)}, {Label: "b", Next: intPtr(3)}}, Default: &fallback}
	runner := &cfRun{byOrd: map[int]cfStep{2: {ord: 2}, 3: {ord: 3}, 4: {ord: 4}}}
	next, ok := runner.intentFallback(cfg)
	if !ok || next != 4 {
		t.Fatalf("esperava default 4, obteve %d ok=%v", next, ok)
	}
}

func TestCFIntentFallbackPicksFirstBranchWhenNoDefault(t *testing.T) {
	cfg := cfIntentConfig{Branches: []cfIntentBranch{{Label: "a", Next: intPtr(2)}, {Label: "b", Next: intPtr(3)}}}
	runner := &cfRun{byOrd: map[int]cfStep{2: {ord: 2}, 3: {ord: 3}}}
	next, ok := runner.intentFallback(cfg)
	if !ok || next != 2 {
		t.Fatalf("esperava primeiro ramo 2, obteve %d ok=%v", next, ok)
	}
}

func TestCFValidBranchOrdRejectsZeroAndMissing(t *testing.T) {
	runner := &cfRun{byOrd: map[int]cfStep{2: {ord: 2}, 3: {ord: 3}}}
	if _, ok := runner.validBranchOrd(intPtr(0)); ok {
		t.Fatal("ord 0 não deveria ser um próximo válido")
	}
	if _, ok := runner.validBranchOrd(intPtr(9)); ok {
		t.Fatal("ord inexistente não deveria ser válido")
	}
	if next, ok := runner.validBranchOrd(intPtr(2)); !ok || next != 2 {
		t.Fatalf("esperava próximo 2, obteve %d ok=%v", next, ok)
	}
}

func intPtr(value int) *int {
	return &value
}

func TestCFMenuMatchByNumberAndLabel(t *testing.T) {
	cfg := cfMenuConfig{Options: []cfMenuOption{{Label: "Financeiro", Value: "fin"}, {Label: "Suporte"}}}
	if index, ok := cfMenuMatch(cfg, "2"); !ok || index != 1 {
		t.Fatalf("esperava índice 1 pelo número, obteve %d ok=%v", index, ok)
	}
	if index, ok := cfMenuMatch(cfg, "financeiro"); !ok || index != 0 {
		t.Fatalf("esperava índice 0 pelo rótulo, obteve %d ok=%v", index, ok)
	}
	if index, ok := cfMenuMatch(cfg, "fin"); !ok || index != 0 {
		t.Fatalf("esperava índice 0 pelo valor, obteve %d ok=%v", index, ok)
	}
	if _, ok := cfMenuMatch(cfg, "9"); ok {
		t.Fatal("número fora da faixa não deveria casar")
	}
	if _, ok := cfMenuMatch(cfg, "  "); ok {
		t.Fatal("mensagem vazia não deveria casar")
	}
}

func TestCFMenuPromptNumbersOptions(t *testing.T) {
	got := cfMenuPrompt("Escolha:", []cfMenuOption{{Label: "A"}, {Label: "B"}})
	want := "Escolha:\n1. A\n2. B"
	if got != want {
		t.Fatalf("esperava %q, obteve %q", want, got)
	}
	if fallback := cfMenuPrompt("", nil); fallback != "Escolha uma opção:" {
		t.Fatalf("esperava fallback padrão, obteve %q", fallback)
	}
}

func TestCFMenuRichButtonsUseNumberIDs(t *testing.T) {
	cfg := cfMenuConfig{Type: "buttons", Options: []cfMenuOption{{Label: "Vendas"}, {Label: "Suporte"}}}
	rich := cfMenuRich(cfg, "Menu")
	if rich == nil || rich.Kind != "buttons" || len(rich.Buttons) != 2 {
		t.Fatalf("esperava mensagem de botões com 2 opções, obteve %+v", rich)
	}
	if rich.Buttons[1].ID != "2" || rich.Buttons[1].Title != "Suporte" {
		t.Fatalf("botão deveria carregar número como id, obteve %+v", rich.Buttons[1])
	}
	if !strings.Contains(rich.Fallback, "2. Suporte") {
		t.Fatalf("fallback deveria numerar as opções, obteve %q", rich.Fallback)
	}
}

func TestCFDelaySecondsCapsLongDelay(t *testing.T) {
	if got := cfDelaySeconds(cfDelayConfig{Minutes: 10}); got != cfMaxDelaySeconds {
		t.Fatalf("delay longo deveria ser limitado a %d, obteve %d", cfMaxDelaySeconds, got)
	}
	if got := cfDelaySeconds(cfDelayConfig{Seconds: 5}); got != 5 {
		t.Fatalf("esperava 5 segundos, obteve %d", got)
	}
	if got := cfDelaySeconds(cfDelayConfig{Minutes: 1, Seconds: 2}); got != cfMaxDelaySeconds {
		t.Fatalf("62s deveria ser limitado, obteve %d", got)
	}
	if got := cfDelaySeconds(cfDelayConfig{}); got != 0 {
		t.Fatalf("sem configuração deveria ser 0, obteve %d", got)
	}
}

func TestCFGotoStopsAfterJumpLimit(t *testing.T) {
	cfg, _ := json.Marshal(cfGotoConfig{Target: intPtr(1)})
	runner := &cfRun{byOrd: map[int]cfStep{1: {ord: 1, tipo: cfStepGoto, cfg: cfg}}, path: []int{}}
	if err := runner.advance(1); err != nil {
		t.Fatal(err)
	}
	if runner.jumps != maxChatflowJumps {
		t.Fatalf("esperava %d saltos até parar, obteve %d", maxChatflowJumps, runner.jumps)
	}
}

func TestCFGotoToMissingTargetIsIgnored(t *testing.T) {
	runner := &cfRun{byOrd: map[int]cfStep{}}
	if err := runner.jumpTo(99); err != nil {
		t.Fatal(err)
	}
	if runner.jumps != 0 {
		t.Fatalf("salto para bloco inexistente não deveria contar, obteve %d", runner.jumps)
	}
}

func TestCFBusinessHoursOpen(t *testing.T) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		loc = time.UTC
	}
	cfg := cfBusinessHoursConfig{Abertura: "09:00", Fechamento: "18:00", Dias: "0,1,2,3,4,5,6"}
	if !cfBusinessHoursOpen(cfg, time.Date(2026, 9, 16, 10, 0, 0, 0, loc)) {
		t.Fatal("10:00 deveria estar aberto")
	}
	if cfBusinessHoursOpen(cfg, time.Date(2026, 9, 16, 20, 0, 0, 0, loc)) {
		t.Fatal("20:00 deveria estar fechado")
	}
	if !cfBusinessHoursOpen(cfBusinessHoursConfig{}, time.Now()) {
		t.Fatal("sem horário configurado deveria considerar aberto")
	}
	overnight := cfBusinessHoursConfig{Abertura: "22:00", Fechamento: "06:00", Dias: "0,1,2,3,4,5,6"}
	if !cfBusinessHoursOpen(overnight, time.Date(2026, 9, 16, 23, 0, 0, 0, loc)) {
		t.Fatal("turno da madrugada às 23:00 deveria estar aberto")
	}
	if cfBusinessHoursOpen(overnight, time.Date(2026, 9, 16, 12, 0, 0, 0, loc)) {
		t.Fatal("turno da madrugada ao meio-dia deveria estar fechado")
	}
}

func TestCFBusinessHoursRespectsWeekday(t *testing.T) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		loc = time.UTC
	}
	cfg := cfBusinessHoursConfig{Abertura: "00:00", Fechamento: "23:59", Dias: "1"}
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, loc)
	if int(now.Weekday()) == 1 {
		if !cfBusinessHoursOpen(cfg, now) {
			t.Fatal("segunda-feira deveria estar aberto")
		}
	} else if cfBusinessHoursOpen(cfg, now) {
		t.Fatal("fora de segunda-feira deveria estar fechado")
	}
}

func TestCFSetVariableOperations(t *testing.T) {
	runner := &cfRun{vars: map[string]string{"nome": "Ana", "total": "2"}}
	runner.applySetVar(cfSetVarConfig{Variable: "saudacao", Value: "Olá {{nome}}", Operation: "set"})
	if runner.vars["saudacao"] != "Olá Ana" {
		t.Fatalf("set deveria interpolar, obteve %q", runner.vars["saudacao"])
	}
	runner.applySetVar(cfSetVarConfig{Variable: "saudacao", Value: "!", Operation: "append"})
	if runner.vars["saudacao"] != "Olá Ana!" {
		t.Fatalf("append deveria concatenar, obteve %q", runner.vars["saudacao"])
	}
	runner.applySetVar(cfSetVarConfig{Variable: "total", Value: "3", Operation: "add"})
	if runner.vars["total"] != "5" {
		t.Fatalf("add deveria somar, obteve %q", runner.vars["total"])
	}
	runner.applySetVar(cfSetVarConfig{Variable: "", Value: "x"})
	if len(runner.vars) != 3 {
		t.Fatalf("variável vazia não deveria criar nada, obteve %d", len(runner.vars))
	}
}

func TestCFSatisfactionRichHasFiveButtons(t *testing.T) {
	rich := cfSatisfactionRich("Avalie")
	if rich == nil || rich.Kind != "buttons" || len(rich.Buttons) != 5 {
		t.Fatalf("esperava 5 botões de nota, obteve %+v", rich)
	}
	if rich.Fallback != "Avalie" {
		t.Fatalf("fallback deveria ser o texto, obteve %q", rich.Fallback)
	}
	if cfSatisfactionRich("  ") != nil {
		t.Fatal("texto vazio não deveria gerar mensagem rica")
	}
}
