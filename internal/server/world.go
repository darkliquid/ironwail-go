// Package server implements the Quake server physics and game logic.
//
// world.go provides world collision detection and spatial queries.
// It implements:
//   - Hull-based collision against BSP geometry
//   - Entity-to-entity collision clipping
//   - Area grid for efficient spatial queries
//   - Point contents testing
package server

import (
	"log/slog"
	"math"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/model"
)

// Move types for SV_Move
const (
	MoveNormal     = 0 // Normal move
	MoveNoMonsters = 1 // Ignore monsters (only BSP collision)
	MoveMissile    = 2 // Missile move (uses larger bbox for clipping)
	MoveWaterJump  = 3 // Water jump move
)

// Collision constants
const (
	DistEpsilon = 0.03125 // Distance epsilon for hull checks

	// Area grid depth (affects spatial partitioning granularity)
	AreaDepth = 4
	AreaNodes = 1 << AreaDepth
)

// moveClip holds state during a move/clipping operation.
type moveClip struct {
	boxMins   [3]float32 // Enclose the test object along entire move
	boxMaxs   [3]float32
	mins      [3]float32 // Size of the moving object
	maxs      [3]float32
	mins2     [3]float32 // Size when clipping against monsters
	maxs2     [3]float32
	start     [3]float32
	end       [3]float32
	trace     TraceResult
	moveType  int
	passedict *Edict
}

// Box hull state - used for entity bounding box collision
var (
	boxHull       model.Hull
	boxClipNodes  [6]model.MClipNode
	boxPlanes     [6]model.MPlane
	boxHullInited bool
)

// initBoxHull sets up the planes and clipnodes so that the six floats
// of a bounding box can just be stored out and get a proper hull structure.
func initBoxHull() {
	if boxHullInited {
		return
	}

	boxHull.ClipNodes = boxClipNodes[:]
	boxHull.Planes = boxPlanes[:]
	boxHull.FirstClipNode = 0
	boxHull.LastClipNode = 5

	for i := 0; i < 6; i++ {
		boxClipNodes[i].PlaneNum = i
		side := i & 1
		boxClipNodes[i].Children[side] = bsp.ContentsEmpty
		if i != 5 {
			boxClipNodes[i].Children[side^1] = i + 1
		} else {
			boxClipNodes[i].Children[side^1] = bsp.ContentsSolid
		}
		boxPlanes[i].Type = uint8(i >> 1)
		boxPlanes[i].Normal[i>>1] = 1
	}

	boxHullInited = true
}

// hullForBox creates a temporary hull from bounding box sizes.
// To keep everything uniform, bounding boxes are turned into small
// BSP trees instead of being compared directly.
func hullForBox(mins, maxs [3]float32) *model.Hull {
	initBoxHull()

	boxPlanes[0].Dist = maxs[0]
	boxPlanes[1].Dist = mins[0]
	boxPlanes[2].Dist = maxs[1]
	boxPlanes[3].Dist = mins[1]
	boxPlanes[4].Dist = maxs[2]
	boxPlanes[5].Dist = mins[2]

	return &boxHull
}

// hullForEntity returns a hull that can be used for testing or clipping
// an object of mins/maxs size.
// The offset is filled in to contain the adjustment that must be added
// to the testing object's origin to get a point to use with the returned hull.
func (s *Server) hullForEntity(ent *Edict, mins, maxs [3]float32, offset *[3]float32) *model.Hull {
	// Decide which clipping hull to use, based on the size
	if int(ent.Vars.Solid) == int(SolidBSP) {
		// Explicit hulls in the BSP model.
		size := [3]float32{
			maxs[0] - mins[0],
			maxs[1] - mins[1],
			maxs[2] - mins[2],
		}

		hullNum := 0
		if size[0] >= 3 {
			if size[0] <= 32 {
				hullNum = 1
			} else {
				hullNum = 2
			}
		}

		if s.WorldModel != nil {
			if m := s.WorldModel; m != nil {
				var hull model.Hull
				if hullNum >= 0 && hullNum < m.NumHulls() {
					hull = m.Hull(hullNum)
				}
				modelIndex := int(ent.Vars.ModelIndex)
				if ent == s.Edicts[0] || modelIndex <= 1 {
					modelIndex = 1
				}
				if modelIndex > 1 && s.WorldTree != nil && modelIndex-1 < len(s.WorldTree.Models) {
					headNode := int(s.WorldTree.Models[modelIndex-1].HeadNode[hullNum])
					if headNode >= 0 {
						hull.FirstClipNode = headNode
					}
				}
				if len(hull.ClipNodes) > 0 && hull.FirstClipNode >= 0 {
					offset[0] = hull.ClipMins[0] - mins[0] + ent.Vars.Origin[0]
					offset[1] = hull.ClipMins[1] - mins[1] + ent.Vars.Origin[1]
					offset[2] = hull.ClipMins[2] - mins[2] + ent.Vars.Origin[2]
					return &hull
				}
			}
		}

		// Fallback to box hull
		hullMins := [3]float32{
			ent.Vars.Mins[0] - maxs[0],
			ent.Vars.Mins[1] - maxs[1],
			ent.Vars.Mins[2] - maxs[2],
		}
		hullMaxs := [3]float32{
			ent.Vars.Maxs[0] - mins[0],
			ent.Vars.Maxs[1] - mins[1],
			ent.Vars.Maxs[2] - mins[2],
		}
		offset[0] = ent.Vars.Origin[0]
		offset[1] = ent.Vars.Origin[1]
		offset[2] = ent.Vars.Origin[2]
		return hullForBox(hullMins, hullMaxs)
	}

	// Create a temp hull from bounding box sizes
	hullMins := [3]float32{
		ent.Vars.Mins[0] - maxs[0],
		ent.Vars.Mins[1] - maxs[1],
		ent.Vars.Mins[2] - maxs[2],
	}
	hullMaxs := [3]float32{
		ent.Vars.Maxs[0] - mins[0],
		ent.Vars.Maxs[1] - mins[1],
		ent.Vars.Maxs[2] - mins[2],
	}
	hull := hullForBox(hullMins, hullMaxs)

	offset[0] = ent.Vars.Origin[0]
	offset[1] = ent.Vars.Origin[1]
	offset[2] = ent.Vars.Origin[2]
	return hull
}

// ============================================================================
// POINT TESTING IN HULLS
// ============================================================================

// hullPointContents returns the contents at a point within a hull.
// This recursively traverses the BSP tree to find which leaf the point is in.
func hullPointContents(hull *model.Hull, num int, p [3]float32) int {
	// Handle leaf nodes (negative numbers)
	if num < 0 {
		return num
	}

	// Safety check
	if num < hull.FirstClipNode || num > hull.LastClipNode {
		return bsp.ContentsSolid
	}

	for num >= 0 {
		if num < hull.FirstClipNode || num > hull.LastClipNode {
			return bsp.ContentsSolid
		}

		node := &hull.ClipNodes[num]
		plane := &hull.Planes[node.PlaneNum]

		var d float32
		if plane.Type < 3 {
			d = p[plane.Type] - plane.Dist
		} else {
			d = plane.Normal[0]*p[0] + plane.Normal[1]*p[1] + plane.Normal[2]*p[2] - plane.Dist
		}

		if d < 0 {
			num = node.Children[1]
		} else {
			num = node.Children[0]
		}
	}

	return num
}

// PointContents returns the contents at a point in the world.
// This is the main entry point for checking what's at a location.
func (s *Server) PointContents(p [3]float32) int {
	// Get world model's hull 0 (point hull)
	if s.WorldModel == nil {
		return bsp.ContentsSolid
	}

	m := s.WorldModel
	if m == nil || m.NumHulls() == 0 {
		return bsp.ContentsSolid
	}

	hull := m.Hull(0)
	cont := hullPointContents(&hull, 0, p)

	// Current contents are simplified to water
	if cont <= bsp.ContentsCurrent0 && cont >= bsp.ContentsCurrentDown {
		cont = bsp.ContentsWater
	}

	return cont
}

// ============================================================================
// LINE TESTING IN HULLS (Recursive Hull Check)
// ============================================================================

// recursiveHullCheck traces a line through a BSP hull.
// This is the core collision detection function.
func recursiveHullCheck(hull *model.Hull, num int, p1f, p2f float32, p1, p2 [3]float32, trace *TraceResult) bool {
	// Check for empty (leaf node)
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
		return true // empty
	}

	// Safety check
	if num < hull.FirstClipNode || num > hull.LastClipNode {
		return false
	}

	// Find the point distances
	node := &hull.ClipNodes[num]
	plane := &hull.Planes[node.PlaneNum]

	var t1, t2 float32
	if plane.Type < 3 {
		t1 = p1[plane.Type] - plane.Dist
		t2 = p2[plane.Type] - plane.Dist
	} else {
		// Use double precision for non-axial planes to avoid clipping errors
		// on rotated brushes. Matches C DoublePrecisionDotProduct().
		t1 = float32(float64(plane.Normal[0])*float64(p1[0]) + float64(plane.Normal[1])*float64(p1[1]) + float64(plane.Normal[2])*float64(p1[2]) - float64(plane.Dist))
		t2 = float32(float64(plane.Normal[0])*float64(p2[0]) + float64(plane.Normal[1])*float64(p2[1]) + float64(plane.Normal[2])*float64(p2[2]) - float64(plane.Dist))
	}

	// Both points on same side - recurse down that side
	if t1 >= 0 && t2 >= 0 {
		return recursiveHullCheck(hull, node.Children[0], p1f, p2f, p1, p2, trace)
	}
	if t1 < 0 && t2 < 0 {
		return recursiveHullCheck(hull, node.Children[1], p1f, p2f, p1, p2, trace)
	}

	// Put the crosspoint DIST_EPSILON pixels on the near side
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
	var mid [3]float32
	for i := 0; i < 3; i++ {
		mid[i] = p1[i] + frac*(p2[i]-p1[i])
	}

	side := 0
	if t1 < 0 {
		side = 1
	}

	// Move up to the node
	if !recursiveHullCheck(hull, node.Children[side], p1f, midf, p1, mid, trace) {
		return false
	}

	// Go past the node if the other side isn't solid
	if hullPointContents(hull, node.Children[side^1], mid) != bsp.ContentsSolid {
		return recursiveHullCheck(hull, node.Children[side^1], midf, p2f, mid, p2, trace)
	}

	if trace.AllSolid {
		return false // never got out of the solid area
	}

	// The other side of the node is solid, this is the impact point
	if side == 0 {
		trace.PlaneNormal = plane.Normal
		// trace.plane.dist = plane.dist
	} else {
		trace.PlaneNormal = [3]float32{-plane.Normal[0], -plane.Normal[1], -plane.Normal[2]}
		// trace.plane.dist = -plane.dist
	}

	// Back up until we're outside the solid
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

// clipMoveToEntity handles selection of a clipping hull, and offsetting
// of the end points for tracing against a single entity.
func (s *Server) clipMoveToEntity(ent *Edict, start, mins, maxs, end [3]float32) TraceResult {
	// Fill in a default trace
	trace := TraceResult{
		Fraction: 1,
		AllSolid: true,
		EndPos:   end,
	}

	// Get the clipping hull
	var offset [3]float32
	hull := s.hullForEntity(ent, mins, maxs, &offset)

	// Offset start and end points
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

	// Trace a line through the appropriate clipping hull
	recursiveHullCheck(hull, hull.FirstClipNode, 0, 1, startL, endL, &trace)

	// Fix trace up by the offset
	if trace.Fraction != 1 {
		trace.EndPos[0] += offset[0]
		trace.EndPos[1] += offset[1]
		trace.EndPos[2] += offset[2]
	}

	// Did we clip the move?
	if trace.Fraction < 1 || trace.StartSolid {
		trace.Entity = ent
	}

	return trace
}

// ============================================================================
// ENTITY AREA CHECKING
// ============================================================================

// createAreaNode creates an area node for spatial partitioning.
func (s *Server) createAreaNode(depth int, mins, maxs [3]float32) *AreaNode {
	if len(s.Areanodes) <= s.numAreaNodes {
		// Allocate more nodes if needed
		return nil
	}

	node := &s.Areanodes[s.numAreaNodes]
	s.numAreaNodes++

	// Initialize sentinel nodes for doubly-linked lists (matching C's ClearLink)
	node.TriggerEdicts.AreaNext = &node.TriggerEdicts
	node.TriggerEdicts.AreaPrev = &node.TriggerEdicts
	node.SolidEdicts.AreaNext = &node.SolidEdicts
	node.SolidEdicts.AreaPrev = &node.SolidEdicts

	if depth == AreaDepth {
		node.Axis = -1
		node.Children[0] = nil
		node.Children[1] = nil
		return node
	}

	size := [3]float32{
		maxs[0] - mins[0],
		maxs[1] - mins[1],
		maxs[2] - mins[2],
	}

	if size[0] > size[1] {
		node.Axis = 0
	} else {
		node.Axis = 1
	}

	node.Dist = 0.5 * (maxs[node.Axis] + mins[node.Axis])

	mins1 := mins
	maxs1 := maxs
	mins2 := mins
	maxs2 := maxs

	maxs1[node.Axis] = node.Dist
	mins2[node.Axis] = node.Dist

	node.Children[0] = s.createAreaNode(depth+1, mins2, maxs2)
	node.Children[1] = s.createAreaNode(depth+1, mins1, maxs1)

	return node
}

// ClearWorld initializes the area nodes for a new map.
func (s *Server) ClearWorld() {
	initBoxHull()

	if len(s.Areanodes) != AreaNodes {
		s.Areanodes = make([]AreaNode, AreaNodes)
	}

	// Reset area nodes
	s.numAreaNodes = 0

	// Get world bounds
	var mins, maxs [3]float32
	if s.WorldModel != nil {
		mins = s.WorldModel.CollisionClipMins()
		maxs = s.WorldModel.CollisionClipMaxs()
	}

	// Create area node tree
	s.createAreaNode(0, mins, maxs)
}

// UnlinkEdict removes an entity from the area grid.
func UnlinkEdict(ent *Edict) {
	if ent.AreaPrev == nil {
		return // not linked in anywhere
	}

	// Remove from linked list
	if ent.AreaPrev != nil {
		ent.AreaPrev.AreaNext = ent.AreaNext
	}
	if ent.AreaNext != nil {
		ent.AreaNext.AreaPrev = ent.AreaPrev
	}
	ent.AreaPrev = nil
	ent.AreaNext = nil
}

// LinkEdict adds an entity to the area grid.
func (s *Server) LinkEdict(ent *Edict, touchTriggers bool) {
	// Unlink from old position
	UnlinkEdict(ent)

	// Don't add the world
	if ent == s.Edicts[0] {
		return
	}
	if ent.Free {
		return
	}

	// Set the abs box
	ent.Vars.AbsMin[0] = ent.Vars.Origin[0] + ent.Vars.Mins[0]
	ent.Vars.AbsMin[1] = ent.Vars.Origin[1] + ent.Vars.Mins[1]
	ent.Vars.AbsMin[2] = ent.Vars.Origin[2] + ent.Vars.Mins[2]
	ent.Vars.AbsMax[0] = ent.Vars.Origin[0] + ent.Vars.Maxs[0]
	ent.Vars.AbsMax[1] = ent.Vars.Origin[1] + ent.Vars.Maxs[1]
	ent.Vars.AbsMax[2] = ent.Vars.Origin[2] + ent.Vars.Maxs[2]

	// to make items easier to pick up and allow them to be grabbed off
	// of shelves, the abs sizes are expanded
	if int(ent.Vars.Flags)&FlagItem != 0 {
		ent.Vars.AbsMin[0] -= 15
		ent.Vars.AbsMin[1] -= 15
		ent.Vars.AbsMax[0] += 15
		ent.Vars.AbsMax[1] += 15
	} else {
		// because movement is clipped an epsilon away from an actual edge,
		// we must fully check even when bounding boxes don't quite touch
		ent.Vars.AbsMin[0] -= 1
		ent.Vars.AbsMin[1] -= 1
		ent.Vars.AbsMin[2] -= 1
		ent.Vars.AbsMax[0] += 1
		ent.Vars.AbsMax[1] += 1
		ent.Vars.AbsMax[2] += 1
	}

	// Link to PVS leafs
	ent.NumLeafs = 0
	if ent.Vars.ModelIndex != 0 && s.WorldTree != nil && len(s.WorldTree.Nodes) > 0 {
		s.findTouchedLeafs(ent, bsp.TreeChild{Index: 0, IsLeaf: false})
	}

	if int(ent.Vars.Solid) == int(SolidNot) {
		return
	}

	// Find the first node that the ent's box crosses
	if len(s.Areanodes) == 0 {
		return
	}

	node := &s.Areanodes[0]
	for node.Axis != -1 {
		if ent.Vars.AbsMin[node.Axis] > node.Dist {
			if node.Children[0] == nil {
				break
			}
			node = node.Children[0]
		} else if ent.Vars.AbsMax[node.Axis] < node.Dist {
			if node.Children[1] == nil {
				break
			}
			node = node.Children[1]
		} else {
			break // crosses the node
		}
	}

	// Link it in
	if int(ent.Vars.Solid) == int(SolidTrigger) {
		sentinel := &node.TriggerEdicts
		ent.AreaNext = sentinel
		ent.AreaPrev = sentinel.AreaPrev
		ent.AreaPrev.AreaNext = ent
		ent.AreaNext.AreaPrev = ent
	} else {
		sentinel := &node.SolidEdicts
		ent.AreaNext = sentinel
		ent.AreaPrev = sentinel.AreaPrev
		ent.AreaPrev.AreaNext = ent
		ent.AreaNext.AreaPrev = ent
	}

	if touchTriggers {
		s.touchLinks(ent)
	}
}

func (s *Server) areaTriggerEdicts(ent *Edict, node *AreaNode, list *[]*Edict, listCap int) {
	for touch := node.TriggerEdicts.AreaNext; touch != nil && touch != &node.TriggerEdicts; touch = touch.AreaNext {
		if touch == ent {
			continue
		}
		if touch.Vars.Touch == 0 || int(touch.Vars.Solid) != int(SolidTrigger) {
			continue
		}
		if ent.Vars.AbsMin[0] > touch.Vars.AbsMax[0] ||
			ent.Vars.AbsMin[1] > touch.Vars.AbsMax[1] ||
			ent.Vars.AbsMin[2] > touch.Vars.AbsMax[2] ||
			ent.Vars.AbsMax[0] < touch.Vars.AbsMin[0] ||
			ent.Vars.AbsMax[1] < touch.Vars.AbsMin[1] ||
			ent.Vars.AbsMax[2] < touch.Vars.AbsMin[2] {
			continue
		}

		if len(*list) >= listCap {
			return
		}
		*list = append(*list, touch)
	}

	if node.Axis == -1 {
		return
	}

	if ent.Vars.AbsMax[node.Axis] > node.Dist && node.Children[0] != nil {
		s.areaTriggerEdicts(ent, node.Children[0], list, listCap)
	}
	if ent.Vars.AbsMin[node.Axis] < node.Dist && node.Children[1] != nil {
		s.areaTriggerEdicts(ent, node.Children[1], list, listCap)
	}
}

func (s *Server) touchLinks(ent *Edict) {
	if len(s.Areanodes) == 0 || s.QCVM == nil {
		return
	}

	entNum := s.NumForEdict(ent)
	moverClassName := qcString(s.QCVM, ent.Vars.ClassName)
	telemetryEnabled := s.DebugTelemetry != nil && s.DebugTelemetry.EventsEnabled()
	if telemetryEnabled {
		s.DebugTelemetry.LogEventf(DebugEventTrigger, s.QCVM, entNum, ent,
			"touchlinks begin mover_classname=%q touchfn=%d solid=%d absmin=(%.1f %.1f %.1f) absmax=(%.1f %.1f %.1f)",
			moverClassName, ent.Vars.Touch, int(ent.Vars.Solid),
			ent.Vars.AbsMin[0], ent.Vars.AbsMin[1], ent.Vars.AbsMin[2],
			ent.Vars.AbsMax[0], ent.Vars.AbsMax[1], ent.Vars.AbsMax[2])
		defer s.DebugTelemetry.LogEventf(DebugEventTrigger, s.QCVM, entNum, ent,
			"touchlinks end mover_classname=%q solid=%d touchfn=%d absmin=(%.1f %.1f %.1f) absmax=(%.1f %.1f %.1f)",
			moverClassName, int(ent.Vars.Solid), ent.Vars.Touch,
			ent.Vars.AbsMin[0], ent.Vars.AbsMin[1], ent.Vars.AbsMin[2],
			ent.Vars.AbsMax[0], ent.Vars.AbsMax[1], ent.Vars.AbsMax[2])
	}

	touches := make([]*Edict, 0, s.NumEdicts)
	s.areaTriggerEdicts(ent, &s.Areanodes[0], &touches, s.NumEdicts)
	if telemetryEnabled {
		s.DebugTelemetry.LogEventf(DebugEventTrigger, s.QCVM, entNum, ent,
			"touchlinks candidates=%d mover_classname=%q", len(touches), moverClassName)
	}

	oldSelf := s.QCVM.GetGlobalInt("self")
	oldOther := s.QCVM.GetGlobalInt("other")
	defer func() {
		s.QCVM.SetGlobalInt("self", oldSelf)
		s.QCVM.SetGlobalInt("other", oldOther)
	}()

	for _, touch := range touches {
		touchNum := s.NumForEdict(touch)
		touchClassName := qcString(s.QCVM, touch.Vars.ClassName)
		if touch == ent {
			if telemetryEnabled {
				s.DebugTelemetry.LogEventf(DebugEventTrigger, s.QCVM, entNum, ent,
					"touchlinks scan skip-self candidate=%d classname=%q", touchNum, touchClassName)
			}
			continue
		}
		if touch.Vars.Touch == 0 {
			if telemetryEnabled {
				s.DebugTelemetry.LogEventf(DebugEventTrigger, s.QCVM, touchNum, touch,
					"touchlinks scan reject candidate=%d other=%d reason=no-touch classname=%q solid=%d",
					touchNum, entNum, touchClassName, int(touch.Vars.Solid))
			}
			continue
		}
		if int(touch.Vars.Solid) != int(SolidTrigger) {
			if telemetryEnabled {
				s.DebugTelemetry.LogEventf(DebugEventTrigger, s.QCVM, touchNum, touch,
					"touchlinks scan reject candidate=%d other=%d reason=not-trigger classname=%q solid=%d",
					touchNum, entNum, touchClassName, int(touch.Vars.Solid))
			}
			continue
		}
		if ent.Vars.AbsMin[0] > touch.Vars.AbsMax[0] ||
			ent.Vars.AbsMin[1] > touch.Vars.AbsMax[1] ||
			ent.Vars.AbsMin[2] > touch.Vars.AbsMax[2] ||
			ent.Vars.AbsMax[0] < touch.Vars.AbsMin[0] ||
			ent.Vars.AbsMax[1] < touch.Vars.AbsMin[1] ||
			ent.Vars.AbsMax[2] < touch.Vars.AbsMin[2] {
			if telemetryEnabled {
				reason := "axis2"
				switch {
				case ent.Vars.AbsMin[0] > touch.Vars.AbsMax[0] || ent.Vars.AbsMax[0] < touch.Vars.AbsMin[0]:
					reason = "axis0"
				case ent.Vars.AbsMin[1] > touch.Vars.AbsMax[1] || ent.Vars.AbsMax[1] < touch.Vars.AbsMin[1]:
					reason = "axis1"
				}
				s.DebugTelemetry.LogEventf(DebugEventTrigger, s.QCVM, touchNum, touch,
					"touchlinks overlap-reject candidate=%d other=%d reason=%s candidate_abs=(%.1f %.1f %.1f)-(%.1f %.1f %.1f) other_abs=(%.1f %.1f %.1f)-(%.1f %.1f %.1f)",
					touchNum, entNum, reason,
					touch.Vars.AbsMin[0], touch.Vars.AbsMin[1], touch.Vars.AbsMin[2],
					touch.Vars.AbsMax[0], touch.Vars.AbsMax[1], touch.Vars.AbsMax[2],
					ent.Vars.AbsMin[0], ent.Vars.AbsMin[1], ent.Vars.AbsMin[2],
					ent.Vars.AbsMax[0], ent.Vars.AbsMax[1], ent.Vars.AbsMax[2],
				)
			}
			continue
		}

		if telemetryEnabled {
			s.DebugTelemetry.LogEventf(DebugEventTrigger, s.QCVM, touchNum, touch,
				"touchlinks callback begin self=%d(%q) other=%d(%q) fn=%d self_solid=%d other_solid=%d other_flags=%#x other_ground=%d other_vel=(%.1f %.1f %.1f) other_punch=(%.1f %.1f %.1f) other_fixangle=%d other_teleport=%.3f self_abs=(%.1f %.1f %.1f)-(%.1f %.1f %.1f) other_abs=(%.1f %.1f %.1f)-(%.1f %.1f %.1f)",
				touchNum, touchClassName, entNum, moverClassName, touch.Vars.Touch, int(touch.Vars.Solid), int(ent.Vars.Solid),
				uint32(ent.Vars.Flags), int(ent.Vars.GroundEntity),
				ent.Vars.Velocity[0], ent.Vars.Velocity[1], ent.Vars.Velocity[2],
				ent.Vars.PunchAngle[0], ent.Vars.PunchAngle[1], ent.Vars.PunchAngle[2],
				int(ent.Vars.FixAngle), ent.Vars.TeleportTime,
				touch.Vars.AbsMin[0], touch.Vars.AbsMin[1], touch.Vars.AbsMin[2],
				touch.Vars.AbsMax[0], touch.Vars.AbsMax[1], touch.Vars.AbsMax[2],
				ent.Vars.AbsMin[0], ent.Vars.AbsMin[1], ent.Vars.AbsMin[2],
				ent.Vars.AbsMax[0], ent.Vars.AbsMax[1], ent.Vars.AbsMax[2])
		}
		syncEdictToQCVM(s.QCVM, touchNum, touch)
		syncEdictToQCVM(s.QCVM, entNum, ent)
		pusherSnapshots := s.capturePusherSnapshots()
		s.syncPushersToQCVM()
		s.QCVM.SetGlobal("self", touchNum)
		s.QCVM.SetGlobal("other", entNum)
		s.setQCTimeGlobal(s.Time)
		prevNumEdicts := s.NumEdicts
		if err := s.executeQCFunction(int(touch.Vars.Touch)); err != nil {
			slog.Warn("touchlinks callback failed", "self", touchNum, "other", entNum, "func", touch.Vars.Touch, "err", err)
		} else {
			syncEdictFromQCVM(s.QCVM, touchNum, touch)
			syncEdictFromQCVM(s.QCVM, entNum, ent)
			s.syncMutatedPushersFromQCVM(pusherSnapshots)
			s.syncSpawnedEdictsFromQCVM(prevNumEdicts)
		}
		if telemetryEnabled {
			linkState := "linked"
			if touch.AreaPrev == nil {
				linkState = "unlinked"
			}
			s.DebugTelemetry.LogEventf(DebugEventTrigger, s.QCVM, touchNum, touch,
				"touchlinks callback end self=%d(%q) other=%d(%q) fn=%d self_solid=%d other_solid=%d self_link=%s other_flags=%#x other_ground=%d other_vel=(%.1f %.1f %.1f) other_punch=(%.1f %.1f %.1f) other_fixangle=%d other_teleport=%.3f self_origin=(%.1f %.1f %.1f) other_origin=(%.1f %.1f %.1f)",
				touchNum, touchClassName, entNum, moverClassName, touch.Vars.Touch, int(touch.Vars.Solid), int(ent.Vars.Solid), linkState,
				uint32(ent.Vars.Flags), int(ent.Vars.GroundEntity),
				ent.Vars.Velocity[0], ent.Vars.Velocity[1], ent.Vars.Velocity[2],
				ent.Vars.PunchAngle[0], ent.Vars.PunchAngle[1], ent.Vars.PunchAngle[2],
				int(ent.Vars.FixAngle), ent.Vars.TeleportTime,
				touch.Vars.Origin[0], touch.Vars.Origin[1], touch.Vars.Origin[2],
				ent.Vars.Origin[0], ent.Vars.Origin[1], ent.Vars.Origin[2])
		}
	}
}

// findTouchedLeafs finds all PVS leafs that an entity touches.
func (s *Server) findTouchedLeafs(ent *Edict, child bsp.TreeChild) {
	if child.IsLeaf {
		if child.Index < 0 || child.Index >= len(s.WorldTree.Leafs) {
			return
		}
		leaf := &s.WorldTree.Leafs[child.Index]
		if leaf.Contents != bsp.ContentsSolid {
			if ent.NumLeafs < MaxEntityLeafs {
				// Quake leaf numbers are 1-based in the PVS bitmask, but
				// we store the 0-based index here. We'll add 1 when checking PVS.
				ent.LeafNums[ent.NumLeafs] = child.Index
				ent.NumLeafs++
			}
		}
		return
	}

	node := &s.WorldTree.Nodes[child.Index]
	plane := &s.WorldTree.Planes[node.PlaneNum]

	var sides int
	if plane.Type < 3 {
		if ent.Vars.AbsMin[plane.Type] > plane.Dist {
			sides = 1
		} else if ent.Vars.AbsMax[plane.Type] < plane.Dist {
			sides = 2
		} else {
			sides = 3
		}
	} else {
		d1 := VecDot(ent.Vars.AbsMin, plane.Normal) - plane.Dist
		d2 := VecDot(ent.Vars.AbsMax, plane.Normal) - plane.Dist
		// This is a rough approximation for non-axial planes
		if d1 > 0 && d2 > 0 {
			sides = 1
		} else if d1 < 0 && d2 < 0 {
			sides = 2
		} else {
			sides = 3
		}
	}

	if sides&1 != 0 {
		s.findTouchedLeafs(ent, node.Children[0])
	}
	if sides&2 != 0 {
		s.findTouchedLeafs(ent, node.Children[1])
	}
}

// ============================================================================
// MAIN MOVE FUNCTION
// ============================================================================

// moveBounds calculates the bounding box for a move.
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

// clipToLinks clips a move to all entities in an area node.
func (s *Server) clipToLinks(node *AreaNode, clip *moveClip) {
	// Touch linked edicts
	for ent := node.SolidEdicts.AreaNext; ent != nil && ent != &node.SolidEdicts; ent = ent.AreaNext {
		if ent.Vars.Solid == float32(SolidNot) {
			continue
		}
		if ent == clip.passedict {
			continue
		}
		if ent.Vars.Solid == float32(SolidTrigger) {
			continue // Triggers shouldn't be in solid list
		}

		if clip.moveType == MoveNoMonsters && ent.Vars.Solid != float32(SolidBSP) {
			continue
		}

		// Check bounding box overlap
		if clip.boxMins[0] > ent.Vars.AbsMax[0] ||
			clip.boxMins[1] > ent.Vars.AbsMax[1] ||
			clip.boxMins[2] > ent.Vars.AbsMax[2] ||
			clip.boxMaxs[0] < ent.Vars.AbsMin[0] ||
			clip.boxMaxs[1] < ent.Vars.AbsMin[1] ||
			clip.boxMaxs[2] < ent.Vars.AbsMin[2] {
			continue
		}

		// Point entities never interact
		if clip.passedict != nil && clip.passedict.Vars.Size[0] != 0 && ent.Vars.Size[0] == 0 {
			continue
		}

		// Don't clip against own missiles or owner
		if clip.passedict != nil {
			if ent.Vars.Owner != 0 && s.EdictNum(int(ent.Vars.Owner)) == clip.passedict {
				continue
			}
			if clip.passedict.Vars.Owner != 0 && s.EdictNum(int(clip.passedict.Vars.Owner)) == ent {
				continue
			}
		}

		// Do an exact clip
		if clip.trace.AllSolid {
			return
		}

		var trace TraceResult
		if int(ent.Vars.Flags)&FlagMonster != 0 {
			trace = s.clipMoveToEntity(ent, clip.start, clip.mins2, clip.maxs2, clip.end)
		} else {
			trace = s.clipMoveToEntity(ent, clip.start, clip.mins, clip.maxs, clip.end)
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

	// Recurse down both sides
	if node.Axis == -1 {
		return
	}

	if clip.boxMaxs[node.Axis] > node.Dist && node.Children[0] != nil {
		s.clipToLinks(node.Children[0], clip)
	}
	if clip.boxMins[node.Axis] < node.Dist && node.Children[1] != nil {
		s.clipToLinks(node.Children[1], clip)
	}
}

// Move traces a move from start to end with the given bounding box.
// This is the main entry point for all collision detection.
func (s *Server) Move(start, mins, maxs, end [3]float32, moveType MoveType, passedict *Edict) TraceResult {
	var clip moveClip

	// Clip to world first
	if len(s.Edicts) > 0 && s.Edicts[0] != nil {
		clip.trace = s.clipMoveToEntity(s.Edicts[0], start, mins, maxs, end)
	}

	clip.start = start
	clip.end = end
	clip.mins = mins
	clip.maxs = maxs
	clip.moveType = int(moveType)
	clip.passedict = passedict

	// Set up mins2/maxs2 for monster clipping
	if moveType == MoveMissile {
		for i := 0; i < 3; i++ {
			clip.mins2[i] = -15
			clip.maxs2[i] = 15
		}
	} else {
		clip.mins2 = mins
		clip.maxs2 = maxs
	}

	// Create the bounding box of the entire move
	clip.boxMins, clip.boxMaxs = moveBounds(start, clip.mins2, clip.maxs2, end)

	// Clip to entities
	if len(s.Areanodes) > 0 {
		s.clipToLinks(&s.Areanodes[0], &clip)
	}

	return clip.trace
}

// TestEntityPosition tests if an entity is stuck in solid.
func (s *Server) TestEntityPosition(ent *Edict) *Edict {
	trace := s.Move(ent.Vars.Origin, ent.Vars.Mins, ent.Vars.Maxs, ent.Vars.Origin, MoveNormal, ent)

	if trace.StartSolid {
		if trace.Entity != nil {
			return trace.Entity
		}
		return s.Edicts[0] // world
	}

	return nil
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// boxOnPlaneSide returns which side of a plane a box is on.
// Returns 1, 2, or 3 (both sides).
func boxOnPlaneSide(mins, maxs [3]float32, plane *model.MPlane) int {
	// Fast axial cases
	if plane.Type < 3 {
		if plane.Dist <= mins[plane.Type] {
			return 1 // Front
		}
		if plane.Dist >= maxs[plane.Type] {
			return 2 // Back
		}
		return 3 // Crossing
	}

	// General case - compute corners based on plane normal signs
	var corners [2][3]float32
	for i := 0; i < 3; i++ {
		if plane.Normal[i] < 0 {
			corners[0][i] = maxs[i]
			corners[1][i] = mins[i]
		} else {
			corners[0][i] = mins[i]
			corners[1][i] = maxs[i]
		}
	}

	// Check front corner
	d1 := plane.Normal[0]*corners[0][0] + plane.Normal[1]*corners[0][1] + plane.Normal[2]*corners[0][2] - plane.Dist
	// Check back corner
	d2 := plane.Normal[0]*corners[1][0] + plane.Normal[1]*corners[1][1] + plane.Normal[2]*corners[1][2] - plane.Dist

	var sides int
	if d1 >= 0 {
		sides = 1
	}
	if d2 < 0 {
		sides |= 2
	}

	return sides
}

// Vec3Len returns the length of a 3D vector.
func Vec3Len(v [3]float32) float32 {
	return float32(math.Sqrt(float64(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])))
}

// Vec3Normalize normalizes a 3D vector in place.
func Vec3Normalize(v *[3]float32) {
	length := Vec3Len(*v)
	if length > 0 {
		v[0] /= length
		v[1] /= length
		v[2] /= length
	}
}
