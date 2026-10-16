package web

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

type reactReportsSummary struct {
	Conversations          int     `json:"conversations"`
	Active                 int     `json:"active"`
	Waiting                int     `json:"waiting"`
	Closed                 int     `json:"closed"`
	Messages               int     `json:"messages"`
	CustomerMessages       int     `json:"customer_messages"`
	AIMessages             int     `json:"ai_messages"`
	HumanMessages          int     `json:"human_messages"`
	EvaluatedConversations int     `json:"evaluated_conversations"`
	RatingCount            int     `json:"rating_count"`
	OverallRating          float64 `json:"overall_rating"`
	OverallRatingCount     int     `json:"overall_rating_count"`
	AIRating               float64 `json:"ai_rating"`
	AIRatingCount          int     `json:"ai_rating_count"`
	HumanRating            float64 `json:"human_rating"`
	HumanRatingCount       int     `json:"human_rating_count"`
	AvgResponseTime        float64 `json:"avg_response_time"`
	AvgResponseTimeCount   int     `json:"avg_response_time_count"`
	TransferRate           float64 `json:"transfer_rate"`
	TransferredCount       int     `json:"transferred_count"`
	NPS                    float64 `json:"nps"`
	NPSPromoters           int     `json:"nps_promoters"`
	NPSDetractors          int     `json:"nps_detractors"`
	NPSCount               int     `json:"nps_count"`
	QueueAbandonmentRate   float64 `json:"queue_abandonment_rate"`
	QueueAbandonmentCount  int     `json:"queue_abandonment_count"`
	AvgWaitTime            float64 `json:"avg_wait_time"`
	AvgWaitTimeCount       int     `json:"avg_wait_time_count"`
	AvgHandlingTime        float64 `json:"avg_handling_time"`
	AvgHandlingTimeCount   int     `json:"avg_handling_time_count"`
	CSAT                   float64 `json:"csat"`
	CSATSatisfied          int     `json:"csat_satisfied"`
	CSATCount              int     `json:"csat_count"`
	ResolutionRate         float64 `json:"resolution_rate"`
	ResolvedCount          int     `json:"resolved_count"`
	AIResolvedCount        int     `json:"ai_resolved_count"`
	HumanResolvedCount     int     `json:"human_resolved_count"`
	AIResolutionRate       float64 `json:"ai_resolution_rate"`
	HumanResolutionRate    float64 `json:"human_resolution_rate"`
}

type reactReportsFilters struct {
	Period      string `json:"period"`
	From        string `json:"from"`
	To          string `json:"to"`
	Channel     string `json:"channel,omitempty"`
	SectorID    *int   `json:"sector_id,omitempty"`
	AttendantID *int   `json:"attendant_id,omitempty"`
}

type reactReportOption struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type reactReportsOptions struct {
	Sectors    []reactReportOption `json:"sectors"`
	Attendants []reactReportOption `json:"attendants"`
}

type reactReportDaily struct {
	Date          string `json:"date"`
	Label         string `json:"label"`
	Conversations int    `json:"conversations"`
	Messages      int    `json:"messages"`
	Evaluations   int    `json:"evaluations"`
}

type reactReportAttendant struct {
	ID            *int    `json:"id,omitempty"`
	Name          string  `json:"name"`
	Conversations int     `json:"conversations"`
	HumanMessages int     `json:"human_messages"`
	Evaluations   int     `json:"evaluations"`
	Rating        float64 `json:"rating"`
	RatingCount   int     `json:"rating_count"`
	AIRating      float64 `json:"ai_rating"`
	AIRatingCount int     `json:"ai_rating_count"`
}

type reactReportSector struct {
	ID               *int    `json:"id,omitempty"`
	Name             string  `json:"name"`
	Conversations    int     `json:"conversations"`
	Evaluations      int     `json:"evaluations"`
	Rating           float64 `json:"rating"`
	RatingCount      int     `json:"rating_count"`
	AIRating         float64 `json:"ai_rating"`
	AIRatingCount    int     `json:"ai_rating_count"`
	HumanRating      float64 `json:"human_rating"`
	HumanRatingCount int     `json:"human_rating_count"`
}

type reactReportRatingDistribution struct {
	Rating int `json:"rating"`
	AI     int `json:"ai"`
	Human  int `json:"human"`
	Total  int `json:"total"`
}

type reactReportEvaluation struct {
	ConversationID int       `json:"conversation_id"`
	Protocol       string    `json:"protocol"`
	Channel        string    `json:"channel"`
	Sector         string    `json:"sector"`
	Attendant      string    `json:"attendant"`
	OverallRating  *float64  `json:"overall_rating,omitempty"`
	AIRating       *int      `json:"ai_rating,omitempty"`
	HumanRating    *int      `json:"human_rating,omitempty"`
	EvaluatedAt    time.Time `json:"evaluated_at"`
}

type reactReportDetail struct {
	ConversationID   int       `json:"conversation_id"`
	Protocol         string    `json:"protocol"`
	Channel          string    `json:"channel"`
	Sector           string    `json:"sector"`
	Attendant        string    `json:"attendant"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	Messages         int       `json:"messages"`
	CustomerMessages int       `json:"customer_messages"`
	AIMessages       int       `json:"ai_messages"`
	HumanMessages    int       `json:"human_messages"`
	WaitSeconds      float64   `json:"wait_seconds"`
	HandlingSeconds  float64   `json:"handling_seconds"`
	OverallRating    *float64  `json:"overall_rating,omitempty"`
	AIRating         *int      `json:"ai_rating,omitempty"`
	HumanRating      *int      `json:"human_rating,omitempty"`
}

type reactReportsData struct {
	Filters     reactReportsFilters             `json:"filters"`
	Options     reactReportsOptions             `json:"options"`
	Summary     reactReportsSummary             `json:"summary"`
	Daily       []reactReportDaily              `json:"daily"`
	ByAttendant []reactReportAttendant          `json:"by_attendant"`
	BySector    []reactReportSector             `json:"by_sector"`
	Ratings     []reactReportRatingDistribution `json:"ratings"`
	Evaluations []reactReportEvaluation         `json:"evaluations"`
	Details     []reactReportDetail             `json:"details,omitempty"`
}

type reactReportsMeta struct {
	GeneratedAt          time.Time `json:"generated_at"`
	EvaluationTotal      int       `json:"evaluation_total"`
	EvaluationLimit      int       `json:"evaluation_limit"`
	EvaluationOffset     int       `json:"evaluation_offset"`
	EvaluationHasMore    bool      `json:"evaluation_has_more"`
	EvaluationNextCursor string    `json:"evaluation_next_cursor,omitempty"`
	EvaluationTruncated  bool      `json:"evaluation_truncated"`
}

type reactReportFilter struct {
	Period      string
	From        time.Time
	To          time.Time
	Channel     string
	SectorID    *int
	AttendantID *int
}

type reactReportRatingTotals struct {
	Sum   int
	Count int
}

type reactReportAttendantTotals struct {
	ID       *int
	Name     string
	Convs    int
	Messages int
	Human    reactReportRatingTotals
	AI       reactReportRatingTotals
}

type reactReportSectorTotals struct {
	ID    *int
	Name  string
	Convs int
	Human reactReportRatingTotals
	AI    reactReportRatingTotals
}

func (s *Server) reactReportsEndpoint() http.Handler {
	return s.authJSON(s.requireReactPerm(permRelatoriosVer, s.requireReactModule("relatorios", http.HandlerFunc(s.handleReactReports))))
}

func (s *Server) handleReactReports(w http.ResponseWriter, r *http.Request) {
	filter, err := parseReactReportFilter(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_filter", err.Error())
		return
	}
	page, err := parseReactPage(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}

	data, meta, err := s.buildReactReports(r.Context(), filter)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "reports_unavailable", "não foi possível carregar os relatórios")
		return
	}
	total := len(data.Evaluations)
	pageItems, err := reactPageSlice(data.Evaluations, page.Limit, page.Offset)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_cursor", err.Error())
		return
	}
	data.Evaluations = pageItems
	data.Details = nil
	meta.EvaluationTotal = total
	meta.EvaluationLimit = page.Limit
	meta.EvaluationOffset = page.Offset
	meta.EvaluationHasMore = page.Offset+len(pageItems) < total
	if meta.EvaluationHasMore {
		meta.EvaluationNextCursor = encodeReactCursor(page.Offset + len(pageItems))
	}
	meta.EvaluationTruncated = meta.EvaluationHasMore
	writeReactResponse(w, http.StatusOK, data, meta)
}

func (s *Server) handleReactReportsExport(w http.ResponseWriter, r *http.Request) {
	filter, err := parseReactReportFilter(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_filter", err.Error())
		return
	}
	data, _, err := s.buildReactReports(r.Context(), filter)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "reports_unavailable", "não foi possível exportar os relatórios")
		return
	}
	var buffer bytes.Buffer
	if err := writeReactReportsCSV(&buffer, data); err != nil {
		writeReactError(w, http.StatusInternalServerError, "reports_export_failed", "não foi possível gerar o CSV")
		return
	}
	filename := "relatorios-" + data.Filters.From + "-a-" + data.Filters.To + ".csv"
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buffer.Bytes())
}

func parseReactReportFilter(r *http.Request) (reactReportFilter, error) {
	query := r.URL.Query()
	period := strings.ToLower(strings.TrimSpace(query.Get("period")))
	if period == "" {
		period = strings.ToLower(strings.TrimSpace(query.Get("periodo")))
	}
	if period == "" {
		period = "7"
	}

	location := brtz()
	now := time.Now().In(location)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	end := now
	switch period {
	case "hoje", "today", "1", "1d":
		period = "1"
		end = start.AddDate(0, 0, 1)
	case "7", "7d", "semana", "week":
		period = "7"
		start = start.AddDate(0, 0, -6)
		end = start.AddDate(0, 0, 7)
	case "30", "30d", "mes", "month":
		period = "30"
		start = start.AddDate(0, 0, -29)
		end = start.AddDate(0, 0, 30)
	default:
		from := strings.TrimSpace(query.Get("from"))
		to := strings.TrimSpace(query.Get("to"))
		if from == "" || to == "" {
			return reactReportFilter{}, errors.New("período inválido")
		}
		parsedFrom, err := time.ParseInLocation("2006-01-02", from, location)
		if err != nil {
			return reactReportFilter{}, errors.New("data inicial inválida")
		}
		parsedTo, err := time.ParseInLocation("2006-01-02", to, location)
		if err != nil {
			return reactReportFilter{}, errors.New("data final inválida")
		}
		if parsedTo.Before(parsedFrom) || parsedTo.Sub(parsedFrom) > 30*24*time.Hour {
			return reactReportFilter{}, errors.New("o período deve ter no máximo 31 dias")
		}
		period = "custom"
		start = parsedFrom
		end = parsedTo.AddDate(0, 0, 1)
	}

	channel := strings.ToLower(strings.TrimSpace(query.Get("channel")))
	if len(channel) > 40 {
		return reactReportFilter{}, errors.New("canal inválido")
	}

	sectorID, err := optionalPositiveInt(query.Get("sector_id"))
	if err != nil {
		return reactReportFilter{}, errors.New("setor inválido")
	}
	attendantID, err := optionalPositiveInt(query.Get("attendant_id"))
	if err != nil {
		return reactReportFilter{}, errors.New("atendente inválido")
	}

	return reactReportFilter{Period: period, From: start, To: end, Channel: channel, SectorID: sectorID, AttendantID: attendantID}, nil
}

func optionalPositiveInt(value string) (*int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 {
		return nil, errors.New("valor inválido")
	}
	return &n, nil
}

func (s *Server) buildReactReports(ctx context.Context, filter reactReportFilter) (reactReportsData, reactReportsMeta, error) {
	conversations, err := s.st.ListConversationsFiltered(ctx, "")
	if err != nil {
		return reactReportsData{}, reactReportsMeta{}, err
	}
	sectors, err := s.st.ListSetores(ctx)
	if err != nil {
		return reactReportsData{}, reactReportsMeta{}, err
	}
	atendantes, err := s.st.ListAtendentes(ctx)
	if err != nil {
		return reactReportsData{}, reactReportsMeta{}, err
	}

	location := filter.From.Location()
	if location == nil {
		location = brtz()
	}
	days := int(filter.To.Sub(filter.From) / (24 * time.Hour))
	if days < 1 {
		days = 1
	}
	daily := make([]reactReportDaily, 0, days)
	dailyByDate := make(map[string]*reactReportDaily, days)
	for index := 0; index < days; index++ {
		date := filter.From.AddDate(0, 0, index).In(location)
		key := date.Format("2006-01-02")
		item := &reactReportDaily{Date: key, Label: date.Format("02/01")}
		daily = append(daily, *item)
		dailyByDate[key] = &daily[len(daily)-1]
	}

	attendantTotals := map[string]*reactReportAttendantTotals{}
	sectorTotals := map[string]*reactReportSectorTotals{}
	ratingTotals := make([]reactReportRatingDistribution, 6)
	for rating := 1; rating <= 5; rating++ {
		ratingTotals[rating] = reactReportRatingDistribution{Rating: rating}
	}
	summary := reactReportsSummary{}
	evaluations := make([]reactReportEvaluation, 0)
	overall := reactReportRatingTotals{}
	aiOverall := reactReportRatingTotals{}
	humanOverall := reactReportRatingTotals{}
	tmaSumSeconds := 0.0
	tmaCount := 0
	waitSumSeconds := 0.0
	waitCount := 0
	handlingSumSeconds := 0.0
	handlingCount := 0
	transferredCount := 0
	abandonedCount := 0
	promoters := 0
	detractors := 0
	npsCount := 0
	csatSatisfied := 0
	csatCount := 0
	resolvedCount := 0
	aiResolvedCount := 0
	humanResolvedCount := 0
	details := make([]reactReportDetail, 0)

	for _, conversation := range conversations {
		if !reactReportConversationMatches(conversation, filter) {
			continue
		}
		conversationInPeriod := reactReportInRange(conversation.CreatedAt, filter.From, filter.To)
		if conversationInPeriod {
			summary.Conversations++
			if conversation.Status == "closed" {
				summary.Closed++
			} else {
				summary.Active++
			}
			if conversation.Status == "humano_aguardando" {
				summary.Waiting++
			}
			day := conversation.CreatedAt.In(location).Format("2006-01-02")
			if item := dailyByDate[day]; item != nil {
				item.Conversations++
			}
		}

		var messages []store.Message
		if conversationInPeriod {
			messages, err = s.st.ListMessages(ctx, conversation.ID)
			if err != nil {
				return reactReportsData{}, reactReportsMeta{}, err
			}
		}
		attendantIDKey := attendantKey(conversation)
		sectorIDKey := sectorKey(conversation)
		var attendant *reactReportAttendantTotals
		var sector *reactReportSectorTotals
		var stats reactReportConversationStats
		if conversationInPeriod {
			attendant = attendantTotals[attendantIDKey]
			if attendant == nil {
				attendant = reactReportAttendantFor(conversation)
				attendantTotals[attendantIDKey] = attendant
			}
			sector = sectorTotals[sectorIDKey]
			if sector == nil {
				sector = reactReportSectorFor(conversation)
				sectorTotals[sectorIDKey] = sector
			}
			attendant.Convs++
			sector.Convs++
			stats = conversationReportStats(messages, filter)
			for _, message := range messages {
				if !reactReportInRange(message.CreatedAt, filter.From, filter.To) {
					continue
				}
				summary.Messages++
				switch strings.ToLower(message.Source) {
				case "customer", "user":
					summary.CustomerMessages++
				case "ai", "assistant":
					summary.AIMessages++
				case "human", "operator":
					summary.HumanMessages++
				}
				day := message.CreatedAt.In(location).Format("2006-01-02")
				if item := dailyByDate[day]; item != nil {
					item.Messages++
				}
				if strings.EqualFold(message.Source, "human") || strings.EqualFold(message.Source, "operator") {
					attendant.Messages++
				}
			}
			if responseDelta := conversationFirstResponseSeconds(messages, filter); responseDelta.Started {
				tmaCount++
				tmaSumSeconds += responseDelta.Seconds
			}
			if stats.Wait.Started {
				waitCount++
				waitSumSeconds += stats.Wait.Seconds
			}
			if stats.Handling.Started {
				handlingCount++
				handlingSumSeconds += stats.Handling.Seconds
			}
			if stats.HasHuman || conversation.AtendenteID != nil || conversation.Status == "humano_aguardando" || conversation.Status == "humano_atendendo" {
				transferredCount++
			}
			if conversation.Status == "closed" {
				resolvedCount++
				if stats.HasHuman || conversation.AtendenteID != nil {
					humanResolvedCount++
				} else {
					aiResolvedCount++
				}
			}
			if conversation.Status == "humano_aguardando" ||
				(conversation.Status == "closed" &&
					(conversation.Satisfacao == "sem_resposta" || conversation.Satisfacao == "aguardando" || conversation.Satisfacao == "aguardando_operador") &&
					conversation.AtendenteID == nil && !stats.HasHuman) {
				abandonedCount++
			}
		}

		assessment, assessmentErr := s.st.GetAvaliacaoByConversation(ctx, conversation.ID)
		if assessmentErr != nil && !errors.Is(assessmentErr, pgx.ErrNoRows) {
			return reactReportsData{}, reactReportsMeta{}, assessmentErr
		}
		var overallRating *float64
		var aiRating *int
		var humanRating *int
		evaluationInPeriod := assessmentErr == nil && reactReportInRange(assessment.CreatedAt, filter.From, filter.To) && (assessment.NotaIA != nil || assessment.NotaOperador != nil)
		if evaluationInPeriod {
			if attendant == nil {
				attendant = attendantTotals[attendantIDKey]
				if attendant == nil {
					attendant = reactReportAttendantFor(conversation)
					attendantTotals[attendantIDKey] = attendant
				}
			}
			if sector == nil {
				sector = sectorTotals[sectorIDKey]
				if sector == nil {
					sector = reactReportSectorFor(conversation)
					sectorTotals[sectorIDKey] = sector
				}
			}
			summary.EvaluatedConversations++
			if item := dailyByDate[assessment.CreatedAt.In(location).Format("2006-01-02")]; item != nil {
				item.Evaluations++
			}
			if assessment.NotaIA != nil {
				aiOverall.Sum += *assessment.NotaIA
				aiOverall.Count++
				attendant.AI.Sum += *assessment.NotaIA
				attendant.AI.Count++
				sector.AI.Sum += *assessment.NotaIA
				sector.AI.Count++
				aiRating = assessment.NotaIA
				if *assessment.NotaIA >= 1 && *assessment.NotaIA <= 5 {
					ratingTotals[*assessment.NotaIA].AI++
					ratingTotals[*assessment.NotaIA].Total++
					npsCount++
					if *assessment.NotaIA >= 4 {
						promoters++
					}
					if *assessment.NotaIA <= 2 {
						detractors++
					}
				}
			}
			if assessment.NotaOperador != nil {
				humanOverall.Sum += *assessment.NotaOperador
				humanOverall.Count++
				attendant.Human.Sum += *assessment.NotaOperador
				attendant.Human.Count++
				sector.Human.Sum += *assessment.NotaOperador
				sector.Human.Count++
				humanRating = assessment.NotaOperador
				if *assessment.NotaOperador >= 1 && *assessment.NotaOperador <= 5 {
					ratingTotals[*assessment.NotaOperador].Human++
					ratingTotals[*assessment.NotaOperador].Total++
					npsCount++
					if *assessment.NotaOperador >= 4 {
						promoters++
					}
					if *assessment.NotaOperador <= 2 {
						detractors++
					}
				}
			}
			if assessment.NotaIA != nil {
				overall.Sum += *assessment.NotaIA
				overall.Count++
			}
			if assessment.NotaOperador != nil {
				overall.Sum += *assessment.NotaOperador
				overall.Count++
			}
			overallRating = reactReportOverallRating(assessment)
			csatSatisfied += reactReportSatisfiedRatings(assessment)
			csatCount += reactReportRatingCount(assessment)
			evaluations = append(evaluations, reactReportEvaluationFor(conversation, assessment))
		}
		if conversationInPeriod {
			details = append(details, reactReportDetailFor(conversation, stats, overallRating, aiRating, humanRating))
		}
	}

	summary.RatingCount = overall.Count
	summary.OverallRating = reactReportAverage(overall)
	summary.OverallRatingCount = overall.Count
	summary.AIRating = reactReportAverage(aiOverall)
	summary.AIRatingCount = aiOverall.Count
	summary.HumanRating = reactReportAverage(humanOverall)
	summary.HumanRatingCount = humanOverall.Count
	if tmaCount > 0 {
		summary.AvgResponseTime = math.Round((tmaSumSeconds/float64(tmaCount))*10) / 10
	}
	summary.AvgResponseTimeCount = tmaCount
	summary.TransferredCount = transferredCount
	if summary.Conversations > 0 {
		summary.TransferRate = math.Round(float64(transferredCount)/float64(summary.Conversations)*1000) / 10
		summary.QueueAbandonmentRate = math.Round(float64(abandonedCount)/float64(summary.Conversations)*1000) / 10
	}
	summary.NPSPromoters = promoters
	summary.NPSDetractors = detractors
	summary.NPSCount = npsCount
	if npsCount > 0 {
		summary.NPS = math.Round((float64(promoters)-float64(detractors))/float64(npsCount)*1000) / 10
	}
	summary.QueueAbandonmentCount = abandonedCount
	if waitCount > 0 {
		summary.AvgWaitTime = math.Round((waitSumSeconds/float64(waitCount))*10) / 10
	}
	summary.AvgWaitTimeCount = waitCount
	if handlingCount > 0 {
		summary.AvgHandlingTime = math.Round((handlingSumSeconds/float64(handlingCount))*10) / 10
	}
	summary.AvgHandlingTimeCount = handlingCount
	summary.CSATCount = csatCount
	summary.CSATSatisfied = csatSatisfied
	if csatCount > 0 {
		summary.CSAT = math.Round(float64(csatSatisfied)/float64(csatCount)*1000) / 10
	}
	summary.ResolvedCount = resolvedCount
	summary.AIResolvedCount = aiResolvedCount
	summary.HumanResolvedCount = humanResolvedCount
	if resolvedCount > 0 {
		summary.AIResolutionRate = math.Round(float64(aiResolvedCount)/float64(resolvedCount)*1000) / 10
		summary.HumanResolutionRate = math.Round(float64(humanResolvedCount)/float64(resolvedCount)*1000) / 10
	}
	if summary.Conversations > 0 {
		summary.ResolutionRate = math.Round(float64(resolvedCount)/float64(summary.Conversations)*1000) / 10
	}

	data := reactReportsData{
		Filters:     reactReportsFilters{Period: filter.Period, From: filter.From.Format("2006-01-02"), To: filter.To.AddDate(0, 0, -1).Format("2006-01-02"), Channel: filter.Channel, SectorID: filter.SectorID, AttendantID: filter.AttendantID},
		Options:     reactReportsOptions{Sectors: reportSectorOptions(sectors), Attendants: reportAttendantOptions(atendantes)},
		Summary:     summary,
		Daily:       daily,
		ByAttendant: reportAttendantRows(attendantTotals),
		BySector:    reportSectorRows(sectorTotals),
		Ratings:     ratingTotals[1:],
		Evaluations: evaluations,
		Details:     details,
	}
	meta := reactReportsMeta{GeneratedAt: time.Now().In(location), EvaluationTotal: len(evaluations)}
	return data, meta, nil
}

func reactReportConversationMatches(conversation store.Conversation, filter reactReportFilter) bool {
	if filter.Channel != "" && !strings.EqualFold(conversation.Channel, filter.Channel) {
		return false
	}
	if filter.SectorID != nil && (conversation.SetorID == nil || *conversation.SetorID != *filter.SectorID) {
		return false
	}
	if filter.AttendantID != nil && (conversation.AtendenteID == nil || *conversation.AtendenteID != *filter.AttendantID) {
		return false
	}
	return true
}

func reactReportInRange(value, from, to time.Time) bool {
	return !value.Before(from) && value.Before(to)
}

func reactReportAverage(total reactReportRatingTotals) float64 {
	if total.Count == 0 {
		return 0
	}
	return math.Round((float64(total.Sum)/float64(total.Count))*10) / 10
}

type reactReportTiming struct {
	Started bool
	Seconds float64
}

type reactReportConversationStats struct {
	Messages int
	Customer int
	AI       int
	Human    int
	HasHuman bool
	Wait     reactReportTiming
	Handling reactReportTiming
}

func conversationReportStats(messages []store.Message, filter reactReportFilter) reactReportConversationStats {
	stats := reactReportConversationStats{}
	var firstCustomer *time.Time
	var firstHuman *time.Time
	var lastMessage *time.Time
	for _, message := range messages {
		if !reactReportInRange(message.CreatedAt, filter.From, filter.To) {
			continue
		}
		stats.Messages++
		at := message.CreatedAt
		if lastMessage == nil || at.After(*lastMessage) {
			lastMessage = &at
		}
		switch strings.ToLower(message.Source) {
		case "customer", "user":
			stats.Customer++
			if firstCustomer == nil {
				firstCustomer = &at
			}
		case "ai", "assistant":
			stats.AI++
		case "human", "operator":
			stats.Human++
			if firstHuman == nil {
				firstHuman = &at
			}
		}
	}
	stats.HasHuman = stats.Human > 0
	if firstCustomer != nil && firstHuman != nil && firstHuman.After(*firstCustomer) {
		stats.Wait = reactReportTiming{Started: true, Seconds: firstHuman.Sub(*firstCustomer).Seconds()}
	}
	if firstHuman != nil && lastMessage != nil && lastMessage.After(*firstHuman) {
		stats.Handling = reactReportTiming{Started: true, Seconds: lastMessage.Sub(*firstHuman).Seconds()}
	}
	return stats
}

func conversationFirstResponseSeconds(messages []store.Message, filter reactReportFilter) reactReportTiming {
	var lastCustomer *time.Time
	for _, message := range messages {
		if !reactReportInRange(message.CreatedAt, filter.From, filter.To) {
			continue
		}
		switch strings.ToLower(message.Source) {
		case "customer", "user":
			at := message.CreatedAt
			lastCustomer = &at
		case "ai", "assistant", "human", "operator":
			if lastCustomer != nil && message.CreatedAt.After(*lastCustomer) {
				seconds := message.CreatedAt.Sub(*lastCustomer).Seconds()
				if seconds >= 0 {
					return reactReportTiming{Started: true, Seconds: seconds}
				}
			}
		}
	}
	return reactReportTiming{Started: false}
}

func reactReportAttendantFor(conversation store.Conversation) *reactReportAttendantTotals {
	var id *int
	if conversation.AtendenteID != nil {
		value := *conversation.AtendenteID
		id = &value
	}
	name := strings.TrimSpace(conversation.AtendenteName)
	if name == "" {
		name = "Não atribuído"
	}
	return &reactReportAttendantTotals{ID: id, Name: name}
}

func reactReportSectorFor(conversation store.Conversation) *reactReportSectorTotals {
	var id *int
	if conversation.SetorID != nil {
		value := *conversation.SetorID
		id = &value
	}
	name := strings.TrimSpace(conversation.SetorName)
	if name == "" {
		name = "Sem setor"
	}
	return &reactReportSectorTotals{ID: id, Name: name}
}

func attendantKey(conversation store.Conversation) string {
	if conversation.AtendenteID != nil {
		return "id:" + strconv.Itoa(*conversation.AtendenteID)
	}
	return "name:" + reactReportAttendantFor(conversation).Name
}

func sectorKey(conversation store.Conversation) string {
	if conversation.SetorID != nil {
		return "id:" + strconv.Itoa(*conversation.SetorID)
	}
	return "name:" + reactReportSectorFor(conversation).Name
}

func reactReportEvaluationFor(conversation store.Conversation, assessment store.Avaliacao) reactReportEvaluation {
	item := reactReportEvaluation{ConversationID: conversation.ID, Protocol: "#T-" + strconv.Itoa(conversation.ID), Channel: conversation.Channel, Sector: reactReportSectorFor(conversation).Name, Attendant: reactReportAttendantFor(conversation).Name, EvaluatedAt: assessment.CreatedAt}
	if assessment.NotaIA != nil {
		value := *assessment.NotaIA
		item.AIRating = &value
	}
	if assessment.NotaOperador != nil {
		value := *assessment.NotaOperador
		item.HumanRating = &value
	}
	item.OverallRating = reactReportOverallRating(assessment)
	return item
}

func reactReportOverallRating(assessment store.Avaliacao) *float64 {
	total := reactReportRatingTotals{}
	if assessment.NotaIA != nil {
		total.Sum += *assessment.NotaIA
		total.Count++
	}
	if assessment.NotaOperador != nil {
		total.Sum += *assessment.NotaOperador
		total.Count++
	}
	if total.Count == 0 {
		return nil
	}
	value := reactReportAverage(total)
	return &value
}

func reactReportRatingCount(assessment store.Avaliacao) int {
	count := 0
	if assessment.NotaIA != nil {
		count++
	}
	if assessment.NotaOperador != nil {
		count++
	}
	return count
}

func reactReportSatisfiedRatings(assessment store.Avaliacao) int {
	count := 0
	if assessment.NotaIA != nil && *assessment.NotaIA >= 4 {
		count++
	}
	if assessment.NotaOperador != nil && *assessment.NotaOperador >= 4 {
		count++
	}
	return count
}

func reactReportDetailFor(conversation store.Conversation, stats reactReportConversationStats, overall *float64, ai, human *int) reactReportDetail {
	return reactReportDetail{
		ConversationID:   conversation.ID,
		Protocol:         "#T-" + strconv.Itoa(conversation.ID),
		Channel:          conversation.Channel,
		Sector:           reactReportSectorFor(conversation).Name,
		Attendant:        reactReportAttendantFor(conversation).Name,
		Status:           conversation.Status,
		CreatedAt:        conversation.CreatedAt,
		Messages:         stats.Messages,
		CustomerMessages: stats.Customer,
		AIMessages:       stats.AI,
		HumanMessages:    stats.Human,
		WaitSeconds:      math.Round(stats.Wait.Seconds*10) / 10,
		HandlingSeconds:  math.Round(stats.Handling.Seconds*10) / 10,
		OverallRating:    overall,
		AIRating:         ai,
		HumanRating:      human,
	}
}

func reportSectorOptions(items []store.Setor) []reactReportOption {
	options := make([]reactReportOption, 0, len(items))
	for _, item := range items {
		options = append(options, reactReportOption{ID: item.ID, Name: item.Name})
	}
	return options
}

func reportAttendantOptions(items []store.Atendente) []reactReportOption {
	options := make([]reactReportOption, 0, len(items))
	for _, item := range items {
		options = append(options, reactReportOption{ID: item.ID, Name: item.Name})
	}
	return options
}

func reportAttendantRows(items map[string]*reactReportAttendantTotals) []reactReportAttendant {
	rows := make([]reactReportAttendant, 0, len(items))
	for _, item := range items {
		rows = append(rows, reactReportAttendant{ID: item.ID, Name: item.Name, Conversations: item.Convs, HumanMessages: item.Messages, Evaluations: item.Human.Count, Rating: reactReportAverage(item.Human), RatingCount: item.Human.Count, AIRating: reactReportAverage(item.AI), AIRatingCount: item.AI.Count})
	}
	sortReactAttendants(rows)
	return rows
}

func reportSectorRows(items map[string]*reactReportSectorTotals) []reactReportSector {
	rows := make([]reactReportSector, 0, len(items))
	for _, item := range items {
		rows = append(rows, reactReportSector{ID: item.ID, Name: item.Name, Conversations: item.Convs, Evaluations: item.Human.Count + item.AI.Count, Rating: reactReportAverage(reactReportRatingTotals{Sum: item.Human.Sum + item.AI.Sum, Count: item.Human.Count + item.AI.Count}), RatingCount: item.Human.Count + item.AI.Count, AIRating: reactReportAverage(item.AI), AIRatingCount: item.AI.Count, HumanRating: reactReportAverage(item.Human), HumanRatingCount: item.Human.Count})
	}
	sortReactSectors(rows)
	return rows
}

func sortReactAttendants(rows []reactReportAttendant) {
	for index := 1; index < len(rows); index++ {
		current := rows[index]
		position := index
		for position > 0 && reportAttendantBefore(current, rows[position-1]) {
			rows[position] = rows[position-1]
			position--
		}
		rows[position] = current
	}
}

func reportAttendantBefore(left, right reactReportAttendant) bool {
	if left.RatingCount != right.RatingCount {
		return left.RatingCount > right.RatingCount
	}
	if left.Rating != right.Rating {
		return left.Rating > right.Rating
	}
	if left.Conversations != right.Conversations {
		return left.Conversations > right.Conversations
	}
	return strings.ToLower(left.Name) < strings.ToLower(right.Name)
}

func sortReactSectors(rows []reactReportSector) {
	for index := 1; index < len(rows); index++ {
		current := rows[index]
		position := index
		for position > 0 && reportSectorBefore(current, rows[position-1]) {
			rows[position] = rows[position-1]
			position--
		}
		rows[position] = current
	}
}

func reportSectorBefore(left, right reactReportSector) bool {
	if left.RatingCount != right.RatingCount {
		return left.RatingCount > right.RatingCount
	}
	if left.Rating != right.Rating {
		return left.Rating > right.Rating
	}
	if left.Conversations != right.Conversations {
		return left.Conversations > right.Conversations
	}
	return strings.ToLower(left.Name) < strings.ToLower(right.Name)
}

var reactReportsCSVHeader = []string{"Protocolo", "Canal", "Setor", "Atendente", "Status", "Criado em", "Mensagens", "Mensagens do cliente", "Mensagens da IA", "Mensagens humanas", "Espera (s)", "Atendimento (s)", "Nota geral", "Nota IA", "Nota humano"}

func writeReactReportsCSV(w io.Writer, data reactReportsData) error {
	writer := csv.NewWriter(w)
	if err := writer.WriteAll(reactReportsCSVRows(data)); err != nil {
		return err
	}
	return writer.Error()
}

func reactReportsCSVRows(data reactReportsData) [][]string {
	width := len(reactReportsCSVHeader)
	rows := [][]string{
		reactReportsCSVPad([]string{"Relatório", "Notreve Chat Community"}, width),
		reactReportsCSVPad([]string{"Período", data.Filters.From + " a " + data.Filters.To}, width),
		reactReportsCSVPad([]string{"Canal", reactReportsCSVText(data.Filters.Channel, "Todos")}, width),
		reactReportsCSVPad(nil, width),
		reactReportsCSVPad([]string{"Resumo", "Valor"}, width),
		reactReportsCSVPad([]string{"Conversas", strconv.Itoa(data.Summary.Conversations)}, width),
		reactReportsCSVPad([]string{"Encerradas", strconv.Itoa(data.Summary.Closed)}, width),
		reactReportsCSVPad([]string{"Taxa de encerramento (%)", reactReportsCSVNumber(data.Summary.ResolutionRate)}, width),
		reactReportsCSVPad([]string{"Resolvidas pela IA", strconv.Itoa(data.Summary.AIResolvedCount)}, width),
		reactReportsCSVPad([]string{"Resolvidas por humano", strconv.Itoa(data.Summary.HumanResolvedCount)}, width),
		reactReportsCSVPad([]string{"Tempo médio de espera (s)", reactReportsCSVNumber(data.Summary.AvgWaitTime)}, width),
		reactReportsCSVPad([]string{"Tempo médio de atendimento (s)", reactReportsCSVNumber(data.Summary.AvgHandlingTime)}, width),
		reactReportsCSVPad([]string{"CSAT (%)", reactReportsCSVNumber(data.Summary.CSAT)}, width),
		reactReportsCSVPad([]string{"Nota geral", reactReportsCSVNumber(data.Summary.OverallRating)}, width),
		reactReportsCSVPad(nil, width),
		reactReportsCSVHeader,
	}
	for _, item := range data.Details {
		rows = append(rows, reactReportsCSVDetailRow(item))
	}
	return rows
}

func reactReportsCSVPad(row []string, width int) []string {
	if len(row) >= width {
		return row
	}
	padded := make([]string, width)
	copy(padded, row)
	return padded
}

func reactReportsCSVDetailRow(item reactReportDetail) []string {
	return []string{
		item.Protocol,
		item.Channel,
		item.Sector,
		item.Attendant,
		item.Status,
		item.CreatedAt.In(brtz()).Format("2006-01-02 15:04:05"),
		strconv.Itoa(item.Messages),
		strconv.Itoa(item.CustomerMessages),
		strconv.Itoa(item.AIMessages),
		strconv.Itoa(item.HumanMessages),
		reactReportsCSVNumber(item.WaitSeconds),
		reactReportsCSVNumber(item.HandlingSeconds),
		reactReportsCSVOptionalNumber(item.OverallRating),
		reactReportsCSVOptionalInt(item.AIRating),
		reactReportsCSVOptionalInt(item.HumanRating),
	}
}

func reactReportsCSVText(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func reactReportsCSVNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func reactReportsCSVOptionalNumber(value *float64) string {
	if value == nil {
		return ""
	}
	return reactReportsCSVNumber(*value)
}

func reactReportsCSVOptionalInt(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}
