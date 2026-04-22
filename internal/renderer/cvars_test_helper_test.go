package renderer

import (
	"github.com/darkliquid/ironwail-go/internal/cvar"
	rworld "github.com/darkliquid/ironwail-go/internal/renderer/world"
)

// testCV is the cvar system used by the package's own tests. It is installed
// via SetCVarSystem at test init so code under test reads the same instance
// tests write to.
var testCV = cvar.NewCVarSystem()

func init() {
	SetCVarSystem(testCV)
	rworld.SetCVarSystem(testCV)
}
