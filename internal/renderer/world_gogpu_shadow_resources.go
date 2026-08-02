package renderer

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/darkliquid/ironwail-go/internal/model"
	aliasimpl "github.com/darkliquid/ironwail-go/internal/renderer/alias"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// ---- merged from world_cleanup_gogpu_root.go ----
func (r *Renderer) clearAliasModelsLocked() {
	for key, cached := range r.aliasModels {
		if cached.poseBuffer != nil {
			cached.poseBuffer.Release()
		}
		if cached.vertexBuffer != nil {
			cached.vertexBuffer.Release()
		}
		if cached.instanceBindGroup != nil {
			cached.instanceBindGroup.Release()
		}
		for _, skin := range cached.skins {
			if skin.bindGroup != nil {
				skin.bindGroup.Release()
			}
			if skin.fullbrightView != nil {
				skin.fullbrightView.Release()
			}
			if skin.fullbrightTexture != nil {
				skin.fullbrightTexture.Release()
			}
			if skin.view != nil {
				skin.view.Release()
			}
			if skin.texture != nil {
				skin.texture.Release()
			}
		}
		for _, variants := range cached.playerSkins {
			for _, skin := range variants {
				if skin.bindGroup != nil {
					skin.bindGroup.Release()
				}
				if skin.fullbrightView != nil {
					skin.fullbrightView.Release()
				}
				if skin.fullbrightTexture != nil {
					skin.fullbrightTexture.Release()
				}
				if skin.view != nil {
					skin.view.Release()
				}
				if skin.texture != nil {
					skin.texture.Release()
				}
			}
		}
		delete(r.aliasModels, key)
	}
}

func (r *Renderer) destroyAliasResourcesLocked() {
	r.clearAliasModelsLocked()
	if r.aliasScratchBuffer != nil {
		r.aliasScratchBuffer.Release()
		r.aliasScratchBuffer = nil
	}
	if r.brushEntityScratchVertexBuffer != nil {
		r.brushEntityScratchVertexBuffer.Release()
		r.brushEntityScratchVertexBuffer = nil
	}
	if r.brushEntityScratchIndexBuffer != nil {
		r.brushEntityScratchIndexBuffer.Release()
		r.brushEntityScratchIndexBuffer = nil
	}
	if r.aliasUniformBuffer != nil {
		r.aliasUniformBuffer.Release()
		r.aliasUniformBuffer = nil
	}
	if r.aliasInstanceUniformBuffer != nil {
		r.aliasInstanceUniformBuffer.Release()
		r.aliasInstanceUniformBuffer = nil
	}
	if r.aliasInstanceUniformBindGroup != nil {
		r.aliasInstanceUniformBindGroup.Release()
		r.aliasInstanceUniformBindGroup = nil
	}
	if r.aliasUniformBindGroup != nil {
		r.aliasUniformBindGroup.Release()
		r.aliasUniformBindGroup = nil
	}
	if r.aliasInstanceBindGroupLayout != nil {
		r.aliasInstanceBindGroupLayout.Release()
		r.aliasInstanceBindGroupLayout = nil
	}
	if r.aliasSampler != nil {
		r.aliasSampler.Release()
		r.aliasSampler = nil
	}
	if r.aliasPipeline != nil {
		r.aliasPipeline.Release()
		r.aliasPipeline = nil
	}
	if r.aliasPipelineLayout != nil {
		r.aliasPipelineLayout.Release()
		r.aliasPipelineLayout = nil
	}
	if r.aliasVertexShader != nil {
		r.aliasVertexShader.Release()
		r.aliasVertexShader = nil
	}
	if r.aliasFragmentShader != nil {
		r.aliasFragmentShader.Release()
		r.aliasFragmentShader = nil
	}
	if r.aliasUniformBindGroupLayout != nil {
		r.aliasUniformBindGroupLayout.Release()
		r.aliasUniformBindGroupLayout = nil
	}
	if r.aliasTextureBindGroupLayout != nil {
		r.aliasTextureBindGroupLayout.Release()
		r.aliasTextureBindGroupLayout = nil
	}
}

func (r *Renderer) clearSpriteModelsLocked() {
	for key, cached := range r.spriteModels {
		for _, frame := range cached.frames {
			if frame.bindGroup != nil {
				frame.bindGroup.Release()
			}
			if frame.view != nil {
				frame.view.Release()
			}
			if frame.texture != nil {
				frame.texture.Release()
			}
		}
		delete(r.spriteModels, key)
	}
}

func (r *Renderer) destroySpriteResourcesLocked() {
	r.clearSpriteModelsLocked()
	if r.spriteUniformBuffer != nil {
		r.spriteUniformBuffer.Release()
		r.spriteUniformBuffer = nil
	}
	if r.spriteUniformBindGroup != nil {
		r.spriteUniformBindGroup.Release()
		r.spriteUniformBindGroup = nil
	}
	if r.spritePipelineLayout != nil {
		r.spritePipelineLayout.Release()
		r.spritePipelineLayout = nil
	}
	if r.spriteUniformBindGroupLayout != nil {
		r.spriteUniformBindGroupLayout.Release()
		r.spriteUniformBindGroupLayout = nil
	}
	if r.spritePipeline != nil {
		r.spritePipeline.Release()
		r.spritePipeline = nil
	}
	if r.spriteDepthOffsetPipeline != nil {
		r.spriteDepthOffsetPipeline.Release()
		r.spriteDepthOffsetPipeline = nil
	}
	if r.spriteVertexShader != nil {
		r.spriteVertexShader.Release()
		r.spriteVertexShader = nil
	}
	if r.spriteFragmentShader != nil {
		r.spriteFragmentShader.Release()
		r.spriteFragmentShader = nil
	}
}

func (r *Renderer) destroyDecalResourcesLocked() {
	if r.decalBindGroup != nil {
		r.decalBindGroup.Release()
		r.decalBindGroup = nil
	}
	if r.decalAtlasView != nil {
		r.decalAtlasView.Release()
		r.decalAtlasView = nil
	}
	if r.decalAtlasTextureHAL != nil {
		r.decalAtlasTextureHAL.Release()
		r.decalAtlasTextureHAL = nil
	}
	if r.decalUniformBuffer != nil {
		r.decalUniformBuffer.Release()
		r.decalUniformBuffer = nil
	}
	if r.decalUniformBindGroup != nil {
		r.decalUniformBindGroup.Release()
		r.decalUniformBindGroup = nil
	}
	if r.decalUniformLayout != nil {
		r.decalUniformLayout.Release()
		r.decalUniformLayout = nil
	}
	if r.decalPipelineLayout != nil {
		r.decalPipelineLayout.Release()
		r.decalPipelineLayout = nil
	}
	if r.decalPipeline != nil {
		r.decalPipeline.Release()
		r.decalPipeline = nil
	}
	if r.decalVertexShader != nil {
		r.decalVertexShader.Release()
		r.decalVertexShader = nil
	}
	if r.decalFragmentShader != nil {
		r.decalFragmentShader.Release()
		r.decalFragmentShader = nil
	}
}

// ---- merged from world_support_gogpu_root.go ----
type gpuAliasSkin struct {
	texture           *wgpu.Texture
	view              *wgpu.TextureView
	fullbrightTexture *wgpu.Texture
	fullbrightView    *wgpu.TextureView
	bindGroup         *wgpu.BindGroup
}

type gpuAliasModel struct {
	modelID     string
	flags       int
	skins       []gpuAliasSkin
	playerSkins map[uint32][]gpuAliasSkin
	poses       [][]model.TriVertX
	refs        []aliasimpl.MeshRef
	// GPU-side buffers for GPU keyframe interpolation.
	poseBuffer         *wgpu.Buffer
	vertexBuffer       *wgpu.Buffer
	vertexCount        uint32
	scale              [3]float32
	scaleOrigin         [3]float32
	instanceBindGroup   *wgpu.BindGroup
}

type gpuAliasDraw struct {
	alias  *gpuAliasModel
	model  *model.Model
	pose1  int
	pose2  int
	blend  float32
	skin   *gpuAliasSkin
	origin [3]float32
	angles [3]float32
	alpha  float32
	scale  float32
	full   bool
}

type gpuSpriteFrame struct {
	meta      spriteRenderFrame
	texture   *wgpu.Texture
	view      *wgpu.TextureView
	bindGroup *wgpu.BindGroup
}

type gpuSpriteModel struct {
	modelID    string
	spriteType int
	frames     []gpuSpriteFrame
	maxWidth   int
	maxHeight  int
	bounds     [3][2]float32
}

type gpuSpriteDraw struct {
	sprite *gpuSpriteModel
	frame  int
	origin [3]float32
	angles [3]float32
	alpha  float32
	scale  float32
}

func (r *Renderer) ensureAliasInstanceUniformBufferLocked(device *wgpu.Device, numDraws int) error {
	needed := uint64(numDraws) * worldUniformAlign
	if needed < aliasInstanceUniformSize {
		needed = aliasInstanceUniformSize
	}
	minNeeded := uint64(aliasInitialDrawCapacity) * worldUniformAlign
	if needed < minNeeded {
		needed = minNeeded
	}
	if r.aliasInstanceUniformBuffer != nil && r.aliasInstanceUniformBuffer.Size() >= needed {
		return nil
	}
	oldBuffer := r.aliasInstanceUniformBuffer
	oldBindGroup := r.aliasInstanceUniformBindGroup
	buf, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "Alias Instance Uniform Buffer",
		Size:             needed,
		Usage:            gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return fmt.Errorf("grow alias instance uniform buffer: %w", err)
	}
	r.aliasInstanceUniformBuffer = buf
	if oldBuffer != nil {
		old := oldBuffer
		oldBG := oldBindGroup
		r.enqueueReleaseLocked(func() {
			oldBG.Release()
			old.Release()
		})
	}
	return nil
}

func (r *Renderer) ensureAliasScratchBufferLocked(device *wgpu.Device, size uint64) error {
	if size == 0 {
		size = 44
	}
	// Pre-allocate a generous minimum to avoid releasing and recreating the
	// buffer mid-frame while the GPU may still be reading from it via prior
	// command buffer submissions.
	minSize := uint64(2 * 1024 * 1024) // 2 MB — enough for ~40k vertices
	if size < minSize {
		size = minSize
	}
	if r.aliasScratchBuffer != nil && r.aliasScratchBufferSize >= size {
		return nil
	}
	// Grow: create a new buffer and defer-release the old one to avoid
	// GPU buffer-use-after-free.
	oldBuffer := r.aliasScratchBuffer
	buffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "Alias Scratch Buffer",
		Size:             size,
		Usage:            gputypes.BufferUsageVertex | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return fmt.Errorf("create alias scratch buffer: %w", err)
	}
	r.aliasScratchBuffer = buffer
	r.aliasScratchBufferSize = size
	if oldBuffer != nil {
		old := oldBuffer
		r.enqueueReleaseLocked(func() { old.Release() })
	}
	return nil
}

func (r *Renderer) ensureBrushEntityScratchBuffersLocked(device *wgpu.Device, vertexSize, indexSize uint64) error {
	if vertexSize == 0 {
		vertexSize = 44
	}
	if indexSize == 0 {
		indexSize = 4
	}
	if r.brushEntityScratchVertexBuffer != nil && r.brushEntityScratchVertexSize >= vertexSize &&
		r.brushEntityScratchIndexBuffer != nil && r.brushEntityScratchIndexSize >= indexSize {
		return nil
	}
	oldVertexBuffer := r.brushEntityScratchVertexBuffer
	oldIndexBuffer := r.brushEntityScratchIndexBuffer
	vertexBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "Brush Entity Vertex Scratch Buffer",
		Size:             vertexSize,
		Usage:            gputypes.BufferUsageVertex | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return fmt.Errorf("create brush entity vertex scratch buffer: %w", err)
	}
	indexBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "Brush Entity Index Scratch Buffer",
		Size:             indexSize,
		Usage:            gputypes.BufferUsageIndex | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		vertexBuffer.Release()
		return fmt.Errorf("create brush entity index scratch buffer: %w", err)
	}
	r.brushEntityScratchVertexBuffer = vertexBuffer
	r.brushEntityScratchVertexSize = vertexSize
	r.brushEntityScratchIndexBuffer = indexBuffer
	r.brushEntityScratchIndexSize = indexSize
	if oldVertexBuffer != nil {
		oldVB := oldVertexBuffer
		r.enqueueReleaseLocked(func() { oldVB.Release() })
	}
	if oldIndexBuffer != nil {
		oldIB := oldIndexBuffer
		r.enqueueReleaseLocked(func() { oldIB.Release() })
	}
	return nil
}

// ensureAliasUniformBufferLocked ensures the alias uniform buffer is large
// enough for numDraws draws. It never releases the existing buffer mid-frame
// to avoid GPU buffer-use-after-free — if the buffer is too small, it skips
// growing rather than corrupting GPU state.
func (r *Renderer) ensureAliasUniformBufferLocked(device *wgpu.Device, numDraws int) error {
	needed := uint64(numDraws) * worldUniformAlign
	if needed < aliasSceneUniformBufferSize {
		needed = aliasSceneUniformBufferSize
	}
	// Pre-allocate for the initial draw capacity to avoid growing mid-frame.
	minNeeded := uint64(aliasInitialDrawCapacity) * worldUniformAlign
	if needed < minNeeded {
		needed = minNeeded
	}
	if r.aliasUniformBuffer != nil && r.aliasUniformBuffer.Size() >= needed {
		return nil
	}
	// Grow: create a new buffer and defer-release the old one to avoid
	// GPU buffer-use-after-free.
	oldBuffer := r.aliasUniformBuffer
	oldBindGroup := r.aliasUniformBindGroup
	buf, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "Alias Uniform Buffer",
		Size:             needed,
		Usage:            gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return fmt.Errorf("grow alias uniform buffer: %w", err)
	}
	bg, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:   "Alias Uniform BG",
		Layout:  r.aliasUniformBindGroupLayout,
		Entries: []wgpu.BindGroupEntry{{Binding: 0, Buffer: buf, Offset: 0, Size: aliasSceneUniformBufferSize}},
	})
	if err != nil {
		buf.Release()
		return fmt.Errorf("recreate alias uniform bind group: %w", err)
	}
	r.aliasUniformBuffer = buf
	r.aliasUniformBindGroup = bg
	if oldBuffer != nil {
		old := oldBuffer
		oldBG := oldBindGroup
		r.enqueueReleaseLocked(func() {
			oldBG.Release()
			old.Release()
		})
	}
	return nil
}

func aliasDepthAttachmentForView(view *wgpu.TextureView) *wgpu.RenderPassDepthStencilAttachment {
	if view == nil {
		return nil
	}
	return &wgpu.RenderPassDepthStencilAttachment{
		View:              view,
		DepthLoadOp:       gputypes.LoadOpLoad,
		DepthStoreOp:      gputypes.StoreOpStore,
		DepthClearValue:   1.0,
		DepthReadOnly:     false,
		StencilLoadOp:     gputypes.LoadOpLoad,
		StencilStoreOp:    gputypes.StoreOpStore,
		StencilClearValue: 0,
		StencilReadOnly:   true,
	}
}

func putFloat32s(dst []byte, values []float32) {
	for i, value := range values {
		binary.LittleEndian.PutUint32(dst[i*4:(i+1)*4], math.Float32bits(value))
	}
}

// ---- merged from world_translucent_sort_gogpu_root.go ----
func destroyGoGPUTransientBuffers(buffers []*wgpu.Buffer) {
	for _, buffer := range buffers {
		if buffer != nil {
			buffer.Release()
		}
	}
}
