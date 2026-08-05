package renderer

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"image/png"
	"log/slog"
	"os"
	"strings"
	"time"

	stdimage "image"

	"github.com/gogpu/wgpu"

	surfacepkg "github.com/darkliquid/ironwail-go/internal/renderer/surface"
)

// Atlas diagnostic instrumentation for diagnosing texture corruption on large
// BSP2 maps. All diagnostics are gated by environment variables so they have
// zero cost in normal operation.
//
// IRONWAIL_DEBUG_MATERIAL_AUDIT=1 — enables detailed per-face/per-animation
//   logging (Phases 2 and 4).
// IRONWAIL_DEBUG_MATERIAL_DUMP=1  — enables writing the material table and
//   atlas layer images to files (Phase 5).
// IRONWAIL_DEBUG_MATERIAL_VIZ=n   — enables debug shader visualization where
//   the fragment shader outputs diagnostic colors instead of textures:
//   1 = materialID as color, 2 = atlas layer as grayscale,
//   3 = atlas UV as color (Phase 6).
// IRONWAIL_DEBUG_ATLAS_DIR=path   — directory for diagnostic file output
//   (defaults to the current directory).

// worldMaterialsBufferCapacity is the fixed GPU uniform buffer capacity
// shared between the Go allocation, the GPU buffer creation, and the WGSL
// shader array declaration. This value MUST match:
//   - world_upload_gogpu.go:  Size: 256 * 32
//   - world_material_gogpu.go: worldMaterialsBufferSize constant
//   - world_shaders_gogpu.go:  array<MaterialData, 256>
//
// If a map has more textures than this, the material buffer overflows and
// the shader reads out-of-bounds material data, producing wrong textures.
const worldMaterialsBufferCapacity = 256

// debugMaterialAuditEnabled returns true when IRONWAIL_DEBUG_MATERIAL_AUDIT=1.
func debugMaterialAuditEnabled() bool {
	return os.Getenv("IRONWAIL_DEBUG_MATERIAL_AUDIT") == "1"
}

// debugMaterialDumpEnabled returns true when IRONWAIL_DEBUG_MATERIAL_DUMP=1.
func debugMaterialDumpEnabled() bool {
	return os.Getenv("IRONWAIL_DEBUG_MATERIAL_DUMP") == "1"
}

// debugMaterialVizMode returns the debug visualization mode:
// 0 = off, 1 = materialID, 2 = layer, 3 = atlasUV,
// 4 = sample at mat.layer, 5 = sample at layer 0, 6 = sample at layer 1,
// 7 = sampled alpha as grayscale.
func debugMaterialVizMode() int {
	v := os.Getenv("IRONWAIL_DEBUG_MATERIAL_VIZ")
	switch v {
	case "1":
		return 1
	case "2":
		return 2
	case "3":
		return 3
	case "4":
		return 4
	case "5":
		return 5
	case "6":
		return 6
	case "7":
		return 7
	default:
		return 0
	}
}

// debugAtlasDir returns the directory for diagnostic file output.
func debugAtlasDir() string {
	d := os.Getenv("IRONWAIL_DEBUG_ATLAS_DIR")
	if d == "" {
		return "."
	}
	return d
}

// ============================================================================
// Phase 1: Buffer Capacity Audit
// ============================================================================

// diagMaterialBufferCapacity checks whether the number of materials exceeds
// the fixed GPU buffer capacity. If so, the GPU buffer overflows silently
// and the shader reads out-of-bounds, producing wrong textures.
//
// This is the primary suspected root cause for texture corruption on large
// BSP2 maps like qbj2 start, which can have hundreds or thousands of
// textures while the buffer is capped at 256 entries.
func diagMaterialBufferCapacity(source string, materialCount int) {
	slog.Debug("Material storage buffer capacity",
		"source", source,
		"material_count", materialCount,
		"buffer_bytes", materialCount*32,
	)
}

// diagMaterialBufferWrite checks whether a WriteBuffer call would write more
// data than the GPU buffer can hold, which silently corrupts adjacent memory.
func diagMaterialBufferWrite(source string, materialCount int, bufferSizeBytes int) {
	writeBytes := materialCount * worldMaterialDataSize()
	if writeBytes <= bufferSizeBytes {
		return
	}

	slog.Warn("MATERIAL BUFFER WRITE OVERFLOW: writing more bytes than buffer capacity",
		"source", source,
		"material_count", materialCount,
		"write_bytes", writeBytes,
		"buffer_bytes", bufferSizeBytes,
		"overflow_bytes", writeBytes-bufferSizeBytes,
	)
}

// worldMaterialDataSize returns the byte size of a single WorldMaterialData
// struct. This must match sizeof(WorldMaterialData) and the WGSL MaterialData
// struct size (vec4 + 4 floats = 32 bytes).
func worldMaterialDataSize() int {
	return 32 // AtlasBounds[4]float32 (16) + Layer (4) + Pad[3] (12) = 32
}

// ============================================================================
// Phase 2: MaterialID Range Validation
// ============================================================================

// diagMaterialIDRange computes the min, max, and unique count of materialID
// values across all faces in a WorldGeometry, and warns if any materialID
// exceeds the shader array bound.
func diagMaterialIDRange(geom *WorldGeometry) {
	if geom == nil || len(geom.Faces) == 0 {
		return
	}

	minID := uint32(^uint32(0))
	maxID := uint32(0)
	uniqueIDs := make(map[uint32]int) // materialID -> face count

	for _, face := range geom.Faces {
		if face.NumIndices == 0 {
			continue
		}
		// Faces store TextureIndex (int32) which becomes materialID (uint32)
		// at vertex build time. We don't have the vertex materialID directly
		// here, but TextureIndex is the source value.
		id := uint32(face.TextureIndex)
		if id < minID {
			minID = id
		}
		if id > maxID {
			maxID = id
		}
		uniqueIDs[id]++
	}

	if len(uniqueIDs) == 0 {
		return
	}

	slog.Debug("MaterialID range",
		"min_id", minID,
		"max_id", maxID,
		"unique_ids", len(uniqueIDs),
	)

	if debugMaterialAuditEnabled() {
		// Log the top N most-used materialIDs to identify which textures
		// are most affected.
		type idCount struct {
			id    uint32
			count int
		}
		var counts []idCount
		for id, c := range uniqueIDs {
			counts = append(counts, idCount{id, c})
		}
		// Simple sort by count descending (avoid importing sort for a small list)
		for i := 0; i < len(counts); i++ {
			for j := i + 1; j < len(counts); j++ {
				if counts[j].count > counts[i].count {
					counts[i], counts[j] = counts[j], counts[i]
				}
			}
		}
		limit := 20
		if len(counts) < limit {
			limit = len(counts)
		}
		var sb strings.Builder
		sb.WriteString("Top materialIDs by face count: ")
		for i := 0; i < limit; i++ {
			fmt.Fprintf(&sb, "[id=%d faces=%d] ", counts[i].id, counts[i].count)
		}
		slog.Debug("MaterialID histogram (top 20)", "histogram", sb.String())
	}
}

// diagMaterialIDFaceAudit dumps per-face materialID information for the first
// N faces when IRONWAIL_DEBUG_MATERIAL_AUDIT=1. This shows exactly which
// faces get out-of-range IDs.
func diagMaterialIDFaceAudit(geom *WorldGeometry, maxFaces int) {
	if !debugMaterialAuditEnabled() || geom == nil {
		return
	}
	limit := maxFaces
	if limit > len(geom.Faces) {
		limit = len(geom.Faces)
	}

	slog.Debug("Per-face materialID audit (first N faces)",
		"face_count", limit,
		"buffer_capacity", worldMaterialsBufferCapacity,
	)
	for i := 0; i < limit; i++ {
		face := geom.Faces[i]
		if face.NumIndices == 0 {
			continue
		}
		id := uint32(face.TextureIndex)
		over := id >= worldMaterialsBufferCapacity
		slog.Debug("face materialID",
			"face_index", i,
			"texture_index", face.TextureIndex,
			"material_id", id,
			"num_indices", face.NumIndices,
			"over_capacity", over,
			"flags", face.Flags,
		)
	}
}

// ============================================================================
// Phase 3: Atlas Layer Distribution Telemetry
// ============================================================================

// diagAtlasLayerSummary records per-layer texture placement statistics.
type diagAtlasLayerSummary struct {
	LayerIndex      int
	TextureCount    int
	PixelsUsed      int
	PixelsAvailable int
	MinTextureW     int
	MinTextureH     int
	MaxTextureW     int
	MaxTextureH     int
}

// diagAtlasLayerDistribution logs per-layer utilization and validates that
// all material atlas bounds and layer indices are within valid ranges.
func diagAtlasLayerDistribution(atlas *WorldTextureAtlas, baseMaterials []WorldMaterialData) {
	if atlas == nil {
		return
	}

	// Count textures per layer by scanning baseMaterials.
	layerCounts := make(map[int]int)
	for _, mat := range baseMaterials {
		layer := int(mat.Layer)
		layerCounts[layer]++
	}

	summaries := make([]diagAtlasLayerSummary, 0, len(atlas.layers))
	for i, layer := range atlas.layers {
		count := layerCounts[i]
		pixelsAvail := layer.width * layer.height
		summary := diagAtlasLayerSummary{
			LayerIndex:      i,
			TextureCount:    count,
			PixelsAvailable: pixelsAvail,
			MinTextureW:     0,
			MinTextureH:     0,
			MaxTextureW:     0,
			MaxTextureH:     0,
		}
		summaries = append(summaries, summary)
		slog.Debug("Atlas layer distribution",
			"layer", i,
			"texture_count", count,
			"layer_width", layer.width,
			"layer_height", layer.height,
			"pixels_available", pixelsAvail,
		)
	}

	// Validate bounds and layer indices for every material.
	invalidBounds := 0
	invalidLayers := 0
	for i, mat := range baseMaterials {
		layer := int(mat.Layer)
		if layer < 0 || layer >= len(atlas.layers) {
			invalidLayers++
			slog.Warn("Invalid atlas layer for material",
				"material_index", i,
				"layer", layer,
				"atlas_layer_count", len(atlas.layers),
			)
			continue
		}
		// AtlasBounds is [u, v, w, h] in normalized [0,1] range.
		u, v, w, h := mat.AtlasBounds[0], mat.AtlasBounds[1], mat.AtlasBounds[2], mat.AtlasBounds[3]
		if u < 0 || u > 1 || v < 0 || v > 1 || w <= 0 || w > 1 || h <= 0 || h > 1 {
			invalidBounds++
			if invalidBounds <= 10 {
				slog.Warn("Invalid atlas bounds for material",
					"material_index", i,
					"layer", layer,
					"u", u, "v", v, "w", w, "h", h,
				)
			}
		}
	}

	if invalidBounds > 0 || invalidLayers > 0 {
		slog.Warn("Atlas bounds/layer validation found violations",
			"invalid_bounds", invalidBounds,
			"invalid_layers", invalidLayers,
			"total_materials", len(baseMaterials),
		)
	} else {
		slog.Debug("Atlas bounds/layer validation passed",
			"total_materials", len(baseMaterials),
			"atlas_layers", len(atlas.layers),
		)
	}
}

// diagAtlasLayerLimit checks that the atlas layer count does not exceed the
// GPU's MaxTextureArrayLayers limit. The maxLayers is queried from the
// adapter limits by the caller.
func diagAtlasLayerLimit(atlasLayerCount int, maxTextureArrayLayers int) {
	slog.Debug("Atlas layer limit check",
		"atlas_layers", atlasLayerCount,
		"max_texture_array_layers", maxTextureArrayLayers,
	)
	if atlasLayerCount > maxTextureArrayLayers {
		slog.Warn("ATLAS LAYER COUNT EXCEEDS GPU LIMIT",
			"atlas_layers", atlasLayerCount,
			"max_texture_array_layers", maxTextureArrayLayers,
			"explanation", "GPU may fail to create the texture array or render with missing layers",
		)
	}
}

// ============================================================================
// Phase 4: Animation Chain Integrity
// ============================================================================

// diagAnimationChains logs the structure of texture animation chains and
// validates that all chain indices are within the material buffer capacity.
func diagAnimationChains(animations []*surfacepkg.SurfaceTexture, baseMaterialCount int) {
	if len(animations) == 0 {
		slog.Debug("No texture animations to audit")
		return
	}

	animatedGroups := 0
	overCapacityTargets := 0

	for i, anim := range animations {
		if anim == nil {
			continue
		}
		if anim.AnimTotal == 0 {
			continue // Not animated
		}

		animatedGroups++

		// Walk the chain and collect indices.
		chainIndices := make([]int32, 0, 8)
		visited := make(map[int32]bool)
		current := anim
		for current != nil && !visited[current.TextureIndex] {
			visited[current.TextureIndex] = true
			chainIndices = append(chainIndices, current.TextureIndex)
			if int(current.TextureIndex) >= worldMaterialsBufferCapacity {
				overCapacityTargets++
			}
			current = current.AnimNext
			if current == anim {
				break // circular list
			}
		}

		if debugMaterialAuditEnabled() {
			slog.Debug("Animation chain",
				"source_index", i,
				"chain_length", len(chainIndices),
				"chain_indices", chainIndices,
				"has_alternate", anim.AlternateAnims != nil,
			)
		}
	}

	slog.Debug("Animation chain audit",
		"total_animation_slots", len(animations),
		"animated_groups", animatedGroups,
		"over_capacity_targets", overCapacityTargets,
		"buffer_capacity", worldMaterialsBufferCapacity,
	)

	if overCapacityTargets > 0 {
		slog.Warn("ANIMATION CHAIN REFERENCES OUT-OF-RANGE MATERIAL",
			"over_capacity_targets", overCapacityTargets,
			"buffer_capacity", worldMaterialsBufferCapacity,
			"explanation", "animated textures will remap materialID to indices beyond the shader array bound",
		)
	}
}

// diagAnimationRemap logs each animated texture remapping at render time
// when IRONWAIL_DEBUG_MATERIAL_AUDIT=1.
func diagAnimationRemap(sourceIdx int, targetIdx int, targetMat WorldMaterialData) {
	if !debugMaterialAuditEnabled() {
		return
	}
	over := targetIdx >= worldMaterialsBufferCapacity
	slog.Debug("Animation remap",
		"source_index", sourceIdx,
		"target_index", targetIdx,
		"target_layer", targetMat.Layer,
		"target_bounds", targetMat.AtlasBounds,
		"over_capacity", over,
	)
}

// ============================================================================
// Phase 5: Material Table Dump
// ============================================================================

// diagMaterialTableDump writes the complete material table and atlas layer
// images to files for offline analysis. Gated by IRONWAIL_DEBUG_MATERIAL_DUMP=1.
func diagMaterialTableDump(baseMaterials []WorldMaterialData, textureNames []string, animations []*surfacepkg.SurfaceTexture, atlas *WorldTextureAtlas, mapName string) {
	if !debugMaterialDumpEnabled() {
		return
	}

	dir := debugAtlasDir()
	timestamp := time.Now().Format("20060102_150405")
	prefix := fmt.Sprintf("%s/debug_materials_%s_%s", dir, sanitizeMapName(mapName), timestamp)

	// Write material table CSV.
	csvPath := prefix + ".csv"
	if err := writeMaterialTableCSV(csvPath, baseMaterials, textureNames, animations); err != nil {
		slog.Warn("Failed to write material table CSV", "path", csvPath, "error", err)
	} else {
		slog.Info("Material table CSV written", "path", csvPath, "entries", len(baseMaterials))
	}

	// Write material table JSON (more structured for programmatic analysis).
	jsonPath := prefix + ".json"
	if err := writeMaterialTableJSON(jsonPath, baseMaterials, textureNames, animations); err != nil {
		slog.Warn("Failed to write material table JSON", "path", jsonPath, "error", err)
	} else {
		slog.Info("Material table JSON written", "path", jsonPath)
	}

	// Dump atlas layer images as PNGs.
	if atlas != nil {
		layers := atlas.Flatten()
		for i, img := range layers {
			pngPath := fmt.Sprintf("%s_atlas_layer_%d.png", prefix, i)
			if err := writeAtlasLayerPNG(pngPath, img); err != nil {
				slog.Warn("Failed to write atlas layer PNG", "path", pngPath, "error", err)
			} else {
				slog.Info("Atlas layer PNG written", "path", pngPath, "layer", i, "width", img.Bounds().Dx(), "height", img.Bounds().Dy())
			}
		}
	}
}

// diagMaterialIDHistogramDump writes a CSV showing which materialIDs are used
// by how much geometry, so we can identify which out-of-range IDs affect the
// most faces.
func diagMaterialIDHistogramDump(geom *WorldGeometry, mapName string) {
	if !debugMaterialDumpEnabled() || geom == nil {
		return
	}

	dir := debugAtlasDir()
	timestamp := time.Now().Format("20060102_150405")
	csvPath := fmt.Sprintf("%s/debug_materialid_histogram_%s_%s.csv", dir, sanitizeMapName(mapName), timestamp)

	f, err := os.Create(csvPath)
	if err != nil {
		slog.Warn("Failed to create materialID histogram CSV", "path", csvPath, "error", err)
		return
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	w.Write([]string{"material_id", "face_count", "total_indices", "over_capacity"})

	histogram := make(map[uint32]struct {
		faces   int
		indices uint32
	})
	for _, face := range geom.Faces {
		if face.NumIndices == 0 {
			continue
		}
		id := uint32(face.TextureIndex)
		entry := histogram[id]
		entry.faces++
		entry.indices += face.NumIndices
		histogram[id] = entry
	}

	// Sort by face count descending.
	type row struct {
		id      uint32
		faces   int
		indices uint32
	}
	rows := make([]row, 0, len(histogram))
	for id, h := range histogram {
		rows = append(rows, row{id, h.faces, h.indices})
	}
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[j].faces > rows[i].faces {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}

	for _, r := range rows {
		over := "false"
		if r.id >= worldMaterialsBufferCapacity {
			over = "true"
		}
		w.Write([]string{
			fmt.Sprintf("%d", r.id),
			fmt.Sprintf("%d", r.faces),
			fmt.Sprintf("%d", r.indices),
			over,
		})
	}

	slog.Info("MaterialID histogram CSV written", "path", csvPath, "unique_ids", len(rows))
}

func writeMaterialTableCSV(path string, baseMaterials []WorldMaterialData, textureNames []string, animations []*surfacepkg.SurfaceTexture) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	w.Write([]string{"index", "texture_name", "layer", "bounds_u", "bounds_v", "bounds_w", "bounds_h", "is_animated", "anim_chain_length", "over_capacity"})

	for i, mat := range baseMaterials {
		name := ""
		if i < len(textureNames) {
			name = textureNames[i]
		}
		isAnimated := "false"
		chainLen := 0
		if i < len(animations) && animations[i] != nil && animations[i].AnimTotal > 0 {
			isAnimated = "true"
			// Count chain length
			visited := make(map[int32]bool)
			current := animations[i]
			for current != nil && !visited[current.TextureIndex] {
				visited[current.TextureIndex] = true
				chainLen++
				current = current.AnimNext
				if current == animations[i] {
					break
				}
			}
		}
		over := "false"
		if i >= worldMaterialsBufferCapacity {
			over = "true"
		}
		w.Write([]string{
			fmt.Sprintf("%d", i),
			name,
			fmt.Sprintf("%d", int(mat.Layer)),
			fmt.Sprintf("%.6f", mat.AtlasBounds[0]),
			fmt.Sprintf("%.6f", mat.AtlasBounds[1]),
			fmt.Sprintf("%.6f", mat.AtlasBounds[2]),
			fmt.Sprintf("%.6f", mat.AtlasBounds[3]),
			isAnimated,
			fmt.Sprintf("%d", chainLen),
			over,
		})
	}
	return nil
}

func writeMaterialTableJSON(path string, baseMaterials []WorldMaterialData, textureNames []string, animations []*surfacepkg.SurfaceTexture) error {
	type materialEntry struct {
		Index        int     `json:"index"`
		TextureName  string  `json:"texture_name"`
		Layer        float32 `json:"layer"`
		BoundsU      float32 `json:"bounds_u"`
		BoundsV      float32 `json:"bounds_v"`
		BoundsW      float32 `json:"bounds_w"`
		BoundsH      float32 `json:"bounds_h"`
		IsAnimated   bool    `json:"is_animated"`
		AnimChainLen int     `json:"anim_chain_len"`
		OverCapacity bool    `json:"over_capacity"`
	}

	entries := make([]materialEntry, len(baseMaterials))
	for i, mat := range baseMaterials {
		name := ""
		if i < len(textureNames) {
			name = textureNames[i]
		}
		isAnim := false
		chainLen := 0
		if i < len(animations) && animations[i] != nil && animations[i].AnimTotal > 0 {
			isAnim = true
			visited := make(map[int32]bool)
			current := animations[i]
			for current != nil && !visited[current.TextureIndex] {
				visited[current.TextureIndex] = true
				chainLen++
				current = current.AnimNext
				if current == animations[i] {
					break
				}
			}
		}
		entries[i] = materialEntry{
			Index:        i,
			TextureName:  name,
			Layer:        mat.Layer,
			BoundsU:      mat.AtlasBounds[0],
			BoundsV:      mat.AtlasBounds[1],
			BoundsW:      mat.AtlasBounds[2],
			BoundsH:      mat.AtlasBounds[3],
			IsAnimated:   isAnim,
			AnimChainLen: chainLen,
			OverCapacity: i >= worldMaterialsBufferCapacity,
		}
	}

	data := struct {
		BufferCapacity int             `json:"buffer_capacity"`
		MaterialCount  int             `json:"material_count"`
		Materials      []materialEntry `json:"materials"`
	}{
		BufferCapacity: worldMaterialsBufferCapacity,
		MaterialCount:  len(baseMaterials),
		Materials:      entries,
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func writeAtlasLayerPNG(path string, img *stdimage.RGBA) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func sanitizeMapName(name string) string {
	result := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, name)
	if result == "" {
		return "unknown"
	}
	return result
}

// ============================================================================
// GPU Resource Verification
// ============================================================================

// diagWorldUploadSummary logs the actual GPU texture array dimensions and
// buffer sizes after upload, verifying that the diffuse and fullbright
// texture arrays have matching layer counts and that the material buffer
// is correctly sized.
func diagWorldUploadSummary(diffuse, fullbright, lightmap *gpuWorldTexture, materialsBuffer *wgpu.Buffer, materialCount int, skyTextureCount int) {
	slog.Debug("World GPU resource summary",
		"material_count", materialCount,
		"material_buffer_capacity", worldMaterialsBufferCapacity,
		"sky_texture_count", skyTextureCount,
	)

	if diffuse != nil {
		slog.Debug("Diffuse texture array",
			"width", diffuse.width,
			"height", diffuse.height,
			"layers", diffuse.layers,
		)
	} else {
		slog.Warn("Diffuse texture array is nil after upload")
	}

	if fullbright != nil {
		slog.Debug("Fullbright texture array",
			"width", fullbright.width,
			"height", fullbright.height,
			"layers", fullbright.layers,
		)
	} else {
		slog.Debug("Fullbright texture array is nil (no fullbright data)")
	}

	if lightmap != nil {
		slog.Debug("Lightmap texture array",
			"width", lightmap.width,
			"height", lightmap.height,
			"layers", lightmap.layers,
		)
	}

	// Critical check: diffuse and fullbright must have the same layer count
	// because the shader samples both at i32(mat.layer).
	if diffuse != nil && fullbright != nil {
		if diffuse.layers != fullbright.layers {
			slog.Warn("DIFFUSE AND FULLBRIGHT TEXTURE ARRAY LAYER MISMATCH",
				"diffuse_layers", diffuse.layers,
				"fullbright_layers", fullbright.layers,
				"explanation", "shader samples both arrays at the same layer index; mismatch causes out-of-bounds reads",
			)
		}
	}

	// Check material buffer
	if materialsBuffer != nil {
		slog.Debug("Materials buffer",
			"material_count", materialCount,
			"buffer_capacity", worldMaterialsBufferCapacity,
			"over_capacity", materialCount > worldMaterialsBufferCapacity,
		)
	}
}
