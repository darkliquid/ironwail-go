// Package main generates a standalone Quake v29 BSP file for test_transparency.bsp.
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

func main() {
	outPath := flag.String("o", "test_transparency.bsp", "Output BSP file path")
	flag.Parse()

	bspData, err := generateTransparencyBSP()
	if err != nil {
		log.Fatalf("generate BSP: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(*outPath), 0755); err != nil && filepath.Dir(*outPath) != "." {
		log.Fatalf("mkdir: %v", err)
	}

	if err := os.WriteFile(*outPath, bspData, 0644); err != nil {
		log.Fatalf("write file: %v", err)
	}

	fmt.Printf("Successfully generated %s (%d bytes)\n", *outPath, len(bspData))
}

func generateTransparencyBSP() ([]byte, error) {
	// Build entities lump
	entBuf := &bytes.Buffer{}
	entBuf.WriteString("{\n\"classname\" \"worldspawn\"\n\"message\" \"Transparency Parity Test Map\"\n\"wateralpha\" \"0.6\"\n\"slimealpha\" \"0.75\"\n\"lavaalpha\" \"0.85\"\n\"telealpha\" \"0.66\"\n\"sounds\" \"1\"\n}\n")
	// Player start
	entBuf.WriteString("{\n\"classname\" \"info_player_start\"\n\"origin\" \"0 -80 0\"\n\"angle\" \"90\"\n}\n")
	// Lights
	entBuf.WriteString("{\n\"classname\" \"light\"\n\"origin\" \"0 0 128\"\n\"light\" \"300\"\n}\n")
	entBuf.WriteString("{\n\"classname\" \"light\"\n\"origin\" \"-120 -120 32\"\n\"light\" \"250\"\n}\n")
	entBuf.WriteString("{\n\"classname\" \"light\"\n\"origin\" \"120 -120 32\"\n\"light\" \"250\"\n}\n")
	entBuf.WriteString("{\n\"classname\" \"light\"\n\"origin\" \"-120 120 32\"\n\"light\" \"250\"\n}\n")
	entBuf.WriteString("{\n\"classname\" \"light\"\n\"origin\" \"120 120 32\"\n\"light\" \"250\"\n}\n")
	// Glass func_wall
	entBuf.WriteString("{\n\"classname\" \"func_wall\"\n\"model\" \"*1\"\n\"alpha\" \"0.5\"\n}\n")
	// Monster behind glass
	entBuf.WriteString("{\n\"classname\" \"monster_ogre\"\n\"origin\" \"120 140 -32\"\n\"angle\" \"270\"\n}\n")
	// Submerged monster in water
	entBuf.WriteString("{\n\"classname\" \"monster_fish\"\n\"origin\" \"-120 -120 -96\"\n\"angle\" \"90\"\n}\n")
	entBuf.WriteByte(0)
	entBytes := entBuf.Bytes()

	// Textures
	textureNames := []string{"wall", "floor", "*water1", "*slime0", "*lava1", "*teleport", "glass"}
	texLump, err := buildTextureLump(textureNames)
	if err != nil {
		return nil, fmt.Errorf("build texture lump: %w", err)
	}

	// Planes
	planes := []bsp.DPlane{
		// 0: Floor z = -64
		{Normal: types.Vec3{X: 0, Y: 0, Z: 1}, Dist: -64, Type: bsp.PlaneZ},
		// 1: Ceiling z = 192
		{Normal: types.Vec3{X: 0, Y: 0, Z: -1}, Dist: -192, Type: bsp.PlaneZ},
		// 2: Wall North y = 256
		{Normal: types.Vec3{X: 0, Y: -1, Z: 0}, Dist: -256, Type: bsp.PlaneY},
		// 3: Wall South y = -256
		{Normal: types.Vec3{X: 0, Y: 1, Z: 0}, Dist: -256, Type: bsp.PlaneY},
		// 4: Wall East x = 256
		{Normal: types.Vec3{X: -1, Y: 0, Z: 0}, Dist: -256, Type: bsp.PlaneX},
		// 5: Wall West x = -256
		{Normal: types.Vec3{X: 1, Y: 0, Z: 0}, Dist: -256, Type: bsp.PlaneX},

		// 6: Water surface z = -64 (Q1)
		{Normal: types.Vec3{X: 0, Y: 0, Z: 1}, Dist: -64, Type: bsp.PlaneZ},
		// 7: Water bottom z = -128
		{Normal: types.Vec3{X: 0, Y: 0, Z: 1}, Dist: -128, Type: bsp.PlaneZ},

		// 8: Slime surface z = -64 (Q2)
		{Normal: types.Vec3{X: 0, Y: 0, Z: 1}, Dist: -64, Type: bsp.PlaneZ},
		// 9: Slime bottom z = -128
		{Normal: types.Vec3{X: 0, Y: 0, Z: 1}, Dist: -128, Type: bsp.PlaneZ},

		// 10: Lava surface z = -64 (Q3)
		{Normal: types.Vec3{X: 0, Y: 0, Z: 1}, Dist: -64, Type: bsp.PlaneZ},
		// 11: Lava bottom z = -128
		{Normal: types.Vec3{X: 0, Y: 0, Z: 1}, Dist: -128, Type: bsp.PlaneZ},

		// 12: Teleport surface z = -64 (Q4)
		{Normal: types.Vec3{X: 0, Y: 0, Z: 1}, Dist: -64, Type: bsp.PlaneZ},

		// 13: Glass entity plane (front y = 80)
		{Normal: types.Vec3{X: 0, Y: -1, Z: 0}, Dist: -80, Type: bsp.PlaneY},
		// 14: Glass back y = 96
		{Normal: types.Vec3{X: 0, Y: 1, Z: 0}, Dist: -96, Type: bsp.PlaneY},
		// 15: Glass top z = 32
		{Normal: types.Vec3{X: 0, Y: 0, Z: 1}, Dist: 32, Type: bsp.PlaneZ},
		// 16: Glass bottom z = -64
		{Normal: types.Vec3{X: 0, Y: 0, Z: -1}, Dist: 64, Type: bsp.PlaneZ},
		// 17: Glass left x = 60
		{Normal: types.Vec3{X: 1, Y: 0, Z: 0}, Dist: -60, Type: bsp.PlaneX},
		// 18: Glass right x = -180
		{Normal: types.Vec3{X: -1, Y: 0, Z: 0}, Dist: -180, Type: bsp.PlaneX},
	}

	// Vertices
	vertices := []bsp.DVertex{
		// Main room outer box:
		// 0..3 Floor
		{Point: types.Vec3{X: -256, Y: -256, Z: -64}},
		{Point: types.Vec3{X: 256, Y: -256, Z: -64}},
		{Point: types.Vec3{X: 256, Y: 256, Z: -64}},
		{Point: types.Vec3{X: -256, Y: 256, Z: -64}},
		// 4..7 Ceiling
		{Point: types.Vec3{X: -256, Y: -256, Z: 192}},
		{Point: types.Vec3{X: 256, Y: -256, Z: 192}},
		{Point: types.Vec3{X: 256, Y: 256, Z: 192}},
		{Point: types.Vec3{X: -256, Y: 256, Z: 192}},

		// Q1 Water Surface (-200..-40, -200..-40, -64):
		{Point: types.Vec3{X: -200, Y: -200, Z: -64}}, // 8
		{Point: types.Vec3{X: -40, Y: -200, Z: -64}},  // 9
		{Point: types.Vec3{X: -40, Y: -40, Z: -64}},   // 10
		{Point: types.Vec3{X: -200, Y: -40, Z: -64}},  // 11
		// Q1 Water Pit Floor (-200..-40, -200..-40, -128):
		{Point: types.Vec3{X: -200, Y: -200, Z: -128}}, // 12
		{Point: types.Vec3{X: -40, Y: -200, Z: -128}},  // 13
		{Point: types.Vec3{X: -40, Y: -40, Z: -128}},   // 14
		{Point: types.Vec3{X: -200, Y: -40, Z: -128}},  // 15

		// Q2 Slime Surface (40..200, -200..-40, -64):
		{Point: types.Vec3{X: 40, Y: -200, Z: -64}},  // 16
		{Point: types.Vec3{X: 200, Y: -200, Z: -64}}, // 17
		{Point: types.Vec3{X: 200, Y: -40, Z: -64}},  // 18
		{Point: types.Vec3{X: 40, Y: -40, Z: -64}},   // 19
		// Q2 Slime Pit Floor (40..200, -200..-40, -128):
		{Point: types.Vec3{X: 40, Y: -200, Z: -128}},  // 20
		{Point: types.Vec3{X: 200, Y: -200, Z: -128}}, // 21
		{Point: types.Vec3{X: 200, Y: -40, Z: -128}},  // 22
		{Point: types.Vec3{X: 40, Y: -40, Z: -128}},   // 23

		// Q3 Lava Surface (-200..-40, 40..200, -64):
		{Point: types.Vec3{X: -200, Y: 40, Z: -64}},  // 24
		{Point: types.Vec3{X: -40, Y: 40, Z: -64}},   // 25
		{Point: types.Vec3{X: -40, Y: 200, Z: -64}},  // 26
		{Point: types.Vec3{X: -200, Y: 200, Z: -64}}, // 27
		// Q3 Lava Pit Floor (-200..-40, 40..200, -128):
		{Point: types.Vec3{X: -200, Y: 40, Z: -128}},  // 28
		{Point: types.Vec3{X: -40, Y: 40, Z: -128}},   // 29
		{Point: types.Vec3{X: -40, Y: 200, Z: -128}},  // 30
		{Point: types.Vec3{X: -200, Y: 200, Z: -128}}, // 31

		// Q4 Teleport Surface (40..200, 40..200, -64):
		{Point: types.Vec3{X: 40, Y: 40, Z: -64}},   // 32
		{Point: types.Vec3{X: 200, Y: 40, Z: -64}},  // 33
		{Point: types.Vec3{X: 200, Y: 200, Z: -64}}, // 34
		{Point: types.Vec3{X: 40, Y: 200, Z: -64}},  // 35

		// Glass brush entity (60..180, 80..96, -64..32):
		{Point: types.Vec3{X: 60, Y: 80, Z: -64}},  // 36
		{Point: types.Vec3{X: 180, Y: 80, Z: -64}}, // 37
		{Point: types.Vec3{X: 180, Y: 80, Z: 32}},  // 38
		{Point: types.Vec3{X: 60, Y: 80, Z: 32}},   // 39
		{Point: types.Vec3{X: 60, Y: 96, Z: -64}},  // 40
		{Point: types.Vec3{X: 180, Y: 96, Z: -64}}, // 41
		{Point: types.Vec3{X: 180, Y: 96, Z: 32}},  // 42
		{Point: types.Vec3{X: 60, Y: 96, Z: 32}},   // 43
	}

	// Texinfo
	texinfos := []bsp.Texinfo{
		// 0: Wall
		{Vecs: [2][4]float32{{1, 0, 0, 0}, {0, 0, -1, 0}}, Miptex: 0, Flags: 0},
		// 1: Floor
		{Vecs: [2][4]float32{{1, 0, 0, 0}, {0, 1, 0, 0}}, Miptex: 1, Flags: 0},
		// 2: *water1
		{Vecs: [2][4]float32{{1, 0, 0, 0}, {0, 1, 0, 0}}, Miptex: 2, Flags: bsp.TexSpecial},
		// 3: *slime0
		{Vecs: [2][4]float32{{1, 0, 0, 0}, {0, 1, 0, 0}}, Miptex: 3, Flags: bsp.TexSpecial},
		// 4: *lava1
		{Vecs: [2][4]float32{{1, 0, 0, 0}, {0, 1, 0, 0}}, Miptex: 4, Flags: bsp.TexSpecial},
		// 5: *teleport
		{Vecs: [2][4]float32{{1, 0, 0, 0}, {0, 1, 0, 0}}, Miptex: 5, Flags: bsp.TexSpecial},
		// 6: glass
		{Vecs: [2][4]float32{{1, 0, 0, 0}, {0, 0, -1, 0}}, Miptex: 6, Flags: 0},
	}

	// Build edges, surfedges, faces
	var edges []bsp.DSEdge
	var surfedges []int32
	var faces []bsp.DSFace

	addEdge := func(v1, v2 uint16) int32 {
		edgeIdx := int32(len(edges))
		edges = append(edges, bsp.DSEdge{V: [2]uint16{v1, v2}})
		return edgeIdx
	}

	addQuadFace := func(planeNum int16, side int16, texinfoIdx int16, v0, v1, v2, v3 uint16) {
		firstSurfEdge := int32(len(surfedges))
		e0 := addEdge(v0, v1)
		e1 := addEdge(v1, v2)
		e2 := addEdge(v2, v3)
		e3 := addEdge(v3, v0)
		surfedges = append(surfedges, e0, e1, e2, e3)
		faces = append(faces, bsp.DSFace{
			PlaneNum:  planeNum,
			Side:      side,
			FirstEdge: firstSurfEdge,
			NumEdges:  4,
			Texinfo:   texinfoIdx,
			Styles:    [bsp.MaxLightmaps]uint8{255, 255, 255, 255},
			LightOfs:  -1,
		})
	}

	// World Faces:
	// 0: Floor
	addQuadFace(0, 0, 1, 0, 1, 2, 3)
	// 1: Ceiling
	addQuadFace(1, 0, 0, 7, 6, 5, 4)
	// 2: Wall North
	addQuadFace(2, 0, 0, 3, 2, 6, 7)
	// 3: Wall South
	addQuadFace(3, 0, 0, 1, 0, 4, 5)
	// 4: Wall East
	addQuadFace(4, 0, 0, 2, 1, 5, 6)
	// 5: Wall West
	addQuadFace(5, 0, 0, 0, 3, 7, 4)

	// Q1 Water Surface (face 6)
	addQuadFace(6, 0, 2, 8, 9, 10, 11)
	// Q1 Water Pit Floor (face 7)
	addQuadFace(7, 0, 1, 12, 13, 14, 15)
	// Q1 Pit Walls (faces 8..11)
	addQuadFace(3, 0, 0, 12, 13, 9, 8)
	addQuadFace(4, 0, 0, 13, 14, 10, 9)
	addQuadFace(2, 0, 0, 14, 15, 11, 10)
	addQuadFace(5, 0, 0, 15, 12, 8, 11)

	// Q2 Slime Surface (face 12)
	addQuadFace(8, 0, 3, 16, 17, 18, 19)
	// Q2 Slime Pit Floor (face 13)
	addQuadFace(9, 0, 1, 20, 21, 22, 23)
	// Q2 Pit Walls (faces 14..17)
	addQuadFace(3, 0, 0, 20, 21, 17, 16)
	addQuadFace(4, 0, 0, 21, 22, 18, 17)
	addQuadFace(2, 0, 0, 22, 23, 19, 18)
	addQuadFace(5, 0, 0, 23, 20, 16, 19)

	// Q3 Lava Surface (face 18)
	addQuadFace(10, 0, 4, 24, 25, 26, 27)
	// Q3 Lava Pit Floor (face 19)
	addQuadFace(11, 0, 1, 28, 29, 30, 31)
	// Q3 Pit Walls (faces 20..23)
	addQuadFace(3, 0, 0, 28, 29, 25, 24)
	addQuadFace(4, 0, 0, 29, 30, 26, 25)
	addQuadFace(2, 0, 0, 30, 31, 27, 26)
	addQuadFace(5, 0, 0, 31, 28, 24, 27)

	// Q4 Teleport Surface (face 24)
	addQuadFace(12, 0, 5, 32, 33, 34, 35)

	worldFaceCount := int32(len(faces))

	// Model 1 (Glass Brush Entity):
	firstGlassFace := int32(len(faces))
	// Front face (face 25)
	addQuadFace(13, 0, 6, 36, 37, 38, 39)
	// Back face (face 26)
	addQuadFace(14, 0, 6, 41, 40, 43, 42)
	// Top face (face 27)
	addQuadFace(15, 0, 6, 39, 38, 42, 43)
	// Bottom face (face 28)
	addQuadFace(16, 0, 6, 40, 41, 37, 36)
	// Left face (face 29)
	addQuadFace(17, 0, 6, 40, 36, 39, 43)
	// Right face (face 30)
	addQuadFace(18, 0, 6, 37, 41, 42, 38)
	glassFaceCount := int32(len(faces)) - firstGlassFace

	// MarkSurfaces: indices of all world faces
	var markSurfaces []uint16
	for i := uint16(0); i < uint16(worldFaceCount); i++ {
		markSurfaces = append(markSurfaces, i)
	}

	// Leafs:
	// Leaf 0 is always solid (empty)
	// Leaf 1 is the main empty room leaf
	leafs := []bsp.DSLeaf{
		{
			Contents:         bsp.ContentsSolid,
			VisOfs:           -1,
			BoundsMin:        [3]int16{-256, -256, -128},
			BoundsMax:        [3]int16{256, 256, 192},
			FirstMarkSurface: 0,
			NumMarkSurfaces:  0,
		},
		{
			Contents:         bsp.ContentsEmpty,
			VisOfs:           -1,
			BoundsMin:        [3]int16{-256, -256, -128},
			BoundsMax:        [3]int16{256, 256, 192},
			FirstMarkSurface: 0,
			NumMarkSurfaces:  uint16(len(markSurfaces)),
		},
	}

	// Nodes: single root node partitioning at z = -64
	nodes := []bsp.DSNode{
		{
			PlaneNum:  0,
			Children:  [2]int16{-(1 + 1), -1}, // Child 0: Leaf 1 (empty), Child 1: Leaf 0 (solid)
			BoundsMin: [3]int16{-256, -256, -128},
			BoundsMax: [3]int16{256, 256, 192},
			FirstFace: 0,
			NumFaces:  uint16(worldFaceCount),
		},
	}

	// Clipnodes for collision
	clipnodes := []bsp.DSClipNode{
		{
			PlaneNum: 0, // z = -64
			Children: [2]int32{bsp.ContentsEmpty, bsp.ContentsSolid},
		},
	}

	// Models
	models := []bsp.DModel{
		// Model 0: World
		{
			BoundsMin: types.Vec3{X: -256, Y: -256, Z: -128},
			BoundsMax: types.Vec3{X: 256, Y: 256, Z: 192},
			Origin:    types.Vec3{},
			HeadNode:  [bsp.MaxMapHulls]int32{0, 0, 0, 0},
			VisLeafs:  1,
			FirstFace: 0,
			NumFaces:  worldFaceCount,
		},
		// Model 1: Glass Entity (*1)
		{
			BoundsMin: types.Vec3{X: 60, Y: 80, Z: -64},
			BoundsMax: types.Vec3{X: 180, Y: 96, Z: 32},
			Origin:    types.Vec3{X: 120, Y: 88, Z: -16},
			HeadNode:  [bsp.MaxMapHulls]int32{0, 0, 0, 0},
			VisLeafs:  0,
			FirstFace: firstGlassFace,
			NumFaces:  glassFaceCount,
		},
	}

	// Serialize all lumps into a BSP v29 binary
	header := bsp.DHeader{Version: bsp.BSPVersion}
	fileBuf := &bytes.Buffer{}

	// Write placeholder header (4 + 15*8 = 124 bytes)
	headerBytes := make([]byte, 4+bsp.HeaderLumps*8)
	fileBuf.Write(headerBytes)

	writeLump := func(lumpIdx int, data []byte) {
		if len(data) == 0 {
			header.Lumps[lumpIdx] = bsp.Lump{FileOffset: 0, FileLength: 0}
			return
		}
		offset := int32(fileBuf.Len())
		length := int32(len(data))
		header.Lumps[lumpIdx] = bsp.Lump{FileOffset: offset, FileLength: length}
		fileBuf.Write(data)
	}

	// 0: Entities
	writeLump(bsp.LumpEntities, entBytes)

	// 1: Planes
	planeBuf := &bytes.Buffer{}
	for _, p := range planes {
		_ = binary.Write(planeBuf, binary.LittleEndian, p.Normal.X)
		_ = binary.Write(planeBuf, binary.LittleEndian, p.Normal.Y)
		_ = binary.Write(planeBuf, binary.LittleEndian, p.Normal.Z)
		_ = binary.Write(planeBuf, binary.LittleEndian, p.Dist)
		_ = binary.Write(planeBuf, binary.LittleEndian, p.Type)
	}
	writeLump(bsp.LumpPlanes, planeBuf.Bytes())

	// 2: Textures
	writeLump(bsp.LumpTextures, texLump)

	// 3: Vertexes
	vertBuf := &bytes.Buffer{}
	for _, v := range vertices {
		_ = binary.Write(vertBuf, binary.LittleEndian, v.Point.X)
		_ = binary.Write(vertBuf, binary.LittleEndian, v.Point.Y)
		_ = binary.Write(vertBuf, binary.LittleEndian, v.Point.Z)
	}
	writeLump(bsp.LumpVertexes, vertBuf.Bytes())

	// 4: Visibility (empty / all visible)
	writeLump(bsp.LumpVisibility, nil)

	// 5: Nodes
	nodeBuf := &bytes.Buffer{}
	for _, n := range nodes {
		_ = binary.Write(nodeBuf, binary.LittleEndian, n.PlaneNum)
		_ = binary.Write(nodeBuf, binary.LittleEndian, n.Children[0])
		_ = binary.Write(nodeBuf, binary.LittleEndian, n.Children[1])
		_ = binary.Write(nodeBuf, binary.LittleEndian, n.BoundsMin[0])
		_ = binary.Write(nodeBuf, binary.LittleEndian, n.BoundsMin[1])
		_ = binary.Write(nodeBuf, binary.LittleEndian, n.BoundsMin[2])
		_ = binary.Write(nodeBuf, binary.LittleEndian, n.BoundsMax[0])
		_ = binary.Write(nodeBuf, binary.LittleEndian, n.BoundsMax[1])
		_ = binary.Write(nodeBuf, binary.LittleEndian, n.BoundsMax[2])
		_ = binary.Write(nodeBuf, binary.LittleEndian, n.FirstFace)
		_ = binary.Write(nodeBuf, binary.LittleEndian, n.NumFaces)
	}
	writeLump(bsp.LumpNodes, nodeBuf.Bytes())

	// 6: Texinfo
	texinfoBuf := &bytes.Buffer{}
	for _, ti := range texinfos {
		for i := 0; i < 2; i++ {
			for j := 0; j < 4; j++ {
				_ = binary.Write(texinfoBuf, binary.LittleEndian, ti.Vecs[i][j])
			}
		}
		_ = binary.Write(texinfoBuf, binary.LittleEndian, ti.Miptex)
		_ = binary.Write(texinfoBuf, binary.LittleEndian, ti.Flags)
	}
	writeLump(bsp.LumpTexinfo, texinfoBuf.Bytes())

	// 7: Faces
	faceBuf := &bytes.Buffer{}
	for _, f := range faces {
		_ = binary.Write(faceBuf, binary.LittleEndian, f.PlaneNum)
		_ = binary.Write(faceBuf, binary.LittleEndian, f.Side)
		_ = binary.Write(faceBuf, binary.LittleEndian, f.FirstEdge)
		_ = binary.Write(faceBuf, binary.LittleEndian, f.NumEdges)
		_ = binary.Write(faceBuf, binary.LittleEndian, f.Texinfo)
		_ = binary.Write(faceBuf, binary.LittleEndian, f.Styles)
		_ = binary.Write(faceBuf, binary.LittleEndian, f.LightOfs)
	}
	writeLump(bsp.LumpFaces, faceBuf.Bytes())

	// 8: Lighting
	writeLump(bsp.LumpLighting, nil)

	// 9: Clipnodes
	clipBuf := &bytes.Buffer{}
	for _, cn := range clipnodes {
		_ = binary.Write(clipBuf, binary.LittleEndian, cn.PlaneNum)
		_ = binary.Write(clipBuf, binary.LittleEndian, int16(cn.Children[0]))
		_ = binary.Write(clipBuf, binary.LittleEndian, int16(cn.Children[1]))
	}
	writeLump(bsp.LumpClipnodes, clipBuf.Bytes())

	// 10: Leafs
	leafBuf := &bytes.Buffer{}
	for _, l := range leafs {
		_ = binary.Write(leafBuf, binary.LittleEndian, l.Contents)
		_ = binary.Write(leafBuf, binary.LittleEndian, l.VisOfs)
		_ = binary.Write(leafBuf, binary.LittleEndian, l.BoundsMin[0])
		_ = binary.Write(leafBuf, binary.LittleEndian, l.BoundsMin[1])
		_ = binary.Write(leafBuf, binary.LittleEndian, l.BoundsMin[2])
		_ = binary.Write(leafBuf, binary.LittleEndian, l.BoundsMax[0])
		_ = binary.Write(leafBuf, binary.LittleEndian, l.BoundsMax[1])
		_ = binary.Write(leafBuf, binary.LittleEndian, l.BoundsMax[2])
		_ = binary.Write(leafBuf, binary.LittleEndian, l.FirstMarkSurface)
		_ = binary.Write(leafBuf, binary.LittleEndian, l.NumMarkSurfaces)
		_ = binary.Write(leafBuf, binary.LittleEndian, l.AmbientLevel)
	}
	writeLump(bsp.LumpLeafs, leafBuf.Bytes())

	// 11: Marksurfaces
	markBuf := &bytes.Buffer{}
	for _, ms := range markSurfaces {
		_ = binary.Write(markBuf, binary.LittleEndian, ms)
	}
	writeLump(bsp.LumpMarksurfaces, markBuf.Bytes())

	// 12: Edges
	edgeBuf := &bytes.Buffer{}
	for _, e := range edges {
		_ = binary.Write(edgeBuf, binary.LittleEndian, e.V[0])
		_ = binary.Write(edgeBuf, binary.LittleEndian, e.V[1])
	}
	writeLump(bsp.LumpEdges, edgeBuf.Bytes())

	// 13: Surfedges
	surfedgeBuf := &bytes.Buffer{}
	for _, se := range surfedges {
		_ = binary.Write(surfedgeBuf, binary.LittleEndian, se)
	}
	writeLump(bsp.LumpSurfedges, surfedgeBuf.Bytes())

	// 14: Models
	modelBuf := &bytes.Buffer{}
	for _, m := range models {
		_ = binary.Write(modelBuf, binary.LittleEndian, m.BoundsMin.X)
		_ = binary.Write(modelBuf, binary.LittleEndian, m.BoundsMin.Y)
		_ = binary.Write(modelBuf, binary.LittleEndian, m.BoundsMin.Z)
		_ = binary.Write(modelBuf, binary.LittleEndian, m.BoundsMax.X)
		_ = binary.Write(modelBuf, binary.LittleEndian, m.BoundsMax.Y)
		_ = binary.Write(modelBuf, binary.LittleEndian, m.BoundsMax.Z)
		_ = binary.Write(modelBuf, binary.LittleEndian, m.Origin.X)
		_ = binary.Write(modelBuf, binary.LittleEndian, m.Origin.Y)
		_ = binary.Write(modelBuf, binary.LittleEndian, m.Origin.Z)
		for h := 0; h < bsp.MaxMapHulls; h++ {
			_ = binary.Write(modelBuf, binary.LittleEndian, m.HeadNode[h])
		}
		_ = binary.Write(modelBuf, binary.LittleEndian, m.VisLeafs)
		_ = binary.Write(modelBuf, binary.LittleEndian, m.FirstFace)
		_ = binary.Write(modelBuf, binary.LittleEndian, m.NumFaces)
	}
	writeLump(bsp.LumpModels, modelBuf.Bytes())

	// Overwrite header with populated offsets and lengths
	raw := fileBuf.Bytes()
	headerBuf := &bytes.Buffer{}
	_ = binary.Write(headerBuf, binary.LittleEndian, header.Version)
	for i := 0; i < bsp.HeaderLumps; i++ {
		_ = binary.Write(headerBuf, binary.LittleEndian, header.Lumps[i].FileOffset)
		_ = binary.Write(headerBuf, binary.LittleEndian, header.Lumps[i].FileLength)
	}
	copy(raw[:4+bsp.HeaderLumps*8], headerBuf.Bytes())

	return raw, nil
}

func buildTextureLump(names []string) ([]byte, error) {
	buf := &bytes.Buffer{}
	numTex := int32(len(names))
	_ = binary.Write(buf, binary.LittleEndian, numTex)

	// Reserve space for offset table
	offsetTablePos := buf.Len()
	for i := 0; i < len(names); i++ {
		_ = binary.Write(buf, binary.LittleEndian, int32(0))
	}

	offsets := make([]int32, len(names))
	const width = uint32(64)
	const height = uint32(64)
	mip0Size := int(width * height)
	mip1Size := int((width / 2) * (height / 2))
	mip2Size := int((width / 4) * (height / 4))
	mip3Size := int((width / 8) * (height / 8))
	totalPixelBytes := mip0Size + mip1Size + mip2Size + mip3Size

	for i, name := range names {
		offsets[i] = int32(buf.Len())
		var nameBytes [16]byte
		copy(nameBytes[:], name)

		mipOfs0 := uint32(40)
		mipOfs1 := mipOfs0 + uint32(mip0Size)
		mipOfs2 := mipOfs1 + uint32(mip1Size)
		mipOfs3 := mipOfs2 + uint32(mip2Size)

		_ = binary.Write(buf, binary.LittleEndian, nameBytes)
		_ = binary.Write(buf, binary.LittleEndian, width)
		_ = binary.Write(buf, binary.LittleEndian, height)
		_ = binary.Write(buf, binary.LittleEndian, [4]uint32{mipOfs0, mipOfs1, mipOfs2, mipOfs3})

		// Fill solid test color pixels
		pixelVal := byte(144) // bright stone wall
		switch name {
		case "floor":
			pixelVal = 160 // bright stone floor
		case "*water1":
			pixelVal = 200 // blue
		case "*slime0":
			pixelVal = 112 // green
		case "*lava1":
			pixelVal = 224 // orange/red
		case "*teleport":
			pixelVal = 240 // red
		case "glass":
			pixelVal = 192 // ice/cyan
		}
		pixels := make([]byte, totalPixelBytes)
		for p := range pixels {
			pixels[p] = pixelVal
		}
		buf.Write(pixels)
	}

	// Overwrite offset table
	raw := buf.Bytes()
	for i, ofs := range offsets {
		binary.LittleEndian.PutUint32(raw[offsetTablePos+i*4:offsetTablePos+(i+1)*4], uint32(ofs))
	}

	return raw, nil
}
