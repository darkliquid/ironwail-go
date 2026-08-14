// hull.go provides collision hull creation and point/plane queries.
package collision

import (
	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
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
		switch i >> 1 {
		case 0:
			boxPlanes[i].Normal = qtypes.Vec3{X: 1, Y: 0, Z: 0}
		case 1:
			boxPlanes[i].Normal = qtypes.Vec3{X: 0, Y: 1, Z: 0}
		case 2:
			boxPlanes[i].Normal = qtypes.Vec3{X: 0, Y: 0, Z: 1}
		}
	}

	boxHullInited = true
}

// hullForBox creates a temporary hull from bounding box sizes.
func hullForBox(mins, maxs qtypes.Vec3) *model.Hull {
	initBoxHull()

	boxPlanes[0].Dist = maxs.X
	boxPlanes[1].Dist = mins.X
	boxPlanes[2].Dist = maxs.Y
	boxPlanes[3].Dist = mins.Y
	boxPlanes[4].Dist = maxs.Z
	boxPlanes[5].Dist = mins.Z

	return &boxHull
}

// hullForEntity returns a hull used for testing or clipping an object of mins/maxs size.
func (c *System) hullForEntity(ent *srvtypes.Edict, mins, maxs qtypes.Vec3, offset *qtypes.Vec3) *model.Hull {
	sh := c.sh
	if int(ent.Solid(sh)) == int(srvtypes.SolidBSP) {
		size := maxs.Sub(mins)

		hullNum := 0
		if size.X >= 3 {
			if size.X <= 32 {
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
					*offset = hull.ClipMins.Sub(mins).Add(origin)
					return &hull
				}
			}
		}

		entMins := ent.Mins(sh)
		entMaxs := ent.Maxs(sh)
		origin := ent.Origin(sh)
		hullMins := entMins.Sub(maxs)
		hullMaxs := entMaxs.Sub(mins)
		*offset = origin
		return hullForBox(hullMins, hullMaxs)
	}

	entMins := ent.Mins(sh)
	entMaxs := ent.Maxs(sh)
	origin := ent.Origin(sh)
	hullMins := entMins.Sub(maxs)
	hullMaxs := entMaxs.Sub(mins)
	hull := hullForBox(hullMins, hullMaxs)

	*offset = origin
	return hull
}

func hullPlaneDistance(plane *model.MPlane, p qtypes.Vec3) float32 {
	if plane.Type < 3 {
		switch plane.Type {
		case 0:
			return p.X - plane.Dist
		case 1:
			return p.Y - plane.Dist
		default:
			return p.Z - plane.Dist
		}
	}

	return float32(float64(plane.Normal.X)*float64(p.X) +
		float64(plane.Normal.Y)*float64(p.Y) +
		float64(plane.Normal.Z)*float64(p.Z) -
		float64(plane.Dist))
}

func HullPointContents(hull *model.Hull, num int, p qtypes.Vec3) int {
	return hullPointContents(hull, num, p)
}

func hullPointContents(hull *model.Hull, num int, p qtypes.Vec3) int {
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

func boxOnPlaneSide(emins, emaxs qtypes.Vec3, p *model.MPlane) int {
	if p.Type < 3 {
		var minVal, maxVal float32
		switch p.Type {
		case 0:
			minVal, maxVal = emins.X, emaxs.X
		case 1:
			minVal, maxVal = emins.Y, emaxs.Y
		default:
			minVal, maxVal = emins.Z, emaxs.Z
		}
		if p.Dist <= minVal {
			return 1
		}
		if p.Dist >= maxVal {
			return 2
		}
		return 3
	}

	var dist1, dist2 float32
	if p.Normal.X < 0 {
		dist1 += p.Normal.X * emins.X
		dist2 += p.Normal.X * emaxs.X
	} else {
		dist1 += p.Normal.X * emaxs.X
		dist2 += p.Normal.X * emins.X
	}
	if p.Normal.Y < 0 {
		dist1 += p.Normal.Y * emins.Y
		dist2 += p.Normal.Y * emaxs.Y
	} else {
		dist1 += p.Normal.Y * emaxs.Y
		dist2 += p.Normal.Y * emins.Y
	}
	if p.Normal.Z < 0 {
		dist1 += p.Normal.Z * emins.Z
		dist2 += p.Normal.Z * emaxs.Z
	} else {
		dist1 += p.Normal.Z * emaxs.Z
		dist2 += p.Normal.Z * emins.Z
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
