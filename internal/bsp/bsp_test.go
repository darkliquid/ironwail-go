package bsp

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/testutil"
)

func TestLoadTreeFromPak0(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	vfs := fs.NewFileSystem()
	err := vfs.Init(baseDir, "id1")
	testutil.AssertNoError(t, err)
	defer vfs.Close()

	data, err := vfs.LoadFile("maps/e1m1.bsp")
	testutil.AssertNoError(t, err)

	tree, err := LoadTree(bytes.NewReader(data))
	testutil.AssertNoError(t, err)

	if len(tree.Entities) == 0 {
		t.Fatal("entities lump is empty")
	}
	if !bytes.Contains(tree.Entities, []byte("\"classname\" \"worldspawn\"")) {
		t.Fatal("worldspawn entity not found")
	}

	if len(tree.Planes) == 0 {
		t.Fatal("planes not loaded")
	}
	if len(tree.Texinfo) == 0 {
		t.Fatal("texinfo not loaded")
	}
	if len(tree.Vertexes) == 0 {
		t.Fatal("vertexes not loaded")
	}
	if len(tree.TextureData) == 0 {
		t.Fatal("texture data not loaded")
	}
	if tree.NumTextures <= 0 {
		t.Fatalf("num textures = %d, want > 0", tree.NumTextures)
	}
	if len(tree.Edges) == 0 {
		t.Fatal("edges not loaded")
	}
	if len(tree.Surfedges) == 0 {
		t.Fatal("surfedges not loaded")
	}
	if len(tree.Faces) == 0 {
		t.Fatal("faces not loaded")
	}
	if len(tree.MarkSurfaces) == 0 {
		t.Fatal("marksurfaces not loaded")
	}
	if len(tree.Leafs) == 0 {
		t.Fatal("leafs not loaded")
	}
	if len(tree.Nodes) == 0 {
		t.Fatal("nodes not loaded")
	}
	if len(tree.Models) == 0 {
		t.Fatal("models not loaded")
	}

	if tree.Nodes[0].Parent != -1 {
		t.Fatalf("root node parent = %d, want -1", tree.Nodes[0].Parent)
	}

	for i, n := range tree.Nodes {
		if n.PlaneNum < 0 || int(n.PlaneNum) >= len(tree.Planes) {
			t.Fatalf("node %d has invalid plane index %d", i, n.PlaneNum)
		}
		for childSide, child := range n.Children {
			if child.IsLeaf {
				if child.Index < 0 || child.Index >= len(tree.Leafs) {
					t.Fatalf("node %d child %d leaf index out of bounds: %d", i, childSide, child.Index)
				}
				// Leaf 0 is the special solid outside leaf shared by many nodes;
				// skip the unique-parent check for it.
				if child.Index != 0 && tree.Leafs[child.Index].Parent != i {
					t.Fatalf("leaf %d parent = %d, want %d", child.Index, tree.Leafs[child.Index].Parent, i)
				}
				continue
			}
			if child.Index < 0 || child.Index >= len(tree.Nodes) {
				t.Fatalf("node %d child %d node index out of bounds: %d", i, childSide, child.Index)
			}
			if child.Index != 0 && tree.Nodes[child.Index].Parent != i {
				t.Fatalf("node %d parent = %d, want %d", child.Index, tree.Nodes[child.Index].Parent, i)
			}
		}
	}

	for i, leaf := range tree.Leafs {
		if leaf.NumMarkSurfaces == 0 {
			continue
		}
		start := int(leaf.FirstMarkSurface)
		end := start + int(leaf.NumMarkSurfaces)
		if start < 0 || end > len(tree.MarkSurfaces) {
			t.Fatalf("leaf %d marksurface range [%d:%d] out of bounds %d", i, start, end, len(tree.MarkSurfaces))
		}
	}

	world := tree.Models[0]
	if world.NumFaces <= 0 {
		t.Fatalf("world model has invalid face count %d", world.NumFaces)
	}
	if world.FirstFace < 0 || int(world.FirstFace+world.NumFaces) > len(tree.Faces) {
		t.Fatalf("world model face range [%d:%d] out of bounds %d", world.FirstFace, world.FirstFace+world.NumFaces, len(tree.Faces))
	}
}

func TestLoadFromPak0(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	vfs := fs.NewFileSystem()
	err := vfs.Init(baseDir, "id1")
	testutil.AssertNoError(t, err)
	defer vfs.Close()

	data, err := vfs.LoadFile("maps/e1m1.bsp")
	testutil.AssertNoError(t, err)

	file, err := Load(bytes.NewReader(data))
	testutil.AssertNoError(t, err)

	nodes, ok := file.Nodes.([]DSNode)
	if !ok {
		t.Fatalf("nodes type = %T, want []DSNode", file.Nodes)
	}
	if len(nodes) == 0 {
		t.Fatal("nodes not loaded")
	}
	if nodes[0].NumFaces == 0 {
		t.Fatal("first node NumFaces = 0, want > 0")
	}
	if int(nodes[0].PlaneNum) >= len(file.Planes) || nodes[0].PlaneNum < 0 {
		t.Fatalf("first node plane index = %d, want within [0,%d)", nodes[0].PlaneNum, len(file.Planes))
	}
}

func TestDecompressVisUsesVisLeafCount(t *testing.T) {
	tree := &Tree{
		Leafs:  make([]TreeLeaf, 5), // includes solid leaf 0
		Models: []DModel{{VisLeafs: 4}},
	}

	out := tree.DecompressVis([]byte{0xFF})
	if len(out) != 1 {
		t.Fatalf("decompressed length = %d, want 1 byte for 4 visleafs", len(out))
	}
}

func TestLeafPVSAllVisibleUsesVisLeafCount(t *testing.T) {
	tree := &Tree{
		Leafs:  make([]TreeLeaf, 5), // includes solid leaf 0
		Models: []DModel{{VisLeafs: 4}},
	}

	pvs := tree.LeafPVS(nil)
	if len(pvs) != 1 {
		t.Fatalf("all-visible PVS length = %d, want 1 byte for 4 visleafs", len(pvs))
	}
	if pvs[0] != 0xFF {
		t.Fatalf("all-visible PVS byte = 0x%02x, want 0xFF", pvs[0])
	}
}
