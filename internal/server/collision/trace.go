// trace.go provides ray/box sweep tracing and collision clipping.
package collision

import (
	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

const DistEpsilon = 0.03125

type moveClip struct {
	boxMins   qtypes.Vec3
	boxMaxs   qtypes.Vec3
	mins      qtypes.Vec3
	maxs      qtypes.Vec3
	mins2     qtypes.Vec3
	maxs2     qtypes.Vec3
	start     qtypes.Vec3
	end       qtypes.Vec3
	trace     srvtypes.TraceResult
	moveType  int
	passedict *srvtypes.Edict
}

func RecursiveHullCheck(hull *model.Hull, num int, p1f, p2f float32, p1, p2 qtypes.Vec3, trace *srvtypes.TraceResult) bool {
	return recursiveHullCheck(hull, num, p1f, p2f, p1, p2, trace)
}

func recursiveHullCheck(hull *model.Hull, num int, p1f, p2f float32, p1, p2 qtypes.Vec3, trace *srvtypes.TraceResult) bool {

	if num < 0 {
		if num != bsp.ContentsSolid {
			trace.AllSolid = false
			if num == bsp.ContentsEmpty {
				trace.InOpen = true
			} else {
				trace.InWater = true
			}
		} else {
			trace.StartSolid = true
		}
		return true
	}

	if num < hull.FirstClipNode || num > hull.LastClipNode {
		return false
	}

	node := &hull.ClipNodes[num]
	plane := &hull.Planes[node.PlaneNum]
	t1 := hullPlaneDistance(plane, p1)
	t2 := hullPlaneDistance(plane, p2)

	if t1 >= 0 && t2 >= 0 {
		return recursiveHullCheck(hull, node.Children[0], p1f, p2f, p1, p2, trace)
	}
	if t1 < 0 && t2 < 0 {
		return recursiveHullCheck(hull, node.Children[1], p1f, p2f, p1, p2, trace)
	}

	var frac float32
	if t1 < 0 {
		frac = (t1 + DistEpsilon) / (t1 - t2)
	} else {
		frac = (t1 - DistEpsilon) / (t1 - t2)
	}
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}

	midf := p1f + (p2f-p1f)*frac
	mid := p1.Add(p2.Sub(p1).Scale(frac))

	side := 0
	if t1 < 0 {
		side = 1
	}

	if !recursiveHullCheck(hull, node.Children[side], p1f, midf, p1, mid, trace) {
		return false
	}

	var frac2 float32
	if t1 < 0 {
		frac2 = (t1 - DistEpsilon) / (t1 - t2)
	} else {
		frac2 = (t1 + DistEpsilon) / (t1 - t2)
	}
	if frac2 < 0 {
		frac2 = 0
	}
	if frac2 > 1 {
		frac2 = 1
	}
	mid2 := p1.Add(p2.Sub(p1).Scale(frac2))

	if hullPointContents(hull, node.Children[side^1], mid2) != bsp.ContentsSolid {
		return recursiveHullCheck(hull, node.Children[side^1], midf, p2f, mid, p2, trace)
	}

	if trace.AllSolid {
		return false
	}

	if side == 0 {
		trace.PlaneNormal = plane.Normal
		trace.PlaneDist = plane.Dist
	} else {
		trace.PlaneNormal = plane.Normal.Scale(-1)
		trace.PlaneDist = -plane.Dist
	}

	for hullPointContents(hull, hull.FirstClipNode, mid) == bsp.ContentsSolid {
		frac -= 0.1
		if frac < 0 {
			trace.Fraction = midf
			trace.EndPos = mid
			return false
		}
		midf = p1f + (p2f-p1f)*frac
		mid = p1.Add(p2.Sub(p1).Scale(frac))
	}

	trace.Fraction = midf
	trace.EndPos = mid

	return false
}

func (c *System) clipMoveToEntity(ent *srvtypes.Edict, start, mins, maxs, end qtypes.Vec3) srvtypes.TraceResult {
	trace := srvtypes.TraceResult{
		Fraction: 1,
		AllSolid: true,
		EndPos:   end,
	}

	var offset qtypes.Vec3
	hull := c.hullForEntity(ent, mins, maxs, &offset)

	startL := start.Sub(offset)
	endL := end.Sub(offset)

	recursiveHullCheck(hull, hull.FirstClipNode, 0, 1, startL, endL, &trace)

	if trace.Fraction != 1 {
		trace.EndPos = trace.EndPos.Add(offset)
	}

	if trace.Fraction < 1 || trace.StartSolid {
		trace.Entity = ent
	}

	return trace
}

func moveBounds(start, mins, maxs, end qtypes.Vec3) (boxmins, boxmaxs qtypes.Vec3) {
	if end.X > start.X {
		boxmins.X = start.X + mins.X - 1
		boxmaxs.X = end.X + maxs.X + 1
	} else {
		boxmins.X = end.X + mins.X - 1
		boxmaxs.X = start.X + maxs.X + 1
	}
	if end.Y > start.Y {
		boxmins.Y = start.Y + mins.Y - 1
		boxmaxs.Y = end.Y + maxs.Y + 1
	} else {
		boxmins.Y = end.Y + mins.Y - 1
		boxmaxs.Y = start.Y + maxs.Y + 1
	}
	if end.Z > start.Z {
		boxmins.Z = start.Z + mins.Z - 1
		boxmaxs.Z = end.Z + maxs.Z + 1
	} else {
		boxmins.Z = end.Z + mins.Z - 1
		boxmaxs.Z = start.Z + maxs.Z + 1
	}
	return
}

func (c *System) clipToLinks(node *AreaNode, clip *moveClip) {
	sh := c.sh
	for ent := node.SolidEdicts.AreaNext; ent != nil && ent != &node.SolidEdicts; ent = ent.AreaNext {
		entSolid := ent.Solid(sh)
		if entSolid == float32(srvtypes.SolidNot) {
			continue
		}
		if ent == clip.passedict {
			continue
		}
		if entSolid == float32(srvtypes.SolidTrigger) {
			continue
		}

		if clip.moveType == int(srvtypes.MoveNoMonsters) && entSolid != float32(srvtypes.SolidBSP) {
			continue
		}

		entAbsMin := ent.AbsMin(sh)
		entAbsMax := ent.AbsMax(sh)
		if clip.boxMins.X > entAbsMax.X ||
			clip.boxMins.Y > entAbsMax.Y ||
			clip.boxMins.Z > entAbsMax.Z ||
			clip.boxMaxs.X < entAbsMin.X ||
			clip.boxMaxs.Y < entAbsMin.Y ||
			clip.boxMaxs.Z < entAbsMin.Z {
			continue
		}

		if clip.passedict != nil && clip.passedict.Size(sh).X != 0 && ent.Size(sh).X == 0 {
			continue
		}

		if clip.passedict != nil {
			if ownerRef := clip.passedict.Owner(sh); ownerRef != 0 && c.store != nil {
				if owner := c.store.EdictNum(int(ownerRef)); owner == ent {
					continue
				}
			}
			if ownerRef := ent.Owner(sh); ownerRef != 0 && c.store != nil {
				if owner := c.store.EdictNum(int(ownerRef)); owner == clip.passedict {
					continue
				}
			}
		}

		if clip.trace.AllSolid {
			return
		}

		var trace srvtypes.TraceResult
		if uint32(ent.Flags(sh))&srvtypes.FlagMonster != 0 {
			trace = c.clipMoveToEntity(ent, clip.start, clip.mins2, clip.maxs2, clip.end)
		} else {
			trace = c.clipMoveToEntity(ent, clip.start, clip.mins, clip.maxs, clip.end)
		}

		if trace.AllSolid || trace.StartSolid || trace.Fraction < clip.trace.Fraction {
			trace.Entity = ent
			if clip.trace.StartSolid {
				clip.trace = trace
				clip.trace.StartSolid = true
			} else {
				clip.trace = trace
			}
		} else if trace.StartSolid {
			clip.trace.StartSolid = true
		}
	}

	if node.Axis == -1 {
		return
	}

	var minVal, maxVal float32
	if node.Axis == 0 {
		minVal, maxVal = clip.boxMins.X, clip.boxMaxs.X
	} else {
		minVal, maxVal = clip.boxMins.Y, clip.boxMaxs.Y
	}

	if maxVal > node.Dist && node.Children[0] != nil {
		c.clipToLinks(node.Children[0], clip)
	}
	if minVal < node.Dist && node.Children[1] != nil {
		c.clipToLinks(node.Children[1], clip)
	}
}

// Move traces a move from start to end with the given bounding box.
func (c *System) Move(start, mins, maxs, end qtypes.Vec3, moveType srvtypes.MoveType, passedict *srvtypes.Edict) srvtypes.TraceResult {
	var clip moveClip

	if c.store != nil && c.store.EdictNum(0) != nil {
		clip.trace = c.clipMoveToEntity(c.store.EdictNum(0), start, mins, maxs, end)
	}

	clip.start = start
	clip.end = end
	clip.mins = mins
	clip.maxs = maxs
	clip.moveType = int(moveType)
	clip.passedict = passedict

	if moveType == srvtypes.MoveMissile {
		clip.mins2 = qtypes.Vec3{X: -15, Y: -15, Z: -15}
		clip.maxs2 = qtypes.Vec3{X: 15, Y: 15, Z: 15}
	} else {
		clip.mins2 = mins
		clip.maxs2 = maxs
	}

	clip.boxMins, clip.boxMaxs = moveBounds(start, clip.mins2, clip.maxs2, end)

	if len(c.areanodes) > 0 {
		c.clipToLinks(&c.areanodes[0], &clip)
	}

	return clip.trace
}

// TestEntityPosition tests if an entity is stuck in solid.
func (c *System) TestEntityPosition(ent *srvtypes.Edict) *srvtypes.Edict {
	sh := c.sh
	origin := ent.Origin(sh)
	trace := c.Move(origin, ent.Mins(sh), ent.Maxs(sh), origin, srvtypes.MoveNormal, ent)

	if trace.StartSolid {
		if trace.Entity != nil {
			return trace.Entity
		}
		if c.store != nil {
			return c.store.EdictNum(0)
		}
		return nil
	}

	return nil
}

// PointContents returns the contents at a point in the world.
func (c *System) PointContents(p qtypes.Vec3) int {
	if c.world == nil || isNilCollisionModel(c.world.GetWorldModel()) {
		return bsp.ContentsSolid
	}

	m := c.world.GetWorldModel()
	if m == nil || m.NumHulls() == 0 {
		return bsp.ContentsSolid
	}

	hull := m.Hull(0)
	cont := hullPointContents(&hull, 0, p)

	if cont <= bsp.ContentsCurrent0 && cont >= bsp.ContentsCurrentDown {
		cont = bsp.ContentsWater
	}

	return cont
}
