// pass_dump.go provides intermediate render-pass attachment inspection,
// PNG readback dumpers, and runtime render pass isolation.
package renderer

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)



var (
	passDumpPrefix string
)

// ShouldDumpPasses reports whether pass dumping is requested for the active frame.
func ShouldDumpPasses() bool {
	if pkgCVars != nil {
		if cv := pkgCVars.Get(CvarRDumpPasses); cv != nil {
			return cv.Bool() || cv.Int > 0
		}
	}
	return false
}

// SetPassDumpPrefix sets an optional prefix for pass dump directories.
func SetPassDumpPrefix(prefix string) {
	passDumpPrefix = prefix
}

// PassDumpPrefix returns the current pass dump prefix.
func PassDumpPrefix() string {
	return passDumpPrefix
}

// halfToFloat32 converts a 16-bit IEEE 754 half-precision float to float32.
func halfToFloat32(h uint16) float32 {
	s := uint32(h>>15) & 0x00000001
	e := uint32(h>>10) & 0x0000001f
	m := uint32(h) & 0x000003ff

	switch e {
	case 0:
		if m == 0 {
			return math.Float32frombits(s << 31)
		}
		for (m & 0x00000400) == 0 {
			m <<= 1
			e--
		}
		e++
		m &= ^uint32(0x00000400)
	case 31:
		if m == 0 {
			return math.Float32frombits((s << 31) | 0x7f800000)
		}
		return math.Float32frombits((s << 31) | 0x7f800000 | (m << 13))
	}
	e = e + (127 - 15)
	m = m << 13
	return math.Float32frombits((s << 31) | (e << 23) | m)
}

// EncodeLinearizedDepthToGrayImage converts float32 depth values to a grayscale image,
// applying perspective linearization if near > 0 and far > near.
func EncodeLinearizedDepthToGrayImage(depth []float32, width, height int, near, far float32) *image.Gray {
	if len(depth) != width*height || width <= 0 || height <= 0 {
		return nil
	}
	img := image.NewGray(image.Rect(0, 0, width, height))
	linearize := near > 0 && far > near
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			d := depth[y*width+x]
			if math.IsNaN(float64(d)) || d < 0 {
				d = 0
			} else if math.IsInf(float64(d), 1) || d > 1 {
				d = 1
			}

			var linear float32
			if linearize {
				if d >= 1.0 {
					linear = 1.0
				} else if d <= 0.0 {
					linear = 0.0
				} else {
					z := (near * far) / (far - d*(far-near))
					linear = (z - near) / (far - near)
				}
			} else {
				linear = d
			}

			if linear < 0 {
				linear = 0
			} else if linear > 1 {
				linear = 1
			}
			img.SetGray(x, y, color.Gray{Y: uint8(linear * 255.0)})
		}
	}
	return img
}

// EncodeDepthToGrayImage converts normalized [0.0, 1.0] depth values directly to *image.Gray.
func EncodeDepthToGrayImage(depth []float32, width, height int) *image.Gray {
	return EncodeLinearizedDepthToGrayImage(depth, width, height, 0, 0)
}

// EncodeDepthBytesToGrayImage converts a float32 depth byte buffer (with potential row padding)
// to a linearized *image.Gray.
func EncodeDepthBytesToGrayImage(data []byte, width, height int, bytesPerRow int, near, far float32) *image.Gray {
	if width <= 0 || height <= 0 || len(data) == 0 {
		return nil
	}
	if bytesPerRow <= 0 {
		bytesPerRow = width * 4
	}
	depths := make([]float32, width*height)
	for y := 0; y < height; y++ {
		rowOffset := y * bytesPerRow
		for x := 0; x < width; x++ {
			idx := rowOffset + x*4
			if idx+4 > len(data) {
				break
			}
			bits := uint32(data[idx]) | (uint32(data[idx+1]) << 8) | (uint32(data[idx+2]) << 16) | (uint32(data[idx+3]) << 24)
			depths[y*width+x] = math.Float32frombits(bits)
		}
	}
	return EncodeLinearizedDepthToGrayImage(depths, width, height, near, far)
}

// EncodeOITRevealToGrayImage converts an R8Unorm byte buffer to a grayscale image.
func EncodeOITRevealToGrayImage(data []byte, width, height int, bytesPerRow int) *image.Gray {
	if width <= 0 || height <= 0 || len(data) == 0 {
		return nil
	}
	if bytesPerRow <= 0 {
		bytesPerRow = width
	}
	img := image.NewGray(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		srcOffset := y * bytesPerRow
		dstOffset := y * img.Stride
		for x := 0; x < width; x++ {
			if srcOffset+x < len(data) {
				img.Pix[dstOffset+x] = data[srcOffset+x]
			}
		}
	}
	return img
}

// EncodeRGBA8ToNRGBA converts an 8-bit RGBA or BGRA byte buffer to *image.NRGBA.
func EncodeRGBA8ToNRGBA(data []byte, width, height int, bytesPerRow int, isBGRA bool) *image.NRGBA {
	if width <= 0 || height <= 0 || len(data) == 0 {
		return nil
	}
	if bytesPerRow <= 0 {
		bytesPerRow = width * 4
	}
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		srcRow := y * bytesPerRow
		dstRow := y * img.Stride
		for x := 0; x < width; x++ {
			srcIdx := srcRow + x*4
			dstIdx := dstRow + x*4
			if srcIdx+4 > len(data) {
				break
			}
			if isBGRA {
				img.Pix[dstIdx+0] = data[srcIdx+2] // R
				img.Pix[dstIdx+1] = data[srcIdx+1] // G
				img.Pix[dstIdx+2] = data[srcIdx+0] // B
				img.Pix[dstIdx+3] = data[srcIdx+3] // A
			} else {
				img.Pix[dstIdx+0] = data[srcIdx+0] // R
				img.Pix[dstIdx+1] = data[srcIdx+1] // G
				img.Pix[dstIdx+2] = data[srcIdx+2] // B
				img.Pix[dstIdx+3] = data[srcIdx+3] // A
			}
		}
	}
	return img
}

// EncodeRGBA16FloatToNRGBA converts a 16-bit float RGBA buffer (such as OITAccum) to *image.NRGBA.
func EncodeRGBA16FloatToNRGBA(data []byte, width, height int, bytesPerRow int) *image.NRGBA {
	if width <= 0 || height <= 0 || len(data) == 0 {
		return nil
	}
	if bytesPerRow <= 0 {
		bytesPerRow = width * 8
	}
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		srcRow := y * bytesPerRow
		dstRow := y * img.Stride
		for x := 0; x < width; x++ {
			srcIdx := srcRow + x*8
			dstIdx := dstRow + x*4
			if srcIdx+8 > len(data) {
				break
			}
			rBits := uint16(data[srcIdx+0]) | (uint16(data[srcIdx+1]) << 8)
			gBits := uint16(data[srcIdx+2]) | (uint16(data[srcIdx+3]) << 8)
			bBits := uint16(data[srcIdx+4]) | (uint16(data[srcIdx+5]) << 8)
			aBits := uint16(data[srcIdx+6]) | (uint16(data[srcIdx+7]) << 8)

			rf := halfToFloat32(rBits)
			gf := halfToFloat32(gBits)
			bf := halfToFloat32(bBits)
			af := halfToFloat32(aBits)

			// Tone-map or clamp to [0, 1]
			clampF := func(v float32) uint8 {
				if math.IsNaN(float64(v)) || v <= 0 {
					return 0
				}
				if math.IsInf(float64(v), 1) || v >= 1.0 {
					return 255
				}
				return uint8(v * 255.0)
			}

			img.Pix[dstIdx+0] = clampF(rf)
			img.Pix[dstIdx+1] = clampF(gf)
			img.Pix[dstIdx+2] = clampF(bf)
			img.Pix[dstIdx+3] = clampF(af)
		}
	}
	return img
}

// saveImagePNG creates any missing parent directories and encodes an image to PNG.
func saveImagePNG(img image.Image, filePath string) error {
	if img == nil {
		return fmt.Errorf("nil image")
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("create image directory: %w", err)
	}
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("create image file: %w", err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("encode png: %w", err)
	}
	return nil
}

// readbackTextureStaging reads back arbitrary texture data into CPU memory using a staging buffer.
func readbackTextureStaging(
	device *wgpu.Device,
	queue *wgpu.Queue,
	texture *wgpu.Texture,
	width, height int,
	bytesPerPixel int,
	aspect gputypes.TextureAspect,
) ([]byte, int, error) {
	if device == nil || queue == nil || texture == nil || width <= 0 || height <= 0 {
		return nil, 0, fmt.Errorf("invalid readback parameters")
	}

	bytesPerRow := (width*bytesPerPixel + 255) &^ 255
	bufferSize := uint64(bytesPerRow * height)

	stagingBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "Pass Dump Staging Buffer",
		Size:             bufferSize,
		Usage:            gputypes.BufferUsageCopyDst | gputypes.BufferUsageMapRead,
		MappedAtCreation: false,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("create staging buffer: %w", err)
	}
	defer stagingBuffer.Release()

	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{
		Label: "Pass Dump Staging Encoder",
	})
	if err != nil {
		return nil, 0, fmt.Errorf("create encoder: %w", err)
	}

	encoder.CopyTextureToBuffer(texture, stagingBuffer, []wgpu.BufferTextureCopy{{
		BufferLayout: wgpu.ImageDataLayout{
			Offset:       0,
			BytesPerRow:  uint32(bytesPerRow),
			RowsPerImage: uint32(height),
		},
		TextureBase: wgpu.ImageCopyTexture{
			Aspect:   aspect,
			MipLevel: 0,
			Origin:   wgpu.Origin3D{X: 0, Y: 0, Z: 0},
		},
		Size: wgpu.Extent3D{
			Width:              uint32(width),
			Height:             uint32(height),
			DepthOrArrayLayers: 1,
		},
	}})

	cmdBuffer, err := encoder.Finish()
	if err != nil {
		return nil, 0, fmt.Errorf("finish command encoder: %w", err)
	}

	if _, err := queue.Submit(cmdBuffer); err != nil {
		return nil, 0, fmt.Errorf("submit staging copy: %w", err)
	}

	if err := stagingBuffer.Map(context.Background(), wgpu.MapModeRead, 0, bufferSize); err != nil {
		return nil, 0, fmt.Errorf("map staging buffer: %w", err)
	}
	defer func() { _ = stagingBuffer.Unmap() }()

	mappedRange, err := stagingBuffer.MappedRange(0, bufferSize)
	if err != nil {
		return nil, 0, fmt.Errorf("mapped range: %w", err)
	}
	defer mappedRange.Release()

	out := make([]byte, bufferSize)
	copy(out, mappedRange.Bytes())
	return out, bytesPerRow, nil
}

// PassDumpSession orchestrates capturing and saving intermediate pass attachments.
type PassDumpSession struct {
	DumpDir string
	Width   int
	Height  int
}

// NewPassDumpSession creates a new dump directory with a timestamp.
func NewPassDumpSession(baseDir, prefix string, width, height int) *PassDumpSession {
	timestamp := time.Now().Format("20060102_150405")
	dirName := fmt.Sprintf("pass_dump_%s", timestamp)
	if prefix != "" {
		dirName = fmt.Sprintf("%s_%s", prefix, timestamp)
	}
	if baseDir == "" {
		baseDir = "dumps"
	}
	dumpDir := filepath.Join(baseDir, dirName)
	return &PassDumpSession{
		DumpDir: dumpDir,
		Width:   width,
		Height:  height,
	}
}

// DumpStageRGBA8 saves an RGBA8 or BGRA8 texture to the dump session directory.
func (s *PassDumpSession) DumpStageRGBA8(filename string, data []byte, bytesPerRow int, isBGRA bool) error {
	img := EncodeRGBA8ToNRGBA(data, s.Width, s.Height, bytesPerRow, isBGRA)
	if img == nil {
		return fmt.Errorf("encode %s failed", filename)
	}
	return saveImagePNG(img, filepath.Join(s.DumpDir, filename))
}

// DumpStageRGBA16Float saves an RGBA16F texture (such as OITAccum) to PNG.
func (s *PassDumpSession) DumpStageRGBA16Float(filename string, data []byte, bytesPerRow int) error {
	img := EncodeRGBA16FloatToNRGBA(data, s.Width, s.Height, bytesPerRow)
	if img == nil {
		return fmt.Errorf("encode %s failed", filename)
	}
	return saveImagePNG(img, filepath.Join(s.DumpDir, filename))
}

// DumpStageReveal saves an R8Unorm reveal texture to PNG.
func (s *PassDumpSession) DumpStageReveal(filename string, data []byte, bytesPerRow int) error {
	img := EncodeOITRevealToGrayImage(data, s.Width, s.Height, bytesPerRow)
	if img == nil {
		return fmt.Errorf("encode %s failed", filename)
	}
	return saveImagePNG(img, filepath.Join(s.DumpDir, filename))
}

// DumpStageDepth saves a depth buffer to a linearized grayscale PNG.
func (s *PassDumpSession) DumpStageDepth(filename string, data []byte, bytesPerRow int, near, far float32) error {
	img := EncodeDepthBytesToGrayImage(data, s.Width, s.Height, bytesPerRow, near, far)
	if img == nil {
		return fmt.Errorf("encode %s failed", filename)
	}
	return saveImagePNG(img, filepath.Join(s.DumpDir, filename))
}

// CapturePassDumper records intermediate textures during frame execution.
type CapturePassDumper struct {
	session     *PassDumpSession
	device      *wgpu.Device
	queue       *wgpu.Queue
	r           *Renderer
	surfaceView *wgpu.TextureView
	isBGRA      bool
	completed   bool
}

// SetSurfaceView sets the current swapchain/surface texture view for postprocess/swapchain capture.
func (d *CapturePassDumper) SetSurfaceView(view *wgpu.TextureView) {
	if d != nil {
		d.surfaceView = view
	}
}

// BeginPassDump initializes a pass dumper for the current frame if requested.
func (r *Renderer) BeginPassDump() *CapturePassDumper {
	if !ShouldDumpPasses() {
		return nil
	}
	device := r.getWGPUDevice()
	queue := r.getWGPUQueue()
	if device == nil || queue == nil {
		return nil
	}
	width, height := r.Size()
	if width <= 0 || height <= 0 {
		return nil
	}
	format := r.sceneSurfaceFormat()
	isBGRA := format == gputypes.TextureFormatBGRA8Unorm || format == gputypes.TextureFormatBGRA8UnormSrgb
	session := NewPassDumpSession("dumps", passDumpPrefix, width, height)
	return &CapturePassDumper{
		session: session,
		device:  device,
		queue:   queue,
		r:       r,
		isBGRA:  isBGRA,
	}
}

// CaptureOpaqueAndDepth records "01_opaque_scene.png" and "02_scene_depth.png".
func (d *CapturePassDumper) CaptureOpaqueAndDepth() {
	if d == nil || d.r == nil {
		return
	}
	d.r.mu.RLock()
	sceneTex := d.r.resources.WorldRenderTexture
	depthTex := d.r.resources.WorldDepthTexture
	width := d.session.Width
	height := d.session.Height
	d.r.mu.RUnlock()

	if sceneTex != nil {
		if data, bpr, err := readbackTextureStaging(d.device, d.queue, sceneTex, width, height, 4, gputypes.TextureAspectAll); err == nil {
			_ = d.session.DumpStageRGBA8("01_opaque_scene.png", data, bpr, d.isBGRA)
		}
	}
	if depthTex != nil {
		if data, bpr, err := readbackTextureStaging(d.device, d.queue, depthTex, width, height, 4, gputypes.TextureAspectAll); err == nil {
			_ = d.session.DumpStageDepth("02_scene_depth.png", data, bpr, 4.0, 4096.0)
		}
	}
}

// CaptureOITAccumAndReveal records "03_oit_accum_rgb.png" and "04_oit_reveal.png".
func (d *CapturePassDumper) CaptureOITAccumAndReveal() {
	if d == nil || d.r == nil {
		return
	}
	d.r.mu.RLock()
	accumTex := d.r.resources.OITAccumTexture
	revealTex := d.r.resources.OITRevealTexture
	width := d.session.Width
	height := d.session.Height
	d.r.mu.RUnlock()

	if accumTex != nil {
		if data, bpr, err := readbackTextureStaging(d.device, d.queue, accumTex, width, height, 8, gputypes.TextureAspectAll); err == nil {
			_ = d.session.DumpStageRGBA16Float("03_oit_accum_rgb.png", data, bpr)
		}
	}
	if revealTex != nil {
		if data, bpr, err := readbackTextureStaging(d.device, d.queue, revealTex, width, height, 1, gputypes.TextureAspectAll); err == nil {
			_ = d.session.DumpStageReveal("04_oit_reveal.png", data, bpr)
		}
	}
}

// CaptureResolvedScene records "05_resolved_scene.png".
func (d *CapturePassDumper) CaptureResolvedScene() {
	if d == nil || d.r == nil {
		return
	}
	d.r.mu.RLock()
	sceneTex := d.r.resources.WorldRenderTexture
	width := d.session.Width
	height := d.session.Height
	d.r.mu.RUnlock()

	if sceneTex != nil {
		if data, bpr, err := readbackTextureStaging(d.device, d.queue, sceneTex, width, height, 4, gputypes.TextureAspectAll); err == nil {
			_ = d.session.DumpStageRGBA8("05_resolved_scene.png", data, bpr, d.isBGRA)
		}
	}
}

// CaptureViewModelScene records "06_viewmodel_scene.png".
func (d *CapturePassDumper) CaptureViewModelScene() {
	if d == nil || d.r == nil {
		return
	}
	d.r.mu.RLock()
	sceneTex := d.r.resources.WorldRenderTexture
	width := d.session.Width
	height := d.session.Height
	d.r.mu.RUnlock()

	if sceneTex != nil {
		if data, bpr, err := readbackTextureStaging(d.device, d.queue, sceneTex, width, height, 4, gputypes.TextureAspectAll); err == nil {
			_ = d.session.DumpStageRGBA8("06_viewmodel_scene.png", data, bpr, d.isBGRA)
		}
	}
}

func (d *CapturePassDumper) readbackSurfaceOrWorldTexture(views ...*wgpu.TextureView) ([]byte, int, bool) {
	if d == nil || d.r == nil {
		return nil, 0, false
	}
	var view *wgpu.TextureView
	if len(views) > 0 && views[0] != nil {
		view = views[0]
	} else if d.surfaceView != nil {
		view = d.surfaceView
	}

	width := d.session.Width
	height := d.session.Height

	if view != nil && d.device != nil && d.queue != nil {
		tex := view.Texture()
		if tex != nil {
			data, bpr, err := readbackTextureStaging(d.device, d.queue, tex, width, height, 4, gputypes.TextureAspectAll)
			if err == nil && len(data) > 0 {
				return data, bpr, true
			}
		}
	}

	// Fallback to WorldRenderTexture if swapchain view texture is unavailable
	data, w, _, ok := d.r.ReadbackWorldTexture()
	if ok && len(data) > 0 {
		bytesPerRow := (w*4 + 255) &^ 255
		return data, bytesPerRow, true
	}
	return nil, 0, false
}

// CapturePostprocessed records "07_postprocessed.png".
func (d *CapturePassDumper) CapturePostprocessed(views ...*wgpu.TextureView) {
	if d == nil || d.r == nil {
		return
	}
	if data, bpr, ok := d.readbackSurfaceOrWorldTexture(views...); ok {
		_ = d.session.DumpStageRGBA8("07_postprocessed.png", data, bpr, d.isBGRA)
	}
}

// CaptureFinalSwapchain records "08_final_swapchain.png" and completes the dump.
func (d *CapturePassDumper) CaptureFinalSwapchain(views ...*wgpu.TextureView) {
	if d == nil || d.completed || d.r == nil {
		return
	}
	d.completed = true
	if data, bpr, ok := d.readbackSurfaceOrWorldTexture(views...); ok {
		_ = d.session.DumpStageRGBA8("08_final_swapchain.png", data, bpr, d.isBGRA)
	}
	slog.Info("Pass dump completed", "dir", d.session.DumpDir)
	if pkgCVars != nil {
		pkgCVars.Set(CvarRDumpPasses, "0")
	}
}

