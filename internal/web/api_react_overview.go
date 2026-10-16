package web

import (
	"net/http"
	"strings"
)

type reactOverviewSummary struct {
	Active   int `json:"active"`
	Waiting  int `json:"waiting"`
	AI       int `json:"ai"`
	Channels int `json:"channels"`
}

func (s *Server) handleReactOverview(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	conversations, err := s.st.ListConversationsForUser(r.Context(), "", user, s.userSetores(r.Context(), user), s.modoAberto(r.Context()), userHasPerm(user, permConversasVerIA))
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "overview_unavailable", "não foi possível carregar o resumo da operação")
		return
	}

	channels := map[string]struct{}{}
	var summary reactOverviewSummary
	for _, conversation := range conversations {
		if conversation.Status == "closed" {
			continue
		}
		summary.Active++
		if conversation.Status == "humano_aguardando" {
			summary.Waiting++
		}
		if conversation.Status == "ia" {
			summary.AI++
		}
		channel := strings.TrimSpace(conversation.Channel)
		if channel == "" {
			channel = strings.TrimSpace(conversation.Origem)
		}
		if channel != "" {
			channels[strings.ToLower(channel)] = struct{}{}
		}
	}
	summary.Channels = len(channels)
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"summary": summary}, map[string]interface{}{"scope": "workspace"})
}
