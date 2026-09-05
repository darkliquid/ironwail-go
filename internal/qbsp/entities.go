package qbsp

import (
	"fmt"
	"strings"
)

// brushGroup is one model's brush set: the world (entity 0) or a single
// brush entity (func_wall/func_door style, compiled to an inline *N
// submodel).
type brushGroup struct {
	entityIdx int
	isWorld   bool
	brushes   []worldBrush
	bounds    [2]vec3
	origin    vec3
}

// collectAllBrushes registers every entity's brush planes/texinfos and
// partitions the world + brush entities into model groups. Returns the
// groups in model order (world first).
func (c *compiler) collectAllBrushes(m *Map, omitDetail bool) ([]brushGroup, error) {
	var groups []brushGroup
	worldGroup := brushGroup{entityIdx: 0, isWorld: true}
	if _, err := c.collectBrushesInto(m.Entities[0].Brushes, &worldGroup); err != nil {
		return nil, err
	}
	groups = append(groups, worldGroup)

	// Brush entities become submodels in encounter order.
	for ei := 1; ei < len(m.Entities); ei++ {
		ent := m.Entities[ei]
		if len(ent.Brushes) == 0 {
			continue
		}
		if omitDetail && ent.isDetail() {
			continue
		}
		solid := false
		for _, br := range ent.Brushes {
			if _, draw := contentsForBrush(br.Faces); draw {
				solid = true
				break
			}
		}
		if !solid {
			continue
		}
		g := brushGroup{entityIdx: ei}
		if o, ok := ent.Value("origin"); ok {
			if v, err := parseOrigin(o); err == nil {
				g.origin = v
			}
		}
		if _, err := c.collectBrushesInto(ent.Brushes, &g); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}

	// Bind submodel entities to their inline models (the engine resolves
	// "model" "*N" to BSP submodel N).
	for i := 1; i < len(groups); i++ {
		g := &groups[i]
		if g.isWorld {
			continue
		}
		ent := &m.Entities[g.entityIdx]
		ent.Epairs = append(ent.Epairs, Epair{Key: "model", Value: fmt.Sprintf("*%d", i)})
	}
	return groups, nil
}

// collectBrushesInto registers a brush set's planes/texinfos and appends
// worldBrush entries to the group.
func (c *compiler) collectBrushesInto(brushList []MapBrush, g *brushGroup) ([]brushGroup, error) {
	addBrush := func(brush MapBrush, content int32, sortKey int64) error {
		wb := worldBrush{orig: brush, content: content, sortKey: sortKey}
		for _, face := range brush.Faces {
			pi, ok := c.planeIndexFor(face)
			if !ok {
				continue
			}
			wb.planes = append(wb.planes, pi)
			if _, exists := c.texByPlane[pi]; !exists {
				ti := c.texinfoIndex(face)
				c.texByPlane[pi] = ti
			}
		}
		wm, wx, err := brushBounds(brush)
		if err != nil {
			return err
		}
		wb.bounds = [2]vec3{wm, wx}
		if len(g.brushes) == 0 {
			g.bounds = wb.bounds
		} else {
			for i := 0; i < 3; i++ {
				if wm[i] < g.bounds[0][i] {
					g.bounds[0][i] = wm[i]
				}
				if wx[i] > g.bounds[1][i] {
					g.bounds[1][i] = wx[i]
				}
			}
		}
		g.brushes = append(g.brushes, wb)
		return nil
	}

	for _, brush := range brushList {
		content, draw := contentsForBrush(brush.Faces)
		if !draw {
			continue
		}
		key := int64(g.entityIdx)<<32 | int64(brush.Line)
		if err := addBrush(brush, content, key); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// OutwardPlanes returns the oriented outward planes of the original brush
// faces (used to seed solidbsp polyhedra).
func (wb *worldBrush) OutwardPlanes() []plane {
	ps := make([]plane, 0, len(wb.orig.Faces))
	for _, f := range wb.orig.Faces {
		p := f.Plane()
		if v3Length(p.Normal) < 1e-9 {
			continue
		}
		p.Dist = snapPlaneDist(p.Dist)
		ps = append(ps, p)
	}
	return ps
}

// worldBoundsOf resolves the compiler root bounds from a group's brush
// bounds (degenerate groups fall back to a unit box).
func worldBoundsOf(g *brushGroup) [2]vec3 {
	if len(g.brushes) == 0 {
		return [2]vec3{{-1, -1, -1}, {1, 1, 1}}
	}
	return g.bounds
}

// bspBrushList converts a group's world brushes into solidbsp polyhedra
// with table-registered planenums.
func (c *compiler) bspBrushList(g *brushGroup) []*bspBrush {
	bounds := worldBoundsOf(g)
	var list []*bspBrush
	for _, wb := range g.brushes {
		ps := wb.OutwardPlanes()
		faces := make([]brushFace, len(ps))
		for i, p := range ps {
			faces[i] = brushFace{p: p, pn: c.addPlaneIndex(p)}
		}
		b := buildBspBrushFaces(faces, bounds)
		if b == nil {
			continue
		}
		b.content = wb.content
		b.sortKey = wb.sortKey
		list = append(list, b)
	}
	return list
}

// isDetail reports whether the entity is a func_detail* brush entity
// (omitted entirely with -omitdetail).
func (e *Entity) isDetail() bool {
	cn, _ := e.Value("classname")
	return strings.HasPrefix(cn, "func_detail")
}
