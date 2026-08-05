package renderer

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/renderer/pipeline"
	"github.com/gogpu/gogpu"
	"github.com/gogpu/wgpu"

	surfacepkg "github.com/darkliquid/ironwail-go/internal/renderer/surface"
)

type DrawContext struct {
	// ctx is the underlying gogpu rendering context.
	ctx *gogpu.Context

	// gamma is the current gamma correction value.
	gamma float32

	// renderer is the parent Renderer instance.
	renderer *Renderer

	// Canvas coordinate system state.
	canvas            CanvasState
	canvasParams      CanvasTransformParams
	sceneRenderActive bool
	sceneRenderTarget *wgpu.TextureView

	// overlay is the CPU-side 2D compositor buffer. All 2D draw calls
	// (DrawPic, DrawFill, DrawCharacter, DrawString) composite into this
	// buffer at screen resolution instead of issuing individual GPU
	// submissions through gogpu's 2D API. The overlay is flushed as a
	// single texture upload + draw at the end of the 2D overlay phase.
	overlay *overlay2D

	// Persistent scratch buffers for alias model rendering to avoid per-frame heap allocations.
	aliasPreparedScratch []gpuPreparedAliasDraw
	aliasVertexScratch   []WorldVertex
	aliasBulkVertexData  []byte
	aliasBulkUniformData []byte
	aliasVertexOffsets   []uint64
	aliasVertexCounts    []uint32
	aliasUniformOffsets  []uint32
}

type gpuPreparedAliasDraw struct {
	draw        gpuAliasDraw
	skin        *gpuAliasSkin
	alpha       float32
	vertexCount uint32
}

func validatedGoGPURenderPipeline(device *wgpu.Device, desc *wgpu.RenderPipelineDescriptor) (*wgpu.RenderPipeline, error) {
	if device == nil {
		return nil, fmt.Errorf("nil device")
	}
	if desc == nil {
		return nil, fmt.Errorf("nil render pipeline descriptor")
	}
	slog.Debug("Creating GPU Render Pipeline", "label", desc.Label, "vertex shader", fmt.Sprintf("%p", desc.Vertex.Module), "fragment shader", fmt.Sprintf("%p", desc.Fragment))
	return device.CreateRenderPipeline(desc)
}

var halOnlyFrameConsumed atomic.Bool

// overlay2D is a CPU-side RGBA compositor buffer at screen resolution.
// Instead of issuing one GPU command encoder + render pass + submit per 2D
// draw call (which is what gogpu's DrawTextureScaled/DrawTextureEx does),
// all 2D draws composite into this buffer on the CPU. The buffer is then
// uploaded as a single GPU texture and drawn in one submit at the end of
// the 2D overlay phase.

type cacheKey struct {
	pic *image.QPic
}

type cachedTexture struct {
	texture *gogpu.Texture
	width   int
	height  int
}

const gogpuWorldBatchCacheEntryCount = 4

type gogpuWorldBatchCacheEntry struct {
	valid             bool
	leaf              int
	liquidAlpha       worldLiquidAlphaSettings
	faceCount         int
	skyFaces          []WorldFace
	translucentLiquid []WorldFace
	indices           []uint32
	opaque            []gogpuWorldFaceBatch
	alpha             []gogpuWorldFaceBatch
	liquid            []gogpuWorldFaceBatch
}

// Renderer is the main rendering context for the Ironwail-Go engine.
// It manages the gogpu application window, handles the game loop,
// and provides rendering callbacks for the game logic.
//
// Thread Safety:
//
//	Renderer is thread-safe for configuration changes via SetConfig.
//	OnUpdate runs on gogpu's main-thread event loop, while OnDraw and OnClose
//	run on gogpu's dedicated locked render thread.
//
// Lifecycle:
//
//  1. Create with New() or NewWithConfig()
//  2. Set up callbacks with OnDraw() and OnUpdate()
//  3. Run the main loop with Run()
//  4. Shutdown() is called automatically or manually for cleanup
type Renderer struct {
	mu sync.RWMutex

	// app is the gogpu application instance.
	app *gogpu.App

	// config is the current video configuration.
	config Config

	// drawCallback is called each frame to render the scene.
	drawCallback func(dc RenderContext)
	// updateCallback is called each frame for game logic updates.
	updateCallback func(dt float64)

	// closeCallback is called when the window is closed.
	closeCallback func()

	// running indicates if the main loop is active.
	running bool

	// textureCache stores uploaded textures to avoid re-uploading
	textureCache map[cacheKey]*cachedTexture

	// colorTextures stores 1x1 textures for solid colors
	colorTextures [256]*gogpu.Texture

	// palette is the current Quake palette (768 bytes)
	palette []byte

	// concharsData is the raw 128×128 indexed-pixel data for the console font.
	concharsData []byte

	// charCache caches per-character 8×8 QPic objects extracted from concharsData.
	charCache [256]*image.QPic

	// charTextures caches uploaded GPU textures for the 256 font characters.
	charTextures [256]*gogpu.Texture

	// Camera state and matrices for view/projection
	// cameraState holds the current camera position and orientation.
	cameraState CameraState
	// viewMatrices caches computed view and projection matrices.
	viewMatrices ViewMatrixData

	// worldData holds GPU-side resources for BSP world rendering.
	// Set via UploadWorld() when a map is loaded.
	worldData *WorldRenderData
	// worldFirstFrameStatsLogged gates one-shot first-world-frame diagnostics per upload.
	worldFirstFrameStatsLogged atomic.Bool
	// worldVisibleFacesScratch reuses visibility marking/storage across world passes.
	worldVisibleFacesScratch      worldVisibilityScratch
	worldSkyFacesScratch          []WorldFace
	worldTranslucentLiquidScratch []WorldFace
	worldOpaqueDrawsScratch       []gogpuWorldFaceDraw
	worldAlphaDrawsScratch        []gogpuWorldFaceDraw
	worldLiquidDrawsScratch       []gogpuWorldFaceDraw
	worldBatchedIndexScratch      []uint32
	worldOpaqueBatchScratch       []gogpuWorldFaceBatch
	worldAlphaBatchScratch        []gogpuWorldFaceBatch
	worldLiquidBatchScratch       []gogpuWorldFaceBatch
	worldBatchCacheEntries        [gogpuWorldBatchCacheEntryCount]gogpuWorldBatchCacheEntry
	worldBatchCacheNext           int

	// Scratch buffers for render state
	// (removed scratch maps for textures)
	brushTextureAnimationsScratch []*surfacepkg.SurfaceTexture
	activeDynamicLightsScratch    []DynamicLight
	uniformDataScratch            []byte
	uniformOffset                 uint32
	particleScratchBuffer         *wgpu.Buffer
	worldVertexBuffer             *wgpu.Buffer
	worldIndexBuffer              *wgpu.Buffer
	worldDynamicIndexBuffer       *wgpu.Buffer
	worldDynamicIndexBufferSize   uint64
	worldIndexCount               uint32
	// resources owns the renderer's wgpu object graph (plan 16+2a); the
	// fields below are the ones whose types live in renderer-root and cannot
	// move into pipeline.Resources without an import cycle.
	resources               *pipeline.Resources
	worldTextures           *gpuWorldTexture
	worldFullbrightTextures *gpuWorldTexture
	worldSkySolidTextures   map[int32]*gpuWorldTexture
	worldSkyAlphaTextures   map[int32]*gpuWorldTexture
	worldTextureAnimations  []*surfacepkg.SurfaceTexture
	worldBaseMaterials      []WorldMaterialData
	worldSkyExternalFaces   [6]externalSkyboxFace
	worldSkyExternalWind    externalSkyboxWind
	worldSkyExternalMode    externalSkyboxRenderMode
	worldLightmapArray      *gpuWorldTexture
	// Alias-model resources for the gogpu backend.
	lightPool                            *glLightPool
	brushModelGeometry                   map[int]*WorldGeometry
	brushModelLightmaps                  map[int]*gpuWorldTexture
	externalBrushGeometry                map[string]*WorldGeometry
	externalBrushTextures                map[string]*gpuWorldTexture
	externalBrushFullbright              map[string]*gpuWorldTexture
	externalBrushAnimations              map[string][]*surfacepkg.SurfaceTexture
	externalBrushBaseMaterials           map[string][]WorldMaterialData
	externalBrushMaterialsBuffers        map[string]*wgpu.Buffer
	externalBrushMaterialsBuffersFrame1  map[string]*wgpu.Buffer
	externalBrushUniformBindGroups       map[string]*wgpu.BindGroup
	externalBrushUniformBindGroupsFrame1 map[string]*wgpu.BindGroup
	aliasModels                          map[string]*gpuAliasModel
	spriteModels                         map[string]*gpuSpriteModel
	aliasEntityStates                    map[int]*AliasEntity
	viewModelAliasState                  *AliasEntity
	aliasScratchBuffer                   *wgpu.Buffer
	aliasScratchBufferSize               uint64
	brushEntityScratchVertexBuffer       *wgpu.Buffer
	brushEntityScratchVertexSize         uint64
	brushEntityScratchIndexBuffer        *wgpu.Buffer
	brushEntityScratchIndexSize          uint64
	aliasPipeline                        *wgpu.RenderPipeline
	aliasPipelineLayout                  *wgpu.PipelineLayout
	aliasVertexShader                    *wgpu.ShaderModule
	aliasFragmentShader                  *wgpu.ShaderModule
	aliasUniformBuffer                   *wgpu.Buffer
	aliasUniformBindGroup                *wgpu.BindGroup
	aliasUniformBindGroupLayout          *wgpu.BindGroupLayout
	aliasTextureBindGroupLayout          *wgpu.BindGroupLayout
	aliasSampler                         *wgpu.Sampler
	spriteUniformBuffer                  *wgpu.Buffer
	spriteUniformBindGroup               *wgpu.BindGroup
	spriteUniformBindGroupLayout         *wgpu.BindGroupLayout
	spritePipelineLayout                 *wgpu.PipelineLayout
	spritePipeline                       *wgpu.RenderPipeline
	spriteDepthOffsetPipeline            *wgpu.RenderPipeline
	spriteVertexShader                   *wgpu.ShaderModule
	spriteFragmentShader                 *wgpu.ShaderModule
	particleOpaquePipeline               *wgpu.RenderPipeline
	particleTranslucentPipeline          *wgpu.RenderPipeline
	particlePipelineLayout               *wgpu.PipelineLayout
	particleVertexShader                 *wgpu.ShaderModule
	particleFragmentShader               *wgpu.ShaderModule
	particleUniformBuffer                *wgpu.Buffer
	particleUniformBindGroup             *wgpu.BindGroup
	particleUniformBindGroupLayout       *wgpu.BindGroupLayout
	decalPipeline                        *wgpu.RenderPipeline
	decalPipelineLayout                  *wgpu.PipelineLayout
	decalVertexShader                    *wgpu.ShaderModule
	decalFragmentShader                  *wgpu.ShaderModule
	decalUniformBuffer                   *wgpu.Buffer
	decalUniformBindGroup                *wgpu.BindGroup
	decalUniformLayout                   *wgpu.BindGroupLayout
	decalAtlasTextureHAL                 *wgpu.Texture
	decalAtlasView                       *wgpu.TextureView
	decalBindGroup                       *wgpu.BindGroup
	polyBlendPipeline                    *wgpu.RenderPipeline
	polyBlendPipelineLayout              *wgpu.PipelineLayout
	polyBlendVertexShader                *wgpu.ShaderModule
	polyBlendFragmentShader              *wgpu.ShaderModule
	polyBlendUniformBuffer               *wgpu.Buffer
	polyBlendBindGroupLayout             *wgpu.BindGroupLayout
	polyBlendBindGroup                   *wgpu.BindGroup

	// Cached overlay texture for 2D compositing — avoids creating a new
	// GPU texture every frame. Recreated only when screen dimensions change.
	overlayTexture       *gogpu.Texture
	overlayTextureWidth  int
	overlayTextureHeight int
	// Pooled CPU pixel buffer — avoids ~4.5MB allocation per frame.
	overlayPixelBuf          []byte
	overlayUploadBuf         []byte
	overlayTextureDirtyX     int
	overlayTextureDirtyY     int
	overlayTextureDirtyW     int
	overlayTextureDirtyH     int
	overlayTextureDirtyValid bool

	// pendingReleases holds GPU resources that were replaced mid-frame
	// and need to be released after the GPU finishes processing prior
	// command buffers. Entries are drained after a 2-frame delay.
	pendingReleases []pendingRelease
	frameCounter    uint64
}

type pendingRelease struct {
	releaseFunc func()
	frame       uint64
}

// drainPendingReleases releases GPU resources whose frame delay has elapsed.
// Must be called with r.mu held.
func (r *Renderer) drainPendingReleasesLocked() {
	if len(r.pendingReleases) == 0 {
		return
	}
	kept := r.pendingReleases[:0]
	for _, pr := range r.pendingReleases {
		if r.frameCounter-pr.frame >= 2 {
			pr.releaseFunc()
		} else {
			kept = append(kept, pr)
		}
	}
	r.pendingReleases = kept
}

// enqueueReleaseLocked schedules a GPU resource for deferred release after
// a 2-frame delay, ensuring the GPU has finished processing any command
// buffers that reference it. Must be called with r.mu held.
func (r *Renderer) enqueueReleaseLocked(releaseFunc func()) {
	r.pendingReleases = append(r.pendingReleases, pendingRelease{
		releaseFunc: releaseFunc,
		frame:       r.frameCounter,
	})
}

// New creates a new Renderer with configuration from cvars.
// This is the standard way to create a renderer in Ironwail-Go,
// as it respects user-configurable video settings.
//
// Example:
//
//	r, err := renderer.New()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer r.Shutdown()
