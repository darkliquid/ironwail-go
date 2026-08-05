package main

import (
	"fmt"
	"os"
	"strings"
)

func waitLines(count int) string {
	if count <= 0 {
		return ""
	}
	var b strings.Builder
	for i := 0; i < count; i++ {
		b.WriteString("wait\n")
	}
	return b.String()
}

func captureEnv(reference bool) []string {
	env := []string{
		"WAYLAND_DISPLAY=",
		"XDG_SESSION_TYPE=x11",
	}
	if reference {
		env = append(env, "SDL_VIDEODRIVER=x11")
	}
	return env
}

func goCaptureEnv(vp viewpoint) []string {
	env := captureEnv(false)
	env = append(env,
		"PARITY_RUN=1",
		fmt.Sprintf("PARITY_POS=%s %s %s", fmtFloat(vp.Pos[0]), fmtFloat(vp.Pos[1]), fmtFloat(vp.Pos[2])),
		fmt.Sprintf("PARITY_ANGLES=%s %s %s", fmtFloat(vp.Angles[0]), fmtFloat(vp.Angles[1]), fmtFloat(vp.Angles[2])),
	)
	return env
}

func fmtFloat(v float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.6f", v), "0"), ".")
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func parseFloatEnv(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	var parsed float64
	if _, err := fmt.Sscanf(value, "%f", &parsed); err != nil {
		return fallback
	}
	return parsed
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

func printUsage() {
	fmt.Println("Usage: go run ./tools/parity_screenshots {reference|go|compare|both|report}")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  reference  Capture reference screenshots from C Ironwail")
	fmt.Println("  go         Capture screenshots from the Go GoGPU parity build")
	fmt.Println("  compare    Compare reference vs Go screenshots (nonzero on diffs/missing captures)")
	fmt.Println("  both       Do all three in sequence (nonzero on diffs/missing captures)")
	fmt.Println("  report     Compare screenshots and emit structured JSON + Markdown summary table")
	fmt.Println()
	fmt.Println("Environment:")
	fmt.Println("  QUAKE_BASEDIR  Path to Quake data")
	fmt.Println("  IRONWAIL_BIN   Path to C Ironwail binary")
	fmt.Println("  GO_BIN         Path to Go binary (default: ./ironwailgo)")
	fmt.Println("  PARITY_GO_CAPTURE  Go capture method: window or engine (default: window)")
	fmt.Println("  PARITY_GO_WINDOW_SETTLE_MS  Delay before window capture (default: 2500)")
	fmt.Println("  PARITY_ONION_ALPHA  Blend weight for reference image in overlay output (default: 0.5)")
}
