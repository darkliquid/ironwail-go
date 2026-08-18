package hud

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/hud"
	"github.com/darkliquid/ironwail-go/internal/quakeui/widgets"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// testConchars returns a minimal 128x128 conchars atlas.
func testConchars() []byte {
	data := make([]byte, 128*128)
	for i := range data {
		data[i] = byte(i / 64)
	}
	return data
}

// testState returns a canned hud.State with representative values.
func testState() hud.State {
	return hud.State{
		Health:       100,
		Armor:        50,
		Ammo:         25,
		ActiveWeapon: 3,
		Shells:       10,
		Nails:        20,
		Rockets:      5,
		Cells:        30,
		Items:        1 << 0,
	}
}

// TestHUDStatusBarClassic asserts the classic style exposes health/armor/ammo
// values from a canned state (AC5: same values as legacy).
func TestHUDStatusBarClassic(t *testing.T) {
	wt := widgets.NewQuakeText(testConchars(), nil)
	sb := NewStatusBarWidget(testState(), hud.HUDStyleClassic, wt)

	if sb.Health() != 100 {
		t.Fatalf("Health() = %d, want 100", sb.Health())
	}
	if sb.Armor() != 50 {
		t.Fatalf("Armor() = %d, want 50", sb.Armor())
	}
	if sb.Ammo() != 25 {
		t.Fatalf("Ammo() = %d, want 25", sb.Ammo())
	}
}

// TestHUDStatusBarWeapon asserts the active weapon is exposed.
func TestHUDStatusBarWeapon(t *testing.T) {
	wt := widgets.NewQuakeText(testConchars(), nil)
	sb := NewStatusBarWidget(testState(), hud.HUDStyleClassic, wt)

	if sb.Weapon() != 3 {
		t.Fatalf("Weapon() = %d, want 3", sb.Weapon())
	}
}

// TestHUDStatusBarAmmoCounts asserts the per-ammo-type counts are exposed.
func TestHUDStatusBarAmmoCounts(t *testing.T) {
	wt := widgets.NewQuakeText(testConchars(), nil)
	sb := NewStatusBarWidget(testState(), hud.HUDStyleClassic, wt)

	if sb.Shells() != 10 || sb.Nails() != 20 || sb.Rockets() != 5 || sb.Cells() != 30 {
		t.Fatalf("ammo counts = (%d,%d,%d,%d), want (10,20,5,30)",
			sb.Shells(), sb.Nails(), sb.Rockets(), sb.Cells())
	}
}

// TestHUDStatusBarStyle asserts the widget reflects the style.
func TestHUDStatusBarStyle(t *testing.T) {
	wt := widgets.NewQuakeText(testConchars(), nil)
	for _, style := range []hud.HUDStyle{
		hud.HUDStyleClassic,
		hud.HUDStyleModernCenterAmmo,
		hud.HUDStyleModernSideAmmo,
		hud.HUDStyleQuakeWorld,
	} {
		sb := NewStatusBarWidget(testState(), style, wt)
		if sb.Style() != style {
			t.Fatalf("Style() = %v, want %v", sb.Style(), style)
		}
	}
}

// TestHUDStatusBarLayout asserts the widget lays out to a positive size.
func TestHUDStatusBarLayout(t *testing.T) {
	wt := widgets.NewQuakeText(testConchars(), nil)
	sb := NewStatusBarWidget(testState(), hud.HUDStyleClassic, wt)

	ctx := widget.NewContext()
	size := sb.Layout(ctx, geometry.Expand())
	if size.Width <= 0 || size.Height <= 0 {
		t.Fatalf("Layout size = %v, want positive", size)
	}
}

// TestHUDStatusBarScoreboard asserts scoreboard entries are exposed when
// ShowScores is set.
func TestHUDStatusBarScoreboard(t *testing.T) {
	st := testState()
	st.ShowScores = true
	st.Scoreboard = []hud.ScoreEntry{
		{Name: "player1", Frags: 10, IsCurrent: true},
		{Name: "player2", Frags: 5},
	}
	wt := widgets.NewQuakeText(testConchars(), nil)
	sb := NewStatusBarWidget(st, hud.HUDStyleClassic, wt)

	rows := sb.ScoreRows()
	if len(rows) != 2 {
		t.Fatalf("ScoreRows() = %d, want 2", len(rows))
	}
	if rows[0].Name != "player1" || rows[0].Frags != 10 {
		t.Fatalf("ScoreRows()[0] = %+v, want player1/10", rows[0])
	}
	if !rows[0].Current {
		t.Fatal("ScoreRows()[0].Current = false, want true")
	}
}
