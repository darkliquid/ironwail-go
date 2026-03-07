// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	cl "github.com/ironwail/ironwail-go/internal/client"
	inet "github.com/ironwail/ironwail-go/internal/net"
)

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

func TestCmdKill(t *testing.T) {
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

	h.CmdKill(&subs.Subsystems)
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
	subs := &mockSubsystems{
		server:  &mockServer{},
		client:  &mockClient{},
		console: &mockConsole{},
	}
	subs.Subsystems.Server = subs.server
	subs.Subsystems.Client = subs.client
	subs.Subsystems.Console = subs.console

	h.Init(&InitParams{BaseDir: "."}, &subs.Subsystems)

	h.CmdName("Player", &subs.Subsystems)
}

func TestCmdColor(t *testing.T) {
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

	h.CmdColor("13", &subs.Subsystems)
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

func TestCmdSaveRejectsInvalidName(t *testing.T) {
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

	h.CmdSave("../bad", &subs.Subsystems)

	if len(subs.console.messages) == 0 {
		t.Fatal("expected console output")
	}
	if got := strings.Join(subs.console.messages, ""); !strings.Contains(got, "invalid save name") {
		t.Fatalf("console output = %q, want invalid save name", got)
	}
}

func TestCmdLoadRejectsInvalidName(t *testing.T) {
	h := NewHost()
	subs := &mockSubsystems{
		server:  &mockServer{},
		client:  &mockClient{},
		console: &mockConsole{},
	}
	subs.Subsystems.Server = subs.server
	subs.Subsystems.Client = subs.client
	subs.Subsystems.Console = subs.console

	if err := h.Init(&InitParams{BaseDir: ".", UserDir: t.TempDir()}, &subs.Subsystems); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	h.CmdLoad("../bad", &subs.Subsystems)

	if len(subs.console.messages) == 0 {
		t.Fatal("expected console output")
	}
	if got := strings.Join(subs.console.messages, ""); !strings.Contains(got, "invalid save name") {
		t.Fatalf("console output = %q, want invalid save name", got)
	}
}

func TestCmdRecordUsesLoopbackClientCDTrack(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(%q) failed: %v", tmpDir, err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	h := NewHost()
	console := &mockConsole{}
	lc := newLocalLoopbackClient()
	lc.inner.CDTrack = 7
	subs := &Subsystems{
		Client:  lc,
		Console: console,
	}

	h.CmdRecord("music_header", subs)
	if h.demoState == nil {
		t.Fatal("expected demo state to be created")
	}
	if got := h.demoState.CDTrack; got != 7 {
		t.Fatalf("demo CDTrack = %d, want 7", got)
	}
	if err := h.demoState.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "demos", "music_header.dem"))
	if err != nil {
		t.Fatalf("ReadFile(demo): %v", err)
	}
	if !strings.HasPrefix(string(data), "7\n") {
		t.Fatalf("demo header = %q, want prefix %q", string(data), "7\\n")
	}
}

func TestCmdStopWritesDisconnectTrailer(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(%q) failed: %v", tmpDir, err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	h := NewHost()
	console := &mockConsole{}
	lc := newLocalLoopbackClient()
	lc.inner.ViewAngles = [3]float32{11, 22, 33}
	subs := &Subsystems{
		Client:  lc,
		Console: console,
	}

	h.demoState = cl.NewDemoState()
	if err := h.demoState.StartDemoRecording("stop_trailer", 0); err != nil {
		t.Fatalf("StartDemoRecording failed: %v", err)
	}

	h.CmdStop(subs)
	if h.demoState.Recording {
		t.Fatal("demo recording still active after stop")
	}
	if got := strings.Join(console.messages, ""); !strings.Contains(got, "Completed demo") {
		t.Fatalf("console output = %q, want completion message", got)
	}

	replay := cl.NewDemoState()
	if err := replay.StartDemoPlayback(filepath.Join(tmpDir, "demos", "stop_trailer.dem")); err != nil {
		t.Fatalf("StartDemoPlayback failed: %v", err)
	}
	defer replay.StopPlayback()

	message, angles, err := replay.ReadDemoFrame()
	if err != nil {
		t.Fatalf("ReadDemoFrame failed: %v", err)
	}
	if len(message) != 1 || message[0] != inet.SVCDisconnect {
		t.Fatalf("disconnect message = %v, want [%d]", message, inet.SVCDisconnect)
	}
	if angles != lc.inner.ViewAngles {
		t.Fatalf("disconnect angles = %v, want %v", angles, lc.inner.ViewAngles)
	}
}

func TestCmdRecordWritesInitialStateSnapshotWhenConnected(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(%q) failed: %v", tmpDir, err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	h := NewHost()
	console := &mockConsole{}
	lc := newLocalLoopbackClient()
	lc.inner.State = cl.StateActive
	lc.inner.Signon = 4
	lc.inner.Protocol = inet.PROTOCOL_FITZQUAKE
	lc.inner.MaxClients = 1
	lc.inner.LevelName = "Snapshot Command"
	lc.inner.ModelPrecache = []string{"maps/start.bsp"}
	lc.inner.SoundPrecache = []string{"misc/null.wav"}
	lc.inner.ViewEntity = 1
	lc.inner.CDTrack = 3
	lc.inner.LoopTrack = 3
	subs := &Subsystems{
		Client:  lc,
		Console: console,
	}

	h.CmdRecord("record_snapshot", subs)
	if h.demoState == nil {
		t.Fatal("expected demo state")
	}
	if err := h.demoState.StopRecording(); err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}

	replay := cl.NewDemoState()
	if err := replay.StartDemoPlayback(filepath.Join(tmpDir, "demos", "record_snapshot.dem")); err != nil {
		t.Fatalf("StartDemoPlayback failed: %v", err)
	}
	defer replay.StopPlayback()

	message, _, err := replay.ReadDemoFrame()
	if err != nil {
		t.Fatalf("ReadDemoFrame failed: %v", err)
	}
	if len(message) == 0 || message[0] != byte(inet.SVCServerInfo) {
		t.Fatalf("first frame = %v, want serverinfo snapshot", message)
	}
}

func TestCmdPlaydemoLeavesLoopbackClientDisconnectedForServerInfo(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(%q) failed: %v", tmpDir, err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	if err := os.MkdirAll(filepath.Join(tmpDir, "demos"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "demos", "bootstrap.dem"), []byte("0\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	h := NewHost()
	console := &mockConsole{}
	lc := newLocalLoopbackClient()
	lc.inner.State = cl.StateActive
	lc.inner.Signon = 4
	subs := &Subsystems{
		Client:  lc,
		Console: console,
	}

	h.CmdPlaydemo("bootstrap", subs)
	if lc.inner.State != cl.StateDisconnected {
		t.Fatalf("loopback client state = %v, want disconnected", lc.inner.State)
	}
}
