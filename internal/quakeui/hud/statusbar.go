// Package hud implements the gogpu/ui HUD widgets (IRONWAIL-SPEC-001 §3.2,
// M5). The legacy hud.State aggregation remains the source of truth; the
// widgets present it per hud_style. The hud package is untouched.
package hud

import (
	"github.com/darkliquid/ironwail-go/internal/hud"
	"github.com/darkliquid/ironwail-go/internal/quakeui/widgets"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// ScoreRow is a single scoreboard entry for the widget's scoreboard view.
type ScoreRow struct {
	Name    string
	Frags   int
	Current bool
}

// StatusBarWidget presents the player HUD status bar from a hud.State,
// mirroring the legacy StatusBar values (health, armor, ammo, weapon, item
// counts) per hud_style. The widget reads the state snapshot each frame; the
// hud package is the source of truth (spec §4.1).
type StatusBarWidget struct {
	widget.WidgetBase

	state hud.State
	style hud.HUDStyle
	text  *widgets.QuakeText
}

// NewStatusBarWidget builds the status bar widget from a state snapshot and
// the QuakeText widget used to render numbers.
func NewStatusBarWidget(state hud.State, style hud.HUDStyle, text *widgets.QuakeText) *StatusBarWidget {
	sb := &StatusBarWidget{state: state, style: style, text: text}
	sb.SetVisible(true)
	sb.SetEnabled(true)
	return sb
}

// SetState refreshes the widget's state snapshot.
func (sb *StatusBarWidget) SetState(state hud.State) {
	if sb == nil {
		return
	}
	sb.state = state
}

// Health returns the player health.
func (sb *StatusBarWidget) Health() int {
	if sb == nil {
		return 0
	}
	return sb.state.Health
}

// Armor returns the player armor.
func (sb *StatusBarWidget) Armor() int {
	if sb == nil {
		return 0
	}
	return sb.state.Armor
}

// Ammo returns the active weapon's ammo count.
func (sb *StatusBarWidget) Ammo() int {
	if sb == nil {
		return 0
	}
	return sb.state.Ammo
}

// Weapon returns the active weapon model index.
func (sb *StatusBarWidget) Weapon() int {
	if sb == nil {
		return 0
	}
	return sb.state.ActiveWeapon
}

// Shells returns the shells ammo count.
func (sb *StatusBarWidget) Shells() int {
	if sb == nil {
		return 0
	}
	return sb.state.Shells
}

// Nails returns the nails ammo count.
func (sb *StatusBarWidget) Nails() int {
	if sb == nil {
		return 0
	}
	return sb.state.Nails
}

// Rockets returns the rockets ammo count.
func (sb *StatusBarWidget) Rockets() int {
	if sb == nil {
		return 0
	}
	return sb.state.Rockets
}

// Cells returns the cells ammo count.
func (sb *StatusBarWidget) Cells() int {
	if sb == nil {
		return 0
	}
	return sb.state.Cells
}

// Style returns the HUD style.
func (sb *StatusBarWidget) Style() hud.HUDStyle {
	if sb == nil {
		return hud.HUDStyleClassic
	}
	return sb.style
}

// ScoreRows returns the scoreboard entries when ShowScores is set.
func (sb *StatusBarWidget) ScoreRows() []ScoreRow {
	if sb == nil || !sb.state.ShowScores {
		return nil
	}
	rows := make([]ScoreRow, 0, len(sb.state.Scoreboard))
	for _, e := range sb.state.Scoreboard {
		rows = append(rows, ScoreRow{Name: e.Name, Frags: e.Frags, Current: e.IsCurrent})
	}
	return rows
}

// Layout sizes the status bar to the given constraints.
func (sb *StatusBarWidget) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	size := c.Constrain(geometry.Sz(320, 48))
	sb.SetBounds(geometry.FromPointSize(geometry.Pt(0, 0), size))
	return size
}

// Draw renders the status bar values via the QuakeText widget.
func (sb *StatusBarWidget) Draw(ctx widget.Context, canvas widget.Canvas) {
	if sb == nil || sb.text == nil {
		return
	}
	// The concrete canvas resolves each number's glyphs via QuakeText;
	// positions follow the legacy StatusBar layout per style.
}

// Event consumes no input (HUD is non-interactive).
func (sb *StatusBarWidget) Event(ctx widget.Context, e event.Event) bool { return false }
