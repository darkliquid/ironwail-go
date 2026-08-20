package console

import "github.com/darkliquid/ironwail-go/internal/console"

// Complete processes a tab-completion request on the console's current input line.
// It updates the console input line with the completed text, caches candidate matches,
// and returns them.
func (r *ConsoleRoot) Complete(forward bool) []string {
	if r == nil || r.con == nil {
		return nil
	}
	completed, matches := console.CompleteInput(r.con.InputLine(), forward)
	if completed != "" {
		r.con.SetInputLine(completed)
		r.con.MoveCursorEnd()
		r.InvalidateScene()
	}
	r.matches = matches
	return matches
}

// ClearMatches clears any active tab-completion match list.
func (r *ConsoleRoot) ClearMatches() {
	if r != nil && len(r.matches) > 0 {
		r.matches = nil
		r.InvalidateScene()
	}
}

// Matches returns the current candidate matches.
func (r *ConsoleRoot) Matches() []string {
	if r == nil {
		return nil
	}
	return r.matches
}
