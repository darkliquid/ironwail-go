package renderer

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

func (r *Renderer) createWorldExternalSkyBindGroup(device *wgpu.Device, sampler *wgpu.Sampler, views [6]*wgpu.TextureView) (*wgpu.BindGroup, error) {
	if device == nil || sampler == nil || r.resources.WorldSkyExternalBindGroupLayout == nil {
		return nil, fmt.Errorf("missing external sky bind group resources")
	}
	for i, view := range views {
		if view == nil {
			return nil, fmt.Errorf("missing external sky texture view %d", i)
		}
	}
	start := time.Now()
	slog.Debug("external skybox bind group create begin", "subsystem", externalSkyboxLogSubsystem, "name", r.resources.WorldSkyExternalName)
	bindGroup, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "World External Sky BG",
		Layout: r.resources.WorldSkyExternalBindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Sampler: sampler},
			{Binding: 1, TextureView: views[0]},
			{Binding: 2, TextureView: views[1]},
			{Binding: 3, TextureView: views[2]},
			{Binding: 4, TextureView: views[3]},
			{Binding: 5, TextureView: views[4]},
			{Binding: 6, TextureView: views[5]},
		},
	})
	if err != nil {
		slog.Warn("external skybox bind group create failed", "subsystem", externalSkyboxLogSubsystem, "name", r.resources.WorldSkyExternalName, "error", err, "elapsed_ms", elapsedMilliseconds(start))
		return nil, err
	}
	slog.Debug("external skybox bind group create complete", "subsystem", externalSkyboxLogSubsystem, "name", r.resources.WorldSkyExternalName, "elapsed_ms", elapsedMilliseconds(start))
	return bindGroup, nil
}

func (r *Renderer) createWorldExternalSkyFaceTexture(device *wgpu.Device, queue *wgpu.Queue, label string, rgba []byte, width, height int) (*wgpu.Texture, *wgpu.TextureView, error) {
	if device == nil || queue == nil {
		return nil, nil, fmt.Errorf("invalid external sky texture upload inputs")
	}
	if width <= 0 || height <= 0 || len(rgba) != width*height*4 {
		return nil, nil, fmt.Errorf("invalid external sky texture size/data %dx%d (%d bytes)", width, height, len(rgba))
	}
	start := time.Now()
	slog.Debug("external sky texture create begin", "subsystem", externalSkyboxLogSubsystem, "name", r.resources.WorldSkyExternalName, "label", label, "width", width, "height", height, "rgba_bytes", len(rgba), "bytes_per_row", width*4)
	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         label,
		Size:          wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		slog.Warn("external sky texture create failed", "subsystem", externalSkyboxLogSubsystem, "name", r.resources.WorldSkyExternalName, "label", label, "width", width, "height", height, "rgba_bytes", len(rgba), "error", err, "elapsed_ms", elapsedMilliseconds(start))
		return nil, nil, fmt.Errorf("create external sky texture: %w", err)
	}
	slog.Debug("external sky texture create complete", "subsystem", externalSkyboxLogSubsystem, "name", r.resources.WorldSkyExternalName, "label", label, "elapsed_ms", elapsedMilliseconds(start))

	writeStart := time.Now()
	slog.Debug("external sky texture write begin via queue.WriteTexture", "subsystem", externalSkyboxLogSubsystem, "name", r.resources.WorldSkyExternalName, "label", label, "width", width, "height", height, "rgba_bytes", len(rgba))
	if err := queue.WriteTexture(&wgpu.ImageCopyTexture{
		Texture:  texture,
		MipLevel: 0,
		Origin:   wgpu.Origin3D{X: 0, Y: 0, Z: 0},
		Aspect:   gputypes.TextureAspectAll,
	}, rgba, &wgpu.ImageDataLayout{
		Offset:       0,
		BytesPerRow:  uint32(width * 4),
		RowsPerImage: uint32(height),
	}, &wgpu.Extent3D{
		Width:              uint32(width),
		Height:             uint32(height),
		DepthOrArrayLayers: 1,
	}); err != nil {
		texture.Release()
		return nil, nil, fmt.Errorf("write external sky texture: %w", err)
	}
	slog.Debug("external sky texture write complete", "subsystem", externalSkyboxLogSubsystem, "name", r.resources.WorldSkyExternalName, "label", label, "elapsed_ms", elapsedMilliseconds(writeStart))

	viewStart := time.Now()
	slog.Debug("external sky texture view create begin", "subsystem", externalSkyboxLogSubsystem, "name", r.resources.WorldSkyExternalName, "label", label)
	view, err := device.CreateTextureView(texture, &wgpu.TextureViewDescriptor{
		Label:           label + " View",
		Format:          gputypes.TextureFormatRGBA8Unorm,
		Dimension:       gputypes.TextureViewDimension2D,
		Aspect:          gputypes.TextureAspectAll,
		BaseMipLevel:    0,
		MipLevelCount:   1,
		BaseArrayLayer:  0,
		ArrayLayerCount: 1,
	})
	if err != nil {
		texture.Release()
		slog.Warn("external sky texture view create failed", "subsystem", externalSkyboxLogSubsystem, "name", r.resources.WorldSkyExternalName, "label", label, "error", err, "elapsed_ms", elapsedMilliseconds(viewStart))
		return nil, nil, fmt.Errorf("create external sky texture view: %w", err)
	}
	slog.Debug("external sky texture view create complete", "subsystem", externalSkyboxLogSubsystem, "name", r.resources.WorldSkyExternalName, "label", label, "elapsed_ms", elapsedMilliseconds(viewStart))
	return texture, view, nil
}

func elapsedMilliseconds(start time.Time) float64 {
	return float64(time.Since(start)) / float64(time.Millisecond)
}
