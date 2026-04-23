package host

import (
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

func TestHostDebugSysEmitsWhenEnabled(t *testing.T) {
	cv := cvar.NewCVarSystem()
	RegisterHostDebugTelemetryCVars(cv)

	var captured []string
	origEmit := hostDebugSysEmit
	origCVar := hostDebugSysCVar
	t.Cleanup(func() {
		hostDebugSysEmit = origEmit
		hostDebugSysCVar = origCVar
	})
	hostDebugSysEmit = func(line string) { captured = append(captured, line) }

	// Disabled by default.
	hostDebugSysLogf("savegame", "slot=%d bytes=%d", 1, 1234)
	if len(captured) != 0 {
		t.Fatalf("expected no emission when disabled, got %v", captured)
	}

	// Level=1 enables basic emission.
	cv.Set(hostDebugSysCVarName, "1")
	hostDebugSysLogf("savegame", "slot=%d bytes=%d", 1, 1234)
	if len(captured) != 1 {
		t.Fatalf("expected 1 emission, got %d", len(captured))
	}
	if !strings.Contains(captured[0], "[sysdbg kind=savegame]") {
		t.Fatalf("missing prefix: %q", captured[0])
	}
	if !strings.Contains(captured[0], "slot=1 bytes=1234") {
		t.Fatalf("missing payload: %q", captured[0])
	}

	// Level gating: at level 1, level-2 logs stay silent.
	hostDebugSysLogfAt(2, "modchunk", "bytes=%d", 4096)
	if len(captured) != 1 {
		t.Fatalf("expected level-2 emission to be suppressed at level=1, got %d", len(captured))
	}

	// At level 2, verbose emissions flow.
	cv.Set(hostDebugSysCVarName, "2")
	hostDebugSysLogfAt(2, "modchunk", "bytes=%d", 4096)
	if len(captured) != 2 {
		t.Fatalf("expected verbose emission at level=2, got %d", len(captured))
	}
	if !strings.Contains(captured[1], "kind=modchunk") || !strings.Contains(captured[1], "bytes=4096") {
		t.Fatalf("missing verbose payload: %q", captured[1])
	}
}

func TestHostDebugSysLevelNilCVar(t *testing.T) {
	orig := hostDebugSysCVar
	t.Cleanup(func() { hostDebugSysCVar = orig })
	hostDebugSysCVar = nil
	if got := hostDebugSysLevel(); got != 0 {
		t.Fatalf("expected 0 level when cvar unregistered, got %d", got)
	}
	if hostDebugSysEnabled(1) {
		t.Fatalf("expected disabled when cvar unregistered")
	}
}
