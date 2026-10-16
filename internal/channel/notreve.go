package channel

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/agent"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func truncateBytes(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n])
}

// Notreve é o canal WhatsApp via API Notreve.
// Recebe os webhooks de mensagem e envia as respostas via /chat/send/text.
// A configuração (base URL + token) vem das settings no banco (painel).
type Notreve struct {
	proc *Processor
	st   *store.Store
	http *http.Client

	// hmacKey, quando definida, faz a validação (e log) da assinatura HMAC do
	// webhook (header x-hmac-signature). Definida via SetHMACKey a partir de
	// env WHATSAPP_HMAC_KEY. Se vazia, a validação é pulada (compatibilidade).
	hmacKey string

	lidMu sync.Mutex
	lid   map[string]string
}

const maxWhatsAppResponse = 32 << 20

func NewNotreve(proc *Processor, st *store.Store) *Notreve {
	return &Notreve{
		proc: proc,
		st:   st,
		http: &http.Client{Transport: secretAwareHTTPTransport, Timeout: 30 * time.Second},
		lid:  map[string]string{},
	}
}

// SetHMACKey define a chave usada para validar a assinatura HMAC do webhook.
func (n *Notreve) SetHMACKey(key string) { n.hmacKey = strings.TrimSpace(key) }

// rememberLID guarda o vínculo observado no próprio webhook entre a identidade
// privada (@lid) e o telefone real (SenderAlt/phoneJid).
func (n *Notreve) rememberLID(tenantID int, lidJID, phone string) {
	lidJID = normalizeLID(lidJID)
	phone = digitsOnly(phone)
	if lidJID == "" || !isRealPhone(phone) {
		return
	}
	n.lidMu.Lock()
	n.lid[fmt.Sprintf("%d:%s", tenantID, lidJID)] = phone
	n.lidMu.Unlock()
}

func (n *Notreve) cachedPhoneForLID(tenantID int, lidJID string) string {
	lidJID = normalizeLID(lidJID)
	n.lidMu.Lock()
	phone := n.lid[fmt.Sprintf("%d:%s", tenantID, lidJID)]
	n.lidMu.Unlock()
	return phone
}

func (n *Notreve) channel(ctx context.Context) (store.Channel, error) {
	return n.st.ActiveChannel(ctx, "whatsapp")
}

// Payload do webhook da API Notreve (formato real):
//
//	{
//	  "type": "Message",
//	  "event": {
//	    "Info": {
//	      "Chat": "<lid-id>@lid",
//	      "IsFromMe": false,
//	      "IsGroup": false,
//	      "PushName": "Cliente Exemplo",
//	      "Sender": "<lid-id>@lid",
//	      "SenderAlt": "<phone>@s.whatsapp.net"
//	    },
//	    "Message": { "conversation": "Quais produtos vocês tem?" }
//	  },
//	  "instanceName": "COMERCIAL",
//	  "userID": "..."
//	}
type notrevePayload struct {
	Type              string          `json:"type"`
	Event             json.RawMessage `json:"event"`
	InstanceName      string          `json:"instanceName"`
	InstanceNameSnake string          `json:"instance_name"`
	UserID            string          `json:"userID"`
	UserIDSnake       string          `json:"user_id"`
	// A Notreve API também replica estes campos no topo do evento Message.
	// Algumas versões/combinações de configuração entregam o conteúdo apenas
	// aqui, enquanto outras o colocam dentro de event.Info/event.Message.
	Phone               string `json:"phone"`
	PhoneJID            string `json:"phoneJid"`
	PhoneJIDSnake       string `json:"phone_jid"`
	Lid                 string `json:"lid"`
	AddressingMode      string `json:"addressingMode"`
	AddressingModeSnake string `json:"addressing_mode"`
	ChatJID             string `json:"chatJid"`
	ChatJIDSnake        string `json:"chat_jid"`
	IsGroup             bool   `json:"isGroup"`
	IsGroupSnake        bool   `json:"is_group"`
	IsFromMe            bool   `json:"isFromMe"`
	IsFromMeSnake       bool   `json:"is_from_me"`
	MessageType         string `json:"messageType"`
	MessageTypeSnake    string `json:"message_type"`
	Body                string `json:"body"`
	MediaLink           string `json:"mediaLink"`
	MediaLinkSnake      string `json:"media_link"`
	Base64              string `json:"base64"`
	MimeType            string `json:"mimeType"`
	MimeTypeSnake       string `json:"mime_type"`
	FileName            string `json:"fileName"`
	FileNameSnake       string `json:"file_name"`
	S3                  *struct {
		URL string `json:"url"`
	} `json:"s3"`
}

func normalizeNotrevePayload(payload *notrevePayload) {
	if payload == nil {
		return
	}
	if payload.InstanceName == "" {
		payload.InstanceName = payload.InstanceNameSnake
	}
	if payload.UserID == "" {
		payload.UserID = payload.UserIDSnake
	}
	if payload.PhoneJID == "" {
		payload.PhoneJID = payload.PhoneJIDSnake
	}
	if payload.AddressingMode == "" {
		payload.AddressingMode = payload.AddressingModeSnake
	}
	if payload.ChatJID == "" {
		payload.ChatJID = payload.ChatJIDSnake
	}
	if payload.IsGroupSnake {
		payload.IsGroup = true
	}
	if payload.IsFromMeSnake {
		payload.IsFromMe = true
	}
	if payload.MessageType == "" {
		payload.MessageType = payload.MessageTypeSnake
	}
	if payload.MediaLink == "" {
		payload.MediaLink = payload.MediaLinkSnake
	}
	if payload.MimeType == "" {
		payload.MimeType = payload.MimeTypeSnake
	}
	if payload.FileName == "" {
		payload.FileName = payload.FileNameSnake
	}
}

type notreveCallEvent struct {
	CallCreator      string            `json:"CallCreator"`
	CallCreatorAlt   string            `json:"CallCreatorAlt"`
	CallID           string            `json:"CallID"`
	CallFrom         string            `json:"call_from"`
	CallIDSnake      string            `json:"call_id"`
	CallCreatorCamel string            `json:"callCreator"`
	CallIDCamel      string            `json:"callId"`
	CallerJID        string            `json:"caller_jid"`
	CallerAlt        string            `json:"CallerAlt"`
	SenderAlt        string            `json:"SenderAlt"`
	From             string            `json:"from"`
	Phone            string            `json:"phone"`
	PhoneJID         string            `json:"phoneJid"`
	Chat             string            `json:"Chat"`
	ChatJID          string            `json:"chatJid"`
	JID              string            `json:"JID"`
	ID               string            `json:"id"`
	Info             *notreveCallEvent `json:"Info"`
	BasicCallMeta    *notreveCallEvent `json:"BasicCallMeta"`
	Call             *notreveCallEvent `json:"call"`
	Data             *notreveCallEvent `json:"data"`
	Payload          *notreveCallEvent `json:"payload"`
	Event            *notreveCallEvent `json:"event"`
}

func (e notreveCallEvent) values() (creator, callID string) {
	creator = firstNonEmpty(e.CallCreator, e.CallFrom, e.CallCreatorCamel, e.CallerJID, e.From, e.CallCreatorAlt, e.CallerAlt, e.SenderAlt, e.PhoneJID, e.Phone, e.ChatJID, e.Chat, e.JID)
	callID = firstNonEmpty(e.CallID, e.CallIDSnake, e.CallIDCamel, e.ID)
	if creator == "" || callID == "" {
		for _, nested := range e.nested() {
			if nested == nil {
				continue
			}
			nestedCreator, nestedID := nested.values()
			if creator == "" {
				creator = nestedCreator
			}
			if callID == "" {
				callID = nestedID
			}
		}
	}
	return strings.TrimSpace(creator), strings.TrimSpace(callID)
}

func (e notreveCallEvent) nested() []*notreveCallEvent {
	return []*notreveCallEvent{e.Info, e.BasicCallMeta, e.Call, e.Data, e.Payload, e.Event}
}

func (e notreveCallEvent) creatorCandidates() []string {
	candidates := []string{e.CallCreator, e.CallFrom, e.CallCreatorCamel, e.CallerJID, e.From, e.CallCreatorAlt, e.CallerAlt, e.SenderAlt, e.PhoneJID, e.Phone, e.ChatJID, e.Chat, e.JID}
	for _, nested := range e.nested() {
		if nested != nil {
			candidates = append(candidates, nested.creatorCandidates()...)
		}
	}
	return candidates
}

func callEventPhone(e notreveCallEvent) (creator, phone string) {
	for _, candidate := range e.creatorCandidates() {
		if resolved := resolvePhone(candidate); resolved != "" {
			return strings.TrimSpace(candidate), resolved
		}
	}
	return "", ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func decodeNotreveCallEvent(raw []byte) (notreveCallEvent, error) {
	var evt notreveCallEvent
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return evt, nil
	}
	if raw[0] == '"' {
		var encoded string
		if err := json.Unmarshal(raw, &encoded); err != nil {
			return evt, err
		}
		raw = bytes.TrimSpace([]byte(encoded))
	}
	if err := json.Unmarshal(raw, &evt); err != nil {
		return evt, err
	}
	return evt, nil
}

func callRejectJID(creator, phone string) string {
	creator = strings.TrimSpace(creator)
	if strings.Contains(creator, "@") {
		return creator
	}
	if phone != "" {
		return phone + "@s.whatsapp.net"
	}
	return creator
}

func isNotreveCallOfferType(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "calloffer", "call.offer", "call_offer", "call":
		return true
	default:
		return false
	}
}

type notreveEvent struct {
	Info struct {
		Chat           string `json:"Chat"`
		ChatSnake      string `json:"chat"`
		Sender         string `json:"Sender"`
		SenderSnake    string `json:"sender"`
		SenderAlt      string `json:"SenderAlt"`
		SenderAltSnake string `json:"sender_alt"`
		IsFromMe       bool   `json:"IsFromMe"`
		IsFromMeSnake  bool   `json:"is_from_me"`
		IsGroup        bool   `json:"IsGroup"`
		IsGroupSnake   bool   `json:"is_group"`
		PushName       string `json:"PushName"`
		PushNameSnake  string `json:"push_name"`
		ID             string `json:"ID"`
		IDSnake        string `json:"id"`
		Type           string `json:"Type"`
		TypeSnake      string `json:"type"`
		// Campos novos da API (identidade do contato resolvida):
		Phone         string `json:"phone"`
		PhoneJID      string `json:"phoneJid"`
		PhoneJIDSnake string `json:"phone_jid"`
		Lid           string `json:"lid"`
		ChatJID       string `json:"chatJid"`
		ChatJIDSnake  string `json:"chat_jid"`
	} `json:"Info"`
	Message notreveMessage `json:"Message"`
}

type notreveMessage struct {
	Conversation        string `json:"conversation"`
	ExtendedTextMessage struct {
		Text string `json:"text"`
	} `json:"extendedTextMessage"`
	ImageMessage    *mediaMsg `json:"imageMessage"`
	VideoMessage    *mediaMsg `json:"videoMessage"`
	AudioMessage    *mediaMsg `json:"audioMessage"`
	DocumentMessage *mediaMsg `json:"documentMessage"`
	StickerMessage  *mediaMsg `json:"stickerMessage"`
	ContactMessage  *struct {
		Vcard string `json:"vcard"`
	} `json:"contactMessage"`
	LocationMessage *struct {
		Name      string  `json:"name"`
		Address   string  `json:"address"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"locationMessage"`
	LiveLocationMessage *struct {
		Name      string  `json:"name"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"liveLocationMessage"`
	ButtonsResponseMessage *struct {
		SelectedButtonID    string `json:"selectedButtonID"`
		SelectedDisplayText string `json:"selectedDisplayText"`
	} `json:"buttonsResponseMessage"`
	ListResponseMessage *struct {
		Title             string `json:"title"`
		SingleSelectReply *struct {
			SelectedRowID string `json:"selectedRowID"`
		} `json:"singleSelectReply"`
	} `json:"listResponseMessage"`
	TemplateButtonReplyMessage *struct {
		SelectedID          string `json:"selectedId"`
		SelectedDisplayText string `json:"selectedDisplayText"`
	} `json:"templateButtonReplyMessage"`
	InteractiveResponseMessage *struct {
		Body *struct {
			Text string `json:"text"`
		} `json:"body"`
		NativeFlowResponseMessage *struct {
			Name       string `json:"name"`
			ParamsJSON string `json:"paramsJson"`
		} `json:"nativeFlowResponseMessage"`
	} `json:"interactiveResponseMessage"`
}

// mediaMsg carrega os dados de mídia entregues no webhook (com media_delivery=base64
// e/ou os campos de download quando a entrega inline não vem).
type mediaMsg struct {
	Caption       string `json:"caption"`
	Base64        string `json:"base64"`
	MimeType      string `json:"mimeType"`
	MimeTypeSnake string `json:"mime_type"`
	FileName      string `json:"fileName"`
	FileNameSnake string `json:"file_name"`

	Url                string  `json:"url"`
	MediaLink          string  `json:"mediaLink"` // entrega modo "s3": URL direta da mídia
	MediaLinkSnake     string  `json:"media_link"`
	DirectPath         string  `json:"directPath"`
	DirectPathSnake    string  `json:"direct_path"`
	MediaKey           string  `json:"mediaKey"`
	MediaKeySnake      string  `json:"media_key"`
	FileEncSHA256      string  `json:"fileEncSha256"`
	FileEncSHA256Snake string  `json:"file_enc_sha256"`
	FileSHA256         string  `json:"fileSha256"`
	FileSHA256Snake    string  `json:"file_sha256"`
	FileLength         float64 `json:"fileLength"`
	FileLengthSnake    float64 `json:"file_length"`
}

func (m *mediaMsg) normalize() {
	if m == nil {
		return
	}
	if m.MimeType == "" {
		m.MimeType = m.MimeTypeSnake
	}
	if m.FileName == "" {
		m.FileName = m.FileNameSnake
	}
	if m.MediaLink == "" {
		m.MediaLink = m.MediaLinkSnake
	}
	if m.DirectPath == "" {
		m.DirectPath = m.DirectPathSnake
	}
	if m.MediaKey == "" {
		m.MediaKey = m.MediaKeySnake
	}
	if m.FileEncSHA256 == "" {
		m.FileEncSHA256 = m.FileEncSHA256Snake
	}
	if m.FileSHA256 == "" {
		m.FileSHA256 = m.FileSHA256Snake
	}
	if m.FileLength == 0 {
		m.FileLength = m.FileLengthSnake
	}
}

// downloadMediaURL baixa uma URL de mídia (modo de entrega "s3"/mediaLink) e
// devolve o conteúdo em base64. Limita o tamanho do download.
func downloadMediaURL(ctx context.Context, url string) (string, error) {
	if strings.TrimSpace(url) == "" {
		return "", fmt.Errorf("url de mídia vazia")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := mediaDownloadClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download mediaLink %d", resp.StatusCode)
	}
	const maxMediaBytes = 30 << 20 // 30 MB
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxMediaBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxMediaBytes {
		return "", fmt.Errorf("mídia excede o limite de 30 MB")
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// mediaFileExtension devolve a extensão (com ponto) de um nome de arquivo ou URL.
func mediaFileExtension(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if i := strings.IndexAny(value, "?#"); i >= 0 {
		value = value[:i]
	}
	if i := strings.LastIndex(value, "/"); i >= 0 {
		value = value[i+1:]
	}
	if i := strings.LastIndex(value, "."); i >= 0 {
		return strings.ToLower(value[i:])
	}
	return ""
}

// normalizeMediaMime resolve o MIME da mídia recebida. A entrega por base64
// costuma trazer mimeType; a entrega por s3/mediaLink não traz, então inferimos
// pela extensão (áudio do WhatsApp é OGG/Opus). O player do painel depende de um
// Content-Type tocável — sem isso o navegador não decodifica e fica em 0:00.
func normalizeMediaMime(mediaType, declared, name, rawURL string) string {
	declared = strings.TrimSpace(declared)
	if strings.Contains(declared, ";base64,") {
		declared = ""
	}
	if strings.HasPrefix(declared, "data:") {
		rest := strings.TrimPrefix(declared, "data:")
		if i := strings.IndexAny(rest, ";,"); i >= 0 {
			rest = rest[:i]
		}
		declared = strings.TrimSpace(rest)
	}
	base := strings.ToLower(strings.TrimSpace(strings.SplitN(declared, ";", 2)[0]))
	if base != "" && base != "application/octet-stream" {
		return declared
	}
	ext := mediaFileExtension(name)
	if ext == "" {
		ext = mediaFileExtension(rawURL)
	}
	switch ext {
	case ".ogg", ".oga", ".opus":
		return "audio/ogg"
	case ".mp3":
		return "audio/mpeg"
	case ".m4a", ".aac":
		return "audio/mp4"
	case ".wav":
		return "audio/wav"
	case ".webm":
		if mediaType == "audio" {
			return "audio/webm"
		}
		return "video/webm"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".pdf":
		return "application/pdf"
	case ".mp4", ".3gp", ".mov":
		return "video/mp4"
	}
	switch mediaType {
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

// verifyWebhookSignature confere a assinatura HMAC-SHA256 do body recebido.
// Aceita o header em hex ou base64. Comparação em tempo constante.
func verifyWebhookSignature(key string, body []byte, signature string) bool {
	if key == "" || signature == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write(body)
	sum := mac.Sum(nil)
	got := strings.ToLower(strings.TrimSpace(signature))
	if subtle.ConstantTimeCompare([]byte(got), []byte(hex.EncodeToString(sum))) == 1 {
		return true
	}
	return subtle.ConstantTimeCompare([]byte(strings.TrimSpace(signature)), []byte(base64.StdEncoding.EncodeToString(sum))) == 1
}

// handleDelivery registra (log) os eventos de entrega do webhook (sent/error).
// Usado para acompanhar falhas de envio, especialmente em campanhas/agendados.
func (n *Notreve) handleDelivery(ctx context.Context, raw []byte) {
	var d struct {
		Status      string `json:"status"` // sent | error
		MessageID   string `json:"message_id"`
		Phone       string `json:"phone"`
		MessageType string `json:"messageType"`
		IsGroup     bool   `json:"isGroup"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return
	}
	if d.Status == "error" {
		Logger.Printf("webhook Delivery: erro no envio para %s (%s): %s", d.Phone, d.MessageType, d.Error)
		return
	}
	Logger.Printf("webhook Delivery: enviado para %s (%s)", d.Phone, d.MessageType)
}

// firstMedia devolve a primeira mídia presente na mensagem (se houver).
func firstMedia(m *notreveMessage) *mediaMsg {
	if m == nil {
		return nil
	}
	switch {
	case m.ImageMessage != nil:
		m.ImageMessage.normalize()
		return m.ImageMessage
	case m.VideoMessage != nil:
		m.VideoMessage.normalize()
		return m.VideoMessage
	case m.AudioMessage != nil:
		m.AudioMessage.normalize()
		return m.AudioMessage
	case m.DocumentMessage != nil:
		m.DocumentMessage.normalize()
		return m.DocumentMessage
	case m.StickerMessage != nil:
		m.StickerMessage.normalize()
		return m.StickerMessage
	}
	return nil
}

func mediaTypeOf(m *notreveMessage) string {
	if m == nil {
		return ""
	}
	switch {
	case m.ImageMessage != nil:
		return "image"
	case m.VideoMessage != nil:
		return "video"
	case m.AudioMessage != nil:
		return "audio"
	case m.DocumentMessage != nil:
		if m.DocumentMessage.MimeType == "application/pdf" {
			return "pdf"
		}
		return "document"
	case m.StickerMessage != nil:
		return "image" // sticker é imagem webp
	}
	return ""
}

// extractText pega o texto da mensagem; se for mídia, devolve um resumo
// para a IA saber o que o cliente enviou.
func extractText(m *notreveMessage) string {
	if m == nil {
		return ""
	}
	if t := strings.TrimSpace(m.Conversation); t != "" {
		return t
	}
	if t := strings.TrimSpace(m.ExtendedTextMessage.Text); t != "" {
		return t
	}
	if m.ImageMessage != nil {
		if c := strings.TrimSpace(m.ImageMessage.Caption); c != "" {
			return "[📷 Cliente enviou uma imagem] Legenda: " + c
		}
		return "[📷 Cliente enviou uma imagem]"
	}
	if m.VideoMessage != nil {
		if c := strings.TrimSpace(m.VideoMessage.Caption); c != "" {
			return "[🎬 Cliente enviou um vídeo] Legenda: " + c
		}
		return "[🎬 Cliente enviou um vídeo]"
	}
	if m.AudioMessage != nil {
		return "[🎵 Cliente enviou um áudio]"
	}
	if m.DocumentMessage != nil {
		return "[📄 Cliente enviou um documento: " + m.DocumentMessage.FileName + "]"
	}
	if m.StickerMessage != nil {
		return "[😀 Cliente enviou um sticker]"
	}
	if m.ContactMessage != nil {
		name := contactNameFromVCard(m.ContactMessage.Vcard)
		if name != "" {
			return "[👤 Cliente enviou um contato: " + name + "]"
		}
		return "[👤 Cliente enviou um contato]"
	}
	if m.LocationMessage != nil {
		return "[📍 Cliente enviou uma localização]"
	}
	if m.LiveLocationMessage != nil {
		return "[🟢 Cliente enviou localização em tempo real]"
	}
	if m.ButtonsResponseMessage != nil {
		if t := strings.TrimSpace(m.ButtonsResponseMessage.SelectedDisplayText); t != "" {
			return t
		}
		return strings.TrimSpace(m.ButtonsResponseMessage.SelectedButtonID)
	}
	if m.ListResponseMessage != nil {
		if m.ListResponseMessage.SingleSelectReply != nil {
			if t := strings.TrimSpace(m.ListResponseMessage.SingleSelectReply.SelectedRowID); t != "" {
				return t
			}
		}
		return strings.TrimSpace(m.ListResponseMessage.Title)
	}
	if m.TemplateButtonReplyMessage != nil {
		if t := strings.TrimSpace(m.TemplateButtonReplyMessage.SelectedDisplayText); t != "" {
			return t
		}
		return strings.TrimSpace(m.TemplateButtonReplyMessage.SelectedID)
	}
	if m.InteractiveResponseMessage != nil {
		if m.InteractiveResponseMessage.Body != nil {
			if t := strings.TrimSpace(m.InteractiveResponseMessage.Body.Text); t != "" {
				return t
			}
		}
		if nf := m.InteractiveResponseMessage.NativeFlowResponseMessage; nf != nil {
			if t := interactiveResponseText(nf.ParamsJSON, nf.Name); t != "" {
				return t
			}
		}
	}
	return ""
}

// normalizeNotreveEvent unifica os dois formatos publicados pela API:
// campos dentro de event.Info/event.Message e campos replicados no topo do
// webhook. Assim uma mudança de formato não faz mensagens silenciosamente
// desaparecerem do histórico.
func normalizeNotreveEvent(evt *notreveEvent, payload notrevePayload) {
	if evt == nil {
		return
	}
	if evt.Info.Chat == "" {
		evt.Info.Chat = evt.Info.ChatSnake
	}
	if evt.Info.Sender == "" {
		evt.Info.Sender = evt.Info.SenderSnake
	}
	if evt.Info.SenderAlt == "" {
		evt.Info.SenderAlt = evt.Info.SenderAltSnake
	}
	if evt.Info.IsFromMeSnake {
		evt.Info.IsFromMe = true
	}
	if evt.Info.IsGroupSnake {
		evt.Info.IsGroup = true
	}
	if evt.Info.PushName == "" {
		evt.Info.PushName = evt.Info.PushNameSnake
	}
	if evt.Info.ID == "" {
		evt.Info.ID = evt.Info.IDSnake
	}
	if evt.Info.Type == "" {
		evt.Info.Type = evt.Info.TypeSnake
	}
	if evt.Info.Phone == "" {
		evt.Info.Phone = payload.Phone
	}
	if evt.Info.PhoneJID == "" {
		if evt.Info.PhoneJIDSnake != "" {
			evt.Info.PhoneJID = evt.Info.PhoneJIDSnake
		} else {
			evt.Info.PhoneJID = payload.PhoneJID
		}
	}
	if evt.Info.Lid == "" {
		evt.Info.Lid = payload.Lid
	}
	if evt.Info.ChatJID == "" {
		if evt.Info.ChatJIDSnake != "" {
			evt.Info.ChatJID = evt.Info.ChatJIDSnake
		} else {
			evt.Info.ChatJID = payload.ChatJID
		}
	}
	if !evt.Info.IsGroup {
		evt.Info.IsGroup = payload.IsGroup
	}
	if !evt.Info.IsFromMe {
		evt.Info.IsFromMe = payload.IsFromMe
	}
	if evt.Info.Type == "" {
		evt.Info.Type = payload.MessageType
	}

	// body é o texto/caption no formato compacto. Não substitua um texto
	// completo já recebido dentro de event.Message.
	if strings.TrimSpace(extractText(&evt.Message)) == "" && strings.TrimSpace(payload.Body) != "" {
		evt.Message.Conversation = strings.TrimSpace(payload.Body)
	}

	mediaLink := strings.TrimSpace(payload.MediaLink)
	if mediaLink == "" && payload.S3 != nil {
		mediaLink = strings.TrimSpace(payload.S3.URL)
	}
	if md := firstMedia(&evt.Message); md != nil {
		if md.MediaLink == "" {
			md.MediaLink = mediaLink
		}
		if md.Base64 == "" {
			md.Base64 = payload.Base64
		}
		if md.MimeType == "" {
			md.MimeType = payload.MimeType
		}
		if md.FileName == "" {
			md.FileName = payload.FileName
		}
		if md.Caption == "" && strings.EqualFold(payload.MessageType, "document") {
			md.Caption = strings.TrimSpace(payload.Body)
		}
		return
	}

	// Se a versão compacta não envia event.Message, ainda crie o marcador da
	// mídia para ela ser registrada no painel. O download é opcional; sem link
	// a mensagem continua visível como evento recebido.
	var md *mediaMsg
	switch strings.ToLower(strings.TrimSpace(payload.MessageType)) {
	case "image", "photo":
		md = &mediaMsg{MediaLink: mediaLink, Caption: payload.Body}
		evt.Message.ImageMessage = md
	case "video":
		md = &mediaMsg{MediaLink: mediaLink, Caption: payload.Body}
		evt.Message.VideoMessage = md
	case "audio", "ptt", "voice":
		md = &mediaMsg{MediaLink: mediaLink}
		evt.Message.AudioMessage = md
	case "document", "pdf", "file":
		md = &mediaMsg{MediaLink: mediaLink, Caption: payload.Body}
		evt.Message.DocumentMessage = md
	case "sticker":
		md = &mediaMsg{MediaLink: mediaLink}
		evt.Message.StickerMessage = md
	}
	if md != nil {
		md.Base64 = payload.Base64
		md.MimeType = payload.MimeType
		md.FileName = payload.FileName
	}
}

// interactiveResponseText extrai o texto de uma resposta de fluxo nativo
// (botões/listas nativos). O ParamsJSON (string) contém o display_text/id; se a
// resposta for só um nome, devolve o nome.
func interactiveResponseText(paramsJSON, name string) string {
	if strings.TrimSpace(paramsJSON) != "" {
		var payload struct {
			DisplayText string `json:"display_text"`
			ID          string `json:"id"`
			Text        string `json:"text"`
			Title       string `json:"title"`
		}
		if json.Unmarshal([]byte(paramsJSON), &payload) == nil {
			if t := strings.TrimSpace(payload.DisplayText); t != "" {
				return t
			}
			if t := strings.TrimSpace(payload.Title); t != "" {
				return t
			}
			if t := strings.TrimSpace(payload.Text); t != "" {
				return t
			}
			if t := strings.TrimSpace(payload.ID); t != "" {
				return t
			}
		}
	}
	return strings.TrimSpace(name)
}

// contactTelFromVCard extrai o telefone (TEL) de um vCard simples.
func contactTelFromVCard(vcard string) string {
	for _, line := range strings.Split(vcard, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "TEL") && strings.Contains(t, ":") {
			tel := strings.TrimSpace(t[strings.Index(t, ":")+1:])
			tel = strings.TrimPrefix(tel, "+")
			var digits []rune
			for _, r := range tel {
				if r >= '0' && r <= '9' {
					digits = append(digits, r)
				}
			}
			return string(digits)
		}
	}
	return ""
}

// contactNameFromVCard extrai o nome (FN/N) de um vCard simples.
func contactNameFromVCard(vcard string) string {
	for _, line := range strings.Split(vcard, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "FN:") {
			if n := strings.TrimSpace(strings.TrimPrefix(t, "FN:")); n != "" {
				return n
			}
		}
	}
	for _, line := range strings.Split(vcard, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "N:") {
			parts := strings.Split(strings.TrimPrefix(t, "N:"), ";")
			var fam, given string
			if len(parts) > 0 {
				fam = strings.TrimSpace(parts[0])
			}
			if len(parts) > 1 {
				given = strings.TrimSpace(parts[1])
			}
			return strings.TrimSpace(strings.Join([]string{given, fam}, " "))
		}
	}
	return ""
}

func (n *Notreve) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	const maxWebhookBody = 25 << 20
	wireRaw, readErr := io.ReadAll(io.LimitReader(r.Body, maxWebhookBody+1))
	if readErr != nil {
		http.Error(rw, "erro lendo payload", http.StatusBadRequest)
		return
	}
	if len(wireRaw) > maxWebhookBody {
		http.Error(rw, "payload muito grande", http.StatusRequestEntityTooLarge)
		return
	}

	// Validação opcional da assinatura HMAC do webhook (header x-hmac-signature).
	// Só valida/loga quando WHATSAPP_HMAC_KEY está configurada; não bloqueia, para
	// não interromper o bot caso o formato de assinatura mude na sua API.
	if n.hmacKey != "" {
		// A documentação da API assina os bytes comprimidos quando GZIP está
		// habilitado. Valide antes de descompactar.
		if !verifyWebhookSignature(n.hmacKey, wireRaw, r.Header.Get("x-hmac-signature")) {
			Logger.Printf("webhook: assinatura HMAC inválida (validando, não bloqueando) — confira WHATSAPP_HMAC_KEY")
		}
	}
	raw := wireRaw
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("Content-Encoding")), "gzip") {
		gz, err := gzip.NewReader(bytes.NewReader(wireRaw))
		if err != nil {
			Logger.Printf("webhook: gzip inválido: %v", err)
			http.Error(rw, "gzip inválido", http.StatusBadRequest)
			return
		}
		decoded, err := io.ReadAll(io.LimitReader(gz, maxWebhookBody+1))
		closeErr := gz.Close()
		if err != nil {
			http.Error(rw, "erro descompactando payload", http.StatusBadRequest)
			return
		}
		if closeErr != nil {
			http.Error(rw, "erro fechando payload", http.StatusBadRequest)
			return
		}
		if len(decoded) > maxWebhookBody {
			http.Error(rw, "payload muito grande", http.StatusRequestEntityTooLarge)
			return
		}
		raw = decoded
	}
	// LGPD: não loga o conteúdo da mensagem, só o tamanho.
	Logger.Printf("webhook recebido: %d bytes", len(raw))

	var p notrevePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		Logger.Printf("webhook: json inválido: %v", err)
		http.Error(rw, "bad payload", http.StatusBadRequest)
		return
	}
	normalizeNotrevePayload(&p)

	// Roteia pelo token/instância do payload (multi-instância: cada número usa o próprio).
	ch, err := n.st.ChannelByTokenRouted(r.Context(), p.UserID)
	if err != nil && strings.TrimSpace(p.InstanceName) != "" {
		// Alguns formatos da API preenchem userID com o usuário interno e
		// deixam a identificação operacional somente em instanceName.
		ch, err = n.st.ChannelByNameRouted(r.Context(), p.InstanceName)
	}
	if err != nil {
		Logger.Printf("webhook: instância não reconhecida, ignorando")
		rw.WriteHeader(http.StatusOK)
		return
	}
	if err != nil || strings.TrimRight(ch.BaseURL, "/") == "" || ch.Token == "" {
		Logger.Printf("webhook: WhatsApp não configurado no painel, ignorando")
		rw.WriteHeader(http.StatusOK)
		return
	}
	// O webhook é público e não passa pelo login. O canal resolvido pelo
	// identificador da instância é a autoridade para estabelecer o tenant do
	// restante do processamento.
	ctx := store.WithTenantID(r.Context(), ch.TenantID)
	baseURL := strings.TrimRight(ch.BaseURL, "/")
	token := ch.Token

	if !strings.EqualFold(strings.TrimSpace(p.Type), "message") {
		Logger.Printf("webhook evento não-mensagem: tipo=%q instância=%q", strings.TrimSpace(p.Type), ch.Name)
		if strings.EqualFold(strings.TrimSpace(p.Type), "ChatPresence") {
			n.handlePresence(ctx, raw)
		}
		if isNotreveCallOfferType(p.Type) {
			n.handleCall(ctx, raw, baseURL, token, ch)
		}
		if strings.EqualFold(strings.TrimSpace(p.Type), "ReadReceipt") {
			n.handleReadReceipt(ctx, raw)
		}
		if strings.EqualFold(strings.TrimSpace(p.Type), "Delivery") {
			n.handleDelivery(ctx, raw)
		}
		rw.WriteHeader(http.StatusOK)
		return
	}

	var evt notreveEvent
	if len(bytes.TrimSpace(p.Event)) > 0 {
		if err := json.Unmarshal(p.Event, &evt); err != nil {
			// Compatibilidade com APIs que serializam event como uma string JSON.
			var encoded string
			if json.Unmarshal(p.Event, &encoded) != nil || json.Unmarshal([]byte(encoded), &evt) != nil {
				http.Error(rw, "bad event", http.StatusBadRequest)
				return
			}
		}
	}
	normalizeNotreveEvent(&evt, p)

	// Extraia o conteúdo antes de decidir se o evento é vazio. A API envia
	// documentos, áudios, imagens e localizações sem texto quando não há
	// legenda; esses eventos também precisam virar uma mensagem no painel.
	text := extractText(&evt.Message)
	if text == "" {
		Logger.Printf("webhook: evento Message sem conteúdo reconhecido (instância=%q id=%q)", ch.Name, evt.Info.ID)
		rw.WriteHeader(http.StatusOK)
		return
	}

	// Define o destino: canais/newsletters são somente leitura e nunca recebem
	// respostas. Em grupos, a resposta depende da opção do canal. A API
	// normalmente envia IsGroup, mas o JID é a fonte de verdade quando esse
	// campo não vem preenchido.
	if evt.Info.IsFromMe {
		rw.WriteHeader(http.StatusOK)
		return
	}
	if whatsappChannelChat(evt) {
		Logger.Printf("webhook: mensagem de canal/broadcast ignorada")
		rw.WriteHeader(http.StatusOK)
		return
	}
	isGroupChat := whatsappGroupChat(evt)
	var phone, deliveryTarget string
	if isGroupChat {
		if !ch.RespondGrupos {
			Logger.Printf("webhook: mensagem de grupo/canal ignorada (respond_grupos=off)")
			rw.WriteHeader(http.StatusOK)
			return
		}
		phone = evt.Info.Chat // responder no grupo (JID completo)
		deliveryTarget = phone
	} else {
		deliveryTarget = replyJIDOf(evt.Info.ChatJID, evt.Info.Chat, evt.Info.Sender, evt.Info.PhoneJID, evt.Info.SenderAlt)
		lidJID := lidFromCandidates(evt.Info.Lid, evt.Info.ChatJID, evt.Info.Chat, evt.Info.Sender, evt.Info.SenderAlt)
		phone = resolvePhoneAvoidingLID(lidJID, evt.Info.PhoneJID, evt.Info.Phone, evt.Info.SenderAlt, evt.Info.Sender, evt.Info.ChatJID, evt.Info.Chat)
		if phone != "" && lidJID != "" {
			n.rememberLID(ch.TenantID, lidJID, phone)
		}
		if phone == "" && lidJID != "" {
			phone = n.cachedPhoneForLID(ch.TenantID, lidJID)
		}
		if phone == "" {
			phone = deliveryTarget
		}
	}
	if phone == "" {
		rw.WriteHeader(http.StatusOK)
		return
	}
	if deliveryTarget == "" {
		deliveryTarget = phone
	}

	// Allowlist: só responde para números permitidos (configurado em Configurações).
	allowed, canonicalPhone := n.phoneAllowed(ctx, baseURL, token, phone)
	if !allowed {
		Logger.Printf("webhook: remetente não está na lista de permitidos, ignorado (sem resposta)")
		rw.WriteHeader(http.StatusOK)
		return
	}
	if canonicalPhone != "" {
		phone = canonicalPhone
	}

	in := Inbound{
		Message:   text,
		Phone:     phone,
		Name:      evt.Info.PushName,
		WAMsgID:   evt.Info.ID,
		TenantID:  ch.TenantID,
		ChannelID: func() *int { id := ch.ID; return &id }(),
		Channel:   "whatsapp",
	}
	if md := firstMedia(&evt.Message); md != nil {
		b64, dataMime := parseDataURI(strings.TrimSpace(md.Base64))
		in.MediaType = mediaTypeOf(&evt.Message)
		in.MediaB64 = b64
		in.MediaMime = normalizeMediaMime(in.MediaType, firstNonEmpty(md.MimeType, dataMime), md.FileName, md.MediaLink)
		in.MediaName = md.FileName
		in.MediaCaption = md.Caption
		Logger.Printf("media recebida: type=%s b64len=%d mime=%q", in.MediaType, len(in.MediaB64), in.MediaMime)
		// Se a entrega inline não veio, baixa pela API (endpoint /chat/download*).
		if in.MediaB64 == "" && (md.Url != "" || md.MediaKey != "") {
			dlType := in.MediaType
			switch dlType {
			case "pdf":
				dlType = "document"
			case "image":
				if mediaTypeOf(&evt.Message) == "image" && md.MimeType == "image/webp" {
					dlType = "sticker"
				}
			}
			data, mime, derr := InstanciaDownloadMedia(ctx, baseURL, token, dlType, *md)
			if derr == nil && data != "" {
				if i := strings.Index(data, "base64,"); i >= 0 {
					in.MediaB64 = data[i+len("base64,"):]
				} else {
					in.MediaB64 = data
				}
				in.MediaMime = normalizeMediaMime(in.MediaType, firstNonEmpty(in.MediaMime, mime), md.FileName, md.MediaLink)
			} else if derr != nil {
				Logger.Printf("media: falha ao baixar %s: %v", in.MediaType, derr)
			}
		}
		// Modo de entrega "s3": a mídia vem como URL (mediaLink). Baixa por HTTPS.
		if in.MediaB64 == "" && md.MediaLink != "" {
			data, derr := downloadMediaURL(ctx, md.MediaLink)
			if derr == nil && data != "" {
				in.MediaB64 = data
				in.MediaMime = normalizeMediaMime(in.MediaType, firstNonEmpty(in.MediaMime, md.MimeType), md.FileName, md.MediaLink)
			} else if derr != nil {
				Logger.Printf("media: falha ao baixar mediaLink %s: %v", in.MediaType, derr)
			}
		}
		Logger.Printf("media: type=%s b64len=%d mime=%q name=%q", in.MediaType, len(in.MediaB64), in.MediaMime, in.MediaName)
	}

	// Localização / contato (cards no chat).
	if evt.Message.LocationMessage != nil {
		in.Lat = evt.Message.LocationMessage.Latitude
		in.Lng = evt.Message.LocationMessage.Longitude
	}
	if evt.Message.LiveLocationMessage != nil {
		in.Lat = evt.Message.LiveLocationMessage.Latitude
		in.Lng = evt.Message.LiveLocationMessage.Longitude
	}
	if evt.Message.ContactMessage != nil {
		in.ContactName = contactNameFromVCard(evt.Message.ContactMessage.Vcard)
		in.ContactTel = contactTelFromVCard(evt.Message.ContactMessage.Vcard)
	}

	res, err := n.proc.Process(ctx, in)
	if err != nil {
		Logger.Printf("erro processando mensagem %q de %s: %v", text, phone, err)
		http.Error(rw, "erro interno", http.StatusInternalServerError)
		return
	}

	sent := false
	if res.Handled && res.Reply != "" {
		if res.Rich != nil {
			if err := n.sendRichMsg(ctx, baseURL, token, deliveryTarget, res.Rich); err != nil {
				Logger.Printf("envio rico (%s) falhou, usando fallback: %v", res.Rich.Kind, err)
				if bErr := n.post(ctx, baseURL, token, "/chat/send/text", map[string]interface{}{"phone": deliveryTarget, "body": whatsappSanitize(res.Fallback)}); bErr == nil {
					sent = true
				}
			} else {
				sent = true
			}
		} else if err := n.post(ctx, baseURL, token, "/chat/send/text", map[string]interface{}{"phone": deliveryTarget, "body": whatsappSanitize(res.Reply)}); err != nil {
			Logger.Printf("erro enviando resposta ao WhatsApp: %v", err)
		} else {
			sent = true
		}
	}

	// Busca a foto do cliente em background (só se ainda não tiver).
	go func() {
		if c, err := n.st.GetConversation(ctx, res.ConversationID); err == nil && c.AvatarURL == "" {
			if url, err := n.GetAvatar(ctx, deliveryTarget); err == nil && url != "" {
				_ = n.st.SetConversationAvatar(ctx, res.ConversationID, url)
			} else {
				_ = n.st.SetConversationAvatarTried(ctx, res.ConversationID)
			}
		}
	}()

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(map[string]interface{}{
		"conversation_id": res.ConversationID,
		"status":          res.Status,
		"reply_sent":      sent,
	})
}

// GetAvatar retorna a URL da foto de perfil do número (ou "" se não tiver).
func (n *Notreve) GetAvatar(ctx context.Context, phone string) (string, error) {
	ch, err := n.channel(ctx)
	if err != nil || strings.TrimRight(ch.BaseURL, "/") == "" || ch.Token == "" {
		return "", errors.New("WhatsApp não configurado no painel")
	}
	baseURL := strings.TrimRight(ch.BaseURL, "/")
	body, _ := json.Marshal(map[string]interface{}{"phone": phone})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/user/avatar", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("token", ch.Token)
	resp, err := n.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxWhatsAppResponse+1))
	if len(raw) > maxWhatsAppResponse {
		return "", errors.New("resposta da API WhatsApp excede o limite")
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("API WhatsApp %d", resp.StatusCode)
	}
	var out struct {
		Data struct {
			URL        string `json:"url"`
			DirectPath string `json:"direct_path"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.Data.URL == "" && out.Data.DirectPath != "" {
		out.Data.URL = "https://pps.whatsapp.net" + out.Data.DirectPath
	}
	if out.Data.URL == "" {
		Logger.Printf("avatar vazio para %s: %s", phone, truncateBytes(raw, 200))
	}
	return out.Data.URL, nil
}

// CheckWhatsApp verifica se um número está registrado no WhatsApp.
func (n *Notreve) CheckWhatsApp(ctx context.Context, phone string) (bool, error) {
	ch, err := n.channel(ctx)
	if err != nil || strings.TrimRight(ch.BaseURL, "/") == "" || ch.Token == "" {
		return false, errors.New("WhatsApp não configurado no painel")
	}
	baseURL := strings.TrimRight(ch.BaseURL, "/")
	body, _ := json.Marshal(map[string]interface{}{"phone": []string{phone}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/user/check", bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("token", ch.Token)
	resp, err := n.http.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxWhatsAppResponse+1))
	if len(raw) > maxWhatsAppResponse {
		return false, errors.New("resposta da API WhatsApp excede o limite")
	}
	if resp.StatusCode >= 300 {
		return false, fmt.Errorf("API WhatsApp %d: %s", resp.StatusCode, truncateBytes(raw, 300))
	}
	var out struct {
		Data struct {
			Users []struct {
				IsInWhatsapp bool `json:"IsInWhatsapp"`
			} `json:"Users"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return false, err
	}
	if len(out.Data.Users) > 0 {
		return out.Data.Users[0].IsInWhatsapp, nil
	}
	return false, nil
}

// SyncContacts puxa a lista de contatos da conta WhatsApp conectada.
func (n *Notreve) SyncContacts(ctx context.Context) ([]store.Contato, error) {
	ch, err := n.channel(ctx)
	if err != nil || strings.TrimRight(ch.BaseURL, "/") == "" || ch.Token == "" {
		return nil, errors.New("WhatsApp não configurado no painel")
	}
	baseURL := strings.TrimRight(ch.BaseURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/user/contacts", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("token", ch.Token)
	resp, err := n.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxWhatsAppResponse+1))
	if len(raw) > maxWhatsAppResponse {
		return nil, errors.New("resposta da API WhatsApp excede o limite")
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API WhatsApp %d: %s", resp.StatusCode, truncateBytes(raw, 300))
	}
	var out struct {
		Data map[string]struct {
			PushName string `json:"PushName"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	var contatos []store.Contato
	for jid, info := range out.Data {
		phone := phoneFromJID(jid)
		if phone == "" || !isRealPhone(phone) {
			continue
		}
		contatos = append(contatos, store.Contato{Name: info.PushName, Phone: phone, TemWhatsapp: true})
	}
	return contatos, nil
}

// SendText envia uma mensagem de texto pelo WhatsApp (usado pelo painel
// quando um atendente humano responde).
func (n *Notreve) SendText(ctx context.Context, phone, text string) error {
	return n.SendTextReply(ctx, phone, text, "")
}

// SendTextReply envia texto, opcionalmente respondendo a uma mensagem
// específica do cliente (replyToWAMsgID = ID no WhatsApp da mensagem original).
func (n *Notreve) SendTextReply(ctx context.Context, phone, text, replyToWAMsgID string) error {
	ch, err := n.channel(ctx)
	if err != nil || strings.TrimRight(ch.BaseURL, "/") == "" || ch.Token == "" {
		return errors.New("WhatsApp não configurado no painel")
	}
	return n.SendTextChannelReply(ctx, ch, phone, text, replyToWAMsgID)
}

// SendTextChannelReply envia texto pela instância exata da conversa, com
// contexto de resposta quando o canal suporta esse recurso.
func (n *Notreve) SendTextChannelReply(ctx context.Context, ch store.Channel, phone, text, replyToWAMsgID string) error {
	if ch.Type != "whatsapp" || strings.TrimRight(ch.BaseURL, "/") == "" || ch.Token == "" {
		return errors.New("canal WhatsApp não configurado")
	}
	baseURL := strings.TrimRight(ch.BaseURL, "/")
	token := ch.Token
	payload := map[string]interface{}{"phone": phone, "body": whatsappSanitize(text)}
	if replyToWAMsgID != "" {
		payload["context_info"] = map[string]interface{}{
			"stanza_id":   replyToWAMsgID,
			"participant": phone + "@s.whatsapp.net",
		}
	}
	return n.post(ctx, baseURL, token, "/chat/send/text", payload)
}

// SendTextChannel envia por um canal específico (base URL + token) ignorando
// a allowlist — usado por campanhas (ação explícita do admin).
func (n *Notreve) SendTextChannel(ctx context.Context, ch store.Channel, phone, text string) (string, error) {
	if ch.Type != "whatsapp" || strings.TrimRight(ch.BaseURL, "/") == "" || ch.Token == "" {
		return "", errors.New("canal inválido para campanha")
	}
	body, err := n.postRaw(ctx, strings.TrimRight(ch.BaseURL, "/"), ch.Token, "/chat/send/text",
		map[string]interface{}{"phone": phone, "body": whatsappSanitize(text)})
	if err != nil {
		return "", err
	}
	return extractMsgID(body), nil
}

// SendMediaChannel envia mídia por um canal específico (campanhas).
func (n *Notreve) SendMediaChannel(ctx context.Context, ch store.Channel, phone, mediaType, dataB64, mimeType, fileName, caption string) (string, error) {
	if ch.Type != "whatsapp" || strings.TrimRight(ch.BaseURL, "/") == "" || ch.Token == "" {
		return "", errors.New("canal inválido para campanha")
	}
	endpoint := "/chat/send/" + mediaType
	payload := map[string]interface{}{"phone": phone}
	switch mediaType {
	case "image":
		payload["image"] = dataURI(dataB64, mimeType, "image/jpeg")
		if caption != "" {
			payload["caption"] = whatsappSanitize(caption)
		}
	case "audio":
		payload["audio"] = dataURI(dataB64, mimeType, "audio/ogg")
	case "video":
		payload["video"] = dataURI(dataB64, mimeType, "video/mp4")
		if caption != "" {
			payload["caption"] = whatsappSanitize(caption)
		}
	case "document":
		// A API em produção valida o documento como data URI com o MIME
		// application/octet-stream (mesmo quando o arquivo é PDF). O contrato
		// textual da collection dizia base64 bruto, mas a resposta real 400 da
		// API confirma que o prefixo é obrigatório.
		payload["document"] = dataURI(dataB64, "application/octet-stream", "application/octet-stream")
		payload["file_name"] = fileName
	default:
		return "", errors.New("tipo de mídia inválido")
	}
	body, err := n.postRaw(ctx, strings.TrimRight(ch.BaseURL, "/"), ch.Token, endpoint, payload)
	if err != nil {
		return "", err
	}
	return extractMsgID(body), nil
}

// SendRichMessage envia uma mensagem rica pelo canal WhatsApp ativo (resolve o
// canal/token do tenant). Usado pelo painel (ex.: satisfação). O chamador deve
// cair no Fallback se o tipo não for suportado ou a API falhar.
func (n *Notreve) SendRichMessage(ctx context.Context, phone string, rich *agent.RichMessage) error {
	ch, err := n.channel(ctx)
	if err != nil || ch.Type != "whatsapp" || strings.TrimRight(ch.BaseURL, "/") == "" || ch.Token == "" {
		return errors.New("WhatsApp não configurado no painel")
	}
	return n.SendRichMessageChannel(ctx, ch, phone, rich)
}

// SendRichMessageChannel envia uma mensagem rica pela instância exata do
// canal, evitando que satisfação/menu caiam no WhatsApp ativo global.
func (n *Notreve) SendRichMessageChannel(ctx context.Context, ch store.Channel, phone string, rich *agent.RichMessage) error {
	if ch.Type != "whatsapp" || strings.TrimRight(ch.BaseURL, "/") == "" || ch.Token == "" {
		return errors.New("canal WhatsApp não configurado")
	}
	return n.sendRichMsg(ctx, ch.BaseURL, ch.Token, phone, rich)
}

// sendRichMsg envia uma mensagem rica (botões, lista, enquete, contato,
// localização ou figurinha) para o número. O chamador deve cair no Fallback
// (texto) se o tipo não for suportado ou se a API falhar.
func (n *Notreve) sendRichMsg(ctx context.Context, baseURL, token, phone string, rich *agent.RichMessage) error {
	if rich == nil {
		return errors.New("mensagem rica vazia")
	}
	baseURL = strings.TrimRight(baseURL, "/")
	switch rich.Kind {
	case "buttons":
		if len(rich.Buttons) == 0 {
			return errors.New("botões sem itens")
		}
		return n.post(ctx, baseURL, token, "/chat/send/buttons", map[string]interface{}{
			"phone": phone, "body": rich.Text, "title": rich.Title, "footer": rich.Footer, "buttons": rich.Buttons,
		})
	case "list":
		if len(rich.Sections) == 0 {
			return errors.New("lista sem seções")
		}
		type listRow struct {
			Title       string `json:"title"`
			Description string `json:"desc"`
			RowID       string `json:"row_id"`
		}
		sections := make([]map[string]interface{}, 0, len(rich.Sections))
		for _, sec := range rich.Sections {
			rows := make([]listRow, 0, len(sec.Rows))
			for _, r := range sec.Rows {
				rows = append(rows, listRow{Title: r.Title, Description: r.Description, RowID: r.ID})
			}
			sections = append(sections, map[string]interface{}{"title": sec.Title, "rows": rows})
		}
		buttonText := rich.Footer
		if buttonText == "" {
			buttonText = "Ver opções"
		}
		return n.post(ctx, baseURL, token, "/chat/send/list", map[string]interface{}{
			"phone": phone, "desc": rich.Text, "top_text": rich.Title, "footer_text": rich.Footer,
			"sections": sections, "button_text": buttonText,
		})
	case "poll":
		if rich.Poll == nil || len(rich.Poll.Options) < 2 {
			return errors.New("enquete precisa de ao menos 2 opções")
		}
		options := make([]string, 0, len(rich.Poll.Options))
		for _, o := range rich.Poll.Options {
			options = append(options, o.Title)
		}
		return n.post(ctx, baseURL, token, "/chat/send/poll", map[string]interface{}{
			"group": phone, "header": rich.Poll.Question, "options": options,
		})
	case "contact":
		if rich.Contact == nil || rich.Contact.Name == "" {
			return errors.New("contato sem nome")
		}
		vcard := vcardPayload(rich.Contact.Name, rich.Contact.Phone)
		return n.post(ctx, baseURL, token, "/chat/send/contact", map[string]interface{}{
			"phone": phone, "name": rich.Contact.Name, "vcard": vcard,
		})
	case "location":
		if rich.Location == nil {
			return errors.New("localização vazia")
		}
		return n.post(ctx, baseURL, token, "/chat/send/location", map[string]interface{}{
			"phone": phone, "name": rich.Location.Name, "latitude": rich.Location.Latitude, "longitude": rich.Location.Longitude,
		})
	case "sticker":
		if strings.TrimSpace(rich.Sticker) == "" {
			return errors.New("figurinha sem mídia")
		}
		return n.post(ctx, baseURL, token, "/chat/send/sticker", map[string]interface{}{
			"phone": phone, "sticker": rich.Sticker,
		})
	default:
		return errors.New("tipo rico não suportado: " + rich.Kind)
	}
}

// vcardPayload monta um vCard minimalista a partir de nome e telefone.
func vcardPayload(name, phone string) string {
	name = strings.TrimSpace(name)
	phone = strings.TrimSpace(phone)
	if phone != "" && !strings.Contains(phone, "+") {
		phone = "+" + phone
	}
	return "BEGIN:VCARD\nVERSION:3.0\nN:" + name + ";;;\nFN:" + name + "\nTEL;TYPE=CELL:" + phone + "\nEND:VCARD"
}

// extractMsgID tenta extrair o id da mensagem retornado pela API (formato varia).
func extractMsgID(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var m map[string]interface{}
	if json.Unmarshal(body, &m) != nil {
		return ""
	}
	for _, k := range []string{"id", "message_id", "wa_id", "msg_id", "Id", "ID", "key"} {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				if t != "" {
					return t
				}
			case map[string]interface{}:
				if s, ok := t["id"].(string); ok && s != "" {
					return s
				}
			}
		}
	}
	return ""
}

// zeroWidthChars são caracteres invisíveis usados para diferenciar mensagens
// em campanhas (visual idêntico, bytes diferentes) — anti-dedupe.
var zeroWidthChars = []rune{'\u200B', '\u200C', '\u200D', '\u2060'}

// PerturbaTexto insere caracteres invisíveis para que cada envio de campanha
// seja único no nível de bytes. Se variar=false, retorna o texto original.
func PerturbaTexto(text string, variar bool, seed int64) string {
	if !variar || text == "" {
		return text
	}
	r := rand.New(rand.NewSource(seed))
	n := 1
	if seed%2 == 0 {
		n = 2
	}
	runes := []rune(text)
	var out []rune
	pos := 0
	for i := 0; i < len(runes); i++ {
		if n > 0 && (r.Intn(len(runes)) < 1 || i == len(runes)-1) && pos < n {
			out = append(out, zeroWidthChars[r.Intn(len(zeroWidthChars))])
			pos++
		}
		out = append(out, runes[i])
	}
	for pos < n {
		out = append(out, zeroWidthChars[r.Intn(len(zeroWidthChars))])
		pos++
	}
	return string(out)
}

// Media descreve uma mídia a enviar pelo painel.
type Media struct {
	Type     string // image | audio | document | video
	DataB64  string // base64 bruto (sem prefixo)
	MimeType string
	FileName string
	Caption  string
}

// SendMedia envia imagem, áudio, vídeo ou documento (assinatura usada pelo painel).
func (n *Notreve) SendMedia(ctx context.Context, phone, mediaType, dataB64, mimeType, fileName, caption string) error {
	return n.sendMediaStruct(ctx, phone, Media{
		Type: mediaType, DataB64: dataB64, MimeType: mimeType, FileName: fileName, Caption: caption,
	})
}

// sendMediaStruct é a versão com struct.
func (n *Notreve) sendMediaStruct(ctx context.Context, phone string, m Media) error {
	ch, err := n.channel(ctx)
	if err != nil || strings.TrimRight(ch.BaseURL, "/") == "" || ch.Token == "" {
		return errors.New("WhatsApp não configurado no painel")
	}
	baseURL := strings.TrimRight(ch.BaseURL, "/")
	token := ch.Token

	endpoint := "/chat/send/" + m.Type
	payload := map[string]interface{}{"phone": phone}

	switch m.Type {
	case "image":
		payload["image"] = dataURI(m.DataB64, m.MimeType, "image/jpeg")
		if m.Caption != "" {
			payload["caption"] = m.Caption
		}
	case "audio":
		payload["audio"] = dataURI(m.DataB64, m.MimeType, "audio/ogg")
	case "video":
		payload["video"] = dataURI(m.DataB64, m.MimeType, "video/mp4")
		if m.Caption != "" {
			payload["caption"] = m.Caption
		}
	case "document":
		// A API exige o data URI com application/octet-stream para documentos.
		payload["document"] = dataURI(m.DataB64, "application/octet-stream", "application/octet-stream")
		payload["file_name"] = m.FileName
	default:
		return errors.New("tipo de mídia inválido")
	}
	return n.post(ctx, baseURL, token, endpoint, payload)
}

// SendMediaParams é a versão com parâmetros simples (usada pelo painel).
func (n *Notreve) SendMediaParams(ctx context.Context, phone, mediaType, dataB64, mimeType, fileName, caption string) error {
	return n.SendMedia(ctx, phone, mediaType, dataB64, mimeType, fileName, caption)
}

// SendEdit edita uma mensagem já enviada pela própria instância.
func (n *Notreve) SendEdit(ctx context.Context, phone, waMsgID, newText string) error {
	ch, err := n.channel(ctx)
	if err != nil || strings.TrimRight(ch.BaseURL, "/") == "" || ch.Token == "" {
		return errors.New("WhatsApp não configurado no painel")
	}
	return n.SendEditChannel(ctx, ch, phone, waMsgID, newText)
}

func (n *Notreve) SendEditChannel(ctx context.Context, ch store.Channel, phone, waMsgID, newText string) error {
	if ch.Type != "whatsapp" || strings.TrimRight(ch.BaseURL, "/") == "" || ch.Token == "" {
		return errors.New("canal WhatsApp não configurado")
	}
	baseURL := strings.TrimRight(ch.BaseURL, "/")
	token := ch.Token
	return n.post(ctx, baseURL, token, "/chat/send/edit", map[string]interface{}{
		"id": waMsgID, "phone": phone, "body": newText,
	})
}

// SendTyping liga/desliga o indicador "digitando..." pro cliente ver.
func (n *Notreve) SendTyping(ctx context.Context, phone string, typing bool) error {
	ch, err := n.channel(ctx)
	if err != nil || strings.TrimRight(ch.BaseURL, "/") == "" || ch.Token == "" {
		return errors.New("WhatsApp não configurado no painel")
	}
	return n.SendTypingChannel(ctx, ch, phone, typing)
}

func (n *Notreve) SendTypingChannel(ctx context.Context, ch store.Channel, phone string, typing bool) error {
	if ch.Type != "whatsapp" || strings.TrimRight(ch.BaseURL, "/") == "" || ch.Token == "" {
		return errors.New("canal WhatsApp não configurado")
	}
	baseURL := strings.TrimRight(ch.BaseURL, "/")
	token := ch.Token
	state := "paused"
	if typing {
		state = "composing"
	}
	return n.post(ctx, baseURL, token, "/chat/presence", map[string]interface{}{
		"phone": phone, "state": state,
	})
}

// SendReaction reage a uma mensagem com um emoji.
func (n *Notreve) SendReaction(ctx context.Context, phone, waMsgID, emoji string) error {
	ch, err := n.channel(ctx)
	if err != nil || strings.TrimRight(ch.BaseURL, "/") == "" || ch.Token == "" {
		return errors.New("WhatsApp não configurado no painel")
	}
	return n.SendReactionChannel(ctx, ch, phone, waMsgID, emoji)
}

func (n *Notreve) SendReactionChannel(ctx context.Context, ch store.Channel, phone, waMsgID, emoji string) error {
	if ch.Type != "whatsapp" || strings.TrimRight(ch.BaseURL, "/") == "" || ch.Token == "" {
		return errors.New("canal WhatsApp não configurado")
	}
	baseURL := strings.TrimRight(ch.BaseURL, "/")
	token := ch.Token
	return n.post(ctx, baseURL, token, "/chat/react", map[string]interface{}{
		"phone": phone, "body": emoji, "id": waMsgID,
	})
}

// handlePresence processa o evento de digitação do cliente e marca a conversa.
type chatPresenceEvent struct {
	State         string `json:"State"`
	Chat          string `json:"Chat"`
	Sender        string `json:"Sender"`
	SenderAlt     string `json:"SenderAlt"`
	MessageSource struct {
		Chat     string `json:"Chat"`
		IsFromMe bool   `json:"IsFromMe"`
	} `json:"MessageSource"`
}

// handleReadReceipt marca mensagens de campanha como lidas (evento ReadReceipt).
func (n *Notreve) handleReadReceipt(ctx context.Context, raw []byte) {
	var evt struct {
		Event struct {
			Info struct {
				ID string `json:"ID"`
			} `json:"Info"`
			Receipts []struct {
				MessageIDs []string `json:"MessageIDs"`
				Status     string   `json:"Status"`
				ID         string   `json:"ID"`
			} `json:"Receipts"`
		} `json:"event"`
	}
	if err := json.Unmarshal(raw, &evt); err != nil {
		return
	}
	seen := map[string]bool{}
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			_ = n.st.MarkEnvioLida(ctx, id)
		}
	}
	add(evt.Event.Info.ID)
	for _, r := range evt.Event.Receipts {
		for _, mid := range r.MessageIDs {
			add(mid)
		}
		add(r.ID)
	}
}

func (n *Notreve) handlePresence(ctx context.Context, raw []byte) {
	var evt chatPresenceEvent
	if err := json.Unmarshal(raw, &evt); err != nil {
		// tenta dentro de "event"
		var p struct {
			Event json.RawMessage `json:"event"`
		}
		if err2 := json.Unmarshal(raw, &p); err2 == nil {
			_ = json.Unmarshal(p.Event, &evt)
		}
	}
	if evt.MessageSource.IsFromMe {
		return
	}
	phone := resolvePhone(evt.SenderAlt, evt.Sender, evt.Chat, evt.MessageSource.Chat)
	if phone == "" {
		return
	}
	conv, err := n.st.GetOpenConversationByPhone(ctx, phone)
	if err != nil {
		return
	}
	_ = n.st.SetConversationTyping(ctx, conv.ID, evt.State == "composing")
}

// handleCall responde às chamadas de voz/vídeo: avisa que o atendimento
// é apenas por mensagem (configurável por canal e com texto personalizado).
func (n *Notreve) handleCall(ctx context.Context, raw []byte, baseURL, token string, ch store.Channel) {
	if !ch.ResponderChamadas {
		Logger.Printf("chamada recebida com resposta automática desativada no canal %q", ch.Name)
		return
	}
	var p struct {
		Event json.RawMessage `json:"event"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		Logger.Printf("chamada recebida com payload inválido: %v", err)
		return
	}
	evt, err := decodeNotreveCallEvent(p.Event)
	if err != nil {
		Logger.Printf("chamada recebida com evento inválido: %v", err)
		return
	}
	creator, callID := evt.values()
	if creator == "" || callID == "" {
		evt, err = decodeNotreveCallEvent(raw)
		if err != nil {
			Logger.Printf("chamada recebida com corpo inválido: %v", err)
			return
		}
		creator, callID = evt.values()
	}
	resolvedCreator, phone := callEventPhone(evt)
	if creator == "" && resolvedCreator != "" {
		creator = resolvedCreator
	}
	if phone == "" {
		Logger.Printf("chamada recebida sem telefone identificável")
		return
	}
	callFrom := callRejectJID(creator, phone)
	rejected := false
	if callID == "" || callFrom == "" {
		Logger.Printf("chamada de %s sem call_from/call_id; não foi possível rejeitar", phone)
	} else if err := n.post(ctx, baseURL, token, "/call/reject", map[string]interface{}{
		"call_from": callFrom,
		"call_id":   callID,
	}); err != nil {
		Logger.Printf("erro rejeitando chamada de %s: %v", phone, err)
	} else {
		rejected = true
		Logger.Printf("chamada de %s rejeitada", phone)
	}

	text := n.chamadaMensagem(ctx)
	if text == "" {
		text = "Olá! Aqui atendemos apenas por mensagem no WhatsApp. Como posso te ajudar por aqui? 😊"
	}

	if err := n.post(ctx, baseURL, token, "/chat/send/text", map[string]interface{}{"phone": phone, "body": whatsappSanitize(text)}); err != nil {
		Logger.Printf("erro respondendo chamada de %s: %v", phone, err)
		return
	}
	Logger.Printf("resposta automática de chamada enviada para %s", phone)

	conv, err := n.st.GetOpenConversationByPhone(ctx, phone)
	if err != nil {
		Logger.Printf("chamada de %s sem conversa aberta; nada registrado", phone)
		return
	}
	convID := conv.ID
	status := "📞 Chamada de voz/vídeo recebida (resposta automática enviada)"
	if rejected {
		status = "📞 Chamada de voz/vídeo recebida (ligação rejeitada e resposta automática enviada)"
	}
	if err := n.st.AddMessage(ctx, store.Message{ConversationID: convID, Role: "system", Content: status, Source: "system"}); err != nil {
		Logger.Printf("erro ao registrar chamada (system) na conversa %d: %v", convID, err)
	}
	if err := n.st.AddMessage(ctx, store.Message{ConversationID: convID, Role: "assistant", Content: text, Source: "human"}); err != nil {
		Logger.Printf("erro ao registrar chamada (assistant) na conversa %d: %v", convID, err)
	}
	if err := n.st.TouchConversation(ctx, convID); err != nil {
		Logger.Printf("erro ao atualizar conversa %d na chamada: %v", convID, err)
	}
}

func (n *Notreve) chamadaMensagem(ctx context.Context) string {
	msgs, err := n.st.ListMensagens(ctx)
	if err != nil {
		return ""
	}
	for _, m := range msgs {
		if m.Chave == "chamada" {
			return strings.TrimSpace(m.Texto)
		}
	}
	return ""
}

func (n *Notreve) post(ctx context.Context, baseURL, token, path string, payload map[string]interface{}) error {
	_, err := n.postRaw(ctx, baseURL, token, path, payload)
	return err
}

func (n *Notreve) postRaw(ctx context.Context, baseURL, token, path string, payload map[string]interface{}) ([]byte, error) {
	// Trava de segurança: qualquer envio (texto, mídia, edição, ligação) para
	// número fora da allowlist é bloqueado silenciosamente.
	if strings.Contains(path, "/chat/send/") {
		phones := phoneList(payload["phone"])
		for _, ph := range phones {
			allowed, _ := n.phoneAllowed(ctx, baseURL, token, ph)
			if !allowed {
				Logger.Printf("envio bloqueado (destino fora da allowlist)")
				return nil, errors.New("destino fora da lista de números permitidos")
			}
		}
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("token", token)
	resp, err := n.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, maxWhatsAppResponse+1))
	if len(b) > maxWhatsAppResponse {
		return nil, errors.New("resposta da API WhatsApp excede o limite")
	}
	if resp.StatusCode >= 300 {
		return b, fmt.Errorf("API WhatsApp %d: %s", resp.StatusCode, truncateBytes(b, 300))
	}
	return b, nil
}

// phoneList normaliza o campo Phone do payload (pode ser string ou []string).
func phoneList(v interface{}) []string {
	switch t := v.(type) {
	case string:
		if t != "" {
			return []string{t}
		}
	case []string:
		return t
	case []interface{}:
		var out []string
		for _, x := range t {
			if s, ok := x.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func dataURI(b64, mime, fallback string) string {
	if mime == "" {
		mime = fallback
	}
	return "data:" + mime + ";base64," + b64
}

// whatsappSanitize adapta texto pro WhatsApp: converte markdown (como os LLMs
// costumam responder) para a formatação do WhatsApp e preserva o conteúdo,
// mantendo código (``` e `) intacto. Porte do markdown_para_whatsapp (Python).
var (
	waCodeBlock  = regexp.MustCompile("(?s)```.*?```")
	waInlineCode = regexp.MustCompile("`[^`\n]+`")
	waHeader     = regexp.MustCompile(`(?m)^#{1,6}[ \t]+(.+?)[ \t]*$`)
	waBoldTriple = regexp.MustCompile(`\*\*\*(.+?)\*\*\*`)
	waBoldDouble = regexp.MustCompile(`\*\*(.+?)\*\*`)
	waBoldUnder  = regexp.MustCompile(`__(.+?)__`)
	waStrike     = regexp.MustCompile(`~~(.+?)~~`)
	waImage      = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	waLink       = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	waRule       = regexp.MustCompile(`(?m)^[ \t]*[-*_]{3,}[ \t]*$`)
	waQuote      = regexp.MustCompile(`(?m)^[ \t]*>[ \t]?`)
)

func whatsappSanitize(s string) string {
	if strings.TrimSpace(s) == "" {
		return s
	}
	var codeBlocks, inlineCodes, headers []string
	s = waCodeBlock.ReplaceAllStringFunc(s, func(m string) string {
		codeBlocks = append(codeBlocks, m)
		return fmt.Sprintf("\x00CODEBLOCK%d\x00", len(codeBlocks)-1)
	})
	s = waInlineCode.ReplaceAllStringFunc(s, func(m string) string {
		inlineCodes = append(inlineCodes, m)
		return fmt.Sprintf("\x00INLINE%d\x00", len(inlineCodes)-1)
	})
	s = waHeader.ReplaceAllStringFunc(s, func(m string) string {
		sub := waHeader.FindStringSubmatch(m)
		headers = append(headers, "*"+strings.TrimSpace(sub[1])+"*")
		return fmt.Sprintf("\x00HEADER%d\x00", len(headers)-1)
	})
	s = waBoldTriple.ReplaceAllString(s, "*_${1}_*")
	s = waBoldDouble.ReplaceAllString(s, "*${1}*")
	s = waBoldUnder.ReplaceAllString(s, "*${1}*")
	s = waStrike.ReplaceAllString(s, "~${1}~")
	s = waImage.ReplaceAllString(s, "${1} - ${2}")
	s = waLink.ReplaceAllString(s, "${1} (${2})")
	s = waRule.ReplaceAllString(s, "─────────────────")
	s = waQuote.ReplaceAllString(s, "")
	for i, c := range headers {
		s = strings.ReplaceAll(s, fmt.Sprintf("\x00HEADER%d\x00", i), c)
	}
	for i, c := range inlineCodes {
		s = strings.ReplaceAll(s, fmt.Sprintf("\x00INLINE%d\x00", i), c)
	}
	for i, c := range codeBlocks {
		s = strings.ReplaceAll(s, fmt.Sprintf("\x00CODEBLOCK%d\x00", i), c)
	}
	return strings.TrimSpace(s)
}

func phoneFromJID(jid string) string {
	jid = strings.TrimSpace(jid)
	lower := strings.ToLower(jid)
	for _, suffix := range []string{"@lid", "@g.us", "@broadcast", "@newsletter"} {
		if strings.Contains(lower, suffix) {
			return ""
		}
	}
	user := jid
	if i := strings.Index(user, "@"); i > 0 {
		user = user[:i]
	}
	if i := strings.Index(user, "."); i > 0 {
		user = user[:i]
	}
	if i := strings.Index(user, ":"); i > 0 {
		user = user[:i]
	}
	return digitsOnly(user)
}

// whatsappChannelChat identifica canais/newsletters, que não são conversas
// privadas e não devem receber mensagens de resposta.
func whatsappChannelChat(evt notreveEvent) bool {
	switch strings.ToLower(strings.TrimSpace(evt.Info.Type)) {
	case "channel", "channels", "broadcast", "newsletter", "status":
		return true
	}
	for _, jid := range []string{evt.Info.ChatJID, evt.Info.Chat, evt.Info.Sender, evt.Info.SenderAlt} {
		value := strings.ToLower(strings.TrimSpace(jid))
		if value == "status@broadcast" || strings.HasSuffix(value, "@broadcast") || strings.HasSuffix(value, "@newsletter") {
			return true
		}
	}
	return false
}

// whatsappGroupChat identifica grupos mesmo quando a API não envia o booleano
// IsGroup. O JID de grupo termina em @g.us.
func whatsappGroupChat(evt notreveEvent) bool {
	if evt.Info.IsGroup {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(evt.Info.Type)) {
	case "group", "groups", "community", "comunidade":
		return true
	}
	for _, jid := range []string{evt.Info.ChatJID, evt.Info.Chat, evt.Info.Sender, evt.Info.SenderAlt} {
		value := strings.ToLower(strings.TrimSpace(jid))
		if strings.HasSuffix(value, "@g.us") {
			return true
		}
	}
	return false
}

// resolvePhone pega o telefone real do remetente, ignorando LIDs (@lid).
// O SenderAlt contém o número verdadeiro (ex.: <phone>@s.whatsapp.net).
func resolvePhone(candidates ...string) string {
	for _, c := range candidates {
		if c == "" {
			continue
		}
		phone := phoneFromJID(c)
		if isRealPhone(phone) {
			return phone
		}
	}
	return ""
}

func resolvePhoneAvoidingLID(lidJID string, candidates ...string) string {
	lidDigits := digitsOnly(lidJID)
	for _, candidate := range candidates {
		phone := resolvePhone(candidate)
		if phone != "" && (lidDigits == "" || phone != lidDigits) {
			return phone
		}
	}
	return ""
}

func digitsOnly(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func normalizeLID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	user := value
	if i := strings.Index(user, "@"); i >= 0 {
		if user[i:] != "@lid" {
			return ""
		}
		user = user[:i]
	}
	if i := strings.Index(user, ":"); i > 0 {
		user = user[:i]
	}
	user = digitsOnly(user)
	if user == "" {
		return ""
	}
	return user + "@lid"
}

func lidFromCandidates(candidates ...string) string {
	for _, candidate := range candidates {
		if strings.Contains(strings.ToLower(candidate), "@lid") {
			if lid := normalizeLID(candidate); lid != "" {
				return lid
			}
		}
	}
	return ""
}

func replyJIDOf(candidates ...string) string {
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		lower := strings.ToLower(candidate)
		if strings.Contains(lower, "@lid") || strings.Contains(lower, "@s.whatsapp.net") {
			return candidate
		}
	}
	return ""
}

func isRealPhone(phone string) bool {
	if phone == "" {
		return false
	}
	for _, r := range phone {
		if r < '0' || r > '9' {
			return false
		}
	}
	if len(phone) < 12 || len(phone) > 15 {
		return false
	}
	if allSameDigit(phone) {
		return false
	}
	if !strings.HasPrefix(phone, "55") {
		return false
	}
	return validBrazilDDD(phone[2:4])
}

func allSameDigit(value string) bool {
	if value == "" {
		return false
	}
	for i := 1; i < len(value); i++ {
		if value[i] != value[0] {
			return false
		}
	}
	return true
}

func validBrazilDDD(ddd string) bool {
	if len(ddd) != 2 {
		return false
	}
	code := int(ddd[0]-'0')*10 + int(ddd[1]-'0')
	if code < 11 || code > 99 {
		return false
	}
	switch code {
	case 20, 23, 25, 26, 29, 30, 36, 39, 40, 50, 52, 56, 57, 58, 59, 60, 70, 76, 78, 80, 90:
		return false
	}
	return true
}

func whatsappConversationPhoneValid(phone string) bool {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return false
	}
	lower := strings.ToLower(phone)
	if strings.Contains(lower, "@") {
		return strings.HasSuffix(lower, "@g.us")
	}
	return isRealPhone(phone)
}

// phoneAllowed diz se o número pode receber respostas da IA.
// A lista vem de settings.numeros_permitidos (um número por linha).
// Vazia = responde para qualquer número (para testes locais).
func (n *Notreve) phoneAllowed(ctx context.Context, baseURL, token, phone string) (bool, string) {
	st, err := n.st.GetSettings(ctx)
	if err != nil {
		return false, ""
	}
	if strings.TrimSpace(st.NumerosPermitidos) == "" {
		return true, phone
	}
	want := phoneFromJID(phone)
	incomingLID := normalizeLID(phone)
	for _, line := range strings.Split(st.NumerosPermitidos, "\n") {
		allowedPhone := digitsOnly(line)
		if allowedPhone == "" {
			continue
		}
		if want != "" && want == allowedPhone {
			return true, allowedPhone
		}
		if incomingLID == "" {
			continue
		}
		if n.cachedPhoneForLID(store.TenantID(ctx), incomingLID) == allowedPhone {
			return true, allowedPhone
		}
		mappedLID, mapErr := InstanciaPhoneToLID(ctx, baseURL, token, allowedPhone)
		if mapErr == nil && normalizeLID(mappedLID) == incomingLID {
			n.rememberLID(store.TenantID(ctx), incomingLID, allowedPhone)
			return true, allowedPhone
		}
	}
	return false, ""
}
