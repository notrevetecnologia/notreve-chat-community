package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func TestParseReactDashboardPeriod(t *testing.T) {
	cases := map[string]string{
		"":     "7",
		"7d":   "7",
		"hoje": "1",
		"1":    "1",
		"30d":  "30",
		"mes":  "30",
	}
	for raw, want := range cases {
		request := httptest.NewRequest(http.MethodGet, "/api/react/dashboard?period="+raw, nil)
		period, err := parseReactDashboardPeriod(request)
		if err != nil {
			t.Fatalf("período %q rejeitado: %v", raw, err)
		}
		if period.Key != want {
			t.Fatalf("período %q => %q; esperado %q", raw, period.Key, want)
		}
		if period.Days != int(period.To.Sub(period.From)/(24*time.Hour)) {
			t.Fatalf("período %q: dias %d não bate com o intervalo", raw, period.Days)
		}
	}
}

func TestParseReactDashboardPeriodRejectsUnknown(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/react/dashboard?period=90d", nil)
	if _, err := parseReactDashboardPeriod(request); err == nil {
		t.Fatal("período desconhecido deveria ser rejeitado")
	}
}

func TestBuildDashboardVolumePreencheTodosOsDias(t *testing.T) {
	location := brtz()
	from := time.Date(2026, 3, 1, 0, 0, 0, 0, location)
	period := reactDashboardPeriod{Key: "7", From: from, To: from.AddDate(0, 0, 7), Days: 7}
	rows := []store.DashboardVolumeDia{
		{Dia: from, Recebidas: 4, Enviadas: 6},
		{Dia: from.AddDate(0, 0, 2), Recebidas: 1, Enviadas: 2},
	}
	volume := buildDashboardVolume(rows, period)
	if len(volume) != 7 {
		t.Fatalf("série tem %d dias; esperado 7", len(volume))
	}
	if volume[0].Recebidas != 4 || volume[0].Enviadas != 6 {
		t.Fatalf("primeiro dia inesperado: %#v", volume[0])
	}
	if volume[2].Recebidas != 1 || volume[2].Enviadas != 2 {
		t.Fatalf("terceiro dia inesperado: %#v", volume[2])
	}
	if volume[1].Recebidas != 0 || volume[1].Enviadas != 0 {
		t.Fatalf("dia sem dados deveria ser zero: %#v", volume[1])
	}
	if volume[0].Label != "01/03" || volume[0].Date != "2026-03-01" {
		t.Fatalf("rótulo/data inesperados: %#v", volume[0])
	}
}

func TestBuildDashboardVolumeSingleDay(t *testing.T) {
	location := brtz()
	from := time.Date(2026, 3, 10, 0, 0, 0, 0, location)
	period := reactDashboardPeriod{Key: "1", From: from, To: from.AddDate(0, 0, 1), Days: 1}
	volume := buildDashboardVolume(nil, period)
	if len(volume) != 1 || volume[0].Recebidas != 0 || volume[0].Enviadas != 0 {
		t.Fatalf("série de um dia inesperada: %#v", volume)
	}
}

func TestDashboardRate(t *testing.T) {
	cases := []struct {
		part, total int
		want        float64
	}{
		{0, 0, 0},
		{3, 4, 75},
		{1, 3, 33.3},
		{2, 2, 100},
	}
	for _, testCase := range cases {
		if got := dashboardRate(testCase.part, testCase.total); got != testCase.want {
			t.Fatalf("taxa(%d,%d)=%v; esperado %v", testCase.part, testCase.total, got, testCase.want)
		}
	}
}

func TestMapDashboardSectorsAndAttendants(t *testing.T) {
	sectorID := 9
	sectors := mapDashboardSectors([]store.DashboardSetor{{ID: &sectorID, Nome: "Financeiro", Conversas: 12, Aguardando: 2}})
	if len(sectors) != 1 || sectors[0].ID == nil || *sectors[0].ID != 9 || sectors[0].Conversations != 12 || sectors[0].Waiting != 2 {
		t.Fatalf("setor mapeado incorretamente: %#v", sectors)
	}
	attendants := mapDashboardAttendants([]store.DashboardAtendente{{ID: 5, Nome: "Ana", Ativas: 3, Aguardando: 1}})
	if len(attendants) != 1 || attendants[0].ID != 5 || attendants[0].Active != 3 || attendants[0].Waiting != 1 {
		t.Fatalf("atendente mapeado incorretamente: %#v", attendants)
	}
}

func TestReactDashboardPermissionGating(t *testing.T) {
	cases := []struct {
		name     string
		user     *store.Usuario
		reports  bool
		failures bool
	}{
		{name: "atendente", user: &store.Usuario{Role: "atendente"}, reports: false, failures: false},
		{name: "supervisor", user: &store.Usuario{Role: "supervisor"}, reports: true, failures: true},
		{name: "admin", user: &store.Usuario{Role: "admin"}, reports: true, failures: true},
		{name: "explicit-only-conversas", user: &store.Usuario{Role: "atendente", Permissions: "conversas_ver"}, reports: false, failures: false},
		{name: "explicit-reports", user: &store.Usuario{Role: "atendente", Permissions: "conversas_ver,relatorios_ver"}, reports: true, failures: false},
		{name: "nil", user: nil, reports: false, failures: false},
	}
	for _, testCase := range cases {
		got := reactDashboardPermissionsFor(testCase.user)
		if got.Reports != testCase.reports || got.Failures != testCase.failures {
			t.Fatalf("%s: permissões %#v; esperado reports=%v failures=%v", testCase.name, got, testCase.reports, testCase.failures)
		}
	}
}

func TestHandleReactDashboardRejectsInvalidPeriod(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/react/dashboard?period=90d", nil)
	recorder := httptest.NewRecorder()
	(&Server{}).handleReactDashboard(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status obtido=%d; esperado=%d", recorder.Code, http.StatusBadRequest)
	}
}
