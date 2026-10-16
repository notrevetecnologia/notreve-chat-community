package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
)

const defaultIdleTimeout = 5 * time.Minute

var errManagerClosed = errors.New("gerenciador de sessões MCP encerrado")

type session struct {
	client *Client
	info   ServerInfo

	mu       sync.Mutex
	lastUsed time.Time
	closed   bool
}

func (s *session) call(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return "", errors.New("sessão MCP encerrada")
	}
	out, err := s.client.CallTool(ctx, name, args)
	if err == nil {
		s.lastUsed = time.Now()
	}
	return out, err
}

func (s *session) list(ctx context.Context) ([]Tool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("sessão MCP encerrada")
	}
	tools, err := s.client.ListTools(ctx)
	if err == nil {
		s.lastUsed = time.Now()
	}
	return tools, err
}

func (s *session) expired(now time.Time, idle time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.closed && now.Sub(s.lastUsed) >= idle
}

func (s *session) close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	client := s.client
	s.mu.Unlock()
	if client != nil {
		_ = client.Close()
	}
}

type Manager struct {
	idle time.Duration

	mu       sync.Mutex
	sessions map[string]*session
	closed   bool

	createMu sync.Mutex
	stop     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

func NewManager(idle time.Duration) *Manager {
	if idle <= 0 {
		idle = defaultIdleTimeout
	}
	manager := &Manager{
		idle:     idle,
		sessions: map[string]*session{},
		stop:     make(chan struct{}),
	}
	manager.wg.Add(1)
	go manager.janitor()
	return manager
}

func (m *Manager) acquire(ctx context.Context, cfg ServerConfig) (*session, error) {
	key := sessionKey(cfg)
	if s, ok := m.lookup(key); ok {
		return s, nil
	}

	m.createMu.Lock()
	defer m.createMu.Unlock()

	if m.isClosed() {
		return nil, errManagerClosed
	}
	if s, ok := m.lookup(key); ok {
		return s, nil
	}

	client, err := Dial(ctx, cfg)
	if err != nil {
		return nil, err
	}
	info, err := client.Initialize(ctx)
	if err != nil {
		_ = client.Close()
		return nil, err
	}

	s := &session{client: client, info: info, lastUsed: time.Now()}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		_ = client.Close()
		return nil, errManagerClosed
	}
	if existing, ok := m.sessions[key]; ok {
		m.mu.Unlock()
		_ = client.Close()
		return existing, nil
	}
	m.sessions[key] = s
	m.mu.Unlock()
	return s, nil
}

func (m *Manager) lookup(key string) (*session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, false
	}
	s, ok := m.sessions[key]
	return s, ok
}

func (m *Manager) isClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

func (m *Manager) invalidate(key string, s *session) {
	m.mu.Lock()
	if current, ok := m.sessions[key]; ok && current == s {
		delete(m.sessions, key)
	}
	m.mu.Unlock()
	s.close()
}

func (m *Manager) CallTool(ctx context.Context, cfg ServerConfig, name string, args map[string]interface{}) (string, error) {
	callCtx, cancel := callContext(ctx, cfg)
	defer cancel()

	key := sessionKey(cfg)
	s, err := m.acquire(callCtx, cfg)
	if err != nil {
		if errors.Is(err, errManagerClosed) {
			return "", err
		}
		return CallToolOnce(ctx, cfg, name, args)
	}
	out, err := s.call(callCtx, name, args)
	if err == nil {
		return out, nil
	}
	if ctx.Err() != nil {
		return "", err
	}
	m.invalidate(key, s)
	if errors.Is(err, context.DeadlineExceeded) {
		return "", err
	}
	reconnected, err := m.acquire(callCtx, cfg)
	if err != nil {
		return CallToolOnce(ctx, cfg, name, args)
	}
	out, err = reconnected.call(callCtx, name, args)
	if err != nil {
		m.invalidate(key, reconnected)
		return "", err
	}
	return out, nil
}

func (m *Manager) ListTools(ctx context.Context, cfg ServerConfig) (ServerInfo, []Tool, error) {
	listCtx, cancel := callContext(ctx, cfg)
	defer cancel()

	key := sessionKey(cfg)
	s, err := m.acquire(listCtx, cfg)
	if err != nil {
		return ServerInfo{}, nil, err
	}
	tools, err := s.list(listCtx)
	if err == nil {
		return s.info, tools, nil
	}
	if ctx.Err() != nil {
		return s.info, nil, err
	}
	m.invalidate(key, s)
	reconnected, err := m.acquire(listCtx, cfg)
	if err != nil {
		return s.info, nil, err
	}
	tools, err = reconnected.list(listCtx)
	if err != nil {
		m.invalidate(key, reconnected)
		return reconnected.info, nil, err
	}
	return reconnected.info, tools, nil
}

func (m *Manager) CloseServer(cfg ServerConfig) {
	key := sessionKey(cfg)
	m.mu.Lock()
	s, ok := m.sessions[key]
	if ok {
		delete(m.sessions, key)
	}
	m.mu.Unlock()
	if ok && s != nil {
		s.close()
	}
}

func (m *Manager) Close() error {
	m.stopOnce.Do(func() { close(m.stop) })
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		m.wg.Wait()
		return nil
	}
	m.closed = true
	sessions := make([]*session, 0, len(m.sessions))
	for key, s := range m.sessions {
		delete(m.sessions, key)
		sessions = append(sessions, s)
	}
	m.mu.Unlock()
	for _, s := range sessions {
		s.close()
	}
	m.wg.Wait()
	return nil
}

func (m *Manager) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}

func (m *Manager) janitor() {
	defer m.wg.Done()
	interval := m.idle / 2
	if interval > 30*time.Second {
		interval = 30 * time.Second
	}
	if interval < 50*time.Millisecond {
		interval = 50 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-m.stop:
			return
		case now := <-ticker.C:
			m.sweep(now)
		}
	}
}

func (m *Manager) sweep(now time.Time) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	var expired []*session
	for key, s := range m.sessions {
		if s.expired(now, m.idle) {
			delete(m.sessions, key)
			expired = append(expired, s)
		}
	}
	m.mu.Unlock()
	for _, s := range expired {
		s.close()
	}
}

func callContext(ctx context.Context, cfg ServerConfig) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, configTimeout(cfg))
}

func sessionKey(cfg ServerConfig) string {
	normalized := struct {
		Transport string            `json:"transport"`
		Command   string            `json:"command"`
		Args      []string          `json:"args"`
		Env       map[string]string `json:"env"`
		URL       string            `json:"url"`
		Headers   map[string]string `json:"headers"`
	}{
		Transport: normalizeTransport(cfg.Transport),
		Command:   strings.TrimSpace(cfg.Command),
		Args:      cfg.Args,
		Env:       cfg.Env,
		URL:       strings.TrimSpace(cfg.URL),
		Headers:   cfg.Headers,
	}
	encoded, _ := json.Marshal(normalized)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func normalizeTransport(value string) string {
	name := strings.ToLower(strings.TrimSpace(value))
	if name == "" {
		return "stdio"
	}
	return name
}

var defaultManager = NewManager(defaultIdleTimeout)

func CallTool(ctx context.Context, cfg ServerConfig, name string, args map[string]interface{}) (string, error) {
	return defaultManager.CallTool(ctx, cfg, name, args)
}

func ListTools(ctx context.Context, cfg ServerConfig) (ServerInfo, []Tool, error) {
	return defaultManager.ListTools(ctx, cfg)
}

func CloseServer(cfg ServerConfig) {
	defaultManager.CloseServer(cfg)
}

func Shutdown() error {
	return defaultManager.Close()
}
