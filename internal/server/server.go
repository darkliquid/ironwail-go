package server

import (
	"errors"
	"math"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/console"
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
	Areanodes    []AreaNode
	numAreaNodes int

	// Network messaging
	Datagram *MessageBuffer

	// Precached resources
	SoundPrecache  []string
	ModelPrecache  []string
	StaticEntities []EntityState
	StaticSounds   []StaticSound

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
	s := &Server{
		Gravity:     800,
		MaxVelocity: 2000,
		Friction:    4,
		StopSpeed:   100,
		MaxEdicts:   1024,
		QCVM:        qc.NewVM(),
	}

	// Ensure entity 0 (worldspawn) exists so subsequent allocations
	// return entity indices starting at 1, matching the VM's
	// expectations where entity 0 is the level itself.
	world := &Edict{Vars: &EntVars{}}
	s.Edicts = append(s.Edicts, world)
	s.NumEdicts = 1

	// Register a minimal set of builtin hooks so QuakeC code that calls
	// simple entity APIs (spawn/remove/nextent) will be routed to the
	// server-side implementations. This is intentionally conservative
	// — more sophisticated behaviors (walkmove, findradius, etc.) will
	// be implemented incrementally during the port.
	ensurePrecache := func(cache *[]string, value string) {
		if value == "" {
			return
		}
		if len(*cache) == 0 {
			*cache = make([]string, 2)
			(*cache)[0] = ""
		}
		for _, existing := range *cache {
			if existing == value {
				return
			}
		}
		for i := 1; i < len(*cache); i++ {
			if (*cache)[i] == "" {
				(*cache)[i] = value
				return
			}
		}
		*cache = append(*cache, value)
	}
	clientForEntNum := func(entNum int) *Client {
		if s.Static == nil {
			return nil
		}
		ent := s.EdictNum(entNum)
		if ent == nil {
			return nil
		}
		for _, client := range s.Static.Clients {
			if client != nil && client.Edict == ent {
				return client
			}
		}
		return nil
	}
	writeBuffers := func(vm *qc.VM, dest int) []*MessageBuffer {
		switch dest {
		case 0:
			if s.Datagram != nil {
				return []*MessageBuffer{s.Datagram}
			}
		case 1:
			if client := clientForEntNum(int(vm.GInt(qc.OFSMsgEntity))); client != nil && client.Message != nil {
				return []*MessageBuffer{client.Message}
			}
		case 2, 3:
			if s.Static == nil {
				return nil
			}
			buffers := make([]*MessageBuffer, 0, len(s.Static.Clients))
			for _, client := range s.Static.Clients {
				if client != nil && client.Message != nil {
					buffers = append(buffers, client.Message)
				}
			}
			return buffers
		}
		return nil
	}
	syncEntityToVM := func(vm *qc.VM, entNum int, ent *Edict) {
		if vm == nil || ent == nil || ent.Vars == nil || entNum <= 0 || entNum >= vm.NumEdicts {
			return
		}
		vm.SetEVector(entNum, qc.EntFieldOrigin, ent.Vars.Origin)
		vm.SetEVector(entNum, qc.EntFieldAngles, ent.Vars.Angles)
		vm.SetEVector(entNum, qc.EntFieldAbsMin, ent.Vars.AbsMin)
		vm.SetEVector(entNum, qc.EntFieldAbsMax, ent.Vars.AbsMax)
		vm.SetEFloat(entNum, qc.EntFieldFlags, ent.Vars.Flags)
		vm.SetEFloat(entNum, qc.EntFieldGroundEnt, float32(ent.Vars.GroundEntity))
		vm.SetEFloat(entNum, qc.EntFieldIdealYaw, ent.Vars.IdealYaw)
	}

	qc.SetServerBuiltinHooks(qc.ServerBuiltinHooks{
		Traceline: func(vm *qc.VM, start, end [3]float32, noMonsters bool, passEnt int) qc.BuiltinTraceResult {
			moveType := MoveType(MoveNormal)
			if noMonsters {
				moveType = MoveType(MoveNoMonsters)
			}
			var pass *Edict
			if passEnt > 0 {
				pass = s.EdictNum(passEnt)
			}
			trace := s.SV_Move(start, [3]float32{}, [3]float32{}, end, moveType, pass)
			res := qc.BuiltinTraceResult{
				AllSolid:    trace.AllSolid,
				StartSolid:  trace.StartSolid,
				Fraction:    trace.Fraction,
				EndPos:      trace.EndPos,
				PlaneNormal: trace.PlaneNormal,
				InOpen:      s.PointContents(trace.EndPos) == bsp.ContentsEmpty,
			}
			if trace.Entity != nil {
				res.EntNum = s.NumForEdict(trace.Entity)
			}
			return res
		},
		Spawn: func(vm *qc.VM) (int, error) {
			e := s.AllocEdict()
			if e == nil {
				return 0, errors.New("no free edict")
			}
			return s.NumForEdict(e), nil
		},
		Remove: func(vm *qc.VM, entNum int) error {
			e := s.EdictNum(entNum)
			if e == nil {
				return nil
			}
			s.FreeEdict(e)
			return nil
		},
		Find: func(vm *qc.VM, startEnt, fieldOfs int, match string) int {
			for entNum := startEnt + 1; entNum < vm.NumEdicts; entNum++ {
				if vm.GetString(vm.EString(entNum, fieldOfs)) == match {
					return entNum
				}
			}
			return 0
		},
		FindFloat: func(vm *qc.VM, startEnt, fieldOfs int, match float32) int {
			for entNum := startEnt + 1; entNum < vm.NumEdicts; entNum++ {
				if vm.EFloat(entNum, fieldOfs) == match {
					return entNum
				}
			}
			return 0
		},
		FindRadius: func(vm *qc.VM, org [3]float32, radius float32) int {
			if radius < 0 {
				return 0
			}
			radSq := radius * radius
			for entNum := 1; entNum < vm.NumEdicts; entNum++ {
				entOrg := vm.EVector(entNum, qc.EntFieldOrigin)
				dx := entOrg[0] - org[0]
				dy := entOrg[1] - org[1]
				dz := entOrg[2] - org[2]
				if dx*dx+dy*dy+dz*dz <= radSq {
					return entNum
				}
			}
			return 0
		},
		CheckClient: func(vm *qc.VM) int {
			if s.Static == nil {
				return 0
			}
			self := int(vm.GInt(qc.OFSSelf))
			for _, client := range s.Static.Clients {
				if client == nil || !client.Active || client.Edict == nil || client.Edict.Free {
					continue
				}
				entNum := s.NumForEdict(client.Edict)
				if entNum > 0 && entNum != self {
					return entNum
				}
			}
			return 0
		},
		NextEnt: func(vm *qc.VM, entNum int) int {
			if entNum+1 > 0 && entNum+1 < vm.NumEdicts {
				return entNum + 1
			}
			return 0
		},
		CheckBottom: func(vm *qc.VM, entNum int) bool {
			if entNum <= 0 {
				return false
			}
			e := s.EdictNum(entNum)
			if e == nil || e.Vars == nil {
				return false
			}
			return s.CheckBottom(e)
		},
		PointContents: func(vm *qc.VM, point [3]float32) int {
			return s.PointContents(point)
		},
		Aim: func(vm *qc.VM, entNum int, missileSpeed float32) [3]float32 {
			ent := s.EdictNum(entNum)
			if ent == nil || ent.Vars == nil {
				return vm.GVector(qc.OFSGlobalVForward)
			}
			start := ent.Vars.Origin
			start[2] += ent.Vars.ViewOfs[2]
			targetNum := int(ent.Vars.AimEnt)
			if targetNum == 0 {
				targetNum = int(ent.Vars.Enemy)
			}
			if target := s.EdictNum(targetNum); target != nil && target.Vars != nil {
				end := target.Vars.Origin
				end[2] += 0.5 * (target.Vars.Mins[2] + target.Vars.Maxs[2])
				dir := [3]float32{end[0] - start[0], end[1] - start[1], end[2] - start[2]}
				if l := vm.VectorLength(dir); l > 0 {
					return vm.VectorNormalize(dir)
				}
			}
			return vm.GVector(qc.OFSGlobalVForward)
		},
		WalkMove: func(vm *qc.VM, yaw, dist float32) bool {
			self := int(vm.GInt(qc.OFSSelf))
			if self <= 0 || self >= vm.NumEdicts {
				return false
			}
			// Prefer server-side movement helpers which perform traces
			// and collision resolution. If the server has an Edict for
			// the entity, ask it to step in the given direction.
			if e := s.EdictNum(self); e != nil {
				ok := s.StepDirection(e, yaw, dist)
				// Mirror server-side origin back into the VM fields so
				// QuakeC sees the authoritative position.
				vm.SetEVector(self, qc.EntFieldOrigin, e.Vars.Origin)
				return ok
			}

			// Fallback: use a simple translation (should be rare).
			rad := float64(yaw) * math.Pi / 180.0
			dx := dist * float32(math.Cos(rad))
			dy := dist * float32(math.Sin(rad))
			org := vm.EVector(self, qc.EntFieldOrigin)
			newOrg := [3]float32{org[0] + dx, org[1] + dy, org[2]}
			vm.SetEVector(self, qc.EntFieldOrigin, newOrg)
			return true
		},
		DropToFloor: func(vm *qc.VM) bool {
			self := int(vm.GInt(qc.OFSSelf))
			if self <= 0 || self >= vm.NumEdicts {
				return false
			}
			// If the server has an Edict, run a downward trace using the
			// server Move helpers to land on the floor properly.
			if e := s.EdictNum(self); e != nil && e.Vars != nil {
				start := e.Vars.Origin
				end := start
				end[2] -= 1024 // large drop to find floor
				trace := s.SV_Move(start, e.Vars.Mins, e.Vars.Maxs, end, MoveType(MoveNormal), e)
				if trace.Fraction == 1 {
					return false
				}
				// Place entity on top of the surface found.
				newOrg := trace.EndPos
				vm.SetEVector(self, qc.EntFieldOrigin, newOrg)
				e.Vars.Origin = newOrg
				e.Vars.AbsMin = [3]float32{newOrg[0] + e.Vars.Mins[0], newOrg[1] + e.Vars.Mins[1], newOrg[2] + e.Vars.Mins[2]}
				e.Vars.AbsMax = [3]float32{newOrg[0] + e.Vars.Maxs[0], newOrg[1] + e.Vars.Maxs[1], newOrg[2] + e.Vars.Maxs[2]}
				return true
			}

			// Fallback: naive placement as before.
			mins := vm.EVector(self, qc.EntFieldMins)
			org := vm.EVector(self, qc.EntFieldOrigin)
			org[2] = -mins[2]
			vm.SetEVector(self, qc.EntFieldOrigin, org)
			return true
		},
		SetOrigin: func(vm *qc.VM, entNum int, org [3]float32) {
			vm.SetEVector(entNum, qc.EntFieldOrigin, org)
			mins := vm.EVector(entNum, qc.EntFieldMins)
			maxs := vm.EVector(entNum, qc.EntFieldMaxs)
			vm.SetEVector(entNum, qc.EntFieldAbsMin, [3]float32{org[0] + mins[0], org[1] + mins[1], org[2] + mins[2]})
			vm.SetEVector(entNum, qc.EntFieldAbsMax, [3]float32{org[0] + maxs[0], org[1] + maxs[1], org[2] + maxs[2]})
			if e := s.EdictNum(entNum); e != nil && e.Vars != nil {
				e.Vars.Origin = org
				e.Vars.AbsMin = [3]float32{org[0] + e.Vars.Mins[0], org[1] + e.Vars.Mins[1], org[2] + e.Vars.Mins[2]}
				e.Vars.AbsMax = [3]float32{org[0] + e.Vars.Maxs[0], org[1] + e.Vars.Maxs[1], org[2] + e.Vars.Maxs[2]}
			}
		},
		SetSize: func(vm *qc.VM, entNum int, mins, maxs [3]float32) {
			vm.SetEVector(entNum, qc.EntFieldMins, mins)
			vm.SetEVector(entNum, qc.EntFieldMaxs, maxs)
			size := [3]float32{maxs[0] - mins[0], maxs[1] - mins[1], maxs[2] - mins[2]}
			vm.SetEVector(entNum, qc.EntFieldSize, size)
			origin := vm.EVector(entNum, qc.EntFieldOrigin)
			vm.SetEVector(entNum, qc.EntFieldAbsMin, [3]float32{origin[0] + mins[0], origin[1] + mins[1], origin[2] + mins[2]})
			vm.SetEVector(entNum, qc.EntFieldAbsMax, [3]float32{origin[0] + maxs[0], origin[1] + maxs[1], origin[2] + maxs[2]})
			if e := s.EdictNum(entNum); e != nil && e.Vars != nil {
				e.Vars.Mins = mins
				e.Vars.Maxs = maxs
				e.Vars.Size = size
				e.Vars.AbsMin = [3]float32{origin[0] + mins[0], origin[1] + mins[1], origin[2] + mins[2]}
				e.Vars.AbsMax = [3]float32{origin[0] + maxs[0], origin[1] + maxs[1], origin[2] + maxs[2]}
			}
		},
		SetModel: func(vm *qc.VM, entNum int, modelName string) {
			vm.SetEInt(entNum, qc.EntFieldModel, vm.AllocString(modelName))
			if modelName != "" {
				vm.SetEFloat(entNum, qc.EntFieldModelIndex, 1)
			} else {
				vm.SetEFloat(entNum, qc.EntFieldModelIndex, 0)
			}
			if e := s.EdictNum(entNum); e != nil && e.Vars != nil {
				e.Vars.Model = vm.EInt(entNum, qc.EntFieldModel)
				e.Vars.ModelIndex = vm.EFloat(entNum, qc.EntFieldModelIndex)
			}
		},
		PrecacheSound: func(vm *qc.VM, sample string) {
			ensurePrecache(&s.SoundPrecache, sample)
		},
		PrecacheModel: func(vm *qc.VM, modelName string) {
			ensurePrecache(&s.ModelPrecache, modelName)
		},
		BroadcastPrint: func(vm *qc.VM, msg string) {
			console.Printf("%s", msg)
			if s.Static == nil {
				return
			}
			for _, client := range s.Static.Clients {
				if client == nil || client.Message == nil {
					continue
				}
				client.Message.WriteByte(byte(SVCPrint))
				client.Message.WriteString(msg)
			}
		},
		ClientPrint: func(vm *qc.VM, entNum int, msg string) {
			console.Printf("%s", msg)
			if client := clientForEntNum(entNum); client != nil && client.Message != nil {
				client.Message.WriteByte(byte(SVCPrint))
				client.Message.WriteString(msg)
			}
		},
		DebugPrint: func(vm *qc.VM, msg string) {
			console.Printf("%s", msg)
		},
		CenterPrint: func(vm *qc.VM, entNum int, msg string) {
			console.CenterPrintf(40, "%s", msg)
			if client := clientForEntNum(entNum); client != nil && client.Message != nil {
				client.Message.WriteByte(byte(SVCCenterPrint))
				client.Message.WriteString(msg)
			}
		},
		Sound: func(vm *qc.VM, entNum, channel int, sample string, volume int, attenuation float32) {
			if ent := s.EdictNum(entNum); ent != nil {
				s.StartSound(ent, channel, sample, volume, attenuation)
			}
		},
		StuffCmd: func(vm *qc.VM, entNum int, cmd string) {
			if client := clientForEntNum(entNum); client != nil && client.Message != nil {
				client.Message.WriteByte(byte(SVCStuffText))
				client.Message.WriteString(cmd)
			}
		},
		LightStyle: func(vm *qc.VM, style int, value string) {
			if s.Static == nil {
				return
			}
			for _, client := range s.Static.Clients {
				if client == nil || client.Message == nil {
					continue
				}
				client.Message.WriteByte(byte(SVCLightStyle))
				client.Message.WriteByte(byte(style))
				client.Message.WriteString(value)
			}
		},
		Particle: func(vm *qc.VM, org, dir [3]float32, color, count int) {
			s.StartParticle(org, dir, color, count)
		},
		LocalSound: func(vm *qc.VM, entNum int, sample string) {
			if client := clientForEntNum(entNum); client != nil && client.Message != nil {
				s.LocalSound(client, sample)
			}
		},
		WriteByte: func(vm *qc.VM, dest, value int) {
			for _, buf := range writeBuffers(vm, dest) {
				buf.WriteByte(byte(value))
			}
		},
		WriteChar: func(vm *qc.VM, dest, value int) {
			for _, buf := range writeBuffers(vm, dest) {
				buf.WriteChar(int8(value))
			}
		},
		WriteShort: func(vm *qc.VM, dest, value int) {
			for _, buf := range writeBuffers(vm, dest) {
				buf.WriteShort(int16(value))
			}
		},
		WriteLong: func(vm *qc.VM, dest int, value int32) {
			for _, buf := range writeBuffers(vm, dest) {
				buf.WriteLong(value)
			}
		},
		WriteCoord: func(vm *qc.VM, dest int, value float32) {
			for _, buf := range writeBuffers(vm, dest) {
				buf.WriteCoord(value)
			}
		},
		WriteAngle: func(vm *qc.VM, dest int, value float32) {
			for _, buf := range writeBuffers(vm, dest) {
				buf.WriteAngle(value)
			}
		},
		WriteString: func(vm *qc.VM, dest int, value string) {
			for _, buf := range writeBuffers(vm, dest) {
				buf.WriteString(value)
			}
		},
		WriteEntity: func(vm *qc.VM, dest, entNum int) {
			for _, buf := range writeBuffers(vm, dest) {
				buf.WriteShort(int16(entNum))
			}
		},
		SetSpawnParms: func(vm *qc.VM, entNum int) {
			if s.Static == nil {
				return
			}
			ent := s.EdictNum(entNum)
			if ent == nil {
				return
			}
			for _, client := range s.Static.Clients {
				if client == nil || client.Edict != ent {
					continue
				}
				for i := 0; i < len(client.SpawnParms); i++ {
					vm.SetGFloat(qc.OFSParmStart+i, client.SpawnParms[i])
				}
				return
			}
		},
		MakeStatic: func(vm *qc.VM, entNum int) {
			ent := s.EdictNum(entNum)
			if ent == nil || ent.Vars == nil || ent.Free {
				return
			}
			state := EntityState{
				Origin:     ent.Vars.Origin,
				Angles:     ent.Vars.Angles,
				ModelIndex: int(ent.Vars.ModelIndex),
				Frame:      int(ent.Vars.Frame),
				Colormap:   int(ent.Vars.Colormap),
				Skin:       int(ent.Vars.Skin),
				Effects:    int(ent.Vars.Effects),
				Alpha:      ent.Alpha,
				Scale:      ent.Scale,
			}
			if state.Scale == 0 {
				state.Scale = 16
			}
			s.StaticEntities = append(s.StaticEntities, state)
			if s.Static != nil {
				for _, client := range s.Static.Clients {
					if client == nil || !client.Active || client.Message == nil {
						continue
					}
					s.writeSpawnStaticMessage(client.Message, state)
				}
			}
			UnlinkEdict(ent)
			s.FreeEdict(ent)
		},
		AmbientSound: func(vm *qc.VM, org [3]float32, sample string, volume int, attenuation float32) {
			soundIndex := s.FindSound(sample)
			if soundIndex < 0 {
				return
			}
			st := StaticSound{
				Origin:      org,
				SoundIndex:  soundIndex,
				Volume:      volume,
				Attenuation: attenuation,
			}
			s.StaticSounds = append(s.StaticSounds, st)
			if s.Static != nil {
				for _, client := range s.Static.Clients {
					if client == nil || !client.Active || client.Message == nil {
						continue
					}
					s.writeSpawnStaticSoundMessage(client.Message, st)
				}
			}
		},
		MoveToGoal: func(vm *qc.VM, dist float32) {
			entNum := int(vm.GInt(qc.OFSSelf))
			ent := s.EdictNum(entNum)
			if ent == nil || ent.Vars == nil || ent.Free {
				return
			}
			s.MoveToGoal(ent, dist)
			syncEntityToVM(vm, entNum, ent)
		},
		ChangeYaw: func(vm *qc.VM) {
			entNum := int(vm.GInt(qc.OFSSelf))
			ent := s.EdictNum(entNum)
			if ent == nil || ent.Vars == nil || ent.Free {
				return
			}
			s.changeYaw(ent)
			syncEntityToVM(vm, entNum, ent)
		},
	})
	return s
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

	e := &Edict{Vars: &EntVars{}}
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
