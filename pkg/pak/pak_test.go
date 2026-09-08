package pak_test

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/pkg/pak"
)

func TestReaderAndWriterRoundtrip(t *testing.T) {
	var buf bytes.Buffer
	w := pak.NewWriter(&buf)

	files := map[string][]byte{
		"progs.dat":      []byte("fake progs bytecode"),
		"maps/start.bsp": []byte("fake bsp data"),
		"sound/test.wav": []byte("fake audio sample"),
	}

	for name, data := range files {
		if err := w.AddFile(name, data); err != nil {
			t.Fatalf("AddFile(%q) error: %v", name, err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	pakData := buf.Bytes()
	if len(pakData) < pak.HeaderSize {
		t.Fatalf("PAK data too short: %d bytes", len(pakData))
	}

	reader, err := pak.OpenBytes("test.pak", pakData)
	if err != nil {
		t.Fatalf("OpenBytes failed: %v", err)
	}
	defer func() { _ = reader.Close() }()

	if len(reader.Entries()) != len(files) {
		t.Fatalf("got %d entries, want %d", len(reader.Entries()), len(files))
	}

	for name, wantData := range files {
		data, err := reader.ReadFile(name)
		if err != nil {
			t.Fatalf("ReadFile(%q) failed: %v", name, err)
		}
		if !bytes.Equal(data, wantData) {
			t.Errorf("ReadFile(%q) = %q, want %q", name, data, wantData)
		}

		// Case-insensitive & slash-normalised lookup
		upperName := filepath.ToSlash(filepath.Clean(name))
		altName := bytes.ToUpper([]byte(upperName))
		altData, err := reader.ReadFile(string(altName))
		if err != nil {
			t.Errorf("ReadFile(%q) case-insensitive failed: %v", altName, err)
		} else if !bytes.Equal(altData, wantData) {
			t.Errorf("ReadFile(%q) = %q, want %q", altName, altData, wantData)
		}
	}
}

func TestReaderFS(t *testing.T) {
	entries := []pak.FileEntry{
		{Name: "maps/e1m1.bsp", Data: []byte("e1m1 content")},
		{Name: "maps/e1m2.bsp", Data: []byte("e1m2 content")},
		{Name: "default.cfg", Data: []byte("bind w +forward\n")},
	}

	var buf bytes.Buffer
	if err := pak.WritePack(&buf, entries); err != nil {
		t.Fatalf("WritePack failed: %v", err)
	}

	reader, err := pak.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}
	defer func() { _ = reader.Close() }()

	// io/fs.ReadFileFS
	content, err := fs.ReadFile(reader, "default.cfg")
	if err != nil {
		t.Fatalf("fs.ReadFile failed: %v", err)
	}
	if string(content) != "bind w +forward\n" {
		t.Errorf("unexpected content: %q", string(content))
	}

	// io/fs.StatFS
	fi, err := fs.Stat(reader, "maps/e1m1.bsp")
	if err != nil {
		t.Fatalf("fs.Stat failed: %v", err)
	}
	if fi.Name() != "e1m1.bsp" {
		t.Errorf("fi.Name() = %q, want e1m1.bsp", fi.Name())
	}
	if fi.Size() != int64(len("e1m1 content")) {
		t.Errorf("fi.Size() = %d, want %d", fi.Size(), len("e1m1 content"))
	}

	// io/fs.ReadDirFS
	dirEntries, err := fs.ReadDir(reader, "maps")
	if err != nil {
		t.Fatalf("fs.ReadDir(maps) failed: %v", err)
	}
	if len(dirEntries) != 2 {
		t.Fatalf("got %d dir entries, want 2", len(dirEntries))
	}
	for _, de := range dirEntries {
		if de.IsDir() || de.Type().IsDir() {
			t.Errorf("expected %s to be a file, got isDir=true", de.Name())
		}
	}

	// Read root directory "."
	rootDirEntries, err := fs.ReadDir(reader, ".")
	if err != nil {
		t.Fatalf("fs.ReadDir(.) failed: %v", err)
	}
	// Root should have "default.cfg" (file) and "maps" (dir)
	if len(rootDirEntries) != 2 {
		t.Fatalf("got %d root entries, want 2: %v", len(rootDirEntries), rootDirEntries)
	}
	for _, de := range rootDirEntries {
		if de.Name() == "maps" {
			if !de.IsDir() || !de.Type().IsDir() {
				t.Errorf("expected maps to be a directory, got IsDir=%v Type=%v", de.IsDir(), de.Type())
			}
		} else if de.Name() == "default.cfg" {
			if de.IsDir() || de.Type().IsDir() {
				t.Errorf("expected default.cfg to be a file, got IsDir=%v", de.IsDir())
			}
		}
	}
}

func TestEmptyPack(t *testing.T) {
	var buf bytes.Buffer
	if err := pak.WritePack(&buf, nil); err != nil {
		t.Fatalf("WritePack(nil) failed: %v", err)
	}

	reader, err := pak.OpenBytes("empty.pak", buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes(empty) failed: %v", err)
	}
	defer func() { _ = reader.Close() }()

	if len(reader.Entries()) != 0 {
		t.Errorf("got %d entries for empty pack, want 0", len(reader.Entries()))
	}
}

func TestWriterValidation(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"", true},
		{"a\x00b", true},
		{"a\\b", true},
		{"/abs/path", true},
		{"dir/", true},
		{"path/./file", true},
		{"path/../file", true},
		{"path//file", true},
		{"valid/path/file.txt", false},
		{"this_is_a_file_name_that_is_far_too_long_to_fit_in_the_fifty_six_byte_pak_directory_entry_field.txt", true},
	}

	for _, tt := range tests {
		err := pak.ValidName(tt.name)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidName(%q) err = %v, wantErr = %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestVFS(t *testing.T) {
	tmpDir := t.TempDir()
	looseDir := filepath.Join(tmpDir, "loose")
	pakDir := filepath.Join(tmpDir, "paks")

	if err := os.MkdirAll(looseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(pakDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Loose file overrides pak file with same name
	if err := os.WriteFile(filepath.Join(looseDir, "config.cfg"), []byte("loose config"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(looseDir, "loose_only.txt"), []byte("loose only"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create pak file
	pakFile := filepath.Join(pakDir, "pak0.pak")
	pakEntries := []pak.FileEntry{
		{Name: "config.cfg", Data: []byte("pak config")},
		{Name: "pak_only.txt", Data: []byte("pak only")},
	}
	f, err := os.Create(pakFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := pak.WritePack(f, pakEntries); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	vfs := pak.NewFileSystem()
	defer func() { _ = vfs.Close() }()

	// Mount pak first (lower priority), then loose dir (higher priority)
	if err := vfs.MountPak(pakFile); err != nil {
		t.Fatalf("MountPak failed: %v", err)
	}
	if err := vfs.MountDirectory(looseDir); err != nil {
		t.Fatalf("MountDirectory failed: %v", err)
	}

	// config.cfg should be from loose directory ("loose config")
	cfgData, err := vfs.ReadFile("config.cfg")
	if err != nil {
		t.Fatalf("ReadFile(config.cfg) failed: %v", err)
	}
	if string(cfgData) != "loose config" {
		t.Errorf("ReadFile(config.cfg) = %q, want %q", string(cfgData), "loose config")
	}

	// pak_only.txt should be from pak file
	pakData, err := vfs.ReadFile("pak_only.txt")
	if err != nil {
		t.Fatalf("ReadFile(pak_only.txt) failed: %v", err)
	}
	if string(pakData) != "pak only" {
		t.Errorf("ReadFile(pak_only.txt) = %q, want %q", string(pakData), "pak only")
	}

	// loose_only.txt should be from loose file
	looseData, err := vfs.ReadFile("loose_only.txt")
	if err != nil {
		t.Fatalf("ReadFile(loose_only.txt) failed: %v", err)
	}
	if string(looseData) != "loose only" {
		t.Errorf("ReadFile(loose_only.txt) = %q, want %q", string(looseData), "loose only")
	}

	// FileExists
	if !vfs.FileExists("config.cfg") {
		t.Errorf("FileExists(config.cfg) = false, want true")
	}
	if vfs.FileExists("nonexistent.txt") {
		t.Errorf("FileExists(nonexistent.txt) = true, want false")
	}

	// OpenFile
	r, size, err := vfs.OpenFile("pak_only.txt")
	if err != nil {
		t.Fatalf("OpenFile(pak_only.txt) failed: %v", err)
	}
	defer func() { _ = r.Close() }()
	if size != int64(len("pak only")) {
		t.Errorf("size = %d, want %d", size, len("pak only"))
	}
	readBuf, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("io.ReadAll failed: %v", err)
	}
	if string(readBuf) != "pak only" {
		t.Errorf("readBuf = %q, want %q", string(readBuf), "pak only")
	}

	// SearchPathEntries
	paths := vfs.SearchPathEntries()
	if len(paths) != 2 {
		t.Errorf("got %d search path entries, want 2", len(paths))
	}

	// ListFiles (Quake returns matches across all search paths without deduplication)
	matches := vfs.ListFiles("*.cfg")
	if len(matches) != 2 {
		t.Errorf("ListFiles(*.cfg) = %v, want 2 entries", matches)
	}
}

func TestReaderErrors(t *testing.T) {
	// Too short
	_, err := pak.NewReader(bytes.NewReader([]byte("short")), 5)
	if err == nil {
		t.Errorf("expected error for short buffer")
	}

	// Bad magic
	badMagic := []byte("NOPE\x00\x00\x00\x00\x00\x00\x00\x00")
	_, err = pak.NewReader(bytes.NewReader(badMagic), int64(len(badMagic)))
	if err == nil {
		t.Errorf("expected error for bad magic")
	}

	// Out of bounds directory
	oob := make([]byte, 20)
	copy(oob[:4], "PACK")
	// dirOfs = 100
	oob[4] = 100
	_, err = pak.NewReader(bytes.NewReader(oob), int64(len(oob)))
	if err == nil {
		t.Errorf("expected error for out of bounds directory offset")
	}
}

func TestWriterAddReader(t *testing.T) {
	var buf bytes.Buffer
	w := pak.NewWriter(&buf)
	if err := w.AddReader("test.txt", bytes.NewBufferString("reader content")); err != nil {
		t.Fatalf("AddReader failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	r, err := pak.OpenBytes("test.pak", buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes failed: %v", err)
	}
	defer func() { _ = r.Close() }()

	data, err := r.ReadFile("test.txt")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(data) != "reader content" {
		t.Errorf("got %q, want %q", string(data), "reader content")
	}

	// Negative bounds check in ReadAt
	_, err = r.ReadAt(pak.Entry{FilePos: -1, FileLen: 10})
	if err == nil {
		t.Errorf("expected error for negative FilePos")
	}
	_, err = r.ReadAt(pak.Entry{FilePos: 10, FileLen: -1})
	if err == nil {
		t.Errorf("expected error for negative FileLen")
	}
}

func TestWritePackCaseInsensitiveDuplicates(t *testing.T) {
	entries := []pak.FileEntry{
		{Name: "test.txt", Data: []byte("content 1")},
		{Name: "TEST.TXT", Data: []byte("content 2")},
	}
	var buf bytes.Buffer
	err := pak.WritePack(&buf, entries)
	if err == nil {
		t.Errorf("expected error for case-insensitive duplicate names in WritePack")
	}
}


