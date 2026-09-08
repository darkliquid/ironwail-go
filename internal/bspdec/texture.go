package bspdec

import (
	"bytes"
	"encoding/binary"
	"math"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// texturedFace is a tree face with its plane and winding precomputed.
type texturedFace struct {
	plane   mapfile.Plane
	winding *Winding
	texName string
	vecs    [2][4]float64
}

// textureNames decodes the miptex lump into a name table.
//
// Where in C: dmiptexlump_t/miptex_t in WinQuake bspfile.h.
func textureNames(tree *bsp.Tree) []string {
	data := tree.TextureData
	if len(data) < 4 {
		return nil
	}
	n := int(int32(binary.LittleEndian.Uint32(data)))
	if n < 0 || 4+4*n > len(data) {
		return nil
	}
	names := make([]string, n)
	for i := 0; i < n; i++ {
		ofs := int(int32(binary.LittleEndian.Uint32(data[4+i*4:])))
		if ofs <= 0 || ofs+16 > len(data) {
			continue // unassigned texture slot
		}
		names[i] = string(bytes.TrimRight(data[ofs:ofs+16], "\x00"))
	}
	return names
}

func (d *decompiler) texName(mi int32) string {
	if mi < 0 || int(mi) >= len(d.texNames) {
		return ""
	}
	return d.texNames[mi]
}

// texinfoVecs copies the float32 s/t mapping to float64 for map emission.
func texinfoVecs(ti *bsp.Texinfo) [2][4]float64 {
	var v [2][4]float64
	for i := 0; i < 2; i++ {
		for j := 0; j < 4; j++ {
			v[i][j] = float64(ti.Vecs[i][j])
		}
	}
	return v
}

func vecsEqual(a, b [2][4]float64) bool {
	for i := 0; i < 2; i++ {
		for j := 0; j < 4; j++ {
			if math.Abs(a[i][j]-b[i][j]) > 1e-4 {
				return false
			}
		}
	}
	return true
}

// facePlane returns the face's oriented plane (negated for side 1).
func (d *decompiler) facePlane(f *bsp.TreeFace) mapfile.Plane {
	p := d.plane(f.PlaneNum)
	if f.Side != 0 {
		p = negatePlane(p)
	}
	return p
}

// faceWinding rebuilds the polygon of tree face fi from the surfedge walk.
//
// Where in C: face winding reconstruction in ericw-tools common/decompile.cc
// (surfedges -> edges -> vertexes; negative surfedge = reversed edge).
func (d *decompiler) faceWinding(fi int) *Winding {
	f := d.tree.Faces[fi]
	w := &Winding{}
	for se := f.FirstEdge; se < f.FirstEdge+f.NumEdges; se++ {
		s := d.tree.Surfedges[se]
		var vi uint32
		if s >= 0 {
			vi = d.tree.Edges[s].V[0]
		} else {
			vi = d.tree.Edges[-s].V[1]
		}
		p := d.tree.Vertexes[vi].Point
		w.Points = append(w.Points, mapfile.Vec3{X: float64(p.X), Y: float64(p.Y), Z: float64(p.Z)})
	}
	return w
}

// collectFaces precomputes every tree face's plane and winding once.
func (d *decompiler) collectFaces() []texturedFace {
	if d.faces != nil {
		return d.faces
	}
	d.faces = make([]texturedFace, 0, len(d.tree.Faces))
	for fi := range d.tree.Faces {
		f := &d.tree.Faces[fi]
		w := d.faceWinding(fi)
		if len(w.Points) < 3 {
			continue
		}
		if f.Texinfo < 0 || int(f.Texinfo) >= len(d.tree.Texinfo) {
			continue
		}
		ti := &d.tree.Texinfo[f.Texinfo]
		d.faces = append(d.faces, texturedFace{
			plane:   d.facePlane(f),
			winding: w,
			texName: d.texName(ti.Miptex),
			vecs:    texinfoVecs(ti),
		})
	}
	return d.faces
}

// clipToBrush returns the part of w lying inside brush b, or nil. The plane
// the winding is coplanar with (onPlane) must be excluded from the clip:
// a face winding lies exactly on the very side it is being matched to, and
// clipping by that plane degenerates.
func clipToBrush(w *Winding, b *Brush, onPlane mapfile.Plane) *Winding {
	for _, s := range b.Sides {
		if planesMatch(s.Plane, onPlane) {
			continue
		}
		w = w.Clip(negatePlane(s.Plane))
		if w == nil {
			return nil
		}
	}
	return w
}

// overlapArea returns the area of fw lying inside brush b, excluding the
// side the face is coplanar with, so the clipped polygon is the overlap
// region.
func overlapArea(fw *Winding, b *Brush, onPlane mapfile.Plane) float64 {
	w := clipToBrush(fw, b, onPlane)
	if w == nil {
		return 0
	}
	return w.Area()
}

// textureBrushes assigns every side of every brush a texture.
//
// Where in C: face-overlap texturing in ericw-tools common/decompile.cc.
func (d *decompiler) textureBrushes(brushes []*Brush) {
	for _, b := range brushes {
		d.textureBrush(b)
	}
}

// textureBrush assigns each side the texinfo of the tree face on the same
// plane with the largest overlap area; unmatched sides fall back.
func (d *decompiler) textureBrush(b *Brush) {
	faces := d.collectFaces()
	for _, s := range b.Sides {
		best := -1
		bestArea := 0.0
		for i := range faces {
			if !planesMatch(s.Plane, faces[i].plane) {
				continue
			}
			if a := overlapArea(faces[i].winding, b, s.Plane); a > bestArea {
				bestArea = a
				best = i
			}
		}
		if best >= 0 {
			s.TexName = faces[best].texName
			s.Vecs = faces[best].vecs
			continue
		}
		d.fallbackTexture(b, s)
	}
}

// contentsTexture maps leaf contents to its convention texture ("" if none).
//
// Where in C: Q1_FixContentsTextures in bspc map_q1.c.
func contentsTexture(c int32) string {
	switch c {
	case bsp.ContentsWater:
		return "*water"
	case bsp.ContentsSlime:
		return "*slime"
	case bsp.ContentsLava:
		return "*lava"
	case bsp.ContentsSky:
		return "sky"
	case bsp.ContentsClip:
		return "clip"
	default:
		return ""
	}
}

// fallbackTexture textures a side that matched no tree face: the brush's
// contents texture first (liquid/sky interiors), then the policy.
func (d *decompiler) fallbackTexture(b *Brush, s *Side) {
	if t := contentsTexture(b.Contents); t != "" {
		s.TexName = t
		return
	}
	switch d.opts.TextureFallback {
	case "skip":
		s.TexName = "skip" // ericw qbsp drops SKIP surfaces at compile
	case "trigger":
		s.TexName = "trigger"
	default: // "nearest"
		best := ""
		bestArea := -1.0
		for _, o := range b.Sides {
			if o.TexName == "" || o.Winding == nil {
				continue
			}
			if a := o.Winding.Area(); a > bestArea {
				bestArea = a
				best = o.TexName
			}
		}
		if best == "" {
			d.warnf("brush has no matched side to copy a texture from; using clip")
			best = "clip"
		}
		s.TexName = best
	}
}