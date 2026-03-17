// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package common

import (
	"encoding/binary"
	"io"
	"math"
	"strconv"
	"strings"
)

// ─────────────────────────────────────────────────────────────────────────────
// Little-endian and big-endian binary I/O helpers
// ─────────────────────────────────────────────────────────────────────────────
//
// All Quake data files (BSP maps, MDL models, SPR sprites, PAK archives,
// progs.dat QuakeC bytecode) and the network protocol use little-endian
// byte order — a legacy of the x86-only world of 1996. The original C code
// had LittleShort/LittleLong/LittleFloat functions that were no-ops on x86
// and byte-swapped on big-endian platforms. In Go, encoding/binary handles
// this portably.
//
// Two API styles are provided:
//   - Slice-based (LittleShort, LittleLong, LittleFloat): Operate on []byte
//     slices for direct, zero-copy access into memory-mapped file data.
//     These are used when parsing BSP lumps, PAK directory entries, and
//     other pre-loaded binary blobs.
//   - Reader/Writer-based (ReadLittleShort, WriteLittleShort, etc.): Operate
//     on io.Reader/io.Writer for streaming I/O. These are used when reading
//     from pack files or writing network messages and savegame files.
//
// Big-endian helpers (BigShort, BigLong, BigFloat) exist for completeness
// and for parsing external formats that use network byte order.

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

// COM_ParseIntNewline parses an integer from the beginning of 'buffer',
// then consumes any trailing whitespace including the newline.
//
// This function is designed for reading line-oriented text data files where
// each line contains a single numeric value — for example, Quake's savegame
// format (*.sav) and certain server configuration files. The pattern is:
//
//	remaining, value := COM_ParseIntNewline(buffer)
//	// 'value' is the parsed int, 'remaining' points past the newline
//
// Returns 0 for the value if no valid integer is found (matching C's atoi
// behavior on invalid input).
func COM_ParseIntNewline(buffer string) (string, int) {
	buffer = strings.TrimLeft(buffer, " \t\v\f") // skip leading spaces but not newline
	i := 0
	for i < len(buffer) && (buffer[i] == '-' || (buffer[i] >= '0' && buffer[i] <= '9')) {
		i++
	}
	valStr := buffer[:i]
	val, _ := strconv.Atoi(valStr)
	for i < len(buffer) && (buffer[i] == ' ' || buffer[i] == '\t' || buffer[i] == '\r' || buffer[i] == '\n') {
		i++
	}
	return buffer[i:], val
}

// COM_ParseFloatNewline parses a float32 from the beginning of 'buffer',
// then consumes any trailing whitespace including the newline.
//
// Used for reading float values from line-oriented text data such as
// savegame files (entity field values like origin, velocity) and
// lightmap configuration files. Handles negative values and decimal
// points. Returns 0.0 for invalid input.
func COM_ParseFloatNewline(buffer string) (string, float32) {
	buffer = strings.TrimLeft(buffer, " \t\v\f") // skip leading spaces but not newline
	i := 0
	for i < len(buffer) && (buffer[i] == '-' || buffer[i] == '.' || (buffer[i] >= '0' && buffer[i] <= '9')) {
		i++
	}
	valStr := buffer[:i]
	val64, _ := strconv.ParseFloat(valStr, 32)
	for i < len(buffer) && (buffer[i] == ' ' || buffer[i] == '\t' || buffer[i] == '\r' || buffer[i] == '\n') {
		i++
	}
	return buffer[i:], float32(val64)
}

// COM_ParseStringNewline parses a string of non-whitespace characters from
// 'buffer' into the global ComToken, then consumes trailing whitespace
// including the newline. Returns the remaining unparsed portion of buffer.
//
// This is a simpler, line-oriented alternative to [COM_Parse] — it does
// not handle quoted strings, comments, or special single-character tokens.
// It is used for line-by-line text file parsing where each line contains
// a single unquoted word (e.g., entity classnames in certain config formats).
func COM_ParseStringNewline(buffer string) string {
	i := 0
	for i < len(buffer) && buffer[i] > ' ' {
		i++
	}
	ComToken = buffer[:i]
	for i < len(buffer) && (buffer[i] == ' ' || buffer[i] == '\t' || buffer[i] == '\r' || buffer[i] == '\n') {
		i++
	}
	return buffer[i:]
}
