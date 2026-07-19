package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/model"
	surfacepkg "github.com/darkliquid/ironwail-go/internal/renderer/surface"
	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
)

type texEntry struct {
	index   int
	name    string
	width   int
	height  int
	offset  int
	loaded  bool
	texType model.TextureType
}

type simNode struct {
	x, y, w, h int
	child      [2]*simNode
	filled     bool
}

func (n *simNode) insert(w, h int) *simNode {
	if n.child[0] != nil || n.child[1] != nil {
		r := n.child[0].insert(w, h)
		if r != nil {
			return r
		}
		return n.child[1].insert(w, h)
	}
	if n.filled || w > n.w || h > n.h {
		return nil
	}
	if w == n.w && h == n.h {
		n.filled = true
		return n
	}
	dw, dh := n.w-w, n.h-h
	if dw > dh {
		n.child[0] = &simNode{x: n.x, y: n.y, w: w, h: n.h}
		n.child[1] = &simNode{x: n.x + w, y: n.y, w: n.w - w, h: n.h}
	} else {
		n.child[0] = &simNode{x: n.x, y: n.y, w: n.w, h: h}
		n.child[1] = &simNode{x: n.x, y: n.y + h, w: n.w, h: n.h - h}
	}
	return n.child[0].insert(w, h)
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  bspdiag [info] <quake_dir> <maps/mapname.bsp> [gamedir]")
	fmt.Fprintln(os.Stderr, "  bspdiag entities <quake_dir> <maps/mapname.bsp> [gamedir] [classname_filter]")
	fmt.Fprintln(os.Stderr, "  bspdiag point <x> <y> <z> <quake_dir> <maps/mapname.bsp> [gamedir]")
	fmt.Fprintln(os.Stderr, "  bspdiag face <face_index> <quake_dir> <maps/mapname.bsp> [gamedir]")
	fmt.Fprintln(os.Stderr, "  bspdiag texture <texture_name> <quake_dir> <maps/mapname.bsp> [gamedir]")
	fmt.Fprintln(os.Stderr, "  bspdiag liquids <quake_dir> <maps/mapname.bsp> [gamedir]")
}

func main() {
	if len(os.Args) < 3 {
		printUsage()
		os.Exit(1)
	}

	subcommand := os.Args[1]
	var quakeDir, mapPath, gamedir string
	var extraArgs []string

	switch subcommand {
	case "info":
		if len(os.Args) < 4 {
			printUsage()
			os.Exit(1)
		}
		quakeDir = os.Args[2]
		mapPath = os.Args[3]
		if len(os.Args) >= 5 {
			gamedir = os.Args[4]
		}
	case "entities":
		if len(os.Args) < 4 {
			printUsage()
			os.Exit(1)
		}
		quakeDir = os.Args[2]
		mapPath = os.Args[3]
		if len(os.Args) >= 5 {
			gamedir = os.Args[4]
		}
		if len(os.Args) >= 6 {
			extraArgs = os.Args[5:]
		}
	case "point":
		if len(os.Args) < 6 {
			printUsage()
			os.Exit(1)
		}
		extraArgs = []string{os.Args[2], os.Args[3], os.Args[4]} // x, y, z
		quakeDir = os.Args[5]
		mapPath = os.Args[6]
		if len(os.Args) >= 8 {
			gamedir = os.Args[7]
		}
	case "face":
		if len(os.Args) < 4 {
			printUsage()
			os.Exit(1)
		}
		extraArgs = []string{os.Args[2]} // face_index
		quakeDir = os.Args[3]
		mapPath = os.Args[4]
		if len(os.Args) >= 6 {
			gamedir = os.Args[5]
		}
	case "texture":
		if len(os.Args) < 4 {
			printUsage()
			os.Exit(1)
		}
		extraArgs = []string{os.Args[2]} // texture_name
		quakeDir = os.Args[3]
		mapPath = os.Args[4]
		if len(os.Args) >= 6 {
			gamedir = os.Args[5]
		}
	case "liquids":
		if len(os.Args) < 4 {
			printUsage()
			os.Exit(1)
		}
		quakeDir = os.Args[2]
		mapPath = os.Args[3]
		if len(os.Args) >= 5 {
			gamedir = os.Args[4]
		}
	default:
		// Backward compatibility: default to info mode where arg 1 is quakeDir
		subcommand = "info"
		quakeDir = os.Args[1]
		mapPath = os.Args[2]
		if len(os.Args) >= 4 {
			gamedir = os.Args[3]
		}
	}

	fsys := fs.NewFileSystem()
	if err := fsys.Init(quakeDir, gamedir); err != nil {
		fmt.Fprintf(os.Stderr, "init fs: %v\n", err)
		os.Exit(1)
	}
	r, size, err := fsys.OpenFile(mapPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open %s: %v\n", mapPath, err)
		os.Exit(1)
	}
	defer func() { _ = r.Close() }()
	data := make([]byte, size)
	if _, err := r.Read(data); err != nil {
		fmt.Fprintf(os.Stderr, "read: %v\n", err)
		os.Exit(1)
	}
	tree, err := bsp.LoadTree(bytes.NewReader(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse: %v\n", err)
		os.Exit(1)
	}

	switch subcommand {
	case "info":
		runInfo(tree, mapPath, gamedir)
	case "entities":
		classFilter := ""
		if len(extraArgs) > 0 {
			classFilter = extraArgs[0]
		}
		runEntities(tree, classFilter)
	case "point":
		x, _ := strconv.ParseFloat(extraArgs[0], 32)
		y, _ := strconv.ParseFloat(extraArgs[1], 32)
		z, _ := strconv.ParseFloat(extraArgs[2], 32)
		runPoint(tree, float32(x), float32(y), float32(z))
	case "face":
		faceIdx, err := strconv.Atoi(extraArgs[0])
		if err != nil || faceIdx < 0 || faceIdx >= len(tree.Faces) {
			fmt.Fprintf(os.Stderr, "invalid face index: %s (total faces: %d)\n", extraArgs[0], len(tree.Faces))
			os.Exit(1)
		}
		runFace(tree, faceIdx)
	case "texture":
		runTexture(fsys, tree, extraArgs[0])
	case "liquids":
		runLiquids(fsys, tree, mapPath, gamedir)
	}
}

func runInfo(tree *bsp.Tree, mapPath, gamedir string) {
	textureCount := int(binary.LittleEndian.Uint32(tree.TextureData[:4]))
	fmt.Printf("=== BSP Info ===\n")
	fmt.Printf("Map: %s (game: %s)\n", mapPath, gamedir)
	fmt.Printf("BSP version: %d (BSP2: %v)\n", tree.Version, bsp.IsBSP2(tree.Version))
	fmt.Printf("Texture count: %d\n", textureCount)
	fmt.Printf("Texinfo count: %d\n", len(tree.Texinfo))
	fmt.Printf("Faces: %d\n", len(tree.Faces))
	fmt.Printf("Models: %d\n", len(tree.Models))

	// === Texture table ===
	fmt.Printf("\n=== Texture Table ===\n")
	texList := make([]texEntry, textureCount)
	for i := 0; i < textureCount; i++ {
		offset := int(int32(binary.LittleEndian.Uint32(tree.TextureData[4+i*4:])))
		te := texEntry{index: i, offset: offset}
		if offset <= 0 || offset >= len(tree.TextureData) {
			texList[i] = te
			continue
		}
		mt, err := image.ParseMipTex(tree.TextureData[offset:])
		if err != nil {
			texList[i] = te
			continue
		}
		te.name = mt.Name
		te.width = int(mt.Width)
		te.height = int(mt.Height)
		te.loaded = true
		te.texType = worldimpl.ClassifyTextureName(mt.Name)
		texList[i] = te
	}
	for _, t := range texList {
		status := "OK"
		if !t.loaded {
			status = "MISSING"
		}
		fmt.Printf("  [%2d] %-20s %4dx%-4d  type=%-10d  %s\n",
			t.index, t.name, t.width, t.height, t.texType, status)
	}

	// === Texinfo → Miptex (unique) ===
	fmt.Printf("\n=== Texinfo → Miptex (unique) ===\n")
	type tk struct{ miptex, flags int32 }
	tmap := map[tk]int{}
	for _, ti := range tree.Texinfo {
		tmap[tk{ti.Miptex, ti.Flags}]++
	}
	keys := make([]tk, 0, len(tmap))
	for k := range tmap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].miptex < keys[j].miptex })
	for _, k := range keys {
		name := "<missing>"
		if int(k.miptex) >= 0 && int(k.miptex) < textureCount && texList[k.miptex].loaded {
			name = texList[k.miptex].name
		}
		flags := ""
		if k.flags&bsp.TexSpecial != 0 {
			flags += " SPECIAL"
		}
		if k.flags&bsp.TexMissing != 0 {
			flags += " MISSING"
		}
		fmt.Printf("  miptex=%2d  flags=%d%-12s  refs=%4d  %s\n",
			k.miptex, k.flags, flags, tmap[k], name)
	}

	// === Face → resolved texture index (sampled) ===
	fmt.Printf("\n=== Face → Texture Index (first 50) ===\n")
	for i := 0; i < len(tree.Faces) && i < 50; i++ {
		face := &tree.Faces[i]
		if int(face.Texinfo) < 0 || int(face.Texinfo) >= len(tree.Texinfo) {
			fmt.Printf("  face=%5d  texinfo=%d (OUT OF RANGE)\n", i, face.Texinfo)
			continue
		}
		ti := &tree.Texinfo[face.Texinfo]
		missing := ti.Flags&bsp.TexMissing != 0
		if int(ti.Miptex) < 0 || int(ti.Miptex) >= textureCount {
			missing = true
		} else if !texList[ti.Miptex].loaded {
			missing = true
		}
		var resolved int32
		if missing {
			if ti.Flags&bsp.TexSpecial != 0 {
				resolved = int32(textureCount + 1)
			} else {
				resolved = int32(textureCount)
			}
		} else {
			resolved = ti.Miptex
		}
		name := "<missing>"
		if resolved >= 0 && int(resolved) < textureCount && texList[resolved].loaded {
			name = texList[resolved].name
		} else if resolved == int32(textureCount) {
			name = "<dummy white>"
		} else if resolved == int32(textureCount+1) {
			name = "<dummy transparent>"
		}
		fmt.Printf("  face=%5d  texinfo=%5d  miptex=%2d  resolved=%2d  %s\n",
			i, face.Texinfo, ti.Miptex, resolved, name)
	}

	// === Atlas layout simulation ===
	fmt.Printf("\n=== Atlas Layout Simulation ===\n")
	const atlasSize = 2048
	layers := []*simNode{{x: 0, y: 0, w: atlasSize, h: atlasSize}}
	findLayer := func(n *simNode) int {
		for i, l := range layers {
			if l == n {
				return i
			}
		}
		return -1
	}
	// Dummies first
	for _, name := range []string{"<dummy white>", "<dummy transparent>"} {
		var n *simNode
		for _, l := range layers {
			if n = l.insert(1, 1); n != nil {
				break
			}
		}
		if n == nil {
			nl := &simNode{x: 0, y: 0, w: atlasSize, h: atlasSize}
			layers = append(layers, nl)
			n = nl.insert(1, 1)
		}
		fmt.Printf("  %-20s → layer %d at (%d, %d)\n", name, findLayer(n), n.x, n.y)
	}
	// Real textures
	for i := 0; i < textureCount; i++ {
		if !texList[i].loaded {
			fmt.Printf("  [%2d] %-20s → SKIPPED\n", i, texList[i].name)
			continue
		}
		w, h := texList[i].width, texList[i].height
		if w > atlasSize || h > atlasSize {
			fmt.Printf("  [%2d] %-20s → TOO LARGE\n", i, texList[i].name)
			continue
		}
		var n *simNode
		for _, l := range layers {
			if n = l.insert(w, h); n != nil {
				break
			}
		}
		if n == nil {
			nl := &simNode{x: 0, y: 0, w: atlasSize, h: atlasSize}
			layers = append(layers, nl)
			n = nl.insert(w, h)
		}
		if n != nil {
			fmt.Printf("  [%2d] %-20s %4dx%-4d → layer %d at (%d, %d)\n",
				i, texList[i].name, w, h, findLayer(n), n.x, n.y)
		}
	}
	fmt.Printf("\n  Total atlas layers: %d (%d MB)\n", len(layers), len(layers)*atlasSize*atlasSize*4/1024/1024)

	// Non-power-of-2 check
	for _, t := range texList {
		if t.loaded && !isPow2(t.width) {
			fmt.Printf("  NON-POW2 width: [%d] %s %dx%d\n", t.index, t.name, t.width, t.height)
		}
		if t.loaded && !isPow2(t.height) {
			fmt.Printf("  NON-POW2 height: [%d] %s %dx%d\n", t.index, t.name, t.width, t.height)
		}
	}

	// Lightmap estimate
	fmt.Printf("\n=== Lightmap Estimate ===\n")
	alloc, err := surfacepkg.NewLightmapAllocator(1024, 1024, false)
	if err == nil {
		maxPage := 0
		for _, face := range tree.Faces {
			ti := faceTexInfo(tree, &face)
			if ti == nil {
				continue
			}
			minU, maxU, minV, maxV := 1e9, -1e9, 1e9, -1e9
			for j := int32(0); j < face.NumEdges; j++ {
				se := tree.Surfedges[face.FirstEdge+j]
				var vi uint32
				if se >= 0 {
					vi = tree.Edges[se].V[0]
				} else {
					vi = tree.Edges[-se].V[1]
				}
				p := tree.Vertexes[vi].Point
				u := float64(p[0])*float64(ti.Vecs[0][0]) + float64(p[1])*float64(ti.Vecs[0][1]) + float64(p[2])*float64(ti.Vecs[0][2]) + float64(ti.Vecs[0][3])
				v := float64(p[0])*float64(ti.Vecs[1][0]) + float64(p[1])*float64(ti.Vecs[1][1]) + float64(p[2])*float64(ti.Vecs[1][2]) + float64(ti.Vecs[1][3])
				if u < minU {
					minU = u
				}
				if u > maxU {
					maxU = u
				}
				if v < minV {
					minV = v
				}
				if v > maxV {
					maxV = v
				}
			}
			smax := int((maxU-minU)/16.0) + 1
			tmax := int((maxV-minV)/16.0) + 1
			if smax <= 0 || tmax <= 0 {
				continue
			}
			tn, _, _, e := alloc.AllocBlock(smax, tmax)
			if e != nil {
				continue
			}
			if tn+1 > maxPage {
				maxPage = tn + 1
			}
		}
		fmt.Printf("  Estimated lightmap pages: %d (%d MB)\n", maxPage, maxPage*4)
	}
}

func parseEntityObjects(data string) []string {
	var objects []string
	pos := 0
	for {
		start := strings.IndexByte(data[pos:], '{')
		if start < 0 {
			break
		}
		start += pos
		end := strings.IndexByte(data[start+1:], '}')
		if end < 0 {
			break
		}
		end += start + 1
		objects = append(objects, data[start+1:end])
		pos = end + 1
	}
	return objects
}

func runEntities(tree *bsp.Tree, classFilter string) {
	entStr := string(tree.Entities)
	if strings.TrimSpace(entStr) == "" {
		fmt.Println("No entity lump data in BSP.")
		return
	}

	classFilter = strings.ToLower(classFilter)
	entityBlocks := parseEntityObjects(entStr)
	fmt.Printf("=== BSP Entities (%d total) ===\n", len(entityBlocks))
	matched := 0

	for i, entBlock := range entityBlocks {
		fields := worldimpl.ParseEntityFields(entBlock)
		classname := fields["classname"]
		if classFilter != "" && !strings.Contains(strings.ToLower(classname), classFilter) {
			continue
		}
		matched++
		fmt.Printf("\nEntity #%d: classname = %q\n", i, classname)
		keys := make([]string, 0, len(fields))
		for k := range fields {
			if k == "classname" {
				continue
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("  %-20s = %q\n", k, fields[k])
		}
	}
	fmt.Printf("\nTotal matching entities: %d\n", matched)
}

func runPoint(tree *bsp.Tree, x, y, z float32) {
	pos := [3]float32{x, y, z}
	fmt.Printf("=== BSP Point Query (%.2f, %.2f, %.2f) ===\n", x, y, z)
	leaf := tree.PointInLeaf(pos)
	if leaf == nil {
		fmt.Println("Point returned NIL leaf (out of bounds or invalid tree).")
		return
	}

	contentsStr := contentsToString(leaf.Contents)
	fmt.Printf("Leaf contents: %d (%s)\n", leaf.Contents, contentsStr)

	leafIdx := -1
	for i := range tree.Leafs {
		if &tree.Leafs[i] == leaf {
			leafIdx = i
			break
		}
	}
	if leafIdx >= 0 {
		fmt.Printf("Leaf index: %d\n", leafIdx)
	}
}

func contentsToString(contents int32) string {
	switch contents {
	case -1:
		return "CONTENTS_EMPTY"
	case -2:
		return "CONTENTS_SOLID"
	case -3:
		return "CONTENTS_WATER"
	case -4:
		return "CONTENTS_SLIME"
	case -5:
		return "CONTENTS_LAVA"
	case -6:
		return "CONTENTS_SKY"
	default:
		return fmt.Sprintf("CONTENTS_UNKNOWN(%d)", contents)
	}
}

func runFace(tree *bsp.Tree, faceIdx int) {
	face := &tree.Faces[faceIdx]
	fmt.Printf("=== BSP Face #%d ===\n", faceIdx)
	fmt.Printf("Plane index: %d\n", face.PlaneNum)
	fmt.Printf("Side: %d\n", face.Side)
	fmt.Printf("First edge: %d, Num edges: %d\n", face.FirstEdge, face.NumEdges)
	fmt.Printf("Texinfo: %d\n", face.Texinfo)
	fmt.Printf("LightOfs: %d\n", face.LightOfs)
	fmt.Printf("Styles: %v\n", face.Styles)

	textureCount := int(binary.LittleEndian.Uint32(tree.TextureData[:4]))
	if int(face.Texinfo) >= 0 && int(face.Texinfo) < len(tree.Texinfo) {
		ti := &tree.Texinfo[face.Texinfo]
		fmt.Printf("Texinfo: Miptex=%d, Flags=%d\n", ti.Miptex, ti.Flags)
		if int(ti.Miptex) >= 0 && int(ti.Miptex) < textureCount {
			offset := int(int32(binary.LittleEndian.Uint32(tree.TextureData[4+int(ti.Miptex)*4:])))
			if offset > 0 && offset < len(tree.TextureData) {
				mt, err := image.ParseMipTex(tree.TextureData[offset:])
				if err == nil {
					texType := worldimpl.ClassifyTextureName(mt.Name)
					fmt.Printf("Texture name: %s (%dx%d), type: %v\n", mt.Name, mt.Width, mt.Height, texType)
				}
			}
		}
	}

	minU, maxU, minV, maxV := 1e9, -1e9, 1e9, -1e9
	fmt.Printf("\nVertices (%d points):\n", face.NumEdges)
	ti := faceTexInfo(tree, face)
	for j := int32(0); j < face.NumEdges; j++ {
		se := tree.Surfedges[face.FirstEdge+j]
		var vi uint32
		if se >= 0 {
			vi = tree.Edges[se].V[0]
		} else {
			vi = tree.Edges[-se].V[1]
		}
		p := tree.Vertexes[vi].Point
		fmt.Printf("  V[%2d]: (%.2f, %.2f, %.2f)", j, p[0], p[1], p[2])

		if ti != nil {
			u := float64(p[0])*float64(ti.Vecs[0][0]) + float64(p[1])*float64(ti.Vecs[0][1]) + float64(p[2])*float64(ti.Vecs[0][2]) + float64(ti.Vecs[0][3])
			v := float64(p[0])*float64(ti.Vecs[1][0]) + float64(p[1])*float64(ti.Vecs[1][1]) + float64(p[2])*float64(ti.Vecs[1][2]) + float64(ti.Vecs[1][3])
			fmt.Printf(" -> UV: (%.2f, %.2f)", u, v)
			if u < minU {
				minU = u
			}
			if u > maxU {
				maxU = u
			}
			if v < minV {
				minV = v
			}
			if v > maxV {
				maxV = v
			}
		}
		fmt.Println()
	}

	if ti != nil {
		smax := int((maxU-minU)/16.0) + 1
		tmax := int((maxV-minV)/16.0) + 1
		fmt.Printf("\nLightmap bounds: S=[%.2f .. %.2f], T=[%.2f .. %.2f] -> Grid %dx%d\n",
			minU, maxU, minV, maxV, smax, tmax)
	}

	if face.LightOfs >= 0 && int(face.LightOfs) < len(tree.Lighting) {
		lmData := tree.Lighting[face.LightOfs:]
		limit := 16
		if len(lmData) < limit {
			limit = len(lmData)
		}
		fmt.Printf("Lightmap sample bytes (first %d): %v\n", limit, lmData[:limit])
	} else {
		fmt.Println("Lightmap: NONE (LightOfs = -1 or out of bounds)")
	}
}

func runTexture(fsys *fs.FileSystem, tree *bsp.Tree, texName string) {
	textureCount := int(binary.LittleEndian.Uint32(tree.TextureData[:4]))
	texNameLower := strings.ToLower(texName)
	fmt.Printf("=== BSP Texture Query (%q) ===\n", texName)

	found := false
	for i := 0; i < textureCount; i++ {
		offset := int(int32(binary.LittleEndian.Uint32(tree.TextureData[4+i*4:])))
		if offset <= 0 || offset >= len(tree.TextureData) {
			continue
		}
		mt, err := image.ParseMipTex(tree.TextureData[offset:])
		if err != nil {
			continue
		}
		if strings.ToLower(mt.Name) != texNameLower {
			continue
		}
		found = true
		texType := worldimpl.ClassifyTextureName(mt.Name)
		fmt.Printf("Texture [%d]: name=%q, width=%d, height=%d, type=%v\n",
			i, mt.Name, mt.Width, mt.Height, texType)

		pixels, width, height, err := mt.MipLevel(0)
		if err != nil {
			fmt.Printf("Error getting MipLevel(0): %v\n", err)
			return
		}
		fmt.Printf("MipLevel(0) size: %dx%d (%d pixels)\n", width, height, len(pixels))

		limit := 16
		if len(pixels) < limit {
			limit = len(pixels)
		}
		fmt.Printf("First %d raw palette indices: %v\n", limit, pixels[:limit])

		palette, err := fsys.LoadFile("gfx/palette.lmp")
		if err != nil {
			fmt.Printf("Could not load gfx/palette.lmp: %v\n", err)
			return
		}
		mat := worldimpl.BuildMaterialTextureRGBA(pixels[:limit], palette, texType)
		fmt.Printf("First %d RGBA colors (with Quake palette):\n", limit/4)
		for px := 0; px < limit/4; px++ {
			fmt.Printf("  Px[%2d]: R=%3d G=%3d B=%3d A=%3d\n",
				px,
				mat.DiffuseRGBA[px*4+0],
				mat.DiffuseRGBA[px*4+1],
				mat.DiffuseRGBA[px*4+2],
				mat.DiffuseRGBA[px*4+3],
			)
		}
		break
	}

	if !found {
		fmt.Printf("Texture %q not found in BSP texture lump (%d textures searched).\n", texName, textureCount)
	}
}

func faceTexInfo(tree *bsp.Tree, face *bsp.TreeFace) *bsp.Texinfo {
	if tree == nil || face == nil {
		return nil
	}
	if int(face.Texinfo) < 0 || int(face.Texinfo) >= len(tree.Texinfo) {
		return nil
	}
	return &tree.Texinfo[face.Texinfo]
}

func isPow2(n int) bool { return n > 0 && (n&(n-1)) == 0 }
