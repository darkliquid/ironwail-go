// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"strings"
	"testing"
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
