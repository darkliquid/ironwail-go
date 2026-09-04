package fs

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
)

// PakEntry is a single file to be stored in a PAK archive.
type PakEntry struct {
	Name string // virtual path: forward slashes, <= 56 bytes, NUL-free
	Data []byte
}

// ValidPakName validates a Quake PAK file name against the format and the
// reader's expectations:
//
//   - non-empty, no NUL bytes, at most 56 bytes (the on-disk name field)
//   - forward slashes only (no backslashes)
//   - no leading or trailing '/', no "." or ".." path elements (the last
//     also guarantees extraction cannot escape a directory)
//   - no empty path elements ("//")
//
// The archive format does not require lowercase names — the engine's lookup
// is case-insensitive (canonicalPackLookup) — so case is preserved.
func ValidPakName(name string) error {
	switch {
	case name == "":
		return fmt.Errorf("empty file name")
	case len(name) > 56:
		return fmt.Errorf("file name %q exceeds 56 bytes", name)
	case strings.ContainsRune(name, '\x00'):
		return fmt.Errorf("file name %q contains a NUL byte", name)
	case strings.ContainsRune(name, '\\'):
		return fmt.Errorf("file name %q uses backslashes (use forward slashes)", name)
	case strings.HasPrefix(name, "/"):
		return fmt.Errorf("file name %q has a leading slash", name)
	case strings.HasSuffix(name, "/"):
		return fmt.Errorf("file name %q is a directory, not a file", name)
	}
	for _, elem := range strings.Split(name, "/") {
		switch elem {
		case "":
			return fmt.Errorf("file name %q contains an empty path element", name)
		case ".":
			return fmt.Errorf("file name %q contains a \".\" path element", name)
		case "..":
			return fmt.Errorf("file name %q contains a \"..\" path element", name)
		}
	}
	return nil
}

// WritePack writes a PAK archive to w: the 12-byte "PACK" header, each
// file's data sequentially, then the 64-byte-per-entry directory table.
//
// The layout mirrors what loadPackFromHandle parses (and Quake's
// COM_LoadPackFile): header ID "PACK", int32 DirOfs, int32 DirLen; each
// directory entry is [56]byte NUL-padded name, int32 FilePos, int32
// FileLen, all little-endian.
//
// Entries are sorted by name and names are validated, so identical input
// produces byte-identical output (deterministic archives for tests) and
// the result always round-trips through LoadPackFromBytes.
func WritePack(w io.Writer, entries []PakEntry) error {
	if len(entries) == 0 {
		return writeEmptyPack(w)
	}

	sorted := make([]PakEntry, len(entries))
	copy(sorted, entries)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	seen := make(map[string]struct{}, len(sorted))
	offsets := make([]int, len(sorted))
	pos := 12 // header size
	for i, e := range sorted {
		if err := ValidPakName(e.Name); err != nil {
			return err
		}
		if _, dup := seen[e.Name]; dup {
			return fmt.Errorf("duplicate file name %q in pack", e.Name)
		}
		seen[e.Name] = struct{}{}
		if int64(pos) > math.MaxInt32 {
			return fmt.Errorf("pack exceeds the 2 GiB format limit (entry %q)", e.Name)
		}
		offsets[i] = pos
		pos += len(e.Data)
	}
	if int64(pos) > math.MaxInt32 {
		return fmt.Errorf("pack exceeds the 2 GiB format limit")
	}
	dirOff := pos
	dirLen := 64 * len(sorted)

	var header struct {
		ID     [4]byte
		DirOfs int32
		DirLen int32
	}
	copy(header.ID[:], "PACK")
	header.DirOfs, header.DirLen = int32(dirOff), int32(dirLen)
	if err := binary.Write(w, binary.LittleEndian, &header); err != nil {
		return fmt.Errorf("write pack header: %w", err)
	}

	for _, e := range sorted {
		if _, err := w.Write(e.Data); err != nil {
			return fmt.Errorf("write pack data for %q: %w", e.Name, err)
		}
	}

	for i, e := range sorted {
		var entry struct {
			Name    [56]byte
			FilePos int32
			FileLen int32
		}
		copy(entry.Name[:], e.Name)
		entry.FilePos, entry.FileLen = int32(offsets[i]), int32(len(e.Data))
		if err := binary.Write(w, binary.LittleEndian, &entry); err != nil {
			return fmt.Errorf("write pack directory entry %q: %w", e.Name, err)
		}
	}
	return nil
}

// writeEmptyPack writes a valid archive with no files: data and directory
// both start at byte 12.
func writeEmptyPack(w io.Writer) error {
	_, err := fmt.Fprintf(w, "PACK")
	if err != nil {
		return fmt.Errorf("write pack header: %w", err)
	}
	var nul [8]byte
	binary.LittleEndian.PutUint32(nul[0:4], 12) // DirOfs
	binary.LittleEndian.PutUint32(nul[4:8], 0)  // DirLen
	_, err = w.Write(nul[:])
	if err != nil {
		return fmt.Errorf("write pack header: %w", err)
	}
	return nil
}