// areanode.go provides spatial partitioning and trigger touch links.
package collision

import (
	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

type AreaNode = srvtypes.AreaNode

const (
	AreaDepth      = srvtypes.AreaDepth
	AreaNodes      = srvtypes.AreaNodes
	MaxEntityLeafs = 16
)

func isNilCollisionModel(m srvtypes.CollisionModel) bool {
	if m == nil {
		return true
	}
	if mod, ok := m.(*model.Model); ok && mod == nil {
		return true
	}
	return false
}

func (c *System) Areanodes() []AreaNode {
	return c.areanodes
}

func (c *System) createAreaNode(depth int, mins, maxs [3]float32) *AreaNode {
	if len(c.areanodes) <= c.numAreaNodes {
		return nil
	}

	node := &c.areanodes[c.numAreaNodes]
	c.numAreaNodes++

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

	node.Children[0] = c.createAreaNode(depth+1, mins2, maxs2)
	node.Children[1] = c.createAreaNode(depth+1, mins1, maxs1)

	return node
}

// ClearWorld initializes the area nodes for a new map.
func (c *System) ClearWorld() {
	initBoxHull()

	if len(c.areanodes) != AreaNodes {
		c.areanodes = make([]AreaNode, AreaNodes)
	}

	c.numAreaNodes = 0

	var mins, maxs [3]float32
	if c.world != nil && !isNilCollisionModel(c.world.GetWorldModel()) {
		mins = c.world.GetWorldModel().CollisionClipMins()
		maxs = c.world.GetWorldModel().CollisionClipMaxs()
	}

	c.createAreaNode(0, mins, maxs)
}

// UnlinkEdict removes an entity from the area grid.
func UnlinkEdict(ent *srvtypes.Edict) {
	if ent.AreaPrev == nil {
		return
	}

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
func (c *System) LinkEdict(ent *srvtypes.Edict, touchTriggers bool) {
	if c == nil || ent == nil {
		return
	}
	UnlinkEdict(ent)
	if ent.Num == 0 || ent.Free {
		return
	}

	sh := c.sh
	origin := ent.Origin(sh)
	mins := ent.Mins(sh)
	maxes := ent.Maxs(sh)
	absMin := [3]float32{origin[0] + mins[0], origin[1] + mins[1], origin[2] + mins[2]}
	absMax := [3]float32{origin[0] + maxes[0], origin[1] + maxes[1], origin[2] + maxes[2]}

	if uint32(ent.Flags(sh))&srvtypes.FlagItem != 0 {
		absMin[0] -= 15
		absMin[1] -= 15
		absMax[0] += 15
		absMax[1] += 15
	} else {
		absMin[0] -= 1
		absMin[1] -= 1
		absMin[2] -= 1
		absMax[0] += 1
		absMax[1] += 1
		absMax[2] += 1
	}
	ent.SetAbsMin(sh, absMin)
	ent.SetAbsMax(sh, absMax)

	ent.NumLeafs = 0
	if ent.ModelIndex(sh) != 0 && c.world != nil && c.world.GetWorldTree() != nil && len(c.world.GetWorldTree().Nodes) > 0 {
		c.findTouchedLeafs(ent, bsp.TreeChild{Index: 0, IsLeaf: false})
	}
	if int(ent.Solid(sh)) == int(srvtypes.SolidNot) || len(c.areanodes) == 0 {
		return
	}

	node := &c.areanodes[0]
	for node.Axis != -1 {
		if absMin[node.Axis] > node.Dist {
			if node.Children[0] != nil {
				node = node.Children[0]
			}
		} else if absMax[node.Axis] < node.Dist {
			if node.Children[1] != nil {
				node = node.Children[1]
			}
		}
		if node.Axis == -1 || (absMin[node.Axis] <= node.Dist && absMax[node.Axis] >= node.Dist) {
			break
		}
	}

	sentinel := &node.SolidEdicts
	if int(ent.Solid(sh)) == int(srvtypes.SolidTrigger) {
		sentinel = &node.TriggerEdicts
	}
	ent.AreaNext = sentinel
	ent.AreaPrev = sentinel.AreaPrev
	ent.AreaPrev.AreaNext = ent
	ent.AreaNext.AreaPrev = ent

	if touchTriggers && c.touch != nil {
		c.touch.TouchLinks(ent)
	}
}

func (c *System) FindTouchedLeafs(ent *srvtypes.Edict, child bsp.TreeChild) {
	c.findTouchedLeafs(ent, child)
}

func (c *System) findTouchedLeafs(ent *srvtypes.Edict, child bsp.TreeChild) {
	if c.world == nil || c.world.GetWorldTree() == nil {
		return
	}
	tree := c.world.GetWorldTree()
	sh := c.sh

	if child.IsLeaf {
		if child.Index < 0 || child.Index >= len(tree.Leafs) {
			return
		}
		leaf := &tree.Leafs[child.Index]
		if leaf.Contents != bsp.ContentsSolid {
			visLeafIndex := child.Index - 1
			if visLeafIndex < 0 {
				return
			}
			if ent.NumLeafs < MaxEntityLeafs {
				ent.LeafNums[ent.NumLeafs] = visLeafIndex
				ent.NumLeafs++
			}
		}
		return
	}

	node := &tree.Nodes[child.Index]
	plane := &tree.Planes[node.PlaneNum]
	sides := boxOnPlaneSide(ent.AbsMin(sh), ent.AbsMax(sh), &model.MPlane{
		Normal: plane.Normal,
		Dist:   plane.Dist,
		Type:   uint8(plane.Type),
	})

	if sides&1 != 0 {
		c.findTouchedLeafs(ent, node.Children[0])
	}
	if sides&2 != 0 {
		c.findTouchedLeafs(ent, node.Children[1])
	}
}

func (c *System) AreaTriggerEdicts(ent *srvtypes.Edict, node *AreaNode, list *[]*srvtypes.Edict, listCap int) {
	c.areaTriggerEdicts(ent, node, list, listCap)
}

func (c *System) areaTriggerEdicts(ent *srvtypes.Edict, node *AreaNode, list *[]*srvtypes.Edict, listCap int) {
	if node == nil {
		if len(c.areanodes) == 0 {
			return
		}
		node = &c.areanodes[0]
	}
	sh := c.sh
	entAbsMin := ent.AbsMin(sh)
	entAbsMax := ent.AbsMax(sh)
	for touch := node.TriggerEdicts.AreaNext; touch != nil && touch != &node.TriggerEdicts; touch = touch.AreaNext {
		if touch == ent {
			continue
		}
		if touch.Touch(sh) == 0 || int(touch.Solid(sh)) != int(srvtypes.SolidTrigger) {
			continue
		}
		touchAbsMin := touch.AbsMin(sh)
		touchAbsMax := touch.AbsMax(sh)
		if entAbsMin[0] > touchAbsMax[0] ||
			entAbsMin[1] > touchAbsMax[1] ||
			entAbsMin[2] > touchAbsMax[2] ||
			entAbsMax[0] < touchAbsMin[0] ||
			entAbsMax[1] < touchAbsMin[1] ||
			entAbsMax[2] < touchAbsMin[2] {
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

	if entAbsMax[node.Axis] > node.Dist && node.Children[0] != nil {
		c.areaTriggerEdicts(ent, node.Children[0], list, listCap)
	}
	if entAbsMin[node.Axis] < node.Dist && node.Children[1] != nil {
		c.areaTriggerEdicts(ent, node.Children[1], list, listCap)
	}
}
