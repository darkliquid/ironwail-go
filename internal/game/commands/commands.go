// Package commands implements console command dispatchers, keybinding handlers, and debug camera controls.
package commands

import (
	"github.com/darkliquid/ironwail-go/internal/cmdsys"
)

// Dispatcher manages console command dispatch and execution.
type Dispatcher struct {
	cmdSys *cmdsys.CmdSystem
}

// NewDispatcher creates a new command dispatcher wrapping the given command system.
func NewDispatcher(cmdSys *cmdsys.CmdSystem) *Dispatcher {
	return &Dispatcher{
		cmdSys: cmdSys,
	}
}
