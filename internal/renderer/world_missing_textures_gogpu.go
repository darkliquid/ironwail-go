package renderer

import (
	"log/slog"

	"github.com/darkliquid/ironwail-go/internal/model"
	"github.com/gogpu/wgpu"
)

func (r *Renderer) uploadWorldMissingTextureDummies(device *wgpu.Device, queue *wgpu.Queue, sampler *wgpu.Sampler, textures map[int32]*gpuWorldTexture, textureCount int32) {
	if textures == nil || textureCount < 0 {
		return
	}
	for _, textureIndex := range []int32{textureCount, textureCount + 1} {
		if textures[textureIndex] != nil {
			continue
		}
		rgba := []byte{255, 255, 255, 255}
		worldTexture, err := r.createWorldDiffuseTexture(device, queue, sampler, model.TexTypeDefault, rgba, 1, 1)
		if err != nil {
			slog.Warn("failed to upload world missing-texture dummy", "texture_index", textureIndex, "error", err)
			continue
		}
		textures[textureIndex] = worldTexture
	}
}
