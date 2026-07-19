package main

import (
	"encoding/binary"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/model"
	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
)

// runLiquids inspects every liquid (SURF_DRAWTURB) face in a BSP and reports
// the texinfo flags, lightmap offset, lightmap sample statistics, and the
// resolved per-liquid alpha settings. It applies the .lit sidecar (if present)
// so the lightmap sample analysis reflects what the production renderer sees.
//
// Usage:
//
//	bspdiag liquids <quake_dir> <maps/mapname.bsp> [gamedir]
//
// The output is grouped by texture name and summarised at the end so the
// operator can quickly see whether a map has lit-water data, how many faces
// are degenerate (uniform lightmap), and how many have no lightmap at all.
func runLiquids(fsys litFilesystem, tree *bsp.Tree, mapPath, gamedir string) {
	textureCount := int(binary.LittleEndian.Uint32(tree.TextureData[:4]))

	texList := parseTextureEntries(tree, textureCount)

	fmt.Printf("=== Liquid Face Analysis ===\n")
	fmt.Printf("Map: %s (game: %s)\n", mapPath, gamedir)
	fmt.Printf("BSP version: %d (BSP2: %v)\n", tree.Version, bsp.IsBSP2(tree.Version))
	fmt.Printf("Faces: %d, texinfo: %d, textures: %d\n", len(tree.Faces), len(tree.Texinfo), textureCount)
	fmt.Printf("Lighting bytes (BSP): %d, LightingRGB: %v\n", len(tree.Lighting), tree.LightingRGB)

	// Apply .lit sidecar if the filesystem can provide it.
	if fsys != nil {
		litName := strings.TrimSuffix(mapPath, filepath.Ext(mapPath)) + ".lit"
		if litData, err := fsys.LoadFile(litName); err == nil && len(litData) > 0 {
			before := len(tree.Lighting)
			if lerr := bsp.ApplyLitFile(tree, litData); lerr != nil {
				fmt.Printf("Lit sidecar: present but invalid (%v)\n", lerr)
			} else {
				fmt.Printf("Lit sidecar: applied (%d -> %d lighting bytes)\n", before, len(tree.Lighting))
			}
		} else {
			fmt.Printf("Lit sidecar: none\n")
		}
	}

	// Resolve liquid alpha settings the same way the renderer does.
	overrides := worldimpl.ParseWorldspawnLiquidAlphaOverrides(tree.Entities)
	transWaterSafe := worldimpl.MapVisTransparentWaterSafe(tree)
	settings := worldimpl.ResolveLiquidAlphaSettings(1, 0, 0, 1, overrides, tree)

	fmt.Printf("\n=== Liquid Alpha Resolution ===\n")
	fmt.Printf("  Worldspawn overrides: hasWater=%v water=%.3f  hasLava=%v lava=%.3f  hasSlime=%v slime=%.3f  hasTele=%v tele=%.3f\n",
		overrides.HasWater, overrides.Water,
		overrides.HasLava, overrides.Lava,
		overrides.HasSlime, overrides.Slime,
		overrides.HasTele, overrides.Tele)
	fmt.Printf("  TransparentWaterSafe: %v\n", transWaterSafe)
	fmt.Printf("  Resolved (cvar water=1 lava=0 slime=0 tele=1): water=%.3f lava=%.3f slime=%.3f tele=%.3f\n",
		settings.Water, settings.Lava, settings.Slime, settings.Tele)

	type faceInfo struct {
		idx      int
		texinfo  int32
		miptex   int32
		flags    int32
		name     string
		texType  model.TextureType
		lightOfs int32
		styles   [4]byte
		category string
		samples  []byte
	}

	liquidFaces := make([]faceInfo, 0, 64)
	for i := range tree.Faces {
		face := &tree.Faces[i]
		if int(face.Texinfo) < 0 || int(face.Texinfo) >= len(tree.Texinfo) {
			continue
		}
		ti := &tree.Texinfo[face.Texinfo]
		var name string
		var texType model.TextureType
		if int(ti.Miptex) >= 0 && int(ti.Miptex) < len(texList) && texList[ti.Miptex].loaded {
			name = texList[ti.Miptex].name
			texType = texList[ti.Miptex].texType
		}
		if !strings.HasPrefix(name, "*") {
			continue
		}
		styles := [4]byte{255, 255, 255, 255}
		copy(styles[:], face.Styles[:])
		category, samples := classifyLightmapSamples(tree, face)
		liquidFaces = append(liquidFaces, faceInfo{
			idx:      i,
			texinfo:  face.Texinfo,
			miptex:   ti.Miptex,
			flags:    ti.Flags,
			name:     name,
			texType:  texType,
			lightOfs: face.LightOfs,
			styles:   styles,
			category: category,
			samples:  samples,
		})
	}

	sort.Slice(liquidFaces, func(i, j int) bool {
		if liquidFaces[i].name != liquidFaces[j].name {
			return liquidFaces[i].name < liquidFaces[j].name
		}
		return liquidFaces[i].idx < liquidFaces[j].idx
	})

	fmt.Printf("\n=== Liquid Faces (%d total) ===\n", len(liquidFaces))
	for _, f := range liquidFaces {
		flagsStr := ""
		if f.flags&bsp.TexSpecial != 0 {
			flagsStr = " SPECIAL"
		}
		sampleStr := formatSampleSummary(f.samples, f.category)
		fmt.Printf("  face=%5d texinfo=%5d miptex=%2d flags=%d%-8s texname=%-20s texType=%d LightOfs=%-8d styles=%v %s\n",
			f.idx, f.texinfo, f.miptex, f.flags, flagsStr, f.name, f.texType, f.lightOfs, f.styles, sampleStr)
	}

	// Summary by texture and category.
	type summaryKey struct {
		name, category string
	}
	summary := make(map[summaryKey]int)
	for _, f := range liquidFaces {
		summary[summaryKey{f.name, f.category}]++
	}
	fmt.Printf("\n=== Summary ===\n")
	keys := make([]summaryKey, 0, len(summary))
	for k := range summary {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].name != keys[j].name {
			return keys[i].name < keys[j].name
		}
		return keys[i].category < keys[j].category
	})
	for _, k := range keys {
		fmt.Printf("  %-20s %-10s %4d faces\n", k.name, k.category, summary[k])
	}
}

// litFilesystem is a minimal interface for loading .lit sidecar data.
// The fs.FileSystem satisfies this via LoadFile.
type litFilesystem interface {
	LoadFile(path string) ([]byte, error)
}

// parseTextureEntries reads the BSP miptex lump and returns a slice of texEntry
// structs, one per texture slot.
func parseTextureEntries(tree *bsp.Tree, textureCount int) []texEntry {
	texList := make([]texEntry, textureCount)
	for i := range texList {
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
	return texList
}

// classifyLightmapSamples reads up to 48 bytes of lightmap data for a face and
// categorises them as "VARIED", "UNIFORM", or "NONE". Returns the category and
// the sample bytes (for formatting).
func classifyLightmapSamples(tree *bsp.Tree, face *bsp.TreeFace) (string, []byte) {
	if face.LightOfs < 0 {
		return "NONE", nil
	}
	// Determine how many bytes to read. We read up to 48 bytes (16 RGB triplets
	// or 48 monochrome samples) for a quick statistical check.
	limit := 48
	end := int(face.LightOfs) + limit
	if tree.LightingRGB {
		end = int(face.LightOfs)*3 + limit
	}
	if end > len(tree.Lighting) {
		return "OOB", nil
	}
	start := int(face.LightOfs)
	if tree.LightingRGB {
		start = int(face.LightOfs) * 3
	}
	samples := tree.Lighting[start:end]
	if len(samples) == 0 {
		return "NONE", nil
	}
	first := samples[0]
	allSame := true
	for _, s := range samples[1:] {
		if s != first {
			allSame = false
			break
		}
	}
	if allSame {
		return "UNIFORM", samples
	}
	return "VARIED", samples
}

// formatSampleSummary formats the lightmap sample summary for display.
func formatSampleSummary(samples []byte, category string) string {
	switch category {
	case "NONE":
		return "NONE"
	case "OOB":
		return "OUT_OF_BOUNDS"
	case "UNIFORM":
		if len(samples) == 0 {
			return "UNIFORM(?)"
		}
		return fmt.Sprintf("UNIFORM(=%d) first%d=%v", samples[0], len(samples), samples)
	case "VARIED":
		return fmt.Sprintf("VARIED first%d=%v", len(samples), samples)
	default:
		return category
	}
}
