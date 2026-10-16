package web

import (
	"testing"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

var horarioSempreAberto = store.AtendenteDisponivel{
	HorarioAbertura: "00:00", HorarioFechamento: "23:59", DiasSemana: "0,1,2,3,4,5,6",
}

func atendente(usuarioID, carga, capacidade int) store.AtendenteDisponivel {
	cd := horarioSempreAberto
	cd.UsuarioID = usuarioID
	cd.AtendenteID = usuarioID
	cd.Nome = "Atendente"
	cd.Carga = carga
	cd.Capacidade = capacidade
	return cd
}

func TestPickBestAtendenteEscolheMenosCarregado(t *testing.T) {
	cands := []store.AtendenteDisponivel{atendente(1, 5, 0), atendente(2, 1, 0), atendente(3, 3, 0)}
	best := pickBestAtendente(store.Conversation{}, cands, false, time.Now(), nil, initialAtendenteLoads(cands))
	if best == nil || best.UsuarioID != 2 {
		t.Fatalf("esperava atendente 2 (menos carregado), obtido %#v", best)
	}
}

func TestPickBestAtendenteRespeitaCapacidade(t *testing.T) {
	cands := []store.AtendenteDisponivel{atendente(1, 2, 2), atendente(2, 5, 0)}
	best := pickBestAtendente(store.Conversation{}, cands, false, time.Now(), nil, initialAtendenteLoads(cands))
	if best == nil || best.UsuarioID != 2 {
		t.Fatalf("atendente 1 está na capacidade; esperava atendente 2, obtido %#v", best)
	}
}

func TestPickBestAtendenteSemTetoNaoBloqueia(t *testing.T) {
	cands := []store.AtendenteDisponivel{atendente(1, 99, 0)}
	best := pickBestAtendente(store.Conversation{}, cands, false, time.Now(), nil, initialAtendenteLoads(cands))
	if best == nil || best.UsuarioID != 1 {
		t.Fatalf("capacidade 0 = sem teto; esperava atendente 1, obtido %#v", best)
	}
}

func TestPickBestAtendenteDistribuiEmLote(t *testing.T) {
	cands := []store.AtendenteDisponivel{atendente(1, 0, 0), atendente(2, 0, 0)}
	loads := initialAtendenteLoads(cands)
	first := pickBestAtendente(store.Conversation{}, cands, false, time.Now(), nil, loads)
	if first == nil || first.UsuarioID != 1 {
		t.Fatalf("primeira escolha deveria ser atendente 1, obtido %#v", first)
	}
	loads[first.UsuarioID]++
	second := pickBestAtendente(store.Conversation{}, cands, false, time.Now(), nil, loads)
	if second == nil || second.UsuarioID != 2 {
		t.Fatalf("segunda escolha deveria ir para o atendente 2 (justiça no lote), obtido %#v", second)
	}
}

func TestPickBestAtendenteFiltraPorSetor(t *testing.T) {
	setorID := 7
	comSetor := atendente(1, 0, 0)
	comSetor.Setores = []int{7}
	semSetor := atendente(2, 0, 0)
	semSetor.Setores = []int{8}
	cands := []store.AtendenteDisponivel{comSetor, semSetor}
	best := pickBestAtendente(store.Conversation{SetorID: &setorID}, cands, false, time.Now(), nil, initialAtendenteLoads(cands))
	if best == nil || best.UsuarioID != 1 {
		t.Fatalf("esperava o atendente do setor 7, obtido %#v", best)
	}
}

func TestPickBestAtendenteModoAbertoIgnoraSetor(t *testing.T) {
	setorID := 7
	outro := atendente(2, 0, 0)
	outro.Setores = []int{8}
	cands := []store.AtendenteDisponivel{outro}
	best := pickBestAtendente(store.Conversation{SetorID: &setorID}, cands, true, time.Now(), nil, initialAtendenteLoads(cands))
	if best == nil || best.UsuarioID != 2 {
		t.Fatalf("modo aberto deveria aceitar qualquer setor, obtido %#v", best)
	}
}

func TestPickBestAtendenteRespeitaHorario(t *testing.T) {
	terca := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	folga := horarioSempreAberto
	folga.UsuarioID = 1
	folga.DiasSemana = "0"
	trabalha := horarioSempreAberto
	trabalha.UsuarioID = 2
	trabalha.DiasSemana = "2"
	cands := []store.AtendenteDisponivel{folga, trabalha}
	best := pickBestAtendente(store.Conversation{}, cands, false, terca, nil, initialAtendenteLoads(cands))
	if best == nil || best.UsuarioID != 2 {
		t.Fatalf("atendente de folga não deveria ser escolhido, obtido %#v", best)
	}
}

func TestPickBestAtendentePrefereHabilidade(t *testing.T) {
	semSkill := atendente(1, 0, 0)
	comSkill := atendente(2, 5, 0)
	comSkill.Habilidades = []string{"financeiro"}
	cands := []store.AtendenteDisponivel{semSkill, comSkill}
	best := pickBestAtendente(store.Conversation{}, cands, false, time.Now(), []string{"financeiro"}, initialAtendenteLoads(cands))
	if best == nil || best.UsuarioID != 2 {
		t.Fatalf("esperava o atendente com a habilidade, obtido %#v", best)
	}
}

func TestPickBestAtendenteSemHabilidadeNaoTrava(t *testing.T) {
	cands := []store.AtendenteDisponivel{atendente(1, 2, 0), atendente(2, 4, 0)}
	best := pickBestAtendente(store.Conversation{}, cands, false, time.Now(), []string{"financeiro"}, initialAtendenteLoads(cands))
	if best == nil || best.UsuarioID != 1 {
		t.Fatalf("sem ninguém com a habilidade deve cair no menos carregado, obtido %#v", best)
	}
}

func TestPickBestAtendenteNinguemElegivel(t *testing.T) {
	terca := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	folga := horarioSempreAberto
	folga.UsuarioID = 1
	folga.DiasSemana = "0"
	cands := []store.AtendenteDisponivel{folga}
	if best := pickBestAtendente(store.Conversation{}, cands, false, terca, nil, initialAtendenteLoads(cands)); best != nil {
		t.Fatalf("esperava nil quando ninguém está no horário, obtido %#v", best)
	}
}

func TestPickBestAtendenteFallbackOffline(t *testing.T) {
	offline := atendente(1, 3, 0)
	cands := []store.AtendenteDisponivel{offline}
	best := pickBestAtendente(store.Conversation{}, cands, false, time.Now(), nil, initialAtendenteLoads(cands))
	if best == nil || best.UsuarioID != 1 {
		t.Fatalf("fallback deve escolher o atendente ativo mesmo offline, obtido %#v", best)
	}
}

func TestEffectiveAtendenteLoad(t *testing.T) {
	cd := atendente(1, 4, 0)
	if got := effectiveAtendenteLoad(&cd, nil); got != 4 {
		t.Fatalf("sem mapa deve usar a carga do candidato, obtido %d", got)
	}
	if got := effectiveAtendenteLoad(&cd, map[int]int{1: 9}); got != 9 {
		t.Fatalf("mapa deve sobrepor a carga, obtido %d", got)
	}
}

func TestNormalizeReactAtendenteCapacidade(t *testing.T) {
	cases := map[int]int{-5: 0, 0: 0, 5: 5, 999: 999, 1000: 999}
	for in, want := range cases {
		if got := normalizeReactAtendenteCapacidade(in); got != want {
			t.Fatalf("capacidade %d: esperava %d, obtido %d", in, want, got)
		}
	}
}
