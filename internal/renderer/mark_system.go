package renderer

// DecalMarkSystem keeps projected mark entities alive for a limited lifetime.
type DecalMarkSystem struct {
	marks []timedDecalMark
}

type timedDecalMark struct {
	mark  DecalMarkEntity
	dieAt float32
}

// NewDecalMarkSystem creates an empty decal mark system.
func NewDecalMarkSystem() *DecalMarkSystem {
	return &DecalMarkSystem{marks: make([]timedDecalMark, 0, 256)}
}

// AddMark appends a mark with lifetime in seconds. Non-positive lifetimes are ignored.
func (s *DecalMarkSystem) AddMark(mark DecalMarkEntity, lifetimeSeconds, timeNow float32) {
	if s == nil || lifetimeSeconds <= 0 {
		return
	}
	if mark.Size <= 0 || clamp01(mark.Alpha) <= 0 {
		return
	}
	mark.Alpha = clamp01(mark.Alpha)
	s.marks = append(s.marks, timedDecalMark{mark: mark, dieAt: timeNow + lifetimeSeconds})
}

// Run advances mark expiration.
func (s *DecalMarkSystem) Run(timeNow float32) {
	if s == nil || len(s.marks) == 0 {
		return
	}
	alive := 0
	for i := range s.marks {
		if s.marks[i].dieAt > timeNow {
			s.marks[alive] = s.marks[i]
			alive++
		}
	}
	s.marks = s.marks[:alive]
}

// ActiveMarks returns a copy of currently visible marks.
func (s *DecalMarkSystem) ActiveMarks() []DecalMarkEntity {
	if s == nil || len(s.marks) == 0 {
		return nil
	}
	out := make([]DecalMarkEntity, 0, len(s.marks))
	for i := range s.marks {
		out = append(out, s.marks[i].mark)
	}
	return out
}

// ActiveCount returns number of currently active marks.
func (s *DecalMarkSystem) ActiveCount() int {
	if s == nil {
		return 0
	}
	return len(s.marks)
}
