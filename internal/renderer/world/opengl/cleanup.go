//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package opengl

import "github.com/go-gl/gl/v4.6-core/gl"

func DeleteVertexArrays(handles ...*uint32) {
	for _, handle := range handles {
		if handle == nil || *handle == 0 {
			continue
		}
		gl.DeleteVertexArrays(1, handle)
		*handle = 0
	}
}

func DeleteBuffers(handles ...*uint32) {
	for _, handle := range handles {
		if handle == nil || *handle == 0 {
			continue
		}
		gl.DeleteBuffers(1, handle)
		*handle = 0
	}
}

func DeletePrograms(handles ...*uint32) {
	for _, handle := range handles {
		if handle == nil || *handle == 0 {
			continue
		}
		gl.DeleteProgram(*handle)
		*handle = 0
	}
}

func DeleteTextures(handles ...*uint32) {
	for _, handle := range handles {
		if handle == nil || *handle == 0 {
			continue
		}
		gl.DeleteTextures(1, handle)
		*handle = 0
	}
}

func DeleteTextureMap[K comparable](textures map[K]uint32) {
	for key, tex := range textures {
		if tex != 0 {
			gl.DeleteTextures(1, &tex)
		}
		delete(textures, key)
	}
}

func DeleteTextureSlice(textures []uint32) {
	for i, tex := range textures {
		if tex != 0 {
			gl.DeleteTextures(1, &tex)
			textures[i] = 0
		}
	}
}

func DeleteTextureSliceExcept(textures []uint32, keep uint32) {
	for i, tex := range textures {
		if tex != 0 && tex != keep {
			gl.DeleteTextures(1, &tex)
		}
		textures[i] = 0
	}
}

func DeleteTextureGroupsExcept[K comparable](groups map[K][]uint32, keep uint32) {
	for key, textures := range groups {
		DeleteTextureSliceExcept(textures, keep)
		delete(groups, key)
	}
}

func SetInt32Fields(value int32, fields ...*int32) {
	for _, field := range fields {
		if field == nil {
			continue
		}
		*field = value
	}
}
