package world

import (
	"encoding/binary"
	"strings"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/image"
	"github.com/ironwail/ironwail-go/internal/model"
)

// TextureMeta holds parsed texture metadata from BSP miptex entries.
type TextureMeta struct {
	Width  int
	Height int
	Name   string
	Type   model.TextureType
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
