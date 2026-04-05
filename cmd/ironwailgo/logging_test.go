package main

import (
	"bytes"
	"context"
	"log/slog"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestParseLoggingConfigSupportsGlobalAndOverrides(t *testing.T) {
	cfg, err := parseLoggingConfig("INFO,renderer=WARN,input=DEBUG")
	if err != nil {
		t.Fatalf("parseLoggingConfig returned error: %v", err)
	}

	if cfg.defaultLevel != slog.LevelInfo {
		t.Fatalf("defaultLevel = %v, want %v", cfg.defaultLevel, slog.LevelInfo)
	}
	if cfg.levelForSubsystem("renderer.gogpu") != slog.LevelWarn {
		t.Fatalf("renderer.gogpu level = %v, want %v", cfg.levelForSubsystem("renderer.gogpu"), slog.LevelWarn)
	}
	if cfg.levelForSubsystem("input") != slog.LevelDebug {
		t.Fatalf("input level = %v, want %v", cfg.levelForSubsystem("input"), slog.LevelDebug)
	}
	if cfg.levelForSubsystem("audio") != slog.LevelInfo {
		t.Fatalf("audio level = %v, want %v", cfg.levelForSubsystem("audio"), slog.LevelInfo)
	}
}

func TestParseLoggingConfigRejectsInvalidTokens(t *testing.T) {
	tests := []string{
		"TRACE",
		"renderer=TRACE",
		"=DEBUG",
	}

	for _, spec := range tests {
		if _, err := parseLoggingConfig(spec); err == nil {
			t.Fatalf("parseLoggingConfig(%q) succeeded, want error", spec)
		}
	}
}

func TestSubsystemForSourcePath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "/repo/cmd/ironwailgo/main.go", want: "app"},
		{path: "/repo/internal/input/types.go", want: "input"},
		{path: "/repo/internal/renderer/world.go", want: "renderer"},
		{path: "/repo/internal/renderer/gogpu/input_backend.go", want: "renderer.gogpu"},
		{path: "/repo/internal/renderer/software.go", want: "renderer"},
		{path: "/repo/pkg/types/types.go", want: "types"},
	}

	for _, tc := range tests {
		if got := subsystemForSourcePath(tc.path); got != tc.want {
			t.Fatalf("subsystemForSourcePath(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestSubsystemHandlerFiltersByOverride(t *testing.T) {
	cfg, err := parseLoggingConfig("INFO,renderer=WARN,app=DEBUG")
	if err != nil {
		t.Fatalf("parseLoggingConfig returned error: %v", err)
	}

	handler := newSubsystemHandler(&captureHandler{}, cfg)

	if !handler.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("Enabled(DEBUG) = false, want true because app override lowers minimum")
	}
	if handler.config.allows("renderer.gogpu", slog.LevelInfo) {
		t.Fatal("renderer.gogpu INFO should be filtered by renderer=WARN override")
	}
	if !handler.config.allows("app", slog.LevelDebug) {
		t.Fatal("app DEBUG should be allowed by app=DEBUG override")
	}
}

func TestCaptureLogsAnnotatesSubsystemAndFilters(t *testing.T) {
	var buf bytes.Buffer
	logger, err := captureLogs(&buf, "INFO,app=DEBUG")
	if err != nil {
		t.Fatalf("captureLogs returned error: %v", err)
	}

	logger.Debug("debug from app shell")
	if got := buf.String(); !strings.Contains(got, "subsystem=app") {
		t.Fatalf("expected subsystem annotation in %q", got)
	}

	buf.Reset()
	logger, err = captureLogs(&buf, "WARN")
	if err != nil {
		t.Fatalf("captureLogs returned error: %v", err)
	}
	logger.Info("info suppressed")
	if buf.Len() != 0 {
		t.Fatalf("expected INFO to be suppressed, got %q", buf.String())
	}
}

type captureHandler struct{}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool  { return true }
func (h *captureHandler) Handle(context.Context, slog.Record) error { return nil }
func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler        { return h }
func (h *captureHandler) WithGroup(string) slog.Handler             { return h }

func TestSubsystemForRecordFallsBackToAppForThisFile(t *testing.T) {
	var pcs [1]uintptr
	n := runtime.Callers(1, pcs[:])
	if n == 0 {
		t.Fatal("runtime.Callers returned no frames")
	}
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test", pcs[0])
	if got := subsystemForRecord(record); got != "app" {
		t.Fatalf("subsystemForRecord(...) = %q, want app", got)
	}
}
