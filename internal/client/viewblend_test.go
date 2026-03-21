// What: View shift (color tinting/flashing) tests.
// Why: Verifies visual feedback for damage, powerups, and underwater effects.
// Where in C: viewblend.go

package client

import (
	"math"
	"testing"
)

// TestSetContentsColorLava verifies that entering lava triggers the correct color shift.
// Why: Lava should provide a strong orange-red tint to indicate the player is submerged in a hazardous liquid.
// Where in C: cl_main.c, V_CalcBlend.
func TestSetContentsColorLava(t *testing.T) {
	c := NewClient()
	c.SetContentsColor(-5) // ContentsLava
	if c.CShifts[CShiftContents].Percent != 150 {
		t.Fatalf("lava percent = %g, want 150", c.CShifts[CShiftContents].Percent)
	}
	if c.CShifts[CShiftContents].R != 255 || c.CShifts[CShiftContents].G != 80 || c.CShifts[CShiftContents].B != 0 {
		t.Fatalf("lava color = %v, want {255 80 0}", c.CShifts[CShiftContents])
	}
}

// TestSetContentsColorSlime verifies that entering slime triggers the correct color shift.
// Why: Slime should provide a green tint to indicate the player is submerged in toxic liquid.
// Where in C: cl_main.c, V_CalcBlend.
func TestSetContentsColorSlime(t *testing.T) {
	c := NewClient()
	c.SetContentsColor(-4) // ContentsSlime
	if c.CShifts[CShiftContents].Percent != 150 {
		t.Fatalf("slime percent = %g, want 150", c.CShifts[CShiftContents].Percent)
	}
	if c.CShifts[CShiftContents].R != 0 || c.CShifts[CShiftContents].G != 25 || c.CShifts[CShiftContents].B != 5 {
		t.Fatalf("slime color = %v, want {0 25 5}", c.CShifts[CShiftContents])
	}
}

// TestSetContentsColorWater verifies that entering water triggers the correct color shift.
// Why: Water should provide a blue tint to indicate the player is submerged.
// Where in C: cl_main.c, V_CalcBlend.
func TestSetContentsColorWater(t *testing.T) {
	c := NewClient()
	c.SetContentsColor(-3) // ContentsWater
	if c.CShifts[CShiftContents].Percent != 128 {
		t.Fatalf("water percent = %g, want 128", c.CShifts[CShiftContents].Percent)
	}
}

// TestSetContentsColorEmpty verifies that clear contents do not trigger any color shift.
// Why: Moving between air, solids, or sky should not result in any persistent screen tinting.
// Where in C: cl_main.c, V_CalcBlend.
func TestSetContentsColorEmpty(t *testing.T) {
	c := NewClient()
	for _, contents := range []int32{-1, -2, -6} { // empty, solid, sky
		c.SetContentsColor(contents)
		if c.CShifts[CShiftContents].Percent != 0 {
			t.Fatalf("contents %d: percent = %g, want 0", contents, c.CShifts[CShiftContents].Percent)
		}
	}
}

// TestApplyDamage_BloodOnly verifies that taking health damage triggers a red screen flash.
// Why: Visual feedback for taking damage is critical for gameplay.
// Where in C: cl_main.c, V_CalcBlend.
func TestApplyDamage_BloodOnly(t *testing.T) {
	c := NewClient()
	c.DamageTaken = 20
	c.DamageSaved = 0
	c.ApplyDamage()
	if c.CShifts[CShiftDamage].R != 255 || c.CShifts[CShiftDamage].G != 0 || c.CShifts[CShiftDamage].B != 0 {
		t.Fatalf("blood damage color = %v, want {255 0 0}", c.CShifts[CShiftDamage])
	}
	// count = 20*0.5 + 0 = 10; percent = 3*10 = 30
	wantPct := float32(30)
	if c.CShifts[CShiftDamage].Percent != wantPct {
		t.Fatalf("blood damage percent = %g, want %g", c.CShifts[CShiftDamage].Percent, wantPct)
	}
}

// TestApplyDamage_ArmorOnly verifies that armor hits trigger a distinct color flash.
// Why: Differentiating between health and armor damage helps the player assess their survival state.
// Where in C: cl_main.c, V_CalcBlend.
func TestApplyDamage_ArmorOnly(t *testing.T) {
	c := NewClient()
	c.DamageTaken = 0
	c.DamageSaved = 20
	c.ApplyDamage()
	// armor > blood
	if c.CShifts[CShiftDamage].R != 200 || c.CShifts[CShiftDamage].G != 100 || c.CShifts[CShiftDamage].B != 100 {
		t.Fatalf("armor damage color = %v, want {200 100 100}", c.CShifts[CShiftDamage])
	}
}

// TestApplyDamage_Accumulation verifies that rapid successive hits increase the intensity of the damage flash.
// Why: Cumulative damage should be reflected visually to stress the urgency of the situation.
// Where in C: cl_main.c, V_CalcBlend.
func TestApplyDamage_Accumulation(t *testing.T) {
	c := NewClient()
	c.DamageTaken = 20
	c.DamageSaved = 0
	c.ApplyDamage() // +30
	c.ApplyDamage() // +30 → 60
	if c.CShifts[CShiftDamage].Percent != 60 {
		t.Fatalf("accumulated percent = %g, want 60", c.CShifts[CShiftDamage].Percent)
	}
}

// TestApplyDamage_Cap verifies that the damage flash intensity is capped.
// Why: Preventing the screen from becoming entirely opaque ensures the player can still see to react.
// Where in C: cl_main.c, V_CalcBlend.
func TestApplyDamage_Cap(t *testing.T) {
	c := NewClient()
	c.DamageTaken = 100
	c.DamageSaved = 100
	for i := 0; i < 10; i++ {
		c.ApplyDamage()
	}
	if c.CShifts[CShiftDamage].Percent > 150 {
		t.Fatalf("percent = %g, should be capped at 150", c.CShifts[CShiftDamage].Percent)
	}
}

// TestBonusFlash verifies the golden flash when picking up items.
// Why: Positive visual reinforcement for item collection enhances game feel.
// Where in C: cl_main.c, V_CalcBlend.
func TestBonusFlash(t *testing.T) {
	c := NewClient()
	c.BonusFlash()
	if c.CShifts[CShiftBonus].Percent != 50 {
		t.Fatalf("bonus percent = %g, want 50", c.CShifts[CShiftBonus].Percent)
	}
	if c.CShifts[CShiftBonus].R != 215 || c.CShifts[CShiftBonus].G != 186 || c.CShifts[CShiftBonus].B != 69 {
		t.Fatalf("bonus color = %v, want {215 186 69}", c.CShifts[CShiftBonus])
	}
}

// TestUpdateBlend_DecayDamage verifies that the damage flash fades over time.
// Why: Screen tints should be temporary to avoid obscuring vision indefinitely.
// Where in C: cl_main.c, V_CalcBlend.
func TestUpdateBlend_DecayDamage(t *testing.T) {
	c := NewClient()
	c.DamageTaken = 20
	c.DamageSaved = 0
	c.ApplyDamage() // percent = 30
	c.UpdateBlend(0.1)
	// decays by 150 * 0.1 = 15; expect 30 - 15 = 15
	want := float32(15)
	if c.CShifts[CShiftDamage].Percent != want {
		t.Fatalf("after decay: percent = %g, want %g", c.CShifts[CShiftDamage].Percent, want)
	}
}

// TestUpdateBlend_DecayBonus verifies that the item pickup flash fades over time.
// Why: Visual feedback should be impactful but transient.
// Where in C: cl_main.c, V_CalcBlend.
func TestUpdateBlend_DecayBonus(t *testing.T) {
	c := NewClient()
	c.BonusFlash() // percent = 50
	c.UpdateBlend(0.5)
	// decays by 100 * 0.5 = 50; expect 50 - 50 = 0
	if c.CShifts[CShiftBonus].Percent != 0 {
		t.Fatalf("after decay: bonus percent = %g, want 0", c.CShifts[CShiftBonus].Percent)
	}
}

// TestUpdateBlend_PowerupQuad verifies the blue tint while Quad Damage is active.
// Why: Players need a constant visual reminder of their powered-up state.
// Where in C: cl_main.c, V_CalcBlend.
func TestUpdateBlend_PowerupQuad(t *testing.T) {
	c := NewClient()
	c.Items = uint32(ItemQuad)
	c.UpdateBlend(0)
	if c.CShifts[CShiftPowerup].B != 255 || c.CShifts[CShiftPowerup].Percent != 30 {
		t.Fatalf("quad powerup = %v, want blue/30", c.CShifts[CShiftPowerup])
	}
}

// TestUpdateBlend_PowerupSuit verifies the green tint while the Biohazard Suit is active.
// Why: Indicates protection from environmental hazards.
// Where in C: cl_main.c, V_CalcBlend.
func TestUpdateBlend_PowerupSuit(t *testing.T) {
	c := NewClient()
	c.Items = uint32(ItemSuit)
	c.UpdateBlend(0)
	if c.CShifts[CShiftPowerup].G != 255 || c.CShifts[CShiftPowerup].Percent != 20 {
		t.Fatalf("suit powerup = %v, want green/20", c.CShifts[CShiftPowerup])
	}
}

// TestCalcBlend_ZeroAlphaWhenNoShifts verifies that no tint is applied when no shifts are active.
// Why: The screen should remain clear during normal gameplay.
// Where in C: cl_main.c, V_CalcBlend.
func TestCalcBlend_ZeroAlphaWhenNoShifts(t *testing.T) {
	c := NewClient()
	blend := c.CalcBlend(100, [NumCShifts]float32{100, 100, 100, 100})
	if blend[3] != 0 {
		t.Fatalf("empty blend alpha = %g, want 0", blend[3])
	}
}

// TestCalcBlend_ZeroWhenGlobalPercentIsZero verifies that the global blend scale can disable all tints.
// Why: Provides user control over visual effects through console variables.
// Where in C: cl_main.c, V_CalcBlend.
func TestCalcBlend_ZeroWhenGlobalPercentIsZero(t *testing.T) {
	c := NewClient()
	c.BonusFlash()
	blend := c.CalcBlend(0, [NumCShifts]float32{100, 100, 100, 100})
	if blend[3] != 0 {
		t.Fatalf("blend with 0 global percent = %g, want 0", blend[3])
	}
}

// TestCalcBlend_DamageRedTint verifies that damage shifts result in a red-dominant color.
// Why: Red is the universal indicator for danger and health loss.
// Where in C: cl_main.c, V_CalcBlend.
func TestCalcBlend_DamageRedTint(t *testing.T) {
	c := NewClient()
	c.DamageTaken = 30
	c.DamageSaved = 0
	c.ApplyDamage()
	blend := c.CalcBlend(100, [NumCShifts]float32{100, 100, 100, 100})
	if blend[3] <= 0 {
		t.Fatalf("damage blend alpha = %g, want > 0", blend[3])
	}
	if blend[0] <= blend[1] || blend[0] <= blend[2] {
		t.Fatalf("damage blend should be red-dominant: R=%g G=%g B=%g", blend[0], blend[1], blend[2])
	}
}

// TestCalcBlend_AlphaIsClamped verifies that the final composite alpha never exceeds 100%.
// Why: Rendering systems expect normalized alpha values to avoid artifacts.
// Where in C: cl_main.c, V_CalcBlend.
func TestCalcBlend_AlphaIsClamped(t *testing.T) {
	c := NewClient()
	// Saturate all shifts
	for i := range c.CShifts {
		c.CShifts[i] = ColorShift{R: 255, G: 0, B: 0, Percent: 255}
	}
	blend := c.CalcBlend(100, [NumCShifts]float32{100, 100, 100, 100})
	if blend[3] > 1 {
		t.Fatalf("alpha = %g, want <= 1", blend[3])
	}
}

// TestCalcBlend_IntermissionOnlyContents verifies that only environmental tints apply during intermission.
// Why: Prevents damage or pickup flashes from appearing when the game is paused or over.
// Where in C: cl_main.c, V_CalcBlend.
func TestCalcBlend_IntermissionOnlyContents(t *testing.T) {
	c := NewClient()
	c.Intermission = 1
	c.SetContentsColor(-5) // lava: percent 150, orange-red
	c.BonusFlash()         // bonus: percent 50, gold

	blend := c.CalcBlend(100, [NumCShifts]float32{100, 100, 100, 100})
	// Only lava contents shift should apply.
	// Lava: R=255/255=1.0, G=80/255≈0.314, B=0
	// The blend color should be dominated by lava's orange-red.
	if blend[3] <= 0 {
		t.Fatalf("intermission blend alpha = %g, want > 0 (from lava contents)", blend[3])
	}
	if blend[0] < 0.9 {
		t.Fatalf("intermission lava R = %g, want ~1.0 (lava 255/255)", blend[0])
	}
}

// approxEqual checks two float32 values are within eps of each other.
func approxEqual(a, b, eps float32) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff <= eps
}

// TestCalcBlend_CompositeMultipleShifts verifies that multiple active tints are correctly blended together.
// Why: Ensures correct visuals when taking damage while submerged or powered up.
// Where in C: cl_main.c, V_CalcBlend.
func TestCalcBlend_CompositeMultipleShifts(t *testing.T) {
	c := NewClient()
	// Set a water contents shift (percent=128, color=130,80,50)
	c.SetContentsColor(-3)
	// Set bonus flash (percent=50, color=215,186,69)
	c.BonusFlash()

	blend := c.CalcBlend(100, [NumCShifts]float32{100, 100, 100, 100})
	// Both should contribute: alpha should be somewhere between the two individual alphas.
	if blend[3] <= 0 {
		t.Fatalf("composite blend alpha = %g, want > 0", blend[3])
	}
	// Just check it's in a valid range
	if blend[3] > 1 {
		t.Fatalf("composite blend alpha = %g, want <= 1", blend[3])
	}
	for i, v := range blend {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			t.Fatalf("blend[%d] = %g, want finite", i, v)
		}
	}
}

// TestCalcBlend_PerChannelPercent verifies that individual tint types can be scaled independently.
// Why: Supports fine-grained visual customization (e.g., disabling only damage flashes).
// Where in C: cl_main.c, V_CalcBlend.
func TestCalcBlend_PerChannelPercent(t *testing.T) {
	c := NewClient()
	c.DamageTaken = 50
	c.ApplyDamage()

	full := c.CalcBlend(100, [NumCShifts]float32{100, 100, 100, 100})
	// Zero out damage channel specifically.
	zeroDamage := c.CalcBlend(100, [NumCShifts]float32{100, 0, 100, 100})
	if zeroDamage[3] != 0 {
		t.Fatalf("zeroed damage channel alpha = %g, want 0", zeroDamage[3])
	}
	if full[3] <= 0 {
		t.Fatalf("full damage channel alpha = %g, want > 0", full[3])
	}
}

// TestClearState_ResetsColorShifts verifies that all screen tints are cleared when resetting client state.
// Why: Ensures a clean slate when connecting to a new server or reloading.
// Where in C: cl_main.c, CL_ClearState.
func TestClearState_ResetsColorShifts(t *testing.T) {
	c := NewClient()
	c.BonusFlash()
	c.DamageTaken = 20
	c.DamageSaved = 0
	c.ApplyDamage()
	c.ClearState()
	for i, s := range c.CShifts {
		if s.Percent != 0 || s.R != 0 || s.G != 0 || s.B != 0 {
			t.Fatalf("CShifts[%d] not zeroed after ClearState: %+v", i, s)
		}
	}
}

// TestClearState_ResetsVelocityAndViewHistory verifies that movement and angle history are cleared.
// Why: Prevents prediction or interpolation glitches when restarting state.
// Where in C: cl_main.c, CL_ClearState.
func TestClearState_ResetsVelocityAndViewHistory(t *testing.T) {
	c := NewClient()
	c.ViewAngles = [3]float32{10, 20, 30}
	c.MViewAngles = [2][3]float32{{40, 50, 60}, {70, 80, 90}}
	c.Velocity = [3]float32{100, 200, 300}
	c.MVelocity = [2][3]float32{{1, 2, 3}, {4, 5, 6}}

	c.ClearState()

	if c.ViewAngles != [3]float32{} {
		t.Fatalf("ViewAngles = %v, want zero", c.ViewAngles)
	}
	if c.MViewAngles != [2][3]float32{} {
		t.Fatalf("MViewAngles = %v, want zero", c.MViewAngles)
	}
	if c.Velocity != [3]float32{} {
		t.Fatalf("Velocity = %v, want zero", c.Velocity)
	}
	if c.MVelocity != [2][3]float32{} {
		t.Fatalf("MVelocity = %v, want zero", c.MVelocity)
	}
}
