package server

import (
	"path/filepath"
	"testing"

	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/model"
	inet "github.com/ironwail/ironwail-go/internal/net"
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
