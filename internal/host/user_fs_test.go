// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cmdsys"
)

type mockUserFS struct {
	files map[string][]byte
}

func newMockUserFS() *mockUserFS {
	return &mockUserFS{files: make(map[string][]byte)}
}

func (m *mockUserFS) ReadFile(filename string) ([]byte, error) {
	if data, ok := m.files[filename]; ok {
		cp := make([]byte, len(data))
		copy(cp, data)
		return cp, nil
	}
	return nil, &os.PathError{Op: "open", Path: filename, Err: os.ErrNotExist}
}

func (m *mockUserFS) WriteFile(filename string, data []byte, perm os.FileMode) error {
	cp := make([]byte, len(data))
	copy(cp, data)
	m.files[filename] = cp
	return nil
}

func (m *mockUserFS) Stat(filename string) (os.FileInfo, error) {
	if _, ok := m.files[filename]; ok {
		return nil, nil
	}
	return nil, &os.PathError{Op: "stat", Path: filename, Err: os.ErrNotExist}
}

func (m *mockUserFS) MkdirAll(path string, perm os.FileMode) error {
	return nil
}

func (m *mockUserFS) Remove(filename string) error {
	delete(m.files, filename)
	return nil
}

func TestHostUserFSPersistence(t *testing.T) {
	h := NewHost()
	mfs := newMockUserFS()
	h.userFS = mfs
	h.userDir = "/user"
	h.initialized = true

	configPath := filepath.Join("/user", configFileName)
	err := h.writeUserFile(configPath, []byte("crosshair 1\n"), 0644)
	if err != nil {
		t.Fatalf("writeUserFile failed: %v", err)
	}

	if !h.userFileExists(configPath) {
		t.Fatalf("expected userFileExists(%q) to be true", configPath)
	}

	data, err := h.readUserFile(configPath)
	if err != nil {
		t.Fatalf("readUserFile failed: %v", err)
	}

	if !bytes.Equal(data, []byte("crosshair 1\n")) {
		t.Fatalf("readUserFile data mismatch: got %q, want %q", string(data), "crosshair 1\n")
	}
}

func TestHostExecUserConfigWithUserFS(t *testing.T) {
	h := NewHost()
	mfs := newMockUserFS()
	h.userFS = mfs
	h.userDir = "/user"
	h.gameDir = "id1"
	h.initialized = true

	// Write config into /user/id1/ironwail.cfg
	configPath := filepath.Join("/user", "id1", configFileName)
	_ = h.writeUserFile(configPath, []byte("skill 2\n"), 0644)

	var executed []string
	subs := &Subsystems{
		Commands: &mockCmdSys{
			insertText: func(s string) {
				executed = append(executed, s)
			},
		},
	}

	err := h.execUserConfig(subs)
	if err != nil {
		t.Fatalf("execUserConfig failed: %v", err)
	}

	if len(executed) != 1 || executed[0] != "skill 2\n" {
		t.Fatalf("expected executed [\"skill 2\\n\"], got %#v", executed)
	}
}

type mockCmdSys struct {
	insertText func(s string)
}

func (m *mockCmdSys) Init()                                         {}
func (m *mockCmdSys) Execute()                                          {}
func (m *mockCmdSys) ExecuteWithSource(source cmdsys.CommandSource)     {}
func (m *mockCmdSys) AddText(text string)                               {}
func (m *mockCmdSys) InsertText(text string) {
	if m.insertText != nil {
		m.insertText(text)
	}
}
func (m *mockCmdSys) Shutdown() {}
