package client

import "math"

// Color shift channel indices – mirrors C Quake view.c CSHIFT_* constants.
const (
	CShiftContents = 0 // liquid/environment tint (water/lava/slime)
	CShiftDamage   = 1 // red/armor flash when taking damage
	CShiftBonus    = 2 // gold flash when picking up items (from server-stuffed "bf")
	CShiftPowerup  = 3 // blue/green/etc while holding a powerup
	numCShifts     = 4
)

// ColorShift mirrors C Quake cshift_t: a destination color (0–255 per channel)
// and a blend percentage (0–255).
type ColorShift struct {
	R, G, B float32 // destination color 0–255
	Percent float32 // blend amount 0–255
}

// SetContentsColor sets the contents color shift based on the BSP leaf type.
// Mirrors C view.c:V_SetContentsColor().
//
// Contents constants are defined in bsp.go (ContentsWater = -3, etc.); we
// accept int32 here to avoid a package-import cycle.
//
//   - ContentsLava  (-5): orange-red tint, percent 150
//   - ContentsSlime (-4): dark green tint, percent 150
//   - ContentsWater (-3) and any other non-empty/solid/sky: brown-water tint, percent 128
//   - ContentsEmpty (-1), ContentsSolid (-2), ContentsSky (-6): no tint
func (c *Client) SetContentsColor(contents int32) {
	const (
		contentsEmpty = -1
		contentsSolid = -2
		contentsWater = -3
		contentsSlime = -4
		contentsLava  = -5
		contentsSky   = -6
	)
	switch contents {
	case contentsEmpty, contentsSolid, contentsSky:
		c.CShifts[CShiftContents] = ColorShift{R: 130, G: 80, B: 50, Percent: 0}
	case contentsLava:
		c.CShifts[CShiftContents] = ColorShift{R: 255, G: 80, B: 0, Percent: 150}
	case contentsSlime:
		c.CShifts[CShiftContents] = ColorShift{R: 0, G: 25, B: 5, Percent: 150}
	default: // ContentsWater and any other liquid
		c.CShifts[CShiftContents] = ColorShift{R: 130, G: 80, B: 50, Percent: 128}
	}
}

// ApplyDamage updates the damage color shift based on the most recently parsed
// damage event (DamageTaken + DamageSaved).
// Mirrors C view.c:V_ParseDamage() cshift update logic.
//
// This must be called after parseDamage() has stored the new values, and before
// DamageTaken/DamageSaved are cleared.
func (c *Client) ApplyDamage() {
	blood := float32(c.DamageTaken)
	armor := float32(c.DamageSaved)
	count := blood*0.5 + armor*0.5
	if count < 10 {
		count = 10
	}

	c.CShifts[CShiftDamage].Percent += 3 * count
	if c.CShifts[CShiftDamage].Percent < 0 {
		c.CShifts[CShiftDamage].Percent = 0
	}
	if c.CShifts[CShiftDamage].Percent > 150 {
		c.CShifts[CShiftDamage].Percent = 150
	}

	if armor > blood {
		c.CShifts[CShiftDamage].R = 200
		c.CShifts[CShiftDamage].G = 100
		c.CShifts[CShiftDamage].B = 100
	} else if armor > 0 {
		c.CShifts[CShiftDamage].R = 220
		c.CShifts[CShiftDamage].G = 50
		c.CShifts[CShiftDamage].B = 50
	} else {
		c.CShifts[CShiftDamage].R = 255
		c.CShifts[CShiftDamage].G = 0
		c.CShifts[CShiftDamage].B = 0
	}
}

// BonusFlash triggers the gold item-pickup flash.
// Mirrors C view.c:V_BonusFlash_f().
func (c *Client) BonusFlash() {
	c.CShifts[CShiftBonus] = ColorShift{R: 215, G: 186, B: 69, Percent: 50}
}

// CalculateDamageKick computes damage-induced camera kick angles from the most
// recently parsed damage event.  Mirrors C Ironwail V_ParseDamage damage kick
// calculation (view.c:329-345).
//
// This must be called after parseDamage() has stored DamageTaken/DamageSaved/
// DamageOrigin, typically right after ApplyDamage().
//
// Parameters:
//   - entityOrigin: player entity origin (used to compute damage direction)
//   - entityAngles: player entity angles (used to compute right/forward vectors)
//   - kickTime:     v_kicktime cvar value (duration of kick effect)
//   - kickRoll:     v_kickroll cvar value (roll intensity multiplier)
//   - kickPitch:    v_kickpitch cvar value (pitch intensity multiplier)
func (c *Client) CalculateDamageKick(entityOrigin, entityAngles [3]float32, kickTime, kickRoll, kickPitch float32) {
	blood := float32(c.DamageTaken)
	armor := float32(c.DamageSaved)
	count := blood*0.5 + armor*0.5
	if count < 10 {
		count = 10
	}

	// Compute damage direction: from DamageOrigin to entity.
	from := [3]float32{
		c.DamageOrigin[0] - entityOrigin[0],
		c.DamageOrigin[1] - entityOrigin[1],
		c.DamageOrigin[2] - entityOrigin[2],
	}
	// Normalize.
	length := float32(math.Sqrt(float64(from[0]*from[0] + from[1]*from[1] + from[2]*from[2])))
	if length > 0 {
		from[0] /= length
		from[1] /= length
		from[2] /= length
	}

	// Compute right and forward vectors from entity angles.
	// Quake angles: [pitch, yaw, roll] in degrees.
	yawRad := float64(entityAngles[1]) * math.Pi / 180.0
	pitchRad := float64(entityAngles[0]) * math.Pi / 180.0

	// Forward vector.
	forward := [3]float32{
		float32(math.Cos(yawRad) * math.Cos(pitchRad)),
		float32(math.Sin(yawRad) * math.Cos(pitchRad)),
		float32(-math.Sin(pitchRad)),
	}
	// Right vector (perpendicular to forward, in XY plane).
	right := [3]float32{
		float32(math.Sin(yawRad)),
		float32(-math.Cos(yawRad)),
		0,
	}

	// Roll kick: lateral component of damage direction.
	sideRoll := from[0]*right[0] + from[1]*right[1] + from[2]*right[2]
	c.DamageKickRoll = count * sideRoll * kickRoll

	// Pitch kick: forward/back component of damage direction.
	sidePitch := from[0]*forward[0] + from[1]*forward[1] + from[2]*forward[2]
	c.DamageKickPitch = count * sidePitch * kickPitch

	c.DamageKickTime = kickTime
}

// SetCustomShift overrides the contents (empty) shift with an arbitrary color,
// used by the Quake console command "v_cshift r g b percent".
// Mirrors C view.c:V_cshift_f().
func (c *Client) SetCustomShift(r, g, b, percent float32) {
	c.CShifts[CShiftContents] = ColorShift{R: r, G: g, B: b, Percent: percent}
}

// calcPowerupCshift updates the powerup color shift based on held items.
// Mirrors C view.c:V_CalcPowerupCshift().
func (c *Client) calcPowerupCshift() {
	switch {
	case c.Items&uint32(ItemQuad) != 0:
		c.CShifts[CShiftPowerup] = ColorShift{R: 0, G: 0, B: 255, Percent: 30}
	case c.Items&uint32(ItemSuit) != 0:
		c.CShifts[CShiftPowerup] = ColorShift{R: 0, G: 255, B: 0, Percent: 20}
	case c.Items&uint32(ItemInvisibility) != 0:
		c.CShifts[CShiftPowerup] = ColorShift{R: 100, G: 100, B: 100, Percent: 100}
	case c.Items&uint32(ItemInvulnerability) != 0:
		c.CShifts[CShiftPowerup] = ColorShift{R: 255, G: 255, B: 0, Percent: 30}
	default:
		c.CShifts[CShiftPowerup].Percent = 0
	}
}

// UpdateBlend decays time-based color shifts and recomputes the powerup shift.
// Must be called once per frame with the current frame duration.
// Mirrors C view.c:V_UpdateBlend() (minus the redundant palette-based path).
func (c *Client) UpdateBlend(frametime float64) {
	c.calcPowerupCshift()

	// Damage flash decays at 150 percent/sec.
	c.CShifts[CShiftDamage].Percent -= float32(frametime) * 150
	if c.CShifts[CShiftDamage].Percent < 0 {
		c.CShifts[CShiftDamage].Percent = 0
	}

	// Bonus flash decays at 100 percent/sec.
	c.CShifts[CShiftBonus].Percent -= float32(frametime) * 100
	if c.CShifts[CShiftBonus].Percent < 0 {
		c.CShifts[CShiftBonus].Percent = 0
	}
}

// CalcBlend computes the composite RGBA screen-tint from all active color shifts.
// globalPercent is the value of the gl_cshiftpercent cvar (0–100).
// Returns [r, g, b, a] in the 0–1 float range suitable for GL blending.
//
// During intermission only the contents shift applies (matches C behavior).
//
// Mirrors C view.c:V_CalcBlend().
func (c *Client) CalcBlend(globalPercent float32) [4]float32 {
	if globalPercent <= 0 {
		return [4]float32{}
	}
	var r, g, b, a float32
	for i := 0; i < numCShifts; i++ {
		// During intermission only CSHIFT_CONTENTS applies.
		if c.Intermission != 0 && i != CShiftContents {
			continue
		}
		percent := c.CShifts[i].Percent
		if percent <= 0 {
			continue
		}
		a2 := (percent * globalPercent / 100.0) / 255.0
		if a2 <= 0 {
			continue
		}
		a = a + a2*(1-a)
		if a <= 0 {
			continue
		}
		mix := a2 / a
		r = r*(1-mix) + c.CShifts[i].R*mix
		g = g*(1-mix) + c.CShifts[i].G*mix
		b = b*(1-mix) + c.CShifts[i].B*mix
	}
	if a > 1 {
		a = 1
	}
	if a < 0 {
		a = 0
	}
	return [4]float32{r / 255.0, g / 255.0, b / 255.0, a}
}
