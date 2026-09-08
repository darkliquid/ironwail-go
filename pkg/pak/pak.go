// Package pak implements reading, writing, and layered virtual filesystem (VFS)
// mounting for Quake PAK archives.
//
// # Quake PAK Format
//
// A PAK archive consists of a 12-byte header, file data blocks, and a directory table:
//
//	Header (12 bytes):
//	  [4]byte ID     - Magic string "PACK"
//	  int32   DirOfs - Byte offset to the central directory (little-endian)
//	  int32   DirLen - Byte length of the central directory (DirLen / 64 = number of entries)
//
//	Directory Entry (64 bytes each):
//	  [56]byte Name    - Null-terminated, slash-separated virtual path
//	  int32    FilePos - Byte offset of file data within archive (little-endian)
//	  int32    FileLen - Byte length of file data (little-endian)
//
// All integers are stored little-endian. Lookups are case-insensitive and
// slash-normalised (both forward- and back-slashes are accepted, lowercased).
package pak

import (
	"fmt"
	"strings"
)

// Well-known format constants and path limits.
const (
	Magic         = "PACK"
	HeaderSize    = 12
	EntrySize     = 64
	MaxFileName   = 56
	MaxQPath      = 64
	MaxOSPath     = 1024
	EnginePakName = "ironwail.pak"
)

// Header is the 12-byte header at the start of every PAK file.
type Header struct {
	ID     [4]byte
	DirOfs int32
	DirLen int32
}

// Entry describes a single file within a PAK archive directory.
type Entry struct {
	Name    string // original relative path (e.g. "maps/e1m1.bsp")
	Lookup  string // case-folded, slash-normalised key (e.g. "maps/e1m1.bsp")
	FilePos int32  // byte offset of the file data in the archive
	FileLen int32  // byte length of the file data
}

// Offset returns the byte offset of the file within the archive.
func (e Entry) Offset() int32 { return e.FilePos }

// Length returns the length of the file in bytes.
func (e Entry) Length() int32 { return e.FileLen }

// FileEntry represents a file to be written into a PAK archive.
type FileEntry struct {
	Name string // virtual path: forward slashes, <= 56 bytes, NUL-free
	Data []byte
}

// PakEntry is an alias for FileEntry for backwards compatibility.
type PakEntry = FileEntry

// CanonicalLookup normalises a path for case-insensitive, separator-insensitive
// comparison: backslashes become forward slashes and the path is lowercased.
func CanonicalLookup(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "\\", "/"))
}

// CanonicalPackLookup is an alias for CanonicalLookup.
func CanonicalPackLookup(name string) string {
	return CanonicalLookup(name)
}

// ValidName validates a Quake PAK entry name against format constraints:
//
//   - non-empty, no NUL bytes, at most 56 bytes
//   - forward slashes only (no backslashes)
//   - no leading or trailing '/', no "." or ".." path elements
//   - no empty path elements ("//")
func ValidName(name string) error {
	switch {
	case name == "":
		return fmt.Errorf("empty file name")
	case len(name) > MaxFileName:
		return fmt.Errorf("file name %q exceeds %d bytes", name, MaxFileName)
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

// ValidPakName is an alias for ValidName.
func ValidPakName(name string) error {
	return ValidName(name)
}
