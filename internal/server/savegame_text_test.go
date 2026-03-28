package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/fs"
)

func TestParseTextSaveGamePreservesTitle(t *testing.T) {
	data := "6\nid1\nA descriptive save title\n"
	for i := 0; i < NumSpawnParms; i++ {
		data += "0\n"
	}
	data += "2\n"
	data += "e2m2\n"
	data += "123.5\n"
	for i := 0; i < 64; i++ {
		data += "m\n"
	}
	data += "{\n\"serverflags\" \"0\"\n}\n"
	data += "{\n\"classname\" \"worldspawn\"\n}\n"

	state, err := ParseTextSaveGame([]byte(data))
	if err != nil {
		t.Fatalf("ParseTextSaveGame failed: %v", err)
	}
	if got := state.Title; got != "A descriptive save title" {
		t.Fatalf("Title = %q, want %q", got, "A descriptive save title")
	}
	if got := state.MapName; got != "e2m2" {
		t.Fatalf("MapName = %q, want %q", got, "e2m2")
	}
}

func TestRestoreTextSaveGameStateRejectsMismatchedGameDir(t *testing.T) {
	baseDir := t.TempDir()
	for _, dir := range []string{"id1", "rogue"} {
		if err := os.Mkdir(filepath.Join(baseDir, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	vfs := fs.NewFileSystem()
	if err := vfs.Init(baseDir, "id1"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer vfs.Close()

	s := NewServer()
	s.FileSystem = vfs
	s.Static = &ServerStatic{}

	err := s.RestoreTextSaveGameState(&TextSaveGameState{
		MapName: "start",
		GameDir: "rogue",
	})
	if err == nil {
		t.Fatal("RestoreTextSaveGameState() error = nil, want gamedir mismatch")
	}
	if !strings.Contains(err.Error(), "gamedir") {
		t.Fatalf("RestoreTextSaveGameState() error = %v, want gamedir mismatch", err)
	}
}
