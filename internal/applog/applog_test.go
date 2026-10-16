package applog

import (
	"context"
	"errors"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

type fakeWriter struct {
	entries []store.LogEntry
	err     error
	panics  bool
}

func (f *fakeWriter) InsertLogEntry(_ context.Context, entry store.LogEntry) error {
	if f.panics {
		panic("falha simulada")
	}
	f.entries = append(f.entries, entry)
	return f.err
}

func TestLoggerBestEffortGravaNoWriter(t *testing.T) {
	w := &fakeWriter{}
	logger := New(w)
	logger.Info(context.Background(), "teste", "mensagem", map[string]interface{}{"chave": "valor"})
	if len(w.entries) != 1 {
		t.Fatalf("esperava 1 entrada gravada, obtive %d", len(w.entries))
	}
	entry := w.entries[0]
	if entry.Level != "INFO" || entry.Module != "teste" || entry.Message != "mensagem" {
		t.Fatalf("entrada inesperada: %+v", entry)
	}
	if entry.Extra["chave"] != "valor" {
		t.Fatalf("extra inesperado: %+v", entry.Extra)
	}
}

func TestLoggerBestEffortIgnoraErroDoWriter(t *testing.T) {
	w := &fakeWriter{err: errors.New("banco fora do ar")}
	logger := New(w)
	logger.Error(context.Background(), "teste", "falha", nil)
	if len(w.entries) != 1 {
		t.Fatalf("esperava 1 tentativa de gravação, obtive %d", len(w.entries))
	}
}

func TestLoggerBestEffortNaoQuebraComPanic(t *testing.T) {
	w := &fakeWriter{panics: true}
	logger := New(w)
	logger.Critical(context.Background(), "teste", "panic", nil)
}

func TestLoggerSemWriterNaoQuebra(t *testing.T) {
	SetDefault(New(nil))
	Info(context.Background(), "teste", "sem writer", nil)
}

func TestLoggerRegistraTraceback(t *testing.T) {
	w := &fakeWriter{}
	logger := New(w)
	logger.Log(context.Background(), "error", "modulo", "mensagem", nil, "stacktrace")
	if len(w.entries) != 1 {
		t.Fatalf("esperava 1 entrada, obtive %d", len(w.entries))
	}
	if w.entries[0].Level != "ERROR" {
		t.Fatalf("nível deveria ser normalizado para ERROR: %q", w.entries[0].Level)
	}
	if w.entries[0].Traceback != "stacktrace" {
		t.Fatalf("traceback não registrado: %q", w.entries[0].Traceback)
	}
}
