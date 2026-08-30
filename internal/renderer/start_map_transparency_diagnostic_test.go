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

	t.Logf("=== Camera Leaf and PVS at Nightmare Pool ===")
	camPos := types.Vec3{X: 1456, Y: 1720, Z: 120}
	camLeaf := tree.PointInLeaf(camPos)
	camLeafIdx := -1
	for i := range tree.Leafs {
		if &tree.Leafs[i] == camLeaf {
			camLeafIdx = i
			break
		}
	}
	t.Logf("Camera %v -> Leaf %d (contents=%d)", camPos, camLeafIdx, camLeaf.Contents)
	pvs := tree.LeafPVS(camLeaf)
	fatPVS := tree.FatPVS(camPos)
	t.Logf("Leaf %d PVS has %d bytes, FatPVS has %d bytes", camLeafIdx, len(pvs), len(fatPVS))

	// Check if leaf 263 or other submerged leaves are in PVS
	for leafIdx := range tree.Leafs {
		byteIdx := leafIdx / 8
		bitIdx := uint(leafIdx % 8)
		inPVS := byteIdx < len(pvs) && (pvs[byteIdx]&(1<<bitIdx)) != 0
		inFat := byteIdx < len(fatPVS) && (fatPVS[byteIdx]&(1<<bitIdx)) != 0
		if inPVS || inFat || (leafIdx >= 260 && leafIdx <= 270) {
			t.Logf("Leaf %d (contents=%d, numFaces=%d): inPVS=%v, inFatPVS=%v",
				leafIdx, tree.Leafs[leafIdx].Contents, len(geom.LeafFaces[leafIdx]), inPVS, inFat)
		}
	}

	visibleFaces := selectVisibleWorldFaces(tree, geom.Faces, geom.LeafFaces, camPos)
	t.Logf("selectVisibleWorldFaces returned %d visible faces", len(visibleFaces))
	for _, targetFaceIdx := range []int{762, 763, 764} {
		var leafList []int
		for leafIdx, faces := range geom.LeafFaces {
			for _, fIdx := range faces {
				if fIdx == targetFaceIdx {
					leafList = append(leafList, leafIdx)
				}
			}
		}
		isVisible := false
		for _, f := range visibleFaces {
			if f.TextureIndex == geom.Faces[targetFaceIdx].TextureIndex && f.Center == geom.Faces[targetFaceIdx].Center {
				isVisible = true
				break
			}
		}
		t.Logf("Submerged Wall Face #%d (texture=%d, center=%v): present in leafs %v, isVisible=%v",
			targetFaceIdx, geom.Faces[targetFaceIdx].TextureIndex, geom.Faces[targetFaceIdx].Center, leafList, isVisible)
	}

	t.Logf("=== Visible Turb Faces and Normals ===")
	for _, f := range visibleFaces {
		if f.Flags&model.SurfDrawTurb != 0 {
			vert := geom.Vertices[geom.Indices[f.FirstIndex]]
			t.Logf("Turb Face #%d at center %v, normal=(%.2f, %.2f, %.2f), numIndices=%d",
				f.FirstIndex, f.Center, vert.Normal.X, vert.Normal.Y, vert.Normal.Z, f.NumIndices)
		}
	}
}
