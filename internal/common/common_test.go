package common

import (
	"testing"
)

// TestCOM_CheckParm tests command line parameter checking.
// It ensures the engine can correctly identify and locate command line flags (e.g., -game, -nosound).
// Where in C: COM_CheckParm in common.c
func TestCOM_CheckParm(t *testing.T) {
	args := []string{"quake", "-game", "rogue", "-nosound"}
	COM_InitArgv(args)

	if pos := COM_CheckParm("-game"); pos != 1 {
		t.Errorf("Expected -game at pos 1, got %d", pos)
	}
	if pos := COM_CheckParm("-nosound"); pos != 3 {
		t.Errorf("Expected -nosound at pos 3, got %d", pos)
	}
	if pos := COM_CheckParm("-notfound"); pos != 0 {
		t.Errorf("Expected -notfound at pos 0, got %d", pos)
	}
}

// TestCOM_Parse tests the common token parser.
// It ensures that various formats (quoted strings, comments, blocks, symbols) are tokenized correctly for config and data parsing.
// Where in C: COM_Parse in common.c
func TestCOM_Parse(t *testing.T) {
	data := "token1 token2 \"quoted token\" // comment\n token3 /* block\ncomment */ token4 { } ( ) ' :"

	data = COM_Parse(data)
	if ComToken != "token1" {
		t.Errorf("Expected token1, got %s", ComToken)
	}

	data = COM_Parse(data)
	if ComToken != "token2" {
		t.Errorf("Expected token2, got %s", ComToken)
	}

	data = COM_Parse(data)
	if ComToken != "quoted token" {
		t.Errorf("Expected 'quoted token', got %s", ComToken)
	}

	data = COM_Parse(data)
	if ComToken != "token3" {
		t.Errorf("Expected token3, got %s", ComToken)
	}

	data = COM_Parse(data)
	if ComToken != "token4" {
		t.Errorf("Expected token4, got %s", ComToken)
	}

	data = COM_Parse(data)
	if ComToken != "{" {
		t.Errorf("Expected {, got %s", ComToken)
	}

	data = COM_Parse(data)
	if ComToken != "}" {
		t.Errorf("Expected }, got %s", ComToken)
	}

	data = COM_Parse(data)
	if ComToken != "(" {
		t.Errorf("Expected (, got %s", ComToken)
	}

	data = COM_Parse(data)
	if ComToken != ")" {
		t.Errorf("Expected ), got %s", ComToken)
	}

	data = COM_Parse(data)
	if ComToken != "'" {
		t.Errorf("Expected ', got %s", ComToken)
	}

	data = COM_Parse(data)
	if ComToken != ":" {
		t.Errorf("Expected :, got %s", ComToken)
	}
}

// TestPathUtils tests file path utility functions.
// It ensures consistent handling of file extensions and base names across different platforms.
// Where in C: COM_FileBase, COM_FileGetExtension, etc. in common.c
func TestPathUtils(t *testing.T) {
	path := "dir/subdir/file.ext"

	if base := COM_FileBase(path); base != "file" {
		t.Errorf("Expected file, got %s", base)
	}

	if ext := COM_FileGetExtension(path); ext != "ext" {
		t.Errorf("Expected ext, got %s", ext)
	}

	if stripped := COM_StripExtension(path); stripped != "dir/subdir/file" {
		t.Errorf("Expected dir/subdir/file, got %s", stripped)
	}

	if added := COM_AddExtension("file", ".ext"); added != "file.ext" {
		t.Errorf("Expected file.ext, got %s", added)
	}

	if added := COM_AddExtension("file.ext", ".ext"); added != "file.ext" {
		t.Errorf("Expected file.ext, got %s", added)
	}
}

// TestHash tests the string and block hashing functions.
// It provides efficient data lookup and verification for internal caches and systems.
// Where in C: COM_HashKey or similar in common.c
func TestHash(t *testing.T) {
	s := "hello world"
	h1 := COM_HashString(s)
	h2 := COM_HashBlock([]byte(s))

	if h1 != h2 {
		t.Errorf("HashString and HashBlock should match for same data")
	}

	if h1 == 0 {
		t.Errorf("Hash should not be 0")
	}
}

// TestParseNewline tests newline-aware parsing.
// It correctly handles data formats that rely on line-by-line structure.
// Where in C: COM_Parse variants in common.c
func TestParseNewline(t *testing.T) {
	data := " 123\n 45.6\n token\n"
	data, valInt := COM_ParseIntNewline(data)
	if valInt != 123 {
		t.Errorf("Expected 123, got %d", valInt)
	}
	data, valFloat := COM_ParseFloatNewline(data)
	if valFloat != 45.6 {
		t.Errorf("Expected 45.6, got %f", valFloat)
	}
	data = COM_ParseStringNewline(data)
	if ComToken != "token" {
		t.Errorf("Expected token, got %s", ComToken)
	}
}

// TestSizeBufWriteReadAngle tests 8-bit angle compression in size buffers.
// It optimizes network bandwidth by compressing 360-degree angles into a single byte.
// Where in C: MSG_WriteAngle and MSG_ReadAngle in common.c
func TestSizeBufWriteReadAngle(t *testing.T) {
	tests := []struct {
		name  string
		angle float32
	}{
		{"zero", 0.0},
		{"quarter", 90.0},
		{"half", 180.0},
		{"three-quarter", 270.0},
		{"negative", -45.0},
		{"wraparound", 400.0}, // Should wrap to 40.0
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := NewSizeBuf(10)
			if !buf.WriteAngle(tt.angle) {
				t.Fatal("WriteAngle failed")
			}

			buf.BeginReading()
			got, ok := buf.ReadAngle()
			if !ok {
				t.Fatal("ReadAngle failed")
			}

			// Normalize expected angle to 0-360 range for comparison
			expected := tt.angle
			for expected < 0 {
				expected += 360
			}
			for expected >= 360 {
				expected -= 360
			}

			// 8-bit angles have ~1.4 degree precision (360/256)
			// Allow 2 degrees tolerance for rounding
			diff := got - expected
			if diff < 0 {
				diff = -diff
			}
			if diff > 360-diff {
				diff = 360 - diff // Handle wraparound
			}
			if diff > 2.0 {
				t.Errorf("angle mismatch: got %f, want ~%f (diff %f)", got, expected, diff)
			}
		})
	}
}

// TestSizeBufWriteReadAngle16 tests 16-bit angle compression.
// It provides higher precision for angles while still saving space compared to full floats.
// Where in C: MSG_WriteAngle16 and MSG_ReadAngle16 in common.c
func TestSizeBufWriteReadAngle16(t *testing.T) {
	tests := []struct {
		name  string
		angle float32
	}{
		{"zero", 0.0},
		{"precise", 45.5},
		{"quarter", 90.25},
		{"half", 180.125},
		{"negative", -30.75},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := NewSizeBuf(10)
			if !buf.WriteAngle16(tt.angle) {
				t.Fatal("WriteAngle16 failed")
			}

			buf.BeginReading()
			got, ok := buf.ReadAngle16()
			if !ok {
				t.Fatal("ReadAngle16 failed")
			}

			// Normalize expected angle to 0-360 range
			expected := tt.angle
			for expected < 0 {
				expected += 360
			}
			for expected >= 360 {
				expected -= 360
			}

			// 16-bit angles have ~0.0055 degree precision (360/65536)
			// Allow 0.01 degrees tolerance
			diff := got - expected
			if diff < 0 {
				diff = -diff
			}
			if diff > 360-diff {
				diff = 360 - diff // Handle wraparound
			}
			if diff > 0.01 {
				t.Errorf("angle mismatch: got %f, want ~%f (diff %f)", got, expected, diff)
			}
		})
	}
}

// TestSizeBufAnglePrecision compares precision between 8-bit and 16-bit angles.
// It validates that 16-bit compression provides the expected accuracy improvement.
// Where in C: common.c (logic validation)
func TestSizeBufAnglePrecision(t *testing.T) {
	// Verify that 16-bit angles have better precision than 8-bit
	angle := float32(45.5)

	// Test 8-bit
	buf8 := NewSizeBuf(10)
	buf8.WriteAngle(angle)
	buf8.BeginReading()
	got8, _ := buf8.ReadAngle()

	// Test 16-bit
	buf16 := NewSizeBuf(10)
	buf16.WriteAngle16(angle)
	buf16.BeginReading()
	got16, _ := buf16.ReadAngle16()

	// Calculate errors
	err8 := angle - got8
	if err8 < 0 {
		err8 = -err8
	}
	err16 := angle - got16
	if err16 < 0 {
		err16 = -err16
	}

	// 16-bit should be more precise
	if err16 >= err8 {
		t.Errorf("16-bit angle not more precise than 8-bit: err8=%f, err16=%f", err8, err16)
	}

	// 8-bit precision should be roughly 1.4 degrees (360/256)
	if err8 > 2.0 {
		t.Errorf("8-bit angle error too large: %f", err8)
	}

	// 16-bit precision should be very small
	if err16 > 0.01 {
		t.Errorf("16-bit angle error too large: %f", err16)
	}
}
