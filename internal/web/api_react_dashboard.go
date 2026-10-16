package web

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

type reactDashboardPermissions struct {
	Reports  bool `json:"reports"`
	Failures bool `json:"failures"`
}

type reactDashboardConversations struct {
	IA         int `json:"ia"`
	Aguardando int `json:"aguardando"`
	Atendendo  int `json:"atendendo"`
	Encerradas int `json:"encerradas"`
	Novas      int `json:"novas"`
	Total      int `json:"total"`
}

type reactDashboardQueue struct {
	Aguardando          int `json:"aguardando"`
	EsperaMaxSegundos   int `json:"espera_max_segundos"`
	EsperaMediaSegundos int `json:"espera_media_segundos"`
}

type reactDashboardResolution struct {
	Total        int     `json:"total"`
	IAResolvidas int     `json:"ia_resolvidas"`
	Transferidas int     `json:"transferidas"`
	TaxaIA       float64 `json:"taxa_ia"`
}

type reactDashboardSatisfaction struct {
	Avaliacoes int     `json:"avaliacoes"`
	Media      float64 `json:"media"`
}

type reactDashboardVolume struct {
	Date      string `json:"date"`
	Label     string `json:"label"`
	Recebidas int    `json:"recebidas"`
	Enviadas  int    `json:"enviadas"`
}

type reactDashboardSector struct {
	ID            *int   `json:"id,omitempty"`
	Name          string `json:"name"`
	Conversations int    `json:"conversations"`
	Waiting       int    `json:"waiting"`
}

type reactDashboardAttendant struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Active  int    `json:"active"`
	Waiting int    `json:"waiting"`
}

type reactDashboardFailures struct {
	Open   int            `json:"open"`
	ByType map[string]int `json:"by_type"`
}

type reactDashboardData struct {
	Period        string                      `json:"period"`
	GeneratedAt   time.Time                   `json:"generated_at"`
	Permissions   reactDashboardPermissions   `json:"permissions"`
	Conversations reactDashboardConversations `json:"conversations"`
	Queue         reactDashboardQueue         `json:"queue"`
	Resolution    reactDashboardResolution    `json:"resolution"`
	Satisfaction  *reactDashboardSatisfaction `json:"satisfaction,omitempty"`
	Volume        []reactDashboardVolume      `json:"volume"`
	Sectors       []reactDashboardSector      `json:"sectors,omitempty"`
	Attendants    []reactDashboardAttendant   `json:"attendants"`
	Failures      *reactDashboardFailures     `json:"failures,omitempty"`
}

type reactDashboardPeriod struct {
	Key  string
	From time.Time
	To   time.Time
	Days int
}

func parseReactDashboardPeriod(r *http.Request) (reactDashboardPeriod, error) {
	raw := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("period")))
	if raw == "" {
		raw = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("periodo")))
	}
	location := brtz()
	now := time.Now().In(location)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	switch raw {
	case "", "7", "7d", "semana", "week":
		return reactDashboardPeriod{Key: "7", From: start.AddDate(0, 0, -6), To: start.AddDate(0, 0, 1), Days: 7}, nil
	case "hoje", "today", "1", "1d":
		return reactDashboardPeriod{Key: "1", From: start, To: start.AddDate(0, 0, 1), Days: 1}, nil
	case "30", "30d", "mes", "month":
		return reactDashboardPeriod{Key: "30", From: start.AddDate(0, 0, -29), To: start.AddDate(0, 0, 1), Days: 30}, nil
	default:
		return reactDashboardPeriod{}, errors.New("período inválido")
	}
}

func (s *Server) handleReactDashboard(w http.ResponseWriter, r *http.Request) {
	period, err := parseReactDashboardPeriod(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_period", err.Error())
		return
	}
	user := currentUser(r)
	data, err := s.buildReactDashboard(r.Context(), user, s.userSetores(r.Context(), user), s.modoAberto(r.Context()), userHasPerm(user, permConversasVerIA), period)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "dashboard_unavailable", "não foi possível carregar a visão geral")
		return
	}
	writeReactResponse(w, http.StatusOK, data, map[string]interface{}{"scope": "workspace"})
}

func (s *Server) buildReactDashboard(ctx context.Context, user *store.Usuario, setorIDs []int, modoAberto, verIA bool, period reactDashboardPeriod) (reactDashboardData, error) {
	conversations, err := s.st.DashboardConversations(ctx, period.From, user, setorIDs, modoAberto, verIA)
	if err != nil {
		return reactDashboardData{}, err
	}
	queue, err := s.st.DashboardQueue(ctx, user, setorIDs, modoAberto, verIA)
	if err != nil {
		return reactDashboardData{}, err
	}
	resolution, err := s.st.DashboardResolution(ctx, period.From, user, setorIDs, modoAberto, verIA)
	if err != nil {
		return reactDashboardData{}, err
	}
	volumeRows, err := s.st.DashboardVolume(ctx, period.From, user, setorIDs, modoAberto, verIA)
	if err != nil {
		return reactDashboardData{}, err
	}
	attendants, err := s.st.DashboardAtendentes(ctx, user, setorIDs, modoAberto, verIA)
	if err != nil {
		return reactDashboardData{}, err
	}

	data := reactDashboardData{
		Period:      period.Key,
		GeneratedAt: time.Now().In(brtz()),
		Permissions: reactDashboardPermissionsFor(user),
		Conversations: reactDashboardConversations{
			IA:         conversations.IA,
			Aguardando: conversations.Aguardando,
			Atendendo:  conversations.Atendendo,
			Encerradas: conversations.Encerradas,
			Novas:      conversations.Novas,
			Total:      conversations.Total,
		},
		Queue: reactDashboardQueue{
			Aguardando:          queue.Aguardando,
			EsperaMaxSegundos:   int(math.Round(queue.EsperaMaxSegundos)),
			EsperaMediaSegundos: int(math.Round(queue.EsperaMediaSegundos)),
		},
		Resolution: reactDashboardResolution{
			Total:        resolution.Total,
			IAResolvidas: resolution.IAResolvidas,
			Transferidas: resolution.Transferidas,
			TaxaIA:       dashboardRate(resolution.IAResolvidas, resolution.Total),
		},
		Volume:     buildDashboardVolume(volumeRows, period),
		Attendants: mapDashboardAttendants(attendants),
	}

	if data.Permissions.Reports {
		satisfaction, err := s.st.DashboardSatisfaction(ctx, period.From, user, setorIDs, modoAberto, verIA)
		if err != nil {
			return reactDashboardData{}, err
		}
		sectors, err := s.st.DashboardSetores(ctx, period.From, user, setorIDs, modoAberto, verIA)
		if err != nil {
			return reactDashboardData{}, err
		}
		data.Satisfaction = &reactDashboardSatisfaction{Avaliacoes: satisfaction.Avaliacoes, Media: math.Round(satisfaction.Media*10) / 10}
		data.Sectors = mapDashboardSectors(sectors)
	}

	if data.Permissions.Failures {
		open, err := s.st.CountOpenAgentFailures(ctx)
		if err != nil {
			return reactDashboardData{}, err
		}
		byType, err := s.st.CountAgentFailuresByType(ctx, period.From)
		if err != nil {
			return reactDashboardData{}, err
		}
		data.Failures = &reactDashboardFailures{Open: open, ByType: byType}
	}

	return data, nil
}

func buildDashboardVolume(rows []store.DashboardVolumeDia, period reactDashboardPeriod) []reactDashboardVolume {
	byDay := make(map[string]store.DashboardVolumeDia, len(rows))
	for _, row := range rows {
		byDay[row.Dia.In(period.From.Location()).Format("2006-01-02")] = row
	}
	out := make([]reactDashboardVolume, 0, period.Days)
	for index := 0; index < period.Days; index++ {
		date := period.From.AddDate(0, 0, index)
		key := date.Format("2006-01-02")
		item := reactDashboardVolume{Date: key, Label: date.Format("02/01")}
		if row, ok := byDay[key]; ok {
			item.Recebidas = row.Recebidas
			item.Enviadas = row.Enviadas
		}
		out = append(out, item)
	}
	return out
}

func reactDashboardPermissionsFor(user *store.Usuario) reactDashboardPermissions {
	return reactDashboardPermissions{
		Reports:  userHasPerm(user, permRelatoriosVer),
		Failures: userHasPerm(user, permAprendizadoVer),
	}
}

func dashboardRate(part, total int) float64 {
	if total <= 0 {
		return 0
	}
	return math.Round(float64(part)/float64(total)*1000) / 10
}

func mapDashboardSectors(items []store.DashboardSetor) []reactDashboardSector {
	out := make([]reactDashboardSector, 0, len(items))
	for _, item := range items {
		out = append(out, reactDashboardSector{ID: item.ID, Name: item.Nome, Conversations: item.Conversas, Waiting: item.Aguardando})
	}
	return out
}

func mapDashboardAttendants(items []store.DashboardAtendente) []reactDashboardAttendant {
	out := make([]reactDashboardAttendant, 0, len(items))
	for _, item := range items {
		out = append(out, reactDashboardAttendant{ID: item.ID, Name: item.Nome, Active: item.Ativas, Waiting: item.Aguardando})
	}
	return out
}
