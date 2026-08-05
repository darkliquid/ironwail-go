package renderer

import (
	"github.com/darkliquid/ironwail-go/internal/renderer/decal"
)

// DecalMarkSystem keeps projected mark entities alive for a limited lifetime.
// It delegates to the decal subpackage's mark system; DecalMarkEntity
// satisfies decal.MarkEntity.
type DecalMarkSystem struct {
	system *decal.System
}

// NewDecalMarkSystem creates an empty decal mark system.
func NewDecalMarkSystem() *DecalMarkSystem {
	return &DecalMarkSystem{system: decal.NewSystem()}
}

// AddMark appends a mark with lifetime in seconds. Non-positive lifetimes are ignored.
func (s *DecalMarkSystem) AddMark(mark DecalMarkEntity, lifetimeSeconds, timeNow float32) {
	if s == nil {
		return
	}
	s.system.AddMark(mark, lifetimeSeconds, timeNow)
}

// Run advances mark expiration.
func (s *DecalMarkSystem) Run(timeNow float32) {
	if s == nil {
		return
	}
	s.system.Run(timeNow)
}

// ActiveMarks returns a copy of currently visible marks.
func (s *DecalMarkSystem) ActiveMarks() []DecalMarkEntity {
	if s == nil {
		return nil
	}
	entities := s.system.ActiveMarkEntities()
	out := make([]DecalMarkEntity, 0, len(entities))
	for _, e := range entities {
		out = append(out, e.(DecalMarkEntity))
	}
	return out
}

// ActiveCount returns number of currently active marks.
func (s *DecalMarkSystem) ActiveCount() int {
	if s == nil {
		return 0
	}
	return s.system.ActiveCount()
}
