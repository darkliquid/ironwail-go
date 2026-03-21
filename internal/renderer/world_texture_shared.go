package renderer

import (
	"encoding/binary"
	"strings"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/image"
	"github.com/ironwail/ironwail-go/internal/model"
)

// worldTextureMeta holds parsed texture metadata (name, dimensions, classified type)
// from the BSP miptex lump entries.
type worldTextureMeta struct {
	Width  int
	Height int
	Name   string
	Type   model.TextureType
}

// parseWorldTextureMeta parses the BSP miptex lump to extract texture names and dimensions.
func parseWorldTextureMeta(tree *bsp.Tree) []worldTextureMeta {
	if tree == nil || len(tree.TextureData) < 4 {
		return nil
	}

	count := int(binary.LittleEndian.Uint32(tree.TextureData[:4]))
	if count <= 0 || len(tree.TextureData) < 4+count*4 {
		return nil
	}

	textures := make([]worldTextureMeta, count)
	for i := 0; i < count; i++ {
		offset := int(int32(binary.LittleEndian.Uint32(tree.TextureData[4+i*4:])))
		if offset <= 0 || offset >= len(tree.TextureData) {
			continue
		}
		miptex, err := image.ParseMipTex(tree.TextureData[offset:])
		if err != nil {
			continue
		}
		textures[i] = worldTextureMeta{
			Width:  int(miptex.Width),
			Height: int(miptex.Height),
			Name:   miptex.Name,
			Type:   classifyWorldTextureName(miptex.Name),
		}
	}
	return textures
}

// classifyWorldTextureName classifies a texture by its name prefix convention.
func classifyWorldTextureName(name string) model.TextureType {
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

// deriveWorldFaceFlags converts texture type and texinfo flags into surface rendering flags.
func deriveWorldFaceFlags(textureType model.TextureType, texinfoFlags int32) int32 {
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
