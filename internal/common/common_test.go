package common

import (
	"testing"
)

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
