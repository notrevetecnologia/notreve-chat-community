package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// InstanciaAPI conversa com a Notreve WhatsApp API para gerenciar
// instâncias: criar, conectar (QR), status, desconectar e webhook.
// Autenticação: instância -> header `token`; admin -> header `Authorization`.

type instanciaResp struct {
	Code    int             `json:"code"`
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
}

var instanciaHTTP = &http.Client{Transport: secretAwareHTTPTransport, Timeout: 30 * time.Second}

const maxInstanciaResponse = 32 << 20

func instanciaReq(ctx context.Context, method, baseURL, token, adminToken, path string, payload map[string]interface{}) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(baseURL, "/")+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if adminToken != "" {
		req.Header.Set("Authorization", adminToken)
	} else {
		req.Header.Set("token", token)
	}
	resp, err := instanciaHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, maxInstanciaResponse+1))
	if len(b) > maxInstanciaResponse {
		return nil, errors.New("resposta da API WhatsApp excede o limite")
	}
	if resp.StatusCode >= 300 {
		return b, fmt.Errorf("API WhatsApp %d: %s", resp.StatusCode, truncateBytes(b, 300))
	}
	return b, nil
}

func decodeInstanciaData(b []byte, out interface{}) error {
	var r instanciaResp
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	if len(r.Data) > 0 {
		return json.Unmarshal(r.Data, out)
	}
	return nil
}

type InstanciaUsuario struct {
	ID        string
	Name      string
	Token     string
	Connected bool
	LoggedIn  bool
}

// InstanciaCriar cria um usuário/instância via admin token e registra o webhook.
func InstanciaCriar(ctx context.Context, baseURL, adminToken, name, token, webhook string, events []string) error {
	_, err := InstanciaCriarID(ctx, baseURL, adminToken, name, token, webhook, events)
	return err
}

// InstanciaCriarID cria uma instância e retorna o ID atribuído pela API.
func InstanciaCriarID(ctx context.Context, baseURL, adminToken, name, token, webhook string, events []string) (string, error) {
	payload := map[string]interface{}{
		"name": name, "token": token,
		// A Notreve API define events como string; All é a inscrição suportada
		// para manter todos os eventos configurados na aplicação.
		"webhook_url": webhook, "events": "All", "history": 0,
	}
	b, err := instanciaReq(ctx, http.MethodPost, baseURL, "", adminToken, "/admin/users", payload)
	if err != nil {
		return "", err
	}
	var data map[string]interface{}
	if err := decodeInstanciaData(b, &data); err != nil {
		return "", err
	}
	return instanciaIDFromMap(data), nil
}

// InstanciaEncontrarPorNome consulta as instâncias já existentes antes de
// criar uma nova. Isso torna o botão Conectar idempotente mesmo após reinício
// do app, quando o banco ainda não tem o token local.
func InstanciaEncontrarPorNome(ctx context.Context, baseURL, adminToken, name string) (InstanciaUsuario, bool, error) {
	b, err := instanciaReq(ctx, http.MethodGet, baseURL, "", adminToken, "/admin/users", nil)
	if err != nil {
		return InstanciaUsuario{}, false, err
	}
	var users []map[string]interface{}
	if err := decodeInstanciaData(b, &users); err != nil {
		return InstanciaUsuario{}, false, err
	}
	for _, raw := range users {
		item := instanciaUserFromMap(raw)
		if strings.EqualFold(strings.TrimSpace(item.Name), strings.TrimSpace(name)) {
			return item, true, nil
		}
	}
	return InstanciaUsuario{}, false, nil
}

func instanciaUserFromMap(raw map[string]interface{}) InstanciaUsuario {
	return InstanciaUsuario{
		ID:        instanciaString(raw, "id", "ID", "user_id", "userId"),
		Name:      instanciaString(raw, "name", "Name"),
		Token:     instanciaString(raw, "token", "Token"),
		Connected: instanciaBool(raw, "connected", "Connected"),
		LoggedIn:  instanciaBool(raw, "loggedIn", "LoggedIn"),
	}
}

func instanciaIDFromMap(raw map[string]interface{}) string {
	return instanciaString(raw, "id", "ID", "user_id", "userId")
}

func instanciaString(raw map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := raw[key]; ok {
			switch typed := value.(type) {
			case string:
				return strings.TrimSpace(typed)
			case float64:
				return strings.TrimSpace(fmt.Sprintf("%.0f", typed))
			}
		}
	}
	return ""
}

func instanciaBool(raw map[string]interface{}, keys ...string) bool {
	for _, key := range keys {
		if value, ok := raw[key].(bool); ok {
			return value
		}
	}
	return false
}

// InstanciaConectar inicia a conexão (gera QR se não houver sessão).
func InstanciaConectar(ctx context.Context, baseURL, token string, subscribe []string, immediate bool) error {
	payload := map[string]interface{}{"immediate": immediate}
	if len(subscribe) > 0 {
		payload["subscribe"] = subscribe
	}
	_, err := instanciaReq(ctx, http.MethodPost, baseURL, token, "", "/session/connect", payload)
	return err
}

// InstanciaQR retorna o QR (data URI PNG base64) ou "" se vazio.
func InstanciaQR(ctx context.Context, baseURL, token string) (string, bool, error) {
	b, err := instanciaReq(ctx, http.MethodGet, baseURL, token, "", "/session/qr", nil)
	if err != nil {
		return "", false, err
	}
	var d struct {
		QRCode         string `json:"qr_code"`
		QRCodeLegacy   string `json:"QRCode"`
		PasskeyPending bool   `json:"passkey_pending"`
		PasskeyLegacy  bool   `json:"passkeyPending"`
	}
	if err := decodeInstanciaData(b, &d); err != nil {
		return "", false, err
	}
	if d.QRCode == "" {
		d.QRCode = d.QRCodeLegacy
	}
	return d.QRCode, d.PasskeyPending || d.PasskeyLegacy, nil
}

// InstanciaStatus retorna conectado/logado e o JID da instância.
func InstanciaStatus(ctx context.Context, baseURL, token string) (connected, loggedIn bool, jid, name, instanceID string, err error) {
	b, err := instanciaReq(ctx, http.MethodGet, baseURL, token, "", "/session/status", nil)
	if err != nil {
		return false, false, "", "", "", err
	}
	var d struct {
		Connected      bool   `json:"connected"`
		LoggedIn       bool   `json:"logged_in"`
		LoggedInLegacy bool   `json:"loggedIn"`
		JID            string `json:"jid"`
		Name           string `json:"name"`
		ID             string `json:"id"`
	}
	_ = decodeInstanciaData(b, &d)
	return d.Connected, d.LoggedIn || d.LoggedInLegacy, d.JID, d.Name, d.ID, nil
}

// InstanciaDesconectar desconecta preservando a sessão (reconecta sem QR).
func InstanciaDesconectar(ctx context.Context, baseURL, token string) error {
	_, err := instanciaReq(ctx, http.MethodPost, baseURL, token, "", "/session/disconnect", map[string]interface{}{})
	return err
}

// InstanciaLogout encerra a sessão (próxima conexão exige QR).
func InstanciaLogout(ctx context.Context, baseURL, token string) error {
	_, err := instanciaReq(ctx, http.MethodPost, baseURL, token, "", "/session/logout", map[string]interface{}{})
	return err
}

// InstanciaSetLimits define limites de envio da instância (proteção anti-ban).
func InstanciaSetLimits(ctx context.Context, baseURL, token string, perHour, perDay int) error {
	payload := map[string]interface{}{}
	if perHour > 0 {
		payload["msg_limit_hour"] = perHour
	}
	if perDay > 0 {
		payload["msg_limit_day"] = perDay
	}
	if len(payload) == 0 {
		return nil
	}
	_, err := instanciaReq(ctx, http.MethodPost, baseURL, token, "", "/session/limits", payload)
	return err
}

// InstanciaSetWebhook registra a URL e os eventos no webhook da instância.
// O contrato atual da Notreve usa webhook_url e events.
func InstanciaSetWebhook(ctx context.Context, baseURL, token, webhookURL string, events []string) error {
	payload := map[string]interface{}{
		"webhook_url": webhookURL, "events": events,
	}
	_, err := instanciaReq(ctx, http.MethodPost, baseURL, token, "", "/webhook", payload)
	return err
}

// InstanciaSetMediaDelivery entrega a mídia em base64 no webhook (entender áudio/imagem/PDF).
func InstanciaSetMediaDelivery(ctx context.Context, baseURL, token string) error {
	_, err := instanciaReq(ctx, http.MethodPost, baseURL, token, "", "/session/media/config",
		map[string]interface{}{"media_delivery": "base64"})
	return err
}

// InstanciaAvatar retorna a URL da foto de perfil de um número (ex.: o próprio JID da instância).
func InstanciaAvatar(ctx context.Context, baseURL, token, phone string) (string, error) {
	if phone == "" {
		return "", nil
	}
	b, err := instanciaReq(ctx, http.MethodPost, baseURL, token, "", "/user/avatar",
		map[string]interface{}{"phone": phone, "preview": true})
	if err != nil {
		return "", err
	}
	var d struct {
		URL       string `json:"url"`
		URLLegacy string `json:"URL"`
	}
	if err := json.Unmarshal(b, &d); err != nil {
		return "", err
	}
	if d.URL == "" {
		d.URL = d.URLLegacy
	}
	return d.URL, nil
}

// InstanciaPhoneToLID consulta o LID associado a um telefone. O endpoint faz
// somente PN -> LID; a direção inversa deve vir do SenderAlt no webhook.
func InstanciaPhoneToLID(ctx context.Context, baseURL, token, phone string) (string, error) {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return "", nil
	}
	b, err := instanciaReq(ctx, http.MethodGet, baseURL, token, "", "/user/lid/"+phone, nil)
	if err != nil {
		return "", err
	}
	var d struct {
		LID string `json:"lid"`
	}
	if err := decodeInstanciaData(b, &d); err != nil {
		return "", err
	}
	return d.LID, nil
}

// InstanciaDownloadMedia baixa a mídia de uma mensagem (base64), se a entrega
// inline não veio no webhook. mediaType: image|video|audio|document.
func InstanciaDownloadMedia(ctx context.Context, baseURL, token, mediaType string, md mediaMsg) (string, string, error) {
	if md.Url == "" || md.MediaKey == "" || md.FileSHA256 == "" {
		return "", "", errors.New("mídia sem dados de download")
	}
	payload := map[string]interface{}{
		"url": md.Url, "media_key": md.MediaKey, "mime_type": md.MimeType,
		"file_sha256": md.FileSHA256, "file_length": md.FileLength,
	}
	if md.DirectPath != "" {
		payload["direct_path"] = md.DirectPath
	}
	if md.FileEncSHA256 != "" {
		payload["file_enc_sha256"] = md.FileEncSHA256
	}
	b, err := instanciaReq(ctx, http.MethodPost, baseURL, token, "", "/chat/download"+mediaType, payload)
	if err != nil {
		return "", "", err
	}
	// A resposta pode vir como {code,data:{Data:"data:...;base64,..."}} OU
	// {code,data:{Data:{data/base64/Data: "...", mimeType: "..."}}}. Aceita os dois.
	raw := b
	var wrap struct {
		Data struct {
			Raw json.RawMessage `json:"Data"`
		} `json:"data"`
	}
	if json.Unmarshal(b, &wrap) == nil && len(wrap.Data.Raw) > 0 {
		raw = wrap.Data.Raw
	}
	data, mime := extractBase64Data(raw)
	if data == "" {
		return "", "", errors.New("download sem conteúdo")
	}
	return data, mime, nil
}

// extractBase64Data extrai o base64 (e mime) de uma resposta de download, seja
// string ("data:image/jpeg;base64,...") ou objeto com campos de dados.
func extractBase64Data(raw json.RawMessage) (string, string) {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return parseDataURI(s)
	}
	var obj map[string]interface{}
	if json.Unmarshal(raw, &obj) != nil {
		return "", ""
	}
	mime, _ := obj["mime_type"].(string)
	if mime == "" {
		mime, _ = obj["Mimetype"].(string)
	}
	if mime == "" {
		mime, _ = obj["mimeType"].(string)
	}
	for _, k := range []string{"data", "base64", "Data", "Base64", "url", "URL"} {
		if v, ok := obj[k].(string); ok && v != "" {
			d, m := parseDataURI(v)
			if d != "" {
				if mime == "" {
					mime = m
				}
				return d, mime
			}
		}
	}
	return "", ""
}

func parseDataURI(s string) (string, string) {
	i := strings.Index(s, ";base64,")
	if i >= 0 {
		return s[i+len(";base64,"):], strings.TrimPrefix(strings.Split(s[5:i], ";")[0], "data:")
	}
	if strings.HasPrefix(s, "data:") {
		j := strings.Index(s, ",")
		if j >= 0 {
			return s[j+1:], ""
		}
	}
	return s, ""
}

var _ = errors.New
