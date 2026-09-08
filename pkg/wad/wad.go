// Package wad implements reading and writing of Quake WAD2 texture and 2D graphic archives.
package wad

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// Magic is the 4-byte identifier at the start of every WAD2 file.
const (
	Magic  = "WAD2"
	Wad2Id = "WAD2"
)

// LumpType specifies the interpretation of a lump's raw data.
//
//go:generate go tool stringer --type LumpType
type LumpType int8

// WAD2 lump type constants matching Quake's wad.h.
const (
	TypNone       = LumpType(0)
	TypLabel      = LumpType(1)
	TypLumpy      = LumpType(64)
	TypPalette    = LumpType(64)
	TypQTex       = LumpType(65)
	TypQPic       = LumpType(66)
	TypSound      = LumpType(67)
	TypMipTex     = LumpType(68)
	TypConsolePic = LumpType(69)
)

// Header is the 12-byte header at the start of a WAD2 file.
type Header struct {
	Identification [4]byte
	NumLumps       int32
	InfoTableOfs   int32
}

// LumpInfo is a 32-byte directory entry in the WAD2 info table.
type LumpInfo struct {
	FilePos     int32
	DiskSize    int32
	Size        int32
	Type        LumpType
	Compression int8
	Pad1, Pad2  int8
	Name        [16]byte
}

// Lump represents a single lump extracted from a WAD2 archive.
type Lump struct {
	Name string
	Type LumpType
	Data []byte
}

// Wad represents a loaded WAD2 archive with all lumps parsed in memory.
type Wad struct {
	Lumps    map[string]Lump
	LumpList []Lump
}

// Get looks up a lump by its name (case-insensitive, normalized).
func (w *Wad) Get(name string) (Lump, bool) {
	if w == nil || w.Lumps == nil {
		return Lump{}, false
	}
	l, ok := w.Lumps[CleanupName(name)]
	return l, ok
}

// Lump is an alias for Get.
func (w *Wad) Lump(name string) (Lump, bool) {
	return w.Get(name)
}

// Open reads and parses a complete WAD2 archive from a random-access source.
func Open(r io.ReaderAt, size int64) (*Wad, error) {
	if size > 0 && size < 12 {
		return nil, fmt.Errorf("wad data too short: %d bytes (minimum 12)", size)
	}

	var header Header
	hdrBuf := make([]byte, 12)
	if _, err := r.ReadAt(hdrBuf, 0); err != nil {
		return nil, fmt.Errorf("read wad header: %w", err)
	}

	copy(header.Identification[:], hdrBuf[:4])
	header.NumLumps = int32(binary.LittleEndian.Uint32(hdrBuf[4:8]))
	header.InfoTableOfs = int32(binary.LittleEndian.Uint32(hdrBuf[8:12]))

	if string(header.Identification[:]) != Magic {
		return nil, fmt.Errorf("not a WAD2 file (bad magic %q)", string(header.Identification[:]))
	}

	if header.NumLumps < 0 {
		return nil, fmt.Errorf("invalid lump count: %d", header.NumLumps)
	}
	if header.InfoTableOfs < 12 || (size > 0 && int64(header.InfoTableOfs) > size) {
		return nil, fmt.Errorf("directory offset %d out of bounds (size %d)", header.InfoTableOfs, size)
	}

	tableSize := int64(header.NumLumps) * 32
	if size > 0 && int64(header.InfoTableOfs)+tableSize > size {
		return nil, fmt.Errorf("directory extends past end of file (%d > %d)", int64(header.InfoTableOfs)+tableSize, size)
	}

	tableBuf := make([]byte, tableSize)
	if tableSize > 0 {
		if _, err := r.ReadAt(tableBuf, int64(header.InfoTableOfs)); err != nil {
			return nil, fmt.Errorf("read info table: %w", err)
		}
	}

	lumps := make(map[string]Lump, header.NumLumps)
	lumpList := make([]Lump, 0, header.NumLumps)

	for i := 0; i < int(header.NumLumps); i++ {
		entBytes := tableBuf[i*32 : (i+1)*32]
		filePos := int32(binary.LittleEndian.Uint32(entBytes[0:4]))
		diskSize := int32(binary.LittleEndian.Uint32(entBytes[4:8]))
		lType := LumpType(entBytes[12])
		rawName := string(entBytes[16:32])
		cleanName := CleanupName(rawName)

		if diskSize < 0 || filePos < 0 {
			return nil, fmt.Errorf("invalid lump bounds: pos=%d size=%d", filePos, diskSize)
		}
		if size > 0 && int64(filePos)+int64(diskSize) > size {
			return nil, fmt.Errorf("lump %q extends past end of file", cleanName)
		}

		data := make([]byte, diskSize)
		if diskSize > 0 {
			if _, err := r.ReadAt(data, int64(filePos)); err != nil {
				return nil, fmt.Errorf("read lump %q: %w", cleanName, err)
			}
		}

		l := Lump{
			Name: cleanName,
			Type: lType,
			Data: data,
		}
		lumps[cleanName] = l
		lumpList = append(lumpList, l)
	}

	return &Wad{
		Lumps:    lumps,
		LumpList: lumpList,
	}, nil
}

// LoadWad reads and parses a complete WAD2 archive from r.
func LoadWad(r io.ReaderAt) (*Wad, error) {
	var size int64
	if s, ok := r.(interface{ Size() int64 }); ok {
		size = s.Size()
	}
	return Open(r, size)
}

// OpenBytes parses a WAD2 archive from an in-memory byte slice.
func OpenBytes(data []byte) (*Wad, error) {
	return Open(bytes.NewReader(data), int64(len(data)))
}

// OpenFile opens and parses a WAD2 archive from the filesystem.
func OpenFile(filename string) (*Wad, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("open wad %q: %w", filename, err)
	}
	defer func() { _ = f.Close() }()

	stat, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat wad %q: %w", filename, err)
	}
	return Open(f, stat.Size())
}
