package game

import "github.com/darkliquid/ironwail-go/internal/cmdsys"

// CommandBuffer provides command execution functionality.
type CommandBuffer interface {
	Init()
	Execute()
	ExecuteWithSource(cmdsys.CommandSource)
	ExecuteTextWithSource(string, cmdsys.CommandSource)
	AddText(string)
	InsertText(string)
	Shutdown()
}
