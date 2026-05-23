package server

// Trace, precache builtins, and CheckClient hook tests split from server_hooks_test.go.

import (
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/console"
	"github.com/darkliquid/ironwail-go/internal/fs"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/testutil"
)

func TestServerHooksTraceContentsAndPrecacheBuiltins(t *testing.T) {
	s := NewServer()
	s.Datagram = NewMessageBuffer(MaxDatagram)
	s.Static = &ServerStatic{Clients: []*Client{{Active: true, Message: NewMessageBuffer(MaxDatagram)}}}
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}
	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "id1"); err != nil {
		t.Fatalf("init filesystem: %v", err)
	}
	defer fileSys.Close()
	s.FileSystem = fileSys

	s.WorldModel = CreateSyntheticWorldModel()
	if world := s.EdictNum(0); world != nil && world.Vars != nil {
		world.Vars.Solid = float32(SolidBSP)
	}

	e := s.AllocEdict()
	e.Vars.Origin = [3]float32{0, 0, 24}
	e.Vars.Mins = [3]float32{-16, -16, -24}
	e.Vars.Maxs = [3]float32{16, 16, 32}
	e.Vars.Solid = float32(SolidSlideBox)

	vm := newServerTestVM(s, 8)
	vm.NumEdicts = s.NumEdicts
	qc.RegisterBuiltins(vm)

	// traceline: from above ground into the floor.
	vm.SetGInt(qc.OFSSelf, int32(s.NumForEdict(e)))
	vm.SetGVector(qc.OFSParm0, [3]float32{0, 0, 32})
	vm.SetGVector(qc.OFSParm1, [3]float32{0, 0, -32})
	vm.SetGFloat(qc.OFSParm2, 0)
	vm.SetGInt(qc.OFSParm3, 0)
	if fn := vm.Builtins[16]; fn == nil {
		t.Fatal("traceline builtin not registered")
	} else {
		fn(vm)
	}
	if got := vm.GFloat(qc.OFSTraceFraction); got >= 1 {
		t.Fatalf("trace_fraction = %v, want < 1", got)
	}
	if got := vm.GVector(qc.OFSTraceEndPos); got[2] > DistEpsilon || got[2] < -DistEpsilon {
		t.Fatalf("trace_endpos.z = %v, want approximately 0", got[2])
	}
	if got := vm.GFloat(qc.OFSTracePlaneDist); got != 0 {
		t.Fatalf("trace_plane_dist = %v, want 0 for synthetic floor plane", got)
	}

	other := s.AllocEdict()
	other.Vars.Origin = [3]float32{0, 0, 24}
	other.Vars.Mins = [3]float32{-16, -16, -24}
	other.Vars.Maxs = [3]float32{16, 16, 32}
	other.Vars.Solid = float32(SolidSlideBox)
	s.LinkEdict(other, false)
	vm.NumEdicts = s.NumEdicts

	vm.SetGVector(qc.OFSParm0, [3]float32{0, 0, 48})
	vm.SetGVector(qc.OFSParm1, [3]float32{0, 0, 0})
	vm.SetGFloat(qc.OFSParm2, 0)
	vm.SetGInt(qc.OFSParm3, int32(s.NumForEdict(other)))
	if fn := vm.Builtins[16]; fn == nil {
		t.Fatal("traceline builtin not registered")
	} else {
		fn(vm)
	}
	if got := int(vm.GInt(qc.OFSTraceEnt)); got != 0 {
		t.Fatalf("trace_ent with explicit pass entity = %d, want world 0", got)
	}

	// checkbottom: entity resting on the synthetic plane should be supported.
	vm.SetGInt(qc.OFSParm0, int32(s.NumForEdict(e)))
	if fn := vm.Builtins[40]; fn == nil {
		t.Fatal("checkbottom builtin not registered")
	} else {
		fn(vm)
	}
	if got := vm.GFloat(qc.OFSReturn); got != 1 {
		t.Fatalf("checkbottom return = %v, want 1", got)
	}

	// pointcontents below the plane should be solid.
	vm.SetGVector(qc.OFSParm0, [3]float32{0, 0, -1})
	if fn := vm.Builtins[41]; fn == nil {
		t.Fatal("pointcontents builtin not registered")
	} else {
		fn(vm)
	}
	if got := int(vm.GFloat(qc.OFSReturn)); got == 0 { // pointcontents returns a float content type
		t.Fatalf("pointcontents returned empty contents for solid point")
	}

	// precache_sound/model should populate server lookup tables.
	vm.SetGString(qc.OFSParm0, "misc/menu1.wav")
	vm.Builtins[19](vm)
	if got := s.FindSound("misc/menu1.wav"); got < 0 {
		t.Fatalf("precache_sound did not register sample")
	}

	vm.SetGString(qc.OFSParm0, "progs/player.mdl")
	vm.Builtins[20](vm)
	if got := s.FindModel("progs/player.mdl"); got == 0 {
		t.Fatalf("precache_model did not register model")
	}

	// Attach client to the test entity for client-directed builtins.
	s.Static.Clients[0].Edict = e

	// sound and particle should write to the datagram.
	datagramBefore := s.Datagram.Len()
	vm.SetGInt(qc.OFSParm0, int32(s.NumForEdict(e)))
	vm.SetGFloat(qc.OFSParm1, 1)
	vm.SetGString(qc.OFSParm2, "misc/menu1.wav")
	vm.SetGFloat(qc.OFSParm3, DefaultSoundVolume)
	vm.SetGFloat(qc.OFSParm4, DefaultSoundAttenuation)
	vm.Builtins[8](vm)
	if s.Datagram.Len() <= datagramBefore {
		t.Fatalf("sound builtin did not write to datagram")
	}

	datagramBefore = s.Datagram.Len()
	vm.SetGVector(qc.OFSParm0, [3]float32{0, 0, 10})
	vm.SetGVector(qc.OFSParm1, [3]float32{0, 0, 1})
	vm.SetGFloat(qc.OFSParm2, 5)
	vm.SetGFloat(qc.OFSParm3, 8)
	vm.Builtins[48](vm)
	if s.Datagram.Len() <= datagramBefore {
		t.Fatalf("particle builtin did not write to datagram")
	}

	// client-targeted messaging should write into the client's reliable buffer.
	clientBefore := s.Static.Clients[0].Message.Len()
	vm.SetGInt(qc.OFSParm0, int32(s.NumForEdict(e)))
	vm.SetGString(qc.OFSParm1, "bf\n")
	vm.Builtins[21](vm)
	if s.Static.Clients[0].Message.Len() <= clientBefore {
		t.Fatalf("stuffcmd builtin did not write to client message")
	}
	if got := s.Static.Clients[0].Message.Data[clientBefore]; got != byte(inet.SVCStuffText) {
		t.Fatalf("stuffcmd opcode = %d, want %d", got, inet.SVCStuffText)
	}

	clientBefore = s.Static.Clients[0].Message.Len()
	vm.SetGFloat(qc.OFSParm0, 0)
	vm.SetGString(qc.OFSParm1, "m")
	vm.Builtins[35](vm)
	if s.Static.Clients[0].Message.Len() <= clientBefore {
		t.Fatalf("lightstyle builtin did not write to client message")
	}
	if got := s.Static.Clients[0].Message.Data[clientBefore]; got != byte(inet.SVCLightStyle) {
		t.Fatalf("lightstyle opcode = %d, want %d", got, inet.SVCLightStyle)
	}
	if got := s.LightStyles[0]; got != "m" {
		t.Fatalf("stored lightstyle = %q, want %q", got, "m")
	}

	clientBefore = s.Static.Clients[0].Message.Len()
	var centerConsole []string
	console.SetPrintCallback(func(msg string) {
		centerConsole = append(centerConsole, msg)
	})
	t.Cleanup(func() {
		console.SetPrintCallback(nil)
	})
	vm.SetGInt(qc.OFSParm0, int32(s.NumForEdict(e)))
	vm.SetGString(qc.OFSParm1, "centered")
	vm.Builtins[73](vm)
	if s.Static.Clients[0].Message.Len() <= clientBefore {
		t.Fatalf("centerprint builtin did not write to client message")
	}
	if got := s.Static.Clients[0].Message.Data[clientBefore]; got != byte(inet.SVCCenterPrint) {
		t.Fatalf("centerprint opcode = %d, want %d", got, inet.SVCCenterPrint)
	}
	msg := s.Static.Clients[0].Message.Data[clientBefore+1 : s.Static.Clients[0].Message.Len()-1]
	if got := string(msg); got != "centered" {
		t.Fatalf("centerprint message = %q, want %q", got, "centered")
	}
	if len(centerConsole) != 0 {
		t.Fatalf("centerprint unexpectedly echoed to server console: %q", centerConsole)
	}

	clientBefore = s.Static.Clients[0].Message.Len()
	vm.SetGInt(qc.OFSParm0, int32(s.NumForEdict(e)))
	vm.SetGString(qc.OFSParm1, `line1\nline2`)
	vm.Builtins[73](vm)
	if got := s.Static.Clients[0].Message.Data[clientBefore]; got != byte(inet.SVCCenterPrint) {
		t.Fatalf("escaped centerprint opcode = %d, want %d", got, inet.SVCCenterPrint)
	}
	msg = s.Static.Clients[0].Message.Data[clientBefore+1 : s.Static.Clients[0].Message.Len()-1]
	if got := string(msg); got != "line1\nline2" {
		t.Fatalf("escaped centerprint message = %q, want real newline payload", got)
	}
	if len(centerConsole) != 0 {
		t.Fatalf("escaped centerprint unexpectedly echoed to server console: %q", centerConsole)
	}

	clientBefore = s.Static.Clients[0].Message.Len()
	vm.SetGInt(qc.OFSParm0, int32(s.NumForEdict(e)))
	vm.SetGString(qc.OFSParm1, "misc/menu1.wav")
	vm.Builtins[80](vm)
	if s.Static.Clients[0].Message.Len() <= clientBefore {
		t.Fatalf("localsound builtin did not write to client message")
	}

	// Write* builtins: MSG_ONE should use msg_entity -> client message.
	vm.SetGInt(qc.OFSMsgEntity, int32(s.NumForEdict(e)))
	clientBefore = s.Static.Clients[0].Message.Len()
	vm.SetGFloat(qc.OFSParm0, 1)
	vm.SetGFloat(qc.OFSParm1, 42)
	vm.Builtins[52](vm)
	vm.Builtins[53](vm)
	vm.Builtins[54](vm)
	vm.Builtins[55](vm)
	vm.SetGFloat(qc.OFSParm1, 12.5)
	vm.Builtins[56](vm)
	vm.Builtins[57](vm)
	vm.SetGString(qc.OFSParm1, "hello")
	vm.Builtins[58](vm)
	vm.SetGInt(qc.OFSParm1, int32(s.NumForEdict(e)))
	vm.Builtins[59](vm)
	if s.Static.Clients[0].Message.Len() <= clientBefore {
		t.Fatalf("Write* builtins did not write to MSG_ONE buffer")
	}

	// MSG_BROADCAST should use the datagram.
	datagramBefore = s.Datagram.Len()
	vm.SetGFloat(qc.OFSParm0, 0)
	vm.SetGFloat(qc.OFSParm1, 7)
	vm.Builtins[52](vm)
	if s.Datagram.Len() <= datagramBefore {
		t.Fatalf("WriteByte builtin did not write to MSG_BROADCAST datagram")
	}

	// MSG_ALL should use reliable client messages for every connected client.
	client0Before := s.Static.Clients[0].Message.Len()
	client1 := &Client{Active: true, Message: NewMessageBuffer(MaxDatagram)}
	s.Static.Clients = append(s.Static.Clients, client1)
	vm.SetGFloat(qc.OFSParm0, 2)
	vm.SetGFloat(qc.OFSParm1, 9)
	vm.Builtins[52](vm)
	if s.Static.Clients[0].Message.Len() <= client0Before || client1.Message.Len() == 0 {
		t.Fatalf("WriteByte builtin did not write to MSG_ALL reliable buffers")
	}

	// MSG_INIT should use the signon buffer, not client reliable messages.
	s.Signon = NewMessageBuffer(SignonSize)
	signonBefore := s.Signon.Len()
	client0Before = s.Static.Clients[0].Message.Len()
	vm.SetGFloat(qc.OFSParm0, 3)
	vm.SetGFloat(qc.OFSParm1, 11)
	vm.Builtins[52](vm)
	if s.Signon.Len() <= signonBefore {
		t.Fatalf("WriteByte builtin did not write to MSG_INIT signon")
	}
	if s.Static.Clients[0].Message.Len() != client0Before {
		t.Fatalf("MSG_INIT unexpectedly wrote to client reliable buffer")
	}
}

func TestServerHooksCheckClientAimAndSetSpawnParms(t *testing.T) {
	s := NewServer()
	s.Datagram = NewMessageBuffer(MaxDatagram)
	s.WorldModel = CreateSyntheticWorldModel()
	if world := s.EdictNum(0); world != nil && world.Vars != nil {
		world.Vars.Solid = float32(SolidBSP)
	}
	s.ClearWorld()

	self := s.AllocEdict()
	target := s.AllocEdict()
	self.Vars.Origin = [3]float32{0, 0, 0}
	self.Vars.ViewOfs = [3]float32{0, 0, 16}
	target.Vars.Health = 100
	target.Vars.Origin = [3]float32{0, 100, 64}
	target.Vars.Mins = [3]float32{-16, -16, -24}
	target.Vars.Maxs = [3]float32{16, 16, 32}
	target.Vars.Solid = float32(SolidSlideBox)
	target.Vars.TakeDamage = float32(DamageAim)
	s.LinkEdict(self, false)
	s.LinkEdict(target, false)

	s.Static = &ServerStatic{Clients: []*Client{
		{Active: true, Message: NewMessageBuffer(MaxDatagram), Edict: self},
		{Active: true, Message: NewMessageBuffer(MaxDatagram), Edict: target},
	}}
	s.Static.Clients[1].SpawnParms[0] = 10
	s.Static.Clients[1].SpawnParms[1] = 20

	vm := newServerTestVM(s, 16)
	vm.NumEdicts = s.NumEdicts
	qc.RegisterBuiltins(vm)
	vm.SetGVector(qc.OFSGlobalVForward, [3]float32{0, 1, 0})
	s.syncEdictToQCVM(s.NumForEdict(self), self)
	s.syncEdictToQCVM(s.NumForEdict(target), target)

	vm.SetGInt(qc.OFSSelf, int32(s.NumForEdict(self)))
	if fn := vm.Builtins[17]; fn == nil {
		t.Fatal("checkclient builtin not registered")
	} else {
		fn(vm)
	}
	if got := int(vm.GInt(qc.OFSReturn)); got != s.NumForEdict(target) {
		t.Fatalf("checkclient = %d, want %d", got, s.NumForEdict(target))
	}

	vm.SetGInt(qc.OFSParm0, int32(s.NumForEdict(self)))
	vm.SetGFloat(qc.OFSParm1, 0)
	if fn := vm.Builtins[44]; fn == nil {
		t.Fatal("aim builtin not registered")
	} else {
		fn(vm)
	}
	aim := vm.GVector(qc.OFSReturn)
	if aim[1] <= 0.8 {
		t.Fatalf("aim vector = %v, want mostly +Y", aim)
	}

	s.CVar.Set("sv_aim", "0.99")
	s.CVar.Set("teamplay", "0")
	t.Cleanup(func() {
		s.CVar.Set("sv_aim", "0.93")
		s.CVar.Set("teamplay", "0")
	})
	target.Vars.Origin = [3]float32{40, 100, 64}
	s.LinkEdict(target, false)
	s.syncEdictToQCVM(s.NumForEdict(target), target)
	vm.SetGInt(qc.OFSParm0, int32(s.NumForEdict(self)))
	vm.SetGFloat(qc.OFSParm1, 0)
	vm.Builtins[44](vm)
	aim = vm.GVector(qc.OFSReturn)
	if aim != [3]float32{0, 1, 0} {
		t.Fatalf("high sv_aim should keep forward aim, got %v", aim)
	}

	s.CVar.Set("sv_aim", "0.5")
	s.CVar.Set("teamplay", "1")
	teammate := s.AllocEdict()
	teammate.Vars.Health = 100
	teammate.Vars.Origin = [3]float32{10, 100, 24}
	teammate.Vars.Mins = [3]float32{-16, -16, -24}
	teammate.Vars.Maxs = [3]float32{16, 16, 32}
	teammate.Vars.Solid = float32(SolidSlideBox)
	teammate.Vars.TakeDamage = float32(DamageAim)
	teammate.Vars.Team = 1
	self.Vars.Team = 1
	target.Vars.Team = 2
	s.LinkEdict(teammate, false)
	s.LinkEdict(target, false)
	vm.NumEdicts = s.NumEdicts
	s.syncEdictToQCVM(s.NumForEdict(teammate), teammate)
	s.syncEdictToQCVM(s.NumForEdict(target), target)
	vm.SetGInt(qc.OFSParm0, int32(s.NumForEdict(self)))
	vm.SetGFloat(qc.OFSParm1, 0)
	vm.Builtins[44](vm)
	aim = vm.GVector(qc.OFSReturn)
	if aim[1] <= 0.8 || aim[2] <= 0 {
		t.Fatalf("teamplay/sv_aim filtered aim = %v, want elevated enemy aim", aim)
	}

	vm.SetGInt(qc.OFSParm0, int32(s.NumForEdict(target)))
	if fn := vm.Builtins[78]; fn == nil {
		t.Fatal("setspawnparms builtin not registered")
	} else {
		fn(vm)
	}
	if got := vm.GFloat(qc.OFSParmStart); got != 10 {
		t.Fatalf("parm1 = %v, want 10", got)
	}
	if got := vm.GFloat(qc.OFSParmStart + 1); got != 20 {
		t.Fatalf("parm2 = %v, want 20", got)
	}
}

func TestServerHooksCheckClientRespectsPVS(t *testing.T) {
	s := NewServer()
	s.Datagram = NewMessageBuffer(MaxDatagram)

	self := s.AllocEdict()
	target := s.AllocEdict()
	self.Vars.Origin = [3]float32{-64, 0, 0}
	self.Vars.ViewOfs = [3]float32{}
	target.Vars.Origin = [3]float32{64, 0, 0}
	target.Vars.ViewOfs = [3]float32{}
	target.Vars.Health = 100

	s.Static = &ServerStatic{
		MaxClients: 2,
		Clients: []*Client{
			{Active: true, Message: NewMessageBuffer(MaxDatagram), Edict: self},
			{Active: true, Message: NewMessageBuffer(MaxDatagram), Edict: target},
		},
	}
	s.WorldTree = &bsp.Tree{
		Planes: []bsp.DPlane{{Normal: [3]float32{1, 0, 0}, Dist: 0, Type: 0}},
		Nodes: []bsp.TreeNode{{
			PlaneNum: 0,
			Children: [2]bsp.TreeChild{{IsLeaf: true, Index: 1}, {IsLeaf: true, Index: 2}},
		}},
		Leafs: []bsp.TreeLeaf{
			{Contents: bsp.ContentsSolid, VisOfs: -1},
			{Contents: 0, VisOfs: 0},
			{Contents: 0, VisOfs: 1},
		},
		Visibility: []byte{0x01, 0x02},
		Models:     []bsp.DModel{{VisLeafs: 2}},
	}

	vm := newServerTestVM(s, 16)
	vm.NumEdicts = s.NumEdicts
	qc.RegisterBuiltins(vm)
	s.syncEdictToQCVM(s.NumForEdict(self), self)
	s.syncEdictToQCVM(s.NumForEdict(target), target)

	s.Time = 0.2
	vm.SetGInt(qc.OFSSelf, int32(s.NumForEdict(self)))
	if fn := vm.Builtins[17]; fn == nil {
		t.Fatal("checkclient builtin not registered")
	} else {
		fn(vm)
	}
	if got := int(vm.GInt(qc.OFSReturn)); got != 0 {
		t.Fatalf("checkclient with self outside target PVS = %d, want 0", got)
	}

	self.Vars.Origin = [3]float32{64, 0, 0}
	s.syncEdictToQCVM(s.NumForEdict(self), self)
	s.Time = 0.25
	if fn := vm.Builtins[17]; fn == nil {
		t.Fatal("checkclient builtin not registered")
	} else {
		fn(vm)
	}
	if got := int(vm.GInt(qc.OFSReturn)); got != s.NumForEdict(target) {
		t.Fatalf("checkclient with self inside target PVS = %d, want %d", got, s.NumForEdict(target))
	}
}

func TestServerHooksCheckClientUsesVisLeafNumbering(t *testing.T) {
	s := NewServer()
	s.Datagram = NewMessageBuffer(MaxDatagram)

	self := s.AllocEdict()
	target := s.AllocEdict()
	self.Vars.ViewOfs = [3]float32{}
	target.Vars.ViewOfs = [3]float32{}
	target.Vars.Health = 100

	s.Static = &ServerStatic{
		MaxClients: 2,
		Clients: []*Client{
			{Active: true, Message: NewMessageBuffer(MaxDatagram), Edict: self},
			{Active: true, Message: NewMessageBuffer(MaxDatagram), Edict: target},
		},
	}
	s.WorldTree = &bsp.Tree{
		Planes: []bsp.DPlane{{Normal: [3]float32{1, 0, 0}, Dist: 0, Type: 0}},
		Nodes: []bsp.TreeNode{{
			PlaneNum: 0,
			Children: [2]bsp.TreeChild{{IsLeaf: true, Index: 1}, {IsLeaf: true, Index: 0}},
		}},
		Leafs:      []bsp.TreeLeaf{{Contents: bsp.ContentsSolid, VisOfs: -1}, {Contents: 0, VisOfs: 0}},
		Visibility: []byte{0x01},
		Models:     []bsp.DModel{{VisLeafs: 1}},
	}

	vm := newServerTestVM(s, 16)
	vm.NumEdicts = s.NumEdicts
	qc.RegisterBuiltins(vm)
	s.syncEdictToQCVM(s.NumForEdict(self), self)
	s.syncEdictToQCVM(s.NumForEdict(target), target)

	// Both entities resolve to BSP leaf index 1, which must map to visleaf 0.
	self.Vars.Origin = [3]float32{1, 0, 0}
	target.Vars.Origin = [3]float32{1, 0, 0}

	s.Time = 0.2
	vm.SetGInt(qc.OFSSelf, int32(s.NumForEdict(self)))
	if fn := vm.Builtins[17]; fn == nil {
		t.Fatal("checkclient builtin not registered")
	} else {
		fn(vm)
	}
	if got := int(vm.GInt(qc.OFSReturn)); got != s.NumForEdict(target) {
		t.Fatalf("checkclient = %d, want %d (visleaf 0 should be visible)", got, s.NumForEdict(target))
	}
}

func TestServerHooksCheckClientImportsPendingQCState(t *testing.T) {
	s := NewServer()
	s.Datagram = NewMessageBuffer(MaxDatagram)

	self := s.AllocEdict()
	target := s.AllocEdict()
	self.Vars.Origin = [3]float32{-64, 0, 0}
	self.Vars.ViewOfs = [3]float32{}
	target.Vars.Origin = [3]float32{64, 0, 0}
	target.Vars.ViewOfs = [3]float32{}
	target.Vars.Health = 100

	s.Static = &ServerStatic{
		MaxClients: 2,
		Clients: []*Client{
			{Active: true, Message: NewMessageBuffer(MaxDatagram), Edict: self},
			{Active: true, Message: NewMessageBuffer(MaxDatagram), Edict: target},
		},
	}
	s.WorldTree = &bsp.Tree{
		Planes: []bsp.DPlane{{Normal: [3]float32{1, 0, 0}, Dist: 0, Type: 0}},
		Nodes: []bsp.TreeNode{{
			PlaneNum: 0,
			Children: [2]bsp.TreeChild{{IsLeaf: true, Index: 1}, {IsLeaf: true, Index: 2}},
		}},
		Leafs: []bsp.TreeLeaf{
			{Contents: bsp.ContentsSolid, VisOfs: -1},
			{Contents: 0, VisOfs: 0},
			{Contents: 0, VisOfs: 1},
		},
		Visibility: []byte{0x01, 0x02},
		Models:     []bsp.DModel{{VisLeafs: 2}},
	}

	vm := newServerTestVM(s, 16)
	vm.NumEdicts = s.NumEdicts
	qc.RegisterBuiltins(vm)

	selfNum := s.NumForEdict(self)
	targetNum := s.NumForEdict(target)
	s.syncEdictToQCVM(selfNum, self)
	s.syncEdictToQCVM(targetNum, target)
	vm.SetEVector(selfNum, qc.EntFieldOrigin, [3]float32{64, 0, 0})

	s.Time = 0.2
	vm.SetGInt(qc.OFSSelf, int32(selfNum))
	if fn := vm.Builtins[17]; fn == nil {
		t.Fatal("checkclient builtin not registered")
	} else {
		fn(vm)
	}
	if got := int(vm.GInt(qc.OFSReturn)); got != targetNum {
		t.Fatalf("checkclient with QC-only self origin = %d, want %d", got, targetNum)
	}
	if got := self.Vars.Origin; got != [3]float32{64, 0, 0} {
		t.Fatalf("server self origin not synchronized from QC state: got %v", got)
	}
}
