package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ralphVerbose bool

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func parseIntEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return fallback
	}
	return parsed
}

func parseBoolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func truncate(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n])
}

func verbosef(format string, args ...any) {
	if !ralphVerbose {
		return
	}
	fmt.Printf("[ralph] "+format+"\n", args...)
}

func ensureParentDirs(paths ...string) error {
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
		}
	}
	return nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
