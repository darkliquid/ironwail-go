package client

// relink.go implements per-frame entity interpolation and trail emission,
// matching C CL_RelinkEntities. RelinkEntities lerps entity positions and
// angles between double-buffered network origins, emits particle trail
// events based on model flags, and interpolates demo view angles.

import (
	"github.com/darkliquid/ironwail-go/internal/model"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

const hardResetMsgOriginDelta = 100.0

func entityNeedsHardReset(state inet.EntityState) bool {
	if state.ForceLink {
		return true
	}
	for j := 0; j < 3; j++ {
		delta := state.MsgOrigins[0][j] - state.MsgOrigins[1][j]
		if delta > hardResetMsgOriginDelta || delta < -hardResetMsgOriginDelta {
			return true
		}
	}
	return false
}

func resetEntityTrail(state *inet.EntityState) {
	if state == nil {
		return
	}
	state.TrailDelay = 1.0 / 72.0
	state.TrailOrigin = state.Origin
}

// RelinkEntities interpolates all entity positions and angles between their
// double-buffered network origins, matching C's CL_RelinkEntities behavior.
//
// It must be called once per frame after the server message has been parsed
// and before any entity collection for rendering. It modifies entity Origin
// and Angles in-place so the existing collection functions see lerped positions.
//
// Entities not updated in the last server message are dropped from the current
// render set until the server sends them again, matching C Quake's
// CL_RelinkEntities stale-entity behavior.
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

	c.LocalViewTeleport = false
	localViewEntity := c.ViewEntity

	for entNum, state := range c.Entities {
		if state.ModelIndex == 0 {
			state.LerpFlags |= inet.LerpResetMove | inet.LerpResetAnim
			state.ForceLink = false
			c.Entities[entNum] = state
			continue
		}

		// If this entity was not updated in the latest server message, drop it
		// from the live render set until a later packet reintroduces it.
		if state.MsgTime != c.MTime[0] {
			state.ModelIndex = 0
			state.LerpFlags |= inet.LerpResetMove | inet.LerpResetAnim
			state.ForceLink = false
			c.Entities[entNum] = state
			continue
		}

		teleported := state.ForceLink
		if state.ForceLink {
			// Newly tracked or teleported: jump directly to network position.
			state.Origin = state.MsgOrigins[0]
			state.Angles = state.MsgAngles[0]
		} else {
			f := frac

			// If the position delta is large, assume a teleport and don't lerp.
			if entityNeedsHardReset(state) {
				f = 1
				teleported = true
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
		if teleported {
			state.LerpFlags |= inet.LerpResetMove
		} else {
			state.LerpFlags &^= inet.LerpResetMove
		}
		if teleported || state.LerpFlags&inet.LerpResetMove != 0 {
			resetEntityTrail(&state)
		}

		// Apply EF_ROTATE: spinning bonus items
		precacheIndex := int(state.ModelIndex) - 1
		if c.ModelFlagsFunc != nil && precacheIndex >= 0 && precacheIndex < len(c.ModelPrecache) {
			modelName := c.ModelPrecache[precacheIndex]
			if modelName != "" {
				flags := c.ModelFlagsFunc(modelName)
				if flags&model.EFRotate != 0 {
					state.Angles[1] = bobjRotate
				}
			}
		}

		state.ForceLink = false
		if teleported && entNum == localViewEntity {
			c.LocalViewTeleport = true
			c.resetLocalTeleportPrediction(state.Origin)
		}

		// Emit particle trails based on model flags.
		// Matches C CL_RelinkEntities trail dispatch:
		//   if (model->flags & EF_GIB) R_RocketTrail(old, new, 2);
		//   else if (model->flags & EF_ZOMGIB) R_RocketTrail(old, new, 4);
		//   etc.
		// After trail emission, TrailOrigin is updated to the current position.
		if c.ModelFlagsFunc != nil && precacheIndex >= 0 && precacheIndex < len(c.ModelPrecache) {
			modelName := c.ModelPrecache[precacheIndex]
			if modelName != "" {
				flags := c.ModelFlagsFunc(modelName)
				trailType := -1
				switch {
				case flags&model.EFGib != 0:
					trailType = 2 // blood trail
				case flags&model.EFZomGib != 0:
					trailType = 4 // slight blood trail
				case flags&model.EFTracer != 0:
					trailType = 3 // tracer (green split)
				case flags&model.EFTracer2 != 0:
					trailType = 5 // tracer2 (orange split)
				case flags&model.EFTracer3 != 0:
					trailType = 6 // voor trail (purple)
				case flags&model.EFRocket != 0:
					trailType = 0 // rocket trail + dynamic light
				case flags&model.EFGrenade != 0:
					trailType = 1 // grenade smoke trail
				}
				if trailType >= 0 {
					state.TrailDelay -= c.Time - c.OldTime
					if state.TrailDelay <= 0 {
						c.TrailEvents = append(c.TrailEvents, TrailEvent{
							Start: state.TrailOrigin,
							End:   state.Origin,
							Type:  trailType,
						})
						resetEntityTrail(&state)
					}
				} else {
					resetEntityTrail(&state)
				}
			}
		}
		if state.TrailOrigin == [3]float32{} && state.TrailDelay == 0 {
			resetEntityTrail(&state)
		}

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
