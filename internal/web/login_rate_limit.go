package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

const (
	loginIPLimit   = int64(10)
	loginUserLimit = int64(8)
	loginWindow    = time.Minute
)

func loginUsernameKey(username string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(username))))
	return hex.EncodeToString(sum[:12])
}

func (s *Server) loginRateLimited(ctx context.Context, ip, username string) bool {
	userKey := loginUsernameKey(username)
	if s.infra != nil && s.infra.Redis != nil {
		pipe := s.infra.Redis.Pipeline()
		ipCount := pipe.Get(ctx, "agente:login:ip:"+ip)
		userCount := pipe.Get(ctx, "agente:login:user:"+userKey)
		_, _ = pipe.Exec(ctx)
		ipValue, _ := ipCount.Int64()
		userValue, _ := userCount.Int64()
		return ipValue >= loginIPLimit || userValue >= loginUserLimit
	}

	now := time.Now()
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	limited := false
	for _, key := range []struct {
		name  string
		limit int
	}{
		{"ip:" + ip, int(loginIPLimit)},
		{"user:" + userKey, int(loginUserLimit)},
	} {
		recent := s.loginTry[key.name][:0]
		for _, attempt := range s.loginTry[key.name] {
			if now.Sub(attempt) < loginWindow {
				recent = append(recent, attempt)
			}
		}
		s.loginTry[key.name] = recent
		if len(recent) >= key.limit {
			limited = true
		}
	}
	return limited
}

func (s *Server) recordLoginFailure(ctx context.Context, ip, username string) {
	userKey := loginUsernameKey(username)
	if s.infra != nil && s.infra.Redis != nil {
		pipe := s.infra.Redis.TxPipeline()
		ipCounter := pipe.Incr(ctx, "agente:login:ip:"+ip)
		userCounter := pipe.Incr(ctx, "agente:login:user:"+userKey)
		_, _ = pipe.Exec(ctx)
		if ipCounter.Val() == 1 {
			_ = s.infra.Redis.Expire(ctx, "agente:login:ip:"+ip, loginWindow).Err()
		}
		if userCounter.Val() == 1 {
			_ = s.infra.Redis.Expire(ctx, "agente:login:user:"+userKey, loginWindow).Err()
		}
		return
	}

	now := time.Now()
	s.loginMu.Lock()
	s.loginTry["ip:"+ip] = append(s.loginTry["ip:"+ip], now)
	s.loginTry["user:"+userKey] = append(s.loginTry["user:"+userKey], now)
	s.loginMu.Unlock()
}

func (s *Server) clearUserLoginFailures(ctx context.Context, username string) {
	userKey := loginUsernameKey(username)
	if s.infra != nil && s.infra.Redis != nil {
		_ = s.infra.Redis.Del(ctx, "agente:login:user:"+userKey).Err()
		return
	}
	s.loginMu.Lock()
	delete(s.loginTry, "user:"+userKey)
	s.loginMu.Unlock()
}
