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

// animateWorldMaterials returns a copy of baseMaterials with active texture
// animations applied for the given entity frame. Frame 0 selects primary
// animation chains (+0button, +1button, ...); frame != 0 selects alternate
// chains (+Abutton, +Bbutton, ...) used by pressed buttons and activated
// switches. Most textures have no AlternateAnims, so frame has no effect.
func animateWorldMaterials(baseMaterials []WorldMaterialData, animations []*surfacepkg.SurfaceTexture, frame int, timeValue float32) []WorldMaterialData {
	if len(baseMaterials) == 0 {
		return nil
	}
	animatedMaterials := make([]WorldMaterialData, len(baseMaterials))
	copy(animatedMaterials, baseMaterials)

	for i, anim := range animations {
		if anim == nil {
			continue
		}

		frameTexture, err := surfacepkg.TextureAnimation(anim, frame, float64(timeValue))
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
		diagAnimationRemap(i, targetIdx, baseMaterials[targetIdx], len(baseMaterials))
		animatedMaterials[i] = baseMaterials[targetIdx]
	}
	return animatedMaterials
}

// updateWorldMaterialsBuffer updates the materials uniform buffer with animated
// texture layers. The caller must already hold r.mu (at least RLock) since this
// reads r.worldBaseMaterials and r.worldTextureAnimations without locking.
// It also updates the frame-1 variant (r.resources.WorldMaterialsBufferFrame1) so that
// brush entities with frame != 0 (pressed buttons, activated switches) can
// select alternate texture chains without overwriting the frame-0 data that
// world faces still need.
func (r *Renderer) updateWorldMaterialsBuffer(queue *wgpu.Queue, timeValue float32) error {
	if r.resources.WorldMaterialsBuffer == nil || len(r.worldBaseMaterials) == 0 {
		return nil
	}

	animatedMaterials := animateWorldMaterials(r.worldBaseMaterials, r.worldTextureAnimations, 0, timeValue)

	// Phase 1 diagnostic: check for buffer overflow before per-frame write.
	diagMaterialBufferWrite("updateWorldMaterialsBuffer", len(animatedMaterials), int(r.resources.WorldMaterialsBuffer.Size()))

	byteLen := len(animatedMaterials) * int(unsafe.Sizeof(WorldMaterialData{}))
	byteData := unsafe.Slice((*byte)(unsafe.Pointer(&animatedMaterials[0])), byteLen)

	if err := queue.WriteBuffer(r.resources.WorldMaterialsBuffer, 0, byteData); err != nil {
		return err
	}

	// Update the frame-1 materials buffer so brush entities with frame != 0
	// can bind alternate texture chains. This buffer is the same size and
	// layout as the frame-0 buffer but with AlternateAnims selected.
	if r.resources.WorldMaterialsBufferFrame1 != nil {
		frame1Materials := animateWorldMaterials(r.worldBaseMaterials, r.worldTextureAnimations, 1, timeValue)
		f1ByteLen := len(frame1Materials) * int(unsafe.Sizeof(WorldMaterialData{}))
		f1ByteData := unsafe.Slice((*byte)(unsafe.Pointer(&frame1Materials[0])), f1ByteLen)
		if err := queue.WriteBuffer(r.resources.WorldMaterialsBufferFrame1, 0, f1ByteData); err != nil {
			return err
		}
	}

	// Update frame-1 materials for external BSP models so their brush
	// entities can also use alternate texture chains when frame != 0.
	for key, baseMaterials := range r.externalBrushBaseMaterials {
		frame1Buf, ok := r.externalBrushMaterialsBuffersFrame1[key]
		if !ok || frame1Buf == nil || len(baseMaterials) == 0 {
			continue
		}
		animations := r.externalBrushAnimations[key]
		frame1Materials := animateWorldMaterials(baseMaterials, animations, 1, timeValue)
		extByteLen := len(frame1Materials) * int(unsafe.Sizeof(WorldMaterialData{}))
		extByteData := unsafe.Slice((*byte)(unsafe.Pointer(&frame1Materials[0])), extByteLen)
		if err := queue.WriteBuffer(frame1Buf, 0, extByteData); err != nil {
			return err
		}
	}

	return nil
}
