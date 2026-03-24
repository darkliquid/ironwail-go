package server

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/model"
	inet "github.com/ironwail/ironwail-go/internal/net"
	"github.com/ironwail/ironwail-go/internal/qc"
	"github.com/ironwail/ironwail-go/internal/testutil"
)

func TestStartSoundUsesExtendedPacketForLargeEntityChannelAndSound(t *testing.T) {
	s := NewServer()
	s.MaxEdicts = 9000
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}

	const (
		entNum   = 8192
		channel  = 3
		soundNum = 300
	)
	s.SoundPrecache[soundNum] = "misc/large.wav"
	ent := &Edict{
		Vars: &EntVars{
			Origin: [3]float32{10, 20, 30},
			Mins:   [3]float32{-2, -4, -6},
			Maxs:   [3]float32{2, 4, 6},
		},
	}
	s.Edicts = make([]*Edict, entNum+1)
	s.Edicts[entNum] = ent

	s.StartSound(ent, channel, "misc/large.wav", 200, 0.5)

	data := s.Datagram.Data[:s.Datagram.Len()]
	// 1(svc) + 1(mask) + 1(vol) + 1(atten) + 2(ent) + 1(chan) + 2(snd) + 3*2(coords) = 15
	if len(data) != 15 {
		t.Fatalf("datagram len = %d, want 15", len(data))
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
	// Coords are 16-bit fixed-point (value * 8), 2 bytes each
	for i, want := range []float32{10, 20, 30} {
		start := 9 + i*2
		got := float32(int16(binary.LittleEndian.Uint16(data[start:start+2]))) / 8.0
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
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}

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

	if got := s.GetString(s.Edicts[0].Vars.ClassName); got != "worldspawn" {
		t.Fatalf("world classname = %q, want %q", got, "worldspawn")
	}
	if got := s.GetString(s.Edicts[0].Vars.Message); got == "" {
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
		if ent == nil || ent.Free || ent.Vars == nil {
			continue
		}
		className := s.GetString(ent.Vars.ClassName)
		if className == "info_player_start" {
			foundStart = true
		}
		if className == "trigger_changelevel" {
			foundChangeLevel = true
			if got := s.GetString(ent.Vars.Map); got == "" {
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
		if ent == nil || ent.Free || ent.Vars == nil {
			continue
		}
		className := s.GetString(ent.Vars.ClassName)
		if len(className) < len("monster_") || className[:len("monster_")] != "monster_" {
			continue
		}
		monsterCount++
		if ent.Vars.Origin == [3]float32{} {
			t.Fatalf("monster %d (%s) spawned at origin", entNum, className)
		}
		if blocker := s.TestEntityPosition(ent); blocker != nil {
			blockerClass := ""
			if blocker.Vars != nil {
				blockerClass = s.GetString(blocker.Vars.ClassName)
			}
			t.Fatalf("monster %d (%s) spawned in solid at %v blocker=%d (%s)", entNum, className, ent.Vars.Origin, s.NumForEdict(blocker), blockerClass)
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
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	client := s.Static.Clients[0]
	client.Active = true
	client.Spawned = false
	client.Message.WriteByte(byte(inet.SVCStuffText))
	client.Message.WriteString("bf\n")

	data := s.GetClientLoopbackMessage(0)
	if len(data) == 0 {
		t.Fatal("GetClientLoopbackMessage returned no data")
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

	data = s.GetClientLoopbackMessage(0)
	if len(data) != 0 {
		t.Fatalf("second GetClientLoopbackMessage len = %d, want 0", len(data))
	}
}

func TestLoopbackClientDatagramPreservesEntityDeltaAfterServerSendPhase(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}

	s.ConnectClient(0)
	serverClient := s.Static.Clients[0]
	serverClient.Loopback = true
	serverClient.Spawned = true
	serverClient.Edict.Vars.ModelIndex = 1
	serverClient.Edict.Vars.Colormap = 1
	serverClient.Edict.Vars.Origin = [3]float32{100, 200, 300}

	parserClient := cl.NewClient()
	parser := cl.NewParser(parserClient)

	initial := s.GetClientLoopbackMessage(0)
	if err := parser.ParseServerMessage(initial); err != nil {
		t.Fatalf("parse initial loopback message: %v", err)
	}
	if got := parserClient.Entities[1].Origin; got != [3]float32{100, 200, 300} {
		t.Fatalf("initial parsed origin = %v, want [100 200 300]", got)
	}

	serverClient.Edict.Vars.Origin = [3]float32{104, 208, 296}
	s.SendClientMessages()

	delta := s.GetClientLoopbackMessage(0)
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

	data := s.GetClientLoopbackMessage(0)
	if len(data) == 0 {
		t.Fatal("GetClientLoopbackMessage returned no data after kick")
	}
	if data[len(data)-1] != 0xff {
		t.Fatalf("terminator = 0x%02x, want 0xff", data[len(data)-1])
	}
	if !bytes.Contains(data, []byte("Kicked by Console: bye\n")) {
		t.Fatalf("kick datagram = %q, want kick message", string(data))
	}

	data = s.GetClientLoopbackMessage(0)
	if len(data) != 0 {
		t.Fatalf("second GetClientLoopbackMessage len = %d, want 0", len(data))
	}
}

func TestConnectClientClearsStaleReliableBuffer(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}

	client := s.Static.Clients[0]
	client.Message.WriteByte(byte(inet.SVCPrint))
	client.Message.WriteString("stale before reconnect\n")

	s.ConnectClient(0)

	data := s.GetClientLoopbackMessage(0)
	if len(data) == 0 {
		t.Fatal("GetClientLoopbackMessage returned no serverinfo")
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
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
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
	if got := s.GetString(client.Edict.Vars.ClassName); got != "player" {
		t.Fatalf("player classname = %q, want %q", got, "player")
	}
	if client.Edict.Vars.Health <= 0 {
		t.Fatalf("player health = %v, want > 0", client.Edict.Vars.Health)
	}
	if client.Edict.Vars.MoveType == 0 {
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
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.LoadGame = true
	s.ConnectClient(0)
	client := s.Static.Clients[0]
	client.Name = "Player"
	client.Color = 3
	client.Edict.Vars.Origin = [3]float32{128, 64, 32}
	client.Edict.Vars.Health = 37
	client.Edict.Vars.MoveType = float32(MoveTypeNoClip)

	if err := s.SubmitLoopbackStringCommand(0, "prespawn"); err != nil {
		t.Fatalf("SubmitLoopbackStringCommand(prespawn): %v", err)
	}
	if err := s.SubmitLoopbackStringCommand(0, "spawn"); err != nil {
		t.Fatalf("SubmitLoopbackStringCommand(spawn): %v", err)
	}
	if err := s.SubmitLoopbackStringCommand(0, "begin"); err != nil {
		t.Fatalf("SubmitLoopbackStringCommand(begin): %v", err)
	}

	if got := client.Edict.Vars.Origin; got != ([3]float32{128, 64, 32}) {
		t.Fatalf("player origin = %v, want preserved", got)
	}
	if got := client.Edict.Vars.Health; got != 37 {
		t.Fatalf("player health = %v, want 37", got)
	}
	if got := client.Edict.Vars.MoveType; got != float32(MoveTypeNoClip) {
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
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.QCVM = qc.NewVM()
	s.PreserveSpawnParms = true
	s.ConnectClient(0)
	client := s.Static.Clients[0]
	client.Name = "Player"
	client.Color = 3
	client.Edict.Vars.Origin = [3]float32{128, 64, 32}
	client.SpawnParms[0] = 42

	spawn := s.AllocEdict()
	if spawn == nil {
		t.Fatal("AllocEdict returned nil")
	}
	if spawn.Vars == nil {
		spawn.Vars = &EntVars{}
	}
	spawn.Vars.ClassName = s.QCVM.AllocString("info_player_start")
	spawn.Vars.Origin = [3]float32{480, -320, 64}
	spawn.Vars.Angles = [3]float32{0, 90, 0}

	if err := s.SubmitLoopbackStringCommand(0, "prespawn"); err != nil {
		t.Fatalf("SubmitLoopbackStringCommand(prespawn): %v", err)
	}
	if err := s.SubmitLoopbackStringCommand(0, "spawn"); err != nil {
		t.Fatalf("SubmitLoopbackStringCommand(spawn): %v", err)
	}
	if err := s.SubmitLoopbackStringCommand(0, "begin"); err != nil {
		t.Fatalf("SubmitLoopbackStringCommand(begin): %v", err)
	}

	if got := client.Edict.Vars.Origin; got != spawn.Vars.Origin {
		t.Fatalf("player origin = %v, want respawn at %v", got, spawn.Vars.Origin)
	}
	if got := client.Edict.Vars.Angles; got != spawn.Vars.Angles {
		t.Fatalf("player angles = %v, want %v", got, spawn.Vars.Angles)
	}
	if got := client.Edict.Vars.MoveType; got != float32(MoveTypeWalk) {
		t.Fatalf("player movetype = %v, want %v", got, float32(MoveTypeWalk))
	}
	if client.SpawnParms[0] != 42 {
		t.Fatalf("spawn parms changed unexpectedly: got %v, want 42", client.SpawnParms[0])
	}
}

func TestSpawnCommandWritesInitialSnapshot(t *testing.T) {
	s := NewServer()
	if err := s.Init(2); err != nil {
		t.Fatalf("init server: %v", err)
	}

	s.Time = 12.5
	s.LightStyles[0] = "m"
	s.LightStyles[1] = "abc"

	s.ConnectClient(0)
	client := s.Static.Clients[0]
	client.Name = "player"
	client.Color = 7
	client.Edict.Vars.Frags = 3

	if err := s.SubmitLoopbackStringCommand(0, "prespawn"); err != nil {
		t.Fatalf("prespawn: %v", err)
	}
	if err := s.SubmitLoopbackStringCommand(0, "spawn"); err != nil {
		t.Fatalf("spawn: %v", err)
	}

	data := client.Message.Data[:client.Message.Len()]
	if len(data) < 2 {
		t.Fatal("spawn snapshot missing")
	}
	if data[0] != byte(inet.SVCTime) {
		t.Fatalf("first spawn snapshot command = %d, want SVCTime", data[0])
	}
	if idx := bytes.Index(data, []byte{byte(inet.SVCUpdateName), 0}); idx < 0 {
		t.Fatal("spawn snapshot missing SVCUpdateName for player 0")
	}
	if idx := bytes.Index(data, []byte{byte(inet.SVCUpdateFrags), 0, 3, 0}); idx < 0 {
		t.Fatal("spawn snapshot missing SVCUpdateFrags for player 0")
	}
	if idx := bytes.Index(data, []byte{byte(inet.SVCUpdateColors), 0, 7}); idx < 0 {
		t.Fatal("spawn snapshot missing SVCUpdateColors for player 0")
	}
	if idx := bytes.Index(data, []byte{byte(inet.SVCLightStyle), 0}); idx < 0 {
		t.Fatal("spawn snapshot missing lightstyle 0")
	}
	if idx := bytes.Index(data, []byte{byte(inet.SVCSetAngle)}); idx < 0 {
		t.Fatal("spawn snapshot missing setangle")
	}
	if idx := bytes.Index(data, []byte{byte(inet.SVCClientData)}); idx < 0 {
		t.Fatal("spawn snapshot missing clientdata")
	}
	if got := data[len(data)-2]; got != byte(inet.SVCSignOnNum) {
		t.Fatalf("final spawn snapshot command = 0x%02x, want signon", got)
	}
	if got := data[len(data)-1]; got != 3 {
		t.Fatalf("final spawn signon = %d, want 3", got)
	}
	if client.SendSignon != SignonNone {
		t.Fatalf("SendSignon after spawn = %v, want %v", client.SendSignon, SignonNone)
	}
}

func TestWriteSpawnSetAngleUsesSpawnAnglesForFreshSpawn(t *testing.T) {
	s := &Server{Protocol: ProtocolFitzQuake}
	client := &Client{Edict: &Edict{Vars: &EntVars{}}}
	client.Edict.Vars.Angles = [3]float32{10, 20, 30}
	client.Edict.Vars.VAngle = [3]float32{90, 180, 270}

	msg := NewMessageBuffer(16)
	s.writeSpawnSetAngle(client, msg)

	data := msg.Data[:msg.Len()]
	if got, want := data[0], byte(inet.SVCSetAngle); got != want {
		t.Fatalf("message[0] = %d, want %d", got, want)
	}

	want := NewMessageBuffer(16)
	flags := uint32(s.ProtocolFlags())
	want.WriteAngle(10, flags)
	want.WriteAngle(20, flags)
	want.WriteAngle(0, flags)
	if got := data[1:4]; !bytes.Equal(got, want.Data[:want.Len()]) {
		t.Fatalf("fresh-spawn setangle payload = %v, want %v", got, want.Data[:want.Len()])
	}
}

func TestWriteSpawnSetAngleUsesViewAnglesForLoadGame(t *testing.T) {
	s := &Server{Protocol: ProtocolFitzQuake, LoadGame: true}
	client := &Client{Edict: &Edict{Vars: &EntVars{}}}
	client.Edict.Vars.Angles = [3]float32{10, 20, 30}
	client.Edict.Vars.VAngle = [3]float32{90, 180, 270}

	msg := NewMessageBuffer(16)
	s.writeSpawnSetAngle(client, msg)

	data := msg.Data[:msg.Len()]
	if got, want := data[0], byte(inet.SVCSetAngle); got != want {
		t.Fatalf("message[0] = %d, want %d", got, want)
	}

	want := NewMessageBuffer(16)
	flags := uint32(s.ProtocolFlags())
	want.WriteAngle(90, flags)
	want.WriteAngle(180, flags)
	want.WriteAngle(0, flags)
	if got := data[1:4]; !bytes.Equal(got, want.Data[:want.Len()]) {
		t.Fatalf("loadgame setangle payload = %v, want %v", got, want.Data[:want.Len()])
	}
}

func TestSpawnCommandAcceptsTrailingArgs(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}

	s.ConnectClient(0)
	client := s.Static.Clients[0]

	if err := s.SubmitLoopbackStringCommand(0, "prespawn"); err != nil {
		t.Fatalf("prespawn: %v", err)
	}
	if err := s.SubmitLoopbackStringCommand(0, "spawn 11 22 33"); err != nil {
		t.Fatalf("spawn with args: %v", err)
	}

	if client.SendSignon != SignonNone {
		t.Fatalf("SendSignon after spawn with args = %v, want %v", client.SendSignon, SignonNone)
	}
	if got := client.Message.Data[client.Message.Len()-1]; got != 3 {
		t.Fatalf("final spawn signon = %d, want 3", got)
	}
}

func TestClientNameCommandAcceptsQuotedNames(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}

	s.ConnectClient(0)
	client := s.Static.Clients[0]
	client.Active = true

	if !s.ExecuteClientString(client, `name "Major Player"`) {
		t.Fatal("ExecuteClientString(name quoted) = false, want true")
	}
	if got := client.Name; got != "Major Player" {
		t.Fatalf("client name = %q, want %q", got, "Major Player")
	}
}

func TestClientColorCommandAcceptsTopAndBottom(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}

	s.ConnectClient(0)
	client := s.Static.Clients[0]
	client.Active = true

	if !s.ExecuteClientString(client, "color 2 3") {
		t.Fatal("ExecuteClientString(color top bottom) = false, want true")
	}
	if got := client.Color; got != 0x23 {
		t.Fatalf("client color = 0x%02x, want 0x23", got)
	}
}

func TestKickClientDropsTargetAndWritesReason(t *testing.T) {
	s := NewServer()
	if err := s.Init(2); err != nil {
		t.Fatalf("init server: %v", err)
	}

	s.ConnectClient(0)
	s.ConnectClient(1)
	s.Static.Clients[0].Name = "Ranger"
	target := s.Static.Clients[1]
	target.Name = "Grunt"
	target.Message.Clear()

	if ok := s.KickClient(1, "Ranger", "too much ping"); !ok {
		t.Fatal("KickClient returned false, want true")
	}
	if target.Active {
		t.Fatal("target client still active after kick")
	}
	if target.Spawned {
		t.Fatal("target client still spawned after kick")
	}
	if target.Message.Len() == 0 || target.Message.Data[0] != byte(inet.SVCPrint) {
		t.Fatalf("target message opcode = %v, want %v", target.Message.Data, byte(inet.SVCPrint))
	}
	if !bytes.Contains(target.Message.Data, []byte("Kicked by Ranger: too much ping\n")) {
		t.Fatalf("target message = %q, want kick reason", string(target.Message.Data))
	}
}

func TestKickClientRejectsInvalidTargets(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}

	if ok := s.KickClient(0, "Console", ""); ok {
		t.Fatal("KickClient succeeded for inactive client")
	}
	if ok := s.KickClient(9, "Console", ""); ok {
		t.Fatal("KickClient succeeded for out-of-range client")
	}
}

func TestKillClientRejectsAlreadyDead(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}

	s.ConnectClient(0)
	client := s.Static.Clients[0]
	client.Edict.Vars.Health = 0

	if ok := s.KillClient(0); ok {
		t.Fatal("KillClient succeeded for dead client")
	}
	if !bytes.Contains(client.Message.Data[:client.Message.Len()], []byte("Can't suicide -- already dead!\n")) {
		t.Fatalf("kill rejection message = %q, want already-dead warning", string(client.Message.Data[:client.Message.Len()]))
	}
}

func TestSetClientNameBroadcastsReliableScoreboardUpdate(t *testing.T) {
	s := NewServer()
	if err := s.Init(2); err != nil {
		t.Fatalf("init server: %v", err)
	}

	s.ConnectClient(0)
	s.ConnectClient(1)
	for _, client := range s.Static.Clients[:2] {
		client.Active = true
		client.Message.Clear()
	}

	s.SetClientName(0, "Ranger")

	for i, client := range s.Static.Clients[:2] {
		data := client.Message.Data[:client.Message.Len()]
		if idx := bytes.Index(data, []byte{byte(inet.SVCUpdateName), 0}); idx < 0 {
			t.Fatalf("client %d missing SVCUpdateName broadcast: %v", i, data)
		}
		if !bytes.Contains(data, []byte("Ranger\x00")) {
			t.Fatalf("client %d missing updated player name in reliable stream: %q", i, string(data))
		}
	}
}

func TestSetClientColorBroadcastsReliableScoreboardUpdate(t *testing.T) {
	s := NewServer()
	if err := s.Init(2); err != nil {
		t.Fatalf("init server: %v", err)
	}

	s.ConnectClient(0)
	s.ConnectClient(1)
	for _, client := range s.Static.Clients[:2] {
		client.Active = true
		client.Message.Clear()
	}

	s.SetClientColor(0, 0x23)

	if got := int(s.Static.Clients[0].Edict.Vars.Team); got != 4 {
		t.Fatalf("team = %d, want 4 from bottom color nibble", got)
	}
	for i, client := range s.Static.Clients[:2] {
		data := client.Message.Data[:client.Message.Len()]
		if idx := bytes.Index(data, []byte{byte(inet.SVCUpdateColors), 0, 0x23}); idx < 0 {
			t.Fatalf("client %d missing SVCUpdateColors broadcast: %v", i, data)
		}
	}
}

func TestPutClientInServerRealProgsNoPanic(t *testing.T) {
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

	funcNum := s.QCVM.FindFunction("PutClientInServer")
	if funcNum < 0 {
		t.Skip("PutClientInServer missing in loaded progs")
	}
	entNum := s.NumForEdict(client.Edict)
	if entNum <= 0 {
		t.Fatalf("invalid client edict index %d", entNum)
	}
	s.syncQCVMState()
	syncEdictToQCVM(s.QCVM, entNum, client.Edict)

	s.QCVM.Time = float64(s.Time)
	s.QCVM.SetGlobal("time", s.Time)
	s.QCVM.SetGlobal("frametime", s.FrameTime)
	s.QCVM.SetGlobal("self", entNum)
	s.QCVM.SetGlobal("other", 0)
	s.QCVM.SetGlobal("msg_entity", entNum)
	for i := 0; i < len(client.SpawnParms); i++ {
		s.QCVM.SetGlobal(fmt.Sprintf("parm%d", i+1), client.SpawnParms[i])
	}

	panicked := false
	panicValue := any(nil)
	execErr := error(nil)
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				panicked = true
				panicValue = recovered
			}
		}()
		execErr = s.QCVM.ExecuteFunction(funcNum)
	}()

	if panicked {
		t.Fatalf("PutClientInServer panicked: %v", panicValue)
	}
	if execErr != nil {
		t.Fatalf("PutClientInServer returned error: %v", execErr)
	}

	syncEdictFromQCVM(s.QCVM, entNum, client.Edict)
	if client.Edict.Vars.Health <= 0 {
		t.Fatalf("player health = %v, want > 0 after PutClientInServer", client.Edict.Vars.Health)
	}
}

func TestStartTriggerChangelevelQueuesLevelChange(t *testing.T) {
	s := newStartMapDiagnosticsServer(t)

	s.ConnectClient(0)
	client := s.Static.Clients[0]
	if err := s.runClientSpawnQC(client); err != nil {
		t.Fatalf("runClientSpawnQC: %v", err)
	}
	client.Spawned = true

	player := client.Edict
	if player == nil || player.Vars == nil {
		t.Fatal("client missing spawned edict")
	}
	if got := s.GetString(player.Vars.ClassName); got != "player" {
		t.Fatalf("spawned player classname = %q, want %q", got, "player")
	}

	var trigger *Edict
	var wantLevel string
	for entNum := 1; entNum < s.NumEdicts; entNum++ {
		ent := s.EdictNum(entNum)
		if ent == nil || ent.Free || ent.Vars == nil {
			continue
		}
		if s.GetString(ent.Vars.ClassName) != "trigger_changelevel" {
			continue
		}
		trigger = ent
		wantLevel = s.GetString(ent.Vars.Map)
		break
	}
	if trigger == nil {
		t.Fatal("no trigger_changelevel found on start")
	}
	if wantLevel == "" {
		t.Fatal("trigger_changelevel missing destination map")
	}

	cmdsys.RemoveCommand("changelevel")
	defer cmdsys.RemoveCommand("changelevel")

	var gotLevels []string
	cmdsys.AddCommand("changelevel", func(args []string) {
		if len(args) > 0 {
			gotLevels = append(gotLevels, args[0])
			return
		}
		gotLevels = append(gotLevels, "")
	}, "")
	cmdsys.Execute()
	gotLevels = nil

	player.Vars.Origin = [3]float32{
		(trigger.Vars.AbsMin[0] + trigger.Vars.AbsMax[0]) * 0.5,
		(trigger.Vars.AbsMin[1] + trigger.Vars.AbsMax[1]) * 0.5,
		trigger.Vars.AbsMin[2] - player.Vars.Mins[2] + 1,
	}
	player.Vars.Velocity = [3]float32{}
	player.Vars.Flags = float32(uint32(player.Vars.Flags) | uint32(FlagOnGround))
	s.LinkEdict(player, false)
	s.touchLinks(player)
	cmdsys.Execute()

	if len(gotLevels) != 1 {
		t.Fatalf("changelevel executions = %v, want [%q]", gotLevels, wantLevel)
	}
	if gotLevels[0] != wantLevel {
		t.Fatalf("changelevel target = %q, want %q", gotLevels[0], wantLevel)
	}
}

func TestRunClientSpawnQCRelinksClientAfterQCSpawnMove(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.ClearWorld()

	vm := newServerTestVM(s, 8)
	s.QCVM = vm
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSFrameTime), Name: vm.AllocString("frametime")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSMsgEntity), Name: vm.AllocString("msg_entity")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSParm0), Name: vm.AllocString("parm1")},
	}

	triggerTouches := 0
	const callbackBuiltinOfs = 10
	vm.Builtins[1] = func(vm *qc.VM) {
		self := int(vm.GInt(qc.OFSSelf))
		vm.SetEVector(self, qc.EntFieldOrigin, [3]float32{128, 0, 0})
		vm.SetEVector(self, qc.EntFieldAngles, [3]float32{0, 90, 0})
		vm.SetEVector(self, qc.EntFieldVAngle, [3]float32{0, 90, 0})
		vm.SetEFloat(self, qc.EntFieldHealth, 100)
		vm.SetEInt(self, qc.EntFieldClassName, vm.AllocString("player"))
	}
	vm.Builtins[2] = func(vm *qc.VM) {
		triggerTouches++
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("PutClientInServer"), FirstStatement: 0},
		{Name: vm.AllocString("touch_callback"), FirstStatement: 2},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: uint16(callbackBuiltinOfs)},
		{Op: uint16(qc.OPDone)},
		{Op: uint16(qc.OPCall0), A: uint16(callbackBuiltinOfs + 1)},
		{Op: uint16(qc.OPDone)},
	}
	vm.SetGInt(callbackBuiltinOfs, -1)
	vm.SetGInt(callbackBuiltinOfs+1, -2)

	s.ConnectClient(0)
	client := s.Static.Clients[0]

	trigger := s.AllocEdict()
	if trigger == nil {
		t.Fatal("failed to allocate trigger edict")
	}
	trigger.Vars.Origin = [3]float32{128, 0, 24}
	trigger.Vars.Mins = [3]float32{-16, -16, -24}
	trigger.Vars.Maxs = [3]float32{16, 16, 32}
	trigger.Vars.Solid = float32(SolidTrigger)
	trigger.Vars.Touch = 2
	s.LinkEdict(trigger, false)

	if err := s.runClientSpawnQC(client); err != nil {
		t.Fatalf("runClientSpawnQC() error = %v", err)
	}

	if got := client.Edict.Vars.Origin; got != ([3]float32{128, 0, 0}) {
		t.Fatalf("player origin = %v, want [128 0 0]", got)
	}
	if client.Edict.AreaPrev == nil || client.Edict.AreaNext == nil {
		t.Fatal("player edict was not relinked after QC spawn move")
	}
	if triggerTouches != 1 {
		t.Fatalf("trigger touches = %d, want 1 after QC spawn move", triggerTouches)
	}
}

// --- SV_EdictInPVS tests ---

func TestEdictInPVSVisibleLeaf(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init: %v", err)
	}

	ent := &Edict{
		Vars:     &EntVars{},
		NumLeafs: 1,
	}
	ent.LeafNums[0] = 3 // leaf 3 -> byte 0, bit 3

	pvs := make([]byte, 4)
	pvs[0] = 1 << 3

	if !s.SV_EdictInPVS(ent, pvs) {
		t.Error("expected edict to be visible in PVS")
	}
}

func TestEdictInPVSNotVisible(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init: %v", err)
	}

	ent := &Edict{
		Vars:     &EntVars{},
		NumLeafs: 1,
	}
	ent.LeafNums[0] = 3

	pvs := make([]byte, 4) // all zeros

	if s.SV_EdictInPVS(ent, pvs) {
		t.Error("expected edict to NOT be visible in PVS")
	}
}

func TestSyncEdictFromQCVM_EmptyModelClearsStaleModelIndex(t *testing.T) {
	s := NewServer()
	vm := newServerTestVM(s, 4)
	s.QCVM = vm

	ent := &Edict{Vars: &EntVars{}}
	s.Edicts = []*Edict{{Vars: &EntVars{}}, ent}
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = s.NumEdicts

	ent.Vars.Model = vm.AllocString("progs/test.mdl")
	ent.Vars.ModelIndex = 7
	syncEdictToQCVM(vm, 1, ent)

	vm.SetEInt(1, qc.EntFieldModel, 0)
	vm.SetEFloat(1, qc.EntFieldModelIndex, 7)

	syncEdictFromQCVM(vm, 1, ent)

	if got := ent.Vars.Model; got != 0 {
		t.Fatalf("Model = %d, want 0 after QC raw clear", got)
	}
	if got := ent.Vars.ModelIndex; got != 0 {
		t.Fatalf("ModelIndex = %v, want 0 after QC raw clear", got)
	}
}

func TestEdictInPVSNoLeafs(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init: %v", err)
	}

	ent := &Edict{
		Vars:     &EntVars{},
		NumLeafs: 0,
	}

	pvs := make([]byte, 4)
	if s.SV_EdictInPVS(ent, pvs) {
		t.Error("expected edict with no leafs to be excluded from PVS")
	}
}

func TestEdictInPVSNilPVS(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init: %v", err)
	}

	ent := &Edict{
		Vars:     &EntVars{},
		NumLeafs: 1,
	}
	ent.LeafNums[0] = 5

	if s.SV_EdictInPVS(ent, nil) {
		t.Error("expected edict to be excluded with nil PVS")
	}
}

func TestEdictInPVSMultipleLeafsOneVisible(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init: %v", err)
	}

	ent := &Edict{
		Vars:     &EntVars{},
		NumLeafs: 3,
	}
	ent.LeafNums[0] = 2  // byte 0, bit 2
	ent.LeafNums[1] = 10 // byte 1, bit 2
	ent.LeafNums[2] = 20 // byte 2, bit 4

	pvs := make([]byte, 4)
	pvs[1] = 1 << 2 // only leaf 10 visible

	if !s.SV_EdictInPVS(ent, pvs) {
		t.Error("expected edict to be visible when one of multiple leafs is in PVS")
	}
}

func TestEdictInPVSUsesVisLeafNumbering(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init: %v", err)
	}

	ent := &Edict{
		Vars:     &EntVars{},
		NumLeafs: 1,
	}
	// Visleaf index 0 corresponds to BSP leaf index 1.
	ent.LeafNums[0] = 0

	pvs := []byte{0x01}
	if !s.SV_EdictInPVS(ent, pvs) {
		t.Fatal("expected visleaf 0 to be visible when bit 0 is set")
	}
}

func TestEdictInPVSMaxLeafsStillRequiresVisibleBits(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init: %v", err)
	}

	ent := &Edict{
		Vars:     &EntVars{},
		NumLeafs: MaxEntityLeafs,
	}

	if !s.SV_EdictInPVS(ent, make([]byte, 1)) {
		t.Error("expected edict touching max leafs to be treated as always visible")
	}
}

// --- CheckForNewClients tests ---

func TestCheckForNewClientsNoConnections(t *testing.T) {
	s := NewServer()
	if err := s.Init(2); err != nil {
		t.Fatalf("init: %v", err)
	}

	if err := s.CheckForNewClients(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckForNewClientsRejectsWhenServerFull(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("server init: %v", err)
	}
	s.Static.Clients[0].Active = true
	incoming := inet.NewSocket("incoming")
	s.acceptConnection = func() *inet.Socket {
		if incoming == nil {
			return nil
		}
		sock := incoming
		incoming = nil
		return sock
	}

	if err := s.CheckForNewClients(); err != nil {
		t.Fatalf("CheckForNewClients should not fail when full: %v", err)
	}
	if incoming != nil {
		t.Fatal("expected pending connection to be consumed")
	}
	if s.Static.Clients[0].NetConnection != nil {
		t.Fatal("full server should not bind incoming socket to active client slot")
	}
}

func TestFrameClearsDatagramBeforeSimulation(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init: %v", err)
	}
	s.Active = true
	s.Datagram.WriteByte(0x42)

	if err := s.Frame(0.05); err != nil {
		t.Fatalf("Frame: %v", err)
	}
	if got := s.Datagram.Len(); got != 0 {
		t.Fatalf("datagram len after frame = %d, want 0", got)
	}
}

func TestReadClientMessageProcessesStringCmdWithoutSentinel(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init: %v", err)
	}
	s.ConnectClient(0)
	client := s.Static.Clients[0]

	msg := NewMessageBuffer(32)
	msg.WriteByte(byte(CLCStringCmd))
	msg.WriteString("prespawn")

	if !s.SV_ReadClientMessage(client, msg) {
		t.Fatal("SV_ReadClientMessage rejected a complete stringcmd payload")
	}
	if got := client.SendSignon; got != SignonPrespawn {
		t.Fatalf("SendSignon = %v, want %v", got, SignonPrespawn)
	}
}

func TestSendClientMessagesQueuesKeepaliveNopForIdleRemoteClient(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init: %v", err)
	}

	loop := inet.NewLoopback()
	if err := loop.Init(); err != nil {
		t.Fatalf("loopback init: %v", err)
	}
	clientSock := loop.Connect()
	serverSock := loop.CheckNewConnections()
	if serverSock == nil {
		t.Fatal("server socket missing")
	}
	defer inet.Close(clientSock)
	defer inet.Close(serverSock)

	client := s.Static.Clients[0]
	client.Active = true
	client.Spawned = false
	client.Loopback = false
	client.NetConnection = serverSock
	client.LastMessage = float64(s.Time) - 6
	client.Message.Clear()

	s.SendClientMessages()

	msgType, payload := inet.GetMessage(clientSock)
	if msgType != 2 {
		t.Fatalf("message type = %d, want 2", msgType)
	}
	if len(payload) != 1 || payload[0] != byte(inet.SVCNop) {
		t.Fatalf("payload = %v, want [SVCNop]", payload)
	}
}

func TestSendClientMessagesHoldsReliableDataForIdleUnspawnedClient(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init: %v", err)
	}

	loop := inet.NewLoopback()
	if err := loop.Init(); err != nil {
		t.Fatalf("loopback init: %v", err)
	}
	clientSock := loop.Connect()
	serverSock := loop.CheckNewConnections()
	if serverSock == nil {
		t.Fatal("server socket missing")
	}
	defer inet.Close(clientSock)
	defer inet.Close(serverSock)

	client := s.Static.Clients[0]
	client.Active = true
	client.Spawned = false
	client.Loopback = false
	client.NetConnection = serverSock
	client.SendSignon = SignonNone
	client.Message.WriteByte(byte(inet.SVCPrint))
	client.Message.WriteString("held")

	s.SendClientMessages()

	msgType, payload := inet.GetMessage(clientSock)
	if msgType != 0 || len(payload) != 0 {
		t.Fatalf("got network payload type=%d payload=%v, want none", msgType, payload)
	}
	if client.Message.Len() == 0 {
		t.Fatal("expected reliable payload to stay queued")
	}
}

func TestMessageBufferOverflowSetsFlag(t *testing.T) {
	msg := NewMessageBuffer(1)
	msg.WriteByte(0x01)
	msg.WriteByte(0x02)
	if !msg.Overflowed {
		t.Fatal("expected overflow flag after write past capacity")
	}
	if got := msg.Len(); got != 1 {
		t.Fatalf("len = %d, want 1", got)
	}
	msg.Clear()
	if msg.Overflowed {
		t.Fatal("Clear should reset overflow flag")
	}
}
