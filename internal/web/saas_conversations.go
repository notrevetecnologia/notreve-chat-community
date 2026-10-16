package web

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func saasReactConversationID(path string) (int, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 || parts[0] != "api" || parts[1] != "react" || parts[2] != "conversations" {
		return 0, false
	}
	id, err := strconv.Atoi(parts[3])
	return id, err == nil && id > 0
}

func saasReactConversationStatusID(path string) (int, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 5 || parts[0] != "api" || parts[1] != "react" || parts[2] != "conversations" || parts[4] != "status" {
		return 0, false
	}
	id, err := strconv.Atoi(parts[3])
	return id, err == nil && id > 0
}

func saasReactMessageMediaID(path string) (int, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 5 || parts[0] != "api" || parts[1] != "react" || parts[2] != "messages" || parts[4] != "media" {
		return 0, false
	}
	id, err := strconv.Atoi(parts[3])
	return id, err == nil && id > 0
}

func saasReactConversationMessageIDs(path string) (conversationID, messageID int, ok bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 6 || parts[0] != "api" || parts[1] != "react" || parts[2] != "conversations" || parts[4] != "messages" {
		return 0, 0, false
	}
	conversationID, err := strconv.Atoi(parts[3])
	if err != nil || conversationID < 1 {
		return 0, 0, false
	}
	messageID, err = strconv.Atoi(parts[5])
	if err != nil || messageID < 1 {
		return 0, 0, false
	}
	return conversationID, messageID, true
}

// dispatchSaaSCanonicalConversationRoute intercepta o contrato novo antes do
// roteador HTML legado. Cada rota ainda passa por authJSON, permissão e módulo.
func (s *Server) dispatchSaaSCanonicalConversationRoute(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := saasReactConversationID(r.URL.Path); ok && r.Method == http.MethodGet {
		s.authJSON(s.requireReactPerm(permConversasVer, s.requireReactModule("conversas", http.HandlerFunc(s.handleSaaSConversation)))).ServeHTTP(w, r)
		return true
	}
	if _, ok := saasReactConversationStatusID(r.URL.Path); ok && r.Method == http.MethodPatch {
		s.authJSON(s.requireReactPerm(permConversasEncerrar, s.requireReactModule("conversas", http.HandlerFunc(s.handleSaaSConversationStatus)))).ServeHTTP(w, r)
		return true
	}
	if _, ok := saasReactMessageMediaID(r.URL.Path); ok && r.Method == http.MethodGet {
		s.authJSON(s.requireReactPerm(permConversasVer, s.requireReactModule("conversas", http.HandlerFunc(s.handleSaaSMessageMedia)))).ServeHTTP(w, r)
		return true
	}
	if _, _, ok := saasReactConversationMessageIDs(r.URL.Path); ok && r.Method == http.MethodPatch {
		s.authJSON(s.requireReactPerm(permConversasResponder, s.requireReactModule("conversas", http.HandlerFunc(s.handleSaaSMessageEdit)))).ServeHTTP(w, r)
		return true
	}
	return false
}

func (s *Server) handleSaaSConversation(w http.ResponseWriter, r *http.Request) {
	id, ok := saasReactConversationID(r.URL.Path)
	if !ok {
		writeReactError(w, http.StatusBadRequest, "invalid_id", "id inválido")
		return
	}
	conversation, status, message := s.reactConversationForUser(r, id)
	if status != 0 {
		code := "not_found"
		if status >= 500 {
			code = "internal_error"
		}
		writeReactError(w, status, code, message)
		return
	}
	messages, err := s.st.ListMessages(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar as mensagens")
		return
	}
	items := make([]reactMessage, 0, len(messages))
	for _, message := range messages {
		item := reactMessageFromStore(message)
		if item.HasMedia {
			item.MediaURL = "/api/react/messages/" + strconv.Itoa(item.ID) + "/media"
		}
		items = append(items, item)
	}
	payload := map[string]interface{}{"conversation": conversation, "messages": items}
	if usage, err := s.st.ConversationLLMUsage(r.Context(), id); err == nil {
		payload["usage"] = usage
	}
	writeReactResponse(w, http.StatusOK, payload, nil)
}

func (s *Server) handleSaaSConversationStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := saasReactConversationStatusID(r.URL.Path)
	if !ok {
		writeReactError(w, http.StatusBadRequest, "invalid_id", "id inválido")
		return
	}
	var input struct {
		Status string `json:"status"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 8*1024+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido")
		return
	}
	input.Status = strings.TrimSpace(input.Status)
	switch input.Status {
	case "ia", "humano_aguardando", "humano_atendendo", "closed":
	default:
		writeReactError(w, http.StatusBadRequest, "invalid_status", "status inválido")
		return
	}
	body, _ := json.Marshal(map[string]string{"status": input.Status})
	r.SetPathValue("id", strconv.Itoa(id))
	result, status, message := s.callReactLegacyHandler(r, body, s.handleStatus)
	if status < 200 || status >= 300 {
		writeReactError(w, status, "status_update_failed", message)
		return
	}
	writeReactResponse(w, http.StatusOK, result, nil)
}

func (s *Server) handleSaaSMessageMedia(w http.ResponseWriter, r *http.Request) {
	id, ok := saasReactMessageMediaID(r.URL.Path)
	if !ok {
		writeReactError(w, http.StatusBadRequest, "invalid_id", "id inválido")
		return
	}
	message, err := s.st.GetMessage(r.Context(), id)
	if err != nil || message.MediaB64 == "" {
		writeReactError(w, http.StatusNotFound, "not_found", "mídia não encontrada")
		return
	}
	conversation, err := s.st.GetConversation(r.Context(), message.ConversationID)
	if err != nil || !s.canSee(currentUser(r), conversation, s.userSetores(r.Context(), currentUser(r)), s.modoAberto(r.Context())) {
		writeReactError(w, http.StatusNotFound, "not_found", "mídia não encontrada")
		return
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(message.MediaB64))
	if err != nil || len(data) == 0 {
		writeReactError(w, http.StatusNotFound, "not_found", "mídia inválida")
		return
	}
	writeMediaBytes(w, message.MediaType, message.MediaMime, data)
}

// writeMediaBytes entrega a mídia como bytes (não JSON/base64) com o
// Content-Type correto. O player usa a própria URL como src, então o navegador
// decodifica pelo cabeçalho — áudios OGG/Opus recebidos por mediaLink não têm
// MIME persistido e cairiam em 0:00 sem esse fallback.
func writeMediaBytes(w http.ResponseWriter, mediaType, declaredMime string, data []byte) {
	w.Header().Set("Content-Type", messageMediaContentType(mediaType, declaredMime))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// messageMediaContentType garante um Content-Type tocável/visualizável. Áudios
// recebidos por mediaLink podem ter ficado sem MIME; o navegador não decodifica
// sem um tipo de áudio válido e o player fica em 0:00.
func messageMediaContentType(mediaType, declared string) string {
	declared = strings.TrimSpace(declared)
	base := strings.ToLower(strings.TrimSpace(strings.SplitN(declared, ";", 2)[0]))
	if base != "" && base != "application/octet-stream" {
		return declared
	}
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "audio":
		return "audio/ogg"
	case "image":
		return "image/jpeg"
	case "video":
		return "video/mp4"
	case "pdf":
		return "application/pdf"
	}
	return "application/octet-stream"
}

func (s *Server) handleSaaSMessageEdit(w http.ResponseWriter, r *http.Request) {
	conversationID, messageID, ok := saasReactConversationMessageIDs(r.URL.Path)
	if !ok {
		writeReactError(w, http.StatusBadRequest, "invalid_id", "conversa ou mensagem inválida")
		return
	}
	var input struct {
		Content string `json:"content"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 16*1024+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || strings.TrimSpace(input.Content) == "" {
		writeReactError(w, http.StatusBadRequest, "invalid_content", "conteúdo vazio ou inválido")
		return
	}
	input.Content = strings.TrimSpace(input.Content)

	conversation, err := s.st.GetConversation(r.Context(), conversationID)
	if err != nil {
		writeReactError(w, http.StatusNotFound, "not_found", "conversa não encontrada")
		return
	}
	if !s.canActOn(r.Context(), currentUser(r), conversationID) {
		writeReactError(w, http.StatusForbidden, "conversation_not_assigned", "aceite o atendimento antes de editar")
		return
	}
	message, err := s.st.GetMessage(r.Context(), messageID)
	if err != nil || message.ConversationID != conversationID || message.WAMsgID == "" || message.Role != "assistant" {
		writeReactError(w, http.StatusBadRequest, "message_not_editable", "mensagem não pode ser editada")
		return
	}
	if strings.TrimSpace(conversation.CustomerPhone) == "" {
		writeReactError(w, http.StatusServiceUnavailable, "channel_unavailable", "canal indisponível para edição")
		return
	}
	if err := s.sendConversationEdit(r.Context(), conversation, conversation.CustomerPhone, message.WAMsgID, input.Content); err != nil {
		writeReactError(w, http.StatusBadGateway, "edit_delivery_failed", "não foi possível editar a mensagem no canal")
		return
	}
	if err := s.st.UpdateMessageContent(r.Context(), messageID, input.Content); err != nil {
		writeReactError(w, http.StatusInternalServerError, "update_failed", "mensagem foi editada no canal, mas não foi possível atualizar o histórico")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"ok": true, "edited": true, "message_id": messageID}, nil)
}
