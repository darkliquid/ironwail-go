package fs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/testutil"
)

func TestFilesystemLoadsPak(t *testing.T) {
	quakeDir := testutil.SkipIfNoQuakeDir(t)

	fileSys := fs.NewFileSystem()
	err := fileSys.Init(quakeDir, "id1")
	if err != nil {
		t.Fatalf("Failed to init filesystem: %v", err)
	}

	// Try reading progs.dat
	progs, err := fileSys.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("Failed to read progs.dat: %v", err)
	}
	if len(progs) == 0 {
		t.Errorf("progs.dat is empty")
	}

	// Try reading a map
	startMap, err := fileSys.LoadFile("maps/start.bsp")
	if err != nil {
		t.Fatalf("Failed to read maps/start.bsp: %v", err)
	}
	if len(startMap) == 0 {
		t.Errorf("maps/start.bsp is empty")
	}
}

func TestFilesystemOverrides(t *testing.T) {
	quakeDir := testutil.SkipIfNoQuakeDir(t)
    
	// Create a temporary override file in the id1 directory
	overrideFile := filepath.Join(quakeDir, "id1", "progs.dat")
	
    // Ensure we don't accidentally overwrite a real loose file if the user has one
	if _, err := os.Stat(overrideFile); err == nil {
		t.Skipf("loose progs.dat already exists in %s, skipping override test", overrideFile)
	}
	
	testData := []byte("fake progs.dat override")
	if err := os.WriteFile(overrideFile, testData, 0644); err != nil {
		t.Fatalf("Failed to write override file: %v", err)
	}
	defer os.Remove(overrideFile) // Clean up

	fileSys := fs.NewFileSystem()
	err := fileSys.Init(quakeDir, "id1")
	if err != nil {
		t.Fatalf("Failed to init filesystem: %v", err)
	}

	// Reading progs.dat should now return our override file because loose files have higher priority
	progs, err := fileSys.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("Failed to read progs.dat: %v", err)
	}
	
    expected := string(testData)
    actual := string(progs)
    
    // We expect the exact data! Why didn't it match? Let's check size
	if actual != expected {
		t.Errorf("Expected override file data '%s' (len %d), got '%s' (len %d)", expected, len(expected), actual, len(actual))
	}
}
