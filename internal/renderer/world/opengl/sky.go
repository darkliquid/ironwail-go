//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package opengl

import (
	surfaceimpl "github.com/ironwail/ironwail-go/internal/renderer/surface"
	worldimpl "github.com/ironwail/ironwail-go/internal/renderer/world"
)

func SkyTexturesForFace(face worldimpl.WorldFace, solidTextures, alphaTextures map[int32]uint32, textureAnimations []*surfaceimpl.SurfaceTexture, fallbackSolid, fallbackAlpha uint32, frame int, timeSeconds float64, animateTexture TextureAnimationFunc) (solid, alpha uint32) {
	textureIndex := skyTextureIndex(face, textureAnimations, frame, timeSeconds, animateTexture)

	solid = solidTextures[textureIndex]
	alpha = alphaTextures[textureIndex]
	if (solid == 0 || alpha == 0) && textureIndex != face.TextureIndex {
		if solid == 0 {
			solid = solidTextures[face.TextureIndex]
		}
		if alpha == 0 {
			alpha = alphaTextures[face.TextureIndex]
		}
	}
	if solid == 0 {
		solid = fallbackSolid
	}
	if alpha == 0 {
		alpha = fallbackAlpha
	}
	return solid, alpha
}

func SkyFlatTextureForFace(face worldimpl.WorldFace, flatTextures map[int32]uint32, textureAnimations []*surfaceimpl.SurfaceTexture, fallbackFlat uint32, frame int, timeSeconds float64, animateTexture TextureAnimationFunc) uint32 {
	textureIndex := skyTextureIndex(face, textureAnimations, frame, timeSeconds, animateTexture)

	flat := flatTextures[textureIndex]
	if flat == 0 && textureIndex != face.TextureIndex {
		flat = flatTextures[face.TextureIndex]
	}
	if flat == 0 {
		flat = fallbackFlat
	}
	return flat
}

func skyTextureIndex(face worldimpl.WorldFace, textureAnimations []*surfaceimpl.SurfaceTexture, frame int, timeSeconds float64, animateTexture TextureAnimationFunc) int32 {
	textureIndex := face.TextureIndex
	if animateTexture != nil && textureIndex >= 0 && int(textureIndex) < len(textureAnimations) && textureAnimations[textureIndex] != nil {
		if animated, err := animateTexture(textureAnimations[textureIndex], frame, timeSeconds); err == nil && animated != nil {
			textureIndex = animated.TextureIndex
		}
	}
	return textureIndex
}
