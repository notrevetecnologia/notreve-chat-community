package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeHTTPServer struct {
	server    *httptest.Server
	initCount atomic.Int64
	callCount atomic.Int64

	mu        sync.Mutex
	failCalls map[int64]bool
	dropCalls map[int64]bool
}

func newFakeHTTPServer(t *testing.T) *fakeHTTPServer {
	t.Helper()
	fake := &fakeHTTPServer{failCalls: map[int64]bool{}, dropCalls: map[int64]bool{}}
	fake.server = httptest.NewServer(http.HandlerFunc(fake.handle))
	t.Cleanup(fake.server.Close)
	return fake
}

func (f *fakeHTTPServer) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var msg rpcMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		http.Error(w, "json inválido", http.StatusBadRequest)
		return
	}
	if len(msg.ID) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	switch msg.Method {
	case "initialize":
		f.initCount.Add(1)
		writeRPC(w, msg.ID, map[string]interface{}{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
			"serverInfo":      map[string]interface{}{"name": "fake-count", "version": "1.0"},
		})
	case "tools/list":
		writeRPC(w, msg.ID, map[string]interface{}{
			"tools": []map[string]interface{}{{"name": "echo", "description": "devolve o texto"}},
		})
	case "tools/call":
		index := f.callCount.Add(1)
		f.mu.Lock()
		fail := f.failCalls[index]
		drop := f.dropCalls[index]
		f.mu.Unlock()
		if drop {
			if hijacker, ok := w.(http.Hijacker); ok {
				if conn, _, err := hijacker.Hijack(); err == nil {
					_ = conn.Close()
					return
				}
			}
			http.Error(w, "queda simulada", http.StatusInternalServerError)
			return
		}
		if fail {
			http.Error(w, "falha simulada", http.StatusInternalServerError)
			return
		}
		var params struct {
			Arguments map[string]interface{} `json:"arguments"`
		}
		_ = json.Unmarshal(msg.Params, &params)
		text, _ := params.Arguments["text"].(string)
		writeRPC(w, msg.ID, map[string]interface{}{
			"content": []map[string]interface{}{{"type": "text", "text": "eco:" + text}},
		})
	default:
		http.Error(w, "método desconhecido", http.StatusBadRequest)
	}
}

func stdioFakeConfig(t *testing.T, initLog string) ServerConfig {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("não foi possível resolver o binário de teste: %v", err)
	}
	return ServerConfig{
		Transport: "stdio",
		Command:   exe,
		Args:      []string{"-test.run=TestStdioProbeAndCall"},
		Env:       map[string]string{"MCP_FAKE_STDIO": "1", "MCP_FAKE_INIT_LOG": initLog},
	}
}

func countLogLines(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func TestManagerReusesSessionHTTP(t *testing.T) {
	fake := newFakeHTTPServer(t)
	manager := NewManager(time.Minute)
	defer manager.Close()
	cfg := ServerConfig{Transport: "http", URL: fake.server.URL}

	if _, tools, err := manager.ListTools(context.Background(), cfg); err != nil {
		t.Fatalf("ListTools falhou: %v", err)
	} else if len(tools) != 1 || tools[0].Name != "echo" {
		t.Fatalf("tools inesperadas: %+v", tools)
	}
	for _, text := range []string{"a", "b"} {
		out, err := manager.CallTool(context.Background(), cfg, "echo", map[string]interface{}{"text": text})
		if err != nil {
			t.Fatalf("CallTool falhou: %v", err)
		}
		if out != "eco:"+text {
			t.Fatalf("resultado = %q, esperado %q", out, "eco:"+text)
		}
	}
	if got := fake.initCount.Load(); got != 1 {
		t.Fatalf("initialize chamado %d vezes, esperado 1", got)
	}
}

func TestManagerRecreatesAfterHTTPFailure(t *testing.T) {
	fake := newFakeHTTPServer(t)
	fake.failCalls[1] = true
	manager := NewManager(time.Minute)
	defer manager.Close()
	cfg := ServerConfig{Transport: "http", URL: fake.server.URL}

	out, err := manager.CallTool(context.Background(), cfg, "echo", map[string]interface{}{"text": "x"})
	if err != nil {
		t.Fatalf("CallTool falhou: %v", err)
	}
	if out != "eco:x" {
		t.Fatalf("resultado = %q", out)
	}
	if got := fake.initCount.Load(); got != 2 {
		t.Fatalf("initialize chamado %d vezes, esperado 2 (recriação)", got)
	}
}

func TestManagerRecreatesAfterHTTPDrop(t *testing.T) {
	fake := newFakeHTTPServer(t)
	fake.dropCalls[1] = true
	manager := NewManager(time.Minute)
	defer manager.Close()
	cfg := ServerConfig{Transport: "http", URL: fake.server.URL}

	out, err := manager.CallTool(context.Background(), cfg, "echo", map[string]interface{}{"text": "y"})
	if err != nil {
		t.Fatalf("CallTool falhou: %v", err)
	}
	if out != "eco:y" {
		t.Fatalf("resultado = %q", out)
	}
	if got := fake.initCount.Load(); got != 2 {
		t.Fatalf("initialize chamado %d vezes, esperado 2 (recriação após queda)", got)
	}
}

func TestManagerConcurrentCallsShareSessionHTTP(t *testing.T) {
	fake := newFakeHTTPServer(t)
	manager := NewManager(time.Minute)
	defer manager.Close()
	cfg := ServerConfig{Transport: "http", URL: fake.server.URL}

	const total = 16
	var wg sync.WaitGroup
	errs := make(chan error, total)
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			text := fmt.Sprintf("t%d", i)
			out, err := manager.CallTool(context.Background(), cfg, "echo", map[string]interface{}{"text": text})
			if err != nil {
				errs <- err
				return
			}
			if out != "eco:"+text {
				errs <- fmt.Errorf("resultado = %q, esperado %q", out, "eco:"+text)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if got := fake.initCount.Load(); got != 1 {
		t.Fatalf("initialize chamado %d vezes, esperado 1", got)
	}
}

func TestManagerStdioReusesSession(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "init.log")
	cfg := stdioFakeConfig(t, logPath)
	manager := NewManager(time.Minute)
	defer manager.Close()

	if _, _, err := manager.ListTools(context.Background(), cfg); err != nil {
		t.Fatalf("ListTools stdio falhou: %v", err)
	}
	for _, text := range []string{"a", "b"} {
		out, err := manager.CallTool(context.Background(), cfg, "echo", map[string]interface{}{"text": text})
		if err != nil {
			t.Fatalf("CallTool stdio falhou: %v", err)
		}
		if out != "eco:"+text {
			t.Fatalf("resultado stdio = %q, esperado %q", out, "eco:"+text)
		}
	}
	if lines := countLogLines(t, logPath); lines != 1 {
		t.Fatalf("inicializações stdio = %d, esperado 1", lines)
	}
}

func TestManagerStdioConcurrentCalls(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "init.log")
	cfg := stdioFakeConfig(t, logPath)
	manager := NewManager(time.Minute)
	defer manager.Close()

	const total = 12
	var wg sync.WaitGroup
	errs := make(chan error, total)
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			text := fmt.Sprintf("c%d", i)
			out, err := manager.CallTool(context.Background(), cfg, "echo", map[string]interface{}{"text": text})
			if err != nil {
				errs <- err
				return
			}
			if out != "eco:"+text {
				errs <- fmt.Errorf("resultado = %q, esperado %q", out, "eco:"+text)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if lines := countLogLines(t, logPath); lines != 1 {
		t.Fatalf("inicializações stdio = %d, esperado 1", lines)
	}
}

func TestManagerIdleTimeoutClosesSession(t *testing.T) {
	fake := newFakeHTTPServer(t)
	manager := NewManager(120 * time.Millisecond)
	defer manager.Close()
	cfg := ServerConfig{Transport: "http", URL: fake.server.URL}

	if _, err := manager.CallTool(context.Background(), cfg, "echo", map[string]interface{}{"text": "a"}); err != nil {
		t.Fatalf("CallTool falhou: %v", err)
	}
	if manager.count() != 1 {
		t.Fatalf("sessão não foi registrada")
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if manager.count() == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("sessão não foi encerrada por idle timeout")
}

func TestManagerCloseServer(t *testing.T) {
	fake := newFakeHTTPServer(t)
	manager := NewManager(time.Minute)
	defer manager.Close()
	cfg := ServerConfig{Transport: "http", URL: fake.server.URL}

	if _, err := manager.CallTool(context.Background(), cfg, "echo", map[string]interface{}{"text": "a"}); err != nil {
		t.Fatalf("CallTool falhou: %v", err)
	}
	manager.CloseServer(cfg)
	if manager.count() != 0 {
		t.Fatalf("sessão deveria ter sido removida")
	}
	if _, err := manager.CallTool(context.Background(), cfg, "echo", map[string]interface{}{"text": "b"}); err != nil {
		t.Fatalf("CallTool após CloseServer falhou: %v", err)
	}
	if got := fake.initCount.Load(); got != 2 {
		t.Fatalf("initialize chamado %d vezes, esperado 2", got)
	}
}

func TestManagerCloseRejectsNewSessions(t *testing.T) {
	manager := NewManager(time.Minute)
	if err := manager.Close(); err != nil {
		t.Fatalf("Close falhou: %v", err)
	}
	fake := newFakeHTTPServer(t)
	cfg := ServerConfig{Transport: "http", URL: fake.server.URL}
	if _, err := manager.CallTool(context.Background(), cfg, "echo", nil); err == nil {
		t.Fatalf("esperado erro após Close")
	}
}
