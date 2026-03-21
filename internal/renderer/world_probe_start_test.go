//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/testutil"
)

func loadStartTreeForWorldProbeTest(t *testing.T) *bsp.Tree {
	t.Helper()
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	vfs := fs.NewFileSystem()
	if err := vfs.Init(baseDir, "id1"); err != nil {
		t.Fatalf("init filesystem: %v", err)
	}
	t.Cleanup(vfs.Close)

	data, err := vfs.LoadFile("maps/start.bsp")
	if err != nil {
		t.Fatalf("load start.bsp: %v", err)
	}
	tree, err := bsp.LoadTree(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse start.bsp: %v", err)
	}
	return tree
}

func TestProbeWorldFacesInBBox_StartSecondChamberFloor(t *testing.T) {
	tree := loadStartTreeForWorldProbeTest(t)

	// Suspect second-chamber bbox selected from start.bsp face-center inspection.
	// This isolates the three city4_7 floor patches centered near:
	// (432.0, 698.7, -64.0), (589.7, 636.6, -64.0), and (853.8, 617.5, -64.0).
	suspectMin := [3]float32{300, 560, -96}
	suspectMax := [3]float32{860, 720, 16}
	stats, err := ProbeWorldFacesInBBox(tree, 0, suspectMin, suspectMax, worldLiquidAlphaSettings{
		water: 1,
		lava:  1,
		slime: 1,
		tele:  1,
	})
	if err != nil {
		t.Fatalf("ProbeWorldFacesInBBox failed: %v", err)
	}

	floorRaw := 0
	floorExtracted := 0
	floorOpaque := 0
	for _, face := range stats.Faces {
		if face.Normal[2] < 0.95 {
			continue
		}
		if face.Center[2] < -64 || face.Center[2] > 16 {
			continue
		}
		floorRaw++
		if face.Extracted {
			floorExtracted++
			if face.Pass == worldPassOpaque {
				floorOpaque++
			}
		}
	}

	t.Logf("start.bsp suspect bbox min=%v max=%v raw=%d extracted=%d passes={sky:%d opaque:%d alpha:%d translucent:%d} floorRaw=%d floorExtracted=%d floorOpaque=%d",
		suspectMin, suspectMax,
		stats.RawInBBox, stats.ExtractedInBBox,
		stats.PassSky, stats.PassOpaque, stats.PassAlphaTest, stats.PassTranslucent,
		floorRaw, floorExtracted, floorOpaque)
	for i, face := range stats.Faces {
		if i >= 12 {
			break
		}
		t.Logf("face[%d] src=%d tex=%d texname=%q texFlags=%#x derivedFlags=%#x center=(%.1f %.1f %.1f) normal=(%.2f %.2f %.2f) extracted=%v verts=%d pass=%s err=%q",
			i, face.SourceFaceIndex, face.TextureIndex, face.TextureName, face.TexInfoFlags, face.DerivedFlags,
			face.Center[0], face.Center[1], face.Center[2],
			face.Normal[0], face.Normal[1], face.Normal[2],
			face.Extracted, face.VertexCount, worldRenderPassName(face.Pass), face.ExtractError)
	}

	if stats.RawInBBox == 0 {
		t.Fatalf("expected suspect bbox to contain raw BSP faces")
	}
	if stats.ExtractedInBBox == 0 {
		t.Fatalf("expected suspect bbox to contain extracted faces")
	}
	if floorRaw == 0 {
		t.Fatalf("expected suspect bbox to contain at least one floor-like raw face")
	}
	if floorExtracted == 0 {
		t.Fatalf("expected at least one floor-like face to survive extraction")
	}
}
