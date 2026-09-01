package server

import (
	"fmt"
	"sync"

	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/qc/dap"
)

var (
	dapServerMu sync.Mutex
	dapServer   *dap.Server

	defaultDAPFieldNames = map[string]int{
		"classname":  qc.EntFieldClassName,
		"origin":     qc.EntFieldOrigin,
		"angles":     qc.EntFieldAngles,
		"velocity":   qc.EntFieldVelocity,
		"health":     qc.EntFieldHealth,
		"target":     qc.EntFieldTarget,
		"targetname": qc.EntFieldTargetName,
		"model":      qc.EntFieldModel,
		"solid":      qc.EntFieldSolid,
		"movetype":   qc.EntFieldMoveType,
		"nextthink":  qc.EntFieldNextThink,
	}
)

// VM returns the server's QuakeC virtual machine.
func (s *Server) VM() *qc.VM {
	if s == nil {
		return nil
	}
	return s.QCVM
}

// EdictCount returns the active number of edicts.
func (s *Server) EdictCount() int {
	if s == nil {
		return 0
	}
	return s.NumEdicts
}

// GetEdictFloat returns a float field from an edict.
func (s *Server) GetEdictFloat(entNum, offset int) float32 {
	if s == nil || s.QCVM == nil || entNum < 0 || entNum >= s.NumEdicts {
		return 0
	}
	return s.QCVM.EFloat(entNum, offset)
}

// GetEdictString returns a string field from an edict.
func (s *Server) GetEdictString(entNum, offset int) string {
	if s == nil || s.QCVM == nil || entNum < 0 || entNum >= s.NumEdicts {
		return ""
	}
	strIdx := s.QCVM.EString(entNum, offset)
	return s.QCVM.String(strIdx)
}

// GetEdictVector returns a vector field from an edict.
func (s *Server) GetEdictVector(entNum, offset int) [3]float32 {
	if s == nil || s.QCVM == nil || entNum < 0 || entNum >= s.NumEdicts {
		return [3]float32{}
	}
	v := s.QCVM.EVector(entNum, offset)
	return [3]float32{v.X, v.Y, v.Z}
}

// GetEdictClassName returns the classname of an edict.
func (s *Server) GetEdictClassName(entNum int) string {
	return s.GetEdictString(entNum, qc.EntFieldClassName)
}

// FieldNames returns a map of field names to bytecode offsets.
func (s *Server) FieldNames() map[string]int {
	return defaultDAPFieldNames
}

// StartDAPServer launches a DAP listener on addr for this server.
func (s *Server) StartDAPServer(addr string) (*dap.Server, error) {
	dapServerMu.Lock()
	defer dapServerMu.Unlock()

	if dapServer != nil {
		_ = dapServer.Close()
		dapServer = nil
	}

	srv, err := dap.NewServer(addr, s)
	if err != nil {
		return nil, err
	}
	dapServer = srv
	return srv, nil
}

// StopDAPServer stops any active DAP listener.
func StopDAPServer() {
	dapServerMu.Lock()
	defer dapServerMu.Unlock()
	if dapServer != nil {
		_ = dapServer.Close()
		dapServer = nil
	}
}

// DAPServerStatus returns the status/address of the active DAP listener.
func DAPServerStatus() string {
	dapServerMu.Lock()
	defer dapServerMu.Unlock()
	if dapServer == nil {
		return "DAP debug server is inactive"
	}
	return fmt.Sprintf("DAP debug server listening on %s", dapServer.Addr().String())
}

// ActiveDAPServer returns the active DAP server instance, or nil if none is active.
func ActiveDAPServer() *dap.Server {
	dapServerMu.Lock()
	defer dapServerMu.Unlock()
	return dapServer
}
