package server

import (
	"log/slog"
	"time"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

// Frame runs a single server frame
func (s *Server) Frame(frameTime float64) error {
	frameStart := time.Now()
	hostSpeeds := cvar.BoolValue("host_speeds")
	phaseStart := time.Time{}
	var checkClientsMS float64
	var runClientsMS float64
	var physicsMS float64
	var rulesMS float64
	var sendMessagesMS float64
	if !s.Active || s.Paused {
		return nil
	}

	s.FrameTime = float32(frameTime)
	s.recordDevStatsFrame()
	s.ClearDatagram()

	if hostSpeeds {
		phaseStart = time.Now()
	}
	if err := s.CheckForNewClients(); err != nil {
		return err
	}
	if hostSpeeds {
		checkClientsMS = float64(time.Since(phaseStart)) / float64(time.Millisecond)
		phaseStart = time.Now()
	}

	// Read client input and update player intent before physics, matching C ordering.
	s.RunClients()
	if hostSpeeds {
		runClientsMS = float64(time.Since(phaseStart)) / float64(time.Millisecond)
		phaseStart = time.Now()
	}

	// Run server physics.
	s.Physics()
	if hostSpeeds {
		physicsMS = float64(time.Since(phaseStart)) / float64(time.Millisecond)
		phaseStart = time.Now()
	}

	// Enforce multiplayer match rules after simulation.
	s.CheckRules()
	if hostSpeeds {
		rulesMS = float64(time.Since(phaseStart)) / float64(time.Millisecond)
		phaseStart = time.Now()
	}

	// Handle networking/datagrams
	s.SendClientMessages()
	if hostSpeeds {
		sendMessagesMS = float64(time.Since(phaseStart)) / float64(time.Millisecond)
		slog.Info("server_speeds",
			"time", s.Time,
			"check_clients_ms", checkClientsMS,
			"run_clients_ms", runClientsMS,
			"physics_ms", physicsMS,
			"rules_ms", rulesMS,
			"send_messages_ms", sendMessagesMS,
			"total_ms", float64(time.Since(frameStart))/float64(time.Millisecond),
		)
	}

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

// SetLoadGame sets the LoadGame flag, which preserves full client edict state
// during savegame restore signon.
func (s *Server) SetLoadGame(v bool) {
	s.LoadGame = v
}

// SetPreserveSpawnParms keeps client spawn parms across reconnect while still
// allowing normal player spawn placement in the next map.
func (s *Server) SetPreserveSpawnParms(v bool) {
	s.PreserveSpawnParms = v
}
