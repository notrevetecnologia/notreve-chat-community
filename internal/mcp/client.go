// Package mcp implementa o mínimo do Model Context Protocol necessário para o
// agente: handshake (initialize), descoberta de tools (tools/list) e execução
// (tools/call), sobre transporte stdio ou HTTP/SSE.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

const defaultTimeout = 30 * time.Second

type ServerConfig struct {
	Transport string
	Command   string
	Args      []string
	Env       map[string]string
	URL       string
	Headers   map[string]string
	Timeout   time.Duration
}

type ServerInfo struct {
	Name            string
	Version         string
	ProtocolVersion string
	Capabilities    map[string]interface{}
}

type Tool struct {
	Name        string
	Title       string
	Description string
	InputSchema map[string]interface{}
}

type transport interface {
	Call(ctx context.Context, msg []byte) ([]byte, error)
	Notify(ctx context.Context, msg []byte) error
	Close() error
}

type Client struct {
	tr     transport
	nextID int64
	info   ServerInfo

	initOnce sync.Once
	initErr  error
	mu       sync.Mutex
}

// Dial abre a conexão (spawn do processo ou HTTP) sem fazer o handshake.
func Dial(ctx context.Context, cfg ServerConfig) (*Client, error) {
	transportName := strings.ToLower(strings.TrimSpace(cfg.Transport))
	if transportName == "" {
		transportName = "stdio"
	}
	var tr transport
	var err error
	switch transportName {
	case "stdio":
		tr, err = newStdioTransport(ctx, cfg)
	case "http", "streamable-http", "sse":
		tr, err = newHTTPTransport(cfg)
	default:
		return nil, fmt.Errorf("transporte MCP não suportado: %q", cfg.Transport)
	}
	if err != nil {
		return nil, err
	}
	return &Client{tr: tr}, nil
}

func (c *Client) nextRequestID() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextID++
	return c.nextID
}

func (c *Client) request(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	id := c.nextRequestID()
	payload, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, err
	}
	raw, err := c.tr.Call(ctx, payload)
	if err != nil {
		return nil, err
	}
	var msg rpcMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, fmt.Errorf("resposta MCP inválida: %w", err)
	}
	if msg.Error != nil {
		return nil, fmt.Errorf("erro MCP %d: %s", msg.Error.Code, msg.Error.Message)
	}
	return msg.Result, nil
}

func (c *Client) notify(ctx context.Context, method string, params interface{}) error {
	payload, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return err
	}
	return c.tr.Notify(ctx, payload)
}

// Initialize executa o handshake uma única vez por cliente.
func (c *Client) Initialize(ctx context.Context) (ServerInfo, error) {
	c.initOnce.Do(func() {
		params := map[string]interface{}{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]interface{}{},
			"clientInfo":      map[string]interface{}{"name": "notreve-chat-community", "version": "1.0"},
		}
		result, err := c.request(ctx, "initialize", params)
		if err != nil {
			c.initErr = err
			return
		}
		var info initializeResult
		if err := json.Unmarshal(result, &info); err != nil {
			c.initErr = fmt.Errorf("initialize MCP inválido: %w", err)
			return
		}
		c.info = ServerInfo{
			Name:            info.ServerInfo.Name,
			Version:         info.ServerInfo.Version,
			ProtocolVersion: info.ProtocolVersion,
			Capabilities:    info.Capabilities,
		}
		if err := c.notify(ctx, "notifications/initialized", map[string]interface{}{}); err != nil {
			c.initErr = err
			return
		}
	})
	return c.info, c.initErr
}

func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	result, err := c.request(ctx, "tools/list", map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	var parsed listToolsResult
	if err := json.Unmarshal(result, &parsed); err != nil {
		return nil, fmt.Errorf("tools/list MCP inválido: %w", err)
	}
	tools := make([]Tool, 0, len(parsed.Tools))
	for _, item := range parsed.Tools {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		schema := item.InputSchema
		if schema == nil {
			schema = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
		}
		tools = append(tools, Tool{Name: item.Name, Title: item.Title, Description: item.Description, InputSchema: schema})
	}
	return tools, nil
}

func (c *Client) CallTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	if args == nil {
		args = map[string]interface{}{}
	}
	result, err := c.request(ctx, "tools/call", map[string]interface{}{"name": name, "arguments": args})
	if err != nil {
		return "", err
	}
	var parsed callToolResult
	if err := json.Unmarshal(result, &parsed); err != nil {
		return strings.TrimSpace(string(result)), nil
	}
	var parts []string
	for _, item := range parsed.Content {
		if strings.TrimSpace(item.Text) != "" {
			parts = append(parts, item.Text)
		}
	}
	text := strings.TrimSpace(strings.Join(parts, "\n"))
	if text == "" {
		text = strings.TrimSpace(string(result))
	}
	if parsed.IsError {
		return "Erro ao executar a tool MCP: " + text, nil
	}
	return text, nil
}

func (c *Client) Close() error {
	if c.tr == nil {
		return nil
	}
	return c.tr.Close()
}

func configTimeout(cfg ServerConfig) time.Duration {
	if cfg.Timeout > 0 {
		return cfg.Timeout
	}
	return defaultTimeout
}

// Probe conecta, faz o handshake e lista as tools reutilizando a sessão persistente.
func Probe(ctx context.Context, cfg ServerConfig) (ServerInfo, []Tool, error) {
	return defaultManager.ListTools(ctx, cfg)
}

// CallToolOnce executa uma tool em uma conexão efêmera (abre, inicializa, chama e fecha).
func CallToolOnce(ctx context.Context, cfg ServerConfig, name string, args map[string]interface{}) (string, error) {
	client, err := Dial(ctx, cfg)
	if err != nil {
		return "", err
	}
	defer client.Close()
	if _, err := client.Initialize(ctx); err != nil {
		return "", err
	}
	return client.CallTool(ctx, name, args)
}
