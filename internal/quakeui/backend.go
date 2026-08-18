// Package quakeui hosts the gogpu/ui integration layer for the experimental
// UI rewrite (IRONWAIL-SPEC-001). It is a Go-only presentation layer: the
// legacy menu state machine, console data, and HUD state aggregation remain
// the sources of truth and are consumed by the widget tree when the
// ui_backend cvar selects path 1.
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
