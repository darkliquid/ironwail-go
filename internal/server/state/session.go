package state

import (
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

// ClientSessionState tracks the connection and signon sequence stage of a client.
type ClientSessionState struct {
	Active    bool
	Spawned   bool
	DropAsap  bool
	Signon    srvtypes.SignonStage
	ClientNum int
	SendCount int
	Datagram  *srvtypes.MessageBuffer
	Reliable  *srvtypes.MessageBuffer
}

// NewClientSessionState creates a new client session state container.
func NewClientSessionState(clientNum int, maxMessageSize int) *ClientSessionState {
	return &ClientSessionState{
		ClientNum: clientNum,
		Signon:    srvtypes.SignonNone,
		Datagram:  srvtypes.NewMessageBuffer(maxMessageSize),
		Reliable:  srvtypes.NewMessageBuffer(maxMessageSize),
	}
}

// AdvanceSignon transitions the signon stage if expected matches the current stage.
func (cs *ClientSessionState) AdvanceSignon(expected srvtypes.SignonStage, next srvtypes.SignonStage) bool {
	if cs.Signon == expected {
		cs.Signon = next
		if next == srvtypes.SignonDone {
			cs.Active = true
			cs.Spawned = true
		}
		return true
	}
	return false
}

// Reset clears session state for disconnects or map loads.
func (cs *ClientSessionState) Reset() {
	cs.Active = false
	cs.Spawned = false
	cs.DropAsap = false
	cs.Signon = srvtypes.SignonNone
	cs.SendCount = 0
	if cs.Datagram != nil {
		cs.Datagram.Clear()
	}
	if cs.Reliable != nil {
		cs.Reliable.Clear()
	}
}
