// This file belongs to the Tests subsystem: unit, integration, parity, and e2e tests for the server package.

package server

// Trace, precache builtins, and CheckClient hook tests split from server_hooks_test.go.

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/console"
	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/loc"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/testutil"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
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

	// Set up the VM edict storage up front. newServerTestVM reallocates the
	// VM edict byte array; doing it after entities are configured would wipe
	// their QCVM-backed fields (e.g. the world's SOLID_BSP), breaking
	// collision.
	vm := newServerTestVM(s, 16)
	vm.SetServerHooks(s.QCVM.ServerHooks)
	qc.RegisterBuiltins(vm)

	s.WorldModel = CreateSyntheticWorldModel()
	s.State = ServerStateLoading
	s.ClearWorld()
	if world := s.EdictNum(0); world != nil {
		world.SetSolid(s, float32(SolidBSP))
	}

	e := s.AllocEdict()
	e.SetOrigin(s, qtypes.Vec3{X: 0, Y: 0, Z: 24})
	e.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -24})
	e.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 32})
	e.SetSolid(s, float32(SolidSlideBox))
	vm.NumEdicts = s.NumEdicts

	// traceline: from above ground into the floor.
	vm.SetGInt(qc.OFSSelf, int32(s.NumForEdict(e)))
	vm.SetGVector(qc.OFSParm0, qtypes.Vec3{X: 0, Y: 0, Z: 32})
	vm.SetGVector(qc.OFSParm1, qtypes.Vec3{X: 0, Y: 0, Z: -32})
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
	if got := vm.GVector(qc.OFSTraceEndPos); got.Z > DistEpsilon || got.Z < -DistEpsilon {
		t.Fatalf("trace_endpos.z = %v, want approximately 0", got.Z)
	}
	if got := vm.GFloat(qc.OFSTracePlaneDist); got != 0 {
		t.Fatalf("trace_plane_dist = %v, want 0 for synthetic floor plane", got)
	}

	other := s.AllocEdict()
	other.SetOrigin(s, qtypes.Vec3{X: 0, Y: 0, Z: 24})
	other.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -24})
	other.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 32})
	other.SetSolid(s, float32(SolidSlideBox))
	s.LinkEdict(other, false)
	vm.NumEdicts = s.NumEdicts

	vm.SetGVector(qc.OFSParm0, qtypes.Vec3{X: 0, Y: 0, Z: 48})
	vm.SetGVector(qc.OFSParm1, qtypes.Vec3{})
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
	vm.SetGVector(qc.OFSParm0, qtypes.Vec3{X: 0, Y: 0, Z: -1})
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
	// The sound builtin scales parm3 by 255 (volume is a 0..1 float); pass 1.0
	// for full volume (DefaultSoundVolume is already the scaled byte value).
	vm.SetGFloat(qc.OFSParm3, 1.0)
	vm.SetGFloat(qc.OFSParm4, DefaultSoundAttenuation)
	vm.Builtins[8](vm)
	if s.Datagram.Len() <= datagramBefore {
		t.Fatalf("sound builtin did not write to datagram")
	}

	datagramBefore = s.Datagram.Len()
	vm.SetGVector(qc.OFSParm0, qtypes.Vec3{X: 0, Y: 0, Z: 10})
	vm.SetGVector(qc.OFSParm1, qtypes.Vec3{X: 0, Y: 0, Z: 1})
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
	locSample := `map_skill_normal = "This hall selects NORMAL skill"`
	l := loc.New()
	_ = l.Load(strings.NewReader(locSample))
	oldLoc := loc.Default()
	loc.SetDefault(l)
	t.Cleanup(func() {
		loc.SetDefault(oldLoc)
	})

	vm.SetGInt(qc.OFSParm0, int32(s.NumForEdict(e)))
	vm.SetGString(qc.OFSParm1, "$map_skill_normal")
	vm.Builtins[73](vm)
	if got := s.Static.Clients[0].Message.Data[clientBefore]; got != byte(inet.SVCCenterPrint) {
		t.Fatalf("localized centerprint opcode = %d, want %d", got, inet.SVCCenterPrint)
	}
	msg = s.Static.Clients[0].Message.Data[clientBefore+1 : s.Static.Clients[0].Message.Len()-1]
	if got := string(msg); got != "This hall selects NORMAL skill" {
		t.Fatalf("localized centerprint message = %q, want %q", got, "This hall selects NORMAL skill")
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

	// MSG_ALL should use the shared reliable datagram buffer.
	reliableBefore := s.ReliableDatagram.Len()
	vm.SetGFloat(qc.OFSParm0, 2)
	vm.SetGFloat(qc.OFSParm1, 9)
	vm.Builtins[52](vm)
	if s.ReliableDatagram.Len() <= reliableBefore {
		t.Fatalf("WriteByte builtin did not write to MSG_ALL reliable buffer")
	}

	// MSG_INIT should use the signon buffer, not client reliable messages.
	s.Signon = NewMessageBuffer(SignonSize)
	signonBefore := s.Signon.Len()
	client0Before := s.Static.Clients[0].Message.Len()
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
	newServerTestVM(s, 16)
	s.Datagram = NewMessageBuffer(MaxDatagram)
	s.WorldModel = CreateSyntheticWorldModel()
	if world := s.EdictNum(0); world != nil {
		world.SetSolid(s, float32(SolidBSP))
	}
	s.ClearWorld()

	self := s.AllocEdict()
	target := s.AllocEdict()
	self.SetOrigin(s, qtypes.Vec3{})
	self.SetViewOfs(s, qtypes.Vec3{X: 0, Y: 0, Z: 16})
	target.SetHealth(s, 100)
	target.SetOrigin(s, qtypes.Vec3{X: 0, Y: 100, Z: 64})
	target.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -24})
	target.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 32})
	target.SetSolid(s, float32(SolidSlideBox))
	target.SetTakeDamage(s, float32(DamageAim))
	s.LinkEdict(self, false)
	s.LinkEdict(target, false)

	s.Static = &ServerStatic{Clients: []*Client{
		{Active: true, Message: NewMessageBuffer(MaxDatagram), Edict: self},
		{Active: true, Message: NewMessageBuffer(MaxDatagram), Edict: target},
	}}
	s.Static.Clients[1].SpawnParms[0] = 10
	s.Static.Clients[1].SpawnParms[1] = 20

	vm := s.QCVM
	vm.NumEdicts = s.NumEdicts
	qc.RegisterBuiltins(vm)
	vm.SetGVector(qc.OFSGlobalVForward, qtypes.Vec3{X: 0, Y: 1, Z: 0})

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
	if aim.Y <= 0.8 {
		t.Fatalf("aim vector = %v, want mostly +Y", aim)
	}

	s.CVar.Set("sv_aim", "0.99")
	s.CVar.Set("teamplay", "0")
	t.Cleanup(func() {
		s.CVar.Set("sv_aim", "0.93")
		s.CVar.Set("teamplay", "0")
	})
	target.SetOrigin(s, qtypes.Vec3{X: 40, Y: 100, Z: 64})
	s.LinkEdict(target, false)
	vm.SetGInt(qc.OFSParm0, int32(s.NumForEdict(self)))
	vm.SetGFloat(qc.OFSParm1, 0)
	vm.Builtins[44](vm)
	aim = vm.GVector(qc.OFSReturn)
	if aim != (qtypes.Vec3{X: 0, Y: 1, Z: 0}) {
		t.Fatalf("high sv_aim should keep forward aim, got %v", aim)
	}

	s.CVar.Set("sv_aim", "0.5")
	s.CVar.Set("teamplay", "1")
	teammate := s.AllocEdict()
	teammate.SetHealth(s, 100)
	teammate.SetOrigin(s, qtypes.Vec3{X: 10, Y: 100, Z: 24})
	teammate.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -24})
	teammate.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 32})
	teammate.SetSolid(s, float32(SolidSlideBox))
	teammate.SetTakeDamage(s, float32(DamageAim))
	teammate.SetTeam(s, 1)
	self.SetTeam(s, 1)
	target.SetTeam(s, 2)
	s.LinkEdict(teammate, false)
	s.LinkEdict(target, false)
	vm.NumEdicts = s.NumEdicts
	vm.SetGInt(qc.OFSParm0, int32(s.NumForEdict(self)))
	vm.SetGFloat(qc.OFSParm1, 0)
	vm.Builtins[44](vm)
	aim = vm.GVector(qc.OFSReturn)
	if aim.Y <= 0.8 || aim.Z <= 0 {
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
	newServerTestVM(s, 16)
	s.Datagram = NewMessageBuffer(MaxDatagram)

	self := s.AllocEdict()
	target := s.AllocEdict()
	self.SetOrigin(s, qtypes.Vec3{X: -64, Y: 0, Z: 0})
	self.SetViewOfs(s, qtypes.Vec3{})
	target.SetOrigin(s, qtypes.Vec3{X: 64, Y: 0, Z: 0})
	target.SetViewOfs(s, qtypes.Vec3{})
	target.SetHealth(s, 100)

	s.Static = &ServerStatic{
		MaxClients: 2,
		Clients: []*Client{
			{Active: true, Message: NewMessageBuffer(MaxDatagram), Edict: self},
			{Active: true, Message: NewMessageBuffer(MaxDatagram), Edict: target},
		},
	}
	s.WorldTree = &bsp.Tree{
		Planes: []bsp.DPlane{{Normal: qtypes.Vec3{X: 1, Y: 0, Z: 0}, Dist: 0, Type: 0}},
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

	vm := s.QCVM
	vm.NumEdicts = s.NumEdicts
	qc.RegisterBuiltins(vm)

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

	self.SetOrigin(s, qtypes.Vec3{X: 64, Y: 0, Z: 0})
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
	newServerTestVM(s, 16)
	s.Datagram = NewMessageBuffer(MaxDatagram)

	self := s.AllocEdict()
	target := s.AllocEdict()
	self.SetViewOfs(s, qtypes.Vec3{})
	target.SetViewOfs(s, qtypes.Vec3{})
	target.SetHealth(s, 100)

	s.Static = &ServerStatic{
		MaxClients: 2,
		Clients: []*Client{
			{Active: true, Message: NewMessageBuffer(MaxDatagram), Edict: self},
			{Active: true, Message: NewMessageBuffer(MaxDatagram), Edict: target},
		},
	}
	s.WorldTree = &bsp.Tree{
		Planes: []bsp.DPlane{{Normal: qtypes.Vec3{X: 1, Y: 0, Z: 0}, Dist: 0, Type: 0}},
		Nodes: []bsp.TreeNode{{
			PlaneNum: 0,
			Children: [2]bsp.TreeChild{{IsLeaf: true, Index: 1}, {IsLeaf: true, Index: 0}},
		}},
		Leafs:      []bsp.TreeLeaf{{Contents: bsp.ContentsSolid, VisOfs: -1}, {Contents: 0, VisOfs: 0}},
		Visibility: []byte{0x01},
		Models:     []bsp.DModel{{VisLeafs: 1}},
	}

	vm := s.QCVM
	vm.NumEdicts = s.NumEdicts
	qc.RegisterBuiltins(vm)

	// Both entities resolve to BSP leaf index 1, which must map to visleaf 0.
	self.SetOrigin(s, qtypes.Vec3{X: 1, Y: 0, Z: 0})
	target.SetOrigin(s, qtypes.Vec3{X: 1, Y: 0, Z: 0})

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
	newServerTestVM(s, 16)
	s.Datagram = NewMessageBuffer(MaxDatagram)

	self := s.AllocEdict()
	target := s.AllocEdict()
	self.SetOrigin(s, qtypes.Vec3{X: -64, Y: 0, Z: 0})
	self.SetViewOfs(s, qtypes.Vec3{})
	target.SetOrigin(s, qtypes.Vec3{X: 64, Y: 0, Z: 0})
	target.SetViewOfs(s, qtypes.Vec3{})
	target.SetHealth(s, 100)

	s.Static = &ServerStatic{
		MaxClients: 2,
		Clients: []*Client{
			{Active: true, Message: NewMessageBuffer(MaxDatagram), Edict: self},
			{Active: true, Message: NewMessageBuffer(MaxDatagram), Edict: target},
		},
	}
	s.WorldTree = &bsp.Tree{
		Planes: []bsp.DPlane{{Normal: qtypes.Vec3{X: 1, Y: 0, Z: 0}, Dist: 0, Type: 0}},
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

	vm := s.QCVM
	vm.NumEdicts = s.NumEdicts
	qc.RegisterBuiltins(vm)

	selfNum := s.NumForEdict(self)
	targetNum := s.NumForEdict(target)
	vm.SetEVector(selfNum, qc.EntFieldOrigin, qtypes.Vec3{X: 64, Y: 0, Z: 0})

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
	if got := self.Origin(s); got != (qtypes.Vec3{X: 64, Y: 0, Z: 0}) {
		t.Fatalf("server self origin not synchronized from QC state: got %v", got)
	}
}
