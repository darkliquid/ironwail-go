package client

import (
	"github.com/ironwail/ironwail-go/internal/model"
	inet "github.com/ironwail/ironwail-go/internal/net"
	"github.com/ironwail/ironwail-go/pkg/types"
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
	bobjRotate := types.AngleMod(100 * float32(c.Time))

	// During demo playback, interpolate view angles between double-buffered
	// MViewAngles frames. Matches C CL_RelinkEntities:
	//   if (cls.demoplayback) { for j: d = mviewangles[0]-[1]; wrap; viewangles = [1]+frac*d; }
	if c.DemoPlayback {
		for j := 0; j < 3; j++ {
			d := c.MViewAngles[0][j] - c.MViewAngles[1][j]
			d = wrapAngleDelta(d)
			c.ViewAngles[j] = c.MViewAngles[1][j] + frac*d
		}
	}

	for entNum, state := range c.Entities {
		// If this entity was not updated in the latest server message, skip it.
		// Mirrors C: if (ent->msgtime != cl.mtime[0]) { ent->model = NULL; continue; }
		// C keeps the entity slot alive in a fixed array; we keep it in the map
		// with ModelIndex intact so the next entity update's delta decoding starts
		// from a valid state. The renderer already skips ModelIndex==0 entities,
		// and stale entities won't be collected because they aren't re-linked.
		if state.MsgTime != c.MTime[0] {
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

		// Apply EF_ROTATE: spinning bonus items
		if c.ModelFlagsFunc != nil && int(state.ModelIndex) < len(c.ModelPrecache) {
			modelName := c.ModelPrecache[int(state.ModelIndex)]
			if modelName != "" {
				flags := c.ModelFlagsFunc(modelName)
				if flags&model.EFRotate != 0 {
					state.Angles[1] = bobjRotate
				}
			}
		}

		state.ForceLink = false
		state.LerpFlags &^= inet.LerpResetMove
		c.Entities[entNum] = state
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
