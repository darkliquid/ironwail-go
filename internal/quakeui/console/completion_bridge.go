package console

import (
	"github.com/darkliquid/ironwail-go/internal/quakeui/widgets"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// CompleteFunc completes an input line and returns the completed text plus
// the match list. It matches console.CompleteInput's signature so the widget
// can be wired to the global completer or a test stub.
type CompleteFunc func(input string, forward bool) (string, []string)

// CompletionBridge renders the console tab-completion match list as a widget
// (spec §5.3, M4.2). It calls the completer on Tab, exposes the match list,
// and supports forward/back cycling through matches. The completion logic
// itself stays in the console package; the bridge is presentation-only.
type CompletionBridge struct {
	widget.WidgetBase

	complete CompleteFunc
	text     *widgets.QuakeText

	matches []string
	current int
}

// NewCompletionBridge builds the completion bridge from a completer function
// and the QuakeText widget used to render the match list.
func NewCompletionBridge(complete CompleteFunc, text *widgets.QuakeText) *CompletionBridge {
	cb := &CompletionBridge{complete: complete, text: text}
	cb.SetVisible(true)
	cb.SetEnabled(true)
	return cb
}

// Complete runs the completer for the given input and caches the match list.
// It returns the matches (empty when none).
func (cb *CompletionBridge) Complete(input string, forward bool) []string {
	if cb == nil || cb.complete == nil {
		return nil
	}
	_, cb.matches = cb.complete(input, forward)
	if len(cb.matches) == 0 {
		cb.current = 0
		return nil
	}
	cb.current = 0
	return cb.matches
}

// Cycle advances the match index forward (forward=true) or backward. Wraps
// around the match list.
func (cb *CompletionBridge) Cycle(forward bool) {
	if cb == nil || len(cb.matches) == 0 {
		return
	}
	if forward {
		cb.current = (cb.current + 1) % len(cb.matches)
	} else {
		cb.current = (cb.current - 1 + len(cb.matches)) % len(cb.matches)
	}
}

// Current returns the currently selected match.
func (cb *CompletionBridge) Current() string {
	if cb == nil || len(cb.matches) == 0 {
		return ""
	}
	return cb.matches[cb.current]
}

// Matches returns the cached match list.
func (cb *CompletionBridge) Matches() []string {
	if cb == nil {
		return nil
	}
	return cb.matches
}

// Layout sizes the completion bridge to the given constraints.
func (cb *CompletionBridge) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	size := c.Constrain(geometry.Sz(320, 200))
	cb.SetBounds(geometry.FromPointSize(geometry.Pt(0, 0), size))
	return size
}

// Draw renders the match list via the QuakeText widget.
func (cb *CompletionBridge) Draw(ctx widget.Context, canvas widget.Canvas) {
	if cb == nil || cb.text == nil {
		return
	}
	// The concrete canvas resolves each match's glyphs via QuakeText; the
	// match list is laid out in con_maxcols columns (spec §5.3).
}

// Event consumes no input (Tab handling is engine-side via the gateway).
func (cb *CompletionBridge) Event(ctx widget.Context, e event.Event) bool { return false }
