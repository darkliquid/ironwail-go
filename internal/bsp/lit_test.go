package bsp

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestApplyLitFileReplacesTreeLightingWithRGB(t *testing.T) {
	tree := &Tree{Lighting: []byte{10, 20}}
	lit := make([]byte, 8+len(tree.Lighting)*3)
	copy(lit[:4], []byte("QLIT"))
	binary.LittleEndian.PutUint32(lit[4:8], qlitVersion)
	copy(lit[8:], []byte{1, 2, 3, 4, 5, 6})

	if err := ApplyLitFile(tree, lit); err != nil {
		t.Fatalf("ApplyLitFile error: %v", err)
	}
	if !tree.LightingRGB {
		t.Fatal("expected LightingRGB to be true")
	}
	if got, want := string(tree.Lighting), string([]byte{1, 2, 3, 4, 5, 6}); got != want {
		t.Fatalf("lighting = %v, want %v", tree.Lighting, []byte{1, 2, 3, 4, 5, 6})
	}
}

func TestApplyLitFileRejectsOutdatedSize(t *testing.T) {
	tree := &Tree{Lighting: []byte{10, 20}}
	lit := make([]byte, 8+3)
	copy(lit[:4], []byte("QLIT"))
	binary.LittleEndian.PutUint32(lit[4:8], qlitVersion)

	err := ApplyLitFile(tree, lit)
	if err == nil {
		t.Fatal("expected outdated .lit error")
	}
	if !strings.Contains(err.Error(), "outdated .lit") {
		t.Fatalf("error = %q, want outdated .lit text", err)
	}
	if tree.LightingRGB {
		t.Fatal("LightingRGB changed on invalid .lit")
	}
}
