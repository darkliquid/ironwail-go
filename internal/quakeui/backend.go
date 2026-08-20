// Package quakeui hosts the gogpu/ui integration for the v2 UI rewrite
// (IRONWAIL-SPEC-002, ADR-0009). It is self-contained: it imports only
// gogpu/ui, the legacy state machines (read-only via accessors), and
// internal/image. The engine (internal/game) implements the quakeui.Host
// adapter and calls quakeui.Run; no engine code lives in this package.
package quakeui

import "github.com/darkliquid/ironwail-go/internal/cvar"

// CvarUIBackend selects the UI render path (spec §4.2, ADR-0001).
// 0 = legacy path (untouched parity oracle), 1 = gogpu/ui widget tree.
const CvarUIBackend = "ui_backend"

// UIBackend returns the current ui_backend cvar value. It returns 0
// (legacy) when the cvar is not registered yet, so the engine fails open
// before game init registers it (ADR-0001).
func UIBackend(cvs *cvar.CVarSystem) int {
	if cvs == nil {
		return 0
	}
	return cvs.IntValue(CvarUIBackend)
}

// IsGogpuUIPath reports whether the gogpu/ui render path is active for the
// current frame. Read per frame; toggling the cvar switches the whole UI
// layer (menu + console + HUD together, spec §1.3).
func IsGogpuUIPath(cvs *cvar.CVarSystem) bool {
	return UIBackend(cvs) != 0
}
