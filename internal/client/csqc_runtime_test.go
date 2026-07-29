package client

import (
	"testing"
)

type mockCSQCHandler struct {
	initCalled       bool
	shutdownCalled   bool
	parseCmdCalled   bool
	lastParsedCmd    string
	entUpdateCalled  bool
	lastEntUpdateNew bool
}

func (m *mockCSQCHandler) Init() error {
	m.initCalled = true
	return nil
}

func (m *mockCSQCHandler) Shutdown() error {
	m.shutdownCalled = true
	return nil
}

func (m *mockCSQCHandler) ParseStuffCmd(cmd string) bool {
	m.parseCmdCalled = true
	m.lastParsedCmd = cmd
	return cmd == "csqc_handled\n"
}

func (m *mockCSQCHandler) EntUpdate(isNew bool) {
	m.entUpdateCalled = true
	m.lastEntUpdateNew = isNew
}

func TestClientCSQCHandlerSignonInitAndShutdown(t *testing.T) {
	c := NewClient()
	c.State = StateConnected
	mock := &mockCSQCHandler{}
	c.SetCSQCHandler(mock)

	if err := c.HandleSignonReply("prespawn"); err != nil {
		t.Fatalf("HandleSignonReply failed: %v", err)
	}

	if !mock.initCalled {
		t.Fatal("expected CSQCHandler.Init() to be called on prespawn signon reply")
	}

	c.ClearSignons()

	if !mock.shutdownCalled {
		t.Fatal("expected CSQCHandler.Shutdown() to be called on ClearSignons()")
	}
}

func TestClientCSQCHandlerFiltersStuffCommands(t *testing.T) {
	c := NewClient()
	mock := &mockCSQCHandler{}
	c.SetCSQCHandler(mock)

	c.StuffCmdBuf = "csqc_handled\n"
	cmds := c.ConsumeStuffCommands()

	if !mock.parseCmdCalled {
		t.Fatal("expected CSQCHandler.ParseStuffCmd to be invoked")
	}
	if cmds != "" {
		t.Fatalf("expected stuffcmd handled by CSQC to be filtered out, got %q", cmds)
	}

	c.StuffCmdBuf = "echo test\n"
	cmds = c.ConsumeStuffCommands()
	if cmds != "echo test\n" {
		t.Fatalf("expected unhandled stuffcmd to be returned, got %q", cmds)
	}
}
