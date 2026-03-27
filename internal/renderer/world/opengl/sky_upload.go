//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package opengl

import (
	"github.com/go-gl/gl/v4.6-core/gl"
	skyimpl "github.com/ironwail/ironwail-go/internal/renderer/sky"
	"unsafe"
)

var skyboxCubemapTargets = [...]uint32{
	gl.TEXTURE_CUBE_MAP_POSITIVE_X,
	gl.TEXTURE_CUBE_MAP_NEGATIVE_X,
	gl.TEXTURE_CUBE_MAP_POSITIVE_Y,
	gl.TEXTURE_CUBE_MAP_NEGATIVE_Y,
	gl.TEXTURE_CUBE_MAP_POSITIVE_Z,
	gl.TEXTURE_CUBE_MAP_NEGATIVE_Z,
}

// UploadSkyboxCubemap uploads 6 skybox face images as a GL_TEXTURE_CUBE_MAP, reordering
// faces from Quake convention (rt/bk/lf/ft/up/dn) to OpenGL convention (+X/-X/+Y/-Y/+Z/-Z).
func UploadSkyboxCubemap(faces [6]skyimpl.ExternalSkyboxFace, faceSize int) uint32 {
	if faceSize <= 0 {
		return 0
	}
	var cubemap uint32
	gl.GenTextures(1, &cubemap)
	if cubemap == 0 {
		return 0
	}
	gl.BindTexture(gl.TEXTURE_CUBE_MAP, cubemap)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_WRAP_R, gl.CLAMP_TO_EDGE)
	zeroFace := make([]byte, faceSize*faceSize*4)
	for i, target := range skyboxCubemapTargets {
		face := faces[skyimpl.SkyboxCubemapFaceOrder[i]]
		faceData := zeroFace
		if face.Width > 0 && face.Height > 0 && len(face.RGBA) > 0 {
			if face.Width != faceSize || face.Height != faceSize || len(face.RGBA) != faceSize*faceSize*4 {
				gl.DeleteTextures(1, &cubemap)
				gl.BindTexture(gl.TEXTURE_CUBE_MAP, 0)
				return 0
			}
			faceData = face.RGBA
		}
		if len(faceData) != faceSize*faceSize*4 {
			gl.DeleteTextures(1, &cubemap)
			gl.BindTexture(gl.TEXTURE_CUBE_MAP, 0)
			return 0
		}
		withPinnedPixelData(faceData, func(ptr unsafe.Pointer) {
			gl.TexImage2D(target, 0, gl.RGBA8, int32(faceSize), int32(faceSize), 0, gl.RGBA, gl.UNSIGNED_BYTE, ptr)
		})
	}
	gl.BindTexture(gl.TEXTURE_CUBE_MAP, 0)
	return cubemap
}

// UploadSkyboxFaceTextures uploads each skybox face as an individual GL_TEXTURE_2D,
// used as fallback when faces aren't all square and can't form a cubemap.
func UploadSkyboxFaceTextures(faces [6]skyimpl.ExternalSkyboxFace) (textures [6]uint32, ok bool) {
	fallbackPixel := [4]byte{0, 0, 0, 255}
	for i := range textures {
		gl.GenTextures(1, &textures[i])
		if textures[i] == 0 {
			for j := 0; j < i; j++ {
				if textures[j] != 0 {
					gl.DeleteTextures(1, &textures[j])
					textures[j] = 0
				}
			}
			return textures, false
		}
		gl.BindTexture(gl.TEXTURE_2D, textures[i])
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

		face := faces[i]
		width := face.Width
		height := face.Height
		data := face.RGBA
		if width <= 0 || height <= 0 || len(data) != width*height*4 {
			width, height = 1, 1
			data = fallbackPixel[:]
		}
		withPinnedPixelData(data, func(ptr unsafe.Pointer) {
			gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, int32(width), int32(height), 0, gl.RGBA, gl.UNSIGNED_BYTE, ptr)
		})
	}
	gl.BindTexture(gl.TEXTURE_2D, 0)
	return textures, true
}
