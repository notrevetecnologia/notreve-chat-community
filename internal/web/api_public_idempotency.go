package web

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	publicIdempotencyTTL = 24 * time.Hour
	publicRateWindow     = time.Minute
	publicRateLimit      = 120
	publicMaxReplayItems = 10000
)

var (
	errPublicIdempotencyConflict   = errors.New("public idempotency conflict")
	errPublicIdempotencyInProgress = errors.New("public idempotency in progress")
)

type publicAPIState struct {
	mu         sync.Mutex
	replays    map[string]publicIdempotencyRecord
	rateLimits map[string]publicRateRecord
}

type publicIdempotencyRecord struct {
	fingerprint string
	status      int
	body        []byte
	processing  bool
	expiresAt   time.Time
}

type publicIdempotencyDecision struct {
	replay bool
	status int
	body   []byte
}

type publicRateRecord struct {
	startedAt time.Time
	count     int
}

func newPublicAPIState() publicAPIState {
	return publicAPIState{replays: map[string]publicIdempotencyRecord{}, rateLimits: map[string]publicRateRecord{}}
}

func (state *publicAPIState) allow(key string) bool {
	now := time.Now()
	state.mu.Lock()
	defer state.mu.Unlock()
	record := state.rateLimits[key]
	if record.startedAt.IsZero() || now.Sub(record.startedAt) >= publicRateWindow {
		record = publicRateRecord{startedAt: now}
	}
	if record.count >= publicRateLimit {
		state.rateLimits[key] = record
		return false
	}
	record.count++
	state.rateLimits[key] = record
	return true
}

func (state *publicAPIState) begin(namespace, key, fingerprint string) (publicIdempotencyDecision, error) {
	now := time.Now()
	mapKey := namespace + "\x00" + key
	state.mu.Lock()
	defer state.mu.Unlock()
	state.prune(now)
	if record, ok := state.replays[mapKey]; ok {
		if record.fingerprint != fingerprint {
			return publicIdempotencyDecision{}, errPublicIdempotencyConflict
		}
		if record.processing {
			return publicIdempotencyDecision{}, errPublicIdempotencyInProgress
		}
		return publicIdempotencyDecision{replay: true, status: record.status, body: append([]byte(nil), record.body...)}, nil
	}
	if len(state.replays) >= publicMaxReplayItems {
		return publicIdempotencyDecision{}, errors.New("public idempotency cache full")
	}
	state.replays[mapKey] = publicIdempotencyRecord{fingerprint: fingerprint, processing: true, expiresAt: now.Add(publicIdempotencyTTL)}
	return publicIdempotencyDecision{}, nil
}

func (state *publicAPIState) finish(namespace, key, fingerprint string, status int, body []byte) {
	mapKey := namespace + "\x00" + key
	state.mu.Lock()
	defer state.mu.Unlock()
	record, ok := state.replays[mapKey]
	if !ok || record.fingerprint != fingerprint {
		return
	}
	record.status = status
	record.body = append([]byte(nil), body...)
	record.processing = false
	record.expiresAt = time.Now().Add(publicIdempotencyTTL)
	state.replays[mapKey] = record
}

func (state *publicAPIState) prune(now time.Time) {
	for key, record := range state.replays {
		if now.After(record.expiresAt) {
			delete(state.replays, key)
		}
	}
	for key, record := range state.rateLimits {
		if now.Sub(record.startedAt) >= publicRateWindow {
			delete(state.rateLimits, key)
		}
	}
}

func publicIdempotencyKey(r *http.Request) (string, error) {
	value := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if value == "" {
		return "", errors.New("Idempotency-Key é obrigatório em toda escrita")
	}
	if len([]rune(value)) > 200 {
		return "", errors.New("Idempotency-Key deve ter no máximo 200 caracteres")
	}
	for _, char := range value {
		if char < 0x21 || char == 0x7f {
			return "", errors.New("Idempotency-Key contém caracteres inválidos")
		}
	}
	return value, nil
}
