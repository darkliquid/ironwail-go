// This file belongs to the Physics/Collision subsystem: collision detection, spatial queries, movement, and per-entity physics simulation.

package server

// Physics runs one server physics frame. The per-edict movetype dispatch,
// QC StartFrame, client pre/post think, SendInterval bookkeeping, and time
// advance now live in physics.System.StepFrame; this method is a thin
// delegator wiring the server's injected engines and facades.
func (s *Server) Physics() {
	s.ensurePhysicsSys()
	s.Time = s.PhysicsSys.StepFrame(s, s.Time, s.FrameTime)
}

// PeakEdicts returns the highest active edict count seen by Physics.
func (s *Server) PeakEdicts() int {
	if s == nil {
		return 0
	}
	return s.peakEdicts
}
