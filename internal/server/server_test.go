// This file belongs to the Tests subsystem: unit, integration, parity, and e2e tests for the server package.

package server

import (
	"bytes"
	"encoding/binary"
	"path/filepath"
	"strings"
	"testing"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/model"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/testutil"
)

func TestNewServerUsesExtendedEdictCapacity(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 8)
	if got := s.MaxEdicts; got != MaxEdicts {
		t.Fatalf("MaxEdicts = %d, want %d", got, MaxEdicts)
	}
}

func TestStartSoundUsesExtendedPacketForLargeEntityChannelAndSound(t *testing.T) {
	s := NewServer()
	s.MaxEdicts = 9000
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.QCVM.NumEdicts = 9001

	const (
		entNum   = 8192
		channel  = 3
		soundNum = 300
	)
	s.SoundPrecache[soundNum] = "misc/large.wav"
	ent := &Edict{Num: entNum}
	ent.SetOrigin(s, [3]float32{10, 20, 30})
	ent.SetMins(s, [3]float32{-2, -4, -6})
	ent.SetMaxs(s, [3]float32{2, 4, 6})
	s.Edicts = make([]*Edict, entNum+1)
	s.Edicts[entNum] = ent

	s.StartSound(ent, channel, "misc/large.wav", 200, 0.5)

	data := s.Datagram.Data[:s.Datagram.Len()]
	// RMQ protocol uses PRFL_INT32COORD (4 bytes per coord).
	// 1(svc) + 1(mask) + 1(vol) + 1(atten) + 2(ent) + 1(chan) + 2(snd) + 3*4(coords) = 21
	coordSize := 4 // PRFL_INT32COORD
	wantLen := 9 + 3*coordSize
	if len(data) != wantLen {
		t.Fatalf("datagram len = %d, want %d", len(data), wantLen)
	}
	if got := data[0]; got != byte(inet.SVCSound) {
		t.Fatalf("svc = %d, want %d", got, inet.SVCSound)
	}
	wantMask := byte(inet.SND_VOLUME | inet.SND_ATTENUATION | inet.SND_LARGEENTITY | inet.SND_LARGESOUND)
	if got := data[1]; got != wantMask {
		t.Fatalf("field mask = 0x%02x, want 0x%02x", got, wantMask)
	}
	if got := data[2]; got != 200 {
		t.Fatalf("volume = %d, want 200", got)
	}
	if got := data[3]; got != byte(0.5*64) {
		t.Fatalf("attenuation byte = %d, want %d", got, byte(0.5*64))
	}
	if got := int(binary.LittleEndian.Uint16(data[4:6])); got != entNum {
		t.Fatalf("entity = %d, want %d", got, entNum)
	}
	if got := int(data[6]); got != channel {
		t.Fatalf("channel = %d, want %d", got, channel)
	}
	if got := int(binary.LittleEndian.Uint16(data[7:9])); got != soundNum {
		t.Fatalf("sound = %d, want %d", got, soundNum)
	}
	// Coords are 32-bit int (value * 16) under PRFL_INT32COORD, 4 bytes each
	for i, want := range []float32{10, 20, 30} {
		start := 9 + i*coordSize
		got := float32(int32(binary.LittleEndian.Uint32(data[start:start+4]))) / 16.0
		if got != want {
			t.Fatalf("origin[%d] = %v, want %v", i, got, want)
		}
	}
}

func TestSpawnServerStartMap(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	vfs := fs.NewFileSystem()
	if err := vfs.Init(baseDir, "id1"); err != nil {
		t.Fatalf("init filesystem: %v", err)
	}
	defer vfs.Close()

	s := NewServer()
	newServerTestVM(s, 8)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	qc.RegisterBuiltins(s.QCVM)
	if err := s.SpawnServer("start", vfs); err != nil {
		t.Fatalf("spawn server: %v", err)
	}

	if !s.Active {
		t.Fatalf("server not active after SpawnServer")
	}
	if s.State != ServerStateActive {
		t.Fatalf("server state = %v, want %v", s.State, ServerStateActive)
	}
	if s.ModelName != "maps/start.bsp" {
		t.Fatalf("model name = %q, want %q", s.ModelName, "maps/start.bsp")
	}
	if s.WorldModel == nil {
		t.Fatalf("world model is nil")
	}
	if got := s.WorldModel.ModelType(); got != int(model.ModBrush) {
		t.Fatalf("world model type = %d, want %d", got, model.ModBrush)
	}
	if len(s.WorldTree.Models) > 1 && s.FindModel("*1") == 0 {
		t.Fatal("local brush model *1 was not precached")
	}
	wmHull0 := s.WorldModel.Hull(0)
	if len(wmHull0.ClipNodes) == 0 {
		t.Fatal("world hull 0 was not initialized")
	}
	wmHull1 := s.WorldModel.Hull(1)
	if len(wmHull1.ClipNodes) == 0 {
		t.Fatal("world hull 1 was not initialized")
	}
}

func TestSpawnServerLoadsMapEntitiesIntoQCVM(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	vfs := fs.NewFileSystem()
	if err := vfs.Init(baseDir, "id1"); err != nil {
		t.Fatalf("init filesystem: %v", err)
	}
	defer vfs.Close()

	s := NewServer()
	newServerTestVM(s, 8)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}

	progsData, err := vfs.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("load progs.dat: %v", err)
	}
	if err := s.QCVM.LoadProgs(bytes.NewReader(progsData)); err != nil {
		t.Fatalf("LoadProgs: %v", err)
	}
	qc.RegisterBuiltins(s.QCVM)

	if err := s.SpawnServer("start", vfs); err != nil {
		t.Fatalf("spawn server: %v", err)
	}

	if got := s.String(s.Edicts[0].ClassName(s)); got != "worldspawn" {
		t.Fatalf("world classname = %q, want %q", got, "worldspawn")
	}
	if got := s.String(s.Edicts[0].Message(s)); got == "" {
		t.Fatal("world message was not loaded into QC strings")
	}
	if got := s.QCVM.GString(qc.OFSMapName); got != "start" {
		t.Fatalf("mapname global = %q, want %q", got, "start")
	}
	if s.QCVM.NumEdicts != s.NumEdicts {
		t.Fatalf("QCVM.NumEdicts = %d, want %d", s.QCVM.NumEdicts, s.NumEdicts)
	}

	foundStart := false
	foundChangeLevel := false
	for entNum := 1; entNum < s.NumEdicts; entNum++ {
		ent := s.EdictNum(entNum)
		if ent == nil || ent.Free {
			continue
		}
		className := s.String(ent.ClassName(s))
		if className == "info_player_start" {
			foundStart = true
		}
		if className == "trigger_changelevel" {
			foundChangeLevel = true
			if got := s.String(ent.Map(s)); got == "" {
				t.Fatalf("trigger_changelevel %d missing map key after entity parse", entNum)
			}
		}
	}
	if !foundStart {
		t.Fatal("info_player_start entity was not loaded from the map entity lump")
	}
	if !foundChangeLevel {
		t.Fatal("trigger_changelevel entity was not loaded from the map entity lump")
	}
}

func TestSpawnServerE2M2MonstersDoNotStartInSolid(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	vfs := fs.NewFileSystem()
	if err := vfs.Init(baseDir, "id1"); err != nil {
		t.Fatalf("init filesystem: %v", err)
	}
	defer vfs.Close()

	s := NewServer()
	newServerTestVM(s, 8)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}

	progsData, err := vfs.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("load progs.dat: %v", err)
	}
	if err := s.QCVM.LoadProgs(bytes.NewReader(progsData)); err != nil {
		t.Fatalf("LoadProgs: %v", err)
	}
	qc.RegisterBuiltins(s.QCVM)

	if err := s.SpawnServer("e2m2", vfs); err != nil {
		t.Fatalf("spawn server: %v", err)
	}

	monsterCount := 0
	for entNum := 1; entNum < s.NumEdicts; entNum++ {
		ent := s.EdictNum(entNum)
		if ent == nil || ent.Free {
			continue
		}
		className := s.String(ent.ClassName(s))
		if len(className) < len("monster_") || className[:len("monster_")] != "monster_" {
			continue
		}
		monsterCount++
		if ent.Origin(s) == ([3]float32{}) {
			t.Fatalf("monster %d (%s) spawned at origin", entNum, className)
		}
		if blocker := s.TestEntityPosition(ent); blocker != nil {
			blockerClass := ""
			if blocker != nil {
				blockerClass = s.String(blocker.ClassName(s))
			}
			t.Fatalf("monster %d (%s) spawned in solid at %v blocker=%d (%s)", entNum, className, ent.Origin(s), s.NumForEdict(blocker), blockerClass)
		}
	}
	if monsterCount == 0 {
		t.Fatal("expected monsters on e2m2")
	}
}

func TestSpawnServerE2M2DoesNotWarnWalkmonsterInWall(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	vfs := fs.NewFileSystem()
	if err := vfs.Init(baseDir, "id1"); err != nil {
		t.Fatalf("init filesystem: %v", err)
	}
	defer vfs.Close()

	s := NewServer()
	newServerTestVM(s, 8)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}

	progsData, err := vfs.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("load progs.dat: %v", err)
	}
	if err := s.QCVM.LoadProgs(bytes.NewReader(progsData)); err != nil {
		t.Fatalf("LoadProgs: %v", err)
	}
	qc.RegisterBuiltins(s.QCVM)

	var warnings []string
	oldDPrint := s.QCVM.Builtins[25]
	s.QCVM.Builtins[25] = func(vm *qc.VM) {
		msg := vm.GString(qc.OFSParm0)
		if strings.Contains(msg, "walkmonster in wall at") {
			warnings = append(warnings, msg)
		}
		oldDPrint(vm)
	}

	if err := s.SpawnServer("e2m2", vfs); err != nil {
		t.Fatalf("spawn server: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected walkmonster warnings during spawn: %q", warnings)
	}
}

func TestGetClientLoopbackMessageIncludesReliableBuffer(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 8)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	client := s.Static.Clients[0]
	client.Active = true
	client.Spawned = false
	client.Message.PutByte(byte(inet.SVCStuffText))
	client.Message.WriteString("bf\n")

	data := s.ClientLoopbackMessage(0)
	if len(data) == 0 {
		t.Fatal("ClientLoopbackMessage returned no data")
	}
	if data[len(data)-1] != 0xff {
		t.Fatalf("terminator = 0x%02x, want 0xff", data[len(data)-1])
	}
	if data[0] != byte(inet.SVCStuffText) {
		t.Fatalf("first byte = 0x%02x, want SVCStuffText", data[0])
	}
	if client.Message.Len() != 0 {
		t.Fatalf("client reliable buffer len = %d, want 0", client.Message.Len())
	}

	data = s.ClientLoopbackMessage(0)
	if len(data) != 0 {
		t.Fatalf("second ClientLoopbackMessage len = %d, want 0", len(data))
	}
}

func TestLoopbackClientDatagramPreservesEntityDeltaAfterServerSendPhase(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 8)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}

	s.ConnectClient(0)
	serverClient := s.Static.Clients[0]
	serverClient.Loopback = true
	serverClient.Spawned = true
	serverClient.Edict.SetModelIndex(s, 1)
	serverClient.Edict.SetColormap(s, 1)
	serverClient.Edict.SetOrigin(s, [3]float32{100, 200, 300})

	parserClient := cl.NewClient()
	parser := cl.NewParser(parserClient)

	initial := s.ClientLoopbackMessage(0)
	if err := parser.ParseServerMessage(initial); err != nil {
		t.Fatalf("parse initial loopback message: %v", err)
	}
	if got := parserClient.Entities[1].Origin; got != [3]float32{100, 200, 300} {
		t.Fatalf("initial parsed origin = %v, want [100 200 300]", got)
	}

	serverClient.Edict.SetOrigin(s, [3]float32{104, 208, 296})
	s.SendClientMessages()

	delta := s.ClientLoopbackMessage(0)
	if err := parser.ParseServerMessage(delta); err != nil {
		t.Fatalf("parse loopback delta message: %v", err)
	}
	if got := parserClient.Entities[1].MsgOrigins[0]; got != [3]float32{104, 208, 296} {
		t.Fatalf("parsed raw origin after server send phase = %v, want [104 208 296]", got)
	}
	if got := parserClient.Entities[1].Origin; got != [3]float32{100, 200, 300} {
		t.Fatalf("parsed live origin after server send phase = %v, want preserved [100 200 300] until relink", got)
	}
}

func TestKickClientLeavesFinalLoopbackMessageAvailable(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 8)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}

	s.ConnectClient(0)
	client := s.Static.Clients[0]
	client.Name = "Grunt"
	client.Message.Clear()

	if ok := s.KickClient(0, "Console", "bye"); !ok {
		t.Fatal("KickClient returned false, want true")
	}

	data := s.ClientLoopbackMessage(0)
	if len(data) == 0 {
		t.Fatal("ClientLoopbackMessage returned no data after kick")
	}
	if data[len(data)-1] != 0xff {
		t.Fatalf("terminator = 0x%02x, want 0xff", data[len(data)-1])
	}
	if !bytes.Contains(data, []byte("Kicked by Console: bye\n")) {
		t.Fatalf("kick datagram = %q, want kick message", string(data))
	}

	data = s.ClientLoopbackMessage(0)
	if len(data) != 0 {
		t.Fatalf("second ClientLoopbackMessage len = %d, want 0", len(data))
	}
}

func TestConnectClientClearsStaleReliableBuffer(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 8)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}

	client := s.Static.Clients[0]
	client.Message.PutByte(byte(inet.SVCPrint))
	client.Message.WriteString("stale before reconnect\n")

	s.ConnectClient(0)

	data := s.ClientLoopbackMessage(0)
	if len(data) == 0 {
		t.Fatal("ClientLoopbackMessage returned no serverinfo")
	}
	if bytes.Contains(data, []byte("stale before reconnect\n")) {
		t.Fatalf("serverinfo datagram still contains stale message: %q", string(data))
	}
}

func TestSpawnServerActiveQueuesReconnectForConnectedClients(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	vfs := fs.NewFileSystem()
	if err := vfs.Init(baseDir, "id1"); err != nil {
		t.Fatalf("init filesystem: %v", err)
	}
	defer vfs.Close()

	s := NewServer()
	newServerTestVM(s, 8)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	qc.RegisterBuiltins(s.QCVM)
	if err := s.SpawnServer("start", vfs); err != nil {
		t.Fatalf("first spawn server: %v", err)
	}

	client := s.Static.Clients[0]
	client.Active = true
	client.Message.Clear()

	if err := s.SpawnServer("start", vfs); err != nil {
		t.Fatalf("second spawn server: %v", err)
	}

	if !bytes.Contains(client.Message.Data[:client.Message.Len()], []byte("reconnect\n")) {
		t.Fatalf("client reliable buffer missing reconnect command: %q", string(client.Message.Data[:client.Message.Len()]))
	}
}

func TestSubmitLoopbackStringCommandSpawnRunsQCPlayerSpawn(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	vfs := fs.NewFileSystem()
	if err := vfs.Init(baseDir, "id1"); err != nil {
		t.Fatalf("init filesystem: %v", err)
	}
	defer vfs.Close()

	s := NewServer()
	newServerTestVM(s, 8)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}

	progsData, err := vfs.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("load progs.dat: %v", err)
	}
	if err := s.QCVM.LoadProgs(bytes.NewReader(progsData)); err != nil {
		t.Fatalf("LoadProgs: %v", err)
	}
	qc.RegisterBuiltins(s.QCVM)

	if err := s.SpawnServer("start", vfs); err != nil {
		t.Fatalf("spawn server: %v", err)
	}
	s.ConnectClient(0)
	client := s.Static.Clients[0]
	if err := s.SubmitLoopbackStringCommand(0, "prespawn"); err != nil {
		t.Fatalf("SubmitLoopbackStringCommand(prespawn): %v", err)
	}

	if err := s.SubmitLoopbackStringCommand(0, "spawn"); err != nil {
		t.Fatalf("SubmitLoopbackStringCommand(spawn): %v", err)
	}
	if client.Spawned {
		t.Fatal("client marked spawned before begin")
	}
	if got := s.String(client.Edict.ClassName(s)); got != "player" {
		t.Fatalf("player classname = %q, want %q", got, "player")
	}
	if client.Edict.Health(s) <= 0 {
		t.Fatalf("player health = %v, want > 0", client.Edict.Health(s))
	}
	if client.Edict.MoveType(s) == 0 {
		t.Fatal("player movetype was not initialized by QC spawn")
	}
	if client.Message == nil || client.Message.Len() < 2 {
		t.Fatal("spawn reply buffer missing")
	}
	if got := client.Message.Data[client.Message.Len()-2]; got != byte(inet.SVCSignOnNum) {
		t.Fatalf("spawn reply command = 0x%02x, want signon", got)
	}
	if got := client.Message.Data[client.Message.Len()-1]; got != 3 {
		t.Fatalf("spawn signon = %d, want 3", got)
	}

	if err := s.SubmitLoopbackStringCommand(0, "begin"); err != nil {
		t.Fatalf("SubmitLoopbackStringCommand(begin): %v", err)
	}
	if !client.Spawned {
		t.Fatal("client not marked spawned after begin")
	}
	if client.SendSignon != SignonNone {
		t.Fatalf("SendSignon = %v, want %v", client.SendSignon, SignonNone)
	}
}

func TestSubmitLoopbackStringCommandBeginRequiresSpawn(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 8)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.ConnectClient(0)
	client := s.Static.Clients[0]

	if err := s.SubmitLoopbackStringCommand(0, "begin"); err == nil {
		t.Fatal("begin succeeded before spawn")
	}
	if client.Spawned {
		t.Fatal("client marked spawned by out-of-order begin")
	}
	if client.SendSignon != SignonFlush {
		t.Fatalf("SendSignon = %v, want %v", client.SendSignon, SignonFlush)
	}
}

func TestSubmitLoopbackStringCommandLoadGamePreservesPlayerState(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 8)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.LoadGame = true
	s.ConnectClient(0)
	client := s.Static.Clients[0]
	client.Name = "Player"
	client.Color = 3
	client.Edict.SetOrigin(s, [3]float32{128, 64, 32})
	client.Edict.SetHealth(s, 37)
	client.Edict.SetMoveType(s, float32(MoveTypeNoClip))

	if err := s.SubmitLoopbackStringCommand(0, "prespawn"); err != nil {
		t.Fatalf("SubmitLoopbackStringCommand(prespawn): %v", err)
	}
	if err := s.SubmitLoopbackStringCommand(0, "spawn"); err != nil {
		t.Fatalf("SubmitLoopbackStringCommand(spawn): %v", err)
	}
	if err := s.SubmitLoopbackStringCommand(0, "begin"); err != nil {
		t.Fatalf("SubmitLoopbackStringCommand(begin): %v", err)
	}

	if got := client.Edict.Origin(s); got != ([3]float32{128, 64, 32}) {
		t.Fatalf("player origin = %v, want preserved", got)
	}
	if got := client.Edict.Health(s); got != 37 {
		t.Fatalf("player health = %v, want 37", got)
	}
	if got := client.Edict.MoveType(s); got != float32(MoveTypeNoClip) {
		t.Fatalf("player movetype = %v, want %v", got, float32(MoveTypeNoClip))
	}
	if !client.Spawned {
		t.Fatal("client not marked spawned after load begin")
	}
	if client.SendSignon != SignonNone {
		t.Fatalf("SendSignon = %v, want %v", client.SendSignon, SignonNone)
	}
}

func TestSubmitLoopbackStringCommandPreserveSpawnParmsRespawnsPlayer(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 8)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.PreserveSpawnParms = true
	s.ConnectClient(0)
	client := s.Static.Clients[0]
	client.Name = "Player"
	client.Color = 3
	client.Edict.SetOrigin(s, [3]float32{128, 64, 32})
	client.SpawnParms[0] = 42

	spawn := s.AllocEdict()
	if spawn == nil {
		t.Fatal("AllocEdict returned nil")
	}
	if spawn == nil {
	}
	spawn.SetClassName(s, s.QCVM.AllocString("info_player_start"))
	spawn.SetOrigin(s, [3]float32{480, -320, 64})
	spawn.SetAngles(s, [3]float32{0, 90, 0})

	if err := s.SubmitLoopbackStringCommand(0, "prespawn"); err != nil {
		t.Fatalf("SubmitLoopbackStringCommand(prespawn): %v", err)
	}
	if err := s.SubmitLoopbackStringCommand(0, "spawn"); err != nil {
		t.Fatalf("SubmitLoopbackStringCommand(spawn): %v", err)
	}
	if err := s.SubmitLoopbackStringCommand(0, "begin"); err != nil {
		t.Fatalf("SubmitLoopbackStringCommand(begin): %v", err)
	}

	if got := client.Edict.Origin(s); got != spawn.Origin(s) {
		t.Fatalf("player origin = %v, want respawn at %v", got, spawn.Origin(s))
	}
	if got := client.Edict.Angles(s); got != spawn.Angles(s) {
		t.Fatalf("player angles = %v, want %v", got, spawn.Angles(s))
	}
	if got := client.Edict.MoveType(s); got != float32(MoveTypeWalk) {
		t.Fatalf("player movetype = %v, want %v", got, float32(MoveTypeWalk))
	}
	if client.SpawnParms[0] != 42 {
		t.Fatalf("spawn parms changed unexpectedly: got %v, want 42", client.SpawnParms[0])
	}
}
