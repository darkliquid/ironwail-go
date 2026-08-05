// trace.go provides ray/box sweep tracing and collision clipping.
package collision

import (
	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

const DistEpsilon = 0.03125

type moveClip struct {
	boxMins   [3]float32
	boxMaxs   [3]float32
	mins      [3]float32
	maxs      [3]float32
	mins2     [3]float32
	maxs2     [3]float32
	start     [3]float32
	end       [3]float32
	trace     srvtypes.TraceResult
	moveType  int
	passedict *srvtypes.Edict
}

func RecursiveHullCheck(hull *model.Hull, num int, p1f, p2f float32, p1, p2 [3]float32, trace *srvtypes.TraceResult) bool {
	return recursiveHullCheck(hull, num, p1f, p2f, p1, p2, trace)
}

func recursiveHullCheck(hull *model.Hull, num int, p1f, p2f float32, p1, p2 [3]float32, trace *srvtypes.TraceResult) bool {

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

	var frac, frac2 float32
	if t1 < 0 {
		frac = (t1 + DistEpsilon) / (t1 - t2)
		frac2 = (t1 - DistEpsilon) / (t1 - t2)
	} else if t1 > 0 {
		frac = (t1 - DistEpsilon) / (t1 - t2)
		frac2 = (t1 + DistEpsilon) / (t1 - t2)
	} else {
		frac = 0
		frac2 = 0
	}
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	if frac2 < 0 {
		frac2 = 0
	}
	if frac2 > 1 {
		frac2 = 1
	}

	midf := p1f + (p2f-p1f)*frac
	var mid [3]float32
	for i := 0; i < 3; i++ {
		mid[i] = p1[i] + frac*(p2[i]-p1[i])
	}
	midf2 := p1f + (p2f-p1f)*frac2
	var mid2 [3]float32
	for i := 0; i < 3; i++ {
		mid2[i] = p1[i] + frac2*(p2[i]-p1[i])
	}

	side := 0
	if t1 < 0 {
		side = 1
	}

	if !recursiveHullCheck(hull, node.Children[side], p1f, midf, p1, mid, trace) {
		return false
	}

	if hullPointContents(hull, node.Children[side^1], mid2) != bsp.ContentsSolid {
		return recursiveHullCheck(hull, node.Children[side^1], midf2, p2f, mid2, p2, trace)
	}

	if trace.AllSolid {
		return false
	}

	if side == 0 {
		trace.PlaneNormal = plane.Normal
		trace.PlaneDist = plane.Dist
	} else {
		trace.PlaneNormal = [3]float32{-plane.Normal[0], -plane.Normal[1], -plane.Normal[2]}
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
		for i := 0; i < 3; i++ {
			mid[i] = p1[i] + frac*(p2[i]-p1[i])
		}
	}

	trace.Fraction = midf
	trace.EndPos = mid

	return false
}

func (c *System) clipMoveToEntity(ent *srvtypes.Edict, start, mins, maxs, end [3]float32) srvtypes.TraceResult {
	trace := srvtypes.TraceResult{
		Fraction: 1,
		AllSolid: true,
		EndPos:   end,
	}

	var offset [3]float32
	hull := c.hullForEntity(ent, mins, maxs, &offset)

	startL := [3]float32{
		start[0] - offset[0],
		start[1] - offset[1],
		start[2] - offset[2],
	}
	endL := [3]float32{
		end[0] - offset[0],
		end[1] - offset[1],
		end[2] - offset[2],
	}

	recursiveHullCheck(hull, hull.FirstClipNode, 0, 1, startL, endL, &trace)

	if trace.Fraction != 1 {
		trace.EndPos[0] += offset[0]
		trace.EndPos[1] += offset[1]
		trace.EndPos[2] += offset[2]
	}

	if trace.Fraction < 1 || trace.StartSolid {
		trace.Entity = ent
	}

	return trace
}

func moveBounds(start, mins, maxs, end [3]float32) (boxmins, boxmaxs [3]float32) {
	for i := 0; i < 3; i++ {
		if end[i] > start[i] {
			boxmins[i] = start[i] + mins[i] - 1
			boxmaxs[i] = end[i] + maxs[i] + 1
		} else {
			boxmins[i] = end[i] + mins[i] - 1
			boxmaxs[i] = start[i] + maxs[i] + 1
		}
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
		if clip.boxMins[0] > entAbsMax[0] ||
			clip.boxMins[1] > entAbsMax[1] ||
			clip.boxMins[2] > entAbsMax[2] ||
			clip.boxMaxs[0] < entAbsMin[0] ||
			clip.boxMaxs[1] < entAbsMin[1] ||
			clip.boxMaxs[2] < entAbsMin[2] {
			continue
		}

		if clip.passedict != nil && clip.passedict.Size(sh)[0] != 0 && ent.Size(sh)[0] == 0 {
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

	if clip.boxMaxs[node.Axis] > node.Dist && node.Children[0] != nil {
		c.clipToLinks(node.Children[0], clip)
	}
	if clip.boxMins[node.Axis] < node.Dist && node.Children[1] != nil {
		c.clipToLinks(node.Children[1], clip)
	}
}

// Move traces a move from start to end with the given bounding box.
func (c *System) Move(start, mins, maxs, end [3]float32, moveType srvtypes.MoveType, passedict *srvtypes.Edict) srvtypes.TraceResult {
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
		for i := 0; i < 3; i++ {
			clip.mins2[i] = -15
			clip.maxs2[i] = 15
		}
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
func (c *System) PointContents(p [3]float32) int {
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
