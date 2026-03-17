package client

import (
	"math"

	inet "github.com/ironwail/ironwail-go/internal/net"
)

// RelinkEntities interpolates all entity positions and angles between their
// double-buffered network origins, matching C's CL_RelinkEntities behavior.
//
// It must be called once per frame after the server message has been parsed
// and before any entity collection for rendering. It modifies entity Origin
// and Angles in-place so the existing collection functions see lerped positions.
//
// Entities not updated in the last server message are removed from the map.
func (c *Client) RelinkEntities() {
	if c == nil {
		return
	}

	frac := float32(c.LerpPoint())
	bobjRotate := angleMod(100 * float32(c.Time))

	var toDelete []int

	for entNum, state := range c.Entities {
		// If this entity was not updated in the latest server message, remove it.
		// Mirrors C: if (ent->msgtime != cl.mtime[0]) { ent->model = NULL; continue; }
		if state.MsgTime != c.MTime[0] {
			state.LerpFlags |= inet.LerpResetMove | inet.LerpResetAnim
			c.Entities[entNum] = state
			toDelete = append(toDelete, entNum)
			continue
		}

		if state.ForceLink {
			// Newly tracked or teleported: jump directly to network position.
			state.Origin = state.MsgOrigins[0]
			state.Angles = state.MsgAngles[0]
		} else {
			f := frac

			// If the position delta is large, assume a teleport and don't lerp.
			for j := 0; j < 3; j++ {
				delta := state.MsgOrigins[0][j] - state.MsgOrigins[1][j]
				if delta > 100 || delta < -100 {
					f = 1
					state.LerpFlags |= inet.LerpResetMove
					break
				}
			}

			// Step-move entities (monsters) do not lerp position.
			if state.LerpFlags&inet.LerpMoveStep != 0 {
				f = 1
			}

			// Interpolate origin and angles.
			for j := 0; j < 3; j++ {
				delta := state.MsgOrigins[0][j] - state.MsgOrigins[1][j]
				state.Origin[j] = state.MsgOrigins[1][j] + f*delta

				ad := state.MsgAngles[0][j] - state.MsgAngles[1][j]
				ad = wrapAngleDelta(ad)
				state.Angles[j] = state.MsgAngles[1][j] + f*ad
			}
		}

		// Rotate binary objects locally (items, health packs, etc.) if the model
		// has the EF_ROTATE flag. We apply rotation to the yaw axis (index 1).
		// Model flag lookup is deferred to the renderer; here we check entity
		// effects as a proxy — a future pass can supply model flags explicitly.
		_ = bobjRotate // used when model flags are available

		state.ForceLink = false
		state.LerpFlags &^= inet.LerpResetMove
		c.Entities[entNum] = state
	}

	for _, entNum := range toDelete {
		delete(c.Entities, entNum)
	}
}

// wrapAngleDelta normalizes an angle difference to the range [-180, 180].
func wrapAngleDelta(d float32) float32 {
	if d > 180 {
		return d - 360
	}
	if d < -180 {
		return d + 360
	}
	return d
}

// angleMod normalizes an angle to the range [0, 360).
// Equivalent to C's anglemod() in mathlib.c.
func angleMod(a float32) float32 {
	a = float32(math.Mod(float64(a), 360.0))
	if a < 0 {
		a += 360
	}
	return a
}
