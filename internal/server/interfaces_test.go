// interfaces_test.go provides static compile-time assertions that *Server implements
// all subsystem interfaces (CollisionWorld, EntityStore, PhysicsConfig, FrameTiming,
// ThinkExecutor, PhysicsEngine).

// This file belongs to the Server Lifecycle subsystem: server struct, frame timing, PVS, user commands, spawn parameters, rules, and core server types.
package server

import (
	"testing"
)

// Static compile-time interface satisfaction checks.
var (
	_ CollisionWorld = (*Server)(nil)
	_ EntityStore    = (*Server)(nil)
	_ PhysicsConfig  = (*Server)(nil)
	_ FrameTiming    = (*Server)(nil)
	_ ThinkExecutor  = (*Server)(nil)
	_ PhysicsEngine  = (*Server)(nil)
	_ MovementEngine = (*Server)(nil)
)

func TestServerImplementsInterfaces(t *testing.T) {
	// Creating a zero-value Server instance to verify interface compliance dynamically as well.
	var s Server
	var _ CollisionWorld = &s
	var _ EntityStore = &s
	var _ PhysicsConfig = &s
	var _ FrameTiming = &s
	var _ ThinkExecutor = &s
	var _ PhysicsEngine = &s
	var _ MovementEngine = &s
}

