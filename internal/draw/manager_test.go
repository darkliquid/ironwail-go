package draw

import (
	qimage "github.com/darkliquid/ironwail-go/internal/image"
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
	pic := m.Pic("test.lmp")
	if pic != nil {
		t.Error("Pic should return nil when manager is not initialized")
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

func TestManagerCustomConcharsDetection(t *testing.T) {
	if detectCustomConchars(nil) {
		t.Fatal("missing WAD detected as custom conchars")
	}

	wad := &qimage.Wad{Lumps: map[string]qimage.Lump{
		"conchars": {Name: "conchars", Type: qimage.TypMipTex, Data: make([]byte, 128*128-1)},
	}}
	if detectCustomConchars(wad) {
		t.Fatal("truncated conchars detected as custom")
	}

	custom := make([]byte, 128*128)
	custom[0] = 1
	wad.Lumps["conchars"] = qimage.Lump{Name: "conchars", Type: qimage.TypMipTex, Data: custom}
	if !detectCustomConchars(wad) {
		t.Fatal("modified conchars not detected as custom")
	}
}

func TestFNV1A32MatchesCReference(t *testing.T) {
	// C Ironwail's COM_HashBlock uses 32-bit FNV-1a.
	if got, want := fnv1a32([]byte("conchars")), uint32(0x7df3fdb0); got != want {
		t.Fatalf("fnv1a32 = %#x, want %#x", got, want)
	}
}
