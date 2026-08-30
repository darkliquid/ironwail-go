package renderer

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/model"
	"github.com/darkliquid/ironwail-go/internal/testutil"
)

func TestStartMapTransparencyDiagnostics(t *testing.T) {
	quakeDir, err := testutil.LocateQuakeDir()
	if err != nil {
		t.Skip("Quake directory not found")
	}

	vfs := fs.NewFileSystem()
	if err := vfs.AddGameDirectory(filepath.Join(quakeDir, "id1")); err != nil {
		t.Skipf("AddGameDirectory failed: %v", err)
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

	liquidAlpha := worldLiquidAlphaSettingsForGeometry(geom)
	t.Logf("liquidAlpha: Water=%v, Slime=%v, Lava=%v, Tele=%v, Safe=%v", liquidAlpha.water, liquidAlpha.slime, liquidAlpha.lava, liquidAlpha.tele, geom.TransparentWaterSafe)

	t.Logf("=== All Water Faces in Map ===")
	for leafIdx, faces := range geom.LeafFaces {
		for _, faceIdx := range faces {
			face := geom.Faces[faceIdx]
			if face.Flags&model.SurfDrawTurb != 0 {
				vert := geom.Vertices[geom.Indices[face.FirstIndex]]
				t.Logf("Leaf %d (contents=%d) has Water Face #%d: center=%v, normal=(%.2f, %.2f, %.2f), tex=%d",
					leafIdx, tree.Leafs[leafIdx].Contents, faceIdx, face.Center, vert.Normal.X, vert.Normal.Y, vert.Normal.Z, face.TextureIndex)
			}
		}
	}
}
