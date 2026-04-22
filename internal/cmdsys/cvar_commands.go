package cmdsys

import (
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

// RegisterCvarCommands registers console commands for cvar manipulation:
// cvarlist, toggle, cycle, cycleback, inc, reset, resetall, resetcfg. These match the C Ironwail
// console commands that let users modify cvars via the console.
func (c *CmdSystem) RegisterCvarCommands() {
	c.AddCommand("cvarlist", c.cmdCvarList, "List all registered cvars")
	c.AddCommand("toggle", c.cmdToggle, "Toggle a boolean cvar between 0 and 1")
	c.AddCommand("cycle", c.cmdCycle, "Cycle a cvar through a list of values")
	c.AddCommand("cycleback", c.cmdCycleBack, "Cycle a cvar backward through a list of values")
	c.AddCommand("inc", c.cmdInc, "Increment a cvar by a value (default 1)")
	c.AddCommand("reset", c.cmdReset, "Reset a cvar to its default value")
	c.AddCommand("resetall", c.cmdResetAll, "Reset all cvars to their default values")
	c.AddCommand("resetcfg", c.cmdResetCfg, "Reset all archived cvars to their default values")
}

func (c *CmdSystem) cmdCvarList(args []string) {
	if c.CVar == nil {
		return
	}
	vars := c.CVar.All()
	slices.SortFunc(vars, func(a, b *cvar.CVar) int {
		return strings.Compare(a.Name, b.Name)
	})

	partial := ""
	if len(args) > 0 {
		partial = strings.ToLower(args[0])
	}

	count := 0
	for _, cv := range vars {
		if partial != "" && !strings.HasPrefix(cv.Name, partial) {
			continue
		}
		archiveMarker := " "
		if cv.Flags&cvar.FlagArchive != 0 {
			archiveMarker = "*"
		}
		notifyMarker := " "
		if cv.Flags&cvar.FlagNotify != 0 {
			notifyMarker = "s"
		}
		c.printCallback(fmt.Sprintf("%s%s %s %q\n", archiveMarker, notifyMarker, cv.Name, cv.String))
		count++
	}

	msg := fmt.Sprintf("%d cvars", count)
	if partial != "" {
		msg += fmt.Sprintf(" beginning with %q", partial)
	}
	c.printCallback(msg + "\n")
}

func (c *CmdSystem) cmdToggle(args []string) {
	if len(args) < 1 {
		slog.Info("usage: toggle <cvar>")
		return
	}
	if c.CVar == nil {
		return
	}
	cv := c.CVar.Get(args[0])
	if cv == nil {
		slog.Info("unknown cvar", "name", args[0])
		return
	}
	if cv.Float == 0 {
		c.CVar.Set(cv.Name, "1")
	} else {
		c.CVar.Set(cv.Name, "0")
	}
}

func cvarHasValue(cv *cvar.CVar, value string) bool {
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return cv.Float == f
	}
	return cv.String == value
}

func (c *CmdSystem) cmdCycle(args []string) {
	if len(args) < 3 {
		slog.Info("usage: cycle <cvar> <val1> <val2> [...]")
		return
	}
	if c.CVar == nil {
		return
	}
	cv := c.CVar.Get(args[0])
	if cv == nil {
		slog.Info("unknown cvar", "name", args[0])
		return
	}
	values := args[1:]
	next := values[0]
	for i, v := range values {
		if cvarHasValue(cv, v) {
			next = values[(i+1)%len(values)]
			break
		}
	}
	c.CVar.Set(cv.Name, next)
}

func (c *CmdSystem) cmdCycleBack(args []string) {
	if len(args) < 3 {
		slog.Info("usage: cycleback <cvar> <val1> <val2> [...]")
		return
	}
	if c.CVar == nil {
		return
	}
	cv := c.CVar.Get(args[0])
	if cv == nil {
		slog.Info("unknown cvar", "name", args[0])
		return
	}
	values := args[1:]
	prev := values[len(values)-1]
	for i := len(values) - 1; i >= 0; i-- {
		if cvarHasValue(cv, values[i]) {
			prev = values[(i-1+len(values))%len(values)]
			break
		}
	}
	c.CVar.Set(cv.Name, prev)
}

func (c *CmdSystem) cmdInc(args []string) {
	if len(args) < 1 {
		slog.Info("usage: inc <cvar> [amount]")
		return
	}
	if c.CVar == nil {
		return
	}
	cv := c.CVar.Get(args[0])
	if cv == nil {
		slog.Info("unknown cvar", "name", args[0])
		return
	}
	amount := 1.0
	if len(args) >= 2 {
		if v, err := strconv.ParseFloat(args[1], 64); err == nil {
			amount = v
		}
	}
	c.CVar.Set(cv.Name, fmt.Sprintf("%g", cv.Float+amount))
}

func (c *CmdSystem) cmdReset(args []string) {
	if len(args) < 1 {
		slog.Info("usage: reset <cvar>")
		return
	}
	if c.CVar == nil {
		return
	}
	cv := c.CVar.Get(args[0])
	if cv == nil {
		slog.Info("unknown cvar", "name", args[0])
		return
	}
	c.CVar.Set(cv.Name, cv.DefaultValue)
}

func (c *CmdSystem) cmdResetAll(_ []string) {
	if c.CVar == nil {
		return
	}
	for _, cv := range c.CVar.All() {
		c.CVar.Set(cv.Name, cv.DefaultValue)
	}
}

func (c *CmdSystem) cmdResetCfg(_ []string) {
	if c.CVar == nil {
		return
	}
	for _, cv := range c.CVar.All() {
		if cv.Flags&cvar.FlagArchive != 0 {
			c.CVar.Set(cv.Name, cv.DefaultValue)
		}
	}
}
