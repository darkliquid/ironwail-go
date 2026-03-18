package cmdsys

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/ironwail/ironwail-go/internal/cvar"
)

// RegisterCvarCommands registers console commands for cvar manipulation:
// toggle, cycle, inc, reset, resetall, resetcfg. These match the C Ironwail
// console commands that let users modify cvars via the console.
func (c *CmdSystem) RegisterCvarCommands() {
	c.AddCommand("toggle", cmdToggle, "Toggle a boolean cvar between 0 and 1")
	c.AddCommand("cycle", cmdCycle, "Cycle a cvar through a list of values")
	c.AddCommand("inc", cmdInc, "Increment a cvar by a value (default 1)")
	c.AddCommand("reset", cmdReset, "Reset a cvar to its default value")
	c.AddCommand("resetall", cmdResetAll, "Reset all cvars to their default values")
	c.AddCommand("resetcfg", cmdResetCfg, "Reset all archived cvars to their default values")
}

func cmdToggle(args []string) {
	if len(args) < 1 {
		slog.Info("usage: toggle <cvar>")
		return
	}
	cv := cvar.Get(args[0])
	if cv == nil {
		slog.Info("unknown cvar", "name", args[0])
		return
	}
	if cv.Float == 0 {
		cvar.Set(cv.Name, "1")
	} else {
		cvar.Set(cv.Name, "0")
	}
}

func cmdCycle(args []string) {
	if len(args) < 3 {
		slog.Info("usage: cycle <cvar> <val1> <val2> [...]")
		return
	}
	name := args[0]
	values := args[1:]
	cur := cvar.StringValue(name)
	next := values[0]
	for i, v := range values {
		if v == cur {
			next = values[(i+1)%len(values)]
			break
		}
	}
	cvar.Set(name, next)
}

func cmdInc(args []string) {
	if len(args) < 1 {
		slog.Info("usage: inc <cvar> [amount]")
		return
	}
	amount := 1.0
	if len(args) >= 2 {
		if v, err := strconv.ParseFloat(args[1], 64); err == nil {
			amount = v
		}
	}
	cur := cvar.FloatValue(args[0])
	cvar.Set(args[0], fmt.Sprintf("%g", cur+amount))
}

func cmdReset(args []string) {
	if len(args) < 1 {
		slog.Info("usage: reset <cvar>")
		return
	}
	cv := cvar.Get(args[0])
	if cv == nil {
		slog.Info("unknown cvar", "name", args[0])
		return
	}
	cvar.Set(cv.Name, cv.DefaultValue)
}

func cmdResetAll(_ []string) {
	for _, cv := range cvar.All() {
		cvar.Set(cv.Name, cv.DefaultValue)
	}
}

func cmdResetCfg(_ []string) {
	for _, cv := range cvar.All() {
		if cv.Flags&cvar.FlagArchive != 0 {
			cvar.Set(cv.Name, cv.DefaultValue)
		}
	}
}
