package progs

import (
	"quake"
	"quake/engine"
)

// FuncVar is a package-level function value that is expected to be
// re-assigned at runtime (like QuakeC prototypes).
var FuncVar func()

// Target is a normal target function; its index must be storable into
// a func cell and callable indirectly.
func Target() {
	engine.Bprint("target called")
}

// RunFunc assigns a function to the entity's func-valued Think field and
// calls through it, proving the function index lands in the field cell and
// OP_CALL executes the stored function.
func RunFunc(th *quake.Entity) float32 {
	th.Think = Target
	th.Think()
	return 42
}

// SprintfStr tests quake.Sprintf expansion: a literal format with %s and
// %f directives joined via engine strcat/ftos builtins.
func SprintfStr(name string, hp float32) string {
	return quake.Sprintf("$qc_test %s %f", name, hp)
}
