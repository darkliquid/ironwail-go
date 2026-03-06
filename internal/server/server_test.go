package server

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/model"
	inet "github.com/ironwail/ironwail-go/internal/net"
	"github.com/ironwail/ironwail-go/internal/qc"
	"github.com/ironwail/ironwail-go/internal/testutil"
)

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
	if _, ok := s.WorldModel.(*model.Model); !ok {
		t.Fatalf("world model has unexpected type %T", s.WorldModel)
	}
	if len(s.WorldTree.Models) > 1 && s.FindModel("*1") == 0 {
		t.Fatal("local brush model *1 was not precached")
	}
	wm := s.WorldModel.(*model.Model)
	if len(wm.Hulls[0].ClipNodes) == 0 {
		t.Fatal("world hull 0 was not initialized")
	}
	if len(wm.Hulls[1].ClipNodes) == 0 {
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
