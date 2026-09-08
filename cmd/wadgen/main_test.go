package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/pkg/wad"
)

func TestWadgenPlaceholder(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "placeholder.wad")

	writePlaceholderWad(out)

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read wad: %v", err)
	}
	archive, err := wad.OpenBytes(data)
	if err != nil {
		t.Fatalf("open wad bytes: %v", err)
	}

	expectedLumps := []string{"palette.lmp", "gfx/qplaque.lmp", "gfx/mainmenu.lmp", "gfx/m_surfs.lmp"}
	for _, name := range expectedLumps {
		if _, ok := archive.Get(name); !ok {
			t.Errorf("missing lump %s", name)
		}
	}
}
