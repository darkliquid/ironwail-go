package bspdec

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// BSPXBrush is one original brush recovered from the BRUSHLIST lump: its
// model, axis-aligned bounds, contents, and full face-plane set.
type BSPXBrush struct {
	Model    int
	Mins     mapfile.Vec3
	Maxs     mapfile.Vec3
	Contents int32
	Faces    []mapfile.Plane
}

var (
	brushListErr = fmt.Errorf("bspdec: BRUSHLIST parse")
	errNoLump    = fmt.Errorf("%w: no BRUSHLIST lump", brushListErr)
)

// ParseBrushList parses the concatenated per-model BRUSHLIST records
// written by internal/qbsp.serializeBSPXBrushes (the ericw
// bspxbrushes_permodel layout: ver, modelnum, numbrushes, numfaces, then
// per brush aabb3f bounds, int16 contents, uint16 numfaces, per face a
// qplane3f).
func ParseBrushList(buf []byte) ([]BSPXBrush, error) {
	var out []BSPXBrush
	off := 0
	for off+16 <= len(buf) {
		ver := int32(binary.LittleEndian.Uint32(buf[off:]))
		model := int(binary.LittleEndian.Uint32(buf[off+4:]))
		nbrushes := int(binary.LittleEndian.Uint32(buf[off+8:]))
		off += 16
		if ver != 1 {
			return nil, fmt.Errorf("%w: unsupported version %d", brushListErr, ver)
		}
		for i := 0; i < nbrushes; i++ {
			if off+32 > len(buf) {
				return nil, fmt.Errorf("%w: truncated brush", brushListErr)
			}
			b := BSPXBrush{Model: model}
			b.Mins.X = float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[off:])))
			b.Mins.Y = float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[off+4:])))
			b.Mins.Z = float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[off+8:])))
			b.Maxs.X = float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[off+12:])))
			b.Maxs.Y = float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[off+16:])))
			b.Maxs.Z = float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[off+20:])))
			// writeBSPXBrush emits bounds (24) + 4 pad bytes + contents (2) +
			// numfaces (2); skip the pad and read the two counts at +28/+30,
			// mirroring ReadBSPXBrushList's p+=28; p+=4 walk.
			b.Contents = int32(int16(binary.LittleEndian.Uint16(buf[off+28:])))
			nfaces := int(binary.LittleEndian.Uint16(buf[off+30:]))
			off += 32
			for j := 0; j < nfaces; j++ {
				if off+16 > len(buf) {
					return nil, fmt.Errorf("%w: truncated face", brushListErr)
				}
				pl := mapfile.Plane{
					Normal: mapfile.Vec3{
						X: float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[off:]))),
						Y: float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[off+4:]))),
						Z: float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[off+8:]))),
					},
					Dist: float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[off+12:]))),
				}
				b.Faces = append(b.Faces, pl)
				off += 16
			}
			out = append(out, b)
		}
	}
	return out, nil
}

// brushListPayload locates the appended BSPX block (8-byte "BSPX"+numlumps
// header, then 32-byte lump entries) and returns the BRUSHLIST payload.
// It mirrors the header walk in internal/qbsp.ReadBSPXBrushList; the qbsp
// helper only returns per-model counts, so the scan is replicated here.
func brushListPayload(bspData []byte) ([]byte, error) {
	idx := bytes.Index(bspData, []byte("BSPX"))
	if idx < 0 {
		return nil, errNoLump
	}
	if idx+8 > len(bspData) {
		return nil, fmt.Errorf("%w: truncated header", brushListErr)
	}
	numLumps := binary.LittleEndian.Uint32(bspData[idx+4:])
	if numLumps == 0 {
		return nil, errNoLump
	}
	ent := idx + 8
	if ent+32*int(numLumps) > len(bspData) {
		return nil, fmt.Errorf("%w: truncated lump table", brushListErr)
	}
	for i := 0; i < int(numLumps); i++ {
		e := ent + i*32
		name := string(bytes.TrimRight(bspData[e:e+24], "\x00"))
		if name != "BRUSHLIST" {
			continue
		}
		ofs := binary.LittleEndian.Uint32(bspData[e+24:])
		ln := binary.LittleEndian.Uint32(bspData[e+28:])
		if int(ofs)+int(ln) > len(bspData) {
			return nil, fmt.Errorf("%w: brush list out of range", brushListErr)
		}
		return bspData[ofs : ofs+ln], nil
	}
	return nil, errNoLump
}

// BrushListFromBSP locates the appended BRUSHLIST lump and parses it. It
// returns (nil, nil) when the BSP carries no BRUSHLIST lump.
func BrushListFromBSP(bspData []byte) ([]BSPXBrush, error) {
	payload, err := brushListPayload(bspData)
	if err != nil {
		if errors.Is(err, errNoLump) {
			return nil, nil
		}
		return nil, err
	}
	return ParseBrushList(payload)
}

// planePoints returns three non-collinear points on p, spaced by a multiple
// of the 8-unit master lattice so grid snapping preserves the plane. The
// ordering reproduces ericw's PlaneFromPoints convention
// (normal = normalize(cross(p0-p1, p2-p1))), so re-deriving the plane from
// the emitted face points yields the stored outward normal and dist.
func planePoints(p mapfile.Plane) [3]mapfile.Vec3 {
	const span = 64.0
	ax, ay, az := math.Abs(p.Normal.X), math.Abs(p.Normal.Y), math.Abs(p.Normal.Z)
	x := 0
	if ay >= ax && ay >= az {
		x = 1
	}
	if az >= ax && az >= ay {
		x = 2
	}
	up := mapfile.Vec3{}
	switch x {
	case 0, 1:
		up.Z = 1
	default:
		up.X = 1
	}
	// project up onto the plane and normalize
	up = v3Sub(up, v3Scale(p.Normal, v3Dot(up, p.Normal)))
	if l := v3Len(up); l > 0 {
		up = v3Scale(up, 1/l)
	}
	org := v3Scale(p.Normal, p.Dist)
	right := v3Cross(up, p.Normal)
	return [3]mapfile.Vec3{
		org,
		v3Add(org, v3Scale(up, span)),
		v3Add(org, v3Scale(right, span)),
	}
}

// BrushFromBSPX builds a .map brush from a BRUSHLIST brush: one MapFace per
// stored plane, textured with the M1 placeholder mt_wall. The M1 direct
// path has no texture info (BSP texinfo recovery is a later stage), and the
// Valve-220 axes stay zeroed until the texture pass fills them.
func BrushFromBSPX(b BSPXBrush) (mapfile.MapBrush, error) {
	if len(b.Faces) < 4 {
		return mapfile.MapBrush{}, fmt.Errorf("%w: model %d brush has %d faces", brushListErr, b.Model, len(b.Faces))
	}
	var mb mapfile.MapBrush
	for _, pl := range b.Faces {
		pts := planePoints(pl)
		if p, length := mapfile.PlaneFromPoints(pts[0], pts[1], pts[2]); length > 0.01 && v3Dot(p.Normal, pl.Normal) < 0 {
			pts[1], pts[2] = pts[2], pts[1]
		}
		mb.Faces = append(mb.Faces, mapfile.MapFace{
			Points:  pts,
			Normal:  pl.Normal,
			Dist:    pl.Dist,
			TexName: "mt_wall",
			Line:    0,
		})
	}
	return mb, nil
}

// brushFromMapBrush rebuilds the internal halfspace brush from a map brush
// (one side per face); the caller sets Contents afterwards.
func brushFromMapBrush(mb mapfile.MapBrush) *Brush {
	b := &Brush{}
	for _, f := range mb.Faces {
		b.Sides = append(b.Sides, &Side{
			Plane:   mapfile.Plane{Normal: f.Normal, Dist: f.Dist},
			TexName: f.TexName,
			Vecs:    f.Vecs,
		})
	}
	rebuildWindings(b)
	return b
}

// solidLeafCount counts non-empty leaves of the render tree.
func solidLeafCount(tree *bsp.Tree) int {
	n := 0
	for _, l := range tree.Leafs {
		if l.Contents != bsp.ContentsEmpty {
			n++
		}
	}
	return n
}

// decompileFromBrushList is the M1 direct-emission path (spec section 5 step
// 9, BRUSHLIST shortcut): the lump already holds the original brushes, so
// recovery is conversion plus the merge/cleanup stages instead of a BSP
// treewalk. Ents and tree come from the already-parsed BSP in Decompile.
func decompileFromBrushList(ents *mapfile.Map, tree *bsp.Tree, ls []BSPXBrush, opts Options) (*mapfile.Map, []ModelStats, error) {
	maxModel := 0
	for _, b := range ls {
		if b.Model > maxModel {
			maxModel = b.Model
		}
	}
	perModel := make([][]*Brush, maxModel+1)
	stats := make([]ModelStats, 0, maxModel+1)
	for mi := range perModel {
		var brushes []*Brush
		for _, b := range ls {
			if b.Model != mi {
				continue
			}
			mb, err := BrushFromBSPX(b)
			if err != nil {
				return nil, nil, err
			}
			br := brushFromMapBrush(mb)
			br.Contents = b.Contents
			brushes = append(brushes, br)
		}
		if opts.MergeConvex {
			brushes = mergeConvex(brushes)
		}
		// shared cleanup with the treewalk path: dedupe coplanar sides, drop
		// redundant planes and sub-grid degenerate fragments.
		alive := brushes[:0]
		for _, br := range brushes {
			dedupeCoplanarSides(br)
			removeRedundantPlanes(br)
			kept := br.Sides[:0]
			for _, s := range br.Sides {
				if sideSurvivesGrid(s, opts.GridSnap) {
					kept = append(kept, s)
				}
			}
			br.Sides = kept
			if len(br.Sides) >= 4 {
				alive = append(alive, br)
			}
		}
		brushes = alive
		canonicalizeBrush(brushes)
		perModel[mi] = brushes
		stats = append(stats, ModelStats{
			Model:       mi,
			Brushes:     len(brushes),
			LeavesSolid: solidLeafCount(tree),
			PlanesUsed:  planeSetCount(brushes),
		})
	}
	return attachBrushes(ents, perModel, tree), stats, nil
}