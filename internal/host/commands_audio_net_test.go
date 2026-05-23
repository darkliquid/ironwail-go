package host

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/cmdsys"
	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/server"
)

func TestCmdPlayAppendsWAVAndLoadsFromSoundDir(t *testing.T) {
	h := NewHost()
	audio := &audioCommandTracking{}
	files := &audioCommandFiles{loaded: map[string][]byte{
		"sound/misc/menu1.wav": []byte("menu1"),
	}}
	subs := &Subsystems{
		Audio: audio,
		Files: files,
	}

	h.CmdPlay([]string{"misc/menu1"}, subs)

	if len(audio.playedSounds) != 1 {
		t.Fatalf("played sound count = %d, want 1", len(audio.playedSounds))
	}
	if got := audio.playedSounds[0].name; got != "misc/menu1.wav" {
		t.Fatalf("played sound name = %q, want misc/menu1.wav", got)
	}
	if got := string(audio.playedSounds[0].data); got != "menu1" {
		t.Fatalf("loaded data = %q, want menu1", got)
	}
	if got := files.calls; !reflect.DeepEqual(got, []string{"sound/misc/menu1.wav"}) {
		t.Fatalf("LoadFile calls = %v, want [sound/misc/menu1.wav]", got)
	}
}

func TestCmdPlayVolUsesExplicitVolumes(t *testing.T) {
	h := NewHost()
	audio := &audioCommandTracking{}
	files := &audioCommandFiles{loaded: map[string][]byte{
		"sound/misc/menu1.wav": []byte("one"),
		"sound/misc/menu2.wav": []byte("two"),
	}}
	subs := &Subsystems{
		Audio:   audio,
		Files:   files,
		Console: &mockConsole{},
	}

	h.CmdPlayVol([]string{"misc/menu1", "0.25", "misc/menu2.wav", "0.5"}, subs)

	if len(audio.playedSounds) != 2 {
		t.Fatalf("played sound count = %d, want 2", len(audio.playedSounds))
	}
	if got := audio.playedSounds[0].vol; got != 0.25 {
		t.Fatalf("first volume = %v, want 0.25", got)
	}
	if got := audio.playedSounds[1].name; got != "misc/menu2.wav" {
		t.Fatalf("second sound name = %q, want misc/menu2.wav", got)
	}
	if got := audio.playedSounds[1].vol; got != 0.5 {
		t.Fatalf("second volume = %v, want 0.5", got)
	}
}

func TestCmdSoundlistPrintsAudioListing(t *testing.T) {
	h := NewHost()
	audio := &audioCommandTracking{soundList: "L(16b)    128 : misc/menu1.wav\n1 sounds, 128 bytes\n"}
	console := &mockConsole{}
	subs := &Subsystems{
		Audio:   audio,
		Console: console,
	}

	h.CmdSoundlist(subs)

	if got := strings.Join(console.messages, ""); got != audio.soundList {
		t.Fatalf("console output = %q, want %q", got, audio.soundList)
	}
}

func TestCmdMusicWithoutArgsReportsCurrentTrack(t *testing.T) {
	h := NewHost()
	audio := &audioCommandTracking{}
	audio.currentMusic = "music/track02.ogg"
	console := &mockConsole{}
	subs := &Subsystems{
		Audio:   audio,
		Console: console,
	}

	h.CmdMusic(nil, subs)

	if got := strings.Join(console.messages, ""); got != "Playing track02, use 'music <musicfile>' to change\n" {
		t.Fatalf("console output = %q", got)
	}
}

func TestCmdMusicLoopToggleMatchesCanonicalMessages(t *testing.T) {
	h := NewHost()
	audio := &audioCommandTracking{}
	console := &mockConsole{}
	subs := &Subsystems{
		Audio:   audio,
		Console: console,
	}

	h.CmdMusicLoop([]string{"toggle"}, subs)
	if got := strings.Join(console.messages, ""); got != "Music will be looped\n" {
		t.Fatalf("toggle output = %q, want looped message", got)
	}

	console.Clear()
	h.CmdMusicLoop([]string{"off"}, subs)
	if got := strings.Join(console.messages, ""); got != "Music will not be looped\n" {
		t.Fatalf("off output = %q, want not-looped message", got)
	}
}

func TestCmdMusicJumpPrintsUsageOnInvalidArgs(t *testing.T) {
	h := NewHost()
	audio := &audioCommandTracking{}
	console := &mockConsole{}
	subs := &Subsystems{
		Audio:   audio,
		Console: console,
	}

	h.CmdMusicJump(nil, subs)

	if got := strings.Join(console.messages, ""); got != "music_jump <ordernum>\n" {
		t.Fatalf("console output = %q, want usage", got)
	}
}

func TestCmdFogWithoutArgsPrintsUsageAndCurrentValues(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	client := newLocalLoopbackClient()
	client.inner.FogDensity = 128
	client.inner.FogColor = [3]byte{64, 128, 255}
	subs := &Subsystems{
		Client:  client,
		Console: console,
	}

	h.CmdFog(nil, subs)

	got := strings.Join(console.messages, "")
	if !strings.Contains(got, "usage:\n") {
		t.Fatalf("fog usage missing in %q", got)
	}
	if !strings.Contains(got, "\"density\" is \"0.5019608\"") {
		t.Fatalf("fog density line missing in %q", got)
	}
	if !strings.Contains(got, "\"blue\"    is \"1\"") {
		t.Fatalf("fog blue line missing in %q", got)
	}
}

func TestCmdFogDensityOnlyPreservesColor(t *testing.T) {
	h := NewHost()
	client := newLocalLoopbackClient()
	client.inner.FogColor = [3]byte{51, 102, 153}
	subs := &Subsystems{
		Client:  client,
		Console: &mockConsole{},
	}

	h.CmdFog([]string{"0.25"}, subs)

	if got := client.inner.FogDensity; got != 64 {
		t.Fatalf("FogDensity = %d, want 64", got)
	}
	if got := client.inner.FogColor; got != [3]byte{51, 102, 153} {
		t.Fatalf("FogColor = %v, want preserved [51 102 153]", got)
	}
	if got := client.inner.FogTime; got != 0 {
		t.Fatalf("FogTime = %v, want 0", got)
	}
}

func TestCmdFogRGBOnlyPreservesDensity(t *testing.T) {
	h := NewHost()
	client := newLocalLoopbackClient()
	client.inner.FogDensity = 200
	subs := &Subsystems{
		Client:  client,
		Console: &mockConsole{},
	}

	h.CmdFog([]string{"0.1", "0.2", "0.3"}, subs)

	if got := client.inner.FogDensity; got != 200 {
		t.Fatalf("FogDensity = %d, want preserved 200", got)
	}
	if got := client.inner.FogColor; got != [3]byte{26, 51, 77} {
		t.Fatalf("FogColor = %v, want [26 51 77]", got)
	}
}

func TestCmdFogDensityRGBTimeClampsInputs(t *testing.T) {
	h := NewHost()
	client := newLocalLoopbackClient()
	client.inner.Time = 2
	client.inner.SetFogState(255, [3]byte{255, 128, 0}, 4)
	client.inner.Time = 4
	subs := &Subsystems{
		Client:  client,
		Console: &mockConsole{},
	}

	h.CmdFog([]string{"-1", "-0.5", "2", "0.4", "1.5"}, subs)

	if got := client.inner.FogDensity; got != 0 {
		t.Fatalf("FogDensity = %d, want clamped 0", got)
	}
	if got := client.inner.FogColor; got != [3]byte{0, 255, 102} {
		t.Fatalf("FogColor = %v, want [0 255 102]", got)
	}
	if got := client.inner.FogTime; got != 1.5 {
		t.Fatalf("FogTime = %v, want 1.5", got)
	}
	currentDensity, currentColor := client.inner.CurrentFog()
	if currentDensity < 0.49 || currentDensity > 0.51 {
		t.Fatalf("CurrentFog density = %v, want ~0.5", currentDensity)
	}
	if currentColor[0] < 0.49 || currentColor[0] > 0.51 || currentColor[1] < 0.24 || currentColor[1] > 0.26 || currentColor[2] != 0 {
		t.Fatalf("CurrentFog color = %v, want previous in-flight fade color", currentColor)
	}
}

func TestRegisteredFogCommandAcceptsServerStuffText(t *testing.T) {
	h := NewHost()
	client := newLocalLoopbackClient()
	subs := &Subsystems{
		Client:  client,
		Console: &mockConsole{},
	}
	h.RegisterCommands(subs)

	h.Cmd.ExecuteTextWithSource("fog 0.03 0.3 0.15 0.2\n", cmdsys.SrcServer)

	if got := client.inner.FogDensity; got != 8 {
		t.Fatalf("FogDensity = %d, want rounded 0.03 density byte 8", got)
	}
	if got := client.inner.FogColor; got != [3]byte{77, 38, 51} {
		t.Fatalf("FogColor = %v, want rounded qbj3 fog color [77 38 51]", got)
	}
}

func TestCmdStatusForwardsToRemoteServerWhenNoLocalServer(t *testing.T) {
	h := NewHost()
	client := &forwardingTrackingClient{state: caActive}
	subs := &Subsystems{
		Client:  client,
		Console: &mockConsole{},
	}

	h.CmdStatus(subs)

	if got := client.commands; !reflect.DeepEqual(got, []string{"status"}) {
		t.Fatalf("forwarded commands = %v, want [status]", got)
	}
}

func TestListenCommandRegistrationExecutes(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console}

	_ = h.Net.Init()
	t.Cleanup(h.Net.Shutdown)
	_ = h.Net.Listen(false)

	h.RegisterCommands(subs)
	h.Cmd.ExecuteText("listen")

	if got := strings.Join(console.messages, ""); !strings.Contains(got, "\"listen\" is \"0\"") {
		t.Fatalf("listen query output = %q", got)
	}
}

func TestCmdListenQueryAndToggle(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console}

	port := testFreeUDPPort(t)
	_ = h.Net.Init()
	t.Cleanup(h.Net.Shutdown)
	h.Net.SetHostPort(port)
	_ = h.Net.Listen(false)

	h.CmdListen(nil, subs)
	if got := strings.Join(console.messages, ""); got != "\"listen\" is \"0\"\n" {
		t.Fatalf("listen query output = %q, want disabled state", got)
	}

	h.CmdListen([]string{"1"}, subs)
	if !h.Net.IsListening() {
		t.Fatal("expected listening enabled")
	}

	h.CmdListen([]string{"0"}, subs)
	if h.Net.IsListening() {
		t.Fatal("expected listening disabled")
	}
}

func TestCmdMaxPlayersQuerySetAndDeathmatch(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console}

	oldDeathmatch := h.CVar.StringValue("deathmatch")
	oldMaxPlayers := h.CVar.StringValue("maxplayers")
	t.Cleanup(func() {
		h.CVar.Set("deathmatch", oldDeathmatch)
		h.CVar.Set("maxplayers", oldMaxPlayers)
	})

	h.maxClients = 1
	h.CVar.Set("maxplayers", "1")
	h.CVar.Set("deathmatch", "0")

	h.CmdMaxPlayers(nil, subs)
	if got := strings.Join(console.messages, ""); got != "\"maxplayers\" is \"1\"\n" {
		t.Fatalf("maxplayers query output = %q", got)
	}

	console.messages = nil
	h.CmdMaxPlayers([]string{"4"}, subs)
	if got := h.MaxClients(); got != 4 {
		t.Fatalf("maxclients = %d, want 4", got)
	}
	if got := h.CVar.IntValue("maxplayers"); got != 4 {
		t.Fatalf("maxplayers cvar = %d, want 4", got)
	}
	if got := h.CVar.IntValue("deathmatch"); got != 1 {
		t.Fatalf("deathmatch = %d, want 1", got)
	}
	if len(console.messages) != 0 {
		t.Fatalf("unexpected console output: %q", strings.Join(console.messages, ""))
	}
}

func TestCmdMaxPlayersRejectsWhenServerRunning(t *testing.T) {
	h := NewHost()
	h.maxClients = 2
	h.SetServerActive(true)
	console := &mockConsole{}
	subs := &Subsystems{Console: console}

	h.CmdMaxPlayers([]string{"8"}, subs)

	if got := h.MaxClients(); got != 2 {
		t.Fatalf("maxclients changed to %d while server active", got)
	}
	if got := strings.Join(console.messages, ""); got != "maxplayers can not be changed while a server is running.\n" {
		t.Fatalf("console output = %q", got)
	}
}

func TestCmdMaxPlayersQueuesListenTransition(t *testing.T) {
	h := NewHost()
	h.maxClients = 1
	cmdBuf := &insertTrackingCommandBuffer{}
	subs := &Subsystems{Commands: cmdBuf}

	port := testFreeUDPPort(t)
	_ = h.Net.Init()
	t.Cleanup(h.Net.Shutdown)
	h.Net.SetHostPort(port)
	_ = h.Net.Listen(false)

	h.CmdMaxPlayers([]string{"3"}, subs)
	if got := cmdBuf.added; !reflect.DeepEqual(got, []string{"listen 1\n"}) {
		t.Fatalf("queued commands = %v, want [listen 1\\n]", got)
	}

	cmdBuf.added = nil
	if err := h.Net.Listen(true); err != nil {
		t.Fatalf("Listen(true): %v", err)
	}
	h.CmdMaxPlayers([]string{"1"}, subs)
	if got := cmdBuf.added; !reflect.DeepEqual(got, []string{"listen 0\n"}) {
		t.Fatalf("queued commands = %v, want [listen 0\\n]", got)
	}
}

func TestCmdPortQuerySetValidationAndListenRestart(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	cmdBuf := &insertTrackingCommandBuffer{}
	subs := &Subsystems{Console: console, Commands: cmdBuf}

	oldPort := h.Net.HostPort()
	t.Cleanup(func() { h.Net.SetHostPort(oldPort) })

	port := testFreeUDPPort(t)
	_ = h.Net.Init()
	t.Cleanup(h.Net.Shutdown)
	_ = h.Net.Listen(false)
	h.Net.SetHostPort(port)

	h.CmdPort(nil, subs)
	if got := strings.Join(console.messages, ""); got != fmt.Sprintf("\"port\" is \"%d\"\n", port) {
		t.Fatalf("port query output = %q", got)
	}

	console.messages = nil
	h.CmdPort([]string{"70000"}, subs)
	if got := strings.Join(console.messages, ""); got != "Bad value, must be between 1 and 65534\n" {
		t.Fatalf("invalid port output = %q", got)
	}

	newPort := testFreeUDPPort(t)
	h.CmdPort([]string{strconv.Itoa(newPort)}, subs)
	if got := h.Net.HostPort(); got != newPort {
		t.Fatalf("host port = %d, want %d", got, newPort)
	}

	if err := h.Net.Listen(true); err != nil {
		t.Fatalf("Listen(true): %v", err)
	}
	cmdBuf.added = nil
	nextPort := testFreeUDPPort(t)
	h.CmdPort([]string{strconv.Itoa(nextPort)}, subs)
	if got := h.Net.HostPort(); got != nextPort {
		t.Fatalf("host port after change = %d, want %d", got, nextPort)
	}
	if got := cmdBuf.added; !reflect.DeepEqual(got, []string{"listen 0\n", "listen 1\n"}) {
		t.Fatalf("queued commands = %v, want [listen 0\\n listen 1\\n]", got)
	}
}

func TestCmdNameUpdatesCVarAndForwardsWhenRemoteConnected(t *testing.T) {
	h := NewHost()
	client := &forwardingTrackingClient{state: caConnected}
	subs := &Subsystems{
		Client:  client,
		Console: &mockConsole{},
	}

	h.CmdName("Ranger", subs)

	if got := h.CVar.StringValue(clientNameCVar); got != "Ranger" {
		t.Fatalf("%s = %q, want Ranger", clientNameCVar, got)
	}
	if got := client.commands; !reflect.DeepEqual(got, []string{"name Ranger"}) {
		t.Fatalf("forwarded commands = %v, want [name Ranger]", got)
	}
}

func TestCmdStatusForwardsWithInitializedInactiveServer(t *testing.T) {
	h := NewHost()
	client := &forwardingTrackingClient{state: caActive}
	subs := &Subsystems{
		Server:  server.NewServer(),
		Client:  client,
		Console: &mockConsole{},
	}
	if err := h.Init(&InitParams{BaseDir: "."}, subs); err != nil {
		t.Fatalf("Init: %v", err)
	}

	h.CmdStatus(subs)

	if got := client.commands; !reflect.DeepEqual(got, []string{"status"}) {
		t.Fatalf("forwarded commands = %v, want [status]", got)
	}
}

func TestCmdNameForwardsWithInitializedInactiveServer(t *testing.T) {
	h := NewHost()
	client := &forwardingTrackingClient{state: caConnected}
	subs := &Subsystems{
		Server:  server.NewServer(),
		Client:  client,
		Console: &mockConsole{},
	}
	if err := h.Init(&InitParams{BaseDir: "."}, subs); err != nil {
		t.Fatalf("Init: %v", err)
	}

	h.CmdName("Ranger", subs)

	if got := h.CVar.StringValue(clientNameCVar); got != "Ranger" {
		t.Fatalf("%s = %q, want Ranger", clientNameCVar, got)
	}
	if got := client.commands; !reflect.DeepEqual(got, []string{"name Ranger"}) {
		t.Fatalf("forwarded commands = %v, want [name Ranger]", got)
	}
}

func TestRegisterCommandsAddsCmdForwarder(t *testing.T) {
	h := NewHost()
	client := &forwardingTrackingClient{state: caActive}
	subs := &Subsystems{
		Client:  client,
		Console: &mockConsole{},
	}

	h.RegisterCommands(subs)
	if !h.Cmd.Exists("cmd") {
		t.Fatal("cmd command was not registered")
	}
	h.Cmd.ExecuteText("cmd status")

	if got := client.commands; !reflect.DeepEqual(got, []string{"status"}) {
		t.Fatalf("forwarded commands = %v, want [status]", got)
	}
}

func TestRegisterCommandsAddsRconForwarder(t *testing.T) {
	h := NewHost()
	client := &forwardingTrackingClient{state: caActive}
	subs := &Subsystems{
		Client:  client,
		Console: &mockConsole{},
	}

	h.RegisterCommands(subs)
	if !h.Cmd.Exists("rcon") {
		t.Fatal("rcon command was not registered")
	}
	h.Cmd.ExecuteText("rcon status")

	if got := client.commands; !reflect.DeepEqual(got, []string{"rcon status"}) {
		t.Fatalf("forwarded commands = %v, want [rcon status]", got)
	}
}

func TestCmdRconPrintsUsageWithoutArgs(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console}

	h.CmdRcon(nil, subs)

	if got := strings.Join(console.messages, ""); got != "usage: rcon <command>\n" {
		t.Fatalf("console output = %q", got)
	}
}

func TestCmdRconPrintsNotConnected(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console}

	h.CmdRcon([]string{"status"}, subs)

	if got := strings.Join(console.messages, ""); got != "Can't \"rcon\", not connected\n" {
		t.Fatalf("console output = %q", got)
	}
}

func TestCmdForwardToServerStillForwardsWhenLocalServerActive(t *testing.T) {
	h := NewHost()
	h.serverActive = true
	client := &forwardingTrackingClient{state: caActive}
	subs := &Subsystems{
		Client:  client,
		Console: &mockConsole{},
	}

	h.CmdForwardToServer([]string{"status"}, subs)

	if got := client.commands; !reflect.DeepEqual(got, []string{"status"}) {
		t.Fatalf("forwarded commands = %v, want [status]", got)
	}
}

func TestCmdForwardToServerPrintsNotConnected(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console}

	h.CmdForwardToServer([]string{"status"}, subs)

	if got := strings.Join(console.messages, ""); got != "Can't \"cmd\", not connected\n" {
		t.Fatalf("console output = %q", got)
	}
}

func TestCmdForwardToServerNoopsDuringDemoPlayback(t *testing.T) {
	h := NewHost()
	h.demoState = &cl.DemoState{Playback: true}
	client := &forwardingTrackingClient{state: caActive}
	console := &mockConsole{}
	subs := &Subsystems{
		Client:  client,
		Console: console,
	}

	h.CmdForwardToServer([]string{"status"}, subs)

	if len(client.commands) != 0 {
		t.Fatalf("forwarded commands = %v, want none", client.commands)
	}
	if got := strings.Join(console.messages, ""); got != "" {
		t.Fatalf("console output = %q, want empty", got)
	}
}

func TestCmdForwardToServerSendsNewlineForBareCmd(t *testing.T) {
	h := NewHost()
	client := &forwardingTrackingClient{state: caActive}
	subs := &Subsystems{
		Client:  client,
		Console: &mockConsole{},
	}

	h.CmdForwardToServer(nil, subs)

	if got := client.commands; !reflect.DeepEqual(got, []string{"\n"}) {
		t.Fatalf("forwarded commands = %q, want [\n]", got)
	}
}

// --- randmap tests ---

type mapListingFiles struct {
	files []string
}

func (m *mapListingFiles) Init(baseDir, gameDir string) error { return nil }
func (m *mapListingFiles) Close()                             {}
func (m *mapListingFiles) LoadFile(filename string) ([]byte, error) {
	return nil, fmt.Errorf("not found")
}
func (m *mapListingFiles) LoadFirstAvailable(filenames []string) (string, []byte, error) {
	return "", nil, fmt.Errorf("not found")
}
func (m *mapListingFiles) FileExists(filename string) bool { return false }

type testingT interface {
	Helper()
	Fatalf(format string, args ...any)
}

func writeCommandTestPak(t testingT, path string, files map[string][]byte) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create(%s): %v", path, err)
	}
	defer func() { _ = file.Close() }()

	if _, err := file.Write([]byte("PACK")); err != nil {
		t.Fatalf("Write magic: %v", err)
	}
	if err := binary.Write(file, binary.LittleEndian, int32(0)); err != nil {
		t.Fatalf("Write dir ofs placeholder: %v", err)
	}
	if err := binary.Write(file, binary.LittleEndian, int32(0)); err != nil {
		t.Fatalf("Write dir len placeholder: %v", err)
	}

	type dirEntry struct {
		name string
		pos  int32
		size int32
	}
	entries := make([]dirEntry, 0, len(files))
	for name, data := range files {
		pos, err := file.Seek(0, io.SeekCurrent)
		if err != nil {
			t.Fatalf("Seek current: %v", err)
		}
		if _, err := file.Write(data); err != nil {
			t.Fatalf("Write data for %s: %v", name, err)
		}
		entries = append(entries, dirEntry{name: name, pos: int32(pos), size: int32(len(data))})
	}

	dirOfs, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		t.Fatalf("Seek dir ofs: %v", err)
	}
	for _, entry := range entries {
		var name [56]byte
		copy(name[:], []byte(entry.name))
		if _, err := file.Write(name[:]); err != nil {
			t.Fatalf("Write dir name: %v", err)
		}
		if err := binary.Write(file, binary.LittleEndian, entry.pos); err != nil {
			t.Fatalf("Write dir pos: %v", err)
		}
		if err := binary.Write(file, binary.LittleEndian, entry.size); err != nil {
			t.Fatalf("Write dir size: %v", err)
		}
	}

	dirLen := int32(len(entries) * 64)
	if _, err := file.Seek(4, io.SeekStart); err != nil {
		t.Fatalf("Seek header patch: %v", err)
	}
	if err := binary.Write(file, binary.LittleEndian, int32(dirOfs)); err != nil {
		t.Fatalf("Patch dir ofs: %v", err)
	}
	if err := binary.Write(file, binary.LittleEndian, dirLen); err != nil {
		t.Fatalf("Patch dir len: %v", err)
	}
}

func TestCmdRandmapNoServer(t *testing.T) {
	baseDir := t.TempDir()
	id1Dir := filepath.Join(baseDir, "id1")
	if err := os.MkdirAll(id1Dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(id1): %v", err)
	}
	writeCommandTestPak(t, filepath.Join(id1Dir, "pak0.pak"), map[string][]byte{
		"maps/start.bsp": []byte("start"),
	})
	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "id1"); err != nil {
		t.Fatalf("Init: %v", err)
	}

	h := NewHost()
	console := &mockConsole{}
	commands := &insertTrackingCommandBuffer{}
	subs := &Subsystems{
		Server:   &mockServer{},
		Client:   &mockClient{},
		Console:  console,
		Files:    fileSys,
		Commands: commands,
	}
	_ = h.Init(&InitParams{BaseDir: "."}, subs)
	h.CmdRandmap(subs)

	output := strings.Join(console.messages, "")
	if !strings.Contains(output, "randmap: changing to start") {
		t.Errorf("expected randmap change message, got %q", output)
	}
	if !reflect.DeepEqual(commands.added, []string{"map start\n"}) {
		t.Fatalf("queued commands = %v, want [\"map start\\n\"]", commands.added)
	}
}

func TestCmdRandmapNoFiles(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Server:  &mockServer{},
		Client:  &mockClient{},
		Console: console,
		Files:   &mapListingFiles{files: nil},
	}
	_ = h.Init(&InitParams{BaseDir: "."}, subs)
	h.SetServerActive(true)
	// Files is not *fs.FileSystem, so the type assertion fails and returns early silently
	h.CmdRandmap(subs)
}

func TestCmdPathPrintsSearchPathStack(t *testing.T) {
	baseDir := t.TempDir()
	id1Dir := filepath.Join(baseDir, "id1")
	modDir := filepath.Join(baseDir, "hipnotic")
	if err := os.MkdirAll(id1Dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(id1): %v", err)
	}
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(mod): %v", err)
	}

	writeCommandTestPak(t, filepath.Join(id1Dir, "pak0.pak"), map[string][]byte{
		"maps/start.bsp": []byte("id1"),
	})
	writeCommandTestPak(t, filepath.Join(modDir, "pak0.pak"), map[string][]byte{
		"maps/e1m1.bsp": []byte("mod0"),
	})
	writeCommandTestPak(t, filepath.Join(modDir, "pak1.pak"), map[string][]byte{
		"maps/e1m2.bsp": []byte("mod1"),
		"progs.dat":     []byte("progs"),
	})

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "hipnotic"); err != nil {
		t.Fatalf("Init: %v", err)
	}

	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console, Files: fileSys}

	h.CmdPath(subs)

	got := strings.Join(console.messages, "")
	if !strings.HasPrefix(got, "Current search path:\n") {
		t.Fatalf("path output missing header:\n%s", got)
	}

	wantInOrder := []string{
		filepath.Join(modDir, "pak1.pak") + " (2 files)\n",
		filepath.Join(modDir, "pak0.pak") + " (1 files)\n",
		modDir + "\n",
		filepath.Join(id1Dir, "pak0.pak") + " (1 files)\n",
		id1Dir + "\n",
	}
	last := -1
	for _, want := range wantInOrder {
		idx := strings.Index(got, want)
		if idx < 0 {
			t.Fatalf("path output missing %q:\n%s", want, got)
		}
		if idx <= last {
			t.Fatalf("path output out of order for %q:\n%s", want, got)
		}
		last = idx
	}
}

func TestCmdSkiesPrintsAvailableSkyboxes(t *testing.T) {
	baseDir := t.TempDir()
	envDir := filepath.Join(baseDir, "id1", "gfx", "env")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(env): %v", err)
	}
	for _, name := range []string{"stormup.tga", "stormrt.tga", "plasmaup.tga", "junkup.jpg"} {
		if err := os.WriteFile(filepath.Join(envDir, name), []byte(name), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", name, err)
		}
	}
	writeCommandTestPak(t, filepath.Join(baseDir, "id1", "pak0.pak"), map[string][]byte{
		"gfx/env/iceup.tga":   []byte("iceup"),
		"gfx/env/icert.tga":   []byte("icert"),
		"gfx/env/stormup.tga": []byte("duplicate"),
	})

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, ""); err != nil {
		t.Fatalf("Init: %v", err)
	}

	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console, Files: fileSys}

	h.CmdSkies(nil, subs)

	got := strings.Join(console.messages, "")
	for _, want := range []string{"   ice\n", "   plasma\n", "   storm\n", "3 skies\n"} {
		if !strings.Contains(got, want) {
			t.Fatalf("skies output missing %q:\n%s", want, got)
		}
	}
}

func TestCmdSkiesFilter(t *testing.T) {
	baseDir := t.TempDir()
	envDir := filepath.Join(baseDir, "id1", "gfx", "env")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(env): %v", err)
	}
	for _, name := range []string{"stormup.tga", "plasmaup.tga"} {
		if err := os.WriteFile(filepath.Join(envDir, name), []byte(name), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", name, err)
		}
	}

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, ""); err != nil {
		t.Fatalf("Init: %v", err)
	}

	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{Console: console, Files: fileSys}

	h.CmdSkies([]string{"sto"}, subs)
	filtered := strings.Join(console.messages, "")
	if !strings.Contains(filtered, "   storm\n") || strings.Contains(filtered, "plasma") {
		t.Fatalf("filtered skies output mismatch:\n%s", filtered)
	}
	if !strings.Contains(filtered, "1 sky containing \"sto\"\n") {
		t.Fatalf("filtered skies summary mismatch:\n%s", filtered)
	}

	console.messages = nil
	h.CmdSkies([]string{"zzz"}, subs)
	if got := strings.Join(console.messages, ""); got != "no skies found containing \"zzz\"\n" {
		t.Fatalf("missing-filter output = %q", got)
	}
}

// --- viewframe/viewnext/viewprev tests ---

func TestCmdViewframeNoServer(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Server:  &mockServer{},
		Client:  &mockClient{},
		Console: console,
	}
	_ = h.Init(&InitParams{BaseDir: "."}, subs)
	h.CmdViewframe(5, subs)
	output := strings.Join(console.messages, "")
	if !strings.Contains(output, "no server running") {
		t.Errorf("expected 'no server running', got %q", output)
	}
}

func TestCmdViewframeNoViewthing(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Server:  &mockServer{},
		Client:  &mockClient{},
		Console: console,
	}
	_ = h.Init(&InitParams{BaseDir: "."}, subs)
	h.SetServerActive(true)
	h.CmdViewframe(5, subs)
	output := strings.Join(console.messages, "")
	if !strings.Contains(output, "no viewthing") {
		t.Errorf("expected 'no viewthing', got %q", output)
	}
}

func TestCmdViewnextNoViewthing(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Server:  &mockServer{},
		Client:  &mockClient{},
		Console: console,
	}
	_ = h.Init(&InitParams{BaseDir: "."}, subs)
	h.SetServerActive(true)
	h.CmdViewnext(subs)
	output := strings.Join(console.messages, "")
	if !strings.Contains(output, "no viewthing") {
		t.Errorf("expected 'no viewthing', got %q", output)
	}
}

func TestCmdViewprevNoViewthing(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Server:  &mockServer{},
		Client:  &mockClient{},
		Console: console,
	}
	_ = h.Init(&InitParams{BaseDir: "."}, subs)
	h.SetServerActive(true)
	h.CmdViewprev(subs)
	output := strings.Join(console.messages, "")
	if !strings.Contains(output, "no viewthing") {
		t.Errorf("expected 'no viewthing', got %q", output)
	}
}

func TestCmdViewframeNegativeClampsToZero(t *testing.T) {
	h := NewHost()
	console := &mockConsole{}
	subs := &Subsystems{
		Server:  &mockServer{},
		Client:  &mockClient{},
		Console: console,
	}
	_ = h.Init(&InitParams{BaseDir: "."}, subs)
	h.SetServerActive(true)
	// With mockServer, findViewthing returns nil (type assertion fails).
	// This tests the "no viewthing" path with negative frame.
	h.CmdViewframe(-5, subs)
	output := strings.Join(console.messages, "")
	if !strings.Contains(output, "no viewthing") {
		t.Errorf("expected 'no viewthing', got %q", output)
	}
}
