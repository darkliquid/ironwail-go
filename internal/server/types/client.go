// client.go defines the Client struct representing a connected client's
// per-connection simulation state.
package types

import (
	inet "github.com/darkliquid/ironwail-go/internal/net"
)

// Client represents a connected client's per-connection simulation state:
// signon progression, input, spawn parameters, stats, and per-client
// network/entity state. It was moved out of package server so subpackage
// interfaces (e.g. a client thinker or net manager) can reference it without
// importing the server facade.
type Client struct {
	Active        bool
	Spawned       bool
	DropASAP      bool
	SendSignon    SignonStage
	Loopback      bool
	NetConnection *inet.Socket // Per-client network socket

	LastMessage float64

	Name  string
	Color int

	Edict *Edict

	PingTimes [16]float32
	NumPings  int

	SpawnParms [16]float32
	// Client input state
	LastCmd            UserCmd
	LoopbackCmdPending bool
	Message            *MessageBuffer
	SignonIdx          int
	OldFrags           int // Previous frags count for reliable message updates
	EntityStates       map[int]EntityState
	RespawnTime        float32
	FatPVS             []byte
	Stats              [32]int32
	OldStats           [32]int32
}
