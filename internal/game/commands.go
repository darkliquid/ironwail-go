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

// GlobalCommandBuffer is an implementation that delegates to the global cmdsys.
type GlobalCommandBuffer struct{}

var _ CommandBuffer = GlobalCommandBuffer{}

func (GlobalCommandBuffer) Init()    {}
func (GlobalCommandBuffer) Execute() { cmdsys.Execute() }
func (GlobalCommandBuffer) ExecuteWithSource(source cmdsys.CommandSource) {
	cmdsys.ExecuteWithSource(source)
}
func (GlobalCommandBuffer) ExecuteTextWithSource(text string, source cmdsys.CommandSource) {
	cmdsys.ExecuteTextWithSource(text, source)
}
func (GlobalCommandBuffer) AddText(text string) { cmdsys.AddText(text) }
func (GlobalCommandBuffer) InsertText(text string) {
	cmdsys.InsertText(text)
}
func (GlobalCommandBuffer) Shutdown() {}
