// Package sim provides a deterministic, standalone simulation harness for
// QuakeGo mod code, independent of the full engine.
//
// It is the engine-side foundation of cmd/qcmod (plan 25): a World owns an
// edict registry, a deterministic clock, and a wiring of the QuakeGo engine
// builtin Backend (pkg/qgo/quake/engine) so mod functions (think/touch/use)
// run in ordinary Go with Go-native asserts — no progs.dat bytecode, no GPU,
// no assets.
//
// Because pkg/qgo/quake is a separate Go module (AGENTS.md gotcha #2), this
// package must live inside that module. It imports "quake" and
// "quake/engine" only.
package sim

import (
	"fmt"
	"sync"

	"quake"
	"quake/engine"
)

// World is a minimal deterministic Quake world for mod testing.
//
// It is NOT a physics engine: it gives mod code a place to spawn entities,
// advance a clock, invoke touches/uses/thinks, and observe engine builtin
// side effects through the Reporter/recorder hooks. Physics fidelity lives in
// the In-VM runner (cmd/qcmod --vm) which drives the real internal/server
// physics System against the QCVM.
type World struct {
	mu sync.Mutex

	// Ents is the edict table; index 0 is the world spawn.
	Ents []*quake.Entity

	// Time is the current game time.
	Time float32

	// FrameTime is the per-step duration.
	FrameTime float32

	// BuiltinCalls records every engine builtin invocation (name -> count).
	BuiltinCalls map[string]int

	// Sounds records sound() invocations in order.
	Sounds []string

	lastDprint string
}

// New creates a World with a world-spawn edict (entity 0) and the default
// frame time.
func New() *World {
	w := &World{
		Ents:         []*quake.Entity{{}},
		FrameTime:    0.1,
		BuiltinCalls: map[string]int{},
	}
	w.InstallBackend()
	return w
}

// InstallBackend wires the QuakeGo engine Backend so builtins run against
// this World's registry + clock instead of the stub behavior.
func (w *World) InstallBackend() {
	engine.SetBackend(engine.Backend{
		Spawn: func() *quake.Entity {
			w.BuiltinCalls["spawn"]++
			e := &quake.Entity{}
			w.Ents = append(w.Ents, e)
			return e
		},
		Remove: func(e *quake.Entity) {
			w.BuiltinCalls["remove"]++
			for i, ent := range w.Ents {
				if ent == e {
					w.Ents[i] = nil
					return
				}
			}
		},
		SetOrigin: func(e *quake.Entity, org quake.Vec3) {
			w.BuiltinCalls["setorigin"]++
			old := e.Origin
			e.Origin = org
			// QC convention: origin change shifts absmin/absmax by same delta.
			delta := org.Sub(old)
			e.AbsMin = e.AbsMin.Add(delta)
			e.AbsMax = e.AbsMax.Add(delta)
		},
		SetModel: func(e *quake.Entity, m string) {
			w.BuiltinCalls["setmodel"]++
			e.Model = m
		},
		SetSize: func(e *quake.Entity, min, max quake.Vec3) {
			w.BuiltinCalls["setsize"]++
			e.Mins = min
			e.Maxs = max
			e.Size = max.Sub(min)
		},
		Random: func() float32 {
			w.BuiltinCalls["random"]++
			// Deterministic: 0.5 always (tests can vary by installing their
			// own Backend over this one).
			return 0.5
		},
		Sound: func(e *quake.Entity, ch int, samp string, vol float32, atten float32) {
			w.BuiltinCalls["sound"]++
			w.Sounds = append(w.Sounds, samp)
		},
		PrecacheSound: func(s string) string { w.BuiltinCalls["precachesound"]++; return s },
		PrecacheModel: func(s string) string { w.BuiltinCalls["precachemodel"]++; return s },
		Dprint: func(s string) {
			w.BuiltinCalls["dprint"]++
			w.lastDprint += s
		},
		Find: func(e *quake.Entity, field, match string) *quake.Entity {
			w.BuiltinCalls["find"]++
			for _, ent := range w.Ents {
				if ent == nil || ent == e {
					continue
				}
				if matchField(ent, field) == match {
					return ent
				}
			}
			return nil
		},
		FAbs: func(f float32) float32 {
			w.BuiltinCalls["fabs"]++
			if f < 0 {
				return -f
			}
			return f
		},
	})
}

// matchField resolves a QuakeC-style field name on an Entity for the find()
// builtin. Supports the most common string fields.
func matchField(e *quake.Entity, field string) string {
	switch field {
	case "classname":
		return e.ClassName
	case "targetname":
		return e.TargetName
	case "target":
		return e.Target
	case "message":
		return e.Message
	case "model":
		return e.Model
	}
	return ""
}

// Spawn allocates a new entity with the given classname and returns it.
func (w *World) Spawn(classname string) *quake.Entity {
	w.mu.Lock()
	defer w.mu.Unlock()
	e := &quake.Entity{ClassName: classname}
	w.Ents = append(w.Ents, e)
	return e
}

// Add appends an externally-constructed entity (e.g. a pre-linked door) and
// returns it.
func (w *World) Add(e *quake.Entity) *quake.Entity {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Ents = append(w.Ents, e)
	return e
}

// Step advances the clock by one frame time.
func (w *World) Step() {
	w.mu.Lock()
	w.Time += w.FrameTime
	w.mu.Unlock()
}

// StepN advances the clock by n frames.
func (w *World) StepN(n int) {
	for i := 0; i < n; i++ {
		w.Step()
	}
}

// Fire sets the self/other globals and calls fn, mirroring how the engine
// dispatches a QC callback. fn is a plain Go func (quake.Func is `type Func
// func()`), usually a mod entity's Think/Touch/Use closure.
func (w *World) Fire(self *quake.Entity, other *quake.Entity, fn func()) error {
	if fn == nil {
		return fmt.Errorf("sim: nil function")
	}
	w.mu.Lock()
	engine.Self = *self
	engine.Time = w.Time
	if other != nil {
		engine.Other = *other
	}
	fn()
	w.mu.Unlock()
	return nil
}

// Dprint returns the accumulated dprint output since last ResetDprint.
func (w *World) Dprint() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastDprint
}

// ResetDprint clears accumulated dprint output.
func (w *World) ResetDprint() {
	w.mu.Lock()
	w.lastDprint = ""
	w.mu.Unlock()
}

// Entity returns entity n, or nil.
func (w *World) Entity(n int) *quake.Entity {
	w.mu.Lock()
	defer w.mu.Unlock()
	if n < 0 || n >= len(w.Ents) {
		return nil
	}
	return w.Ents[n]
}
