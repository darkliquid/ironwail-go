// message.go re-exports MessageBuffer for package server.
package server

import (
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

// NewMessageBuffer creates a new message buffer with the given capacity.
func NewMessageBuffer(size int) *MessageBuffer {
	return srvtypes.NewMessageBuffer(size)
}
