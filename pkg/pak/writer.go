package pak

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sort"
)

// Writer provides sequential construction of a Quake PAK archive.
type Writer struct {
	w       io.Writer
	entries []FileEntry
	closed  bool
}

// NewWriter creates a new Writer writing to w.
func NewWriter(w io.Writer) *Writer {
	return &Writer{
		w:       w,
		entries: make([]FileEntry, 0),
	}
}

// AddFile adds a file with the given name and byte contents to the archive.
func (w *Writer) AddFile(name string, data []byte) error {
	if w.closed {
		return fmt.Errorf("pak: writer closed")
	}
	if err := ValidName(name); err != nil {
		return err
	}
	w.entries = append(w.entries, FileEntry{
		Name: name,
		Data: data,
	})
	return nil
}

// AddReader reads all data from r and adds it as a file with the given name.
func (w *Writer) AddReader(name string, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read file data for %q: %w", name, err)
	}
	return w.AddFile(name, data)
}

// Close finalizes and writes the PAK archive to the underlying io.Writer.
func (w *Writer) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	return WritePack(w.w, w.entries)
}

// WritePack writes a complete PAK archive containing entries to w:
// the 12-byte "PACK" header, each file's data sequentially, and the
// 64-byte-per-entry directory table.
//
// Entries are sorted by name, ensuring deterministic, byte-identical
// output across runs.
func WritePack(w io.Writer, entries []FileEntry) error {
	if len(entries) == 0 {
		return WriteEmptyPack(w)
	}

	sorted := make([]FileEntry, len(entries))
	copy(sorted, entries)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	seen := make(map[string]struct{}, len(sorted))
	offsets := make([]int, len(sorted))
	pos := HeaderSize

	for i, e := range sorted {
		if err := ValidName(e.Name); err != nil {
			return err
		}
		canonical := CanonicalLookup(e.Name)
		if _, dup := seen[canonical]; dup {
			return fmt.Errorf("duplicate file name %q in pack", e.Name)
		}
		seen[canonical] = struct{}{}
		if int64(pos) > math.MaxInt32 {
			return fmt.Errorf("pack exceeds 2 GiB format limit (entry %q)", e.Name)
		}
		offsets[i] = pos
		pos += len(e.Data)
	}

	if int64(pos) > math.MaxInt32 {
		return fmt.Errorf("pack exceeds 2 GiB format limit")
	}

	dirOff := pos
	dirLen := EntrySize * len(sorted)

	var header Header
	copy(header.ID[:], Magic)
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
		entry.FilePos = int32(offsets[i])
		entry.FileLen = int32(len(e.Data))
		if err := binary.Write(w, binary.LittleEndian, &entry); err != nil {
			return fmt.Errorf("write pack directory entry %q: %w", e.Name, err)
		}
	}

	return nil
}

// WriteEmptyPack writes a valid archive with no files.
func WriteEmptyPack(w io.Writer) error {
	var header Header
	copy(header.ID[:], Magic)
	header.DirOfs = HeaderSize
	header.DirLen = 0

	if err := binary.Write(w, binary.LittleEndian, &header); err != nil {
		return fmt.Errorf("write empty pack header: %w", err)
	}
	return nil
}
