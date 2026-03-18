package server

// Frame runs a single server frame
func (s *Server) Frame(frameTime float64) error {
	if !s.Active || s.Paused {
		return nil
	}

	s.FrameTime = float32(frameTime)

	s.ClearDatagram()

	if err := s.CheckForNewClients(); err != nil {
		return err
	}

	// Read client input and update player intent before physics, matching C ordering.
	s.RunClients()

	// Run server physics.
	s.Physics()

	// Enforce multiplayer match rules after simulation.
	s.CheckRules()

	// Handle networking/datagrams
	s.SendClientMessages()

	return nil
}

// IsActive returns whether the server is currently active
func (s *Server) IsActive() bool {
	return s.Active
}

// IsPaused returns whether the server is currently paused
func (s *Server) IsPaused() bool {
	return s.Paused
}
