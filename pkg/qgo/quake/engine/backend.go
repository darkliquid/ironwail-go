package engine

import (
	"sync"

	"github.com/darkliquid/ironwail-go/pkg/qgo/quake"
)

// Backend provides optional runtime hooks for selected engine builtins.
//
// Nil hooks preserve existing stub behavior, while configured hooks let tests
// execute translated gameplay logic with deterministic engine behavior.
type Backend struct {
	MakeVectors   func(ang quake.Vec3)
	SetOrigin     func(e *quake.Entity, org quake.Vec3)
	SetModel      func(e *quake.Entity, m string)
	SetSize       func(e *quake.Entity, min, max quake.Vec3)
	Random        func() float32
	Sound         func(e *quake.Entity, ch int, samp string, vol float32, atten float32)
	ObjError      func(s string)
	Vlen          func(v quake.Vec3) float32
	Vectoyaw      func(v quake.Vec3) float32
	Spawn         func() *quake.Entity
	Remove        func(e *quake.Entity)
	Find          func(e *quake.Entity, field string, match string) *quake.Entity
	PrecacheSound func(s string) string
	PrecacheModel func(s string) string
	Dprint        func(s string)
	Vtos          func(v quake.Vec3) string
	WalkMove      func(yaw float32, dist float32) float32
	DropToFloor   func() float32
	FAbs          func(f float32) float32
	Cvar          func(s string) float32
	LocalCmd      func(s string)
	WriteByte     func(dest float32, b float32)
	WriteString   func(dest float32, s string)
	Changelevel   func(s string)
	Centerprint   func(s string)
	SetSpawnParms func(e *quake.Entity)
}

var (
	backendMu sync.RWMutex
	current   Backend
)

// SetBackend installs the backend used by selected builtins in this package.
func SetBackend(backend Backend) {
	backendMu.Lock()
	current = backend
	backendMu.Unlock()
}

// ResetBackend clears all hooks and restores legacy stub behavior.
func ResetBackend() {
	SetBackend(Backend{})
}

func backend() Backend {
	backendMu.RLock()
	b := current
	backendMu.RUnlock()
	return b
}
