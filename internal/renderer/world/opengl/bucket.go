//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package opengl

import (
	"sort"

	"github.com/darkliquid/ironwail-go/internal/model"
	surfaceimpl "github.com/darkliquid/ironwail-go/internal/renderer/surface"
	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
)

type DrawCall struct {
	Face              worldimpl.WorldFace
	Texture           uint32
	FullbrightTexture uint32
	Lightmap          uint32
	Alpha             float32
	Turbulent         bool
	HasLitWater       bool
	DistanceSq        float32
	Light             [3]float32
	VAO               uint32
	ModelOffset       [3]float32
	ModelRotation     [16]float32
	ModelScale        float32
}

type TextureAnimationFunc func(base *surfaceimpl.SurfaceTexture, frame int, timeSeconds float64) (*surfaceimpl.SurfaceTexture, error)
type LightEvaluator func(point [3]float32) [3]float32

func BucketFacesWithLights(
	faces []worldimpl.WorldFace,
	hasLitWater bool,
	textures map[int32]uint32,
	fullbrightTextures map[int32]uint32,
	textureAnimations []*surfaceimpl.SurfaceTexture,
	lightmaps []uint32,
	fallbackTexture, fallbackLightmap, vao uint32,
	modelOffset [3]float32,
	modelRotation [16]float32,
	modelScale, entityAlpha float32,
	entityFrame int,
	timeSeconds float64,
	cameraOrigin [3]float32,
	liquidAlpha worldimpl.LiquidAlphaSettings,
	animateTexture TextureAnimationFunc,
	evaluateLights LightEvaluator,
) (sky, opaque, alphaTest, liquidOpaque, liquidTranslucent, translucent []DrawCall) {
	for _, face := range faces {
		center := worldimpl.TransformModelSpacePoint(face.Center, modelOffset, modelRotation, modelScale)
		call := DrawCall{
			Face:              face,
			Texture:           textureForFace(face, textures, textureAnimations, fallbackTexture, entityFrame, timeSeconds, animateTexture),
			FullbrightTexture: textureForFace(face, fullbrightTextures, textureAnimations, 0, entityFrame, timeSeconds, animateTexture),
			Lightmap:          lightmapForFace(face, lightmaps, fallbackLightmap),
			Alpha:             worldimpl.FaceAlpha(face.Flags, liquidAlpha) * entityAlpha,
			Turbulent:         worldimpl.FaceUsesTurb(face.Flags),
			HasLitWater:       hasLitWater,
			DistanceSq:        worldimpl.FaceDistanceSq(center, cameraOrigin),
			VAO:               vao,
			ModelOffset:       modelOffset,
			ModelRotation:     modelRotation,
			ModelScale:        modelScale,
		}
		if evaluateLights != nil {
			call.Light = evaluateLights(center)
		}

		switch worldimpl.FacePass(face.Flags, call.Alpha) {
		case worldimpl.PassSky:
			sky = append(sky, call)
		case worldimpl.PassAlphaTest:
			alphaTest = append(alphaTest, call)
		case worldimpl.PassTranslucent:
			if worldimpl.FaceIsLiquid(face.Flags) {
				liquidTranslucent = append(liquidTranslucent, call)
				continue
			}
			translucent = append(translucent, call)
		default:
			if worldimpl.FaceIsLiquid(face.Flags) {
				liquidOpaque = append(liquidOpaque, call)
				continue
			}
			opaque = append(opaque, call)
		}
	}

	sort.SliceStable(liquidTranslucent, func(i, j int) bool {
		return liquidTranslucent[i].DistanceSq > liquidTranslucent[j].DistanceSq
	})
	sort.SliceStable(translucent, func(i, j int) bool {
		return translucent[i].DistanceSq > translucent[j].DistanceSq
	})

	return sky, opaque, alphaTest, liquidOpaque, liquidTranslucent, translucent
}

func BucketFaces(
	faces []worldimpl.WorldFace,
	textures map[int32]uint32,
	fullbrightTextures map[int32]uint32,
	textureAnimations []*surfaceimpl.SurfaceTexture,
	lightmaps []uint32,
	fallbackTexture, fallbackLightmap uint32,
	modelOffset [3]float32,
	cameraOrigin [3]float32,
	cameraTime float64,
	liquidAlpha worldimpl.LiquidAlphaSettings,
	animateTexture TextureAnimationFunc,
) (sky, opaque, alphaTest, liquidOpaque, liquidTranslucent, translucent []DrawCall) {
	return BucketFacesWithLights(
		faces,
		facesHaveLitWater(faces),
		textures,
		fullbrightTextures,
		textureAnimations,
		lightmaps,
		fallbackTexture,
		fallbackLightmap,
		0,
		modelOffset,
		worldimpl.IdentityModelRotationMatrix,
		1,
		1,
		0,
		cameraTime,
		cameraOrigin,
		liquidAlpha,
		animateTexture,
		nil,
	)
}

func textureForFace(face worldimpl.WorldFace, textures map[int32]uint32, textureAnimations []*surfaceimpl.SurfaceTexture, fallbackTexture uint32, frame int, timeSeconds float64, animateTexture TextureAnimationFunc) uint32 {
	textureIndex := face.TextureIndex
	if animateTexture != nil && textureIndex >= 0 && int(textureIndex) < len(textureAnimations) && textureAnimations[textureIndex] != nil {
		if animated, err := animateTexture(textureAnimations[textureIndex], frame, timeSeconds); err == nil && animated != nil {
			textureIndex = animated.TextureIndex
		}
	}

	tex := textures[textureIndex]
	if tex == 0 && textureIndex != face.TextureIndex {
		tex = textures[face.TextureIndex]
	}
	if tex == 0 {
		tex = fallbackTexture
	}
	return tex
}

func lightmapForFace(face worldimpl.WorldFace, lightmaps []uint32, fallbackLightmap uint32) uint32 {
	if face.LightmapIndex >= 0 && int(face.LightmapIndex) < len(lightmaps) && lightmaps[face.LightmapIndex] != 0 {
		return lightmaps[face.LightmapIndex]
	}
	return fallbackLightmap
}

func facesHaveLitWater(faces []worldimpl.WorldFace) bool {
	for _, face := range faces {
		if face.Flags&model.SurfDrawTurb != 0 && face.LightmapIndex >= 0 {
			return true
		}
	}
	return false
}
