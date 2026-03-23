package renderer

import (
	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/model"
	"strconv"
	"strings"
)

type worldLiquidAlphaOverrides struct {
	hasWater bool
	water    float32
	hasLava  bool
	lava     float32
	hasSlime bool
	slime    float32
	hasTele  bool
	tele     float32
}

// worldLiquidAlphaSettingsFromCvars reads liquid alpha cvars (r_wateralpha etc.) and applies worldspawn overrides to compute the final per-liquid-type alpha settings.
func worldLiquidAlphaSettingsFromCvars(overrides worldLiquidAlphaOverrides, tree *bsp.Tree) worldLiquidAlphaSettings {
	return resolveWorldLiquidAlphaSettings(
		readWorldAlphaCvar(CvarRWaterAlpha, 1),
		readWorldAlphaCvar(CvarRLavaAlpha, 0),
		readWorldAlphaCvar(CvarRSlimeAlpha, 0),
		readWorldAlphaCvar(CvarRTeleAlpha, 0),
		overrides,
		tree,
	)
}

// resolveWorldLiquidAlphaSettings resolves final liquid alpha values with fallback logic: slime/lava/tele default to the water alpha if their cvar is 0, and worldspawn overrides can force specific values.
func resolveWorldLiquidAlphaSettings(cvarWater, cvarLava, cvarSlime, cvarTele float32, overrides worldLiquidAlphaOverrides, tree *bsp.Tree) worldLiquidAlphaSettings {
	water := clamp01(cvarWater)
	if overrides.hasWater {
		water = clamp01(overrides.water)
	}
	fallback := water

	lava := fallback
	if cvarLava > 0 {
		lava = clamp01(cvarLava)
	}
	if overrides.hasLava {
		if overrides.lava > 0 {
			lava = clamp01(overrides.lava)
		} else {
			lava = fallback
		}
	}

	slime := fallback
	if cvarSlime > 0 {
		slime = clamp01(cvarSlime)
	}
	if overrides.hasSlime {
		if overrides.slime > 0 {
			slime = clamp01(overrides.slime)
		} else {
			slime = fallback
		}
	}

	tele := fallback
	if cvarTele > 0 {
		tele = clamp01(cvarTele)
	}
	if overrides.hasTele {
		if overrides.tele > 0 {
			tele = clamp01(overrides.tele)
		} else {
			tele = fallback
		}
	}

	settings := worldLiquidAlphaSettings{water: water, lava: lava, slime: slime, tele: tele}

	// Force opaque if map is not vis-safe for transparent water
	if !mapVisTransparentWaterSafe(tree) {
		settings.water = 1.0
		settings.lava = 1.0
		settings.slime = 1.0
		settings.tele = 1.0
	}

	return settings
}

// parseWorldspawnLiquidAlphaOverrides parses the BSP entity lump's worldspawn for liquid alpha override keys.
func parseWorldspawnLiquidAlphaOverrides(entities []byte) worldLiquidAlphaOverrides {
	if len(entities) == 0 {
		return worldLiquidAlphaOverrides{}
	}

	entity, ok := firstEntityLumpObject(string(entities))
	if !ok {
		return worldLiquidAlphaOverrides{}
	}

	fields := parseEntityFields(entity)
	if !strings.EqualFold(fields["classname"], "worldspawn") {
		return worldLiquidAlphaOverrides{}
	}

	var overrides worldLiquidAlphaOverrides
	if value, ok := parseEntityAlphaField(fields, "wateralpha"); ok {
		overrides.hasWater = true
		overrides.water = value
	}
	if value, ok := parseEntityAlphaField(fields, "lavaalpha"); ok {
		overrides.hasLava = true
		overrides.lava = value
	}
	if value, ok := parseEntityAlphaField(fields, "slimealpha"); ok {
		overrides.hasSlime = true
		overrides.slime = value
	}
	if value, ok := parseEntityAlphaField(fields, "telealpha"); ok {
		overrides.hasTele = true
		overrides.tele = value
	}

	return overrides
}

// worldspawnTransparentWaterOverride reads the _watervis worldspawn key to determine if the map explicitly supports transparent water rendering.
func worldspawnTransparentWaterOverride(entities []byte) (bool, bool) {
	entity, ok := firstEntityLumpObject(string(entities))
	if !ok {
		return false, false
	}
	fields := parseEntityFields(entity)
	if !strings.EqualFold(fields["classname"], "worldspawn") {
		return false, false
	}
	for _, key := range []string{"transwater", "watervis"} {
		if value, ok := parseEntityBoolField(fields, key); ok {
			return value, true
		}
	}
	return false, false
}

// mapVisTransparentWaterSafe determines if the map's visibility data is compiled for transparent water.
// In Quake 1, transparent water requires special VIS-time flags; maps without them render water opaque to prevent rendering errors.
// Returns true if map is safe for transparent liquids, false if opaque should be forced.
func mapVisTransparentWaterSafe(tree *bsp.Tree) bool {
	if tree == nil {
		return true
	}

	if override, ok := worldspawnTransparentWaterOverride(tree.Entities); ok {
		return override
	}

	contentTransparent, contentFound := worldLiquidVisibilityMasks(tree)
	if contentFound == 0 {
		return true
	}
	return contentTransparent&contentFound == contentFound
}

// worldLiquidVisibilityMasks scans BSP leaves to determine which liquid content types exist and which are marked transparent in the PVS data.
func worldLiquidVisibilityMasks(tree *bsp.Tree) (contentTransparent, contentFound int32) {
	if tree == nil || len(tree.Leafs) <= 1 {
		return 0, 0
	}
	leafCount := len(tree.Leafs) - 1
	for leafIndex := 1; leafIndex < len(tree.Leafs); leafIndex++ {
		leaf := tree.Leafs[leafIndex]
		contentType := liquidTypeForLeaf(tree, leaf)
		if contentType == 0 {
			continue
		}
		contentFound |= contentType
		if contentTransparent&contentType != 0 {
			continue
		}
		visibleMask := decompressLeafVisibility(tree.Visibility, leaf.VisOfs, leafCount)
		for visibleIndex := 1; visibleIndex < len(tree.Leafs); visibleIndex++ {
			if !leafVisibleInMask(visibleMask, visibleIndex-1) {
				continue
			}
			visibleContentType := liquidTypeForLeaf(tree, tree.Leafs[visibleIndex])
			if visibleContentType&contentType == 0 {
				contentTransparent |= contentType
				break
			}
		}
	}
	return contentTransparent, contentFound
}

// liquidTypeForLeaf determines the liquid content type of a BSP leaf from its contents field (water, slime, lava, or none).
func liquidTypeForLeaf(tree *bsp.Tree, leaf bsp.TreeLeaf) int32 {
	switch leaf.Contents {
	case bsp.ContentsWater:
		return liquidWaterOrTeleTypeForLeaf(tree, leaf)
	case bsp.ContentsLava:
		return model.SurfDrawLava
	case bsp.ContentsSlime:
		return model.SurfDrawSlime
	default:
		return 0
	}
}

// liquidWaterOrTeleTypeForLeaf checks if a BSP leaf contains water or teleporter liquid content.
func liquidWaterOrTeleTypeForLeaf(tree *bsp.Tree, leaf bsp.TreeLeaf) int32 {
	start := int(leaf.FirstMarkSurface)
	count := int(leaf.NumMarkSurfaces)
	if start < 0 || count <= 0 || start >= len(tree.MarkSurfaces) {
		return 0
	}
	end := min(start+count, len(tree.MarkSurfaces))
	for i := start; i < end; i++ {
		faceIndex := tree.MarkSurfaces[i]
		if faceIndex < 0 || faceIndex >= len(tree.Faces) {
			continue
		}
		texinfoIndex := tree.Faces[faceIndex].Texinfo
		if texinfoIndex < 0 || int(texinfoIndex) >= len(tree.Texinfo) {
			continue
		}
		flags := tree.Texinfo[texinfoIndex].Flags & (model.SurfDrawWater | model.SurfDrawTele)
		if flags != 0 {
			return flags
		}
	}
	return 0
}

// readWorldAlphaCvar reads a liquid alpha cvar value with a fallback default for when the cvar is unset.
func readWorldAlphaCvar(name string, fallback float32) float32 {
	cv := cvar.Get(name)
	if cv == nil {
		return clamp01(fallback)
	}
	return clamp01(cv.Float32())
}

// decompressLeafVisibility decompresses a BSP PVS (Potentially Visible Set) bitstring using Quake's run-length encoding. Each bit represents whether a leaf is visible from the given leaf.
func decompressLeafVisibility(visibility []byte, visOfs int32, leafCount int) []byte {
	maskBytes := (leafCount + 7) / 8
	if maskBytes <= 0 {
		return nil
	}
	mask := make([]byte, maskBytes)
	if len(visibility) == 0 || visOfs < 0 || int(visOfs) >= len(visibility) {
		for i := range mask {
			mask[i] = 0xFF
		}
		return mask
	}
	in := int(visOfs)
	out := 0
	for out < len(mask) && in < len(visibility) {
		b := visibility[in]
		in++
		if b != 0 {
			mask[out] = b
			out++
			continue
		}
		if in >= len(visibility) {
			break
		}
		run := int(visibility[in])
		in++
		out += min(run, len(mask)-out)
	}
	return mask
}

// parseEntityAlphaField parses a floating-point alpha value from an entity key-value field.
func parseEntityAlphaField(fields map[string]string, key string) (float32, bool) {
	value, ok := fields[key]
	if !ok {
		value, ok = fields["_"+key]
		if !ok {
			return 0, false
		}
	}
	f, err := strconv.ParseFloat(value, 32)
	if err != nil {
		return 0, false
	}
	return float32(f), true
}

// parseEntityBoolField parses a boolean entity field using Quake's convention: 0 means false, any non-zero value means true.
func parseEntityBoolField(fields map[string]string, key string) (bool, bool) {
	value, ok := fields[key]
	if !ok {
		value, ok = fields["_"+key]
		if !ok {
			return false, false
		}
	}
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	}
	f, err := strconv.ParseFloat(value, 32)
	if err != nil {
		return false, false
	}
	return f != 0, true
}

// parseEntityFields parses key-value pairs from a Quake entity definition string into a map.
func parseEntityFields(data string) map[string]string {
	fields := make(map[string]string)
	pos := 0
	for {
		key, next, ok := nextQuotedEntityToken(data, pos)
		if !ok {
			break
		}
		value, nextValue, ok := nextQuotedEntityToken(data, next)
		if !ok {
			break
		}
		fields[strings.ToLower(key)] = value
		pos = nextValue
	}
	return fields
}

// firstEntityLumpObject extracts the first entity block (the worldspawn) from the BSP entity lump.
func firstEntityLumpObject(data string) (string, bool) {
	start := strings.IndexByte(data, '{')
	if start < 0 {
		return "", false
	}
	end := strings.IndexByte(data[start+1:], '}')
	if end < 0 {
		return "", false
	}
	return data[start+1 : start+1+end], true
}

// nextQuotedEntityToken extracts the next double-quoted string token from Quake entity lump data.
func nextQuotedEntityToken(data string, pos int) (string, int, bool) {
	start := strings.IndexByte(data[pos:], '"')
	if start < 0 {
		return "", pos, false
	}
	start += pos
	end := strings.IndexByte(data[start+1:], '"')
	if end < 0 {
		return "", pos, false
	}
	end += start + 1
	return data[start+1 : end], end + 1, true
}
