package channel

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

// Webhook é o canal HTTP genérico (qualquer origem pode enviar mensagens).
type Webhook struct {
	proc *Processor
}

type inbound struct {
	Message string `json:"message"`
	Phone   string `json:"phone"`
	Name    string `json:"name"`
	AgentID *int   `json:"agent_id"`
}

type outbound struct {
	ConversationID int      `json:"conversation_id"`
	Status         string   `json:"status"`
	Reply          string   `json:"reply"`
	UsedTools      []string `json:"used_tools,omitempty"`
}

func NewWebhook(proc *Processor) *Webhook {
	return &Webhook{proc: proc}
}

func (w *Webhook) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	tenantID, err := store.TenantIDRequired(r.Context())
	if err != nil {
		http.Error(rw, `{"error":"tenant context required"}`, http.StatusUnauthorized)
		return
	}

	var in inbound
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		http.Error(rw, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	in.Message = strings.TrimSpace(in.Message)
	if in.Message == "" {
		http.Error(rw, `{"error":"mensagem vazia"}`, http.StatusBadRequest)
		return
	}

	res, err := w.proc.Process(r.Context(), Inbound{
		Message: in.Message, Phone: in.Phone, Name: in.Name, AgentID: in.AgentID, TenantID: tenantID,
	})
	if err != nil {
		Logger.Printf("erro no webhook genérico (%s): %v", in.Phone, err)
		http.Error(rw, `{"error":"erro interno"}`, http.StatusInternalServerError)
		return
	}
	json.NewEncoder(rw).Encode(outbound{
		ConversationID: res.ConversationID,
		Status:         res.Status,
		Reply:          res.Reply,
		UsedTools:      res.UsedTools,
	})
}
