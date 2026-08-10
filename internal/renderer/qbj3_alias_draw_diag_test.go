package renderer

import (
	"bytes"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/model"
	aliasimpl "github.com/darkliquid/ironwail-go/internal/renderer/alias"
	"github.com/darkliquid/ironwail-go/internal/testutil"
)

// TestQbj3AliasDrawPipeline builds the exact vertex data the GPU shader would
// consume for the qbj3 weapon model (progs/v_wrench.mdl) and the keycard
// (progs/b_s_key.mdl), using the real model files. This exercises
// SetupAliasFrame + buildAliasVerticesInterpolatedInto — the pure path
// between the render collectors and the GPU buffer. If pose range or vertex
// building silently failed, the weapon/keycard would be invisible even though
// the collect layer logs "draw".
func TestQbj3AliasDrawPipeline(t *testing.T) {
	quakeDir := testutil.SkipIfNoQuakeDir(t)

	vfs := fs.NewFileSystem()
	if err := vfs.Init(quakeDir, "qbj3"); err != nil {
		t.Fatalf("Init(qbj3): %v", err)
	}
	t.Cleanup(func() { vfs.Close() })

	for _, tc := range []struct {
		name string
		mdl  string
	}{
		{"v_wrench", "progs/v_wrench.mdl"},
		{"b_s_key", "progs/b_s_key.mdl"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := vfs.LoadFile(tc.mdl)
			if err != nil {
				t.Fatalf("LoadFile(%s): %v", tc.mdl, err)
			}
			m, err := model.LoadAliasModel(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("LoadAliasModel(%s): %v", tc.mdl, err)
			}
			hdr := m.AliasHeader
			if hdr == nil || hdr.NumFrames < 1 {
				t.Fatalf("%s: empty alias header", tc.mdl)
			}

			alias := &gpuAliasModel{modelID: tc.mdl}
			for _, f := range hdr.Frames {
				for p := 0; p < f.NumPoses; p++ {
					idx := f.FirstPose + p
					if idx < 0 || idx >= len(hdr.Poses) {
						t.Fatalf("%s: frame pose idx %d out of range (len=%d)", tc.mdl, idx, len(hdr.Poses))
					}
				}
			}
			alias.poses = hdr.Poses
			// Build mesh refs exactly as ensureAliasModelLocked does.
			for _, tri := range hdr.Triangles {
				for vertexIndex := 0; vertexIndex < 3; vertexIndex++ {
					idx := int(tri.VertIndex[vertexIndex])
					if idx < 0 || idx >= len(hdr.STVerts) {
						t.Fatalf("%s: triangle vertex idx %d out of STVerts range (%d)", tc.mdl, idx, len(hdr.STVerts))
					}
					st := hdr.STVerts[idx]
					s := float32(st.S) + 0.5
					if tri.FacesFront == 0 && st.OnSeam != 0 {
						s += float32(hdr.SkinWidth) * 0.5
					}
					alias.refs = append(alias.refs, aliasimpl.MeshRef{
						VertexIndex: idx,
						TexCoord: [2]float32{
							s / float32(hdr.SkinWidth),
							(float32(st.T) + 0.5) / float32(hdr.SkinHeight),
						},
					})
				}
			}
			if len(alias.refs) == 0 {
				t.Fatalf("%s: no mesh refs built", tc.mdl)
			}

			// Interpolate every frame (skipping to later frames to
			// exercise pose cycling), then build vertices.
			scratch := make([]WorldVertex, 0, len(alias.refs))
			for frame := 0; frame < hdr.NumFrames; frame += max(1, hdr.NumFrames/8) {
				state := &AliasEntity{Frame: frame}
				lerp, err := SetupAliasFrame(state, aliasHeaderFromModel(hdr), 0.5, true, false, 1)
				if err != nil {
					t.Fatalf("%s frame %d: SetupAliasFrame: %v", tc.mdl, frame, err)
				}
				if lerp.Pose1 < 0 || lerp.Pose1 >= len(alias.poses) ||
					lerp.Pose2 < 0 || lerp.Pose2 >= len(alias.poses) {
					t.Fatalf("%s frame %d: pose range (%d,%d), poses=%d",
						tc.mdl, frame, lerp.Pose1, lerp.Pose2, len(alias.poses))
				}
				verts := buildAliasVerticesInterpolatedInto(scratch[:0], alias, m,
					lerp.Pose1, lerp.Pose2, lerp.Blend,
					[3]float32{0, 0, 0}, [3]float32{0, 0, 0}, 1, false)
				if len(verts) == 0 {
					t.Fatalf("%s frame %d: buildAliasVerticesInterpolatedInto returned no vertices", tc.mdl, frame)
				}
			}
			t.Logf("%s: OK poses=%d refs=%d frames=%d",
				tc.mdl, len(alias.poses), len(alias.refs), hdr.NumFrames)
		})
	}
}

// TestQbj3AliasSkinsValid verifies each qbj3 model has a non-empty, fully
// mapped skin so createAliasSkinLocked has valid input in the GPU path.
func TestQbj3AliasSkinsValid(t *testing.T) {
	quakeDir := testutil.SkipIfNoQuakeDir(t)
	vfs := fs.NewFileSystem()
	if err := vfs.Init(quakeDir, "qbj3"); err != nil {
		t.Fatalf("Init(qbj3): %v", err)
	}
	t.Cleanup(func() { vfs.Close() })

	for _, mdl := range []string{"progs/v_wrench.mdl", "progs/b_s_key.mdl"} {
		t.Run(mdl, func(t *testing.T) {
			data, err := vfs.LoadFile(mdl)
			if err != nil {
				t.Fatalf("LoadFile: %v", err)
			}
			m, err := model.LoadAliasModel(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("LoadAliasModel: %v", err)
			}
			hdr := m.AliasHeader
			for i, skin := range hdr.Skins {
				if len(skin) != int(hdr.SkinWidth)*int(hdr.SkinHeight) {
					t.Fatalf("skin %d: len=%d want %dx%d", i, len(skin), hdr.SkinWidth, hdr.SkinHeight)
				}
			}
			t.Logf("%s: skins=%d %dx%d", mdl, len(hdr.Skins), hdr.SkinWidth, hdr.SkinHeight)
		})
	}
}
