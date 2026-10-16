package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

type Message struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type Choice struct {
	Index   int     `json:"index"`
	Message Message `json:"message"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type ChatResponse struct {
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Client struct {
	BaseURL     string
	APIKey      string
	Model       string
	Temperature float64
	HTTP        *http.Client
}

const (
	NotreveBaseURL     = "https://api.ia.notreve.com.br/v1"
	NotreveChatModel   = "notreve-v1-pro"
	NotreveVisionModel = "notreve-vision"
	NotreveSTTModel    = "notreve-transcribe"
)

const maxResponseBody = 8 << 20
const maxCompleteAttempts = 3

func NewClient(baseURL, apiKey, model string, temperature float64) *Client {
	return &Client{
		BaseURL:     strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		APIKey:      apiKey,
		Model:       model,
		Temperature: temperature,
		HTTP:        &http.Client{Timeout: 60 * time.Second},
	}
}

// Complete sends the conversation and returns the assistant message
// (which may contain tool_calls). Falhas transitórias são repetidas e alguns
// providers/modelos que rejeitam temperature ganham uma segunda tentativa sem
// esse parâmetro, mantendo compatibilidade com APIs OpenAI-like mais restritas.
func (c *Client) Complete(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error) {
	if strings.TrimSpace(c.BaseURL) == "" {
		return nil, fmt.Errorf("base_url da IA está vazia")
	}
	if strings.TrimSpace(c.Model) == "" {
		return nil, fmt.Errorf("modelo da IA está vazio")
	}

	includeTemperature := true
	var lastErr error
	for attempt := 0; attempt < maxCompleteAttempts; attempt++ {
		body := map[string]interface{}{
			"model":    c.Model,
			"messages": messages,
		}
		if includeTemperature {
			body["temperature"] = c.Temperature
		}
		if len(tools) > 0 {
			body["tools"] = tools
			body["tool_choice"] = "auto"
		}

		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.APIKey)

		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			if attempt+1 < maxCompleteAttempts && ctx.Err() == nil {
				if !waitRetry(ctx, attempt) {
					return nil, ctx.Err()
				}
				continue
			}
			return nil, err
		}

		data, readErr := readLimited(resp.Body, maxResponseBody)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if attempt+1 < maxCompleteAttempts && ctx.Err() == nil {
				if !waitRetry(ctx, attempt) {
					return nil, ctx.Err()
				}
				continue
			}
			return nil, readErr
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("LLM HTTP %d: %s", resp.StatusCode, truncate(string(data), 500))

			// Alguns modelos de raciocínio/providers OpenAI-compat não aceitam
			// temperature. Nesse caso repetimos sem o campo em vez de encaminhar
			// o cliente indevidamente para atendimento humano.
			if resp.StatusCode == http.StatusBadRequest && includeTemperature && rejectsTemperature(data) {
				includeTemperature = false
				continue
			}
			if retryableStatus(resp.StatusCode) && attempt+1 < maxCompleteAttempts && ctx.Err() == nil {
				if !waitRetry(ctx, attempt) {
					return nil, ctx.Err()
				}
				continue
			}
			return nil, lastErr
		}

		var out ChatResponse
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, fmt.Errorf("decodificando resposta da IA: %w", err)
		}
		if len(out.Choices) == 0 {
			return nil, fmt.Errorf("IA não retornou respostas")
		}
		return &out, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("falha desconhecida ao consultar IA")
	}
	return nil, lastErr
}

func retryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusRequestTimeout || status >= 500
}

func rejectsTemperature(data []byte) bool {
	msg := strings.ToLower(string(data))
	if !strings.Contains(msg, "temperature") {
		return false
	}
	return strings.Contains(msg, "unsupported") || strings.Contains(msg, "not supported") ||
		strings.Contains(msg, "does not support") || strings.Contains(msg, "invalid") ||
		strings.Contains(msg, "unknown") || strings.Contains(msg, "não suport")
}

func waitRetry(ctx context.Context, attempt int) bool {
	delay := time.Duration(attempt+1) * 300 * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// Transcribe converte um áudio (base64) em texto via endpoint compatível com
// a API OpenAI /audio/transcriptions.
func (c *Client) Transcribe(ctx context.Context, baseURL, apiKey, model string, audioB64 []byte, mime, fileName string) (string, error) {
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("file", fileName)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(audioB64); err != nil {
		return "", err
	}
	for key, value := range map[string]string{"model": model, "language": "pt", "response_format": "json"} {
		if err := w.WriteField(key, value); err != nil {
			return "", err
		}
	}
	if err := w.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/audio/transcriptions", body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := readLimited(resp.Body, maxResponseBody)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("STT HTTP %d: %s", resp.StatusCode, truncate(string(data), 300))
	}
	var out struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", err
	}
	return out.Text, nil
}

func readLimited(r io.Reader, limit int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("resposta excede o limite de %d bytes", limit)
	}
	return b, nil
}

type ImageResult struct {
	B64      string
	MimeType string
	URL      string
}

const imageGenerationTimeout = 120 * time.Second

// GenerateImage gera uma imagem via chat completions compatível com OpenRouter
// (modalities ["image","text"] + image_config.aspect_ratio).
func (c *Client) GenerateImage(ctx context.Context, prompt, aspectRatio string) (ImageResult, error) {
	if strings.TrimSpace(c.BaseURL) == "" {
		return ImageResult{}, fmt.Errorf("base_url da IA está vazia")
	}
	if strings.TrimSpace(c.Model) == "" {
		return ImageResult{}, fmt.Errorf("modelo da IA está vazio")
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return ImageResult{}, fmt.Errorf("prompt da imagem está vazio")
	}
	aspectRatio = strings.TrimSpace(aspectRatio)
	if aspectRatio == "" {
		aspectRatio = "1:1"
	}

	body := map[string]interface{}{
		"model":        c.Model,
		"messages":     []map[string]interface{}{{"role": "user", "content": prompt}},
		"modalities":   []string{"image", "text"},
		"image_config": map[string]interface{}{"aspect_ratio": aspectRatio},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return ImageResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return ImageResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: imageGenerationTimeout}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return ImageResult{}, err
	}
	defer resp.Body.Close()
	data, err := readLimited(resp.Body, maxResponseBody)
	if err != nil {
		return ImageResult{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ImageResult{}, fmt.Errorf("geração de imagem HTTP %d: %s", resp.StatusCode, truncate(string(data), 500))
	}
	result, err := parseImageResponse(data)
	if err != nil {
		return ImageResult{}, err
	}
	if result.B64 == "" && result.URL != "" {
		b64, mime, err := fetchImageURL(ctx, httpClient, result.URL)
		if err != nil {
			return ImageResult{}, err
		}
		result.B64, result.MimeType = b64, mime
	}
	return result, nil
}

func fetchImageURL(ctx context.Context, client *http.Client, url string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("download da imagem HTTP %d", resp.StatusCode)
	}
	data, err := readLimited(resp.Body, maxResponseBody)
	if err != nil {
		return "", "", err
	}
	mime := resp.Header.Get("Content-Type")
	if semi := strings.Index(mime, ";"); semi >= 0 {
		mime = mime[:semi]
	}
	if strings.TrimSpace(mime) == "" {
		mime = "image/png"
	}
	return base64.StdEncoding.EncodeToString(data), mime, nil
}

func parseImageResponse(data []byte) (ImageResult, error) {
	var out struct {
		Choices []struct {
			Message struct {
				Content interface{} `json:"content"`
				Images  []struct {
					URL      string `json:"url"`
					ImageURL string `json:"image_url"`
				} `json:"images"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return ImageResult{}, fmt.Errorf("decodificando resposta de imagem: %w", err)
	}
	if len(out.Choices) == 0 {
		return ImageResult{}, fmt.Errorf("provedor não retornou imagem")
	}
	for _, image := range out.Choices[0].Message.Images {
		if result, ok := imageResultFromURL(firstNonEmptyString(image.URL, image.ImageURL)); ok {
			return result, nil
		}
	}
	if result, ok := imageResultFromContent(out.Choices[0].Message.Content); ok {
		return result, nil
	}
	return ImageResult{}, fmt.Errorf("provedor não retornou imagem")
}

func imageResultFromContent(content interface{}) (ImageResult, bool) {
	parts, ok := content.([]interface{})
	if !ok {
		return ImageResult{}, false
	}
	for _, part := range parts {
		entry, ok := part.(map[string]interface{})
		if !ok {
			continue
		}
		for _, key := range []string{"image_url", "url"} {
			value, ok := entry[key]
			if !ok {
				continue
			}
			if nested, ok := value.(map[string]interface{}); ok {
				if result, ok := imageResultFromURL(stringFromAny(nested["url"])); ok {
					return result, true
				}
				continue
			}
			if result, ok := imageResultFromURL(stringFromAny(value)); ok {
				return result, true
			}
		}
	}
	return ImageResult{}, false
}

func imageResultFromURL(url string) (ImageResult, bool) {
	url = strings.TrimSpace(url)
	if url == "" {
		return ImageResult{}, false
	}
	if strings.HasPrefix(url, "data:") {
		comma := strings.Index(url, ",")
		if comma < 0 {
			return ImageResult{}, false
		}
		meta := url[:comma]
		mime := "image/png"
		if semi := strings.Index(meta, ";"); semi > len("data:") {
			mime = meta[len("data:"):semi]
		}
		return ImageResult{B64: url[comma+1:], MimeType: mime}, true
	}
	return ImageResult{URL: url}, true
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func stringFromAny(value interface{}) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}
