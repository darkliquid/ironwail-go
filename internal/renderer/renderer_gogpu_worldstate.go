package renderer

import (
	"log/slog"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/pkg/types"
	"github.com/gogpu/wgpu"
)

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

// GetViewMatrix returns the currently cached view matrix.
// Thread-safe read.
func (r *Renderer) GetViewMatrix() types.Mat4 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.viewMatrices.View
}

// GetProjectionMatrix returns the currently cached projection matrix.
// Thread-safe read.
func (r *Renderer) GetProjectionMatrix() types.Mat4 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.viewMatrices.Projection
}

// GetViewProjectionMatrix returns the combined View × Projection matrix.
// This is the matrix typically used in vertex shaders for world-to-NDC transformation.
// Thread-safe read.
func (r *Renderer) GetViewProjectionMatrix() types.Mat4 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.viewMatrices.VP
}

// GetCameraState returns the current camera state (position and orientation).
// Thread-safe read. A copy is returned to prevent external modification.
func (r *Renderer) GetCameraState() CameraState {
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
	r.worldSkyExternalRequestID++
	requestID := r.worldSkyExternalRequestID
	if normalized == r.worldSkyExternalName {
		r.mu.Unlock()
		return
	}
	r.mu.Unlock()

	var (
		faces  [6]externalSkyboxFace
		loaded int
	)
	if normalized != "" && loadFile != nil {
		faces, loaded = loadExternalSkyboxFaces(normalized, loadFile)
	}
	renderMode := externalSkyboxRenderEmbedded
	if normalized != "" && loaded > 0 {
		renderMode = externalSkyboxRenderFaces
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if requestID != r.worldSkyExternalRequestID {
		return
	}

	r.destroyGoGPUExternalSkyboxResourcesLocked()
	r.worldSkyExternalFaces = [6]externalSkyboxFace{}
	r.worldSkyExternalLoaded = 0
	r.worldSkyExternalMode = externalSkyboxRenderEmbedded
	r.worldSkyExternalName = ""

	if normalized == "" || renderMode == externalSkyboxRenderEmbedded {
		return
	}

	r.worldSkyExternalFaces = faces
	r.worldSkyExternalLoaded = loaded
	r.worldSkyExternalMode = renderMode
	r.worldSkyExternalName = normalized

	if err := r.ensureGoGPUExternalSkyboxLocked(r.getWGPUDevice(), r.getWGPUQueue()); err != nil {
		slog.Debug("external gogpu skybox upload deferred", "name", normalized, "error", err)
	}
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
