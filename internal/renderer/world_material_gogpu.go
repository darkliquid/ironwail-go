package renderer

import (
	surfacepkg "github.com/darkliquid/ironwail-go/internal/renderer/surface"
	"github.com/gogpu/wgpu"
	"unsafe"
)

// WorldMaterialData matches the MaterialData struct in WGSL.
type WorldMaterialData struct {
	AtlasBounds [4]float32
	Layer       float32
	Pad         [3]float32
}

// worldMaterialsBufferSize is the maximum size for the materials uniform buffer.
const worldMaterialsBufferSize = 256 * int(unsafe.Sizeof(WorldMaterialData{}))

// animateWorldMaterials returns a copy of baseMaterials with active texture animations applied.
func animateWorldMaterials(baseMaterials []WorldMaterialData, animations []*surfacepkg.SurfaceTexture, timeValue float32) []WorldMaterialData {
	if len(baseMaterials) == 0 {
		return nil
	}
	animatedMaterials := make([]WorldMaterialData, len(baseMaterials))
	copy(animatedMaterials, baseMaterials)

	for i, anim := range animations {
		if anim == nil {
			continue
		}

		// World textures use frame 0 unless a specific entity overrides it.
		frameTexture, err := surfacepkg.TextureAnimation(anim, 0, float64(timeValue))
		if err != nil || frameTexture == nil {
			continue
		}

		// An animated texture swaps its entire material configuration (bounds + layer)
		// to point at the animated frame's location in the atlas.
		targetIdx := int(frameTexture.TextureIndex)
		if targetIdx < 0 || targetIdx >= len(baseMaterials) {
			continue
		}
		// Phase 4 diagnostic: log animation remap when audit is enabled.
		diagAnimationRemap(i, targetIdx, baseMaterials[targetIdx])
		animatedMaterials[i] = baseMaterials[targetIdx]
	}
	return animatedMaterials
}

// updateWorldMaterialsBuffer updates the materials uniform buffer with animated
// texture layers. The caller must already hold r.mu (at least RLock) since this
// reads r.worldBaseMaterials and r.worldTextureAnimations without locking.
func (r *Renderer) updateWorldMaterialsBuffer(queue *wgpu.Queue, timeValue float32) error {
	if r.worldMaterialsBuffer == nil || len(r.worldBaseMaterials) == 0 {
		return nil
	}

	animatedMaterials := animateWorldMaterials(r.worldBaseMaterials, r.worldTextureAnimations, timeValue)

	// Phase 1 diagnostic: check for buffer overflow before per-frame write.
	diagMaterialBufferWrite("updateWorldMaterialsBuffer", len(animatedMaterials), worldMaterialsBufferSize)

	byteLen := len(animatedMaterials) * int(unsafe.Sizeof(WorldMaterialData{}))
	byteData := unsafe.Slice((*byte)(unsafe.Pointer(&animatedMaterials[0])), byteLen)

	return queue.WriteBuffer(r.worldMaterialsBuffer, 0, byteData)
}

