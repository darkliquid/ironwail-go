package world

import (
	"strings"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
)

const (
	CvarWaterAlpha = "r_wateralpha"
	CvarLavaAlpha  = "r_lavaalpha"
	CvarSlimeAlpha = "r_slimealpha"
	CvarTeleAlpha  = "r_telealpha"
)

// LiquidAlphaSettings stores per-liquid-type alpha values from cvars.
type LiquidAlphaSettings struct {
	Water float32
	Lava  float32
	Slime float32
	Tele  float32
}

// LiquidAlphaOverrides stores worldspawn overrides for per-liquid alpha settings.
type LiquidAlphaOverrides struct {
	HasWater bool
	Water    float32
	HasLava  bool
	Lava     float32
	HasSlime bool
	Slime    float32
	HasTele  bool
	Tele     float32
}

func ReadLiquidAlphaSettings(overrides LiquidAlphaOverrides, tree *bsp.Tree) LiquidAlphaSettings {
	return ResolveLiquidAlphaSettings(
		ReadAlphaCvar(CvarWaterAlpha, 1),
		ReadAlphaCvar(CvarLavaAlpha, 0),
		ReadAlphaCvar(CvarSlimeAlpha, 0),
		ReadAlphaCvar(CvarTeleAlpha, 0),
		overrides,
		tree,
	)
}

func ResolveLiquidAlphaSettings(cvarWater, cvarLava, cvarSlime, cvarTele float32, overrides LiquidAlphaOverrides, tree *bsp.Tree) LiquidAlphaSettings {
	water := clamp01(cvarWater)
	if overrides.HasWater {
		water = clamp01(overrides.Water)
	}
	fallback := water

	lava := fallback
	if cvarLava > 0 {
		lava = clamp01(cvarLava)
	}
	if overrides.HasLava {
		if overrides.Lava > 0 {
			lava = clamp01(overrides.Lava)
		} else {
			lava = fallback
		}
	}

	slime := fallback
	if cvarSlime > 0 {
		slime = clamp01(cvarSlime)
	}
	if overrides.HasSlime {
		if overrides.Slime > 0 {
			slime = clamp01(overrides.Slime)
		} else {
			slime = fallback
		}
	}

	tele := fallback
	if cvarTele > 0 {
		tele = clamp01(cvarTele)
	}
	if overrides.HasTele {
		if overrides.Tele > 0 {
			tele = clamp01(overrides.Tele)
		} else {
			tele = fallback
		}
	}

	settings := LiquidAlphaSettings{Water: water, Lava: lava, Slime: slime, Tele: tele}
	if !MapVisTransparentWaterSafe(tree) {
		settings.Water = 1
		settings.Lava = 1
		settings.Slime = 1
		settings.Tele = 1
	}
	return settings
}

func ParseWorldspawnLiquidAlphaOverrides(entities []byte) LiquidAlphaOverrides {
	if len(entities) == 0 {
		return LiquidAlphaOverrides{}
	}

	entity, ok := FirstEntityLumpObject(string(entities))
	if !ok {
		return LiquidAlphaOverrides{}
	}

	fields := ParseEntityFields(entity)
	if !strings.EqualFold(fields["classname"], "worldspawn") {
		return LiquidAlphaOverrides{}
	}

	var overrides LiquidAlphaOverrides
	if value, ok := ParseEntityAlphaField(fields, "wateralpha"); ok {
		overrides.HasWater = true
		overrides.Water = value
	}
	if value, ok := ParseEntityAlphaField(fields, "lavaalpha"); ok {
		overrides.HasLava = true
		overrides.Lava = value
	}
	if value, ok := ParseEntityAlphaField(fields, "slimealpha"); ok {
		overrides.HasSlime = true
		overrides.Slime = value
	}
	if value, ok := ParseEntityAlphaField(fields, "telealpha"); ok {
		overrides.HasTele = true
		overrides.Tele = value
	}
	return overrides
}

func MapVisTransparentWaterSafe(tree *bsp.Tree) bool {
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

func worldspawnTransparentWaterOverride(entities []byte) (bool, bool) {
	entity, ok := FirstEntityLumpObject(string(entities))
	if !ok {
		return false, false
	}
	fields := ParseEntityFields(entity)
	if !strings.EqualFold(fields["classname"], "worldspawn") {
		return false, false
	}
	for _, key := range []string{"transwater", "watervis"} {
		if value, ok := ParseEntityBoolField(fields, key); ok {
			return value, true
		}
	}
	return false, false
}

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
			if !LeafVisibleInMask(visibleMask, visibleIndex-1) {
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

func liquidWaterOrTeleTypeForLeaf(tree *bsp.Tree, leaf bsp.TreeLeaf) int32 {
	start := int(leaf.FirstMarkSurface)
	count := int(leaf.NumMarkSurfaces)
	if start < 0 || count <= 0 || start >= len(tree.MarkSurfaces) {
		return 0
	}
	end := minInt(start+count, len(tree.MarkSurfaces))
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
		out += minInt(run, len(mask)-out)
	}
	return mask
}
