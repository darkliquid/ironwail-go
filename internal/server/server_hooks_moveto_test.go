// This file belongs to the Tests subsystem: unit, integration, parity, and e2e tests for the server package.

package server

// MakeStatic/AmbientSound, precache validation, and MoveToGoal hook tests split from server_hooks_test.go.

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/model"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/qc"
)

func TestServerHooksMakeStaticAndAmbientSound(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
	s.Datagram = NewMessageBuffer(MaxDatagram)
	clientMsg := NewMessageBuffer(MaxDatagram)
	world := s.EdictNum(0)
	if world == nil {
		t.Fatal("missing world edict")
	}
	s.Static = &ServerStatic{Clients: []*Client{{Active: true, Message: clientMsg, Edict: world}}}
	s.SoundPrecache = make([]string, MaxSounds)
	s.SoundPrecache[1] = "ambience/drip.wav"

	ent := s.AllocEdict()
	ent.SetOrigin(s, [3]float32{1, 2, 3})
	ent.SetAngles(s, [3]float32{0, 90, 0})
	ent.SetModelIndex(s, 7)
	ent.SetFrame(s, 2)
	ent.SetColormap(s, 3)
	ent.SetSkin(s, 4)

	vm := newServerTestVM(s, 16)
	vm.NumEdicts = s.NumEdicts
	qc.RegisterBuiltins(vm)

	before := clientMsg.Len()
	vm.SetGInt(qc.OFSParm0, int32(s.NumForEdict(ent)))
	if fn := vm.Builtins[69]; fn == nil {
		t.Fatal("makestatic builtin not registered")
	} else {
		fn(vm)
	}
	if got := len(s.StaticEntities); got != 1 {
		t.Fatalf("static entities len = %d, want 1", got)
	}
	if !ent.Free {
		t.Fatalf("entity not freed after makestatic")
	}
	if clientMsg.Len() <= before {
		t.Fatalf("makestatic did not write to client message")
	}

	invisible := s.AllocEdict()
	invisible.Alpha = inet.ENTALPHA_ZERO
	vm.NumEdicts = s.NumEdicts
	before = clientMsg.Len()
	vm.SetGInt(qc.OFSParm0, int32(s.NumForEdict(invisible)))
	vm.Builtins[69](vm)
	if got := len(s.StaticEntities); got != 1 {
		t.Fatalf("invisible makestatic changed static entity count to %d, want 1", got)
	}
	if clientMsg.Len() != before {
		t.Fatalf("invisible makestatic wrote unexpected client message")
	}

	s.Protocol = ProtocolNetQuake
	unsupported := s.AllocEdict()
	unsupported.SetModelIndex(s, 300)
	unsupported.SetFrame(s, 2)
	vm.NumEdicts = s.NumEdicts
	before = clientMsg.Len()
	vm.SetGInt(qc.OFSParm0, int32(s.NumForEdict(unsupported)))
	vm.Builtins[69](vm)
	if got := len(s.StaticEntities); got != 1 {
		t.Fatalf("unsupported netquake makestatic changed static entity count to %d, want 1", got)
	}
	if clientMsg.Len() != before {
		t.Fatalf("unsupported netquake makestatic wrote unexpected client message")
	}

	before = clientMsg.Len()
	vm.SetGVector(qc.OFSParm0, [3]float32{4, 5, 6})
	vm.SetGString(qc.OFSParm1, "ambience/drip.wav")
	vm.SetGFloat(qc.OFSParm2, 255)
	vm.SetGFloat(qc.OFSParm3, 1)
	if fn := vm.Builtins[74]; fn == nil {
		t.Fatal("ambientsound builtin not registered")
	} else {
		fn(vm)
	}
	if got := len(s.StaticSounds); got != 1 {
		t.Fatalf("static sounds len = %d, want 1", got)
	}
	if clientMsg.Len() <= before {
		t.Fatalf("ambientsound did not write to client message")
	}

	newClient := &Client{Edict: world, Message: NewMessageBuffer(MaxDatagram)}
	s.SendServerInfo(newClient)
	// Static entities and sounds are now in signon buffers (populated during
	// SpawnServer). Build them here to simulate the full flow.
	if err := s.buildSignonBuffers(); err != nil {
		t.Fatalf("buildSignonBuffers: %v", err)
	}
	s.SendSignonBuffers(newClient)
	if newClient.Message.Len() == 0 {
		t.Fatalf("SendServerInfo did not produce signon message")
	}
	data := newClient.Message.Data[:newClient.Message.Len()]
	foundStatic := false
	foundAmbient := false
	for _, b := range data {
		if b == byte(inet.SVCSpawnStatic) || b == byte(inet.SVCSpawnStatic2) {
			foundStatic = true
		}
		if b == byte(inet.SVCSpawnStaticSound) || b == byte(inet.SVCSpawnStaticSound2) {
			foundAmbient = true
		}
	}
	if !foundStatic {
		t.Fatalf("SendServerInfo missing spawnstatic message")
	}
	if !foundAmbient {
		t.Fatalf("SendServerInfo missing spawnstaticsound message")
	}
}

func writeTestSprite(t *testing.T, path string, width, height int32) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir sprite dir: %v", err)
	}
	var buf bytes.Buffer
	write := func(v any) {
		if err := binary.Write(&buf, binary.LittleEndian, v); err != nil {
			t.Fatalf("write sprite data: %v", err)
		}
	}
	write(int32(0x50534449))
	write(int32(1))
	write(int32(0))
	write(float32(0))
	write(width)
	write(height)
	write(int32(1))
	write(float32(0))
	write(int32(0))
	write(int32(0))
	write(int32(0))
	write(int32(0))
	write(width)
	write(height)
	pixels := make([]byte, int(width*height))
	if _, err := buf.Write(pixels); err != nil {
		t.Fatalf("write sprite pixels: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write sprite file: %v", err)
	}
}

func writeTestAliasModel(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir alias dir: %v", err)
	}
	var buf bytes.Buffer
	write := func(v any) {
		if err := binary.Write(&buf, binary.LittleEndian, v); err != nil {
			t.Fatalf("write alias data: %v", err)
		}
	}
	write(model.MDLHeader{
		Ident:       model.MDLIdent,
		Version:     model.MDLVersion,
		Scale:       [3]float32{2, 3, 4},
		ScaleOrigin: [3]float32{-1, -2, -3},
		NumSkins:    1,
		SkinWidth:   1,
		SkinHeight:  1,
		NumVerts:    3,
		NumTris:     1,
		NumFrames:   1,
	})
	write(model.DAliasSkinType{Type: int32(model.AliasSkinSingle)})
	if err := buf.WriteByte(7); err != nil {
		t.Fatalf("write alias skin: %v", err)
	}
	write([3]model.STVert{})
	write(model.DTriangle{FacesFront: model.MDLFacesFront, VertIndex: [3]int32{0, 1, 2}})
	write(model.DAliasFrameType{Type: int32(model.AliasSingle)})
	write(model.DAliasFrame{})
	write([3]model.TriVertX{{V: [3]byte{0, 0, 0}}, {V: [3]byte{2, 0, 0}}, {V: [3]byte{0, 3, 1}}})
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write alias file: %v", err)
	}
}

func TestServerHooksPrecacheValidationAndSetModelNonBrushBounds(t *testing.T) {
	s := NewServer()

	vm := newServerTestVM(s, 8)
	qc.RegisterBuiltins(vm)
	ent := s.AllocEdict()
	vm.NumEdicts = s.NumEdicts
	entNum := s.NumForEdict(ent)

	vm.SetGString(qc.OFSParm0, "")
	vm.Builtins[19](vm)
	if vm.BuiltinError == nil {
		t.Fatal("precache_sound empty string did not raise runtime error")
	}
	vm.BuiltinError = nil

	s.State = ServerStateActive
	vm.SetGString(qc.OFSParm0, "misc/menu1.wav")
	vm.Builtins[19](vm)
	if vm.BuiltinError == nil {
		t.Fatal("precache_sound outside spawn did not raise runtime error")
	}
	vm.BuiltinError = nil

	tmpDir := t.TempDir()
	writeTestSprite(t, filepath.Join(tmpDir, "id1", "progs", "test.spr"), 8, 6)
	writeTestAliasModel(t, filepath.Join(tmpDir, "id1", "progs", "test.mdl"))
	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(tmpDir, "id1"); err != nil {
		t.Fatalf("init filesystem: %v", err)
	}
	defer fileSys.Close()

	s.FileSystem = fileSys
	s.State = ServerStateLoading
	vm.SetGString(qc.OFSParm0, "progs/test.spr")
	vm.Builtins[20](vm)
	if vm.BuiltinError != nil {
		t.Fatalf("precache_model runtime error = %v", vm.BuiltinError)
	}
	if got := s.FindModel("progs/test.spr"); got == 0 {
		t.Fatal("precache_model did not register sprite model")
	}

	vm.SetGInt(qc.OFSParm0, int32(entNum))
	vm.SetGString(qc.OFSParm1, "progs/test.spr")
	vm.Builtins[3](vm)
	if got := vm.EVector(entNum, qc.EntFieldMins); got != [3]float32{-4, -4, -3} {
		t.Fatalf("sprite mins = %v", got)
	}
	if got := vm.EVector(entNum, qc.EntFieldMaxs); got != [3]float32{4, 4, 3} {
		t.Fatalf("sprite maxs = %v", got)
	}

	s.ModelPrecache = make([]string, MaxModels)
	s.ModelPrecache[1] = "progs/test.mdl"
	vm.SetGInt(qc.OFSParm0, int32(entNum))
	vm.SetGString(qc.OFSParm1, "progs/test.mdl")
	vm.Builtins[3](vm)
	if vm.BuiltinError != nil {
		t.Fatalf("setmodel alias runtime error = %v", vm.BuiltinError)
	}
	if got := vm.EVector(entNum, qc.EntFieldMins); got != [3]float32{-1, -2, -3} {
		t.Fatalf("alias mins = %v", got)
	}
	if got := vm.EVector(entNum, qc.EntFieldMaxs); got != [3]float32{3, 7, 1} {
		t.Fatalf("alias maxs = %v", got)
	}

	s.SoundPrecache = make([]string, MaxSounds)
	for i := 1; i < len(s.SoundPrecache); i++ {
		s.SoundPrecache[i] = "filled"
	}
	vm.SetGString(qc.OFSParm0, "misc/overflow.wav")
	vm.Builtins[19](vm)
	if vm.BuiltinError == nil {
		t.Fatal("precache_sound overflow did not raise runtime error")
	}
}

func TestServerHooksMoveToGoalImportsPendingSelfState(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)

	s.WorldModel = CreateSyntheticWorldModel()
	if world := s.EdictNum(0); world != nil {
		world.SetSolid(s, float32(SolidBSP))
	}
	s.ClearWorld()

	self := s.AllocEdict()
	goal := s.AllocEdict()
	self.SetOrigin(s, [3]float32{0, 0, 16})
	self.SetMins(s, [3]float32{-1, -1, 0})
	self.SetMaxs(s, [3]float32{1, 1, 56})
	self.SetSolid(s, float32(SolidBSP))
	self.SetFlags(s, 0)
	self.SetIdealYaw(s, 0)
	self.SetYawSpeed(s, 360)
	goal.SetOrigin(s, [3]float32{64, 0, 16})
	goal.SetMins(s, [3]float32{-1, -1, 0})
	goal.SetMaxs(s, [3]float32{1, 1, 56})
	self.SetGoalEntity(s, int32(s.NumForEdict(goal)))

	s.LinkEdict(self, false)
	s.LinkEdict(goal, false)

	vm := s.QCVM
	vm.NumEdicts = s.NumEdicts
	qc.RegisterBuiltins(vm)
	s.syncEdictToQCVM(0, s.Edicts[0])

	selfNum := s.NumForEdict(self)
	vm.SetGInt(qc.OFSSelf, int32(selfNum))
	s.syncEdictToQCVM(selfNum, self)
	s.syncEdictToQCVM(s.NumForEdict(goal), goal)

	vm.SetEFloat(selfNum, qc.EntFieldFlags, float32(FlagOnGround))
	vm.SetGFloat(qc.OFSParm0, 16)
	if fn := vm.Builtins[67]; fn == nil {
		t.Fatal("movetogoal builtin not registered")
	} else {
		fn(vm)
	}
	if got := self.Origin(s)[0]; got <= 0 {
		t.Fatalf("movetogoal did not use QC-only movement flags: origin=%v", self.Origin(s))
	}
	if got := vm.EVector(selfNum, qc.EntFieldOrigin); got != self.Origin(s) {
		t.Fatalf("vm origin not synchronized after movetogoal: got=%v want=%v", got, self.Origin(s))
	}
}

func TestServerHooksMoveToGoalImportsPendingQCGoalEdict(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)

	s.WorldModel = CreateSyntheticWorldModel()
	if world := s.EdictNum(0); world != nil {
		world.SetSolid(s, float32(SolidBSP))
	}
	s.ClearWorld()

	self := s.AllocEdict()
	goal := s.AllocEdict()
	self.SetOrigin(s, [3]float32{0, 0, 16})
	self.SetMins(s, [3]float32{-1, -1, 0})
	self.SetMaxs(s, [3]float32{1, 1, 56})
	self.SetSolid(s, float32(SolidBSP))
	self.SetFlags(s, float32(FlagOnGround))
	self.SetIdealYaw(s, 0)
	self.SetYawSpeed(s, 360)
	goal.SetOrigin(s, [3]float32{-64, 0, 16})
	goal.SetMins(s, [3]float32{-1, -1, 0})
	goal.SetMaxs(s, [3]float32{1, 1, 56})
	self.SetGoalEntity(s, int32(s.NumForEdict(goal)))

	s.LinkEdict(self, false)
	s.LinkEdict(goal, false)

	vm := newServerTestVM(s, 16)
	vm.NumEdicts = s.NumEdicts
	qc.RegisterBuiltins(vm)

	selfNum := s.NumForEdict(self)
	goalNum := s.NumForEdict(goal)
	vm.SetGInt(qc.OFSSelf, int32(selfNum))
	s.syncEdictToQCVM(selfNum, self)
	s.syncEdictToQCVM(goalNum, goal)
	vm.SetEVector(goalNum, qc.EntFieldOrigin, [3]float32{64, 0, 16})

	if fn := vm.Builtins[67]; fn == nil {
		t.Fatal("movetogoal builtin not registered")
	} else {
		fn(vm)
	}

	if got := goal.Origin(s); got != [3]float32{64, 0, 16} {
		t.Fatalf("movetogoal did not import QC-only goal edict origin: %v", got)
	}
}

func TestServerHooksChangeYawImportsPendingQCState(t *testing.T) {
	s := NewServer()

	vm := newServerTestVM(s, 16)
	qc.RegisterBuiltins(vm)

	ent := s.AllocEdict()
	if ent == nil {
		t.Fatal("AllocEdict returned nil")
	}
	entNum := s.NumForEdict(ent)
	vm.NumEdicts = s.NumEdicts

	a := ent.Angles(s)
	a[1] = 10
	ent.SetAngles(s, a)
	ent.SetIdealYaw(s, 20)
	ent.SetYawSpeed(s, 1)
	s.syncEdictToQCVM(entNum, ent)

	vm.SetGInt(qc.OFSSelf, int32(entNum))
	vm.SetEVector(entNum, qc.EntFieldAngles, [3]float32{0, 10, 0})
	vm.SetEFloat(entNum, qc.EntFieldIdealYaw, 350)
	vm.SetEFloat(entNum, qc.EntFieldYawSpeed, 15)
	if fn := vm.Builtins[49]; fn == nil {
		t.Fatal("changeyaw builtin not registered")
	} else {
		fn(vm)
	}
	// anglemod uses 16-bit quantization matching C, so 355 becomes ~355.00122
	if got := ent.Angles(s)[1]; got < 354.99 || got > 355.01 {
		t.Fatalf("changeyaw yaw = %v, want ~355", got)
	}
	if got := vm.EVector(entNum, qc.EntFieldAngles); got[1] < 354.99 || got[1] > 355.01 {
		t.Fatalf("vm yaw not synchronized after changeyaw: got=%v", got[1])
	}
}

func TestServerHooksMoveToGoalRestoresQCContextAfterNestedTouch(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)

	s.WorldModel = CreateSyntheticWorldModel()
	if world := s.EdictNum(0); world != nil {
		world.SetSolid(s, float32(SolidBSP))
	}
	s.ClearWorld()

	vm := newServerTestVM(s, 16)
	vm.NumEdicts = s.NumEdicts
	qc.RegisterBuiltins(vm)
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("touch_callback"), FirstStatement: 0},
		{Name: vm.AllocString("outer_qc_func"), FirstStatement: 1},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPDone)},
		{Op: uint16(qc.OPDone)},
	}

	self := s.AllocEdict()
	goal := s.AllocEdict()
	trigger := s.AllocEdict()
	if self == nil || goal == nil || trigger == nil {
		t.Fatal("failed to allocate edicts")
	}
	vm.NumEdicts = s.NumEdicts

	selfNum := s.NumForEdict(self)
	self.SetOrigin(s, [3]float32{0, 0, 24})
	self.SetMins(s, [3]float32{-16, -16, -24})
	self.SetMaxs(s, [3]float32{16, 16, 32})
	self.SetSolid(s, float32(SolidSlideBox))
	self.SetFlags(s, float32(FlagOnGround))
	self.SetIdealYaw(s, 0)
	self.SetYawSpeed(s, 360)

	goal.SetOrigin(s, [3]float32{64, 0, 24})
	goal.SetMins(s, [3]float32{-16, -16, -24})
	goal.SetMaxs(s, [3]float32{16, 16, 32})
	self.SetGoalEntity(s, int32(s.NumForEdict(goal)))

	trigger.SetOrigin(s, [3]float32{24, 0, 24})
	trigger.SetMins(s, [3]float32{-16, -16, -24})
	trigger.SetMaxs(s, [3]float32{16, 16, 32})
	trigger.SetSolid(s, float32(SolidTrigger))
	trigger.SetTouch(s, 1)

	s.LinkEdict(self, false)
	s.LinkEdict(goal, false)
	s.LinkEdict(trigger, false)
	s.syncEdictToQCVM(selfNum, self)
	s.syncEdictToQCVM(s.NumForEdict(goal), goal)
	s.syncEdictToQCVM(s.NumForEdict(trigger), trigger)

	vm.SetGInt(qc.OFSSelf, int32(selfNum))
	vm.SetGInt(qc.OFSOther, 77)
	vm.XFunction = &vm.Functions[2]
	vm.XFunctionIndex = 2
	vm.SetGFloat(qc.OFSParm0, 24)
	if fn := vm.Builtins[67]; fn == nil {
		t.Fatal("movetogoal builtin not registered")
	} else {
		fn(vm)
	}

	if got := vm.GInt(qc.OFSSelf); got != int32(selfNum) {
		t.Fatalf("self after nested movetogoal = %d, want %d", got, selfNum)
	}
	if got := vm.GInt(qc.OFSOther); got != 77 {
		t.Fatalf("other after nested movetogoal = %d, want 77", got)
	}
	if vm.XFunction != &vm.Functions[2] || vm.XFunctionIndex != 2 {
		t.Fatalf("qc context not restored: xfunction=%p idx=%d", vm.XFunction, vm.XFunctionIndex)
	}
}
