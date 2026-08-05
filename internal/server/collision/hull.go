// hull.go provides collision hull creation and point/plane queries.
package collision

import (
	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

// Box hull state - used for entity bounding box collision
var (
	boxHull       model.Hull
	boxClipNodes  [6]model.MClipNode
	boxPlanes     [6]model.MPlane
	boxHullInited bool
)

// initBoxHull sets up the planes and clipnodes for bounding box hulls.
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

// hullForEntity returns a hull used for testing or clipping an object of mins/maxs size.
func (c *System) hullForEntity(ent *srvtypes.Edict, mins, maxs [3]float32, offset *[3]float32) *model.Hull {
	sh := c.sh
	if int(ent.Solid(sh)) == int(srvtypes.SolidBSP) {
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

		if c.world != nil {
			if m := c.world.GetWorldModel(); m != nil {
				var hull model.Hull
				if hullNum >= 0 && hullNum < m.NumHulls() {
					hull = m.Hull(hullNum)
				}
				modelIndex := int(ent.ModelIndex(sh))
				if ent.Num == 0 || modelIndex <= 1 {
					modelIndex = 1
				}
				if modelIndex > 1 && c.world.GetWorldTree() != nil && modelIndex-1 < len(c.world.GetWorldTree().Models) {
					headNode := int(c.world.GetWorldTree().Models[modelIndex-1].HeadNode[hullNum])
					if headNode >= 0 {
						hull.FirstClipNode = headNode
					}
				}
				if len(hull.ClipNodes) > 0 && hull.FirstClipNode >= 0 {
					origin := ent.Origin(sh)
					offset[0] = hull.ClipMins[0] - mins[0] + origin[0]
					offset[1] = hull.ClipMins[1] - mins[1] + origin[1]
					offset[2] = hull.ClipMins[2] - mins[2] + origin[2]
					return &hull
				}
			}
		}

		entMins := ent.Mins(sh)
		entMaxs := ent.Maxs(sh)
		origin := ent.Origin(sh)
		hullMins := [3]float32{
			entMins[0] - maxs[0],
			entMins[1] - maxs[1],
			entMins[2] - maxs[2],
		}
		hullMaxs := [3]float32{
			entMaxs[0] - mins[0],
			entMaxs[1] - mins[1],
			entMaxs[2] - mins[2],
		}
		offset[0] = origin[0]
		offset[1] = origin[1]
		offset[2] = origin[2]
		return hullForBox(hullMins, hullMaxs)
	}

	entMins := ent.Mins(sh)
	entMaxs := ent.Maxs(sh)
	origin := ent.Origin(sh)
	hullMins := [3]float32{
		entMins[0] - maxs[0],
		entMins[1] - maxs[1],
		entMins[2] - maxs[2],
	}
	hullMaxs := [3]float32{
		entMaxs[0] - mins[0],
		entMaxs[1] - mins[1],
		entMaxs[2] - mins[2],
	}
	hull := hullForBox(hullMins, hullMaxs)

	offset[0] = origin[0]
	offset[1] = origin[1]
	offset[2] = origin[2]
	return hull
}

func hullPlaneDistance(plane *model.MPlane, p [3]float32) float32 {
	if plane.Type < 3 {
		return p[plane.Type] - plane.Dist
	}

	return float32(float64(plane.Normal[0])*float64(p[0]) +
		float64(plane.Normal[1])*float64(p[1]) +
		float64(plane.Normal[2])*float64(p[2]) -
		float64(plane.Dist))
}

func HullPointContents(hull *model.Hull, num int, p [3]float32) int {
	return hullPointContents(hull, num, p)
}

func hullPointContents(hull *model.Hull, num int, p [3]float32) int {
	if num < 0 {
		return num
	}

	if num < hull.FirstClipNode || num > hull.LastClipNode {
		return bsp.ContentsSolid
	}

	for num >= 0 {
		if num < hull.FirstClipNode || num > hull.LastClipNode {
			return bsp.ContentsSolid
		}

		node := &hull.ClipNodes[num]
		plane := &hull.Planes[node.PlaneNum]
		d := hullPlaneDistance(plane, p)

		if d < 0 {
			num = node.Children[1]
		} else {
			num = node.Children[0]
		}
	}

	return num
}

func boxOnPlaneSide(emins, emaxs [3]float32, p *model.MPlane) int {
	if p.Type < 3 {
		if p.Dist <= emins[p.Type] {
			return 1
		}
		if p.Dist >= emaxs[p.Type] {
			return 2
		}
		return 3
	}

	var dist1, dist2 float32
	for i := 0; i < 3; i++ {
		if p.Normal[i] < 0 {
			dist1 += p.Normal[i] * emins[i]
			dist2 += p.Normal[i] * emaxs[i]
		} else {
			dist1 += p.Normal[i] * emaxs[i]
			dist2 += p.Normal[i] * emins[i]
		}
	}

	sides := 0
	if dist1 >= p.Dist {
		sides = 1
	}
	if dist2 < p.Dist {
		sides |= 2
	}

	return sides
}
