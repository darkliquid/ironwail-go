package renderer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
	"github.com/darkliquid/ironwail-go/internal/testutil"
	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
)

func TestQBJ2SpawnLeafWaterPortal(t *testing.T) {
	quakeDir, err := testutil.LocateQuakeDir()
	if err != nil || quakeDir == "" {
		t.Skip("QUAKE_DIR not set")
	}
	bspPath := filepath.Join(quakeDir, "qbj2", "maps", "start.bsp")
	f, err := os.Open(bspPath)
	if err != nil {
		t.Skipf("qbj2 start.bsp not found: %v", err)
	}
	defer f.Close()

	data, err := bsp.LoadTree(f)
	testutil.AssertNoError(t, err)

	textureMeta := worldimpl.ParseTextureMeta(data)
	
	// Spawn position
	origin := [3]float32{-256, -2576, -2152}
	leaf := data.PointInLeaf(origin)
	if leaf == nil {
		t.Fatal("nil leaf")
	}
	
	leafIdx := -1
	for i := range data.Leafs {
		if &data.Leafs[i] == leaf {
			leafIdx = i
			break
		}
	}
	
	t.Logf("Leaf index: %d, contents: %d", leafIdx, leaf.Contents)
	t.Logf("FirstMarkSurface: %d, NumMarkSurfaces: %d", leaf.FirstMarkSurface, leaf.NumMarkSurfaces)
	
	waterCount := 0
	totalFaces := 0
	for i := uint32(0); i < leaf.NumMarkSurfaces; i++ {
		msIdx := leaf.FirstMarkSurface + i
		if int(msIdx) >= len(data.MarkSurfaces) {
			break
		}
		faceIdx := data.MarkSurfaces[msIdx]
		if int(faceIdx) >= len(data.Faces) {
			continue
		}
		face := &data.Faces[faceIdx]
		if int(face.Texinfo) >= len(data.Texinfo) {
			continue
		}
		ti := &data.Texinfo[face.Texinfo]
		textureType := model.TexTypeDefault
		var name string
		if int(ti.Miptex) >= 0 && int(ti.Miptex) < len(textureMeta) {
			textureType = textureMeta[ti.Miptex].Type
			name = textureMeta[ti.Miptex].Name
		}
		flags := worldimpl.DeriveFaceFlags(textureType, ti.Flags)
		totalFaces++
		if flags&model.SurfDrawTurb != 0 {
			waterCount++
			t.Logf("  Water face: idx=%d name=%s flags=%d", faceIdx, name, flags)
		}
	}
	t.Logf("Total marksurfaces: %d, water faces: %d", totalFaces, waterCount)
}
