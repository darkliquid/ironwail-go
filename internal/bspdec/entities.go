package bspdec

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// parseEntities decodes the BSP entity lump (the same "{ }" block grammar as
// .map files, minus brushes).
func parseEntities(tree *bsp.Tree) (*mapfile.Map, error) {
	m, err := mapfile.Parse(bytes.NewReader(tree.Entities))
	if err != nil {
		return nil, fmt.Errorf("parsing entity lump: %w", err)
	}
	return m, nil
}

// setEpair replaces the last value stored under key, or appends it.
func setEpair(ent *mapfile.Entity, key, value string) {
	for i := range ent.Epairs {
		if ent.Epairs[i].Key == key {
			ent.Epairs[i].Value = value
			return
		}
	}
	ent.Epairs = append(ent.Epairs, mapfile.Epair{Key: key, Value: value})
}

// setBmodelOrigin copies dmodel.Origin into the entity's origin key. BSP
// bmodel geometry is stored at its authored position, so the key alone
// reproduces the compile-time placement.
//
// Where in C: model origin recovery in bspc map_q1.c / ericw decompile.cc.
func setBmodelOrigin(ent *mapfile.Entity, m *bsp.DModel) {
	if m.Origin.X == 0 && m.Origin.Y == 0 && m.Origin.Z == 0 {
		return
	}
	setEpair(ent, "origin", fmt.Sprintf("%g %g %g", m.Origin.X, m.Origin.Y, m.Origin.Z))
}

// attachBrushes wires per-model decompiled brushes into the entity map:
// model 0 brushes go to worldspawn (entity 0), model N brushes to the entity
// whose "model" key is "*N". All other epairs pass through verbatim.
//
// Where in C: entity handling in bspc map_q1.c (Q1_LoadBSPFile entity pass).
func attachBrushes(ents *mapfile.Map, perModel [][]*Brush, tree *bsp.Tree) *mapfile.Map {
	out := &mapfile.Map{}
	for ei := range ents.Entities {
		ent := ents.Entities[ei] // shallow copy; Brushes replaced below
		ent.Brushes = nil
		model := -1
		if ei == 0 {
			model = 0
		} else if v, ok := ent.Value("model"); ok && strings.HasPrefix(v, "*") {
			if n, err := strconv.Atoi(v[1:]); err == nil {
				model = n
			}
		}
		if model >= 0 && model < len(perModel) {
			for _, b := range perModel[model] {
				ent.Brushes = append(ent.Brushes, toMapBrush(b))
			}
			if model > 0 && model < len(tree.Models) {
				setBmodelOrigin(&ent, &tree.Models[model])
			}
		}
		out.Entities = append(out.Entities, ent)
	}
	return out
}

// toMapBrush converts a decompiled brush to a .map brush: each side becomes a
// face defined by three non-collinear winding points plus Valve 220 vecs.
// The triple is ordered so the re-derived plane matches the side's outward
// normal (pick3Points may otherwise pick a combination whose ericw-convention
// cross product flips sign, which would invert the face).
func toMapBrush(b *Brush) mapfile.MapBrush {
	var mb mapfile.MapBrush
	for _, s := range b.Sides {
		if s.Winding == nil || len(s.Winding.Points) < 3 {
			continue
		}
		pts := pick3Points(s.Winding)
		if p, length := mapfile.PlaneFromPoints(pts[0], pts[1], pts[2]); length > 0.01 && v3Dot(p.Normal, s.Plane.Normal) < 0 {
			pts[1], pts[2] = pts[2], pts[1]
		}
		mb.Faces = append(mb.Faces, mapfile.MapFace{
			Points:  pts,
			TexName: s.TexName,
			Vecs:    s.Vecs,
		})
	}
	return mb
}

// pick3Points returns the maximum-area triple of winding points, which keeps
// the re-derived plane as numerically accurate as possible.
func pick3Points(w *Winding) [3]mapfile.Vec3 {
	p0 := w.Points[0]
	p1, p2 := w.Points[1], w.Points[1]
	best := -1.0
	for _, p := range w.Points[1:] {
		if d := v3Len(v3Sub(p, p0)); d > best {
			best = d
			p1 = p
		}
	}
	best = -1.0
	for _, p := range w.Points {
		if a := v3Len(v3Cross(v3Sub(p1, p0), v3Sub(p, p0))); a > best {
			best = a
			p2 = p
		}
	}
	return [3]mapfile.Vec3{p0, p1, p2}
}