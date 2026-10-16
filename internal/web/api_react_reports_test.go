package web

import (
	"bytes"
	"encoding/csv"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func TestParseReactReportFilterPeriods(t *testing.T) {
	cases := map[string]string{
		"":     "7",
		"hoje": "1",
		"7d":   "7",
		"30d":  "30",
		"mes":  "30",
	}
	for raw, want := range cases {
		request := httptest.NewRequest(http.MethodGet, "/api/react/reports?period="+raw, nil)
		filter, err := parseReactReportFilter(request)
		if err != nil {
			t.Fatalf("período %q rejeitado: %v", raw, err)
		}
		if filter.Period != want {
			t.Fatalf("período %q => %q; esperado %q", raw, filter.Period, want)
		}
		if !filter.To.After(filter.From) {
			t.Fatalf("período %q: intervalo inválido", raw)
		}
	}
}

func TestParseReactReportFilterCustomAndOptions(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/react/reports?period=custom&from=2026-01-01&to=2026-01-10&channel=WHATSAPP&sector_id=4&attendant_id=9", nil)
	filter, err := parseReactReportFilter(request)
	if err != nil {
		t.Fatalf("filtro rejeitado: %v", err)
	}
	if filter.Period != "custom" {
		t.Fatalf("período=%q; esperado custom", filter.Period)
	}
	if filter.Channel != "whatsapp" {
		t.Fatalf("canal=%q; esperado whatsapp", filter.Channel)
	}
	if filter.SectorID == nil || *filter.SectorID != 4 {
		t.Fatalf("setor=%v; esperado 4", filter.SectorID)
	}
	if filter.AttendantID == nil || *filter.AttendantID != 9 {
		t.Fatalf("atendente=%v; esperado 9", filter.AttendantID)
	}
	if got := filter.To.Sub(filter.From).Hours() / 24; got != 10 {
		t.Fatalf("dias=%.0f; esperado 10", got)
	}
}

func TestParseReactReportFilterRejectsInvalid(t *testing.T) {
	requests := []string{
		"/api/react/reports?period=90d",
		"/api/react/reports?period=custom&from=2026-01-10&to=2026-01-01",
		"/api/react/reports?period=custom&from=2026-01-01&to=2026-03-01",
		"/api/react/reports?period=7d&sector_id=0",
	}
	for _, target := range requests {
		request := httptest.NewRequest(http.MethodGet, target, nil)
		if _, err := parseReactReportFilter(request); err == nil {
			t.Fatalf("filtro inválido aceito: %s", target)
		}
	}
}

func TestConversationReportStats(t *testing.T) {
	location := brtz()
	from := time.Date(2026, 3, 1, 0, 0, 0, 0, location)
	filter := reactReportFilter{From: from, To: from.AddDate(0, 0, 1)}
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, location)
	messages := []store.Message{
		{Source: "customer", CreatedAt: base},
		{Source: "ai", CreatedAt: base.Add(30 * time.Second)},
		{Source: "human", CreatedAt: base.Add(2 * time.Minute)},
		{Source: "human", CreatedAt: base.Add(5 * time.Minute)},
	}
	stats := conversationReportStats(messages, filter)
	if stats.Messages != 4 || stats.Customer != 1 || stats.AI != 1 || stats.Human != 2 {
		t.Fatalf("contagens inesperadas: %#v", stats)
	}
	if !stats.HasHuman {
		t.Fatal("esperava HasHuman=true")
	}
	if !stats.Wait.Started || stats.Wait.Seconds != 120 {
		t.Fatalf("espera inesperada: %#v", stats.Wait)
	}
	if !stats.Handling.Started || stats.Handling.Seconds != 180 {
		t.Fatalf("atendimento inesperado: %#v", stats.Handling)
	}
}

func TestConversationReportStatsWithoutHuman(t *testing.T) {
	location := brtz()
	from := time.Date(2026, 3, 1, 0, 0, 0, 0, location)
	filter := reactReportFilter{From: from, To: from.AddDate(0, 0, 1)}
	base := time.Date(2026, 3, 1, 9, 0, 0, 0, location)
	messages := []store.Message{
		{Source: "customer", CreatedAt: base},
		{Source: "ai", CreatedAt: base.Add(10 * time.Second)},
	}
	stats := conversationReportStats(messages, filter)
	if stats.HasHuman || stats.Wait.Started || stats.Handling.Started {
		t.Fatalf("esperava ausência de humano: %#v", stats)
	}
}

func TestReactReportRatingHelpers(t *testing.T) {
	ai := 5
	human := 3
	assessment := store.Avaliacao{NotaIA: &ai, NotaOperador: &human}
	if got := reactReportRatingCount(assessment); got != 2 {
		t.Fatalf("contagem de notas=%d; esperado 2", got)
	}
	if got := reactReportSatisfiedRatings(assessment); got != 1 {
		t.Fatalf("satisfeitos=%d; esperado 1", got)
	}
	overall := reactReportOverallRating(assessment)
	if overall == nil || math.Abs(*overall-4) > 0.001 {
		t.Fatalf("nota geral=%v; esperado 4", overall)
	}
	if reactReportOverallRating(store.Avaliacao{}) != nil {
		t.Fatal("avaliação sem notas deveria retornar nil")
	}
}

func TestReactReportsCSVRowsHeaderAndDetails(t *testing.T) {
	ai := 5
	human := 4
	overall := 4.5
	data := reactReportsData{
		Filters: reactReportsFilters{From: "2026-03-01", To: "2026-03-07", Channel: "whatsapp"},
		Summary: reactReportsSummary{Conversations: 3, Closed: 2, AIResolvedCount: 1, HumanResolvedCount: 1, AvgWaitTime: 12.5, AvgHandlingTime: 300, CSAT: 100, OverallRating: 4.5},
		Details: []reactReportDetail{{
			ConversationID: 7, Protocol: "#T-7", Channel: "whatsapp", Sector: "Financeiro", Attendant: "Ana",
			Status: "closed", CreatedAt: time.Date(2026, 3, 2, 14, 30, 0, 0, time.UTC),
			Messages: 9, CustomerMessages: 4, AIMessages: 2, HumanMessages: 3,
			WaitSeconds: 12.5, HandlingSeconds: 300, OverallRating: &overall, AIRating: &ai, HumanRating: &human,
		}},
	}
	rows := reactReportsCSVRows(data)
	headerIndex := -1
	for index, row := range rows {
		if len(row) > 0 && row[0] == "Protocolo" {
			headerIndex = index
			break
		}
	}
	if headerIndex < 0 {
		t.Fatal("cabeçalho de atendimentos não encontrado")
	}
	if !equalStrings(rows[headerIndex], reactReportsCSVHeader) {
		t.Fatalf("cabeçalho inesperado: %#v", rows[headerIndex])
	}
	detail := rows[headerIndex+1]
	if detail[0] != "#T-7" || detail[2] != "Financeiro" || detail[3] != "Ana" || detail[6] != "9" {
		t.Fatalf("linha de atendimento inesperada: %#v", detail)
	}
	if detail[10] != "12.5" || detail[11] != "300" || detail[12] != "4.5" || detail[13] != "5" || detail[14] != "4" {
		t.Fatalf("métricas da linha inesperadas: %#v", detail)
	}
}

func TestWriteReactReportsCSVEscaping(t *testing.T) {
	data := reactReportsData{
		Filters: reactReportsFilters{From: "2026-03-01", To: "2026-03-07"},
		Details: []reactReportDetail{{
			ConversationID: 1, Protocol: "#T-1", Channel: "whatsapp", Sector: `Suporte, "Nível 1"`, Attendant: "João\nSilva",
			Status: "closed", CreatedAt: time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC),
		}},
	}
	var buffer bytes.Buffer
	if err := writeReactReportsCSV(&buffer, data); err != nil {
		t.Fatalf("gerando CSV: %v", err)
	}
	raw := buffer.String()
	if !strings.Contains(raw, `"Suporte, ""Nível 1"""`) {
		t.Fatalf("CSV não escapou setor com vírgula/aspas: %q", raw)
	}
	if !strings.Contains(raw, `"João`+"\n"+`Silva"`) {
		t.Fatalf("CSV não escapou atendente com quebra de linha: %q", raw)
	}
	reader := csv.NewReader(strings.NewReader(raw))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("relendo CSV: %v", err)
	}
	var detail []string
	for _, record := range records {
		if len(record) > 0 && record[0] == "#T-1" {
			detail = record
			break
		}
	}
	if detail == nil {
		t.Fatal("linha do atendimento não encontrada após releitura")
	}
	if detail[2] != `Suporte, "Nível 1"` || detail[3] != "João\nSilva" {
		t.Fatalf("valores não preservados após releitura: %#v", detail)
	}
}

func TestReactReportsCSVOptionalEmpty(t *testing.T) {
	row := reactReportsCSVDetailRow(reactReportDetail{Protocol: "#T-2"})
	if row[12] != "" || row[13] != "" || row[14] != "" {
		t.Fatalf("notas ausentes deveriam vir vazias: %#v", row)
	}
}

func TestHandleReactReportsExportRejectsInvalidFilter(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/react/reports/export?period=90d", nil)
	recorder := httptest.NewRecorder()
	(&Server{}).handleReactReportsExport(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status obtido=%d; esperado=%d", recorder.Code, http.StatusBadRequest)
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
