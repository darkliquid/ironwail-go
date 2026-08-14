// renderer_stats.go — live per-frame pass counters for the walkthrough's
// renderer layer panel. Plan 22: the panel previously showed only camera state
// and capability flags; these counters report what the renderer actually
// executes each frame (mirroring the host_speeds RENDER bar breakdown).
package renderer

import (
	"sync/atomic"
)

// FramePassStats is a point-in-time snapshot of the renderer's per-frame
// counters, exposed to the wasm inspector bridge. All counters are monotonic;
// the bridge diffs them to derive per-frame activity.
type FramePassStats struct {
	WorldDraws      uint64 // world passes submitted (BSP world render)
	OverlayDraws    uint64 // overlay passes (HUD/menu/console composite)
	WorldUploads    uint64 // world geometry uploads
	SceneDraws      uint64 // scene target draws
	ParticlesDrawn  uint64 // particle batches issued
	AliasDraws      uint64 // alias model draw calls
	SpriteDraws     uint64 // sprite draw calls
	LightmapUploads uint64 // lightmap texture uploads
}

var (
	statWorldDraws      atomic.Uint64
	statOverlayDraws    atomic.Uint64
	statWorldUploads    atomic.Uint64
	statSceneDraws      atomic.Uint64
	statParticlesDrawn  atomic.Uint64
	statAliasDraws      atomic.Uint64
	statSpriteDraws     atomic.Uint64
	statLightmapUploads atomic.Uint64
)

func incrementWorldDraws()      { statWorldDraws.Add(1) }
func incrementOverlayDraws()    { statOverlayDraws.Add(1) }
func incrementWorldUploads()    { statWorldUploads.Add(1) }
func incrementSceneDraws()      { statSceneDraws.Add(1) }
func incrementParticlesDrawn()  { statParticlesDrawn.Add(1) }
func incrementAliasDraws()      { statAliasDraws.Add(1) }
func incrementSpriteDraws()     { statSpriteDraws.Add(1) }
func incrementLightmapUploads() { statLightmapUploads.Add(1) }

// WasmFramePassStatsSnapshot returns the current monotonic counter values.
// The walkthrough inspector diffs consecutive snapshots to show live pass
// activity per frame.
func WasmFramePassStatsSnapshot() FramePassStats {
	return FramePassStats{
		WorldDraws:      statWorldDraws.Load(),
		OverlayDraws:    statOverlayDraws.Load(),
		WorldUploads:    statWorldUploads.Load(),
		SceneDraws:      statSceneDraws.Load(),
		ParticlesDrawn:  statParticlesDrawn.Load(),
		AliasDraws:      statAliasDraws.Load(),
		SpriteDraws:     statSpriteDraws.Load(),
		LightmapUploads: statLightmapUploads.Load(),
	}
}
