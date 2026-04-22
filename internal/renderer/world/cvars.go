package world

import "github.com/darkliquid/ironwail-go/internal/cvar"

// pkgCVars is the cvar system consulted by world helpers that have no
// direct owner. SetCVarSystem is called during renderer initialisation.
var pkgCVars *cvar.CVarSystem

// SetCVarSystem installs the cvar system used by world helpers. Passing
// nil clears it.
func SetCVarSystem(cv *cvar.CVarSystem) {
	pkgCVars = cv
}
