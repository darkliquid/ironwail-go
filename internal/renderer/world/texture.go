package world

import (
	"encoding/binary"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/model"
)

// TextureMeta holds parsed texture metadata from BSP miptex entries.
type TextureMeta struct {
	Width  int
	Height int
	Name   string
	Type   model.TextureType
}

type MaterialTextureRGBA struct {
	DiffuseRGBA    []byte
	FullbrightRGBA []byte
	HasFullbright  bool
}

// ParseTextureMeta parses the BSP miptex lump to extract texture names and dimensions.
func ParseTextureMeta(tree *bsp.Tree) []TextureMeta {
	if tree == nil || len(tree.TextureData) < 4 {
		return nil
	}

	count := int(binary.LittleEndian.Uint32(tree.TextureData[:4]))
	if count <= 0 || len(tree.TextureData) < 4+count*4 {
		return nil
	}

	textures := make([]TextureMeta, count)
	for i := 0; i < count; i++ {
		offset := int(int32(binary.LittleEndian.Uint32(tree.TextureData[4+i*4:])))
		if offset <= 0 || offset >= len(tree.TextureData) {
			continue
		}
		miptex, err := image.ParseMipTex(tree.TextureData[offset:])
		if err != nil {
			continue
		}
		textures[i] = TextureMeta{
			Width:  int(miptex.Width),
			Height: int(miptex.Height),
			Name:   miptex.Name,
			Type:   ClassifyTextureName(miptex.Name),
		}
	}
	return textures
}

// ClassifyTextureName classifies a texture by Quake naming conventions.
func ClassifyTextureName(name string) model.TextureType {
	name = strings.TrimRight(strings.ToLower(name), "\x00")
	switch {
	case strings.HasPrefix(name, "{"):
		return model.TexTypeCutout
	case strings.HasPrefix(name, "sky"):
		return model.TexTypeSky
	case strings.HasPrefix(name, "*lava"):
		return model.TexTypeLava
	case strings.HasPrefix(name, "*slime"):
		return model.TexTypeSlime
	case strings.HasPrefix(name, "*tele"):
		return model.TexTypeTele
	case strings.HasPrefix(name, "*"):
		return model.TexTypeWater
	default:
		return model.TexTypeDefault
	}
}

// DeriveFaceFlags converts texture type and texinfo flags into surface flags.
// This mirrors the C Ironwail Mod_LoadFaces logic (gl_model.c:1359-1368):
//   - All liquid textures get SURF_DRAWTURB.
//   - SURF_DRAWTILED (unlit, no lightmap) is set ONLY when TEX_SPECIAL is
//     present on the texinfo. Liquid faces without TEX_SPECIAL can have
//     valid lightmap data (lit water maps), so they must NOT be marked
//     tiled — otherwise lit water would be disabled for all liquids.
//   - Sky always gets SURF_DRAWTILED regardless of TEX_SPECIAL.
func DeriveFaceFlags(textureType model.TextureType, texinfoFlags int32) int32 {
	flags := int32(0)
	if texinfoFlags&bsp.TexMissing != 0 {
		flags |= model.SurfNoTexture
	}
	if texinfoFlags&bsp.TexSpecial != 0 {
		flags |= model.SurfDrawTiled
	}

	switch textureType {
	case model.TexTypeCutout:
		flags |= model.SurfDrawFence
	case model.TexTypeSky:
		flags |= model.SurfDrawSky | model.SurfDrawTiled
	case model.TexTypeLava:
		flags |= model.SurfDrawTurb | model.SurfDrawLava
	case model.TexTypeSlime:
		flags |= model.SurfDrawTurb | model.SurfDrawSlime
	case model.TexTypeTele:
		flags |= model.SurfDrawTurb | model.SurfDrawTele
	case model.TexTypeWater:
		flags |= model.SurfDrawTurb | model.SurfDrawWater
	}

	return flags
}

func BuildMaterialTextureRGBA(pixels, palette []byte, textureType model.TextureType) MaterialTextureRGBA {
	diffuse := make([]byte, len(pixels)*4)
	fullbright := make([]byte, len(pixels)*4)
	cutout := textureType == model.TexTypeCutout
	// Liquid (*lava/*slime/*tele/*water) and sky textures do not participate
	// in the "alpha as lighting mask" trick used by regular lit world
	// materials. Their pixels are always fully opaque, matching C's
	// TEXTYPE_ISLIQUID / sky upload paths in gl_model.c.
	liquidOrSky := textureType == model.TexTypeLava ||
		textureType == model.TexTypeSlime ||
		textureType == model.TexTypeTele ||
		textureType == model.TexTypeWater ||
		textureType == model.TexTypeSky
	hasSeparateFullbright := false

	for i, idx := range pixels {
		base := i * 4
		if cutout && idx == 255 {
			continue
		}
		r, g, b := paletteColor(idx, palette)
		if idx >= 224 && idx <= 255 {
			switch {
			case cutout:
				fullbright[base+0] = r
				fullbright[base+1] = g
				fullbright[base+2] = b
				fullbright[base+3] = 255
				diffuse[base+3] = 255
				hasSeparateFullbright = true
			case liquidOrSky:
				diffuse[base+0] = r
				diffuse[base+1] = g
				diffuse[base+2] = b
				diffuse[base+3] = 255
			default:
				diffuse[base+0] = r
				diffuse[base+1] = g
				diffuse[base+2] = b
				// Regular world materials use alpha as a lighting mask for embedded
				// fullbright texels; they are not true transparent pixels. Mirror
				// C Ironwail's is_fullbright set from gl_texmgr.c: every palette
				// index whose colormap row is constant is unlit, which for the
				// standard palette is 224..255 (index 255 is a brownish skin
				// color that C treats as fullbright — do not light it).
				diffuse[base+3] = 0
			}
			continue
		}

		diffuse[base+0] = r
		diffuse[base+1] = g
		diffuse[base+2] = b
		diffuse[base+3] = 255
	}

	if !hasSeparateFullbright {
		fullbright = nil
	}
	return MaterialTextureRGBA{
		DiffuseRGBA:    diffuse,
		FullbrightRGBA: fullbright,
		HasFullbright:  hasSeparateFullbright,
	}
}

// TexCoordDouble projects a world position onto one texinfo axis vector
// (position·vec[0..2] + vec[3]) in float64 precision, mirroring C Ironwail's
// double-precision texture coordinate math.
func TexCoordDouble(position [3]float32, vec [4]float32) float64 {
	return float64(position[0])*float64(vec[0]) +
		float64(position[1])*float64(vec[1]) +
		float64(position[2])*float64(vec[2]) +
		float64(vec[3])
}

// FaceTexInfo resolves the texture-info record for a BSP face, which maps
// geometric vertices into texture/lightmap UV space.
func FaceTexInfo(tree *bsp.Tree, face *bsp.TreeFace) *bsp.Texinfo {
	if tree == nil || face == nil {
		return nil
	}
	if int(face.Texinfo) < 0 || int(face.Texinfo) >= len(tree.Texinfo) {
		return nil
	}
	return &tree.Texinfo[face.Texinfo]
}

// TextureCount reads the miptex lump's texture count (0, false if malformed).
func TextureCount(tree *bsp.Tree) (int32, bool) {
	if tree == nil || len(tree.TextureData) < 4 {
		return 0, false
	}
	count := int32(binary.LittleEndian.Uint32(tree.TextureData[:4]))
	if count < 0 || len(tree.TextureData) < 4+int(count)*4 {
		return 0, false
	}
	return count, true
}

// TextureEntryLoaded reports whether the miptex entry at textureIndex has a
// valid parsed miptex with positive dimensions.
func TextureEntryLoaded(tree *bsp.Tree, textureIndex int) bool {
	if tree == nil || textureIndex < 0 || len(tree.TextureData) < 4 {
		return false
	}
	textureCount := int(int32(binary.LittleEndian.Uint32(tree.TextureData[:4])))
	if textureIndex >= textureCount || len(tree.TextureData) < 4+textureCount*4 {
		return false
	}
	offsetPos := 4 + textureIndex*4
	offset := int(int32(binary.LittleEndian.Uint32(tree.TextureData[offsetPos : offsetPos+4])))
	if offset <= 0 || offset >= len(tree.TextureData) {
		return false
	}
	miptex, err := image.ParseMipTex(tree.TextureData[offset:])
	return err == nil && miptex.Width > 0 && miptex.Height > 0
}

// MissingTextureFallbackIndex returns the fallback atlas slot for a texinfo
// whose texture is missing (the two dummy slots appended by C Ironwail), plus
// whether a fallback is needed.
func MissingTextureFallbackIndex(tree *bsp.Tree, texInfo *bsp.Texinfo) (int32, bool) {
	textureCount, ok := TextureCount(tree)
	if !ok {
		return 0, false
	}
	// C Ironwail appends two dummy texture slots and remaps missing texinfos to
	// them so faces with offset -1 still draw instead of sampling no material.
	missing := texInfo.Flags&bsp.TexMissing != 0
	miptexIndex := int(texInfo.Miptex)
	if miptexIndex < 0 || miptexIndex >= int(textureCount) || !TextureEntryLoaded(tree, miptexIndex) {
		missing = true
	}
	if !missing {
		return 0, false
	}
	if texInfo.Flags&bsp.TexSpecial != 0 {
		return textureCount + 1, true
	}
	return textureCount, true
}

// TextureDimensions fetches source texture dimensions for texel-density and
// UV conversion computations (1x1 defaults when unavailable).
func TextureDimensions(tree *bsp.Tree, texInfo *bsp.Texinfo) (float32, float32) {
	textureWidth := float32(1)
	textureHeight := float32(1)
	if tree == nil || texInfo == nil || texInfo.Miptex < 0 || len(tree.TextureData) < 4 {
		return textureWidth, textureHeight
	}

	textureCount := int(int32(binary.LittleEndian.Uint32(tree.TextureData[:4])))
	miptexIndex := int(texInfo.Miptex)
	if miptexIndex < 0 || miptexIndex >= textureCount {
		return textureWidth, textureHeight
	}
	offsetTableEnd := 4 + textureCount*4
	if len(tree.TextureData) < offsetTableEnd {
		return textureWidth, textureHeight
	}

	offsetPos := 4 + miptexIndex*4
	offset := int(int32(binary.LittleEndian.Uint32(tree.TextureData[offsetPos : offsetPos+4])))
	if offset <= 0 || offset >= len(tree.TextureData) {
		return textureWidth, textureHeight
	}

	miptex, err := image.ParseMipTex(tree.TextureData[offset:])
	if err != nil {
		return textureWidth, textureHeight
	}

	if miptex.Width > 0 {
		textureWidth = float32(miptex.Width)
	}
	if miptex.Height > 0 {
		textureHeight = float32(miptex.Height)
	}
	return textureWidth, textureHeight
}

// FaceTextureIndex resolves the diffuse texture atlas slot for a face so
// world pass shaders can sample the correct base map.
func FaceTextureIndex(tree *bsp.Tree, face *bsp.TreeFace) int32 {
	texInfo := FaceTexInfo(tree, face)
	if texInfo == nil {
		return -1
	}
	if remapped, ok := MissingTextureFallbackIndex(tree, texInfo); ok {
		return remapped
	}
	if texInfo.Miptex < 0 {
		return -1
	}
	return texInfo.Miptex
}

// FaceFlags exposes per-face material/render flags (sky, liquid, turbulent,
// etc.) that drive pass routing and shader behavior. textureMeta may be nil
// or shorter than the texture table; missing entries fall back to the
// classification of the name "" (unknown).
func FaceFlags(textureMeta []TextureMeta, tree *bsp.Tree, face *bsp.TreeFace) int32 {
	texInfo := FaceTexInfo(tree, face)
	if texInfo == nil {
		return 0
	}
	textureType := ClassifyTextureName("")
	texinfoFlags := texInfo.Flags
	if _, missing := MissingTextureFallbackIndex(tree, texInfo); missing {
		texinfoFlags |= bsp.TexMissing
	} else if int(texInfo.Miptex) >= 0 && int(texInfo.Miptex) < len(textureMeta) {
		textureType = textureMeta[texInfo.Miptex].Type
	}
	return DeriveFaceFlags(textureType, texinfoFlags)
}
