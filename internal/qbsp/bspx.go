package qbsp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

// AppendBSPX appends a BRUSHLIST BSPX lump to compiled BSP data (the
// standard "extra lumps appended after the base 15-lump BSP" convention):
// per model, the original solid brushes with their bounds, contents, and
// face planes, so tools can reason about brush geometry without re-parsing
// the .map. The engine tolerates the lump's absence and is untouched by it.
func AppendBSPX(bspData []byte, groups []brushGroup) ([]byte, error) {
	payload := serializeBSPXBrushes(groups)

	const headerSize = 8 // "BSPX" + uint32 numlumps
	const lumpEntrySize = 32

	var b bytes.Buffer
	b.Write(bspData)
	// ericw locates the BSPX header at the 4-aligned end of the last
	// official lump; pad after the base data so the header lands exactly
	// there.
	base := len(bspData)
	for base%4 != 0 {
		b.WriteByte(0)
		base++
	}
	lumpOfs := uint32(base) + headerSize + lumpEntrySize

	b.Write([]byte("BSPX"))
	var nl [4]byte
	binary.LittleEndian.PutUint32(nl[:], 1)
	b.Write(nl[:])

	// Lump entry: 24-byte name + uint32 ofs + uint32 len.
	var name [24]byte
	copy(name[:], "BRUSHLIST")
	b.Write(name[:])
	var hdr [8]byte
	binary.LittleEndian.PutUint32(hdr[0:], lumpOfs)
	binary.LittleEndian.PutUint32(hdr[4:], uint32(len(payload)))
	b.Write(hdr[:])

	b.Write(payload)
	return b.Bytes(), nil
}

// serializeBSPXBrushes renders the BRUSHLIST payload: one permodel record
// per model group (ver=1, modelnum, brush count, face count, brushes),
// matching the ericw bspxbrushes_permodel layout:
//
//	int32 ver; int32 modelnum; int32 numbrushes; int32 numfaces;
//	per brush: aabb3f mins/maxs (6 f32), int16 contents, uint16 numfaces,
//	           per face: qplane3f (normal 3 f32 + dist f32).
func serializeBSPXBrushes(groups []brushGroup) []byte {
	var b bytes.Buffer
	for mi, g := range groups {
		if len(g.brushes) == 0 {
			continue
		}
		var rec [16]byte
		binary.LittleEndian.PutUint32(rec[0:], 1) // ver
		binary.LittleEndian.PutUint32(rec[4:], uint32(mi))
		binary.LittleEndian.PutUint32(rec[8:], uint32(len(g.brushes)))
		totalFaces := 0
		for _, wb := range g.brushes {
			totalFaces += len(wb.orig.Faces)
		}
		binary.LittleEndian.PutUint32(rec[12:], uint32(totalFaces))
		b.Write(rec[:])
		for i := range g.brushes {
			writeBSPXBrush(&b, &g.brushes[i])
		}
	}
	return b.Bytes()
}

func writeBSPXBrush(b *bytes.Buffer, wb *worldBrush) {
	var tmp [28]byte
	for i := 0; i < 3; i++ {
		binary.LittleEndian.PutUint32(tmp[i*4:], math.Float32bits(float32(wb.bounds[0][i])))
		binary.LittleEndian.PutUint32(tmp[12+i*4:], math.Float32bits(float32(wb.bounds[1][i])))
	}
	b.Write(tmp[:])
	var cn [4]byte
	binary.LittleEndian.PutUint16(cn[0:], uint16(int16(wb.content)))
	binary.LittleEndian.PutUint16(cn[2:], uint16(len(wb.orig.Faces)))
	b.Write(cn[:])
	for _, f := range wb.orig.Faces {
		p := f.Plane()
		var pf [16]byte
		for i := 0; i < 3; i++ {
			binary.LittleEndian.PutUint32(pf[i*4:], math.Float32bits(float32(p.Normal[i])))
		}
		binary.LittleEndian.PutUint32(pf[12:], math.Float32bits(float32(p.Dist)))
		b.Write(pf[:])
	}
}

// ReadBSPXBrushList extracts the BRUSHLIST payload from BSP data appended
// via AppendBSPX, returning per-model brush counts (in model order).
func ReadBSPXBrushList(bspData []byte) ([]int, error) {
	idx := bytes.Index(bspData, []byte("BSPX"))
	if idx < 0 {
		return nil, fmt.Errorf("bspx: no BSPX block")
	}
	if idx+8 > len(bspData) {
		return nil, fmt.Errorf("bspx: truncated header")
	}
	numLumps := binary.LittleEndian.Uint32(bspData[idx+4:])
	if numLumps == 0 {
		return nil, fmt.Errorf("bspx: no lumps")
	}
	ent := idx + 8
	if ent+32 > len(bspData) {
		return nil, fmt.Errorf("bspx: truncated lump table")
	}
	name := string(bytes.TrimRight(bspData[ent:ent+24], "\x00"))
	if name != "BRUSHLIST" {
		return nil, fmt.Errorf("bspx: unknown lump %q", name)
	}
	ofs := binary.LittleEndian.Uint32(bspData[ent+24:])
	ln := binary.LittleEndian.Uint32(bspData[ent+28:])
	if int(ofs)+int(ln) > len(bspData) {
		return nil, fmt.Errorf("bspx: brush list out of range")
	}
	payload := bspData[ofs : ofs+ln]
	var counts []int
	p := 0
	for p+16 <= len(payload) {
		modelnum := int32(binary.LittleEndian.Uint32(payload[p+4:]))
		numBrushes := int(binary.LittleEndian.Uint32(payload[p+8:]))
		p += 16
		for bi := 0; bi < numBrushes; bi++ {
			if p+32 > len(payload) {
				return nil, fmt.Errorf("bspx: truncated brush %d of model %d", bi, modelnum)
			}
			p += 28 // aabb bounds
			nf := int(binary.LittleEndian.Uint16(payload[p+2 : p+4]))
			p += 4 // contents + numfaces
			if p+nf*16 > len(payload) {
				return nil, fmt.Errorf("bspx: truncated faces of brush %d", bi)
			}
			p += nf * 16
		}
		for int32(len(counts)) <= modelnum {
			counts = append(counts, 0)
		}
		counts[modelnum] = numBrushes
	}
	return counts, nil
}
