package server

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"path/filepath"
	"testing"

	cl "github.com/ironwail/ironwail-go/internal/client"
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
		channel  = 17
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
	if len(data) != 21 {
		t.Fatalf("datagram len = %d, want 21", len(data))
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
	for i, want := range []float32{10, 20, 30} {
		start := 9 + i*4
		if got := math.Float32frombits(binary.LittleEndian.Uint32(data[start : start+4])); got != want {
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
	for entNum := 1; entNum < s.NumEdicts; entNum++ {
		ent := s.EdictNum(entNum)
		if ent == nil || ent.Free || ent.Vars == nil {
			continue
		}
		if s.GetString(ent.Vars.ClassName) == "info_player_start" {
			foundStart = true
			break
		}
	}
	if !foundStart {
		t.Fatal("info_player_start entity was not loaded from the map entity lump")
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
	if got := parserClient.Entities[1].Origin; got != [3]float32{104, 208, 296} {
		t.Fatalf("parsed origin after server send phase = %v, want [104 208 296]", got)
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
	if client.SendSignon != SignonDone {
		t.Fatalf("SendSignon = %v, want %v", client.SendSignon, SignonDone)
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
	if client.SendSignon != SignonDone {
		t.Fatalf("SendSignon = %v, want %v", client.SendSignon, SignonDone)
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
	if target.Message.Len() == 0 || target.Message.Data[0] != byte(SVCPrint) {
		t.Fatalf("target message opcode = %v, want %v", target.Message.Data, byte(SVCPrint))
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
	ent.LeafNums[0] = 3 // leaf 3: (3-1)=2 → byte 0, bit 2

	pvs := make([]byte, 4)
	pvs[0] = 1 << 2 // bit 2 set

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
	if !s.SV_EdictInPVS(ent, pvs) {
		t.Error("expected edict with no leafs to be visible")
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

	if !s.SV_EdictInPVS(ent, nil) {
		t.Error("expected edict to be visible with nil PVS")
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
	ent.LeafNums[0] = 2  // byte 0, bit 1
	ent.LeafNums[1] = 10 // byte 1, bit 1
	ent.LeafNums[2] = 20 // byte 2, bit 3

	pvs := make([]byte, 4)
	pvs[1] = 1 << 1 // only leaf 10 visible: (10-1)=9 → byte 1, bit 1

	if !s.SV_EdictInPVS(ent, pvs) {
		t.Error("expected edict to be visible when one of multiple leafs is in PVS")
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
