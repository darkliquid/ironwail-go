package fs

import (
	"io"

	"github.com/darkliquid/ironwail-go/pkg/pak"
)

// PakEntry is an alias for pak.FileEntry for backwards compatibility.
type PakEntry = pak.FileEntry

// ValidPakName validates a Quake PAK entry name using pak.ValidName.
func ValidPakName(name string) error {
	return pak.ValidName(name)
}

// WritePack writes a complete PAK archive to w using pak.WritePack.
func WritePack(w io.Writer, entries []PakEntry) error {
	return pak.WritePack(w, entries)
}