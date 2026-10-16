package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func writeRPC(w http.ResponseWriter, id json.RawMessage, result interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
}

func fakeMCPHTTP(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var msg rpcMessage
		_ = json.Unmarshal(body, &msg)
		if len(msg.ID) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		switch msg.Method {
		case "initialize":
			writeRPC(w, msg.ID, map[string]interface{}{
				"protocolVersion": protocolVersion,
				"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
				"serverInfo":      map[string]interface{}{"name": "fake-http", "version": "2.1"},
			})
		case "tools/list":
			writeRPC(w, msg.ID, map[string]interface{}{
				"tools": []map[string]interface{}{{
					"name":        "echo",
					"description": "Devolve o texto recebido",
					"inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"text": map[string]interface{}{"type": "string"}}},
				}},
			})
		case "tools/call":
			var params struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			text, _ := params.Arguments["text"].(string)
			writeRPC(w, msg.ID, map[string]interface{}{
				"content": []map[string]interface{}{{"type": "text", "text": "eco:" + text}},
			})
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
}

func TestProbeAndCallHTTP(t *testing.T) {
	server := fakeMCPHTTP(t)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, tools, err := Probe(ctx, ServerConfig{Transport: "http", URL: server.URL})
	if err != nil {
		t.Fatalf("Probe falhou: %v", err)
	}
	if info.Name != "fake-http" || info.Version != "2.1" {
		t.Fatalf("serverInfo inesperado: %+v", info)
	}
	if len(tools) != 1 || tools[0].Name != "echo" {
		t.Fatalf("tools inesperadas: %+v", tools)
	}

	out, err := CallToolOnce(ctx, ServerConfig{Transport: "http", URL: server.URL}, "echo", map[string]interface{}{"text": "olá"})
	if err != nil {
		t.Fatalf("CallToolOnce falhou: %v", err)
	}
	if out != "eco:olá" {
		t.Fatalf("resultado = %q, esperado %q", out, "eco:olá")
	}
}

func TestParseSSE(t *testing.T) {
	body := []byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":7,\"result\":{\"ok\":true}}\n\n")
	raw, err := parseSSE(body)
	if err != nil {
		t.Fatalf("parseSSE falhou: %v", err)
	}
	if !strings.Contains(string(raw), "\"ok\":true") {
		t.Fatalf("payload SSE inesperado: %s", raw)
	}
}

func runFakeStdioServer() {
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	for {
		line, err := reader.ReadBytes('\n')
		trimmed := strings.TrimSpace(string(line))
		if trimmed != "" {
			var msg rpcMessage
			if json.Unmarshal([]byte(trimmed), &msg) == nil {
				if len(msg.ID) == 0 {
					continue
				}
				var result interface{}
				switch msg.Method {
				case "initialize":
					if path := strings.TrimSpace(os.Getenv("MCP_FAKE_INIT_LOG")); path != "" {
						if f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
							_, _ = f.WriteString("init\n")
							_ = f.Close()
						}
					}
					result = map[string]interface{}{
						"protocolVersion": protocolVersion,
						"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
						"serverInfo":      map[string]interface{}{"name": "fake-stdio", "version": "1.0"},
					}
				case "tools/list":
					result = map[string]interface{}{"tools": []map[string]interface{}{{"name": "ping", "description": "responde pong"}}}
				case "tools/call":
					var params struct {
						Arguments map[string]interface{} `json:"arguments"`
					}
					_ = json.Unmarshal(msg.Params, &params)
					text, _ := params.Arguments["text"].(string)
					if text == "" {
						text = "pong"
					} else {
						text = "eco:" + text
					}
					result = map[string]interface{}{"content": []map[string]interface{}{{"type": "text", "text": text}}}
				default:
					result = map[string]interface{}{}
				}
				payload, _ := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": json.RawMessage(msg.ID), "result": result})
				_, _ = writer.Write(append(payload, '\n'))
				_ = writer.Flush()
			}
		}
		if err != nil {
			return
		}
	}
}

func TestStdioProbeAndCall(t *testing.T) {
	if os.Getenv("MCP_FAKE_STDIO") == "1" {
		runFakeStdioServer()
		return
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("não foi possível resolver o binário de teste: %v", err)
	}
	cfg := ServerConfig{
		Transport: "stdio",
		Command:   exe,
		Args:      []string{"-test.run=TestStdioProbeAndCall"},
		Env:       map[string]string{"MCP_FAKE_STDIO": "1"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	info, tools, err := Probe(ctx, cfg)
	if err != nil {
		t.Fatalf("Probe stdio falhou: %v", err)
	}
	if info.Name != "fake-stdio" {
		t.Fatalf("serverInfo stdio inesperado: %+v", info)
	}
	if len(tools) != 1 || tools[0].Name != "ping" {
		t.Fatalf("tools stdio inesperadas: %+v", tools)
	}

	out, err := CallToolOnce(ctx, cfg, "ping", nil)
	if err != nil {
		t.Fatalf("CallToolOnce stdio falhou: %v", err)
	}
	if out != "pong" {
		t.Fatalf("resultado stdio = %q, esperado %q", out, "pong")
	}
}
