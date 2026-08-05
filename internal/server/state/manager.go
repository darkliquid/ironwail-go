package state

import (
	"fmt"
	"sync"
)

// ServerStateContext provides access to server state properties required by state operations.
type ServerStateContext interface {
	ProtocolFlags() uint32
	MaxClients() int
	DropClient(clientNum int, reason string)
}

// SessionManager manages active client connection session states, packet buffers, and signon tracking.
type SessionManager struct {
	mu             sync.RWMutex
	maxClients     int
	maxMessageSize int
	clients        map[int]*ClientSessionState
}

// NewSessionManager allocates a SessionManager with client slot capacity maxClients.
func NewSessionManager(maxClients int, maxMessageSize int) *SessionManager {
	if maxClients <= 0 {
		maxClients = 16
	}
	if maxMessageSize <= 0 {
		maxMessageSize = 8192
	}
	return &SessionManager{
		maxClients:     maxClients,
		maxMessageSize: maxMessageSize,
		clients:        make(map[int]*ClientSessionState, maxClients),
	}
}

// Client returns the ClientSessionState for clientNum, or nil if unallocated.
func (sm *SessionManager) Client(clientNum int) *ClientSessionState {
	if sm == nil {
		return nil
	}
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.clients[clientNum]
}

// AddClient allocates or reuses a session state for clientNum.
func (sm *SessionManager) AddClient(clientNum int) (*ClientSessionState, error) {
	if sm == nil {
		return nil, fmt.Errorf("nil session manager")
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if clientNum < 0 || clientNum >= sm.maxClients {
		return nil, fmt.Errorf("client num %d out of bounds [0, %d)", clientNum, sm.maxClients)
	}

	cs, ok := sm.clients[clientNum]
	if !ok || cs == nil {
		cs = NewClientSessionState(clientNum, sm.maxMessageSize)
		sm.clients[clientNum] = cs
	} else {
		cs.Reset()
	}
	return cs, nil
}

// RemoveClient drops and clears the session state for clientNum.
func (sm *SessionManager) RemoveClient(clientNum int) {
	if sm == nil {
		return
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if cs, ok := sm.clients[clientNum]; ok && cs != nil {
		cs.Reset()
		delete(sm.clients, clientNum)
	}
}

// ActiveCount returns the number of active/spawned client sessions.
func (sm *SessionManager) ActiveCount() int {
	if sm == nil {
		return 0
	}
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	count := 0
	for _, cs := range sm.clients {
		if cs != nil && cs.Active {
			count++
		}
	}
	return count
}

// Reset clears all client session states between level loads.
func (sm *SessionManager) Reset() {
	if sm == nil {
		return
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, cs := range sm.clients {
		if cs != nil {
			cs.Reset()
		}
	}
	clear(sm.clients)
}
