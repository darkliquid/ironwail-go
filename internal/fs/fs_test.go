package fs_test

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
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

func TestFilesystemSearchPathMatchesQuakePrecedence(t *testing.T) {
	baseDir := t.TempDir()
	id1Dir := filepath.Join(baseDir, "id1")
	modDir := filepath.Join(baseDir, "hipnotic")
	if err := os.MkdirAll(id1Dir, 0o755); err != nil {
		t.Fatalf("failed to create id1 dir: %v", err)
	}
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatalf("failed to create mod dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(id1Dir, "progs.dat"), []byte("id1-loose"), 0o644); err != nil {
		t.Fatalf("failed to write id1 loose file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "maps.txt"), []byte("mod-loose"), 0o644); err != nil {
		t.Fatalf("failed to write mod loose file: %v", err)
	}

	writeTestPak(t, filepath.Join(id1Dir, "pak0.pak"), map[string][]byte{
		"progs.dat": []byte("id1-pak0"),
		"maps.txt":  []byte("id1-pak0-maps"),
	})
	writeTestPak(t, filepath.Join(id1Dir, "pak1.pak"), map[string][]byte{
		"progs.dat": []byte("id1-pak1"),
	})
	writeTestPak(t, filepath.Join(modDir, "pak0.pak"), map[string][]byte{
		"progs.dat": []byte("mod-pak0"),
	})
	writeTestPak(t, filepath.Join(modDir, "pak1.pak"), map[string][]byte{
		"progs.dat": []byte("mod-pak1"),
		"maps.txt":  []byte("mod-pak1-maps"),
	})

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "hipnotic"); err != nil {
		t.Fatalf("failed to init filesystem: %v", err)
	}
	defer fileSys.Close()

	progs, err := fileSys.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("failed to read progs.dat: %v", err)
	}
	if got := string(progs); got != "mod-pak1" {
		t.Fatalf("progs.dat source = %q, want %q", got, "mod-pak1")
	}

	maps, err := fileSys.LoadFile("maps.txt")
	if err != nil {
		t.Fatalf("failed to read maps.txt: %v", err)
	}
	if got := string(maps); got != "mod-pak1-maps" {
		t.Fatalf("maps.txt source = %q, want %q", got, "mod-pak1-maps")
	}
}

func TestPathTraversal(t *testing.T) {
	baseDir := t.TempDir()
	gameDir := filepath.Join(baseDir, "id1")

	if err := os.MkdirAll(gameDir, 0o755); err != nil {
		t.Fatalf("failed to create game dir: %v", err)
	}

	insidePath := filepath.Join(gameDir, "config.cfg")
	if err := os.WriteFile(insidePath, []byte("safe"), 0o644); err != nil {
		t.Fatalf("failed to create inside test file: %v", err)
	}

	outsidePath := filepath.Join(baseDir, "secret.txt")
	if err := os.WriteFile(outsidePath, []byte("nope"), 0o644); err != nil {
		t.Fatalf("failed to create outside test file: %v", err)
	}

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "id1"); err != nil {
		t.Fatalf("failed to init filesystem: %v", err)
	}
	defer fileSys.Close()

	if _, err := fileSys.LoadFile("config.cfg"); err != nil {
		t.Fatalf("expected normal file read to succeed: %v", err)
	}

	for _, badPath := range []string{
		"../secret.txt",
		"maps/../../secret.txt",
		"id1/../../etc/passwd",
		"..\\secret.txt",
	} {
		_, err := fileSys.LoadFile(badPath)
		if err == nil {
			t.Fatalf("expected traversal path %q to fail", badPath)
		}

		if !strings.Contains(strings.ToLower(err.Error()), "traversal") && !strings.Contains(strings.ToLower(err.Error()), "invalid") {
			t.Fatalf("expected clean traversal error for %q, got: %v", badPath, err)
		}
	}
}

func TestPakOverrideOrderIsNumeric(t *testing.T) {
	baseDir := t.TempDir()
	gameDir := filepath.Join(baseDir, "id1")
	if err := os.MkdirAll(gameDir, 0o755); err != nil {
		t.Fatalf("failed to create game dir: %v", err)
	}

	writeTestPak(t, filepath.Join(gameDir, "pak0.pak"), map[string][]byte{
		"progs.dat": []byte("pak0"),
	})
	writeTestPak(t, filepath.Join(gameDir, "pak10.pak"), map[string][]byte{
		"progs.dat": []byte("pak10"),
	})
	writeTestPak(t, filepath.Join(gameDir, "pak2.pak"), map[string][]byte{
		"progs.dat": []byte("pak2"),
	})

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "id1"); err != nil {
		t.Fatalf("failed to init filesystem: %v", err)
	}
	defer fileSys.Close()

	progs, err := fileSys.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("failed to read progs.dat: %v", err)
	}
	if got := string(progs); got != "pak10" {
		t.Fatalf("override data = %q, want %q", got, "pak10")
	}
}

func TestPackLookupIsCaseInsensitive(t *testing.T) {
	baseDir := t.TempDir()
	gameDir := filepath.Join(baseDir, "id1")
	if err := os.MkdirAll(gameDir, 0o755); err != nil {
		t.Fatalf("failed to create game dir: %v", err)
	}

	writeTestPak(t, filepath.Join(gameDir, "PAK0.PAK"), map[string][]byte{
		"Maps/Start.BSP": []byte("mixed-case map"),
	})

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "id1"); err != nil {
		t.Fatalf("failed to init filesystem: %v", err)
	}
	defer fileSys.Close()

	data, err := fileSys.LoadFile("maps/start.bsp")
	if err != nil {
		t.Fatalf("failed to read mixed-case pack entry: %v", err)
	}
	if got := string(data); got != "mixed-case map" {
		t.Fatalf("map data = %q, want %q", got, "mixed-case map")
	}
}

func TestLoadFirstAvailablePrefersSearchPathOverExtensionOrder(t *testing.T) {
	baseDir := t.TempDir()
	id1Dir := filepath.Join(baseDir, "id1")
	modDir := filepath.Join(baseDir, "hipnotic")
	if err := os.MkdirAll(filepath.Join(id1Dir, "music"), 0o755); err != nil {
		t.Fatalf("failed to create id1 music dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(modDir, "music"), 0o755); err != nil {
		t.Fatalf("failed to create mod music dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(id1Dir, "music", "track02.wav"), []byte("id1-wav"), 0o644); err != nil {
		t.Fatalf("failed to write id1 wav: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "music", "track02.ogg"), []byte("mod-ogg"), 0o644); err != nil {
		t.Fatalf("failed to write mod ogg: %v", err)
	}

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "hipnotic"); err != nil {
		t.Fatalf("failed to init filesystem: %v", err)
	}
	defer fileSys.Close()

	name, data, err := fileSys.LoadFirstAvailable([]string{"music/track02.wav", "music/track02.ogg"})
	if err != nil {
		t.Fatalf("LoadFirstAvailable failed: %v", err)
	}
	if name != "music/track02.ogg" {
		t.Fatalf("resolved filename = %q, want %q", name, "music/track02.ogg")
	}
	if got := string(data); got != "mod-ogg" {
		t.Fatalf("resolved data = %q, want %q", got, "mod-ogg")
	}
}

func writeTestPak(t *testing.T, path string, files map[string][]byte) {
	t.Helper()

	var data bytes.Buffer
	type dirEntry struct {
		name string
		pos  int32
		len  int32
	}
	entries := make([]dirEntry, 0, len(files))

	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	for _, name := range names {
		content := files[name]
		entries = append(entries, dirEntry{
			name: name,
			pos:  int32(12 + data.Len()),
			len:  int32(len(content)),
		})
		if _, err := data.Write(content); err != nil {
			t.Fatalf("failed to write pak payload: %v", err)
		}
	}

	var dir bytes.Buffer
	for _, entry := range entries {
		var name [56]byte
		copy(name[:], []byte(entry.name))
		if err := binary.Write(&dir, binary.LittleEndian, name); err != nil {
			t.Fatalf("failed to write pak entry name: %v", err)
		}
		if err := binary.Write(&dir, binary.LittleEndian, entry.pos); err != nil {
			t.Fatalf("failed to write pak entry position: %v", err)
		}
		if err := binary.Write(&dir, binary.LittleEndian, entry.len); err != nil {
			t.Fatalf("failed to write pak entry length: %v", err)
		}
	}

	var pak bytes.Buffer
	if _, err := pak.WriteString("PACK"); err != nil {
		t.Fatalf("failed to write pak header id: %v", err)
	}
	dirOfs := int32(12 + data.Len())
	dirLen := int32(dir.Len())
	if err := binary.Write(&pak, binary.LittleEndian, dirOfs); err != nil {
		t.Fatalf("failed to write pak dir offset: %v", err)
	}
	if err := binary.Write(&pak, binary.LittleEndian, dirLen); err != nil {
		t.Fatalf("failed to write pak dir length: %v", err)
	}
	if _, err := pak.Write(data.Bytes()); err != nil {
		t.Fatalf("failed to append pak payload: %v", err)
	}
	if _, err := pak.Write(dir.Bytes()); err != nil {
		t.Fatalf("failed to append pak directory: %v", err)
	}

	if err := os.WriteFile(path, pak.Bytes(), 0o644); err != nil {
		t.Fatalf("failed to write pak file: %v", err)
	}
}
