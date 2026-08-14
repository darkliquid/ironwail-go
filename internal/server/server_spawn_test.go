// This file belongs to the Tests subsystem: unit, integration, parity, and e2e tests for the server package.

package server

// Spawn command, client command, and PutClientInServer tests split from server_test.go.

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cmdsys"
	"github.com/darkliquid/ironwail-go/internal/fs"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/testutil"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

func TestSpawnCommandWritesInitialSnapshot(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
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
	client.Edict.SetFrags(s, 3)

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

func TestSpawnCommandWritesSkyboxName(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.SkyboxName = "gfx/env/qbj3"

	s.ConnectClient(0)
	client := s.Static.Clients[0]

	if err := s.SubmitLoopbackStringCommand(0, "prespawn"); err != nil {
		t.Fatalf("prespawn: %v", err)
	}
	if err := s.SubmitLoopbackStringCommand(0, "spawn"); err != nil {
		t.Fatalf("spawn: %v", err)
	}

	data := client.Message.Data[:client.Message.Len()]
	if !bytes.Contains(data, []byte{byte(inet.SVCSkyBox), 'g', 'f', 'x', '/', 'e', 'n', 'v', '/', 'q', 'b', 'j', '3', 0}) {
		t.Fatalf("spawn snapshot does not include skybox message: %v", data)
	}
}

func TestWriteSpawnSkyboxWritesSkyboxMessage(t *testing.T) {
	s := &Server{Protocol: ProtocolFitzQuake, SkyboxName: "sky"}
	client := &Client{Edict: &Edict{Num: 1}}
	msg := NewMessageBuffer(32)

	s.writeSpawnSkybox(client, msg)

	data := msg.Data[:msg.Len()]
	if len(data) == 0 {
		t.Fatal("writeSpawnSkybox produced empty message")
	}
	if got, want := data[0], byte(inet.SVCSkyBox); got != want {
		t.Fatalf("message[0] = %d, want %d", got, want)
	}
	if !bytes.Contains(data, []byte("sky\x00")) {
		t.Fatalf("spawn snapshot does not include skybox message: %v", data)
	}
}

func TestWriteSpawnSetAngleUsesSpawnAnglesForFreshSpawn(t *testing.T) {
	s := &Server{Protocol: ProtocolFitzQuake}
	newServerTestVM(s, 8)
	client := &Client{Edict: &Edict{Num: 1}}
	client.Edict.SetAngles(s, qtypes.Vec3{X: 10, Y: 20, Z: 30})
	client.Edict.SetVAngle(s, qtypes.Vec3{X: 90, Y: 180, Z: 270})

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
	newServerTestVM(s, 8)
	client := &Client{Edict: &Edict{Num: 1}}
	client.Edict.SetAngles(s, qtypes.Vec3{X: 10, Y: 20, Z: 30})
	client.Edict.SetVAngle(s, qtypes.Vec3{X: 90, Y: 180, Z: 270})

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
	newServerTestVM(s, 16)
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
	newServerTestVM(s, 16)
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
	newServerTestVM(s, 16)
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

func TestClientBanCommandAppliesIPBanAndPrintsStatus(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.ConnectClient(0)
	client := s.Static.Clients[0]
	client.Active = true
	client.Message.Clear()
	if err := inet.DefaultNetwork().SetIPBan("off", ""); err != nil {
		t.Fatalf("clear starting ban: %v", err)
	}
	oldDeathmatch := s.CVar.StringValue("deathmatch")
	s.CVar.Set("deathmatch", "0")
	t.Cleanup(func() {
		s.CVar.Set("deathmatch", oldDeathmatch)
		_ = inet.DefaultNetwork().SetIPBan("off", "")
	})

	if !s.ExecuteClientString(client, "ban 10.0.0.0 255.255.0.0") {
		t.Fatal("ExecuteClientString(ban set) = false, want true")
	}
	if got := inet.DefaultNetwork().IPBanStatus(); got != "Banning 10.0.0.0 [255.255.0.0]" {
		t.Fatalf("ban status after set = %q", got)
	}
	if client.Message.Len() != 0 {
		t.Fatalf("ban set produced unexpected client print data: %q", string(client.Message.Data[:client.Message.Len()]))
	}

	if !s.ExecuteClientString(client, "ban") {
		t.Fatal("ExecuteClientString(ban status) = false, want true")
	}
	if !bytes.Contains(client.Message.Data[:client.Message.Len()], []byte("Banning 10.0.0.0 [255.255.0.0]\n")) {
		t.Fatalf("ban status print missing, got %q", string(client.Message.Data[:client.Message.Len()]))
	}

	client.Message.Clear()
	if !s.ExecuteClientString(client, "ban off") {
		t.Fatal("ExecuteClientString(ban off) = false, want true")
	}
	if got := inet.DefaultNetwork().IPBanStatus(); got != "Banning not active" {
		t.Fatalf("ban status after off = %q", got)
	}
}

func TestClientBanCommandNoopsDuringDeathmatch(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.ConnectClient(0)
	client := s.Static.Clients[0]
	client.Active = true
	client.Message.Clear()
	if err := inet.DefaultNetwork().SetIPBan("off", ""); err != nil {
		t.Fatalf("clear starting ban: %v", err)
	}
	oldDeathmatch := s.CVar.StringValue("deathmatch")
	s.CVar.Set("deathmatch", "1")
	t.Cleanup(func() {
		s.CVar.Set("deathmatch", oldDeathmatch)
		_ = inet.DefaultNetwork().SetIPBan("off", "")
	})

	if !s.ExecuteClientString(client, "ban 1.2.3.4") {
		t.Fatal("ExecuteClientString(ban in deathmatch) = false, want true")
	}
	if got := inet.DefaultNetwork().IPBanStatus(); got != "Banning not active" {
		t.Fatalf("ban status changed in deathmatch: %q", got)
	}
	if client.Message.Len() != 0 {
		t.Fatalf("deathmatch ban produced unexpected output: %q", string(client.Message.Data[:client.Message.Len()]))
	}
}

func TestKickClientDropsTargetAndWritesReason(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
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
	newServerTestVM(s, 16)
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
	newServerTestVM(s, 16)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}

	s.ConnectClient(0)
	client := s.Static.Clients[0]
	client.Edict.SetHealth(s, 0)

	if ok := s.KillClient(0); ok {
		t.Fatal("KillClient succeeded for dead client")
	}
	if !bytes.Contains(client.Message.Data[:client.Message.Len()], []byte("Can't suicide -- already dead!\n")) {
		t.Fatalf("kill rejection message = %q, want already-dead warning", string(client.Message.Data[:client.Message.Len()]))
	}
}

func TestSetClientNameBroadcastsReliableScoreboardUpdate(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
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
	newServerTestVM(s, 16)
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

	if got := int(s.Static.Clients[0].Edict.Team(s)); got != 4 {
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
	newServerTestVM(s, 16)
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

	if client.Edict.Health(s) <= 0 {
		t.Fatalf("player health = %v, want > 0 after PutClientInServer", client.Edict.Health(s))
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
	if player == nil {
		t.Fatal("client missing spawned edict")
	}
	if got := s.String(player.ClassName(s)); got != "player" {
		t.Fatalf("spawned player classname = %q, want %q", got, "player")
	}

	var trigger *Edict
	var wantLevel string
	for entNum := 1; entNum < s.NumEdicts; entNum++ {
		ent := s.EdictNum(entNum)
		if ent == nil || ent.Free {
			continue
		}
		if s.String(ent.ClassName(s)) != "trigger_changelevel" {
			continue
		}
		trigger = ent
		wantLevel = s.String(ent.Map(s))
		break
	}
	if trigger == nil {
		t.Fatal("no trigger_changelevel found on start")
	}
	if wantLevel == "" {
		t.Fatal("trigger_changelevel missing destination map")
	}

	cs := cmdsys.NewCmdSystem()
	s.Cmd = cs

	var gotLevels []string
	cs.AddCommand("changelevel", func(args []string) {
		if len(args) > 0 {
			gotLevels = append(gotLevels, args[0])
			return
		}
		gotLevels = append(gotLevels, "")
	}, "")
	cs.Execute()
	gotLevels = nil

	player.SetOrigin(s, qtypes.Vec3{
		X: (trigger.AbsMin(s).X + trigger.AbsMax(s).X) * 0.5,
		Y: (trigger.AbsMin(s).Y + trigger.AbsMax(s).Y) * 0.5,
		Z: trigger.AbsMin(s).Z - player.Mins(s).Z + 1,
	})
	player.SetVelocity(s, qtypes.Vec3{})
	player.SetFlags(s, float32(uint32(player.Flags(s))|uint32(FlagOnGround)))
	s.LinkEdict(player, false)
	s.touchLinks(player)
	cs.Execute()

	if len(gotLevels) != 1 {
		t.Fatalf("changelevel executions = %v, want [%q]", gotLevels, wantLevel)
	}
	if gotLevels[0] != wantLevel {
		t.Fatalf("changelevel target = %q, want %q", gotLevels[0], wantLevel)
	}

	// Touch the trigger again before a host-side changelevel reset runs. QC can
	// refire in this window, but the server should guard duplicate enqueueing.
	s.touchLinks(player)
	cs.Execute()
	if len(gotLevels) != 1 {
		t.Fatalf("duplicate trigger touch queued extra changelevel: %v", gotLevels)
	}
}

func TestRunClientSpawnQCRelinksClientAfterQCSpawnMove(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
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
		vm.SetEVector(self, qc.EntFieldOrigin, qtypes.Vec3{X: 128, Y: 0, Z: 0})
		vm.SetEVector(self, qc.EntFieldAngles, qtypes.Vec3{X: 0, Y: 90, Z: 0})
		vm.SetEVector(self, qc.EntFieldVAngle, qtypes.Vec3{X: 0, Y: 90, Z: 0})
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
	trigger.SetOrigin(s, qtypes.Vec3{X: 128, Y: 0, Z: 24})
	trigger.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -24})
	trigger.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 32})
	trigger.SetSolid(s, float32(SolidTrigger))
	s.QCVM.SetEInt(trigger.Num, qc.EntFieldTouch, 2)
	s.LinkEdict(trigger, false)

	if err := s.runClientSpawnQC(client); err != nil {
		t.Fatalf("runClientSpawnQC() error = %v", err)
	}

	if got := client.Edict.Origin(s); got != (qtypes.Vec3{X: 128, Y: 0, Z: 0}) {
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
