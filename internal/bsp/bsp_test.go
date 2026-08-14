package bsp

import (
	"bytes"
	"encoding/binary"
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/engine/arena"
	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/testutil"
)

func putI32(dst []byte, off int, v int32) {
	binary.LittleEndian.PutUint32(dst[off:], uint32(v))
}

func putU32(dst []byte, off int, v uint32) {
	binary.LittleEndian.PutUint32(dst[off:], v)
}

func putI16(dst []byte, off int, v int16) {
	binary.LittleEndian.PutUint16(dst[off:], uint16(v))
}

func putF32(dst []byte, off int, bits uint32) {
	binary.LittleEndian.PutUint32(dst[off:], bits)
}

// TestLoadTreeFromPak0 tests the loading of a complete BSP tree from a PAK file.
// It ensures the engine can correctly parse all BSP lumps (planes, nodes, leafs, faces, textures, etc.) from real game assets.
// Where in C: Mod_LoadBSP in model.c
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

// TestLoadFromPak0 tests the low-level BSP file loader.
// It verifies the basic structure and header parsing of the BSP format.
// Where in C: Mod_LoadBSP in model.c
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

// TestLoadBSP2FileLumpStrides protects the low-level File loader against
// silently misaligning BSP2_BSP2 structures. The original C dmodel_t is 64
// bytes, dl2node_t is 44 bytes, and dl2leaf_t is 44 bytes; custom maps such
// as qbj2/start.bsp rely on these exact strides.
func TestLoadBSP2FileLumpStrides(t *testing.T) {
	t.Run("nodes", func(t *testing.T) {
		data := make([]byte, dl2NodeSize*2)
		putI32(data, 0, 11)
		putI32(data, 4, 1)
		putI32(data, 8, -1)
		putF32(data, 12, 0x3f800000)
		putF32(data, 24, 0x40000000)
		putU32(data, 36, 7)
		putU32(data, 40, 3)
		second := dl2NodeSize
		putI32(data, second, 22)
		putI32(data, second+4, -1)
		putI32(data, second+8, -2)
		putU32(data, second+36, 13)
		putU32(data, second+40, 5)

		f := &File{Version: BSP2Version_BSP2, IsBSP2: true}
		f.Header.Lumps[LumpNodes] = Lump{FileLength: int32(len(data))}
		if err := f.loadNodes(NewReader(bytes.NewReader(data)), nil); err != nil {
			t.Fatalf("loadNodes: %v", err)
		}
		nodes, ok := f.Nodes.([]DL2Node)
		if !ok {
			t.Fatalf("nodes type = %T, want []DL2Node", f.Nodes)
		}
		if len(nodes) != 2 {
			t.Fatalf("node count = %d, want 2", len(nodes))
		}
		if nodes[0].FirstFace != 7 || nodes[0].NumFaces != 3 ||
			nodes[1].PlaneNum != 22 || nodes[1].FirstFace != 13 || nodes[1].NumFaces != 5 {
			t.Fatalf("decoded nodes = %+v", nodes)
		}
	})

	t.Run("leafs", func(t *testing.T) {
		data := make([]byte, dl2LeafSize*2)
		putI32(data, 0, ContentsEmpty)
		putI32(data, 4, 100)
		putF32(data, 8, 0x3f800000)
		putF32(data, 20, 0x40000000)
		putU32(data, 32, 9)
		putU32(data, 36, 4)
		copy(data[40:44], []byte{1, 2, 3, 4})
		second := dl2LeafSize
		putI32(data, second, ContentsWater)
		putI32(data, second+4, 200)
		putU32(data, second+32, 19)
		putU32(data, second+36, 6)
		copy(data[second+40:second+44], []byte{5, 6, 7, 8})

		f := &File{Version: BSP2Version_BSP2, IsBSP2: true}
		f.Header.Lumps[LumpLeafs] = Lump{FileLength: int32(len(data))}
		if err := f.loadLeafs(NewReader(bytes.NewReader(data)), nil); err != nil {
			t.Fatalf("loadLeafs: %v", err)
		}
		leafs, ok := f.Leafs.([]DL2Leaf)
		if !ok {
			t.Fatalf("leafs type = %T, want []DL2Leaf", f.Leafs)
		}
		if len(leafs) != 2 {
			t.Fatalf("leaf count = %d, want 2", len(leafs))
		}
		if leafs[0].FirstMarkSurface != 9 || leafs[0].NumMarkSurfaces != 4 ||
			leafs[0].AmbientLevel != [NumAmbients]uint8{1, 2, 3, 4} ||
			leafs[1].Contents != ContentsWater || leafs[1].FirstMarkSurface != 19 ||
			leafs[1].AmbientLevel != [NumAmbients]uint8{5, 6, 7, 8} {
			t.Fatalf("decoded leafs = %+v", leafs)
		}
	})

	t.Run("models", func(t *testing.T) {
		data := make([]byte, dModelSize*2)
		putF32(data, 0, 0x3f800000)
		putF32(data, 12, 0x40000000)
		putF32(data, 24, 0x40400000)
		putI32(data, 36, 1)
		putI32(data, 40, 2)
		putI32(data, 44, 3)
		putI32(data, 48, 4)
		putI32(data, 52, 5)
		putI32(data, 56, 6)
		putI32(data, 60, 7)
		second := dModelSize
		putF32(data, second, 0x40800000)
		putI32(data, second+36, 11)
		putI32(data, second+52, 15)
		putI32(data, second+56, 16)
		putI32(data, second+60, 17)

		f := &File{}
		f.Header.Lumps[LumpModels] = Lump{FileLength: int32(len(data))}
		if err := f.loadModels(NewReader(bytes.NewReader(data)), nil); err != nil {
			t.Fatalf("loadModels: %v", err)
		}
		if len(f.Models) != 2 {
			t.Fatalf("model count = %d, want 2", len(f.Models))
		}
		if f.Models[0].HeadNode != [MaxMapHulls]int32{1, 2, 3, 4} ||
			f.Models[0].VisLeafs != 5 || f.Models[0].FirstFace != 6 || f.Models[0].NumFaces != 7 ||
			f.Models[1].HeadNode[0] != 11 || f.Models[1].VisLeafs != 15 ||
			f.Models[1].FirstFace != 16 || f.Models[1].NumFaces != 17 {
			t.Fatalf("decoded models = %+v", f.Models)
		}
	})
}

func TestLoadFileRejectsMisalignedLumps(t *testing.T) {
	tests := []struct {
		name string
		lump int
		bsp2 bool
		data []byte
		load func(*File, *Reader, *arena.Arena) error
	}{
		{name: "planes", lump: LumpPlanes, data: make([]byte, dPlaneSize+1), load: (*File).loadPlanes},
		{name: "vertexes", lump: LumpVertexes, data: make([]byte, dVertexSize+1), load: (*File).loadVertexes},
		{name: "texinfo", lump: LumpTexinfo, data: make([]byte, 41), load: (*File).loadTexinfo},
		{name: "faces standard", lump: LumpFaces, data: make([]byte, dsFaceSize+1), load: (*File).loadFaces},
		{name: "faces bsp2", lump: LumpFaces, bsp2: true, data: make([]byte, dlFaceSize+1), load: (*File).loadFaces},
		{name: "clipnodes standard", lump: LumpClipnodes, data: make([]byte, 9), load: (*File).loadClipnodes},
		{name: "clipnodes bsp2", lump: LumpClipnodes, bsp2: true, data: make([]byte, 13), load: (*File).loadClipnodes},
		{name: "leafs standard", lump: LumpLeafs, data: make([]byte, dsLeafSize+1), load: (*File).loadLeafs},
		{name: "marksurfaces standard", lump: LumpMarksurfaces, data: make([]byte, uint16Size+1), load: (*File).loadMarkSurfaces},
		{name: "marksurfaces bsp2", lump: LumpMarksurfaces, bsp2: true, data: make([]byte, uint32Size+1), load: (*File).loadMarkSurfaces},
		{name: "edges standard", lump: LumpEdges, data: make([]byte, dsEdgeSize+1), load: (*File).loadEdges},
		{name: "edges bsp2", lump: LumpEdges, bsp2: true, data: make([]byte, dlEdgeSize+1), load: (*File).loadEdges},
		{name: "surfedges", lump: LumpSurfedges, data: make([]byte, int32Size+1), load: (*File).loadSurfedges},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := &File{IsBSP2: tc.bsp2}
			if tc.bsp2 {
				f.Version = BSP2Version_BSP2
			}
			f.Header.Lumps[tc.lump] = Lump{FileLength: int32(len(tc.data))}
			if err := tc.load(f, NewReader(bytes.NewReader(tc.data)), nil); err == nil {
				t.Fatal("load succeeded, want funny lump size error")
			}
		})
	}
}

func TestLoadStandardClipnodesSupportsUnsignedHighNodeIndexes(t *testing.T) {
	const clipnodeCount = 40000
	data := make([]byte, 8*clipnodeCount)
	putI32(data, 0, 0)
	binary.LittleEndian.PutUint16(data[4:], 32768)
	binary.LittleEndian.PutUint16(data[6:], 40000)

	f := &File{}
	f.Header.Lumps[LumpClipnodes] = Lump{FileLength: int32(len(data))}
	if err := f.loadClipnodes(NewReader(bytes.NewReader(data)), nil); err != nil {
		t.Fatalf("loadClipnodes: %v", err)
	}
	clipnodes, ok := f.Clipnodes.([]DSClipNode)
	if !ok {
		t.Fatalf("clipnodes type = %T, want []DSClipNode", f.Clipnodes)
	}
	if got := clipnodes[0].Children[0]; got != 32768 {
		t.Fatalf("high node child = %d, want 32768", got)
	}
	if got := clipnodes[0].Children[1]; got != -25536 {
		t.Fatalf("content child = %d, want -25536", got)
	}
}

// TestLoadTree2PSBNodeStride guards the 2PSB variant, which keeps short
// bounds but still uses 32-bit child and face fields. Its node stride is 32
// bytes in the C BSP format.
func TestLoadTree2PSBNodeStride(t *testing.T) {
	data := make([]byte, dl1NodeSize)
	putI32(data, 0, 0)
	putI32(data, 4, -1)
	putI32(data, 8, -1)
	putI16(data, 12, -10)
	putI16(data, 18, 10)
	putU32(data, 24, 2)
	putU32(data, 28, 3)

	tree := &Tree{
		Version: BSP2Version_2PSB,
		Planes:  []DPlane{{}},
		Faces:   make([]TreeFace, 5),
		Leafs:   []TreeLeaf{{}},
	}
	tree.Header.Lumps[LumpNodes] = Lump{FileLength: int32(len(data))}
	if err := tree.loadNodes(NewReader(bytes.NewReader(data))); err != nil {
		t.Fatalf("loadNodes: %v", err)
	}
	if len(tree.Nodes) != 1 {
		t.Fatalf("node count = %d, want 1", len(tree.Nodes))
	}
	node := tree.Nodes[0]
	if node.BoundsMin.X != -10 || node.BoundsMax.X != 10 || node.FirstFace != 2 || node.NumFaces != 3 {
		t.Fatalf("decoded node = %+v", node)
	}
}

// TestDecompressVisUsesVisLeafCount tests the VIS decompression logic.
// It ensures the engine can correctly decompress the Potentially Visible Set (PVS) data for a given leaf count.
// Where in C: Mod_DecompressVis in model.c
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

// TestLeafPVSAllVisibleUsesVisLeafCount tests the fallback PVS when no VIS data is available.
// It ensures the engine correctly handles maps without precomputed visibility by making everything visible.
// Where in C: Mod_LeafPVS in model.c
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

func TestLoadWithArena(t *testing.T) {
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

	ar := arena.NewArena(1024 * 1024)
	file, err := LoadWithArena(bytes.NewReader(data), ar)
	testutil.AssertNoError(t, err)

	if len(file.Planes) == 0 || len(file.Vertexes) == 0 || file.Nodes == nil {
		t.Fatalf("expected non-empty lump slices when loaded with arena")
	}
	if ar.BytesAllocated() == 0 {
		t.Fatalf("expected arena to record non-zero bytes allocated")
	}
}

func TestLoadWithArenaSynthetic(t *testing.T) {
	data := make([]byte, 200)
	putI32(data, 0, BSPVersion)
	putI32(data, 4+LumpPlanes*8, 64)
	putI32(data, 4+LumpPlanes*8+4, 20)

	ar := arena.NewArena(1024)
	file, err := LoadWithArena(bytes.NewReader(data), ar)
	if err != nil {
		t.Fatalf("LoadWithArena failed: %v", err)
	}
	if len(file.Planes) != 1 {
		t.Fatalf("expected 1 plane, got %d", len(file.Planes))
	}
	if ar.BytesAllocated() == 0 {
		t.Fatalf("expected non-zero arena bytes allocated")
	}
}
