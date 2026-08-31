// frame_graph.go implements the single-submit frame command orchestrator for
// the GoGPU/WebGPU backend.
//
// # Why a frame graph?
//
// Historically every render stage (world, entities, particles, post-process,
// overlay) created its own CommandEncoder, recorded a render pass, called
// Finish, and submitted to the GPU queue independently. That meant up to ~17
// queue submissions per frame. Besides being hard to extend (adding a pass
// meant juggling yet another submit), it is also the least efficient shape for
// WebGPU: every Submit crosses the WASM/JS boundary in the browser and forces
// the driver to re-validate state.
//
// WebGPU is explicitly designed around the opposite model: record ALL work for
// a frame into command buffers, then submit ONCE. Passes recorded into a single
// command encoder execute in recording order, so as long as each pass writes
// the correct LoadOp (Clear for the first pass that touches a target, Load for
// every later pass that accumulates onto it) the visual result is identical to
// many separate submits.
//
// # What this file provides
//
//   - FrameGraph: owns ONE CommandEncoder for the whole frame. Stages borrow
//     it to record their render/compute passes, then Finish+Submit happen a
//     single time at the end of RenderFrame.
//   - Pass: the interface every render stage implements. A Pass is pure
//     "record commands" logic; it never touches the queue and never submits.
//     Because a Pass only reads renderer state and writes into the shared
//     encoder, it can be run in isolation (build a graph with just that pass)
//     or reordered/extended without touching any submission code.
//
// # Buffer-upload timing (important subtlety)
//
// Stages still call queue.WriteBuffer / CreateBuffer(mapped) to upload uniforms
// and vertex data. That is safe to do DURING recording because of how gogpu
// orders work:
//   - Native: WriteBuffer is staged into a pending-writes command buffer that
//     is flushed to the queue BEFORE the user's command buffer at Submit time
//     (see queue_native.go). So the upload always lands before the draws that
//     consume it, regardless of when during the frame we called WriteBuffer.
//   - Browser/WASM: WriteBuffer is an immediate queue.writeBuffer JS call, which
//     the spec guarantees executes before any later queue.submit.
//
// So "record now, upload whenever, submit once at the end" is correct on both
// targets. The only hard rule is that no stage may submit on its own.
//
// # What is deliberately NOT in the frame graph
//
// ReadbackWorldTexture (screenshot / test harness) performs a CopyTextureToBuffer
// followed by a blocking buffer Map. That is a synchronous readback, not frame
// rendering, so it keeps its own private encoder+submit and is untouched.
package renderer

import (
	"fmt"
	"log/slog"

	"github.com/gogpu/wgpu"
)

// Pass is a single, self-contained render stage.
//
// A Pass records GPU commands into the frame's shared command encoder via the
// FrameContext handed to Record. It must NOT create its own command encoder and
// must NOT call queue.Submit; the FrameGraph owns submission. This is the key
// discipline that lets stages be developed, tested, and run in isolation while
// still composing into a single submit.
//
// Record may return an error to abort the pass; the graph logs it and continues
// with the next pass so one broken stage does not blank the whole frame.
type Pass interface {
	// Name identifies the pass in logs and host_speeds timing output.
	Name() string

	// Record encodes this stage's GPU commands into the frame's shared encoder.
	Record(fc *FrameContext) error
}

// FrameContext is the per-frame recording context handed to every Pass.
//
// It bundles the three things a pass needs to record work:
//   - the shared command encoder (to begin render/compute passes),
//   - the device and queue (for resource uploads such as WriteBuffer),
//   - the active color/depth target views (so passes attach to the right surface).
//
// Keeping these in one struct means a Pass never reaches back into DrawContext
// global state for the encoder, which is what makes passes independently
// runnable and easy to reason about.
type FrameContext struct {
	// Encoder is the frame's single shared command encoder. Begin all render
	// and compute passes from this. Do NOT call Finish on it; the graph does.
	Encoder *wgpu.CommandEncoder

	// Device and Queue are exposed so a pass can upload uniforms/vertices with
	// queue.WriteBuffer or create mapped staging buffers during recording.
	Device *wgpu.Device
	Queue  *wgpu.Queue

	// ColorView is the active color render target for this frame. It is the
	// swapchain surface view normally, or the offscreen scene texture when the
	// water-warp / translucent-liquid scene render target is active.
	ColorView *wgpu.TextureView

	// DepthView is the shared depth-stencil target used by all 3D passes.
	DepthView *wgpu.TextureView

	// Renderer is the owning renderer, for read access to pipelines, bind
	// groups, buffers, and camera state. Passes read from it but do not mutate
	// submission state.
	Renderer *Renderer
}

// BeginRenderPass is a small convenience wrapper that begins a render pass on
// the shared encoder and wraps any error with the pass name for diagnostics.
func (fc *FrameContext) BeginRenderPass(passName string, desc *wgpu.RenderPassDescriptor) (*wgpu.RenderPassEncoder, error) {
	if fc == nil || fc.Encoder == nil {
		return nil, fmt.Errorf("frame context has no command encoder")
	}
	rp, err := fc.Encoder.BeginRenderPass(desc)
	if err != nil {
		return nil, fmt.Errorf("pass %q: begin render pass: %w", passName, err)
	}
	return rp, nil
}

// FrameGraph owns the frame's command encoder and runs an ordered list of
// passes into it, finishing and submitting exactly once.
//
// Lifecycle per frame:
//
//	g := NewFrameGraph(device, queue)
//	g.Begin(colorView, depthView)        // creates the shared encoder
//	g.Add(worldPass, entitiesPass, ...)  // plan stage
//	err := g.Execute()                   // record all + Finish + one Submit
//
// The graph is single-use per frame; create a fresh one each RenderFrame.
type FrameGraph struct {
	device *wgpu.Device
	queue  *wgpu.Queue

	encoder  *wgpu.CommandEncoder
	colorVw  *wgpu.TextureView
	depthVw  *wgpu.TextureView
	passes   []Pass
	renderer *Renderer

	began bool
}

// NewFrameGraph creates a graph bound to a device/queue for one frame.
func NewFrameGraph(renderer *Renderer, device *wgpu.Device, queue *wgpu.Queue) *FrameGraph {
	return &FrameGraph{
		renderer: renderer,
		device:   device,
		queue:    queue,
	}
}

// Begin creates the shared command encoder and fixes the frame's render targets.
// It must be called before Add/Execute. colorView is the frame's color target
// (swapchain or offscreen scene target); depthView is the shared depth buffer.
func (g *FrameGraph) Begin(colorView, depthView *wgpu.TextureView) error {
	if g.device == nil || g.queue == nil {
		return fmt.Errorf("frame graph: nil device or queue")
	}
	if colorView == nil {
		return fmt.Errorf("frame graph: nil color target view")
	}
	enc, err := g.device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{
		Label: "Frame Command Encoder",
	})
	if err != nil {
		return fmt.Errorf("frame graph: create command encoder: %w", err)
	}
	g.encoder = enc
	g.colorVw = colorView
	g.depthVw = depthView
	g.began = true
	return nil
}

// Add appends passes to the frame in execution order. Nil passes are ignored so
// callers can conditionally build a slice without defensive append checks.
func (g *FrameGraph) Add(passes ...Pass) {
	for _, p := range passes {
		if p != nil {
			g.passes = append(g.passes, p)
		}
	}
}

// Context returns the FrameContext that passes use to record. Valid after Begin.
func (g *FrameGraph) Context() *FrameContext {
	if !g.began {
		return nil
	}
	return &FrameContext{
		Encoder:   g.encoder,
		Device:    g.device,
		Queue:     g.queue,
		ColorView: g.colorVw,
		DepthView: g.depthVw,
		Renderer:  g.renderer,
	}
}

// Execute records every registered pass in order into the shared encoder, then
// finishes the encoder and submits the resulting command buffer exactly once.
//
// A pass that returns an error is logged and skipped; remaining passes still
// record so a single failing stage degrades gracefully instead of blanking the
// frame. Execute is idempotent-safe to call once; calling it twice submits an
// empty second buffer only if no passes recorded, which callers avoid by
// building a fresh graph each frame.
func (g *FrameGraph) Execute() error {
	if !g.began {
		return fmt.Errorf("frame graph: Execute called before Begin")
	}
	fc := g.Context()

	// Record phase: each pass appends its commands to the shared encoder.
	for _, p := range g.passes {
		if err := p.Record(fc); err != nil {
			slog.Warn("frame graph: pass failed", "pass", p.Name(), "error", err)
		}
	}

	// Finish the shared encoder exactly once, producing one command buffer.
	cmdBuffer, err := g.encoder.Finish()
	if err != nil {
		return fmt.Errorf("frame graph: finish command encoding: %w", err)
	}

	// Single submission for the entire frame. On WASM this is the sole
	// queue.submit JS call for the frame, which is the whole point of the
	// refactor.
	if _, err := g.queue.Submit(cmdBuffer); err != nil {
		return fmt.Errorf("frame graph: submit: %w", err)
	}
	return nil
}

// --- DrawContext integration -------------------------------------------------
//
// The helpers below are the seam between the existing per-stage `*HAL`
// functions and the frame graph. Each stage used to do:
//
//	encoder, _ := device.CreateCommandEncoder(...)
//	pass, _  := encoder.BeginRenderPass(...)
//	... record draws ...
//	pass.End()
//	cmd, _  := encoder.Finish()
//	queue.Submit(cmd)
//
// In frame-graph mode the stage instead does:
//
//	encoder := dc.frameEncoder(device)   // shared encoder while graph is live
//	pass, _  := encoder.BeginRenderPass(...)
//	... record draws ...
//	pass.End()
//	dc.frameSubmit(queue)                // no-op while graph is live
//
// When no graph is live (e.g. a stage invoked standalone in a test or a debug
// path), frameEncoder falls back to creating a private encoder and
// frameSubmit performs a real Finish+Submit, so every stage still works in
// complete isolation exactly as before. This is what makes each pass truly
// independent: graph mode changes WHERE commands go, not WHETHER they run.

// beginFrameGraph creates the frame's shared command encoder and fixes the
// frame's color/depth targets. Called once at the top of RenderFrame.
func (dc *DrawContext) beginFrameGraph() error {
	device := dc.renderer.getWGPUDevice()
	queue := dc.renderer.getWGPUQueue()
	if device == nil || queue == nil {
		return fmt.Errorf("begin frame graph: device or queue unavailable")
	}
	colorView := dc.currentWGPURenderTargetView()
	if colorView == nil {
		return fmt.Errorf("begin frame graph: no color target view")
	}
	depthView := dc.renderer.resources.WorldDepthTextureView

	g := NewFrameGraph(dc.renderer, device, queue)
	if err := g.Begin(colorView, depthView); err != nil {
		return err
	}
	dc.frameGraph = g
	return nil
}

// endFrameGraph finishes the shared encoder and submits the whole frame once.
// It always clears dc.frameGraph so a failure cannot leak a live graph into
// the next frame. Called once at the bottom of RenderFrame.
func (dc *DrawContext) endFrameGraph() {
	g := dc.frameGraph
	dc.frameGraph = nil
	if g == nil {
		return
	}
	if err := g.Execute(); err != nil {
		slog.Warn("frame graph execute failed", "error", err)
	}
}

// frameGraphActive reports whether a shared frame encoder is currently live.
func (dc *DrawContext) frameGraphActive() bool {
	return dc != nil && dc.frameGraph != nil
}

// frameEncoder returns the encoder a stage should record into this frame.
//
// When the frame graph is live it returns the shared encoder (and the second
// return value is false, signalling "do not finish/submit this yourself").
// When no graph is live it creates and returns a private encoder owned by the
// caller (second return value true), preserving the stage's standalone behavior.
func (dc *DrawContext) frameEncoder(device *wgpu.Device, label string) (*wgpu.CommandEncoder, bool, error) {
	if dc.frameGraphActive() {
		return dc.frameGraph.encoder, false, nil
	}
	enc, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: label})
	if err != nil {
		return nil, false, err
	}
	return enc, true, nil
}

// frameSubmit completes a stage's work.
//
// In frame-graph mode this is intentionally a no-op: the stage's commands are
// already in the shared encoder and will be submitted by endFrameGraph along
// with every other stage. In standalone mode (owned == true) it finishes the
// private encoder and submits it, reproducing the stage's original behavior.
func (dc *DrawContext) frameSubmit(queue *wgpu.Queue, encoder *wgpu.CommandEncoder, owned bool, label string) {
	if !owned {
		// Shared encoder: the graph submits at end of frame.
		return
	}
	cmdBuffer, err := encoder.Finish()
	if err != nil {
		slog.Warn("frameSubmit: failed to finish encoding", "label", label, "error", err)
		return
	}
	if _, err := queue.Submit(cmdBuffer); err != nil {
		slog.Warn("frameSubmit: failed to submit", "label", label, "error", err)
	}
}

// frameReleaseBuffers disposes of transient per-pass GPU buffers that were
// bound during recording.
//
// In standalone mode the stage already submitted, so the buffers can be
// released immediately, matching the original code. In frame-graph mode the
// shared command buffer has NOT been submitted yet, so the GPU may still read
// these buffers once endFrameGraph submits. Releasing them now would be a
// use-after-free on the GPU. We therefore route them through the renderer's
// deferred-release queue, which holds them for 2 frames — long enough for the
// deferred submit to complete.
func (dc *DrawContext) frameReleaseBuffers(buffers []*wgpu.Buffer) {
	if len(buffers) == 0 {
		return
	}
	if !dc.frameGraphActive() {
		for _, b := range buffers {
			if b != nil {
				b.Release()
			}
		}
		return
	}
	r := dc.renderer
	r.mu.Lock()
	for _, b := range buffers {
		if b == nil {
			continue
		}
		buf := b
		r.enqueueReleaseLocked(func() { buf.Release() })
	}
	r.mu.Unlock()
}
