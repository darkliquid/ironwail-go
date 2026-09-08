package bsp

import (
	"encoding/binary"
	"fmt"
)

// QLitVersion is the supported version of the Quake .lit color lightmap format.
const QLitVersion = 1

// ApplyLitFile validates a Quake .lit sidecar and replaces the tree's lighting
// lump with RGB triplets when the file matches the BSP's base light sample
// count. Invalid .lit files return an error so callers can log and fall back to
// the BSP's original monochrome lighting.
func ApplyLitFile(tree *Tree, litData []byte) error {
	if tree == nil || len(litData) == 0 {
		return nil
	}
	if len(litData) < 8 {
		return fmt.Errorf("corrupt .lit file: too short")
	}
	if string(litData[:4]) != "QLIT" {
		return fmt.Errorf("corrupt .lit file: bad magic")
	}
	version := binary.LittleEndian.Uint32(litData[4:8])
	if version != QLitVersion {
		return fmt.Errorf("unknown .lit file version %d", version)
	}

	expected := 8 + len(tree.Lighting)*3
	if len(litData) != expected {
		return fmt.Errorf("outdated .lit file: got %d bytes want %d", len(litData), expected)
	}

	tree.Lighting = append(tree.Lighting[:0:0], litData[8:]...)
	tree.LightingRGB = true
	return nil
}
