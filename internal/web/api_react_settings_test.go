package web

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func TestApplyReactSettingsInput(t *testing.T) {
	settings := store.Settings{
		ModoAberto: true, IAMonitora: false, DistribuicaoAuto: false,
		FollowupAtivo: false, FollowupHoras: 24, IaNaFila: false,
	}
	modoAberto := false
	distribuicao := true
	followup := 48
	input := reactSettingsInput{ModoAberto: &modoAberto, DistribuicaoAuto: &distribuicao, FollowupHoras: &followup}
	if err := applyReactSettingsInput(&settings, input); err != nil {
		t.Fatalf("entrada válida rejeitada: %v", err)
	}
	if settings.ModoAberto || !settings.DistribuicaoAuto || settings.FollowupHoras != 48 {
		t.Fatalf("configurações não aplicadas: %+v", settings)
	}
	if settings.IAMonitora || settings.FollowupAtivo || settings.IaNaFila {
		t.Fatalf("campos omitidos deveriam ser preservados: %+v", settings)
	}
}

func TestApplyReactSettingsInputRejectsInvalidFollowup(t *testing.T) {
	settings := store.Settings{FollowupHoras: 24}
	for _, value := range []int{0, -1, 721} {
		followup := value
		if err := applyReactSettingsInput(&settings, reactSettingsInput{FollowupHoras: &followup}); err == nil {
			t.Fatalf("followup %d deveria ser rejeitado", value)
		}
	}
}

func TestDecodeReactSettingsInput(t *testing.T) {
	r, err := http.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/react/settings", strings.NewReader(`{"modo_aberto":true,"followup_horas":36}`))
	if err != nil {
		t.Fatalf("não foi possível criar a requisição: %v", err)
	}
	input, err := decodeReactSettingsInput(r)
	if err != nil {
		t.Fatalf("JSON válido rejeitado: %v", err)
	}
	if input.ModoAberto == nil || !*input.ModoAberto || input.FollowupHoras == nil || *input.FollowupHoras != 36 {
		t.Fatalf("entrada decodificada incorretamente: %+v", input)
	}
}

func TestReactSettingsDataDoesNotExposeSecrets(t *testing.T) {
	settings := store.Settings{
		DefaultAPIKey: "segredo", WhatsAppToken: "token", STTAPIKey: "stt-segredo",
		ModoAberto: true, FollowupHoras: 12,
	}
	data := reactSettingsDataFromStore(settings, &store.Usuario{Role: "admin"})
	if !data.CanEdit || !data.Operational.ModoAberto || data.Operational.FollowupHoras != 12 {
		t.Fatalf("dados operacionais incorretos: %+v", data)
	}
}

func TestApplyReactSettingsInputWatchdogAndMedia(t *testing.T) {
	settings := store.Settings{}
	ativo := true
	minutos := 7
	ttl := 120
	purga := false
	input := reactSettingsInput{
		WatchdogAtivo: &ativo, WatchdogMinutos: &minutos,
		MediaTTLDias: &ttl, MediaPurgaAtivo: &purga,
	}
	if err := applyReactSettingsInput(&settings, input); err != nil {
		t.Fatalf("entrada válida rejeitada: %v", err)
	}
	if !settings.WatchdogAtivo || settings.WatchdogMinutos != 7 || settings.MediaTTLDias != 120 || settings.MediaPurgaAtivo {
		t.Fatalf("configurações de resiliência não aplicadas: %+v", settings)
	}
}

func TestApplyReactSettingsInputRejectsInvalidResilience(t *testing.T) {
	settings := store.Settings{}
	for _, value := range []int{0, -1, 121} {
		minutos := value
		if err := applyReactSettingsInput(&settings, reactSettingsInput{WatchdogMinutos: &minutos}); err == nil {
			t.Fatalf("watchdog_minutos %d deveria ser rejeitado", value)
		}
	}
	for _, value := range []int{0, -1, 3651} {
		ttl := value
		if err := applyReactSettingsInput(&settings, reactSettingsInput{MediaTTLDias: &ttl}); err == nil {
			t.Fatalf("media_ttl_dias %d deveria ser rejeitado", value)
		}
	}
}

func TestReactSettingsDataExposesResilience(t *testing.T) {
	settings := store.Settings{WatchdogAtivo: true, WatchdogMinutos: 5, MediaTTLDias: 90, MediaPurgaAtivo: true}
	data := reactSettingsDataFromStore(settings, &store.Usuario{Role: "admin"})
	if !data.Operational.WatchdogAtivo || data.Operational.WatchdogMinutos != 5 ||
		data.Operational.MediaTTLDias != 90 || !data.Operational.MediaPurgaAtivo {
		t.Fatalf("campos de resiliência ausentes na saída: %+v", data.Operational)
	}
}

func TestReactSettingsPlatformFieldsChanged(t *testing.T) {
	settings := store.Settings{DefaultModel: "modelo-atual", STTModel: "", NumerosPermitidos: "5511"}
	empty := ""
	unchanged := "modelo-atual"
	if reactSettingsPlatformFieldsChanged(settings, reactSettingsInput{}) {
		t.Fatal("campos ausentes não deveriam contar como alteração")
	}
	if reactSettingsPlatformFieldsChanged(settings, reactSettingsInput{DefaultModel: &unchanged, STTModel: &empty}) {
		t.Fatal("reenviar valores iguais não deveria contar como alteração")
	}
	changed := "outro-modelo"
	if !reactSettingsPlatformFieldsChanged(settings, reactSettingsInput{DefaultModel: &changed}) {
		t.Fatal("alterar default_model deveria ser detectado")
	}
	permitidos := "5521"
	if !reactSettingsPlatformFieldsChanged(settings, reactSettingsInput{NumerosPermitidos: &permitidos}) {
		t.Fatal("alterar numeros_permitidos deveria ser detectado")
	}
}
