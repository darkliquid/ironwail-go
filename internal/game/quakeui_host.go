package game

// quakeuiHost adapts the engine to internal/quakeui.Host (ADR-0009, AC7). The
// engine implements the adapter with plain values only; no quakeui code lives
// in the game package.
type quakeuiHost struct {
	g *Game
}

// CVar reads an engine cvar as a plain float.
func (h *quakeuiHost) CVar(name string) float64 {
	if h == nil || h.g == nil || h.g.Host == nil || h.g.Host.CVar == nil {
		return 0
	}
	return h.g.Host.CVar.FloatValue(name)
}

// ExecuteCommandText queues an engine console command.
func (h *quakeuiHost) ExecuteCommandText(text string) {
	if h == nil || h.g == nil || h.g.Host == nil || h.g.Host.Cmd == nil {
		return
	}
	h.g.Host.Cmd.AddText(text)
}

// PlaySound plays a sound by name through the engine audio path.
func (h *quakeuiHost) PlaySound(name string) {
	if h == nil || h.g == nil {
		return
	}
	h.g.playMenuSound(name)
}

// Quit requests a clean engine shutdown from the ui loop.
func (h *quakeuiHost) Quit() {
	if h == nil || h.g == nil || h.g.Host == nil {
		return
	}
	h.g.Host.Abort("quakeui quit")
}
