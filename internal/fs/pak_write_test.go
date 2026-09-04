package fs

import (
	"bytes"
	"encoding/binary"
	"io"
	"path"
	"strings"
	"testing"
)

func writePackBytes(t *testing.T, entries []PakEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := WritePack(&buf, entries); err != nil {
		t.Fatalf("WritePack: %v", err)
	}
	return buf.Bytes()
}

func TestWritePackRoundTrip(t *testing.T) {
	entries := []PakEntry{
		{Name: "progs.dat", Data: []byte("progs-bytes")},
		{Name: "maps/e1m1.bsp", Data: []byte("bsp-data")},
		{Name: "gfx/palette.lmp", Data: []byte{0, 1, 2, 3, 4}},
	}
	data := writePackBytes(t, entries)

	pack, err := LoadPackFromBytes("pak0.pak", data)
	if err != nil {
		t.Fatalf("LoadPackFromBytes: %v", err)
	}
	if len(pack.Files) != len(entries) {
		t.Fatalf("loaded %d files, want %d", len(pack.Files), len(entries))
	}
	// Entries must be sorted by name in the directory table.
	wantOrder := []string{"gfx/palette.lmp", "maps/e1m1.bsp", "progs.dat"}
	for i, want := range wantOrder {
		if pack.Files[i].Name != want {
			t.Errorf("dir entry %d = %q, want %q (sorted)", i, pack.Files[i].Name, want)
		}
	}

	pakFS := NewPakFS(pack)
	for _, e := range entries {
		got, err := pakFS.ReadFile(e.Name)
		if err != nil {
			t.Errorf("ReadFile(%q): %v", e.Name, err)
			continue
		}
		if !bytes.Equal(got, e.Data) {
			t.Errorf("ReadFile(%q) = %q, want %q", e.Name, got, e.Data)
		}
	}
}

func TestWritePackDeterministic(t *testing.T) {
	entries := []PakEntry{
		{Name: "b.txt", Data: []byte("bb")},
		{Name: "a.txt", Data: []byte("aa")},
	}
	first := writePackBytes(t, entries)
	// Same content, different input order: output must be byte-identical.
	second := writePackBytes(t, []PakEntry{entries[1], entries[0]})
	if !bytes.Equal(first, second) {
		t.Error("WritePack output is not deterministic across input orderings")
	}
}

func TestWritePackEmpty(t *testing.T) {
	data := writePackBytes(t, nil)
	pack, err := LoadPackFromBytes("pak0.pak", data)
	if err != nil {
		t.Fatalf("LoadPackFromBytes(empty): %v", err)
	}
	if len(pack.Files) != 0 {
		t.Errorf("empty pack loaded %d files, want 0", len(pack.Files))
	}
}

func TestWritePackDuplicateNames(t *testing.T) {
	err := WritePack(io.Discard, []PakEntry{
		{Name: "a.txt", Data: []byte("1")},
		{Name: "a.txt", Data: []byte("2")},
	})
	if err == nil {
		t.Fatal("WritePack accepted duplicate names")
	}
}

func TestValidPakName(t *testing.T) {
	long := "x/" + string(bytes.Repeat([]byte("y"), 55)) // name total 57 bytes
	invalid := []struct {
		name, want string
	}{
		{"", "empty"},
		{"a\\b", "backslashes"},
		{"/abs", "leading slash"},
		{"trail/", "directory"},
		{"a//b", "empty path element"},
		{"a/./b", "\".\" path element"},
		{"a/../b", "\"..\" path element"},
		{"..", "\"..\" path element"},
		{".", "\".\" path element"},
		{"a\x00b", "NUL"},
		{long, "exceeds 56 bytes"},
	}
	for _, tc := range invalid {
		err := ValidPakName(tc.name)
		if err == nil {
			t.Errorf("ValidPakName(%q) accepted, want error (%s)", tc.name, tc.want)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("ValidPakName(%q) error %q does not mention %q", tc.name, err, tc.want)
		}
	}

	for _, name := range []string{"progs.dat", "maps/e1m1.bsp", "gfx/palette.lmp", "a/b/c.txt"} {
		if err := ValidPakName(name); err != nil {
			t.Errorf("ValidPakName(%q) rejected: %v", name, err)
		}
	}
	longValid := make([]byte, 56) // exactly the on-disk name field size
	for i := range longValid {
		longValid[i] = 'a'
	}
	if err := ValidPakName(string(longValid)); err != nil {
		t.Errorf("ValidPakName(%d-byte name) rejected: %v", len(longValid), err)
	}
}

func TestLoadPackCorruptEntriesDoNotPanic(t *testing.T) {
	// Craft an archive whose directory entry points far past the end of the
	// file. The reader is lazy (no bounds check); ReadFile must not panic
	// and must either error or return partial data.
	var buf bytes.Buffer
	buf.WriteString("PACK")
	_ = binary.Write(&buf, binary.LittleEndian, int32(14)) // DirOfs: 12 header + 2 data
	_ = binary.Write(&buf, binary.LittleEndian, int32(64)) // DirLen: 1 entry
	buf.Write([]byte("ab"))                           // data at 12..14
	entry := make([]byte, 64)
	copy(entry, "a.txt")
	binary.LittleEndian.PutUint32(entry[56:60], 12)     // FilePos
	binary.LittleEndian.PutUint32(entry[60:64], 999999) // FileLen far past EOF
	buf.Write(entry)

	pack, err := LoadPackFromBytes("corrupt.pak", buf.Bytes())
	if err != nil {
		t.Fatalf("LoadPackFromBytes(corrupt): %v", err)
	}
	got, err := NewPakFS(pack).ReadFile("a.txt")
	if err != nil {
		t.Logf("corrupt read error (acceptable): %v", err)
	}
	if len(got) > 999999 {
		t.Errorf("corrupt read returned %d bytes, want <= 999999", len(got))
	}
}

func TestWritePackVirtualPaths(t *testing.T) {
	// Names use Quake virtual paths (forward slashes); the writer must not
	// interpret them as OS paths.
	entries := []PakEntry{{Name: "maps/start.map", Data: []byte("map")}}
	data := writePackBytes(t, entries)
	pack, err := LoadPackFromBytes("x.pak", data)
	if err != nil {
		t.Fatalf("LoadPackFromBytes: %v", err)
	}
	if got := pack.Files[0].Lookup; got != "maps/start.map" {
		t.Errorf("lookup = %q, want maps/start.map", got)
	}
	if pack.Files[0].Name != path.Clean(pack.Files[0].Name) {
		t.Error("virtual path was altered")
	}
}