package web

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type sessionRecord struct {
	userID      int
	tenantID    int
	role        string
	mfaVerified bool
	lastSeen    time.Time
}

type sessionIdentity struct {
	UserID      int
	TenantID    int
	Role        string
	MFAVerified bool
}

type sessionStore struct {
	mu      sync.Mutex
	mem     map[string]sessionRecord
	started map[string]time.Time
	redis   *redis.Client
	ttl     time.Duration
}

const sessionRedisPrefix = "notreve:session:v4:"

func sessionRedisKey(token string) string {
	return sessionRedisPrefix + token
}

func encodeSessionValue(uid, tenantID int, role string, mfaVerified bool, started time.Time) string {
	return strconv.Itoa(uid) + "|" + strconv.Itoa(tenantID) + "|" + role + "|" + strconv.FormatBool(mfaVerified) + "|" + strconv.FormatInt(started.UnixNano(), 10)
}

func decodeSessionValue(value string) (sessionRecord, time.Time, bool) {
	parts := strings.Split(strings.TrimSpace(value), "|")
	if len(parts) != 5 {
		return sessionRecord{}, time.Time{}, false
	}
	uid, err := strconv.Atoi(parts[0])
	if err != nil || uid < 1 {
		return sessionRecord{}, time.Time{}, false
	}
	tenantID, err := strconv.Atoi(parts[1])
	if err != nil || tenantID < 1 {
		return sessionRecord{}, time.Time{}, false
	}
	role := strings.TrimSpace(parts[2])
	if role == "" {
		return sessionRecord{}, time.Time{}, false
	}
	mfaVerified, err := strconv.ParseBool(parts[3])
	if err != nil {
		return sessionRecord{}, time.Time{}, false
	}
	nanos, err := strconv.ParseInt(parts[4], 10, 64)
	if err != nil || nanos <= 0 {
		return sessionRecord{}, time.Time{}, false
	}
	started := time.Unix(0, nanos)
	return sessionRecord{userID: uid, tenantID: tenantID, role: role, mfaVerified: mfaVerified}, started, true
}

func newSessionStore(rc *redis.Client, ttl time.Duration) *sessionStore {
	return &sessionStore{mem: map[string]sessionRecord{}, started: map[string]time.Time{}, redis: rc, ttl: ttl}
}

// Set existe apenas para manter o código HTML legado compilável. Ele não cria
// uma identidade autenticável porque não conhece empresa/papel. O fluxo SaaS
// canônico deve usar SetIdentity.
func (ss *sessionStore) Set(ctx context.Context, token string, uid int) {
	started := time.Now()
	entry := sessionRecord{userID: uid, lastSeen: started}
	ss.mu.Lock()
	ss.mem[token] = entry
	ss.started[token] = started
	ss.mu.Unlock()
}

func (ss *sessionStore) SetIdentity(ctx context.Context, token string, uid, tenantID int, role string, mfaVerified bool) {
	started := time.Now()
	entry := sessionRecord{userID: uid, tenantID: tenantID, role: strings.TrimSpace(role), mfaVerified: mfaVerified, lastSeen: started}
	ss.mu.Lock()
	ss.mem[token] = entry
	ss.started[token] = started
	ss.mu.Unlock()
	if ss.redis != nil {
		_ = ss.redis.Set(ctx, sessionRedisKey(token), encodeSessionValue(uid, tenantID, role, mfaVerified, started), ss.ttl).Err()
	}
}

func (ss *sessionStore) loadFromRedis(ctx context.Context, token string) (sessionRecord, time.Time, bool) {
	if ss.redis == nil {
		return sessionRecord{}, time.Time{}, false
	}
	v := ss.redis.Get(ctx, sessionRedisKey(token))
	if v == nil || v.Err() != nil {
		return sessionRecord{}, time.Time{}, false
	}
	entry, started, valid := decodeSessionValue(v.Val())
	if !valid {
		_ = ss.redis.Del(ctx, sessionRedisKey(token)).Err()
		return sessionRecord{}, time.Time{}, false
	}
	entry.lastSeen = time.Now()
	ss.mu.Lock()
	ss.mem[token] = entry
	ss.started[token] = started
	ss.mu.Unlock()
	return entry, started, true
}

func (ss *sessionStore) Touch(ctx context.Context, token string) bool {
	ss.mu.Lock()
	e, ok := ss.mem[token]
	if ok {
		e.lastSeen = time.Now()
		ss.mem[token] = e
	}
	ss.mu.Unlock()
	if !ok {
		_, _, ok = ss.loadFromRedis(ctx, token)
	}
	if ok && ss.redis != nil {
		_ = ss.redis.Expire(ctx, sessionRedisKey(token), ss.ttl).Err()
	}
	return ok
}

func (ss *sessionStore) Get(ctx context.Context, token string) (int, bool) {
	identity, ok := ss.Identity(ctx, token)
	return identity.UserID, ok
}

func (ss *sessionStore) Identity(ctx context.Context, token string) (sessionIdentity, bool) {
	ss.mu.Lock()
	e, ok := ss.mem[token]
	ss.mu.Unlock()
	if !ok {
		e, _, ok = ss.loadFromRedis(ctx, token)
	}
	if !ok || e.userID < 1 || e.tenantID < 1 || strings.TrimSpace(e.role) == "" {
		return sessionIdentity{}, false
	}
	return sessionIdentity{UserID: e.userID, TenantID: e.tenantID, Role: e.role, MFAVerified: e.mfaVerified}, true
}

func (ss *sessionStore) StartedAt(ctx context.Context, token string) (time.Time, bool) {
	ss.mu.Lock()
	started, ok := ss.started[token]
	ss.mu.Unlock()
	if ok && !started.IsZero() {
		return started, true
	}
	_, started, ok = ss.loadFromRedis(ctx, token)
	return started, ok && !started.IsZero()
}

func (ss *sessionStore) Delete(ctx context.Context, token string) {
	ss.mu.Lock()
	delete(ss.mem, token)
	delete(ss.started, token)
	ss.mu.Unlock()
	if ss.redis != nil {
		_ = ss.redis.Del(ctx, sessionRedisKey(token)).Err()
	}
}

func (ss *sessionStore) DeleteUser(ctx context.Context, uid int) {
	ss.mu.Lock()
	var tokens []string
	for token, entry := range ss.mem {
		if entry.userID == uid {
			tokens = append(tokens, token)
			delete(ss.mem, token)
			delete(ss.started, token)
		}
	}
	ss.mu.Unlock()
	if ss.redis == nil {
		return
	}
	for _, token := range tokens {
		_ = ss.redis.Del(ctx, sessionRedisKey(token)).Err()
	}
	var cursor uint64
	for {
		keys, next, err := ss.redis.Scan(ctx, cursor, sessionRedisPrefix+"*", 100).Result()
		if err != nil {
			return
		}
		for _, key := range keys {
			value, err := ss.redis.Get(ctx, key).Result()
			if err != nil {
				continue
			}
			entry, _, valid := decodeSessionValue(value)
			if valid && entry.userID == uid {
				_ = ss.redis.Del(ctx, key).Err()
			}
		}
		cursor = next
		if cursor == 0 {
			return
		}
	}
}

func (ss *sessionStore) sweep(now time.Time) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	for token, entry := range ss.mem {
		if now.Sub(entry.lastSeen) > ss.ttl {
			delete(ss.mem, token)
			delete(ss.started, token)
		}
	}
}

func (ss *sessionStore) activeUsers() map[int]bool {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	out := map[int]bool{}
	for _, entry := range ss.mem {
		if entry.userID < 1 || entry.tenantID < 1 || strings.TrimSpace(entry.role) == "" {
			continue
		}
		if entry.role == "super_admin" && !entry.mfaVerified {
			continue
		}
		out[entry.userID] = true
	}
	return out
}
