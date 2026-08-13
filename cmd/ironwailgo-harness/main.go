//go:build js && wasm

// wasm_harness_export.go — pollable engine exports for the Deno test harness
// (harness/wasm). The browser walkthrough build depends on window/rAF/DOM,
// which do not exist under Deno; this build instead exposes a callable ABI:
//
//	state_poll()  uint32   -> address of the shared state struct
//	input_inject(uint32)   -> write an {fwd,strafe,yaw,pitch,buttons[4]} frame
//	engine_advance(int64)  -> run one host frame (dt ns), returns state addr
//
// Only pure wasm-value exports; all data crosses through fixed flat structs
// in linear memory (offset/struct layouts are documented in harness/README.md).
package main

import (
	"fmt"
	"log/slog"
	"os"
	"unsafe"

	"github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/game"
	"github.com/darkliquid/ironwail-go/internal/input"
)

// Shared state layout (little-endian). Keep in sync with harness/README.md.
//
//	0   u32 flags          bit0 running, bit1 map, bit2 paused
//	4   u32 frame_count
//	8   f32 camera_origin[3]
//	20  f32 camera_angles[3]
//	32  f32 player_origin[3]
//	44  u32 map_entities
//	48  u32 pixel_width
//	52  u32 pixel_height
//	56  u32 pixel_stride
//	60  u32 pixel_valid       monotonic counter bumped per captured frame
//	64  u32 pixel_count       bytes valid in the pixel arena
//	68  u32 pixel_offset      byte offset of arena in linear memory
//	72  u32 error_code        0 = none; 1 = engine nil; 2 = frame error
//	76  u32 input_applied
//	80  u32 last_dt_us
//	84  u32 frame_flags       bit0 rendered
//	88  (total)
//
// Input layout: 8 x f32 { fwd, strafe, yaw, pitch, buttons.0..3 } (32 bytes).
const (
	stateSize     = 128 // 88-byte ABI struct + 40-byte debug scratch
	inputSize     = 32
	arenaBytes    = 448 * 352 * 4
	flagRunning   = 1 << 0
	flagMap       = 1 << 1
	flagPaused    = 1 << 2
	frameRendered = 1 << 0
)

var (
	stateMem      = make([]byte, stateSize)
	inputMem      = make([]byte, inputSize)
	pixelArena    = make([]byte, arenaBytes)
	g             *game.Game
	frameIn       [8]float32
	inputApplied  uint32
	lastDTMicros  uint32 = 16666
	pixValid      uint32
	pixFill       uint32
	lastFrameFlag uint32
)

// ---------------------------------------------------------------------------
// endian helpers (linear-memory flat structs)
// ---------------------------------------------------------------------------

func leu32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func rdu32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func lef32(b []byte, f float32) { leu32(b, *(*uint32)(unsafe.Pointer(&f))) }

func rdf32(b []byte) float32 { return *(*float32)(unsafe.Pointer(&b[0])) }

// ---------------------------------------------------------------------------
// exports
// ---------------------------------------------------------------------------

//go:wasmexport state_poll
func statePoll() uint32 {
	fillState()
	return uint32(uintptr(unsafe.Pointer(&stateMem[0])))
}

//go:wasmexport input_inject
func inputInject(addr uint32) {
	if addr == 0 {
		return
	}
	src := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(addr))), inputSize)
	for i := 0; i < 8; i++ {
		frameIn[i] = rdf32(src[4*i:])
	}
	inputApplied = 1
}

//go:wasmexport input_slot
func inputSlot() uint32 {
	// Returns a stable 32-byte arena the host writes an input frame into
	// before calling input_inject. This avoids requiring the host to guess
	// a linear-memory address.
	if inputMem == nil {
		inputMem = make([]byte, inputSize)
	}
	return uint32(uintptr(unsafe.Pointer(&inputMem[0])))
}

//go:wasmexport engine_advance
func engineAdvance(dtNS int64) uint32 {
	if g == nil {
		leu32(stateMem[72:], 1)
		return uint32(uintptr(unsafe.Pointer(&stateMem[0])))
	}
	if dtNS <= 0 {
		dtNS = 16_666_666
	}
	lastDTMicros = uint32(dtNS / 1000)

	if inputApplied != 0 {
		applyInput(g, &frameIn)
		inputApplied = 0
	}

	g.HarnessRuntimeStep(float64(dtNS) / 1e9)
	pixelsCapture() // best-effort: bump pixel_valid when a frame is rendered
	fillState()
	return uint32(uintptr(unsafe.Pointer(&stateMem[0])))
}
//go:wasmexport engine_set_paused
func engineSetPaused(paused uint32) {
	if g == nil {
		return
	}
	g.WasmSetPaused(paused != 0)
}

//go:wasmexport engine_step_frames
func engineStepFrames(n uint32) {
	if g == nil || n == 0 {
		return
	}
	g.WasmStepFrames(int(n))
}



//go:wasmexport boot_renderer
func bootRenderer() uint32 {
	// Boots the rAF-driven browser frame path with gogpu's App.Run initialized
	// in a goroutine. Installs the runtime update callback so each rAF tick ->
	// StepWasmFrame -> one full host frame (input/physics/client).
	// rc is written to stateMem[124] (0 ok, 1 nil renderer) since wasm return
	// values carry garbage high bits. Returns 0 always.
	if g == nil || g.Renderer == nil {
		leu32(stateMem[124:], 1)
		return 0
	}
	g.WasmSetPaused(true)
	renderedFrames = 0
	// Mirror installRuntimeRendererCallbacks: the rAF driver's StepWasmFrame
	// calls this to run the full runtime frame.
	g.Renderer.OnUpdate(func(dt float64) {
		g.DriveRuntimeFrame(dt)
	})
	go func() {
		if err := g.Renderer.Run(); err != nil {
			slog.Warn("WASM harness gogpu renderer loop exited", "error", err)
		}
	}()
	g.StartWasmRendererFrameLoop()
	leu32(stateMem[124:], 0)
	return 0
}

// renderedFrames counts StepWasmFrame-driven renderer updates (diagnostic).
var renderedFrames int64

//go:wasmexport debug_state
func debugState() uint32 {
	// Debug helper: writes diagnostic values into stateMem[88:] (a scratch
	// region past the 88-byte state struct) and returns the state address:
	//   [88] u32 flags       bit0 KeyDest==KeyGame, bit1 menu active,
	//                        bit2 client active, bit3 intermission
	//   [92] i32 cl.ViewAngles[1]*100
	//   [96] i32 harnessMouseDX
	//   [100] i32 harnessMouseDY
	if g == nil || g.Input == nil || g.Client == nil {
		return 0
	}
	flags := uint32(0)
	if g.Input.KeyDest() == input.KeyGame {
		flags |= 1
	}
	if g.Menu != nil && g.Menu.IsActive() {
		flags |= 2
	}
	if g.Client.State == client.StateActive {
		flags |= 4
	}
	if g.Client.Intermission != 0 {
		flags |= 8
	}
	leu32(stateMem[88:], flags)
	leu32(stateMem[92:], uint32(int32(g.Client.ViewAngles[1]*100)))
	leu32(stateMem[96:], uint32(int32(harnessMouseDX)))
	leu32(stateMem[100:], uint32(int32(harnessMouseDY)))
	if st := g.Input.State(); st != nil {
		leu32(stateMem[104:], uint32(int32(st.MouseDX)))
	}
	if g.Host != nil {
		leu32(stateMem[108:], uint32(int32(g.Host.CVar.FloatValue("m_yaw")*100)))
		leu32(stateMem[112:], uint32(int32(g.Host.CVar.FloatValue("m_pitch")*100)))
	}
	return uint32(uintptr(unsafe.Pointer(&stateMem[0])))
}

//go:wasmexport pixels_capture
func pixelsCapture() uint32 {
	// Best-effort readback of the rendered world texture into the pixel
	// arena. Returns 1 when a fresh frame was captured (pixel_valid bumped),
	// 0 when no WebGPU device/texture is present (headless harness runs).
	if g == nil || g.Renderer == nil {
		return 0
	}
	if rb, ok := g.Renderer.(interface {
		ReadbackWorldTexture() ([]byte, int, int, bool)
	}); ok {
		data, w, h, ok := rb.ReadbackWorldTexture()
		if !ok || w <= 0 || h <= 0 || len(data) == 0 {
			return 0
		}
		need := w * h * 4
		if need > len(pixelArena) {
			return 0
		}
		copy(pixelArena[:need], data[:need])
		pixValid++
		pixFill = uint32(need)
		lastPixelW, lastPixelH = w, h
		fillState()
		return 1
	}
	return 0
}

// lastPixelW/H mirror the last captured pixel dims for the state struct.
var lastPixelW, lastPixelH = 0, 0
// Mouse deltas drive yaw/pitch; key states drive forward/strafe/use/attack/
// jump through the same physical key code surface the engine's DOM backend
// uses (so existing binds pick the movement up).
func applyInput(g *game.Game, in *[8]float32) {
	if g.Input == nil {
		return
	}

	// Mouse (view orientation): store for the stub backend so State() sees
	// it through the normal path.
	harnessMouseDX = int32(in[2])
	harnessMouseDY = int32(in[3])
	g.Input.ApplyMouseDelta(int32(in[2]), int32(in[3]))

	// Movement axes -> WASD key states (Quake default binds:
	// w +forward, s +back, a +moveleft, d +moveright).
	axisHolds := []struct {
		key    int
		active bool
	}{
		{'w', in[0] > 0},
		{'s', in[0] < 0},
		{'a', in[1] < 0},
		{'d', in[1] > 0},
	}
	for _, h := range axisHolds {
		g.Input.HandleKeyEvent(input.KeyEvent{Key: h.key, Down: h.active})
	}

	// Buttons: e +moveup(use), left mouse +attack, SPACE +jump.
	btnBinds := []struct {
		key   int
		bit   uint
	}{
		{'e', 0},
		{input.KMouse1, 1},
		{input.KSpace, 2},
	}
	for _, b := range btnBinds {
		down := uint32(frameIn[4+int(b.bit)]) != 0
		g.Input.HandleKeyEvent(input.KeyEvent{Key: b.key, Down: down})
	}
}

// ---------------------------------------------------------------------------
// state fill
// ---------------------------------------------------------------------------

func fillState() {
	buf := stateMem
	for i := range buf {
		buf[i] = 0
	}
	leu32(buf[76:], inputApplied)
	leu32(buf[80:], lastDTMicros)
	leu32(buf[60:], pixValid)
	leu32(buf[48:], uint32(lastPixelW))
	leu32(buf[52:], uint32(lastPixelH))
	leu32(buf[56:], uint32(lastPixelW*4))
	leu32(buf[68:], uint32(uintptr(unsafe.Pointer(&pixelArena[0]))))
	leu32(buf[64:], pixFill)
	leu32(buf[84:], lastFrameFlag)

	flags := uint32(0)
	if g != nil {
		if g.Host != nil && !g.Host.IsAborted() {
			flags |= flagRunning
			leu32(buf[4:], uint32(g.Host.FrameCount()))
		}
		if g.Server != nil && g.Server.Name != "" && g.Server.Active {
			flags |= flagMap
			leu32(buf[44:], uint32(g.Server.NumEdicts))
		}
		o, a := g.WasmViewState()
		lef32(buf[8:], o[0])
		lef32(buf[12:], o[1])
		lef32(buf[16:], o[2])
		lef32(buf[20:], a[0])
		lef32(buf[24:], a[1])
		lef32(buf[28:], a[2])
	}
	if g != nil && g.WasmPaused() {
		flags |= flagPaused
	}
	leu32(buf[0:], flags)
}

// ---------------------------------------------------------------------------
// main — sync init + park; the host drives frames via engine_advance
// ---------------------------------------------------------------------------

func main() {
	fmt.Fprintf(os.Stdout, "Ironwail-Go harness wasm booting\n")
	g = game.New()
	fmt.Fprintf(os.Stdout, "Ironwail-Go harness engine ready (call mount_pak then engine_advance)\n")
	select {}
}

// bootOnce guards lazy engine initialization (idempotent across multiple
// mount_pak calls).
var bootOnce = false

// mountPak is exported so the Deno host can attach the Quake data pak
// synchronously from linear memory before frame-driven init:
//
//	The host instantiates the module (main() parks on select{}), then calls
//	mount_pak(ptr, len); mountPak lazily runs InitSubsystems + CmdMap now
//	that data is available, so engine_advance can drive frames afterwards.
//
//go:wasmexport mount_slot
func mountSlot() uint32 {
	// Return a pointer to a byte arena the host can write pak bytes into;
	// mount_pak(len) then parses+attaches it.
	if pakSlot == nil {
		pakSlot = make([]byte, 256) // placeholder; grows below via realloc
	}
	return uint32(uintptr(unsafe.Pointer(&pakSlot[0])))
}

// pakSlot is the linear-memory buffer the Deno host fills with pak bytes.
var pakSlot []byte

//go:wasmexport mount_resize
func mountResize(needed uint32) uint32 {
	if int(needed) > len(pakSlot) {
		pakSlot = make([]byte, int(needed))
	}
	return uint32(uintptr(unsafe.Pointer(&pakSlot[0])))
}

//go:wasmexport mount_pak
func mountPak() uint32 {
	if pakSlot == nil || len(pakSlot) == 0 || g == nil {
		return 1
	}
	pack, err := fs.LoadPackFromBytes("pak0.pak", pakSlot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mount_pak: parse failed: %v\n", err)
		return 2
	}
	// Bootstrap: InitSubsystems with the pak pre-mounted (idempotent).
	if !bootOnce {
		bootOnce = true
		if err := g.InitSubsystems(false, false, 4, "/", "id1", nil, pack); err != nil {
			fmt.Fprintf(os.Stderr, "harness init failed: %v\n", err)
			return 5
		}
		setHarnessBackend(g)
	}
	if g.Subs != nil && g.Subs.Files != nil && g.Subs.Files.FileExists("maps/start.bsp") {
		if err := g.Host.CmdMap("start", g.Subs); err != nil {
			fmt.Fprintf(os.Stderr, "harness cmd_map start failed: %v\n", err)
			return 3
		}
	}
	return 0
}

// (Removed async fetch path — Deno writes pak bytes via mount_pak; the
// js/wasm event loop cannot resolve promises outside exported calls, so
// blocking fetch is unreachable under the harness's pollable ABI.)

