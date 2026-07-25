package renderer

import (
	"github.com/darkliquid/ironwail-go/internal/cvar"
	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
)

// pkgCVars is the cvar system consulted by renderer free functions that
// render-pipeline code paths (warp scaling, shadow resources, world dispatch,
// frame timing, viewport sizing) reach for without a struct receiver.
// SetCVarSystem wires it once during host/game initialisation; tests install
// their own via renderer.SetCVarSystem and restore the previous value.
//
// This is a dependency-injection handle, not unmanaged state: every reader
// nil-checks before use and every write goes through SetCVarSystem.
var pkgCVars *cvar.CVarSystem

// SetCVarSystem installs the package-level cvar system used by renderer
// free functions. Passing nil restores the uninitialised state (all
// cvar reads default to zero/empty and writes are no-ops).
func SetCVarSystem(cv *cvar.CVarSystem) {
	pkgCVars = cv
	worldimpl.SetCVarSystem(cv)
}

// CVarSystem returns the currently installed package-level cvar system,
// or nil if none is wired. Callers must nil-check before use.
func CVarSystem() *cvar.CVarSystem {
	return pkgCVars
}
