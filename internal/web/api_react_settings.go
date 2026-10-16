package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

type reactOperationalSettings struct {
	ModoAberto                 bool   `json:"modo_aberto"`
	IAMonitora                 bool   `json:"ia_monitora"`
	DistribuicaoAuto           bool   `json:"distribuicao_automatica"`
	FollowupAtivo              bool   `json:"followup_ativo"`
	FollowupHoras              int    `json:"followup_horas"`
	IaNaFila                   bool   `json:"ia_na_fila"`
	DetectarBots               bool   `json:"detectar_bots"`
	BotPatterns                string `json:"bot_patterns"`
	RegistrarFalhasAprendizado bool   `json:"registrar_falhas_aprendizado"`
	TransferirFalhasHumano     bool   `json:"transferir_falhas_humano"`
	KnowledgeTopK              int    `json:"knowledge_top_k"`
	WatchdogAtivo              bool   `json:"watchdog_ativo"`
	WatchdogMinutos            int    `json:"watchdog_minutos"`
	MediaTTLDias               int    `json:"media_ttl_dias"`
	MediaPurgaAtivo            bool   `json:"media_purga_ativo"`
	ImageModel                 string `json:"image_model"`
	ImageAspectRatio           string `json:"image_aspect_ratio"`
	LogsRetencaoDias           int    `json:"logs_retencao_dias"`
	DefaultModel               string `json:"default_model"`
	STTModel                   string `json:"stt_model"`
	NumerosPermitidos          string `json:"numeros_permitidos"`
}

type reactSettingsData struct {
	Operational reactOperationalSettings `json:"operational"`
	Scope       string                   `json:"scope"`
	CanEdit     bool                     `json:"can_edit"`
}

type reactSettingsInput struct {
	ModoAberto                 *bool   `json:"modo_aberto"`
	IAMonitora                 *bool   `json:"ia_monitora"`
	DistribuicaoAuto           *bool   `json:"distribuicao_automatica"`
	FollowupAtivo              *bool   `json:"followup_ativo"`
	FollowupHoras              *int    `json:"followup_horas"`
	IaNaFila                   *bool   `json:"ia_na_fila"`
	DetectarBots               *bool   `json:"detectar_bots"`
	BotPatterns                *string `json:"bot_patterns"`
	RegistrarFalhasAprendizado *bool   `json:"registrar_falhas_aprendizado"`
	TransferirFalhasHumano     *bool   `json:"transferir_falhas_humano"`
	KnowledgeTopK              *int    `json:"knowledge_top_k"`
	WatchdogAtivo              *bool   `json:"watchdog_ativo"`
	WatchdogMinutos            *int    `json:"watchdog_minutos"`
	MediaTTLDias               *int    `json:"media_ttl_dias"`
	MediaPurgaAtivo            *bool   `json:"media_purga_ativo"`
	ImageModel                 *string `json:"image_model"`
	ImageAspectRatio           *string `json:"image_aspect_ratio"`
	LogsRetencaoDias           *int    `json:"logs_retencao_dias"`
	DefaultModel               *string `json:"default_model"`
	STTModel                   *string `json:"stt_model"`
	NumerosPermitidos          *string `json:"numeros_permitidos"`
}

func requireReactSettingsAccess(s *Server, w http.ResponseWriter, r *http.Request) bool {
	if _, err := store.TenantIDRequired(r.Context()); err != nil {
		writeReactError(w, http.StatusUnauthorized, "tenant_required", "contexto da empresa não identificado")
		return false
	}
	if !s.reactModuleEnabled(r.Context(), "config") {
		writeReactError(w, http.StatusForbidden, "module_disabled", "módulo desativado")
		return false
	}
	if !userHasPerm(currentUser(r), permConfigGerenciar) {
		writeReactError(w, http.StatusForbidden, "permission_denied", "sem permissão para alterar as configurações")
		return false
	}
	return true
}

func reactOperationalSettingsFromStore(settings store.Settings) reactOperationalSettings {
	return reactOperationalSettings{
		ModoAberto: settings.ModoAberto, IAMonitora: settings.IAMonitora,
		DistribuicaoAuto: settings.DistribuicaoAuto, FollowupAtivo: settings.FollowupAtivo,
		FollowupHoras: settings.FollowupHoras, IaNaFila: settings.IaNaFila,
		DetectarBots: settings.DetectarBots, BotPatterns: settings.BotPatterns,
		RegistrarFalhasAprendizado: settings.RegistrarFalhasAprendizado, TransferirFalhasHumano: settings.TransferirFalhasHumano,
		KnowledgeTopK: settings.KnowledgeTopK,
		WatchdogAtivo: settings.WatchdogAtivo, WatchdogMinutos: settings.WatchdogMinutos,
		MediaTTLDias: settings.MediaTTLDias, MediaPurgaAtivo: settings.MediaPurgaAtivo,
		ImageModel: settings.ImageModel, ImageAspectRatio: settings.ImageAspectRatio,
		LogsRetencaoDias: settings.LogsRetencaoDias,
	}
}

func reactSettingsDataFromStore(settings store.Settings, user *store.Usuario) reactSettingsData {
	operational := reactOperationalSettingsFromStore(settings)
	if user != nil && user.Role == "super_admin" {
		operational.DefaultModel = settings.DefaultModel
		operational.STTModel = settings.STTModel
		operational.NumerosPermitidos = settings.NumerosPermitidos
	}
	return reactSettingsData{
		Operational: operational, Scope: "tenant",
		CanEdit: userHasPerm(user, permConfigGerenciar),
	}
}

func reactSettingsPlatformFieldsChanged(settings store.Settings, input reactSettingsInput) bool {
	if input.DefaultModel != nil && strings.TrimSpace(*input.DefaultModel) != settings.DefaultModel {
		return true
	}
	if input.STTModel != nil && strings.TrimSpace(*input.STTModel) != settings.STTModel {
		return true
	}
	if input.NumerosPermitidos != nil && strings.TrimSpace(*input.NumerosPermitidos) != settings.NumerosPermitidos {
		return true
	}
	return false
}

func decodeReactSettingsInput(r *http.Request) (reactSettingsInput, error) {
	var input reactSettingsInput
	if err := json.NewDecoder(io.LimitReader(r.Body, 16*1024+1)).Decode(&input); err != nil {
		return input, errors.New("JSON inválido")
	}
	return input, nil
}

func applyReactSettingsInput(settings *store.Settings, input reactSettingsInput) error {
	if input.ModoAberto != nil {
		settings.ModoAberto = *input.ModoAberto
	}
	if input.IAMonitora != nil {
		settings.IAMonitora = *input.IAMonitora
	}
	if input.DistribuicaoAuto != nil {
		settings.DistribuicaoAuto = *input.DistribuicaoAuto
	}
	if input.FollowupAtivo != nil {
		settings.FollowupAtivo = *input.FollowupAtivo
	}
	if input.FollowupHoras != nil {
		if *input.FollowupHoras < 1 || *input.FollowupHoras > 720 {
			return errors.New("followup_horas deve estar entre 1 e 720")
		}
		settings.FollowupHoras = *input.FollowupHoras
	}
	if input.IaNaFila != nil {
		settings.IaNaFila = *input.IaNaFila
	}
	if input.DetectarBots != nil {
		settings.DetectarBots = *input.DetectarBots
	}
	if input.BotPatterns != nil {
		settings.BotPatterns = strings.TrimSpace(*input.BotPatterns)
		if len([]rune(settings.BotPatterns)) > 5000 {
			return errors.New("bot_patterns excede o limite permitido")
		}
	}
	if input.RegistrarFalhasAprendizado != nil {
		settings.RegistrarFalhasAprendizado = *input.RegistrarFalhasAprendizado
	}
	if input.TransferirFalhasHumano != nil {
		settings.TransferirFalhasHumano = *input.TransferirFalhasHumano
	}
	if input.KnowledgeTopK != nil {
		if *input.KnowledgeTopK < 1 || *input.KnowledgeTopK > 20 {
			return errors.New("knowledge_top_k deve estar entre 1 e 20")
		}
		settings.KnowledgeTopK = *input.KnowledgeTopK
	}
	if input.WatchdogAtivo != nil {
		settings.WatchdogAtivo = *input.WatchdogAtivo
	}
	if input.WatchdogMinutos != nil {
		if *input.WatchdogMinutos < 1 || *input.WatchdogMinutos > 120 {
			return errors.New("watchdog_minutos deve estar entre 1 e 120")
		}
		settings.WatchdogMinutos = *input.WatchdogMinutos
	}
	if input.MediaTTLDias != nil {
		if *input.MediaTTLDias < 1 || *input.MediaTTLDias > 3650 {
			return errors.New("media_ttl_dias deve estar entre 1 e 3650")
		}
		settings.MediaTTLDias = *input.MediaTTLDias
	}
	if input.MediaPurgaAtivo != nil {
		settings.MediaPurgaAtivo = *input.MediaPurgaAtivo
	}
	if input.ImageModel != nil {
		settings.ImageModel = strings.TrimSpace(*input.ImageModel)
		if len([]rune(settings.ImageModel)) > 160 {
			return errors.New("image_model deve ter no máximo 160 caracteres")
		}
	}
	if input.ImageAspectRatio != nil {
		settings.ImageAspectRatio = strings.TrimSpace(*input.ImageAspectRatio)
		if len([]rune(settings.ImageAspectRatio)) > 16 {
			return errors.New("image_aspect_ratio deve ter no máximo 16 caracteres")
		}
		if settings.ImageAspectRatio == "" {
			settings.ImageAspectRatio = "1:1"
		}
	}
	if input.LogsRetencaoDias != nil {
		if *input.LogsRetencaoDias < 1 || *input.LogsRetencaoDias > 3650 {
			return errors.New("logs_retencao_dias deve estar entre 1 e 3650")
		}
		settings.LogsRetencaoDias = *input.LogsRetencaoDias
	}
	if input.DefaultModel != nil {
		settings.DefaultModel = strings.TrimSpace(*input.DefaultModel)
		if len([]rune(settings.DefaultModel)) > 160 {
			return errors.New("default_model deve ter no máximo 160 caracteres")
		}
	}
	if input.STTModel != nil {
		settings.STTModel = strings.TrimSpace(*input.STTModel)
		if len([]rune(settings.STTModel)) > 160 {
			return errors.New("stt_model deve ter no máximo 160 caracteres")
		}
	}
	if input.NumerosPermitidos != nil {
		settings.NumerosPermitidos = strings.TrimSpace(*input.NumerosPermitidos)
		if len([]rune(settings.NumerosPermitidos)) > 20000 {
			return errors.New("numeros_permitidos excede o limite permitido")
		}
	}
	return nil
}

func (s *Server) handleReactSettings(w http.ResponseWriter, r *http.Request) {
	if !requireReactSettingsAccess(s, w, r) {
		return
	}
	settings, err := s.st.GetSettings(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar as configurações")
		return
	}
	writeReactResponse(w, http.StatusOK, reactSettingsDataFromStore(settings, currentUser(r)), nil)
}

func (s *Server) handleReactSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireReactSettingsAccess(s, w, r) {
		return
	}
	input, err := decodeReactSettingsInput(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	settings, err := s.st.GetSettings(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar as configurações")
		return
	}
	if current := currentUser(r); current == nil || current.Role != "super_admin" {
		if reactSettingsPlatformFieldsChanged(settings, input) {
			writeReactError(w, http.StatusForbidden, "platform_settings_only", "estes detalhes técnicos pertencem apenas ao super_admin")
			return
		}
	}
	if err := applyReactSettingsInput(&settings, input); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_settings", err.Error())
		return
	}
	if err := s.st.UpdateSettings(r.Context(), settings); err != nil {
		writeReactError(w, http.StatusInternalServerError, "update_failed", "não foi possível salvar as configurações")
		return
	}
	updated, err := s.st.GetSettings(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "configurações salvas, mas não foi possível recarregá-las")
		return
	}
	writeReactResponse(w, http.StatusOK, reactSettingsDataFromStore(updated, currentUser(r)), nil)
}
