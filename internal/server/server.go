package server

import (
	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/qc"
)

// Server holds the state for the current running game.
type Server struct {
	Active   bool
	Paused   bool
	LoadGame bool

	State ServerState

	Name      string
	ModelName string

	WorldModel interface{}
	WorldTree  *bsp.Tree // BSP tree retained for rendering

	// Physics settings
	Gravity     float32
	MaxVelocity float32
	Friction    float32
	StopSpeed   float32

	// Timing
	Time      float32
	FrameTime float32

	// Entity management
	Edicts    []*Edict
	NumEdicts int
	MaxEdicts int

	// QuakeC VM integration
	QCVM *qc.VM
	// Static data (persists across levels)
	Static *ServerStatic

	// Area nodes for spatial partitioning
	Areanodes     []AreaNode
	numAreaNodes int





	// Network messaging
	Datagram *MessageBuffer



	// Precached resources
	SoundPrecache []string
	ModelPrecache []string

	// Game rules
	Coop       bool
	Deathmatch bool
}

// ServerStatic holds state that persists across level changes.
type ServerStatic struct {
	MaxClients        int
	MaxClientsLimit   int
	Clients           []*Client
	ServerFlags       int
	ChangeLevelIssued bool
}

// Client represents a connected player.
type Client struct {
	Active     bool
	Spawned    bool
	DropASAP   bool
	SendSignon SignonStage

	LastMessage float64

	Name  string
	Color int

	Edict *Edict

	PingTimes [16]float32
	NumPings  int

	SpawnParms [16]float32
	// Client input state
	LastCmd  UserCmd
	Message  *MessageBuffer
	OldFrags int // Previous frags count for reliable message updates
}

// AreaNode is a node in the spatial partitioning tree for entity collision.
type AreaNode struct {
	Axis          int
	Dist          float32
	Children      [2]*AreaNode
	TriggerEdicts Edict
	SolidEdicts   Edict
}

// NewServer creates a new server instance.
func NewServer() *Server {
	return &Server{
		Gravity:     800,
		MaxVelocity: 2000,
		Friction:    4,
		StopSpeed:   100,
		MaxEdicts:   1024,
		QCVM:        qc.NewVM(),
	}
}

// AllocEdict allocates a new entity.
func (s *Server) AllocEdict() *Edict {
	for i, e := range s.Edicts {
		if e.Free {
			s.NumEdicts = max(s.NumEdicts, i+1)
			return e
		}
	}

	if len(s.Edicts) >= s.MaxEdicts {
		return nil
	}

	e := &Edict{
		Vars: &EntVars{},
	}
	s.Edicts = append(s.Edicts, e)
	s.NumEdicts = len(s.Edicts)
	return e
}

// FreeEdict marks an entity as free.
func (s *Server) FreeEdict(e *Edict) {
	e.Free = true
	e.FreeTime = s.Time
}

// EdictNum returns the entity at the given index.
func (s *Server) EdictNum(n int) *Edict {
	if n < 0 || n >= len(s.Edicts) {
		return nil
	}
	return s.Edicts[n]
}

// NumForEdict returns the index for the given entity.
func (s *Server) NumForEdict(e *Edict) int {
	for i, ent := range s.Edicts {
		if ent == e {
			return i
		}
	}
	return -1
}
func (s *Server) GetMaxClients() int {
	if s.Static == nil {
		return 0
	}
	return s.Static.MaxClients
}

func (s *Server) GetClientName(clientNum int) string {
	if s.Static == nil || clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return ""
	}
	if s.Static.Clients[clientNum] == nil {
		return ""
	}
	return s.Static.Clients[clientNum].Name
}

func (s *Server) SetClientName(clientNum int, name string) {
	if s.Static == nil || clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return
	}
	if s.Static.Clients[clientNum] == nil {
		return
	}
	s.Static.Clients[clientNum].Name = name
}

func (s *Server) GetClientColor(clientNum int) int {
	if s.Static == nil || clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return 0
	}
	if s.Static.Clients[clientNum] == nil {
		return 0
	}
	return s.Static.Clients[clientNum].Color
}

func (s *Server) SetClientColor(clientNum int, color int) {
	if s.Static == nil || clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return
	}
	if s.Static.Clients[clientNum] == nil {
		return
	}
	s.Static.Clients[clientNum].Color = color
}

func (s *Server) GetClientPing(clientNum int) float32 {
	if s.Static == nil || clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return 0
	}
	c := s.Static.Clients[clientNum]
	if c == nil || c.NumPings == 0 {
		return 0
	}
	
	var total float32
	count := min(c.NumPings, 16)
	for i := 0; i < count; i++ {
		total += c.PingTimes[i]
	}
	return total / float32(count) * 1000
}
func (s *Server) GetMapName() string {
	return s.Name
}


