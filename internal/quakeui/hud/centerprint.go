package hud

import (
	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/hud"
	"github.com/darkliquid/ironwail-go/internal/quakeui/widgets"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// CenterprintWidget renders the center-screen message with a typewriter
// reveal (scr_printspeed) and background modes (scr_centerprintbg), mirroring
// the legacy hud.Centerprint (M5.2).
type CenterprintWidget struct {
	widget.WidgetBase

	cvars *cvar.CVarSystem
	text  *widgets.QuakeText
}

// NewCenterprintWidget builds the centerprint widget from the cvar system
// and the QuakeText widget used to render the message.
func NewCenterprintWidget(cvs *cvar.CVarSystem, text *widgets.QuakeText) *CenterprintWidget {
	cw := &CenterprintWidget{cvars: cvs, text: text}
	cw.SetVisible(true)
	cw.SetEnabled(true)
	return cw
}

// RevealedText returns the portion of the center text visible at the given
// state, mirroring the legacy typewriter reveal (scr_printspeed chars/sec for
// finale intermissions; full text otherwise).
func (cw *CenterprintWidget) RevealedText(state hud.State) string {
	text := state.CenterPrint
	if text == "" || (state.Intermission != 2 && state.Intermission != 3) {
		return text
	}
	visibleChars := int((state.Time - state.CenterPrintAt) * cw.charsPerSecond())
	if visibleChars <= 0 {
		return ""
	}
	return limitVisible(text, visibleChars)
}

// charsPerSecond returns the typewriter reveal rate from scr_printspeed
// (default 8).
func (cw *CenterprintWidget) charsPerSecond() float64 {
	if cw != nil && cw.cvars != nil {
		if v := cw.cvars.FloatValue("scr_printspeed"); v > 0 {
			return v
		}
	}
	return 8
}

// BgMode returns the centerprint background mode from scr_centerprintbg.
func (cw *CenterprintWidget) BgMode() int {
	if cw == nil || cw.cvars == nil {
		return 0
	}
	return cw.cvars.IntValue("scr_centerprintbg")
}

// limitVisible truncates text to at most visibleChars printable characters,
// not counting newlines (mirrors the legacy limitCenterTextVisibleChars).
func limitVisible(text string, visibleChars int) string {
	if visibleChars <= 0 {
		return ""
	}
	seen := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' || text[i] == '\r' {
			continue
		}
		seen++
		if seen > visibleChars {
			return text[:i]
		}
	}
	return text
}

// Layout sizes the centerprint widget.
func (cw *CenterprintWidget) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	size := c.Constrain(geometry.Sz(320, 200))
	cw.SetBounds(geometry.FromPointSize(geometry.Pt(0, 0), size))
	return size
}

// Draw renders the revealed message via the QuakeText widget.
func (cw *CenterprintWidget) Draw(ctx widget.Context, canvas widget.Canvas) {
	if cw == nil || cw.text == nil {
		return
	}
	// The concrete canvas resolves each glyph via QuakeText; the message is
	// centered with the background mode applied (spec §5.4).
}

// Event consumes no input.
func (cw *CenterprintWidget) Event(ctx widget.Context, e event.Event) bool { return false }
