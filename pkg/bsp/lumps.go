package bsp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// ReadLumps reads the BSP version and 15 raw lump data slices from r.
func ReadLumps(r io.ReaderAt) (int32, [][]byte, error) {
	var version int32
	if err := binary.Read(io.NewSectionReader(r, 0, 4), binary.LittleEndian, &version); err != nil {
		return 0, nil, fmt.Errorf("read bsp version: %w", err)
	}

	var entries [HeaderLumps][8]byte
	raw := make([]byte, HeaderLumps*8)
	if _, err := io.ReadFull(io.NewSectionReader(r, 4, HeaderLumps*8), raw); err != nil {
		return 0, nil, fmt.Errorf("read bsp lump table: %w", err)
	}
	for i := range entries {
		copy(entries[i][:], raw[i*8:])
	}

	lumps := make([][]byte, HeaderLumps)
	for i := range lumps {
		ofs := int32(binary.LittleEndian.Uint32(entries[i][0:]))
		ln := int32(binary.LittleEndian.Uint32(entries[i][4:]))
		if ln <= 0 {
			continue
		}
		buf := make([]byte, ln)
		if _, err := r.ReadAt(buf, int64(ofs)); err != nil {
			return 0, nil, fmt.Errorf("read lump %d: %w", i, err)
		}
		lumps[i] = buf
	}

	return version, lumps, nil
}

// WriteLumps writes a complete BSP file image containing 15 lumps into w.
func WriteLumps(w io.Writer, version int32, lumps [][]byte) error {
	if len(lumps) != HeaderLumps {
		return fmt.Errorf("expected %d lumps, got %d", HeaderLumps, len(lumps))
	}

	const headerSize = 4 + HeaderLumps*8
	var header [headerSize]byte
	binary.LittleEndian.PutUint32(header[0:], uint32(version))

	offset := uint32(headerSize)
	for i, lump := range lumps {
		binary.LittleEndian.PutUint32(header[4+i*8:], offset)
		binary.LittleEndian.PutUint32(header[8+i*8:], uint32(len(lump)))
		offset += uint32(len(lump))
	}

	if _, err := w.Write(header[:]); err != nil {
		return fmt.Errorf("write bsp header: %w", err)
	}

	for i, lump := range lumps {
		if len(lump) > 0 {
			if _, err := w.Write(lump); err != nil {
				return fmt.Errorf("write lump %d: %w", i, err)
			}
		}
	}

	return nil
}

// WriteBSP compiles 15 raw lumps into an in-memory BSP byte slice.
func WriteBSP(lumps [][]byte, version int32) ([]byte, error) {
	var b bytes.Buffer
	if err := WriteLumps(&b, version, lumps); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// PatchLump reads the lumps from r, replaces the lump at lumpIndex with data,
// and writes the reconstructed BSP file into w.
func PatchLump(r io.ReaderAt, w io.Writer, lumpIndex int, data []byte) error {
	if lumpIndex < 0 || lumpIndex >= HeaderLumps {
		return fmt.Errorf("invalid lump index %d (must be 0-%d)", lumpIndex, HeaderLumps-1)
	}

	version, lumps, err := ReadLumps(r)
	if err != nil {
		return fmt.Errorf("read bsp lumps: %w", err)
	}

	lumps[lumpIndex] = data
	return WriteLumps(w, version, lumps)
}
