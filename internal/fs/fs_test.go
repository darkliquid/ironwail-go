package fs_test

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/testutil"
)

// TestFilesystemLoadsPak verifies that the filesystem can correctly initialize
// using a real Quake directory and load essential files like progs.dat and maps.
// Why: To ensure the core VFS can interface with actual Quake data and that the
// PAK loading logic is compatible with official assets.
// Where in C: common.c, COM_InitFilesystem and COM_LoadFile.
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

// TestFilesystemSearchPathMatchesQuakePrecedence verifies that the search path
// correctly prioritizes files according to Quake's precedence rules:
// (Mod Loose Files > Mod PAKs > Base Loose Files > Base PAKs).
// Why: Correct override behavior is essential for mods to function without
// modifying original game data.
// Where in C: common.c, COM_AddGameDirectory and the ordering of the search path list.
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

// TestPathTraversal ensures that the filesystem prevents access to files
// outside of the authorized search paths.
// Why: Security is paramount to prevent malicious maps or mods from reading
// sensitive system files.
// Where in C: Modern ports like Ironwail implement this in common.c via
// path normalization and validation logic (e.g., COM_CheckSecurity).
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

// TestPakOverrideOrderIsNumeric verifies that PAK files within a single directory
// are loaded in numeric order, with higher numbers taking precedence.
// Why: This allows "patch" PAKs (like pak1.pak) to override original assets
// in pak0.pak.
// Where in C: common.c, COM_AddGameDirectory, which scans for pak%d.pak files.
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

// TestPackLookupIsCaseInsensitive verifies that file lookups within PAK files
// are case-insensitive.
// Why: Quake's original development environment (DOS/Windows) was case-insensitive,
// and the engine preserves this behavior for cross-platform compatibility.
// Where in C: common.c, COM_FindFile, which uses case-insensitive string comparisons.
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

// TestLoadFirstAvailablePrefersSearchPathOverExtensionOrder verifies that when
// searching for one of multiple possible files, the search path precedence
// (mod vs base) is checked before file extension priority.
// Why: This ensures that a mod can provide an OGG replacement for a base game
// WAV file and have it correctly selected.
// Where in C: Ironwail-specific logic in common.c, though related to the
// general COM_OpenFile loop.
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

func TestLoadMapBSPAndLitIgnoresLowerPriorityLit(t *testing.T) {
	baseDir := t.TempDir()
	id1Dir := filepath.Join(baseDir, "id1", "maps")
	modDir := filepath.Join(baseDir, "hipnotic", "maps")
	if err := os.MkdirAll(id1Dir, 0o755); err != nil {
		t.Fatalf("failed to create id1 maps dir: %v", err)
	}
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatalf("failed to create mod maps dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(id1Dir, "start.lit"), []byte("base-lit"), 0o644); err != nil {
		t.Fatalf("failed to write id1 lit: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "start.bsp"), []byte("mod-bsp"), 0o644); err != nil {
		t.Fatalf("failed to write mod bsp: %v", err)
	}

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "hipnotic"); err != nil {
		t.Fatalf("failed to init filesystem: %v", err)
	}
	defer fileSys.Close()

	bspData, litData, err := fileSys.LoadMapBSPAndLit("maps/start.bsp")
	if err != nil {
		t.Fatalf("LoadMapBSPAndLit failed: %v", err)
	}
	if got := string(bspData); got != "mod-bsp" {
		t.Fatalf("bsp data = %q, want %q", got, "mod-bsp")
	}
	if litData != nil {
		t.Fatalf("expected lower-priority .lit to be ignored, got %q", litData)
	}
}

func TestLoadMapBSPAndLitAcceptsHigherPriorityLit(t *testing.T) {
	baseDir := t.TempDir()
	id1Dir := filepath.Join(baseDir, "id1", "maps")
	modDir := filepath.Join(baseDir, "hipnotic", "maps")
	if err := os.MkdirAll(id1Dir, 0o755); err != nil {
		t.Fatalf("failed to create id1 maps dir: %v", err)
	}
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatalf("failed to create mod maps dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(id1Dir, "start.bsp"), []byte("id1-bsp"), 0o644); err != nil {
		t.Fatalf("failed to write id1 bsp: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "start.lit"), []byte("mod-lit"), 0o644); err != nil {
		t.Fatalf("failed to write mod lit: %v", err)
	}

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "hipnotic"); err != nil {
		t.Fatalf("failed to init filesystem: %v", err)
	}
	defer fileSys.Close()

	bspData, litData, err := fileSys.LoadMapBSPAndLit("maps/start.bsp")
	if err != nil {
		t.Fatalf("LoadMapBSPAndLit failed: %v", err)
	}
	if got := string(bspData); got != "id1-bsp" {
		t.Fatalf("bsp data = %q, want %q", got, "id1-bsp")
	}
	if got := string(litData); got != "mod-lit" {
		t.Fatalf("lit data = %q, want %q", got, "mod-lit")
	}
}

// TestEnginePakLoadedWhenPresent verifies that ironwail.pak is automatically
// loaded from the application root if it exists.
// Why: Ironwail uses a dedicated PAK for engine-level assets (icons, shaders,
// default configs) that should be available regardless of the active mod.
// Where in C: Ironwail's common.c, COM_InitFilesystem.
func TestEnginePakLoadedWhenPresent(t *testing.T) {
	baseDir := t.TempDir()
	id1Dir := filepath.Join(baseDir, "id1")
	if err := os.MkdirAll(id1Dir, 0o755); err != nil {
		t.Fatalf("failed to create id1 dir: %v", err)
	}

	writeTestPak(t, filepath.Join(id1Dir, "pak0.pak"), map[string][]byte{
		"progs.dat": []byte("id1-progs"),
	})
	writeTestPak(t, filepath.Join(baseDir, "ironwail.pak"), map[string][]byte{
		"test.cfg": []byte("test"),
	})

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "id1"); err != nil {
		t.Fatalf("failed to init filesystem: %v", err)
	}
	defer fileSys.Close()

	engineData, err := fileSys.LoadFile("test.cfg")
	if err != nil {
		t.Fatalf("failed to load test.cfg from engine pak: %v", err)
	}
	if got := string(engineData); got != "test" {
		t.Fatalf("test.cfg = %q, want %q", got, "test")
	}

	progs, err := fileSys.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("failed to load id1 pak file: %v", err)
	}
	if got := string(progs); got != "id1-progs" {
		t.Fatalf("progs.dat = %q, want %q", got, "id1-progs")
	}
}

// TestEnginePakOptionalWhenMissing verifies that the engine still initializes
// correctly even if ironwail.pak is missing.
// Why: While ironwail.pak is recommended, the engine should remain functional
// if only base game data is present.
// Where in C: Ironwail's common.c, COM_InitFilesystem.
func TestEnginePakOptionalWhenMissing(t *testing.T) {
	baseDir := t.TempDir()
	id1Dir := filepath.Join(baseDir, "id1")
	if err := os.MkdirAll(id1Dir, 0o755); err != nil {
		t.Fatalf("failed to create id1 dir: %v", err)
	}

	writeTestPak(t, filepath.Join(id1Dir, "pak0.pak"), map[string][]byte{
		"progs.dat": []byte("id1-progs"),
	})

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "id1"); err != nil {
		t.Fatalf("Init should succeed without ironwail.pak: %v", err)
	}
	defer fileSys.Close()

	progs, err := fileSys.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("failed to load id1 pak file: %v", err)
	}
	if got := string(progs); got != "id1-progs" {
		t.Fatalf("progs.dat = %q, want %q", got, "id1-progs")
	}
}

func TestOpenFileFromPakReturnsReadSeekHandle(t *testing.T) {
	baseDir := t.TempDir()
	gameDir := filepath.Join(baseDir, "id1")
	if err := os.MkdirAll(gameDir, 0o755); err != nil {
		t.Fatalf("failed to create game dir: %v", err)
	}
	writeTestPak(t, filepath.Join(gameDir, "pak0.pak"), map[string][]byte{
		"progs.dat": []byte("pak-bytes"),
	})

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "id1"); err != nil {
		t.Fatalf("failed to init filesystem: %v", err)
	}
	defer fileSys.Close()

	handle, size, err := fileSys.OpenFile("progs.dat")
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}
	defer handle.Close()

	if size != int64(len("pak-bytes")) {
		t.Fatalf("size = %d, want %d", size, len("pak-bytes"))
	}

	buf := make([]byte, 3)
	if _, err := io.ReadFull(handle, buf); err != nil {
		t.Fatalf("failed initial read: %v", err)
	}
	if got := string(buf); got != "pak" {
		t.Fatalf("first bytes = %q, want %q", got, "pak")
	}

	if _, err := handle.Seek(4, io.SeekStart); err != nil {
		t.Fatalf("seek failed: %v", err)
	}
	rest, err := io.ReadAll(handle)
	if err != nil {
		t.Fatalf("read after seek failed: %v", err)
	}
	if got := string(rest); got != "bytes" {
		t.Fatalf("seek/read data = %q, want %q", got, "bytes")
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

// TestListModsReturnsValidDirs verifies that ListMods discovers directories
// containing pak files or progs.dat while ignoring id1 and empty directories.
func TestListModsReturnsValidDirs(t *testing.T) {
	baseDir := t.TempDir()

	// id1 – should always be excluded.
	id1 := filepath.Join(baseDir, "id1")
	if err := os.MkdirAll(id1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(id1, "pak0.pak"), []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	// hipnotic – valid: contains pak0.pak.
	hipDir := filepath.Join(baseDir, "hipnotic")
	if err := os.MkdirAll(hipDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hipDir, "pak0.pak"), []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	// mypatch – valid: contains progs.dat.
	patchDir := filepath.Join(baseDir, "mypatch")
	if err := os.MkdirAll(patchDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(patchDir, "progs.dat"), []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	// empty – invalid: no pak or progs.dat.
	emptyDir := filepath.Join(baseDir, "emptydocs")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatal(err)
	}

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "id1"); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	mods := fileSys.ListMods()
	modNames := make(map[string]bool)
	for _, m := range mods {
		modNames[m.Name] = true
	}

	if modNames["id1"] {
		t.Error("id1 should be excluded from mod list")
	}
	if !modNames["hipnotic"] {
		t.Error("hipnotic should be listed as a valid mod (has pak0.pak)")
	}
	if !modNames["mypatch"] {
		t.Error("mypatch should be listed as a valid mod (has progs.dat)")
	}
	if modNames["emptydocs"] {
		t.Error("emptydocs should not be listed (no pak or progs.dat)")
	}
}

// TestListModsEmptyBaseDir verifies that ListMods returns nil for an unset basedir.
func TestListModsEmptyBaseDir(t *testing.T) {
	fileSys := fs.NewFileSystem()
	mods := fileSys.ListMods()
	if mods != nil {
		t.Fatalf("expected nil from ListMods with no basedir, got %v", mods)
	}
}
