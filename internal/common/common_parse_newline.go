// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package common

import (
	"strconv"
	"strings"
)

// COM_ParseIntNewline parses an integer from the beginning of 'buffer',
// then consumes any trailing whitespace including the newline.
//
// This function is designed for reading line-oriented text data files where
// each line contains a single numeric value — for example, Quake's savegame
// format (*.sav) and certain server configuration files. The pattern is:
//
// remaining, value := COM_ParseIntNewline(buffer)
// // 'value' is the parsed int, 'remaining' points past the newline
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
