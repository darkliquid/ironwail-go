package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type loggingConfig struct {
	defaultLevel slog.Level
	overrides    map[string]slog.Level
}

func parseLoggingConfig(spec string) (loggingConfig, error) {
	cfg := loggingConfig{
		defaultLevel: slog.LevelInfo,
		overrides:    make(map[string]slog.Level),
	}

	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return cfg, nil
	}

	for _, token := range strings.Split(trimmed, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}

		if key, value, ok := strings.Cut(token, "="); ok {
			subsystem := normalizeSubsystemName(key)
			if subsystem == "" {
				return loggingConfig{}, fmt.Errorf("invalid logging override %q: empty subsystem", token)
			}
			level, err := parseSlogLevel(value)
			if err != nil {
				return loggingConfig{}, fmt.Errorf("invalid logging override %q: %w", token, err)
			}
			cfg.overrides[subsystem] = level
			continue
		}

		level, err := parseSlogLevel(token)
		if err != nil {
			return loggingConfig{}, fmt.Errorf("invalid global logging level %q: %w", token, err)
		}
		cfg.defaultLevel = level
	}

	return cfg, nil
}

func parseSlogLevel(raw string) (slog.Level, error) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "DEBUG":
		return slog.LevelDebug, nil
	case "INFO":
		return slog.LevelInfo, nil
	case "WARN", "WARNING":
		return slog.LevelWarn, nil
	case "ERROR":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unsupported level %q", raw)
	}
}

func (c loggingConfig) minLevel() slog.Level {
	minLevel := c.defaultLevel
	for _, level := range c.overrides {
		if level < minLevel {
			minLevel = level
		}
	}
	return minLevel
}

func (c loggingConfig) levelForSubsystem(subsystem string) slog.Level {
	subsystem = normalizeSubsystemName(subsystem)
	for subsystem != "" {
		if level, ok := c.overrides[subsystem]; ok {
			return level
		}
		if cut := strings.LastIndexByte(subsystem, '.'); cut >= 0 {
			subsystem = subsystem[:cut]
			continue
		}
		break
	}
	return c.defaultLevel
}

func (c loggingConfig) allows(subsystem string, level slog.Level) bool {
	return level >= c.levelForSubsystem(subsystem)
}

func installLogging(spec string) error {
	cfg, err := parseLoggingConfig(spec)
	if err != nil {
		return err
	}

	handler := newSubsystemHandler(
		slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.minLevel()}),
		cfg,
	)
	slog.SetDefault(slog.New(handler))
	slog.SetLogLoggerLevel(cfg.defaultLevel)
	return nil
}

type subsystemHandler struct {
	next   slog.Handler
	config loggingConfig
}

func newSubsystemHandler(next slog.Handler, config loggingConfig) *subsystemHandler {
	return &subsystemHandler{next: next, config: config}
}

func (h *subsystemHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.config.minLevel()
}

func (h *subsystemHandler) Handle(ctx context.Context, r slog.Record) error {
	subsystem := subsystemForRecord(r)
	if !h.config.allows(subsystem, r.Level) {
		return nil
	}

	clone := r.Clone()
	clone.AddAttrs(slog.String("subsystem", subsystem))
	return h.next.Handle(ctx, clone)
}

func (h *subsystemHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &subsystemHandler{
		next:   h.next.WithAttrs(attrs),
		config: h.config,
	}
}

func (h *subsystemHandler) WithGroup(name string) slog.Handler {
	return &subsystemHandler{
		next:   h.next.WithGroup(name),
		config: h.config,
	}
}

func subsystemForRecord(r slog.Record) string {
	if source := r.Source(); source != nil {
		return subsystemForSourcePath(source.File)
	}
	if r.PC != 0 {
		frames := runtime.CallersFrames([]uintptr{r.PC})
		if frame, _ := frames.Next(); frame.File != "" {
			return subsystemForSourcePath(frame.File)
		}
	}
	return "default"
}

func subsystemForSourcePath(path string) string {
	normalized := filepath.ToSlash(strings.ToLower(path))

	switch {
	case strings.Contains(normalized, "/cmd/ironwailgo/"):
		return "app"
	case strings.Contains(normalized, "/internal/renderer/world/opengl/"),
		strings.Contains(normalized, "/internal/renderer/opengl/"),
		strings.Contains(normalized, "/internal/renderer/world_opengl.go"),
		strings.Contains(normalized, "/internal/renderer/renderer_opengl.go"):
		return "renderer.opengl"
	case strings.Contains(normalized, "/internal/renderer/world/gogpu/"),
		strings.Contains(normalized, "/internal/renderer/gogpu/"),
		strings.Contains(normalized, "_gogpu.go"):
		return "renderer.gogpu"
	case strings.Contains(normalized, "/internal/renderer/"):
		return "renderer"
	}

	if cut := strings.Index(normalized, "/internal/"); cut >= 0 {
		rest := normalized[cut+len("/internal/"):]
		if slash := strings.IndexByte(rest, '/'); slash > 0 {
			return rest[:slash]
		}
		if rest != "" {
			return rest
		}
	}
	if cut := strings.Index(normalized, "/pkg/"); cut >= 0 {
		rest := normalized[cut+len("/pkg/"):]
		if slash := strings.IndexByte(rest, '/'); slash > 0 {
			return rest[:slash]
		}
		if rest != "" {
			return rest
		}
	}
	if cut := strings.Index(normalized, "/qgo/"); cut >= 0 {
		rest := normalized[cut+len("/qgo/"):]
		if slash := strings.IndexByte(rest, '/'); slash > 0 {
			return "qgo." + rest[:slash]
		}
		return "qgo"
	}
	return "default"
}

func normalizeSubsystemName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = strings.ReplaceAll(name, " ", "")
	return strings.Trim(name, ".")
}

func captureLogs(t io.Writer, spec string) (*slog.Logger, error) {
	cfg, err := parseLoggingConfig(spec)
	if err != nil {
		return nil, err
	}
	handler := newSubsystemHandler(
		slog.NewTextHandler(t, &slog.HandlerOptions{Level: cfg.minLevel()}),
		cfg,
	)
	return slog.New(handler), nil
}
