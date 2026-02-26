package draw

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}

	if m.pics == nil {
		t.Error("pics map is not initialized")
	}
}

func TestManagerGetPic_NotInitialized(t *testing.T) {
	m := NewManager()
	pic := m.GetPic("test.lmp")
	if pic != nil {
		t.Error("GetPic should return nil when manager is not initialized")
	}
}

// TestManagerInitFromDir tests initialization from a directory.
// This test requires a gfx.wad file to be present in the testdata directory.
func TestManagerInitFromDir(t *testing.T) {
	// Check if testdata directory exists
	testdataDir := filepath.Join(".", "testdata")
	if _, err := os.Stat(testdataDir); os.IsNotExist(err) {
		t.Skip("testdata directory not found, skipping test")
	}

	// Check if gfx.wad exists
	wadPath := filepath.Join(testdataDir, "gfx.wad")
	if _, err := os.Stat(wadPath); os.IsNotExist(err) {
		t.Skip("gfx.wad not found in testdata, skipping test")
	}

	m := NewManager()
	err := m.InitFromDir(testdataDir)
	if err != nil {
		t.Fatalf("InitFromDir failed: %v", err)
	}

	if !m.initialized {
		t.Error("Manager should be initialized")
	}

	if m.wad == nil {
		t.Error("WAD should be loaded")
	}

	if len(m.palette) != 768 {
		t.Errorf("Palette should be 768 bytes, got %d", len(m.palette))
	}
}

func TestManagerShutdown(t *testing.T) {
	m := NewManager()
	m.Shutdown()

	if m.initialized {
		t.Error("Manager should not be initialized after shutdown")
	}
}
