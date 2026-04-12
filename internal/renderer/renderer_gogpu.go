package renderer

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/darkliquid/ironwail-go/internal/image"
	"github.com/gogpu/gogpu"
	"github.com/gogpu/wgpu"
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
	lightSig          uint64
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

	// picRGBACache caches RGBA conversions of QPic images for CPU overlay compositing.
	// Keyed by QPic pointer identity (same as textureCache).
	picRGBACache map[*image.QPic][]byte
	// charCache caches per-character 8×8 QPic objects extracted from concharsData.
	charCache [256]*image.QPic

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

	// GPU resources for world rendering
	worldVertexBuffer                 *wgpu.Buffer
	worldIndexBuffer                  *wgpu.Buffer
	worldDynamicIndexBuffer           *wgpu.Buffer
	worldDynamicIndexBufferSize       uint64
	worldIndexCount                   uint32
	worldPipeline                     *wgpu.RenderPipeline
	worldAlphaTestPipeline            *wgpu.RenderPipeline
	worldTranslucentPipeline          *wgpu.RenderPipeline
	worldTurbulentPipeline            *wgpu.RenderPipeline
	worldTranslucentTurbulentPipeline *wgpu.RenderPipeline
	worldSkyPipeline                  *wgpu.RenderPipeline
	worldSkyExternalPipeline          *wgpu.RenderPipeline
	worldPipelineLayout               *wgpu.PipelineLayout
	worldSkyExternalPipelineLayout    *wgpu.PipelineLayout
	worldBindGroup                    *wgpu.BindGroup
	worldShader                       *wgpu.ShaderModule
	uniformBuffer                     *wgpu.Buffer
	uniformBindGroup                  *wgpu.BindGroup
	uniformBindGroupLayout            *wgpu.BindGroupLayout
	textureBindGroupLayout            *wgpu.BindGroupLayout
	worldSkyExternalBindGroupLayout   *wgpu.BindGroupLayout
	worldTextureSampler               *wgpu.Sampler
	worldTextures                     map[int32]*gpuWorldTexture
	worldFullbrightTextures           map[int32]*gpuWorldTexture
	worldSkySolidTextures             map[int32]*gpuWorldTexture
	worldSkyAlphaTextures             map[int32]*gpuWorldTexture
	worldTextureAnimations            []*SurfaceTexture
	worldSkyExternalTextures          [6]*wgpu.Texture
	worldSkyExternalViews             [6]*wgpu.TextureView
	worldSkyExternalBindGroup         *wgpu.BindGroup
	worldSkyExternalFaces             [6]externalSkyboxFace
	worldSkyExternalLoaded            int
	worldSkyExternalMode              externalSkyboxRenderMode
	worldSkyExternalName              string
	worldSkyExternalRequestID         uint64
	whiteTextureBindGroup             *wgpu.BindGroup
	transparentTexture                *wgpu.Texture
	transparentTextureView            *wgpu.TextureView
	transparentBindGroup              *wgpu.BindGroup
	worldLightmapSampler              *wgpu.Sampler
	worldLightmapPages                []*gpuWorldTexture
	whiteLightmapBindGroup            *wgpu.BindGroup
	worldLightStyleValues             [64]float32

	// 1x1 white texture for fallback
	whiteTexture          *wgpu.Texture
	whiteTextureView      *wgpu.TextureView
	worldDepthTexture     *wgpu.Texture
	worldDepthTextureView *wgpu.TextureView
	worldDepthWidth       int
	worldDepthHeight      int

	// Offscreen render target for world rendering
	worldRenderTexture              *wgpu.Texture
	worldRenderTextureView          *wgpu.TextureView
	worldRenderTextureGogpu         *gogpu.Texture // gogpu-wrapped version for compositing
	worldRenderWidth                int
	worldRenderHeight               int
	sceneCompositePipeline          *wgpu.RenderPipeline
	sceneCompositePipelineLayout    *wgpu.PipelineLayout
	sceneCompositeVertexShader      *wgpu.ShaderModule
	sceneCompositeFragmentShader    *wgpu.ShaderModule
	sceneCompositeBindGroupLayout   *wgpu.BindGroupLayout
	sceneCompositeSampler           *wgpu.Sampler
	sceneCompositeUniformBuffer     *wgpu.Buffer
	sceneCompositeBindGroup         *wgpu.BindGroup
	overlayCompositePipeline        *wgpu.RenderPipeline
	overlayCompositePipelineLayout  *wgpu.PipelineLayout
	overlayCompositeVertexShader    *wgpu.ShaderModule
	overlayCompositeFragmentShader  *wgpu.ShaderModule
	overlayCompositeBindGroupLayout *wgpu.BindGroupLayout
	overlayCompositeBindGroup       *wgpu.BindGroup
	overlayCompositeTextureView     *wgpu.TextureView

	// Alias-model resources for the gogpu backend.
	lightPool                      *glLightPool
	brushModelGeometry             map[int]*WorldGeometry
	brushModelLightmaps            map[int][]*gpuWorldTexture
	aliasModels                    map[string]*gpuAliasModel
	spriteModels                   map[string]*gpuSpriteModel
	aliasEntityStates              map[int]*AliasEntity
	viewModelAliasState            *AliasEntity
	aliasShadowSkin                *gpuAliasSkin
	aliasScratchBuffer             *wgpu.Buffer
	aliasScratchBufferSize         uint64
	brushEntityScratchVertexBuffer *wgpu.Buffer
	brushEntityScratchVertexSize   uint64
	brushEntityScratchIndexBuffer  *wgpu.Buffer
	brushEntityScratchIndexSize    uint64
	aliasPipeline                  *wgpu.RenderPipeline
	aliasShadowPipeline            *wgpu.RenderPipeline
	aliasPipelineLayout            *wgpu.PipelineLayout
	aliasVertexShader              *wgpu.ShaderModule
	aliasFragmentShader            *wgpu.ShaderModule
	aliasUniformBuffer             *wgpu.Buffer
	aliasUniformBindGroup          *wgpu.BindGroup
	aliasUniformBindGroupLayout    *wgpu.BindGroupLayout
	aliasTextureBindGroupLayout    *wgpu.BindGroupLayout
	aliasSampler                   *wgpu.Sampler
	spriteUniformBuffer            *wgpu.Buffer
	spriteUniformBindGroup         *wgpu.BindGroup
	spritePipeline                 *wgpu.RenderPipeline
	spriteDepthOffsetPipeline      *wgpu.RenderPipeline
	spriteVertexShader             *wgpu.ShaderModule
	spriteFragmentShader           *wgpu.ShaderModule
	particleOpaquePipeline         *wgpu.RenderPipeline
	particleTranslucentPipeline    *wgpu.RenderPipeline
	particlePipelineLayout         *wgpu.PipelineLayout
	particleVertexShader           *wgpu.ShaderModule
	particleFragmentShader         *wgpu.ShaderModule
	particleUniformBuffer          *wgpu.Buffer
	particleUniformBindGroup       *wgpu.BindGroup
	particleUniformBindGroupLayout *wgpu.BindGroupLayout
	decalPipeline                  *wgpu.RenderPipeline
	decalPipelineLayout            *wgpu.PipelineLayout
	decalVertexShader              *wgpu.ShaderModule
	decalFragmentShader            *wgpu.ShaderModule
	decalUniformBuffer             *wgpu.Buffer
	decalUniformBindGroup          *wgpu.BindGroup
	decalUniformLayout             *wgpu.BindGroupLayout
	decalAtlasTextureHAL           *wgpu.Texture
	decalAtlasView                 *wgpu.TextureView
	decalBindGroup                 *wgpu.BindGroup
	polyBlendPipeline              *wgpu.RenderPipeline
	polyBlendPipelineLayout        *wgpu.PipelineLayout
	polyBlendVertexShader          *wgpu.ShaderModule
	polyBlendFragmentShader        *wgpu.ShaderModule
	polyBlendUniformBuffer         *wgpu.Buffer
	polyBlendBindGroupLayout       *wgpu.BindGroupLayout
	polyBlendBindGroup             *wgpu.BindGroup

	// Cached overlay texture for 2D compositing — avoids creating a new
	// GPU texture every frame. Recreated only when screen dimensions change.
	overlayTexture       *gogpu.Texture
	overlayTextureWidth  int
	overlayTextureHeight int
	// Pooled CPU pixel buffer — avoids ~4.5MB allocation per frame.
	overlayPixelBuf          []byte
	overlayBufWidth          int
	overlayBufHeight         int
	overlayUploadBuf         []byte
	overlayTextureDirtyX     int
	overlayTextureDirtyY     int
	overlayTextureDirtyW     int
	overlayTextureDirtyH     int
	overlayTextureDirtyValid bool
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
