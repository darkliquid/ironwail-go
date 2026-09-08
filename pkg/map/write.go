package mapfile

import (
	"fmt"
	"io"
	"math"
	"strconv"
)

// WriteOptions controls .map emission.
type WriteOptions struct {
	// GridSnap quantizes emitted plane points to multiples of N (0 = off).
	// Mappers work on integer lattices; 8 is the decompiler default.
	GridSnap int
}

// Write emits m as a Quake .map: entity blocks in order, epairs first (file
// order preserved), then brushes as Valve 220 face lines.
//
// Where in C: ericw-tools common/mapfile.cc token conventions; Valve 220
// face form per ericw qbsp docs.
func Write(w io.Writer, m *Map, opts WriteOptions) error {
	for i := range m.Entities {
		if err := writeEntity(w, &m.Entities[i], opts); err != nil {
			return err
		}
	}
	return nil
}

func writeEntity(w io.Writer, e *Entity, opts WriteOptions) error {
	if _, err := fmt.Fprintln(w, "{"); err != nil {
		return err
	}
	for _, p := range e.Epairs {
		// Raw quoting, not %q: Quake text may carry high-bit glyph bytes
		// which Go escapes would corrupt (see AGENTS.md console-text note).
		if _, err := fmt.Fprintf(w, "\"%s\" \"%s\"\n", p.Key, p.Value); err != nil {
			return err
		}
	}
	for i := range e.Brushes {
		if err := writeBrush(w, &e.Brushes[i], opts); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w, "}")
	return err
}

func writeBrush(w io.Writer, b *MapBrush, opts WriteOptions) error {
	if _, err := fmt.Fprintln(w, "{"); err != nil {
		return err
	}
	for i := range b.Faces {
		if err := writeFace(w, &b.Faces[i], opts); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w, "}")
	return err
}

// writeFace emits one Valve 220 face line:
//
//	( p0 ) ( p1 ) ( p2 ) tex [ ux uy uz uoff ] [ vx vy vz voff ] rot sx sy
//
// Axes come from the computed Vecs so both QuakeEd- and Valve-parsed faces
// emit a single canonical form (rotation/scale already folded into Vecs).
func writeFace(w io.Writer, f *MapFace, opts WriteOptions) error {
	pts := f.Points
	if opts.GridSnap > 0 {
		pts = snapPoints(pts, opts.GridSnap)
	}
	u := Vec3{X: f.Vecs[0][0], Y: f.Vecs[0][1], Z: f.Vecs[0][2]}
	v := Vec3{X: f.Vecs[1][0], Y: f.Vecs[1][1], Z: f.Vecs[1][2]}
	_, err := fmt.Fprintf(w, "( %s ) ( %s ) ( %s ) %s [ %s %s ] [ %s %s ] 0 1 1\n",
		fmtPoint(pts[0]), fmtPoint(pts[1]), fmtPoint(pts[2]),
		f.TexName,
		fmtPoint(u), fmtNum(f.Vecs[0][3]),
		fmtPoint(v), fmtNum(f.Vecs[1][3]))
	return err
}

func snapPoints(pts [3]Vec3, n int) [3]Vec3 {
	var out [3]Vec3
	f := float64(n)
	round := func(v float64) float64 { return math.Round(v/f) * f }
	for i, p := range pts {
		out[i] = Vec3{X: round(p.X), Y: round(p.Y), Z: round(p.Z)}
	}
	return out
}

func fmtPoint(p Vec3) string {
	return fmtNum(p.X) + " " + fmtNum(p.Y) + " " + fmtNum(p.Z)
}

// fmtNum renders integers without a decimal point (mapper convention) and
// everything else in shortest round-trip form.
func fmtNum(v float64) string {
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}