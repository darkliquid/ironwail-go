// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package binary

import (
	"encoding/binary"
	"io"
	"math"
)

// LittleShort reads a 16-bit signed integer from a 2-byte little-endian slice.
// Used for BSP lump offsets, texture dimensions, and network protocol fields.
func LittleShort(data []byte) int16 {
	return int16(binary.LittleEndian.Uint16(data))
}

// LittleLong reads a 32-bit signed integer from a 4-byte little-endian slice.
// Used for BSP lump sizes, PAK file offsets, model vertex counts, and
// magic numbers (file format identifiers like IDSP, IDPO, PACK).
func LittleLong(data []byte) int32 {
	return int32(binary.LittleEndian.Uint32(data))
}

// LittleFloat reads a 32-bit float from a 4-byte little-endian IEEE 754 slice.
// Used for BSP plane distances, model vertex coordinates, bounding box
// extents, and other geometric data stored in binary file formats.
func LittleFloat(data []byte) float32 {
	bits := binary.LittleEndian.Uint32(data)
	return math.Float32frombits(bits)
}

// BigShort reads a 16-bit signed integer in big-endian (network byte order) format.
func BigShort(data []byte) int16 {
	return int16(binary.BigEndian.Uint16(data))
}

// BigLong reads a 32-bit signed integer in big-endian (network byte order) format.
func BigLong(data []byte) int32 {
	return int32(binary.BigEndian.Uint32(data))
}

// BigFloat reads a 32-bit float in big-endian (network byte order) IEEE 754 format.
func BigFloat(data []byte) float32 {
	bits := binary.BigEndian.Uint32(data)
	return math.Float32frombits(bits)
}

// WriteLittleShort writes a 16-bit signed integer to a writer in little-endian
// format. Used for serializing network messages and savegame data.
func WriteLittleShort(w io.Writer, v int16) error {
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:], uint16(v))
	_, err := w.Write(buf[:])
	return err
}

// WriteLittleLong writes a 32-bit signed integer to a writer in little-endian
// format. Used for serializing entity counts, file offsets, and protocol fields.
func WriteLittleLong(w io.Writer, v int32) error {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], uint32(v))
	_, err := w.Write(buf[:])
	return err
}

// WriteLittleFloat writes a 32-bit float to a writer in little-endian IEEE 754
// format. Used for serializing coordinates, angles, and other floating-point
// values in savegames and binary export formats.
func WriteLittleFloat(w io.Writer, v float32) error {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], math.Float32bits(v))
	_, err := w.Write(buf[:])
	return err
}

// ReadLittleShort reads a 16-bit signed integer from a reader in little-endian
// format. Uses io.ReadFull to ensure exactly 2 bytes are consumed.
func ReadLittleShort(r io.Reader) (int16, error) {
	var buf [2]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return int16(binary.LittleEndian.Uint16(buf[:])), nil
}

// ReadLittleLong reads a 32-bit signed integer from a reader in little-endian
// format. Uses io.ReadFull to ensure exactly 4 bytes are consumed.
func ReadLittleLong(r io.Reader) (int32, error) {
	var buf [4]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(buf[:])), nil
}

// ReadLittleFloat reads a 32-bit float from a reader in little-endian IEEE 754
// format. Uses io.ReadFull to ensure exactly 4 bytes are consumed.
func ReadLittleFloat(r io.Reader) (float32, error) {
	var buf [4]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(binary.LittleEndian.Uint32(buf[:])), nil
}
