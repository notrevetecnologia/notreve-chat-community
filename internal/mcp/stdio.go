package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

type stdioTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stderr bytes.Buffer

	mu      sync.Mutex
	waiters map[string]chan []byte
	closed  bool
	close   error
}

func newStdioTransport(ctx context.Context, cfg ServerConfig) (*stdioTransport, error) {
	command := strings.TrimSpace(cfg.Command)
	if command == "" {
		return nil, errors.New("servidor MCP stdio exige um comando")
	}
	cmd := exec.Command(command, cfg.Args...)
	env := os.Environ()
	for key, value := range cfg.Env {
		env = append(env, key+"="+value)
	}
	cmd.Env = env
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	transport := &stdioTransport{cmd: cmd, stdin: stdin, waiters: map[string]chan []byte{}}
	cmd.Stderr = &transport.stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("não foi possível iniciar o servidor MCP: %w", err)
	}
	go transport.readLoop(stdout)
	return transport, nil
}

func (t *stdioTransport) readLoop(stdout io.ReadCloser) {
	reader := bufio.NewReader(stdout)
	for {
		line, err := reader.ReadBytes('\n')
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) > 0 {
			t.deliver(trimmed)
		}
		if err != nil {
			t.fail(errors.New("conexão com o servidor MCP encerrada"))
			return
		}
	}
}

func rpcIDKey(payload []byte) string {
	var msg rpcMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return ""
	}
	return strings.TrimSpace(string(msg.ID))
}

func (t *stdioTransport) deliver(payload []byte) {
	key := rpcIDKey(payload)
	if key == "" || key == "null" {
		return
	}
	t.mu.Lock()
	ch, ok := t.waiters[key]
	if ok {
		delete(t.waiters, key)
	}
	t.mu.Unlock()
	if ok {
		ch <- payload
	}
}

func (t *stdioTransport) fail(cause error) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	t.closed = true
	t.close = cause
	waiters := t.waiters
	t.waiters = map[string]chan []byte{}
	t.mu.Unlock()
	for _, ch := range waiters {
		close(ch)
	}
}

func (t *stdioTransport) Call(ctx context.Context, msg []byte) ([]byte, error) {
	key := rpcIDKey(msg)
	if key == "" {
		return nil, errors.New("requisição MCP sem id")
	}
	ch := make(chan []byte, 1)
	t.mu.Lock()
	if t.closed {
		err := t.close
		t.mu.Unlock()
		if err == nil {
			err = errors.New("conexão MCP fechada")
		}
		return nil, err
	}
	t.waiters[key] = ch
	t.mu.Unlock()

	if _, err := t.stdin.Write(append(msg, '\n')); err != nil {
		t.mu.Lock()
		delete(t.waiters, key)
		t.mu.Unlock()
		return nil, fmt.Errorf("não foi possível enviar ao servidor MCP: %w", err)
	}

	select {
	case <-ctx.Done():
		t.mu.Lock()
		delete(t.waiters, key)
		t.mu.Unlock()
		return nil, ctx.Err()
	case payload, ok := <-ch:
		if !ok {
			return nil, errors.New("conexão com o servidor MCP encerrada")
		}
		return payload, nil
	}
}

func (t *stdioTransport) Notify(_ context.Context, msg []byte) error {
	t.mu.Lock()
	closed := t.closed
	t.mu.Unlock()
	if closed {
		return errors.New("conexão MCP fechada")
	}
	if _, err := t.stdin.Write(append(msg, '\n')); err != nil {
		return err
	}
	return nil
}

func (t *stdioTransport) Close() error {
	t.mu.Lock()
	alreadyClosed := t.closed
	t.mu.Unlock()
	if !alreadyClosed {
		t.fail(errors.New("conexão MCP fechada pelo cliente"))
	}
	_ = t.stdin.Close()
	if t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
	}
	_ = t.cmd.Wait()
	return nil
}
