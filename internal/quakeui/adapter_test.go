package quakeui

import (
	"os/exec"
	"strings"
	"testing"
)

// TestNoEngineImports asserts the isolation boundary (AC7, ADR-0009):
// internal/quakeui's import closure must not include internal/game.
// This keeps the UI subsystem self-contained and avoids circular dependencies.
func TestNoEngineImports(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "github.com/darkliquid/ironwail-go/internal/quakeui").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps failed: %v\n%s", err, out)
	}
	for _, forbidden := range []string{"internal/game"} {
		if strings.Contains(string(out), forbidden) {
			t.Fatalf("internal/quakeui imports forbidden package %q in its closure (AC7)", forbidden)
		}
	}
}
