package renderer

import (
	"fmt"
	"log/slog"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// UploadWorld prepares BSP world geometry for rendering.
// This should be called once when a map is loaded.
//
// Uploads vertex and index buffers to GPU, compiles shaders,
// creates the render pipeline, and prepares for rendering.
func (r *Renderer) UploadWorld(tree *bsp.Tree) error {
	if tree == nil {
		return fmt.Errorf("nil BSP tree")
	}
	r.worldFirstFrameStatsLogged.Store(false)
	r.mu.Lock()
	r.brushModelGeometry = make(map[int]*WorldGeometry)
	r.resetGoGPUWorldBatchCache()
	r.mu.Unlock()

	slog.Debug("Uploading world geometry to GPU")

	// Build geometry from BSP
	geom, err := BuildWorldGeometry(tree)
	if err != nil {
		return fmt.Errorf("build world geometry: %w", err)
	}
	liquidAlpha := worldLiquidAlphaSettingsForGeometry(geom)
	faceStats := summarizeGoGPUWorldFaceStats(geom.Faces, liquidAlpha)

	// Create render data
	renderData := &WorldRenderData{
		Geometry:      geom,
		TotalVertices: len(geom.Vertices),
		TotalIndices:  len(geom.Indices),
		TotalFaces:    len(geom.Faces),
	}

	if len(geom.Vertices) > 0 {
		boundsMin := geom.Vertices[0].Position
		boundsMax := geom.Vertices[0].Position
		for index := 1; index < len(geom.Vertices); index++ {
			position := geom.Vertices[index].Position
			if position[0] < boundsMin[0] {
				boundsMin[0] = position[0]
			}
			if position[1] < boundsMin[1] {
				boundsMin[1] = position[1]
			}
			if position[2] < boundsMin[2] {
				boundsMin[2] = position[2]
			}

			if position[0] > boundsMax[0] {
				boundsMax[0] = position[0]
			}
			if position[1] > boundsMax[1] {
				boundsMax[1] = position[1]
			}
			if position[2] > boundsMax[2] {
				boundsMax[2] = position[2]
			}
		}

		renderData.BoundsMin = boundsMin
		renderData.BoundsMax = boundsMax
	}

	// Get HAL device and queue from gogpu renderer
	device := r.getWGPUDevice()
	queue := r.getWGPUQueue()
	if device == nil || queue == nil {
		slog.Warn("HAL device or queue not available, skipping GPU upload")
		// Store geometry anyway for later use
		r.mu.Lock()
		r.worldData = renderData
		r.mu.Unlock()
		slog.Info("GoGPU world upload stats",
			"gpu_upload", false,
			"bsp_version", tree.Version,
			"lighting_rgb", tree.LightingRGB,
			"raw_faces", len(tree.Faces),
			"built_faces", faceStats.TotalFaces,
			"built_triangles", faceStats.TotalTriangles,
			"vertices", renderData.TotalVertices,
			"lightmap_pages", len(geom.Lightmaps),
			"lightmapped_faces", faceStats.LightmappedFaces,
			"lit_water_faces", faceStats.LitWaterFaces,
			"turbulent_faces", faceStats.TurbulentFaces,
			"sky_faces", faceStats.SkyFaces,
			"opaque_faces", faceStats.OpaqueFaces,
			"alpha_test_faces", faceStats.AlphaTestFaces,
			"opaque_liquid_faces", faceStats.OpaqueLiquidFaces,
			"translucent_liquid_faces", faceStats.TranslucentLiquidFaces,
			"textures", tree.NumTextures,
			"leafs", len(tree.Leafs),
			"models", len(tree.Models),
		)
		return nil
	}

	// Upload vertex buffer
	vertexBuffer, err := r.createWorldVertexBuffer(device, queue, geom.Vertices)
	if err != nil {
		return fmt.Errorf("upload vertex buffer: %w", err)
	}

	// Upload index buffer
	indexBuffer, indexCount, err := r.createWorldIndexBuffer(device, queue, geom.Indices)
	if err != nil {
		return fmt.Errorf("upload index buffer: %w", err)
	}

	// Create shader modules (WGSL is compiled by HAL internally)
	vertexShader, err := createWorldShaderModule(device, worldVertexShaderWGSL, "World Vertex Shader")
	if err != nil {
		slog.Warn("Failed to create vertex shader", "error", err)
		vertexShader = nil
	}

	fragmentShader, err := createWorldShaderModule(device, worldFragmentShaderWGSL, "World Fragment Shader")
	if err != nil {
		slog.Warn("Failed to create fragment shader", "error", err)
		fragmentShader = nil
	}
	alphaTestFragmentShader, err := createWorldShaderModule(device, worldAlphaTestFragmentShaderWGSL, "World Alpha Test Fragment Shader")
	if err != nil {
		slog.Warn("Failed to create alpha-test fragment shader", "error", err)
		alphaTestFragmentShader = nil
	}
	skyVertexShader, err := createWorldShaderModule(device, worldSkyVertexShaderWGSL, "World Sky Vertex Shader")
	if err != nil {
		slog.Warn("Failed to create sky vertex shader", "error", err)
		skyVertexShader = nil
	}
	skyFragmentShader, err := createWorldShaderModule(device, worldSkyFragmentShaderWGSL, "World Sky Fragment Shader")
	if err != nil {
		slog.Warn("Failed to create sky fragment shader", "error", err)
		skyFragmentShader = nil
	}
	externalSkyFragmentShader, err := createWorldShaderModule(device, worldSkyExternalFaceFragmentShaderWGSL, "World External Sky Fragment Shader")
	if err != nil {
		slog.Warn("Failed to create external sky fragment shader", "error", err)
		externalSkyFragmentShader = nil
	}
	turbulentFragmentShader, err := createWorldShaderModule(device, worldTurbulentFragmentShaderWGSL, "World Turbulent Fragment Shader")
	if err != nil {
		slog.Warn("Failed to create turbulent fragment shader", "error", err)
		turbulentFragmentShader = nil
	}

	// Create render pipeline (may fail if gogpu API not fully exposed)
	var pipeline *wgpu.RenderPipeline
	var pipelineLayout *wgpu.PipelineLayout
	var skyPipeline *wgpu.RenderPipeline
	var externalSkyPipeline *wgpu.RenderPipeline
	var externalSkyPipelineLayout *wgpu.PipelineLayout
	var externalSkyBindGroupLayout *wgpu.BindGroupLayout
	var alphaTestPipeline *wgpu.RenderPipeline
	var translucentPipeline *wgpu.RenderPipeline
	var turbulentPipeline *wgpu.RenderPipeline
	var translucentTurbulentPipeline *wgpu.RenderPipeline
	if vertexShader != nil && fragmentShader != nil {
		var err2 error
		pipeline, pipelineLayout, err2 = r.createWorldPipeline(device, vertexShader, fragmentShader)
		if err2 != nil {
			slog.Warn("Failed to create render pipeline", "error", err2)
		}
	}
	if pipelineLayout != nil && skyVertexShader != nil && skyFragmentShader != nil {
		skyPipeline, err = r.createWorldSkyPipeline(device, skyVertexShader, skyFragmentShader, pipelineLayout)
		if err != nil {
			slog.Warn("Failed to create world sky pipeline", "error", err)
			skyPipeline = nil
		}
	}
	if pipelineLayout != nil && vertexShader != nil && alphaTestFragmentShader != nil {
		alphaTestPipeline, err = r.createWorldOpaquePipeline(device, vertexShader, alphaTestFragmentShader, pipelineLayout)
		if err != nil {
			slog.Warn("Failed to create world alpha-test pipeline", "error", err)
			alphaTestPipeline = nil
		}
	}
	if skyVertexShader != nil && externalSkyFragmentShader != nil {
		externalSkyPipeline, externalSkyPipelineLayout, externalSkyBindGroupLayout, err = r.createWorldExternalSkyPipeline(device, skyVertexShader, externalSkyFragmentShader)
		if err != nil {
			slog.Warn("Failed to create external world sky pipeline", "error", err)
			externalSkyPipeline = nil
			externalSkyPipelineLayout = nil
			externalSkyBindGroupLayout = nil
		}
	}
	if pipelineLayout != nil && vertexShader != nil && fragmentShader != nil {
		translucentPipeline, err = r.createWorldTranslucentPipeline(device, vertexShader, fragmentShader, pipelineLayout)
		if err != nil {
			slog.Warn("Failed to create world translucent pipeline", "error", err)
			translucentPipeline = nil
		}
	}
	if pipelineLayout != nil && vertexShader != nil && turbulentFragmentShader != nil {
		turbulentPipeline, err = r.createWorldTurbulentPipeline(device, vertexShader, turbulentFragmentShader, pipelineLayout)
		if err != nil {
			slog.Warn("Failed to create world turbulent pipeline", "error", err)
			turbulentPipeline = nil
		}
		translucentTurbulentPipeline, err = r.createWorldTranslucentTurbulentPipeline(device, vertexShader, turbulentFragmentShader, pipelineLayout)
		if err != nil {
			slog.Warn("Failed to create world translucent turbulent pipeline", "error", err)
			translucentTurbulentPipeline = nil
		}
	}

	// Create uniform buffer for VP matrix
	uniformBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "World Uniforms",
		Size:             worldUniformBufferSize,
		Usage:            gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return fmt.Errorf("create uniform buffer: %w", err)
	}

	// Create bind group for world uniform buffer.
	uniformLayout := r.uniformBindGroupLayout
	if uniformLayout != nil {
		uniformBindGroup, bindErr := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
			Label:  "World Uniform BG",
			Layout: uniformLayout,
			Entries: []wgpu.BindGroupEntry{
				{Binding: 0, Buffer: uniformBuffer, Offset: 0, Size: worldUniformBufferSize},
			},
		})
		if bindErr != nil {
			slog.Warn("Failed to create world uniform bind group", "error", bindErr)
		} else {
			r.uniformBindGroup = uniformBindGroup
			r.worldBindGroup = uniformBindGroup
		}
	}

	// Create white texture for fallback
	whiteTexture, whiteTextureView, err := r.createWorldWhiteTexture(device, queue)
	if err != nil {
		slog.Warn("Failed to create white texture", "error", err)
		// Don't fail completely, will use fallback rendering
	}
	var worldTextureSampler *wgpu.Sampler
	var whiteTextureBindGroup *wgpu.BindGroup
	var transparentTexture *wgpu.Texture
	var transparentTextureView *wgpu.TextureView
	var transparentBindGroup *wgpu.BindGroup
	if r.textureBindGroupLayout != nil {
		worldTextureSampler, err = r.createWorldTextureSampler(device)
		if err != nil {
			slog.Warn("Failed to create world texture sampler", "error", err)
		} else if whiteTextureView != nil {
			whiteTextureBindGroup, err = r.createWorldTextureBindGroup(device, worldTextureSampler, whiteTextureView)
			if err != nil {
				slog.Warn("Failed to create white world texture bind group", "error", err)
			}
		}
		if worldTextureSampler != nil {
			transparentTextureResource, transparentViewResource, transparentErr := r.createWorldWhiteTexture(device, queue)
			if transparentErr != nil {
				slog.Warn("Failed to create transparent fallback texture", "error", transparentErr)
			} else {
				transparentTexture = transparentTextureResource
				transparentTextureView = transparentViewResource
				if queueErr := queue.WriteTexture(&wgpu.ImageCopyTexture{
					Texture:  transparentTexture,
					MipLevel: 0,
					Aspect:   gputypes.TextureAspectAll,
				}, []byte{0, 0, 0, 0}, &wgpu.ImageDataLayout{BytesPerRow: 4, RowsPerImage: 1}, &wgpu.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1}); queueErr != nil {
					slog.Warn("Failed to zero transparent fallback texture", "error", queueErr)
				} else {
					transparentBindGroup, err = r.createWorldTextureBindGroup(device, worldTextureSampler, transparentTextureView)
					if err != nil {
						slog.Warn("Failed to create transparent world texture bind group", "error", err)
					}
				}
			}
		}
	}
	worldTextures, worldFullbrightTextures, worldTextureAnimations := r.uploadWorldMaterialTextures(device, queue, worldTextureSampler, tree)
	worldSkySolidTextures, worldSkyAlphaTextures := r.uploadWorldEmbeddedSkyTextures(device, queue, worldTextureSampler, tree)
	lightstyleValues := defaultWorldLightStyleValues()
	var worldLightmapSampler *wgpu.Sampler
	var whiteLightmapBindGroup *wgpu.BindGroup
	if r.textureBindGroupLayout != nil {
		worldLightmapSampler, err = r.createWorldLightmapSampler(device)
		if err != nil {
			slog.Warn("Failed to create world lightmap sampler", "error", err)
		} else if fallbackView := worldLightmapFallbackView(transparentTextureView, whiteTextureView); fallbackView != nil {
			// Match C brush rendering: faces without valid lightmap data should
			// sample black, not white, so localized assignment failures stay dark.
			whiteLightmapBindGroup, err = r.createWorldTextureBindGroup(device, worldLightmapSampler, fallbackView)
			if err != nil {
				slog.Warn("Failed to create world lightmap fallback bind group", "error", err)
			}
		}
	}
	worldLightmapPages := r.uploadWorldLightmapPages(device, queue, worldLightmapSampler, geom.Lightmaps, lightstyleValues)

	// Create offscreen render target for world rendering
	if err := r.createWorldRenderTarget(); err != nil {
		slog.Warn("Failed to create world render target", "error", err)
		// Don't fail completely, will use direct rendering fallback
	}

	width, height := r.Size()
	var depthTexture *wgpu.Texture
	var depthTextureView *wgpu.TextureView
	if width > 0 && height > 0 {
		depthTexture, depthTextureView, err = r.createWorldDepthTexture(device, width, height)
		if err != nil {
			slog.Warn("Failed to create world depth texture", "error", err)
		}
	}

	// Store GPU resources in renderer
	r.mu.Lock()
	r.worldData = renderData
	r.worldVertexBuffer = vertexBuffer
	r.worldIndexBuffer = indexBuffer
	r.worldIndexCount = indexCount
	r.worldPipeline = pipeline
	r.worldAlphaTestPipeline = alphaTestPipeline
	r.worldTranslucentPipeline = translucentPipeline
	r.worldTurbulentPipeline = turbulentPipeline
	r.worldTranslucentTurbulentPipeline = translucentTurbulentPipeline
	r.worldSkyPipeline = skyPipeline
	r.worldSkyExternalPipeline = externalSkyPipeline
	r.worldPipelineLayout = pipelineLayout
	r.worldSkyExternalPipelineLayout = externalSkyPipelineLayout
	r.worldShader = vertexShader
	r.uniformBuffer = uniformBuffer
	r.whiteTexture = whiteTexture
	r.whiteTextureView = whiteTextureView
	r.worldTextureSampler = worldTextureSampler
	r.worldTextures = worldTextures
	r.worldFullbrightTextures = worldFullbrightTextures
	r.worldSkySolidTextures = worldSkySolidTextures
	r.worldSkyAlphaTextures = worldSkyAlphaTextures
	r.worldTextureAnimations = worldTextureAnimations
	r.worldSkyExternalBindGroupLayout = externalSkyBindGroupLayout
	r.whiteTextureBindGroup = whiteTextureBindGroup
	r.transparentTexture = transparentTexture
	r.transparentTextureView = transparentTextureView
	r.transparentBindGroup = transparentBindGroup
	r.worldLightmapSampler = worldLightmapSampler
	r.worldLightmapPages = worldLightmapPages
	r.whiteLightmapBindGroup = whiteLightmapBindGroup
	r.worldLightStyleValues = lightstyleValues
	r.worldDepthTexture = depthTexture
	r.worldDepthTextureView = depthTextureView
	if depthTexture != nil {
		r.worldDepthWidth = width
		r.worldDepthHeight = height
	}
	if err := r.ensureGoGPUExternalSkyboxLocked(device, queue); err != nil && r.worldSkyExternalMode == externalSkyboxRenderFaces {
		slog.Debug("external gogpu skybox remains deferred", "name", r.worldSkyExternalName, "error", err)
	}
	renderData.VertexBufferUploaded = vertexBuffer != nil
	renderData.IndexBufferUploaded = indexBuffer != nil
	renderData.HasDiffuseTextures = len(worldTextures) > 0
	renderData.HasLightmapTextures = len(worldLightmapPages) > 0
	renderData.HasDepthBuffer = depthTextureView != nil
	r.mu.Unlock()

	slog.Debug("World geometry uploaded to GPU",
		"vertices", renderData.TotalVertices,
		"indices", renderData.TotalIndices,
		"faces", renderData.TotalFaces,
		"triangles", renderData.TotalIndices/3,
		"boundsMin", renderData.BoundsMin,
		"boundsMax", renderData.BoundsMax,
		"vertexBufferSize", uint64(len(geom.Vertices))*44,
		"indexBufferSize", uint64(len(geom.Indices))*4)
	slog.Info("GoGPU world upload stats",
		"gpu_upload", true,
		"bsp_version", tree.Version,
		"lighting_rgb", tree.LightingRGB,
		"raw_faces", len(tree.Faces),
		"built_faces", faceStats.TotalFaces,
		"built_triangles", faceStats.TotalTriangles,
		"vertices", renderData.TotalVertices,
		"lightmap_pages", len(geom.Lightmaps),
		"gpu_lightmap_pages", len(worldLightmapPages),
		"lightmapped_faces", faceStats.LightmappedFaces,
		"lit_water_faces", faceStats.LitWaterFaces,
		"turbulent_faces", faceStats.TurbulentFaces,
		"sky_faces", faceStats.SkyFaces,
		"opaque_faces", faceStats.OpaqueFaces,
		"alpha_test_faces", faceStats.AlphaTestFaces,
		"opaque_liquid_faces", faceStats.OpaqueLiquidFaces,
		"translucent_liquid_faces", faceStats.TranslucentLiquidFaces,
		"textures", tree.NumTextures,
		"gpu_textures", len(worldTextures),
		"gpu_fullbright_textures", len(worldFullbrightTextures),
		"gpu_sky_solid_textures", len(worldSkySolidTextures),
		"gpu_sky_alpha_textures", len(worldSkyAlphaTextures),
		"leafs", len(tree.Leafs),
		"models", len(tree.Models),
	)

	return nil
}

// renderWorldInternal implements world rendering.
// This records render commands to draw the world geometry with the configured pipeline,
