//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	"fmt"
	"log/slog"
	"math"
	"strings"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu/hal"
	"github.com/ironwail/ironwail-go/internal/cvar"
)

const (
	aliasShadowSegments = 16
	aliasShadowAlpha    = 0.5
	aliasShadowLift     = 0.1
	aliasShadowMinSize  = 8.0
	aliasShadowMaxSize  = 48.0
)

func (dc *DrawContext) renderAliasShadowsHAL(entities []AliasModelEntity) {
	if dc == nil || dc.renderer == nil || len(entities) == 0 {
		return
	}
	if cvar.FloatValue(CvarRShadows) <= 0 {
		return
	}

	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
	if device == nil || queue == nil {
		return
	}

	excludedModels := parseAliasShadowExclusionsGO(cvar.StringValue(CvarRNoshadowList))
	r := dc.renderer
	r.mu.Lock()
	if err := r.ensureAliasResourcesLocked(device); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to initialize alias resources for shadows", "error", err)
		return
	}
	if err := r.ensureAliasShadowSkinLocked(device, queue); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to initialize alias shadow skin", "error", err)
		return
	}
	shadowSkin := r.aliasShadowSkin
	r.mu.Unlock()
	if shadowSkin == nil {
		return
	}

	draws := make([]gpuAliasShadowDraw, 0, len(entities))
	for _, entity := range entities {
		modelID := strings.ToLower(entity.ModelID)
		if _, skip := excludedModels[modelID]; skip {
			continue
		}
		alpha, visible := visibleEntityAlpha(entity.Alpha)
		if !visible || !isFullyOpaqueAlpha(alpha) {
			continue
		}
		vertices := buildAliasShadowVertices(entity)
		if len(vertices) == 0 {
			continue
		}
		draws = append(draws, gpuAliasShadowDraw{vertices: vertices})
	}
	if len(draws) == 0 {
		return
	}

	dc.renderAliasShadowDrawsHAL(draws, shadowSkin)
}

type gpuAliasShadowDraw struct {
	vertices []WorldVertex
}

func (dc *DrawContext) renderAliasShadowDrawsHAL(draws []gpuAliasShadowDraw, shadowSkin *gpuAliasSkin) {
	if len(draws) == 0 || shadowSkin == nil || shadowSkin.bindGroup == nil {
		return
	}
	device := dc.renderer.getHALDevice()
	queue := dc.renderer.getHALQueue()
	if device == nil || queue == nil {
		return
	}
	textureView := dc.currentHALRenderTargetView()
	if textureView == nil {
		return
	}

	maxVertexBytes := uint64(44)
	for _, draw := range draws {
		size := uint64(len(draw.vertices) * 44)
		if size > maxVertexBytes {
			maxVertexBytes = size
		}
	}

	r := dc.renderer
	r.mu.Lock()
	if err := r.ensureAliasScratchBufferLocked(device, maxVertexBytes); err != nil {
		r.mu.Unlock()
		slog.Warn("failed to ensure alias scratch buffer for shadows", "error", err)
		return
	}
	pipeline := r.aliasPipeline
	uniformBuffer := r.aliasUniformBuffer
	uniformBindGroup := r.aliasUniformBindGroup
	scratchBuffer := r.aliasScratchBuffer
	depthView := r.worldDepthTextureView
	r.mu.Unlock()
	if pipeline == nil || uniformBuffer == nil || uniformBindGroup == nil || scratchBuffer == nil {
		return
	}

	encoder, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "Alias Shadow Render Encoder"})
	if err != nil {
		slog.Warn("failed to create alias shadow encoder", "error", err)
		return
	}
	if err := encoder.BeginEncoding("alias-shadow"); err != nil {
		slog.Warn("failed to begin alias shadow encoding", "error", err)
		return
	}

	renderPass := encoder.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "Alias Shadow Render Pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: aliasDepthAttachmentForView(depthView),
	})
	renderPass.SetPipeline(pipeline)
	width, height := r.Size()
	if width > 0 && height > 0 {
		renderPass.SetViewport(0, 0, float32(width), float32(height), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(width), uint32(height))
	}
	renderPass.SetVertexBuffer(0, scratchBuffer, 0)
	renderPass.SetBindGroup(0, uniformBindGroup, nil)
	renderPass.SetBindGroup(1, shadowSkin.bindGroup, nil)

	vpMatrix := r.GetViewProjectionMatrix()
	for _, draw := range draws {
		if len(draw.vertices) == 0 {
			continue
		}
		if err := queue.WriteBuffer(uniformBuffer, 0, aliasUniformBytes(vpMatrix, aliasShadowAlpha)); err != nil {
			slog.Warn("failed to update alias shadow uniform buffer", "error", err)
			continue
		}
		if err := queue.WriteBuffer(scratchBuffer, 0, aliasVertexBytes(draw.vertices)); err != nil {
			slog.Warn("failed to upload alias shadow vertices", "error", err)
			continue
		}
		renderPass.Draw(uint32(len(draw.vertices)), 1, 0, 0)
	}

	renderPass.End()
	cmdBuffer, err := encoder.EndEncoding()
	if err != nil {
		slog.Warn("failed to finish alias shadow encoding", "error", err)
		return
	}
	if err := queue.Submit([]hal.CommandBuffer{cmdBuffer}, nil, 0); err != nil {
		slog.Warn("failed to submit alias shadow commands", "error", err)
	}
}

func (r *Renderer) ensureAliasShadowSkinLocked(device hal.Device, queue hal.Queue) error {
	if r.aliasShadowSkin != nil {
		return nil
	}
	if r.aliasTextureBindGroupLayout == nil || r.aliasSampler == nil {
		return fmt.Errorf("alias texture bind group layout not ready")
	}
	skin, err := r.createAliasSkinLocked(device, queue, 1, 1, []byte{0})
	if err != nil {
		return err
	}
	r.aliasShadowSkin = &skin
	return nil
}

func buildAliasShadowVertices(entity AliasModelEntity) []WorldVertex {
	if entity.Model == nil || entity.Model.AliasHeader == nil {
		return nil
	}

	modelScale := entity.Scale
	if modelScale == 0 {
		modelScale = 1
	}
	mins := entity.Model.Mins
	maxs := entity.Model.Maxs
	spanX := (maxs[0] - mins[0]) * modelScale
	spanY := (maxs[1] - mins[1]) * modelScale
	shadowRadius := 0.5 * float32(math.Max(float64(spanX), float64(spanY)))
	if shadowRadius < aliasShadowMinSize {
		shadowRadius = aliasShadowMinSize
	}
	if shadowRadius > aliasShadowMaxSize {
		shadowRadius = aliasShadowMaxSize
	}

	shadowZ := entity.Origin[2] + mins[2]*modelScale + aliasShadowLift
	center := WorldVertex{
		Position:      [3]float32{entity.Origin[0], entity.Origin[1], shadowZ},
		TexCoord:      [2]float32{0.5, 0.5},
		LightmapCoord: [2]float32{},
		Normal:        [3]float32{0, 0, 1},
	}
	vertices := make([]WorldVertex, 0, aliasShadowSegments*3)
	for i := 0; i < aliasShadowSegments; i++ {
		a0 := float32(i) * 2 * float32(math.Pi) / aliasShadowSegments
		a1 := float32(i+1) * 2 * float32(math.Pi) / aliasShadowSegments
		p0 := WorldVertex{
			Position: [3]float32{
				entity.Origin[0] + float32(math.Cos(float64(a0)))*shadowRadius,
				entity.Origin[1] + float32(math.Sin(float64(a0)))*shadowRadius,
				shadowZ,
			},
			TexCoord:      [2]float32{0, 0},
			LightmapCoord: [2]float32{},
			Normal:        [3]float32{0, 0, 1},
		}
		p1 := WorldVertex{
			Position: [3]float32{
				entity.Origin[0] + float32(math.Cos(float64(a1)))*shadowRadius,
				entity.Origin[1] + float32(math.Sin(float64(a1)))*shadowRadius,
				shadowZ,
			},
			TexCoord:      [2]float32{1, 1},
			LightmapCoord: [2]float32{},
			Normal:        [3]float32{0, 0, 1},
		}
		vertices = append(vertices, center, p0, p1)
	}
	return vertices
}

func parseAliasShadowExclusionsGO(value string) map[string]struct{} {
	fields := strings.Fields(strings.ToLower(value))
	if len(fields) == 0 {
		return nil
	}
	exclusions := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		exclusions[field] = struct{}{}
	}
	return exclusions
}
