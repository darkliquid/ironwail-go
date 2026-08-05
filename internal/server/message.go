// message.go re-exports MessageBuffer for package server.
package server

import (
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

// NewMessageBuffer creates a new message buffer with the given capacity.
func NewMessageBuffer(size int) *MessageBuffer {
	return srvtypes.NewMessageBuffer(size)
}

func coordWireSize(flags uint32) int {
	return srvtypes.CoordWireSize(flags)
}

func angleWireSize(flags uint32) int {
	return srvtypes.AngleWireSize(flags)
}
