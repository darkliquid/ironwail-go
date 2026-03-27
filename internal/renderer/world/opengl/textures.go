//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package opengl

import (
	"encoding/binary"
	"log/slog"
	"runtime"
	"strings"
	"unsafe"

	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/image"
	"github.com/ironwail/ironwail-go/internal/model"
	worldimpl "github.com/ironwail/ironwail-go/internal/renderer/world"
)

type TextureUploadOptions struct {
	MinFilter  int32
	MagFilter  int32
	LodBias    float32
	Anisotropy float32
}

type PaletteToRGBAFunc func(pixels []byte, palette []byte) []byte

type PaletteToFullbrightRGBAFunc func(pixels []byte, palette []byte) ([]byte, bool)

type TextureUploaderFunc func(width, height int, rgba []byte) uint32

type WorldTextureUploadPlan struct {
	Index          int32
	Name           string
	Width          int
	Height         int
	DiffuseRGBA    []byte
	FullbrightRGBA []byte
	HasFullbright  bool
	SkySolidRGBA   []byte
	SkyAlphaRGBA   []byte
	SkyFlatRGBA    [4]byte
	SkyLayerWidth  int
	SkyLayerHeight int
	HasSkyLayers   bool
}

type WorldTextureUploadPlanSet struct {
	TextureNames []string
	Plans        []WorldTextureUploadPlan
}

type UploadedWorldTextures struct {
	Diffuse    map[int32]uint32
	Fullbright map[int32]uint32
	SkySolid   map[int32]uint32
	SkyAlpha   map[int32]uint32
	SkyFlat    map[int32]uint32
}

func ParseTextureMode(mode string) (minFilter, magFilter int32) {
	switch strings.ToUpper(strings.TrimSpace(mode)) {
	case "GL_NEAREST":
		return gl.NEAREST, gl.NEAREST
	case "GL_LINEAR":
		return gl.LINEAR, gl.LINEAR
	case "GL_NEAREST_MIPMAP_NEAREST":
		return gl.NEAREST_MIPMAP_NEAREST, gl.NEAREST
	case "GL_NEAREST_MIPMAP_LINEAR":
		return gl.NEAREST_MIPMAP_LINEAR, gl.NEAREST
	case "GL_LINEAR_MIPMAP_NEAREST":
		return gl.LINEAR_MIPMAP_NEAREST, gl.LINEAR
	case "GL_LINEAR_MIPMAP_LINEAR":
		return gl.LINEAR_MIPMAP_LINEAR, gl.LINEAR
	default:
		return gl.NEAREST, gl.NEAREST
	}
}

func withPinnedPixelData(pixels []byte, fn func(unsafe.Pointer)) {
	if fn == nil {
		return
	}
	if len(pixels) == 0 {
		fn(nil)
		return
	}
	var pinner runtime.Pinner
	pinner.Pin(&pixels[0])
	defer pinner.Unpin()
	fn(unsafe.Pointer(&pixels[0]))
}

func UploadTextureRGBA(width, height int, rgba []byte, options TextureUploadOptions) uint32 {
	var tex uint32
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
	withPinnedPixelData(rgba, func(ptr unsafe.Pointer) {
		gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(width), int32(height), 0, gl.RGBA, gl.UNSIGNED_BYTE, ptr)
	})
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, options.MinFilter)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, options.MagFilter)
	gl.GenerateMipmap(gl.TEXTURE_2D)
	gl.TexParameterf(gl.TEXTURE_2D, gl.TEXTURE_LOD_BIAS, options.LodBias)
	anisotropy := options.Anisotropy
	if anisotropy < 1 {
		anisotropy = 1
	}
	gl.TexParameterf(gl.TEXTURE_2D, gl.TEXTURE_MAX_ANISOTROPY, anisotropy)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.REPEAT)
	return tex
}

func BuildTextureUploadPlan(tree *bsp.Tree, palette []byte, toRGBA PaletteToRGBAFunc, toFullbrightRGBA PaletteToFullbrightRGBAFunc) WorldTextureUploadPlanSet {
	if tree == nil || len(tree.TextureData) < 4 {
		return WorldTextureUploadPlanSet{}
	}

	count := int(binary.LittleEndian.Uint32(tree.TextureData[:4]))
	if count <= 0 || len(tree.TextureData) < 4+count*4 {
		return WorldTextureUploadPlanSet{}
	}

	result := WorldTextureUploadPlanSet{
		TextureNames: make([]string, count),
		Plans:        make([]WorldTextureUploadPlan, 0, count),
	}
	for i := 0; i < count; i++ {
		offset := int(int32(binary.LittleEndian.Uint32(tree.TextureData[4+i*4:])))
		if offset <= 0 || offset >= len(tree.TextureData) {
			continue
		}
		miptex, err := image.ParseMipTex(tree.TextureData[offset:])
		if err != nil {
			slog.Debug("OpenGL world texture parse failed", "index", i, "error", err)
			continue
		}
		pixels, width, height, err := miptex.MipLevel(0)
		if err != nil || width <= 0 || height <= 0 {
			continue
		}
		result.TextureNames[i] = miptex.Name
		plan := WorldTextureUploadPlan{
			Index:  int32(i),
			Name:   miptex.Name,
			Width:  width,
			Height: height,
		}
		if toRGBA != nil {
			plan.DiffuseRGBA = toRGBA(pixels, palette)
		}
		if toFullbrightRGBA != nil {
			plan.FullbrightRGBA, plan.HasFullbright = toFullbrightRGBA(pixels, palette)
		}
		if worldimpl.ClassifyTextureName(miptex.Name) == model.TexTypeSky {
			solidRGBA, alphaRGBA, layerWidth, layerHeight, ok := worldimpl.ExtractEmbeddedSkyLayers(
				pixels,
				width,
				height,
				palette,
				worldimpl.ShouldSplitAsQuake64Sky(tree.Version, width, height),
			)
			if ok {
				plan.SkySolidRGBA = solidRGBA
				plan.SkyAlphaRGBA = alphaRGBA
				plan.SkyFlatRGBA = worldimpl.BuildSkyFlatRGBA(alphaRGBA)
				plan.SkyLayerWidth = layerWidth
				plan.SkyLayerHeight = layerHeight
				plan.HasSkyLayers = true
			}
		}
		result.Plans = append(result.Plans, plan)
	}
	return result
}

func ApplyTextureUploadPlan(plan WorldTextureUploadPlanSet, upload TextureUploaderFunc) UploadedWorldTextures {
	uploaded := UploadedWorldTextures{
		Diffuse:    make(map[int32]uint32),
		Fullbright: make(map[int32]uint32),
		SkySolid:   make(map[int32]uint32),
		SkyAlpha:   make(map[int32]uint32),
		SkyFlat:    make(map[int32]uint32),
	}
	if upload == nil {
		return uploaded
	}
	for _, texturePlan := range plan.Plans {
		tex := upload(texturePlan.Width, texturePlan.Height, texturePlan.DiffuseRGBA)
		if tex != 0 {
			uploaded.Diffuse[texturePlan.Index] = tex
		}
		if texturePlan.HasFullbright {
			fbTex := upload(texturePlan.Width, texturePlan.Height, texturePlan.FullbrightRGBA)
			if fbTex != 0 {
				uploaded.Fullbright[texturePlan.Index] = fbTex
			}
		}
		if !texturePlan.HasSkyLayers {
			continue
		}
		solidTex := upload(texturePlan.SkyLayerWidth, texturePlan.SkyLayerHeight, texturePlan.SkySolidRGBA)
		alphaTex := upload(texturePlan.SkyLayerWidth, texturePlan.SkyLayerHeight, texturePlan.SkyAlphaRGBA)
		if solidTex != 0 {
			uploaded.SkySolid[texturePlan.Index] = solidTex
		}
		if alphaTex != 0 {
			uploaded.SkyAlpha[texturePlan.Index] = alphaTex
		}
		flatTex := upload(1, 1, texturePlan.SkyFlatRGBA[:])
		if flatTex != 0 {
			uploaded.SkyFlat[texturePlan.Index] = flatTex
		}
	}
	return uploaded
}
