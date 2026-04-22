package host

import (
	"encoding/binary"
	stdnet "net"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/menu"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/server"
)

func TestCmdProfileNoOpWithoutActiveServer(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Server: server.NewServer(), Console: console}

	h.CmdProfile(subs)

	if got := strings.Join(console.messages, ""); got != "" {
		t.Fatalf("profile output = %q, want empty when no active local server", got)
	}
}

func TestCmdProfilePrintsTopTenAndClearsCounters(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	srv := server.NewServer()
	srv.Active = true
	h.SetServerActive(true)

	profiles := map[string]int32{
		"func00": 120,
		"func01": 119,
		"func02": 118,
		"func03": 117,
		"func04": 116,
		"func05": 115,
		"func06": 114,
		"func07": 113,
		"func08": 112,
		"func09": 111,
		"func10": 110,
		"func11": 109,
	}
	srv.QCVM = makeQCVMWithProfileResults(profiles)
	subs := &Subsystems{Server: srv, Console: console}

	h.CmdProfile(subs)

	if len(console.messages) != 10 {
		t.Fatalf("profile lines = %d, want 10", len(console.messages))
	}
	output := strings.Join(console.messages, "")
	if !strings.Contains(output, "    120 func00\n") {
		t.Fatalf("profile output missing top function: %q", output)
	}
	if !strings.Contains(output, "    111 func09\n") {
		t.Fatalf("profile output missing tenth function: %q", output)
	}
	if strings.Contains(output, "func10") || strings.Contains(output, "func11") {
		t.Fatalf("profile output includes entries past top 10: %q", output)
	}

	for i, fn := range srv.QCVM.Functions {
		if fn.Profile != 0 {
			t.Fatalf("function %d profile = %d, want 0 after profile command", i, fn.Profile)
		}
	}
}

func TestProfileCommandRegistrationExecutes(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	srv := server.NewServer()
	srv.Active = true
	h.SetServerActive(true)
	srv.QCVM = makeQCVMWithProfileResults(map[string]int32{"qc_profiled": 7})
	subs := &Subsystems{Server: srv, Console: console}

	h.RegisterCommands(subs)
	h.Cmd.ExecuteText("profile")

	if got := strings.Join(console.messages, ""); !strings.Contains(got, "      7 qc_profiled\n") {
		t.Fatalf("profile command output = %q, want formatted QC profile line", got)
	}
}

func TestCmdDevStatsPrintsCStyleTable(t *testing.T) {
	h := NewHost()
	h.SetServerActive(true)
	console := &mockConsole{}
	srv := server.NewServer()
	if err := srv.Init(1); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	srv.Active = true

	subs := &Subsystems{Server: srv, Console: console}
	h.CmdDevStats(subs)

	got := strings.Join(console.messages, "")
	if !strings.Contains(got, "devstats | Curr  Peak\n") {
		t.Fatalf("devstats output missing header:\n%s", got)
	}
	if !strings.Contains(got, "Edicts   |") {
		t.Fatalf("devstats output missing Edicts row:\n%s", got)
	}
	if !strings.Contains(got, "Packet   |") {
		t.Fatalf("devstats output missing Packet row:\n%s", got)
	}
	if !strings.Contains(got, "GL upload|") {
		t.Fatalf("devstats output missing GL upload row:\n%s", got)
	}
}

func TestDevStatsCommandRegistrationExecutes(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	srv := server.NewServer()
	if err := srv.Init(1); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	srv.Active = true
	h.SetServerActive(true)

	subs := &Subsystems{Server: srv, Console: console}
	h.RegisterCommands(subs)
	h.Cmd.ExecuteText("devstats")

	got := strings.Join(console.messages, "")
	if !strings.Contains(got, "devstats | Curr  Peak\n") {
		t.Fatalf("devstats command output = %q, want devstats header", got)
	}
}

func TestCmdChangelevel(t *testing.T) {
	h := NewHost()
	subs := &mockSubsystems{
		server:  &mockServer{},
		client:  &mockClient{},
		console: &mockConsole{},
	}
	subs.Subsystems.Server = subs.server
	subs.Subsystems.Client = subs.client
	subs.Subsystems.Console = subs.console

	h.Init(&InitParams{BaseDir: "."}, &subs.Subsystems)
	h.SetServerActive(true)

	h.CmdChangelevel("start", &subs.Subsystems)
	// For now, we just check if it doesn't crash and maybe logs something
	// Once implemented, we can check for state changes
}

func TestCmdRestart(t *testing.T) {
	h := NewHost()
	subs := &mockSubsystems{
		server:  &mockServer{},
		client:  &mockClient{},
		console: &mockConsole{},
	}
	subs.Subsystems.Server = subs.server
	subs.Subsystems.Client = subs.client
	subs.Subsystems.Console = subs.console

	h.Init(&InitParams{BaseDir: "."}, &subs.Subsystems)
	h.SetServerActive(true)

	h.CmdRestart(&subs.Subsystems)
}

func TestCmdRestartPromptAutoloadShowsConfirmationMenu(t *testing.T) {
	h := NewHost()
	subs := &mockSubsystems{
		server:  &mockServer{active: true},
		client:  &mockClient{},
		console: &mockConsole{},
	}
	subs.Subsystems.Server = subs.server
	subs.Subsystems.Client = subs.client
	subs.Subsystems.Console = subs.console

	if err := h.Init(&InitParams{BaseDir: ".", UserDir: t.TempDir()}, &subs.Subsystems); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	h.SetServerActive(true)
	h.lastSave = "slot1"
	mgr := menu.NewManager(nil, nil, nil)
	h.SetMenu(mgr)

	previousAutoload := h.CVar.StringValue("sv_autoload")
	h.CVar.Set("sv_autoload", "1")
	t.Cleanup(func() {
		h.CVar.Set("sv_autoload", previousAutoload)
	})

	h.CmdRestart(&subs.Subsystems)

	if !mgr.IsActive() {
		t.Fatal("menu should be active for prompt autoload")
	}
	if got := mgr.GetState(); got != menu.MenuQuit {
		t.Fatalf("menu state = %v, want %v", got, menu.MenuQuit)
	}
	if got := strings.Join(subs.console.messages, ""); strings.Contains(got, "Autoloading...") {
		t.Fatalf("console output = %q, want no immediate autoload", got)
	}
}

func TestCmdRestartPromptAutoloadConfirmLoadsLastSave(t *testing.T) {
	h := NewHost()
	subs := &mockSubsystems{
		server:  &mockServer{active: true},
		client:  &mockClient{},
		console: &mockConsole{},
	}
	subs.Subsystems.Server = subs.server
	subs.Subsystems.Client = subs.client
	subs.Subsystems.Console = subs.console

	if err := h.Init(&InitParams{BaseDir: ".", UserDir: t.TempDir()}, &subs.Subsystems); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	h.SetServerActive(true)
	h.lastSave = "slot1"
	mgr := menu.NewManager(nil, nil, nil)
	h.SetMenu(mgr)

	previousAutoload := h.CVar.StringValue("sv_autoload")
	h.CVar.Set("sv_autoload", "1")
	t.Cleanup(func() {
		h.CVar.Set("sv_autoload", previousAutoload)
	})

	h.CmdRestart(&subs.Subsystems)
	mgr.M_Key('y')

	if mgr.IsActive() {
		t.Fatal("menu should hide after confirming autoload prompt")
	}
	if got := strings.Join(subs.console.messages, ""); !strings.Contains(got, "ERROR: slot1.sav not found.") {
		t.Fatalf("console output = %q, want prompted load failure", got)
	}
	if h.lastSave != "" {
		t.Fatalf("lastSave = %q, want cleared after missing prompted load", h.lastSave)
	}
}

func TestCmdRestartPromptAutoloadDeclineClearsLastSave(t *testing.T) {
	h := NewHost()
	subs := &mockSubsystems{
		server:  &mockServer{active: true},
		client:  &mockClient{},
		console: &mockConsole{},
	}
	subs.Subsystems.Server = subs.server
	subs.Subsystems.Client = subs.client
	subs.Subsystems.Console = subs.console

	if err := h.Init(&InitParams{BaseDir: ".", UserDir: t.TempDir()}, &subs.Subsystems); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	h.SetServerActive(true)
	h.lastSave = "slot1"
	mgr := menu.NewManager(nil, nil, nil)
	h.SetMenu(mgr)

	previousAutoload := h.CVar.StringValue("sv_autoload")
	h.CVar.Set("sv_autoload", "1")
	t.Cleanup(func() {
		h.CVar.Set("sv_autoload", previousAutoload)
	})

	h.CmdRestart(&subs.Subsystems)
	mgr.M_Key('n')

	if mgr.IsActive() {
		t.Fatal("menu should hide after declining autoload prompt")
	}
	if h.lastSave != "" {
		t.Fatalf("lastSave = %q, want cleared after declining prompt", h.lastSave)
	}
}

func TestCmdKill(t *testing.T) {
	h := NewHost()
	server := &killTrackingServer{}
	subs := &mockSubsystems{
		server:  &server.mockServer,
		client:  &mockClient{},
		console: &mockConsole{},
	}
	subs.Subsystems.Server = server
	subs.Subsystems.Client = subs.client
	subs.Subsystems.Console = subs.console

	h.Init(&InitParams{BaseDir: "."}, &subs.Subsystems)
	h.SetServerActive(true)

	h.CmdKill(&subs.Subsystems)
	if len(server.killCalls) != 1 || server.killCalls[0] != 0 {
		t.Fatalf("KillClient calls = %v, want [0]", server.killCalls)
	}
}

func TestCmdGod(t *testing.T) {
	h := NewHost()
	subs := &mockSubsystems{
		server:  &mockServer{},
		client:  &mockClient{},
		console: &mockConsole{},
	}
	subs.Subsystems.Server = subs.server
	subs.Subsystems.Client = subs.client
	subs.Subsystems.Console = subs.console

	h.Init(&InitParams{BaseDir: "."}, &subs.Subsystems)
	h.SetServerActive(true)

	h.CmdGod(&subs.Subsystems)
}

func TestCmdNoClip(t *testing.T) {
	h := NewHost()
	subs := &mockSubsystems{
		server:  &mockServer{},
		client:  &mockClient{},
		console: &mockConsole{},
	}
	subs.Subsystems.Server = subs.server
	subs.Subsystems.Client = subs.client
	subs.Subsystems.Console = subs.console

	h.Init(&InitParams{BaseDir: "."}, &subs.Subsystems)
	h.SetServerActive(true)

	h.CmdNoClip(&subs.Subsystems)
}

func TestCmdNotarget(t *testing.T) {
	h := NewHost()
	subs := &mockSubsystems{
		server:  &mockServer{},
		client:  &mockClient{},
		console: &mockConsole{},
	}
	subs.Subsystems.Server = subs.server
	subs.Subsystems.Client = subs.client
	subs.Subsystems.Console = subs.console

	h.Init(&InitParams{BaseDir: "."}, &subs.Subsystems)
	h.SetServerActive(true)

	h.CmdNotarget(&subs.Subsystems)
}

func TestCmdGive(t *testing.T) {
	h := NewHost()
	subs := &mockSubsystems{
		server:  &mockServer{},
		client:  &mockClient{},
		console: &mockConsole{},
	}
	subs.Subsystems.Server = subs.server
	subs.Subsystems.Client = subs.client
	subs.Subsystems.Console = subs.console

	h.Init(&InitParams{BaseDir: "."}, &subs.Subsystems)
	h.SetServerActive(true)

	h.CmdGive("all", "", &subs.Subsystems)
}

func TestCmdName(t *testing.T) {
	h := NewHost()
	srv := &nameTrackingServer{}
	subs := &mockSubsystems{
		server:  &srv.mockServer,
		client:  &mockClient{},
		console: &mockConsole{},
	}
	subs.Subsystems.Server = srv
	subs.Subsystems.Client = subs.client
	subs.Subsystems.Console = subs.console

	h.Init(&InitParams{BaseDir: "."}, &subs.Subsystems)
	oldName := h.CVar.StringValue(clientNameCVar)
	t.Cleanup(func() {
		h.CVar.Set(clientNameCVar, oldName)
	})

	h.CmdName("Player", &subs.Subsystems)
	if got := srv.lastName; got != "Player" {
		t.Fatalf("server name = %q, want %q", got, "Player")
	}
	if got := h.CVar.StringValue(clientNameCVar); got != "Player" {
		t.Fatalf("%s = %q, want %q", clientNameCVar, got, "Player")
	}
}

func TestCmdColor(t *testing.T) {
	h := NewHost()
	srv := &colorTrackingServer{}
	subs := &Subsystems{
		Server:  srv,
		Client:  &mockClient{},
		Console: &mockConsole{},
	}

	h.Init(&InitParams{BaseDir: "."}, subs)
	oldColor := h.CVar.StringValue(clientColorCVar)
	t.Cleanup(func() {
		h.CVar.Set(clientColorCVar, oldColor)
	})

	h.CmdColor([]string{"13"}, subs)
	if got := srv.lastColor; got != 221 {
		t.Fatalf("single-arg color = %d, want 221", got)
	}
	if got := h.CVar.IntValue(clientColorCVar); got != 221 {
		t.Fatalf("%s = %d, want 221", clientColorCVar, got)
	}

	h.CmdColor([]string{"1", "2"}, subs)
	if got := srv.lastColor; got != 18 {
		t.Fatalf("two-arg color = %d, want 18", got)
	}
	if got := h.CVar.IntValue(clientColorCVar); got != 18 {
		t.Fatalf("%s = %d, want 18", clientColorCVar, got)
	}
}

func TestCmdServerInfoIncludesHostname(t *testing.T) {
	h := NewHost()
	subs := &mockSubsystems{
		server:  &mockServer{},
		client:  &mockClient{},
		console: &mockConsole{},
	}
	subs.Subsystems.Server = subs.server
	subs.Subsystems.Client = subs.client
	subs.Subsystems.Console = subs.console

	h.Init(&InitParams{BaseDir: "."}, &subs.Subsystems)
	oldHostname := h.CVar.StringValue(serverHostnameCVar)
	t.Cleanup(func() {
		h.CVar.Set(serverHostnameCVar, oldHostname)
	})
	h.CVar.Set(serverHostnameCVar, "LAN Party")

	h.CmdServerInfo(&subs.Subsystems)

	got := strings.Join(subs.console.messages, "")
	if !strings.Contains(got, "host:      LAN Party\n") {
		t.Fatalf("serverinfo output missing hostname in:\n%s", got)
	}
}

func TestCmdPing(t *testing.T) {
	h := NewHost()
	subs := &mockSubsystems{
		server:  &mockServer{},
		client:  &mockClient{},
		console: &mockConsole{},
	}
	subs.Subsystems.Server = subs.server
	subs.Subsystems.Client = subs.client
	subs.Subsystems.Console = subs.console

	h.Init(&InitParams{BaseDir: "."}, &subs.Subsystems)

	h.CmdPing(&subs.Subsystems)
}

func TestCmdTest2PrintsQueriedRules(t *testing.T) {
	serverConn, err := stdnet.ListenUDP("udp4", &stdnet.UDPAddr{IP: stdnet.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer serverConn.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 1024)
		for {
			n, addr, err := serverConn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			if n < inet.HeaderSize+1 || buf[8] != inet.CCReqRuleInfo {
				continue
			}
			prev := strings.TrimRight(string(buf[9:n]), "\x00")
			var resp []byte
			switch prev {
			case "":
				resp = buildHostRuleInfoResponse("deathmatch", "1")
			case "deathmatch":
				resp = buildHostRuleInfoResponse("teamplay", "0")
			default:
				resp = buildHostRuleInfoResponse("", "")
			}
			serverConn.WriteToUDP(resp, addr)
		}
	}()

	h := NewHost()
	subs := &mockSubsystems{
		console: &mockConsole{},
	}
	subs.Subsystems.Console = subs.console

	h.CmdTest2(serverConn.LocalAddr().String(), &subs.Subsystems)

	got := strings.Join(subs.console.messages, "")
	if !strings.Contains(got, "deathmatch") || !strings.Contains(got, "teamplay") {
		t.Fatalf("test2 output missing expected rules:\n%s", got)
	}
}

func TestCmdPlayersPrintsQueriedPlayers(t *testing.T) {
	serverConn, err := stdnet.ListenUDP("udp4", &stdnet.UDPAddr{IP: stdnet.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer serverConn.Close()

	go func() {
		buf := make([]byte, 1024)
		for {
			n, addr, err := serverConn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			if n < inet.HeaderSize+2 || buf[8] != inet.CCReqPlayerInfo {
				continue
			}
			switch buf[9] {
			case 0:
				serverConn.WriteToUDP(buildHostPlayerInfoResponse(0, "Ranger", 0x49, 15, 32, "10.0.0.2:26000"), addr)
			case 1:
				serverConn.WriteToUDP(buildHostPlayerInfoResponse(1, "Shambler", 0xdd, 42, 60, "10.0.0.3:26000"), addr)
			default:
				serverConn.WriteToUDP(buildHostPlayerInfoResponse(buf[9], "", 0, 0, 0, ""), addr)
			}
		}
	}()

	h := NewHost()
	subs := &mockSubsystems{
		console: &mockConsole{},
	}
	subs.Subsystems.Console = subs.console

	h.CmdPlayers(serverConn.LocalAddr().String(), &subs.Subsystems)

	got := strings.Join(subs.console.messages, "")
	if !strings.Contains(got, "slot  name              color  frags  ping") {
		t.Fatalf("players output missing header:\n%s", got)
	}
	if !strings.Contains(got, "Ranger") || !strings.Contains(got, "Shambler") {
		t.Fatalf("players output missing expected player names:\n%s", got)
	}
}

func TestCmdNetStatsPrintsGlobalDatagramCounters(t *testing.T) {
	h := NewHost()
	stats := h.NetStats()
	stats.Reset()
	t.Cleanup(stats.Reset)

	stats.UnreliableSent.Store(11)
	stats.ReliableReceived.Store(7)
	stats.DroppedDatagrams.Store(3)
	subs := &mockSubsystems{
		console: &mockConsole{},
	}
	subs.Subsystems.Console = subs.console

	h.CmdNetStats(&subs.Subsystems)

	got := strings.Join(subs.console.messages, "")
	if !strings.Contains(got, "unreliable messages sent   = 11\n") {
		t.Fatalf("net_stats output missing unreliable sent count:\n%s", got)
	}
	if !strings.Contains(got, "reliable messages received = 7\n") {
		t.Fatalf("net_stats output missing reliable received count:\n%s", got)
	}
	if !strings.Contains(got, "droppedDatagrams           = 3\n") {
		t.Fatalf("net_stats output missing dropped datagrams count:\n%s", got)
	}
}

func TestCmdSlistPrintsCStyleHeaderEntriesAndTrailer(t *testing.T) {
	h := NewHost()
	subs := &Subsystems{Console: &mockConsole{}}
	console := subs.Console.(*mockConsole)

	oldFactory := h.ServerBrowserFactory
	t.Cleanup(func() { h.ServerBrowserFactory = oldFactory })
	h.ServerBrowserFactory = func() serverBrowser {
		return &fakeServerBrowser{
			results: []inet.HostCacheEntry{
				{Name: "LAN Test", Map: "e1m1", Players: 2, MaxPlayers: 8, Address: "127.0.0.1:26000"},
				{Name: "NoSlots", Map: "start", Players: 0, MaxPlayers: 0, Address: "127.0.0.1:26001"},
			},
		}
	}

	h.CmdSlist(subs)

	got := strings.Join(console.messages, "")
	if !strings.Contains(got, "Looking for Quake servers...\n") {
		t.Fatalf("slist output missing banner:\n%s", got)
	}
	if !strings.Contains(got, "Server          Map             Users\n") {
		t.Fatalf("slist output missing header:\n%s", got)
	}
	if !strings.Contains(got, "--------------- --------------- -----\n") {
		t.Fatalf("slist output missing separator:\n%s", got)
	}
	if !strings.Contains(got, "LAN Test        e1m1             2/ 8\n") {
		t.Fatalf("slist output missing users row:\n%s", got)
	}
	if !strings.Contains(got, "NoSlots         start          \n") {
		t.Fatalf("slist output missing zero-max row:\n%s", got)
	}
	if !strings.Contains(got, "== end list ==\n\n") {
		t.Fatalf("slist output missing trailer:\n%s", got)
	}
}

func TestCmdSlistPrintsNoServersMessageWhenEmpty(t *testing.T) {
	h := NewHost()
	subs := &Subsystems{Console: &mockConsole{}}
	console := subs.Console.(*mockConsole)

	oldFactory := h.ServerBrowserFactory
	t.Cleanup(func() { h.ServerBrowserFactory = oldFactory })
	h.ServerBrowserFactory = func() serverBrowser {
		return &fakeServerBrowser{}
	}

	h.CmdSlist(subs)

	got := strings.Join(console.messages, "")
	if !strings.Contains(got, "Looking for Quake servers...\n") {
		t.Fatalf("slist output missing banner:\n%s", got)
	}
	if !strings.Contains(got, "No Quake servers found.\n\n") {
		t.Fatalf("slist output missing empty message:\n%s", got)
	}
}

func buildHostRuleInfoResponse(name, value string) []byte {
	payloadLen := 1
	if name != "" {
		payloadLen += len(name) + 1 + len(value) + 1
	}
	buf := make([]byte, inet.HeaderSize+payloadLen)
	binary.BigEndian.PutUint32(buf[0:], uint32(len(buf))|inet.FlagCtl)
	binary.BigEndian.PutUint32(buf[4:], 0xffffffff)
	buf[8] = inet.CCRepRuleInfo
	if name != "" {
		copy(buf[9:], name)
		buf[9+len(name)] = 0
		copy(buf[10+len(name):], value)
	}
	return buf
}

func buildHostPlayerInfoResponse(slot byte, name string, colors byte, frags int32, ping int32, address string) []byte {
	payloadLen := 2 // slot + empty name terminator
	if name != "" {
		payloadLen += len(name) + 1 + 4 + 4 + 4 + len(address) + 1
	}
	buf := make([]byte, inet.HeaderSize+1+payloadLen)
	binary.BigEndian.PutUint32(buf[0:], uint32(len(buf))|inet.FlagCtl)
	binary.BigEndian.PutUint32(buf[4:], 0xffffffff)
	buf[8] = inet.CCRepPlayerInfo
	buf[9] = slot
	if name != "" {
		copy(buf[10:], name)
		nameEnd := 10 + len(name)
		buf[nameEnd] = 0
		binary.LittleEndian.PutUint32(buf[nameEnd+1:], uint32(colors))
		binary.LittleEndian.PutUint32(buf[nameEnd+5:], uint32(frags))
		binary.LittleEndian.PutUint32(buf[nameEnd+9:], uint32(ping))
		copy(buf[nameEnd+13:], address)
	}
	return buf
}

func TestCmdKickBySlot(t *testing.T) {
	h := NewHost()
	srv := newKickTrackingServer("Ranger", "Grunt")
	subs := &Subsystems{Server: srv}

	h.SetServerActive(true)
	h.CmdKick([]string{"#", "2"}, subs)

	if len(srv.kicks) != 1 {
		t.Fatalf("kick count = %d, want 1", len(srv.kicks))
	}
	if got := srv.kicks[0].clientNum; got != 1 {
		t.Fatalf("kicked slot = %d, want 1", got)
	}
}

func TestCmdKickByNameCaseInsensitive(t *testing.T) {
	h := NewHost()
	srv := newKickTrackingServer("Ranger", "Grunt")
	subs := &Subsystems{Server: srv}

	h.SetServerActive(true)
	h.CmdKick([]string{"gRuNt"}, subs)

	if len(srv.kicks) != 1 {
		t.Fatalf("kick count = %d, want 1", len(srv.kicks))
	}
	if got := srv.kicks[0].clientNum; got != 1 {
		t.Fatalf("kicked slot = %d, want 1", got)
	}
}

func TestCmdKickIncludesMessage(t *testing.T) {
	h := NewHost()
	srv := newKickTrackingServer("Ranger", "Grunt")
	subs := &Subsystems{Server: srv}

	h.SetServerActive(true)
	h.CmdKick([]string{"#", "2", "watch", "your", "step"}, subs)

	if len(srv.kicks) != 1 {
		t.Fatalf("kick count = %d, want 1", len(srv.kicks))
	}
	if got := srv.kicks[0].reason; got != "watch your step" {
		t.Fatalf("kick reason = %q, want %q", got, "watch your step")
	}
}

func TestCmdKickPreventsSelfKick(t *testing.T) {
	h := NewHost()
	srv := newKickTrackingServer("Ranger", "Grunt")
	subs := &Subsystems{Server: srv}

	h.SetServerActive(true)
	h.CmdKick([]string{"#", "1"}, subs)
	h.CmdKick([]string{"ranger"}, subs)

	if len(srv.kicks) != 0 {
		t.Fatalf("kick count = %d, want 0", len(srv.kicks))
	}
}

func TestCmdKickUnknownTargetNoOp(t *testing.T) {
	h := NewHost()
	srv := newKickTrackingServer("Ranger", "Grunt")
	subs := &Subsystems{Server: srv}

	h.SetServerActive(true)
	h.CmdKick([]string{"#", "99"}, subs)
	h.CmdKick([]string{"ogre"}, subs)

	if len(srv.kicks) != 0 {
		t.Fatalf("kick count = %d, want 0", len(srv.kicks))
	}
}
