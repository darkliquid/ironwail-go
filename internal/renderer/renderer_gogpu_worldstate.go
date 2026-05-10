package renderer

import (
	"log/slog"
	"time"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/pkg/types"
	"github.com/gogpu/wgpu"

	surfacepkg "github.com/darkliquid/ironwail-go/internal/renderer/surface"
)

const externalSkyboxLogSubsystem = "renderer.skybox"

// UpdateCamera updates the camera state and recomputes view/projection matrices.
// This should be called once per frame with the current player position and orientation
// from client prediction.
func (r *Renderer) UpdateCamera(camera CameraState, nearPlane, farPlane float32) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cameraState = camera

	// Compute view matrix from camera state
	r.viewMatrices.View = ComputeViewMatrix(camera)

	// Compute projection matrix (aspect ratio from window size)
	// Use a default aspect ratio if the app is not initialized
	aspect := float32(16.0 / 9.0)
	if r.app != nil {
		w, h := r.Size()
		if w > 0 && h > 0 {
			aspect = float32(w) / float32(h)
		}
	}

	r.viewMatrices.Projection = ComputeProjectionMatrix(projectionFOVForCamera(camera), aspect, nearPlane, farPlane)

	// Log individual matrices before multiplication
	slog.Debug("Camera matrices computed",
		"view_m00", r.viewMatrices.View[0],
		"view_m11", r.viewMatrices.View[5],
		"view_m22", r.viewMatrices.View[10],
		"view_m33", r.viewMatrices.View[15],
		"proj_m00", r.viewMatrices.Projection[0],
		"proj_m11", r.viewMatrices.Projection[5],
		"proj_m22", r.viewMatrices.Projection[10],
		"proj_m33", r.viewMatrices.Projection[15])

	// Compute combined VP matrix
	r.viewMatrices.VP = types.Mat4Multiply(r.viewMatrices.Projection, r.viewMatrices.View)

	// Log VP matrix for debugging
	slog.Debug("Camera updated",
		"position", camera.Origin,
		"angles", camera.Angles,
		"near", nearPlane,
		"far", farPlane,
		"aspect", aspect,
		"fov", camera.FOV,
		"vp_matrix_0_0", r.viewMatrices.VP[0],
		"vp_matrix_3_2", r.viewMatrices.VP[14])
}

// ViewMatrix returns the currently cached view matrix.
// Thread-safe read.
func (r *Renderer) ViewMatrix() types.Mat4 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.viewMatrices.View
}

// ProjectionMatrix returns the currently cached projection matrix.
// Thread-safe read.
func (r *Renderer) ProjectionMatrix() types.Mat4 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.viewMatrices.Projection
}

// ViewProjectionMatrix returns the combined Projection × View matrix.
// This is the matrix typically used in vertex shaders for world-to-NDC transformation.
// Thread-safe read.
func (r *Renderer) ViewProjectionMatrix() types.Mat4 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.viewMatrices.VP
}

// CameraState returns the current camera state (position and orientation).
// Thread-safe read. A copy is returned to prevent external modification.
func (r *Renderer) CameraState() CameraState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cameraState
}

// HasWorldData reports whether GPU world geometry has been uploaded.
func (r *Renderer) HasWorldData() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.worldData != nil && r.worldVertexBuffer != nil && r.worldIndexBuffer != nil && r.worldIndexCount > 0 && r.worldPipeline != nil
}

func (r *Renderer) SpawnDynamicLight(light DynamicLight) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lightPool == nil {
		return false
	}
	return r.lightPool.SpawnLight(light)
}

func (r *Renderer) SpawnKeyedDynamicLight(light DynamicLight) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lightPool == nil {
		return false
	}
	return r.lightPool.SpawnOrReplaceKeyed(light)
}

func (r *Renderer) UpdateLights(deltaTime float32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lightPool != nil {
		r.lightPool.UpdateAndFilter(deltaTime)
	}
}

func (r *Renderer) ClearDynamicLights() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lightPool != nil {
		r.lightPool.Clear()
	}
}

func (r *Renderer) SetExternalSkybox(name string, loadFile func(string) ([]byte, error)) {
	normalized := normalizeSkyboxBaseName(name)

	r.mu.Lock()
	if normalized == r.worldSkyExternalName {
		r.mu.Unlock()
		return
	}
	r.worldSkyExternalRequestID++
	requestID := r.worldSkyExternalRequestID

	r.destroyGoGPUExternalSkyboxResourcesLocked()
	r.worldSkyExternalFaces = [6]externalSkyboxFace{}
	r.worldSkyExternalWind = externalSkyboxWind{}
	r.worldSkyExternalWindLoaded = false
	r.worldSkyExternalLoaded = 0
	r.worldSkyExternalMode = externalSkyboxRenderEmbedded
	r.worldSkyExternalName = normalized
	r.worldSkyExternalLoading = normalized != "" && loadFile != nil
	r.worldSkyExternalUploadCursor = 0
	r.worldSkyExternalWorldDrawLogged = false
	r.worldSkyExternalBrushDrawLogged = false

	if !r.worldSkyExternalLoading {
		if normalized == "" {
			slog.Info("external skybox cleared", "subsystem", externalSkyboxLogSubsystem, "raw_name", name, "request_id", requestID)
		} else {
			slog.Warn("external skybox request ignored without loader", "subsystem", externalSkyboxLogSubsystem, "raw_name", name, "normalized_name", normalized, "request_id", requestID)
		}
		r.mu.Unlock()
		return
	}
	slog.Info("external skybox request queued", "subsystem", externalSkyboxLogSubsystem, "raw_name", name, "normalized_name", normalized, "request_id", requestID)
	r.mu.Unlock()

	go r.loadExternalSkyboxAsync(requestID, normalized, loadFile)
}

func (r *Renderer) loadExternalSkyboxAsync(requestID uint64, normalized string, loadFile func(string) ([]byte, error)) {
	start := time.Now()
	slog.Info("external skybox async load begin", "subsystem", externalSkyboxLogSubsystem, "name", normalized, "request_id", requestID)
	faces, loaded := loadExternalSkyboxFaces(normalized, loadFile)
	wind, windLoaded := loadExternalSkyboxWind(normalized, loadFile)

	r.mu.Lock()
	defer r.mu.Unlock()
	if requestID != r.worldSkyExternalRequestID || normalized != r.worldSkyExternalName {
		slog.Info("external skybox async load discarded", "subsystem", externalSkyboxLogSubsystem, "name", normalized, "request_id", requestID, "current_request_id", r.worldSkyExternalRequestID, "current_name", r.worldSkyExternalName, "loaded_faces", loaded, "elapsed_ms", elapsedMilliseconds(start))
		return
	}
	r.worldSkyExternalLoading = false
	if normalized == "" || loaded <= 0 {
		slog.Warn("external skybox async load found no usable faces", "subsystem", externalSkyboxLogSubsystem, "name", normalized, "request_id", requestID, "loaded_faces", loaded, "elapsed_ms", elapsedMilliseconds(start))
		return
	}
	r.worldSkyExternalFaces = faces
	r.worldSkyExternalWind = wind
	r.worldSkyExternalWindLoaded = windLoaded
	r.worldSkyExternalLoaded = loaded
	r.worldSkyExternalMode = externalSkyboxRenderFaces
	slog.Info("external skybox async load ready for upload", "subsystem", externalSkyboxLogSubsystem, "name", normalized, "request_id", requestID, "loaded_faces", loaded, "wind_loaded", windLoaded, "wind_dist", wind.Dist, "elapsed_ms", elapsedMilliseconds(start))
}

func (r *Renderer) UploadPendingExternalSkybox() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.worldSkyExternalMode != externalSkyboxRenderFaces || r.worldSkyExternalLoaded == 0 || r.worldSkyExternalBindGroup != nil {
		return nil
	}
	slog.Info("external skybox pending upload tick", "subsystem", externalSkyboxLogSubsystem, "name", r.worldSkyExternalName, "loaded_faces", r.worldSkyExternalLoaded, "upload_cursor", r.worldSkyExternalUploadCursor)
	if err := r.uploadNextGoGPUExternalSkyboxFaceLocked(r.getWGPUDevice(), r.getWGPUQueue()); err != nil {
		slog.Warn("external gogpu skybox upload deferred", "subsystem", externalSkyboxLogSubsystem, "name", r.worldSkyExternalName, "upload_cursor", r.worldSkyExternalUploadCursor, "error", err)
		return err
	}
	return nil
}

// NeedsWorldGPUUpload reports whether CPU world geometry exists but GPU buffers
// are not uploaded yet.
func (r *Renderer) NeedsWorldGPUUpload() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.worldData != nil && (r.worldVertexBuffer == nil || r.worldIndexBuffer == nil || r.worldIndexCount == 0)
}

// getWGPUDevice returns the public WebGPU device exposed by the app provider.
func (r *Renderer) getWGPUDevice() *wgpu.Device {
	if r.app == nil {
		return nil
	}
	provider := r.app.DeviceProvider()
	if provider == nil {
		return nil
	}
	raw := any(provider.Device())
	device, ok := raw.(*wgpu.Device)
	if !ok {
		return nil
	}
	return device
}

func (r *Renderer) getWGPUQueue() *wgpu.Queue {
	device := r.getWGPUDevice()
	if device == nil {
		return nil
	}
	return device.Queue()
}

func (r *Renderer) destroyGoGPUExternalSkyboxResourcesLocked() {
	if r.worldSkyExternalBindGroup != nil {
		r.worldSkyExternalBindGroup.Release()
		r.worldSkyExternalBindGroup = nil
	}
	for i := range r.worldSkyExternalViews {
		if r.worldSkyExternalViews[i] != nil {
			r.worldSkyExternalViews[i].Release()
			r.worldSkyExternalViews[i] = nil
		}
		if r.worldSkyExternalTextures[i] != nil {
			r.worldSkyExternalTextures[i].Release()
			r.worldSkyExternalTextures[i] = nil
		}
	}
	r.worldSkyExternalUploadCursor = 0
}

func (r *Renderer) ensureBrushModelGeometry(submodelIndex int) *WorldGeometry {
	if submodelIndex <= 0 {
		return nil
	}
	r.mu.RLock()
	if geom := r.brushModelGeometry[submodelIndex]; geom != nil {
		r.mu.RUnlock()
		return geom
	}
	tree := (*bsp.Tree)(nil)
	if r.worldData != nil && r.worldData.Geometry != nil {
		tree = r.worldData.Geometry.Tree
	}
	r.mu.RUnlock()
	if tree == nil {
		return nil
	}
	geom, err := BuildModelGeometry(tree, submodelIndex)
	if err != nil {
		slog.Debug("GoGPU brush model build skipped", "submodel", submodelIndex, "error", err)
		return nil
	}
	if geom == nil || len(geom.Vertices) == 0 {
		return nil
	}
	r.mu.Lock()
	if r.brushModelGeometry == nil {
		r.brushModelGeometry = make(map[int]*WorldGeometry)
	}
	if existing := r.brushModelGeometry[submodelIndex]; existing != nil {
		r.mu.Unlock()
		return existing
	}
	r.brushModelGeometry[submodelIndex] = geom
	r.mu.Unlock()
	return geom
}

func (r *Renderer) ensureBrushModelLightmaps(submodelIndex int, geom *WorldGeometry) []*gpuWorldTexture {
	if submodelIndex <= 0 || geom == nil || len(geom.Lightmaps) == 0 {
		return nil
	}
	r.mu.RLock()
	if cached := r.brushModelLightmaps[submodelIndex]; len(cached) > 0 {
		r.mu.RUnlock()
		return cached
	}
	sampler := r.worldLightmapSampler
	values := r.worldLightStyleValues
	r.mu.RUnlock()
	device := r.getWGPUDevice()
	queue := r.getWGPUQueue()
	if device == nil || queue == nil || sampler == nil {
		return nil
	}
	uploaded := r.uploadWorldLightmapPages(device, queue, sampler, geom.Lightmaps, values)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.brushModelLightmaps == nil {
		r.brushModelLightmaps = make(map[int][]*gpuWorldTexture)
	}
	if existing := r.brushModelLightmaps[submodelIndex]; len(existing) > 0 {
		for _, page := range uploaded {
			if page == nil {
				continue
			}
			if page.bindGroup != nil {
				page.bindGroup.Release()
			}
			if page.view != nil {
				page.view.Release()
			}
			if page.texture != nil {
				page.texture.Release()
			}
		}
		return existing
	}
	r.brushModelLightmaps[submodelIndex] = uploaded
	return uploaded
}

// ensureExternalBrushModelGeometry builds and caches geometry for a
// standalone BSP file (e.g. "maps/b_rock0.bsp") that is referenced as
// a brush entity but is not an inline submodel of the current world.
// The cache is keyed by external-model name so the same tree is only
// uploaded once regardless of how many entities reference it.
//
// Face texture indices inside an external BSP live in their own index
// space and will not match the current world texture table — the render
// path's whiteTexture fallback handles that (untextured geometry is
// still visually correct in position, shape and lighting).
func (r *Renderer) ensureExternalBrushModelGeometry(key string, tree *bsp.Tree) *WorldGeometry {
	if key == "" || tree == nil {
		return nil
	}
	r.mu.RLock()
	if geom := r.externalBrushGeometry[key]; geom != nil {
		r.mu.RUnlock()
		return geom
	}
	r.mu.RUnlock()
	geom, err := BuildModelGeometry(tree, 0)
	if err != nil {
		slog.Debug("GoGPU external brush model build skipped", "key", key, "error", err)
		return nil
	}
	if geom == nil || len(geom.Vertices) == 0 {
		return nil
	}
	r.mu.Lock()
	if r.externalBrushGeometry == nil {
		r.externalBrushGeometry = make(map[string]*WorldGeometry)
	}
	if existing := r.externalBrushGeometry[key]; existing != nil {
		r.mu.Unlock()
		return existing
	}
	r.externalBrushGeometry[key] = geom
	r.mu.Unlock()
	return geom
}

// ensureExternalBrushModelLightmaps uploads and caches lightmap pages
// for an external (standalone) BSP. Unlike world lightmaps these are
// self-contained inside the external tree's geometry and are not
// animated by the world's lightstyle values.
func (r *Renderer) ensureExternalBrushModelLightmaps(key string, geom *WorldGeometry) []*gpuWorldTexture {
	if key == "" || geom == nil || len(geom.Lightmaps) == 0 {
		return nil
	}
	r.mu.RLock()
	if cached := r.externalBrushLightmaps[key]; len(cached) > 0 {
		r.mu.RUnlock()
		return cached
	}
	sampler := r.worldLightmapSampler
	values := r.worldLightStyleValues
	r.mu.RUnlock()
	device := r.getWGPUDevice()
	queue := r.getWGPUQueue()
	if device == nil || queue == nil || sampler == nil {
		return nil
	}
	uploaded := r.uploadWorldLightmapPages(device, queue, sampler, geom.Lightmaps, values)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.externalBrushLightmaps == nil {
		r.externalBrushLightmaps = make(map[string][]*gpuWorldTexture)
	}
	if existing := r.externalBrushLightmaps[key]; len(existing) > 0 {
		for _, page := range uploaded {
			if page == nil {
				continue
			}
			if page.bindGroup != nil {
				page.bindGroup.Release()
			}
			if page.view != nil {
				page.view.Release()
			}
			if page.texture != nil {
				page.texture.Release()
			}
		}
		return existing
	}
	r.externalBrushLightmaps[key] = uploaded
	return uploaded
}

// brushEntityGeometry returns the geometry for a brush entity,
// dispatching to the external-BSP path when BrushEntity.ExternalKey is set.
func (r *Renderer) brushEntityGeometry(entity BrushEntity) *WorldGeometry {
	if entity.ExternalKey != "" && entity.ExternalTree != nil {
		return r.ensureExternalBrushModelGeometry(entity.ExternalKey, entity.ExternalTree)
	}
	return r.ensureBrushModelGeometry(entity.SubmodelIndex)
}

// brushEntityLightmaps returns lightmap pages for a brush entity,
// dispatching to the external-BSP path when BrushEntity.ExternalKey is set.
func (r *Renderer) brushEntityLightmaps(entity BrushEntity, geom *WorldGeometry) []*gpuWorldTexture {
	if entity.ExternalKey != "" {
		return r.ensureExternalBrushModelLightmaps(entity.ExternalKey, geom)
	}
	return r.ensureBrushModelLightmaps(entity.SubmodelIndex, geom)
}

// ensureExternalBrushModelTextures uploads and caches diffuse + fullbright
// textures and texture-animation chains for a standalone BSP file. Like
// the geometry/lightmap caches the entries are keyed by external-model
// name so repeat entities share the same GPU resources.
func (r *Renderer) ensureExternalBrushModelTextures(key string, tree *bsp.Tree) (map[int32]*gpuWorldTexture, map[int32]*gpuWorldTexture, []*surfacepkg.SurfaceTexture) {
	if key == "" || tree == nil {
		return nil, nil, nil
	}
	r.mu.RLock()
	if textures, ok := r.externalBrushTextures[key]; ok {
		fullbright := r.externalBrushFullbright[key]
		animations := r.externalBrushAnimations[key]
		r.mu.RUnlock()
		return textures, fullbright, animations
	}
	sampler := r.worldTextureSampler
	r.mu.RUnlock()
	device := r.getWGPUDevice()
	queue := r.getWGPUQueue()
	if device == nil || queue == nil || sampler == nil {
		return nil, nil, nil
	}
	textures, fullbright, animations := r.uploadWorldMaterialTextures(device, queue, sampler, tree)
	r.mu.Lock()
	if r.externalBrushTextures == nil {
		r.externalBrushTextures = make(map[string]map[int32]*gpuWorldTexture)
		r.externalBrushFullbright = make(map[string]map[int32]*gpuWorldTexture)
		r.externalBrushAnimations = make(map[string][]*surfacepkg.SurfaceTexture)
	}
	if existing, ok := r.externalBrushTextures[key]; ok {
		r.mu.Unlock()
		return existing, r.externalBrushFullbright[key], r.externalBrushAnimations[key]
	}
	// Cache even empty results so we don't re-upload on failure.
	if textures == nil {
		textures = map[int32]*gpuWorldTexture{}
	}
	if fullbright == nil {
		fullbright = map[int32]*gpuWorldTexture{}
	}
	r.externalBrushTextures[key] = textures
	r.externalBrushFullbright[key] = fullbright
	r.externalBrushAnimations[key] = animations
	r.mu.Unlock()
	return textures, fullbright, animations
}

// brushEntityTextures returns the texture / fullbright / animations
// triplet appropriate for a brush entity. External-BSP entities get
// per-key maps uploaded on demand; inline submodels return (nil, nil, nil)
// and fall back to the world texture tables.
func (r *Renderer) brushEntityTextures(entity BrushEntity) (map[int32]*gpuWorldTexture, map[int32]*gpuWorldTexture, []*surfacepkg.SurfaceTexture) {
	if entity.ExternalKey == "" || entity.ExternalTree == nil {
		return nil, nil, nil
	}
	return r.ensureExternalBrushModelTextures(entity.ExternalKey, entity.ExternalTree)
}
