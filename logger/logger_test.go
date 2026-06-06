package logger

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func TestPrettyLogger_TextFormatting(t *testing.T) {
	var buf bytes.Buffer
	l := New().
		WithLevel(slog.LevelInfo).
		WithWriter(&buf).
		WithColor(false). // test plain text
		Build()

	l.Info("hello world", "foo", "bar", "num", 42)

	output := buf.String()
	if !strings.Contains(output, "[INFO] hello world") {
		t.Errorf("Expected output to contain level and message, got: %q", output)
	}
	if !strings.Contains(output, "foo=bar") {
		t.Errorf("Expected output to contain foo=bar, got: %q", output)
	}
	if !strings.Contains(output, "num=42") {
		t.Errorf("Expected output to contain num=42, got: %q", output)
	}
}

func TestPrettyLogger_Colors(t *testing.T) {
	var buf bytes.Buffer
	l := New().
		WithLevel(slog.LevelDebug).
		WithWriter(&buf).
		WithColor(true).
		Build()

	l.Debug("colored debug message")

	output := buf.String()
	// Should contain ANSI color reset and some escape codes
	if !strings.Contains(output, "\033[") {
		t.Errorf("Expected ANSI escape codes, got: %q", output)
	}
}

func TestPrettyLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	l := New().
		WithLevel(slog.LevelWarn).
		WithWriter(&buf).
		Build()

	l.Info("should be ignored")
	if buf.Len() > 0 {
		t.Errorf("Expected no log output, got: %q", buf.String())
	}

	l.Warn("should be printed")
	if buf.Len() == 0 {
		t.Error("Expected warning log output, got nothing")
	}
}

func TestPrettyLogger_Groups(t *testing.T) {
	var buf bytes.Buffer
	l := New().
		WithLevel(slog.LevelInfo).
		WithWriter(&buf).
		WithColor(false).
		Build()

	groupLog := l.WithGroup("user_ctx").With("user_id", "john_123")
	groupLog.Info("login action", "ip", "127.0.0.1")

	output := buf.String()
	if !strings.Contains(output, "user_ctx.user_id=john_123") {
		t.Errorf("Expected grouped attribute user_ctx.user_id, got: %q", output)
	}
	if !strings.Contains(output, "user_ctx.ip=127.0.0.1") {
		t.Errorf("Expected grouped attribute user_ctx.ip, got: %q", output)
	}
}

func TestPrettyLogger_JSON(t *testing.T) {
	var buf bytes.Buffer
	l := New().
		WithLevel(slog.LevelInfo).
		WithWriter(&buf).
		WithJSON(true).
		Build()

	l.Info("json message", "status", "ok")

	output := buf.String()
	var parsed map[string]interface{}
	err := json.Unmarshal([]byte(output), &parsed)
	if err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if parsed["msg"] != "json message" {
		t.Errorf("Expected msg 'json message', got: %v", parsed["msg"])
	}
	if parsed["status"] != "ok" {
		t.Errorf("Expected status 'ok', got: %v", parsed["status"])
	}
}

func TestLoggerBuilder_Error(t *testing.T) {
	b := New().WithWriter(nil)
	if b.Error() == nil {
		t.Error("Expected error for nil writer configuration")
	}
}

func BenchmarkPrettyLogger_Plain(b *testing.B) {
	l := New().
		WithLevel(slog.LevelInfo).
		WithWriter(io.Discard).
		WithColor(false).
		Build()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		l.Info("benchmark message", "foo", "bar", "num", 42, "bool", true)
	}
}

func BenchmarkPrettyLogger_Color(b *testing.B) {
	l := New().
		WithLevel(slog.LevelInfo).
		WithWriter(io.Discard).
		WithColor(true).
		Build()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		l.Info("benchmark message", "foo", "bar", "num", 42, "bool", true)
	}
}
