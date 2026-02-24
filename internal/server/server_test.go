package server

import (
	"path/filepath"
	"testing"

	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/model"
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
}
