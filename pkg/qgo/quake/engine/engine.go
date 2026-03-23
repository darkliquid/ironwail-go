package engine

import "github.com/ironwail/ironwail-go/pkg/qgo/quake"

// Engine globals
var (
	Self   quake.Entity
	Other  quake.Entity
	World  quake.Entity
	Time   float32
	NextEnt quake.Entity
)

// Built-in functions (mapped to QC builtins via //qgo:builtin N)

//go:noinline
func Dprint(s string) {}

//go:noinline
func Print(s string) {}

//go:noinline
func Bprint(s string) {}

//go:noinline
func Error(s string) {}

//go:noinline
func Vlen(v quake.Vec3) float32 { return 0 }

//go:noinline
func Vectoyaw(v quake.Vec3) float32 { return 0 }

//go:noinline
func Normalize(v quake.Vec3) quake.Vec3 { return v }

//go:noinline
func Spawn() quake.Entity { return 0 }

//go:noinline
func Remove(e quake.Entity) {}

//go:noinline
func PrecacheModel(s string) string { return s }

//go:noinline
func PrecacheSound(s string) string { return s }

//go:noinline
func SetModel(e quake.Entity, m string) {}

//go:noinline
func SetSize(e quake.Entity, min, max quake.Vec3) {}

//go:noinline
func SetOrigin(e quake.Entity, org quake.Vec3) {}

//go:noinline
func Ambientsound(pos quake.Vec3, samp string, vol, atten float32) {}

//go:noinline
func Sound(e quake.Entity, ch int, samp string, vol, atten float32) {}

//go:noinline
func Traceline(v1, v2 quake.Vec3, nomonsters int, e quake.Entity) {}

//go:noinline
func Random() float32 { return 0 }

//go:noinline
func Changelevel(s string) {}

//go:noinline
func Cvar(s string) float32 { return 0 }

//go:noinline
func CvarSet(s string, v float32) {}

//go:noinline
func Centerprint(s string) {}
