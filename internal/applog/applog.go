package applog

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

type writer interface {
	InsertLogEntry(ctx context.Context, entry store.LogEntry) error
}

type Logger struct {
	w writer
}

func New(w writer) *Logger {
	return &Logger{w: w}
}

var standard = &Logger{}

func SetDefault(l *Logger) {
	if l != nil {
		standard = l
	}
}

func (l *Logger) log(ctx context.Context, level, module, message string, extra map[string]interface{}, traceback string) {
	level = strings.ToUpper(strings.TrimSpace(level))
	if level == "" {
		level = "INFO"
	}
	module = strings.TrimSpace(module)
	log.Printf("[%s] %s: %s", level, module, message)
	if l == nil || l.w == nil {
		return
	}
	defer func() { _ = recover() }()
	entry := store.LogEntry{
		Ts: time.Now().UTC(), Level: level, Module: module, Message: message,
		Extra: extra, Traceback: traceback,
	}
	_ = l.w.InsertLogEntry(ctx, entry)
}

func (l *Logger) Log(ctx context.Context, level, module, message string, extra map[string]interface{}, traceback string) {
	l.log(ctx, level, module, message, extra, traceback)
}

func (l *Logger) Debug(ctx context.Context, module, message string, extra map[string]interface{}) {
	l.log(ctx, "DEBUG", module, message, extra, "")
}

func (l *Logger) Info(ctx context.Context, module, message string, extra map[string]interface{}) {
	l.log(ctx, "INFO", module, message, extra, "")
}

func (l *Logger) Warn(ctx context.Context, module, message string, extra map[string]interface{}) {
	l.log(ctx, "WARNING", module, message, extra, "")
}

func (l *Logger) Error(ctx context.Context, module, message string, extra map[string]interface{}) {
	l.log(ctx, "ERROR", module, message, extra, "")
}

func (l *Logger) Critical(ctx context.Context, module, message string, extra map[string]interface{}) {
	l.log(ctx, "CRITICAL", module, message, extra, "")
}

func Debug(ctx context.Context, module, message string, extra map[string]interface{}) {
	standard.log(ctx, "DEBUG", module, message, extra, "")
}

func Info(ctx context.Context, module, message string, extra map[string]interface{}) {
	standard.log(ctx, "INFO", module, message, extra, "")
}

func Warn(ctx context.Context, module, message string, extra map[string]interface{}) {
	standard.log(ctx, "WARNING", module, message, extra, "")
}

func Error(ctx context.Context, module, message string, extra map[string]interface{}) {
	standard.log(ctx, "ERROR", module, message, extra, "")
}

func Critical(ctx context.Context, module, message string, extra map[string]interface{}) {
	standard.log(ctx, "CRITICAL", module, message, extra, "")
}

func Log(ctx context.Context, level, module, message string, extra map[string]interface{}, traceback string) {
	standard.log(ctx, level, module, message, extra, traceback)
}
