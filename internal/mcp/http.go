package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

const maxHTTPResponse = 8 << 20

type httpTransport struct {
	url     string
	headers map[string]string
	client  *http.Client

	mu        sync.Mutex
	sessionID string
}

func newHTTPTransport(cfg ServerConfig) (*httpTransport, error) {
	address := strings.TrimSpace(cfg.URL)
	if address == "" {
		return nil, errors.New("servidor MCP HTTP exige uma URL")
	}
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		return nil, errors.New("URL do servidor MCP deve usar http ou https")
	}
	return &httpTransport{
		url:     address,
		headers: cfg.Headers,
		client:  &http.Client{Timeout: configTimeout(cfg)},
	}, nil
}

func (t *httpTransport) post(ctx context.Context, msg []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.url, bytes.NewReader(msg))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for key, value := range t.headers {
		req.Header.Set(key, value)
	}
	t.mu.Lock()
	session := t.sessionID
	t.mu.Unlock()
	if session != "" {
		req.Header.Set("Mcp-Session-Id", session)
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	if id := strings.TrimSpace(resp.Header.Get("Mcp-Session-Id")); id != "" {
		t.mu.Lock()
		t.sessionID = id
		t.mu.Unlock()
	}
	return resp, nil
}

func (t *httpTransport) Call(ctx context.Context, msg []byte) ([]byte, error) {
	resp, err := t.post(ctx, msg)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPResponse+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxHTTPResponse {
		return nil, errors.New("resposta do servidor MCP excede o limite")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("servidor MCP HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		return parseSSE(body)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, errors.New("servidor MCP retornou resposta vazia")
	}
	return body, nil
}

func (t *httpTransport) Notify(ctx context.Context, msg []byte) error {
	resp, err := t.post(ctx, msg)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("servidor MCP HTTP %d", resp.StatusCode)
	}
	return nil
}

func (t *httpTransport) Close() error {
	t.client.CloseIdleConnections()
	return nil
}

// parseSSE extrai a primeira mensagem JSON-RPC de um corpo text/event-stream.
func parseSSE(body []byte) ([]byte, error) {
	var last []byte
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		if !json.Valid([]byte(data)) {
			continue
		}
		var msg rpcMessage
		if err := json.Unmarshal([]byte(data), &msg); err == nil && len(msg.ID) > 0 {
			return []byte(data), nil
		}
		last = []byte(data)
	}
	if last == nil {
		return nil, errors.New("resposta SSE do servidor MCP sem mensagem JSON")
	}
	return last, nil
}
