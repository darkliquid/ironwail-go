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

// updateWorldMaterialsBuffer updates the materials uniform buffer with animated
// texture layers. The caller must already hold r.mu (at least RLock) since this
// reads r.worldBaseMaterials and r.worldTextureAnimations without locking.
func (r *Renderer) updateWorldMaterialsBuffer(queue *wgpu.Queue, timeValue float32) error {
	if r.worldMaterialsBuffer == nil || len(r.worldBaseMaterials) == 0 {
		return nil
	}

	// Swap atlas layers for animated textures so the shader samples the
	// correct animation frame. The buffer is small (8KB max), so a full
	// upload every frame is cheap.

	// Keep a snapshot of original layers so chained animations don't
	// read already-modified values.
	originalLayers := make([]float32, len(r.worldBaseMaterials))
	for i := range r.worldBaseMaterials {
		originalLayers[i] = r.worldBaseMaterials[i].Layer
	}

	for i, anim := range r.worldTextureAnimations {
		if anim == nil {
			continue
		}

		// World textures use frame 0 unless a specific entity overrides it.
		frameTexture, err := surfacepkg.TextureAnimation(anim, 0, float64(timeValue))
		if err != nil || frameTexture == nil {
			continue
		}

		// An animated texture swaps its atlas layer to point at the
		// animated frame's layer. The base material at index i gets the
		// layer from the material at frameTexture.TextureIndex.
		targetIdx := int(frameTexture.TextureIndex)
		if targetIdx < 0 || targetIdx >= len(originalLayers) {
			continue
		}
		r.worldBaseMaterials[i].Layer = originalLayers[targetIdx]
	}

	if len(r.worldBaseMaterials) == 0 {
		return nil
	}

	// WorldMaterialData is 32 bytes and tightly packed, so we can
	// reinterpret the slice as a byte slice for a single buffer upload.
	byteLen := len(r.worldBaseMaterials) * int(unsafe.Sizeof(WorldMaterialData{}))
	byteData := unsafe.Slice((*byte)(unsafe.Pointer(&r.worldBaseMaterials[0])), byteLen)

	err := queue.WriteBuffer(r.worldMaterialsBuffer, 0, byteData)

	// Restore original layers so the next frame starts from the base state.
	for i := range originalLayers {
		r.worldBaseMaterials[i].Layer = originalLayers[i]
	}

	return err
}
