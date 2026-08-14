// pass_mask.go provides bitmask and name-based controls to selectively toggle
// individual GPU rendering passes (Sky, Opaque BSP world, Lightmaps, Brush
// entities, Alias entities, Viewmodel, Translucent liquids, Particles/Trails,
// Decals, 2D overlays) at runtime.
package renderer

import "sync/atomic"

// RenderPassFlags is a bitmask controlling individual rendering passes.
type RenderPassFlags uint32

const (
	PassSky                RenderPassFlags = 1 << 0 // 1: Sky rendering
	PassWorldOpaque        RenderPassFlags = 1 << 1 // 2: 3D BSP world (opaque, alpha-test, opaque liquids)
	PassLightmaps          RenderPassFlags = 1 << 2 // 4: Lightmaps / surface lighting modulation
	PassBrushEntities      RenderPassFlags = 1 << 3 // 8: Inline BSP submodels / brush entities
	PassAliasEntities      RenderPassFlags = 1 << 4 // 16: Alias MDL models (players, monsters, items)
	PassViewModel          RenderPassFlags = 1 << 5 // 32: First-person viewmodel / weapon
	PassTranslucentLiquids RenderPassFlags = 1 << 6 // 64: Translucent liquids (water, slime, lava)
	PassParticles          RenderPassFlags = 1 << 7 // 128: Particles and rocket trails
	PassDecals             RenderPassFlags = 1 << 8 // 256: Decal marks on geometry
	Pass2DOverlay          RenderPassFlags = 1 << 9 // 512: 2D HUD, console, menu, and text

	PassAll RenderPassFlags = (1 << 10) - 1
)

var globalPassFlags atomic.Uint32

func init() {
	globalPassFlags.Store(uint32(PassAll))
}

// GetGlobalPassFlags returns the active global pass mask.
func GetGlobalPassFlags() RenderPassFlags {
	return RenderPassFlags(globalPassFlags.Load())
}

// SetGlobalPassFlags sets the entire pass bitmask.
func SetGlobalPassFlags(flags RenderPassFlags) {
	globalPassFlags.Store(uint32(flags))
}

// SetGlobalPassEnabled enables or disables a single pass.
func SetGlobalPassEnabled(pass RenderPassFlags, enabled bool) {
	for {
		cur := globalPassFlags.Load()
		var next uint32
		if enabled {
			next = cur | uint32(pass)
		} else {
			next = cur &^ uint32(pass)
		}
		if globalPassFlags.CompareAndSwap(cur, next) {
			break
		}
	}
}

// IsGlobalPassEnabled checks if the given pass is enabled.
func IsGlobalPassEnabled(pass RenderPassFlags) bool {
	return (globalPassFlags.Load() & uint32(pass)) != 0
}

// PassNameToFlag maps string names to pass flags.
var PassNameToFlag = map[string]RenderPassFlags{
	"sky":       PassSky,
	"world":     PassWorldOpaque,
	"lightmaps": PassLightmaps,
	"brush":     PassBrushEntities,
	"alias":     PassAliasEntities,
	"viewmodel": PassViewModel,
	"water":     PassTranslucentLiquids,
	"particles": PassParticles,
	"decals":    PassDecals,
	"overlay":   Pass2DOverlay,
}

// GetPassTogglesMap returns a map of pass names to enabled booleans.
func GetPassTogglesMap() map[string]bool {
	mask := GetGlobalPassFlags()
	res := make(map[string]bool, len(PassNameToFlag))
	for name, flag := range PassNameToFlag {
		res[name] = (mask & flag) != 0
	}
	return res
}

// SetPassToggleByName sets a pass state by string name.
func SetPassToggleByName(name string, enabled bool) bool {
	flag, ok := PassNameToFlag[name]
	if !ok {
		return false
	}
	SetGlobalPassEnabled(flag, enabled)
	return true
}
