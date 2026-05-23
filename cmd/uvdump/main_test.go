package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunRequiresPakPathOrEnvironment(t *testing.T) {
	t.Setenv("QUAKE_PAKPATH", "")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"progs/player.mdl"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "pak file path is required") {
		t.Fatalf("stderr = %q, want pak path diagnostic", stderr.String())
	}
}

func TestRunReportsReadErrors(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"/does/not/exist.pak", "progs/player.mdl"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "uvdump: read /does/not/exist.pak") {
		t.Fatalf("stderr = %q, want read diagnostic", stderr.String())
	}
}
