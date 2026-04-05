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
		flags |= model.SurfDrawTurb | model.SurfDrawLava | model.SurfDrawTiled
	case model.TexTypeSlime:
		flags |= model.SurfDrawTurb | model.SurfDrawSlime | model.SurfDrawTiled
	case model.TexTypeTele:
		flags |= model.SurfDrawTurb | model.SurfDrawTele | model.SurfDrawTiled
	case model.TexTypeWater:
		flags |= model.SurfDrawTurb | model.SurfDrawWater | model.SurfDrawTiled
	}

	return flags
}

func BuildMaterialTextureRGBA(pixels, palette []byte, textureType model.TextureType) MaterialTextureRGBA {
	diffuse := make([]byte, len(pixels)*4)
	fullbright := make([]byte, len(pixels)*4)
	cutout := textureType == model.TexTypeCutout
	hasSeparateFullbright := false

	for i, idx := range pixels {
		base := i * 4
		if cutout && idx == 255 {
			continue
		}
		r, g, b := paletteColor(idx, palette)
		if idx >= 224 && idx <= 254 {
			if cutout {
				fullbright[base+0] = r
				fullbright[base+1] = g
				fullbright[base+2] = b
				fullbright[base+3] = 255
				diffuse[base+3] = 255
				hasSeparateFullbright = true
			} else {
				diffuse[base+0] = r
				diffuse[base+1] = g
				diffuse[base+2] = b
				// Regular world materials use alpha as a lighting mask for embedded
				// fullbright texels; they are not true transparent pixels.
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
