package renderer

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/model"
	"github.com/darkliquid/ironwail-go/internal/testutil"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

func TestWaterFaceOverlapDiagnostic(t *testing.T) {
	quakeDir, err := testutil.LocateQuakeDir()
	if err != nil {
		t.Skip("Quake directory not found")
	}

	vfs := fs.NewFileSystem()
	if err := vfs.AddGameDirectory(filepath.Join(quakeDir, "id1")); err != nil {
		t.Skipf("AddGameDirectory id1 failed: %v", err)
	}
	if err := vfs.AddGameDirectory(filepath.Join(quakeDir, "qbj2")); err != nil {
		t.Skipf("AddGameDirectory qbj2 failed: %v", err)
	}

	data, err := vfs.LoadFile("maps/start.bsp")
	if err != nil {
		t.Skipf("Failed to read maps/start.bsp: %v", err)
	}

	tree, err := bsp.LoadTree(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Failed to parse start.bsp: %v", err)
	}

	geom, err := BuildWorldGeometry(tree)
	if err != nil {
		t.Fatalf("BuildWorldGeometry failed: %v", err)
	}

	camPos := types.Vec3{X: -231.38, Y: -1768.12, Z: -2114.00}
	visibleFaces := selectVisibleWorldFaces(tree, geom.Faces, geom.LeafFaces, camPos)

	type faceInfo struct {
		idx     int
		center  types.Vec3
		normal  types.Vec3
		indices uint32
	}
	var turbFaces []faceInfo
	for i, f := range visibleFaces {
		if f.Flags&model.SurfDrawTurb != 0 && f.NumIndices >= 3 {
			i0 := geom.Indices[f.FirstIndex]
			i1 := geom.Indices[f.FirstIndex+1]
			i2 := geom.Indices[f.FirstIndex+2]
			v0 := geom.Vertices[i0].Position
			v1 := geom.Vertices[i1].Position
			v2 := geom.Vertices[i2].Position
			e1 := types.Vec3{X: v1.X - v0.X, Y: v1.Y - v0.Y, Z: v1.Z - v0.Z}
			e2 := types.Vec3{X: v2.X - v0.X, Y: v2.Y - v0.Y, Z: v2.Z - v0.Z}
			norm := types.Vec3Cross(e1, e2).Normalize()
			turbFaces = append(turbFaces, faceInfo{
				idx:     i,
				center:  f.Center,
				normal:  norm,
				indices: f.NumIndices,
			})
		}
	}
	t.Logf("Found %d turb faces visible from %v", len(turbFaces), camPos)

	// Check how many have normal Z > 0 vs normal Z < 0 vs normal Z == 0
	var up, down, side int
	for _, f := range turbFaces {
		if f.normal.Z > 0.5 {
			up++
		} else if f.normal.Z < -0.5 {
			down++
		} else {
			side++
		}
	}
	t.Logf("Normals: up=%d, down=%d, side=%d", up, down, side)

	// Project faces to screen using camera view-projection matrix
	// PARITY_POS="-231.38 -1768.12 -2114.00" PARITY_ANGLES="29.41 99.98 0.00"
	// Let's create the view matrix and projection matrix
	angles := types.Vec3{X: 29.41, Y: 99.98, Z: 0.00}
	camera := CameraState{Origin: camPos, Angles: angles, FOV: 90}
	viewMat := ComputeViewMatrix(camera)
	projMat := ComputeProjectionMatrix(90.0, 1280.0/720.0, 4.0, 4096.0)
	vpMat := types.Mat4Multiply(projMat, viewMat)

	type vec2 struct{ X, Y float32 }
	type tri2D struct {
		p0, p1, p2 vec2
		faceIdx    int
	}
	transform := func(m types.Mat4, v types.Vec3) (float32, float32, float32, float32) {
		x := m[0]*v.X + m[4]*v.Y + m[8]*v.Z + m[12]
		y := m[1]*v.X + m[5]*v.Y + m[9]*v.Z + m[13]
		z := m[2]*v.X + m[6]*v.Y + m[10]*v.Z + m[14]
		w := m[3]*v.X + m[7]*v.Y + m[11]*v.Z + m[15]
		return x, y, z, w
	}

	var screenTris []tri2D
	for i, f := range visibleFaces {
		if f.Flags&model.SurfDrawTurb == 0 || f.NumIndices < 3 {
			continue
		}
		for idx := uint32(0); idx < f.NumIndices; idx += 3 {
			i0 := geom.Indices[f.FirstIndex+idx]
			i1 := geom.Indices[f.FirstIndex+idx+1]
			i2 := geom.Indices[f.FirstIndex+idx+2]
			v0 := geom.Vertices[i0].Position
			v1 := geom.Vertices[i1].Position
			v2 := geom.Vertices[i2].Position

			x0, y0, _, w0 := transform(vpMat, v0)
			x1, y1, _, w1 := transform(vpMat, v1)
			x2, y2, _, w2 := transform(vpMat, v2)

			// Check if in front of camera
			if w0 <= 0 || w1 <= 0 || w2 <= 0 {
				continue
			}
			ndc0 := vec2{X: x0 / w0, Y: y0 / w0}
			ndc1 := vec2{X: x1 / w1, Y: y1 / w1}
			ndc2 := vec2{X: x2 / w2, Y: y2 / w2}

			// Check winding in screen space (CCW vs CW)
			area := (ndc1.X-ndc0.X)*(ndc2.Y-ndc0.Y) - (ndc1.Y-ndc0.Y)*(ndc2.X-ndc0.X)
			// FrontFaceCCW + CullModeFront culls CCW (area > 0), keeps CW (area < 0)
			if area >= 0 {
				continue // Culled!
			}

			screenTris = append(screenTris, tri2D{p0: ndc0, p1: ndc1, p2: ndc2, faceIdx: i})
		}
	}
	t.Logf("Total un-culled screen triangles: %d", len(screenTris))

	// Point in triangle test for screen center (0, -0.2)
	pointInTri := func(pt, v0, v1, v2 vec2) bool {
		d1 := (pt.X-v1.X)*(v0.Y-v1.Y) - (v0.X-v1.X)*(pt.Y-v1.Y)
		d2 := (pt.X-v2.X)*(v1.Y-v2.Y) - (v1.X-v2.X)*(pt.Y-v2.Y)
		d3 := (pt.X-v0.X)*(v2.Y-v0.Y) - (v2.X-v0.X)*(pt.Y-v0.Y)
		hasNeg := (d1 < 0) || (d2 < 0) || (d3 < 0)
		hasPos := (d1 > 0) || (d2 > 0) || (d3 > 0)
		return !hasNeg || !hasPos
	}

	overlapping := 0
	for _, tri := range screenTris {
		if pointInTri(vec2{X: 0, Y: -0.2}, tri.p0, tri.p1, tri.p2) {
			overlapping++
		}
	}
	t.Logf("Overlapping triangles at screen (0, -0.2): %d", overlapping)
}
