package image

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

const (
	Wad2Id = "WAD2"
)

// LumpType specifies the type of a lump file.
//
//go:generate go tool stringer --type LumpType
type LumpType int8

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

type WadHeader struct {
	Identification [4]byte
	NumLumps       int32
	InfoTableOfs   int32
}

type LumpInfo struct {
	FilePos     int32
	DiskSize    int32
	Size        int32
	Type        LumpType
	Compression int8
	Pad1, Pad2  int8
	Name        [16]byte
}

type Lump struct {
	Name string
	Type LumpType
	Data []byte
}

type Wad struct {
	Lumps map[string]Lump
}

func CleanupName(name string) string {
	name = strings.ToLower(name)
	if i := strings.IndexByte(name, 0); i != -1 {
		name = name[:i]
	}
	return strings.TrimRight(name, " ")
}

func LoadWad(r io.ReaderAt) (*Wad, error) {
	var header WadHeader
	sr := io.NewSectionReader(r, 0, 12)
	if err := binary.Read(sr, binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	if string(header.Identification[:]) != Wad2Id {
		return nil, fmt.Errorf("not a WAD2 file: %s", string(header.Identification[:]))
	}

	lumps := make(map[string]Lump)
	sr = io.NewSectionReader(r, int64(header.InfoTableOfs), int64(header.NumLumps)*32)
	for i := 0; i < int(header.NumLumps); i++ {
		var info LumpInfo
		if err := binary.Read(sr, binary.LittleEndian, &info); err != nil {
			return nil, err
		}

		name := CleanupName(string(info.Name[:]))
		data := make([]byte, info.DiskSize)
		if _, err := r.ReadAt(data, int64(info.FilePos)); err != nil {
			return nil, err
		}

		// Handle QPic swapping if necessary
		if info.Type == TypQPic {
			// QPic has width and height as int32 at the beginning
			// wad.c calls SwapPic which does LittleLong on width and height.
			// Since we are reading from a little-endian file, they should already be correct
			// if we treat them as little-endian.
		}

		lumps[name] = Lump{
			Name: name,
			Type: info.Type,
			Data: data,
		}
	}

	return &Wad{Lumps: lumps}, nil
}

type QPic struct {
	Width  uint32
	Height uint32
	Pixels []byte
}

func ParseQPic(data []byte) (*QPic, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("qpic data too short")
	}

	width := binary.LittleEndian.Uint32(data[0:4])
	height := binary.LittleEndian.Uint32(data[4:8])
	if width == 0 || height == 0 {
		return nil, fmt.Errorf("qpic has invalid size of %dx%d", width, height)
	}

	if len(data) < 8+int(width*height) {
		return nil, fmt.Errorf("qpic data too short for %dx%d", width, height)
	}

	return &QPic{
		Width:  width,
		Height: height,
		Pixels: data[8 : 8+width*height],
	}, nil
}
