// Package vis computes Quake PVS visibility from a compiled BSP plus its .prt
// portal file and writes the results back into the BSP's visibility lump
// (bead ironwail-go-t63, M3). The row format matches the engine's
// DecompressVis (literal bytes + 0x00/count zero-skips); leaf visofs offsets
// and the world model's visleafs are patched accordingly.
package vis

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/qbsp"
)

// Run computes PVS for the map's leaves and returns a new BSP image with
// the visibility lump filled. bspData must be a BSP produced by our qbsp
// (non-solid leaves numbered first; model visleafs = non-solid count).
func Run(bspData, prtData []byte) ([]byte, error) {
	version, lumps, err := qbsp.ReadBSPLumps(bytes.NewReader(bspData))
	if err != nil {
		return nil, fmt.Errorf("vis: read bsp: %w", err)
	}
	bsp2 := version == bsp.BSP2Version_BSP2 || version == bsp.BSP2Version_2PSB

	pf, err := loadPrtFile(bytes.NewReader(prtData))
	if err != nil {
		return nil, err
	}

	// Leaf parameters from the raw leafs lump.
	leafLump := lumps[10]
	leafSize := 28
	if bsp2 {
		leafSize = 44
	}
	if len(leafLump) == 0 || len(leafLump)%leafSize != 0 {
		return nil, fmt.Errorf("vis: bad leafs lump (%d bytes, %d/leaf)", len(leafLump), leafSize)
	}
	leafCount := len(leafLump) / leafSize
	visLeafs := 0
	solid := make([]bool, leafCount)
	for i := 0; i < leafCount; i++ {
		contents := int32(binary.LittleEndian.Uint32(leafLump[i*leafSize:]))
		solid[i] = contents == bsp.ContentsSolid
		if !solid[i] {
			visLeafs++
		}
	}

	rows := computePVS(visLeafs, pf.Portals)

	// Compress rows and assign visofs.
	visLump := make([]byte, 0, visLeafs*8)
	visOfs := make([]int32, leafCount)
	for i := range visOfs {
		visOfs[i] = -1
	}
	for i := 0; i < leafCount; i++ {
		if solid[i] {
			continue
		}
		visOfs[i] = int32(len(visLump))
		visLump = append(visLump, compressRow(rows[i])...)
	}

	// Patch the leafs lump (visofs @ +4).
	newLeafs := make([]byte, len(leafLump))
	copy(newLeafs, leafLump)
	for i := 0; i < leafCount; i++ {
		binary.LittleEndian.PutUint32(newLeafs[i*leafSize+4:], uint32(visOfs[i]))
	}

	// Patch the world model's visleafs (DModel offset 52) so the engine
	// sizes PVS rows correctly.
	if len(lumps[14]) >= 56 {
		models := append([]byte(nil), lumps[14]...)
		binary.LittleEndian.PutUint32(models[52:], uint32(visLeafs))
		lumps[14] = models
	}

	lumps[4] = visLump
	lumps[10] = newLeafs
	return qbsp.WriteBSP(lumps, version)
}

// compressRow RLE-compresses one uncompressed PVS row for the engine's
// DecompressVis: literal bytes are copied; a 0x00 byte followed by a count
// skips that many (pre-zeroed) output bytes.
func compressRow(row []byte) []byte {
	out := make([]byte, 0, len(row))
	i := 0
	for i < len(row) {
		if row[i] == 0 {
			run := 1
			for i+run < len(row) && row[i+run] == 0 && run < 255 {
				run++
			}
			out = append(out, 0, byte(run))
			i += run
			continue
		}
		out = append(out, row[i])
		i++
	}
	return out
}

var _ = io.Discard