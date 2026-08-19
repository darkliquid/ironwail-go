package quakui

import (
	"os/exec"
	"strings"
	"testing"
)

// TestNoEngineImports asserts the isolation boundary (AC7, ADR-0009):
// internal/quakui's import closure must not include internal/game or
// internal/renderer. This keeps the UI subsystem self-contained.
func TestNoEngineImports(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "github.com/darkliquid/ironwail-go/internal/quakui").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps failed: %v\n%s", err, out)
	}
	for _, forbidden := range []string{"internal/game", "internal/renderer"} {
		if strings.Contains(string(out), forbidden) {
			t.Fatalf("internal/quakui imports forbidden package %q in its closure (AC7)", forbidden)
		}
	}
}
