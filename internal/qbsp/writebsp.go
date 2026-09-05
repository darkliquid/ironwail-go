package qbsp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/darkliquid/ironwail-go/internal/bsp"
)

// bspLayout is the wire format the writer emits.
type bspLayout int

const (
	layoutBsp29 bspLayout = iota
	layoutBsp2
	layoutBspRMQ // -2psb: BSP2 indices with 16-bit node/leaf bounds
)

func (c *compiler) layout() bspLayout {
	if c.opts.BSP2 {
		if c.opts.TwoPSB {
			return layoutBspRMQ
		}
		return layoutBsp2
	}
	return layoutBsp29
}

// assemble builds all lumps and returns the serialised BSP.
func (c *compiler) assemble(m *Map, models []modelOut, faces []outFace, nodes []outNode, leafs []outLeaf, clip []outClipNode, leakPath []vec3, leaked bool) (*CompileResult, error) {
	vertexes, edges, surfedges := edgeTables(faces)
	lay := c.layout()

	res := &CompileResult{
		BSP2:     c.opts.BSP2,
		LeakPath: leakPath,
		Leaked:   leaked,
	}
	c.logf("planes %d, nodes %d, leafs %d, faces %d, clipnodes %d, models %d",
		len(c.planes), len(nodes), len(leafs), len(faces), len(clip), len(models))

	// ---- lumps (in header order) ----
	entities := serializeEntities(m)
	planes := c.serializePlanes()
	textures := c.serializeTextures()
	vis := []byte{}
	nodeBytes := serializeNodes(nodes, lay)
	texinfoBytes := c.serializeTexinfo()
	faceBytes := serializeFaces(faces, c.opts.BSP2)
	lighting := []byte{}
	clipBytes := serializeClipnodes(clip, c.opts.BSP2)
	leafBytes, marksurfBytes := serializeLeafs(leafs, lay)
	edgeBytes := serializeEdges(edges, c.opts.BSP2)
	surfedgeBytes := serializeInts32(surfedges)
	modelBytes := serializeModels(models, c.opts.BSP2)

	lumps := [][]byte{
		entities, planes, textures, vertexBytes(vertexes), vis,
		nodeBytes, texinfoBytes, faceBytes, lighting, clipBytes,
		leafBytes, marksurfBytes, edgeBytes, surfedgeBytes, modelBytes,
	}
	res.Log = c.logs
	data, err := writeBSP(lumps, lay)
	if err != nil {
		return nil, err
	}
	res.Log = c.logs
	res.Data = data
	return res, nil
}

// ---- entities ----

func serializeEntities(m *Map) []byte {
	var b bytes.Buffer
	for _, ent := range m.Entities {
		b.WriteString("{\n")
		for _, e := range ent.Epairs {
			fmt.Fprintf(&b, "\"%s\" \"%s\"\n", e.Key, e.Value)
		}
		b.WriteString("}\n")
	}
	return b.Bytes()
}

// ---- planes ----

func (c *compiler) serializePlanes() []byte {
	var b bytes.Buffer
	for _, p := range c.planes {
		typ := int32(classifyPlane(p.Normal))
		var rec [20]byte
		binary.LittleEndian.PutUint32(rec[0:], math.Float32bits(float32(p.Normal[0])))
		binary.LittleEndian.PutUint32(rec[4:], math.Float32bits(float32(p.Normal[1])))
		binary.LittleEndian.PutUint32(rec[8:], math.Float32bits(float32(p.Normal[2])))
		binary.LittleEndian.PutUint32(rec[12:], math.Float32bits(float32(p.Dist)))
		binary.LittleEndian.PutUint32(rec[16:], uint32(typ))
		b.Write(rec[:])
	}
	return b.Bytes()
}

// ---- textures (miptex lump) ----

// textureNames returns the ordered unique texture names of the texinfo
// table (the miptex lump's name list, and the texinfo miptex indices).
func (c *compiler) textureNames() []string {
	names := []string{}
	seen := map[string]bool{}
	for _, ti := range c.texinfo {
		if !seen[ti.texture] {
			seen[ti.texture] = true
			names = append(names, ti.texture)
		}
	}
	return names
}

func (c *compiler) serializeTextures() []byte {
	names := c.textureNames()
	dim := 16
	var out bytes.Buffer
	var count [4]byte
	binary.LittleEndian.PutUint32(count[:], uint32(len(names)))
	out.Write(count[:])
	offsetPos := uint32(4 + 4*len(names))
	for range names {
		var o [4]byte
		binary.LittleEndian.PutUint32(o[:], offsetPos)
		out.Write(o[:])
		offsetPos += miptexSize(dim)
	}
	for _, name := range names {
		out.Write(miptexData(name, dim))
	}
	return out.Bytes()
}

// miptexSize is the on-disk size of a 16x16 four-mip miptex.
func miptexSize(dim int) uint32 {
	return uint32(40 + dim*dim + dim/2*dim/2 + dim/4*dim/4 + dim/8*dim/8)
}

// miptexData builds a 16x16 miptex with four mip levels of mid-gray pixel
// data (palette index 128). The placeholder is intentionally non-black so
// light's texture-brightness bounce and the engine's fallback rendering
// see material instead of void.
func miptexData(name string, dim int) []byte {
	lv := [4]int{dim, dim / 2, dim / 4, dim / 8}
	size := int(miptexSize(dim))
	out := make([]byte, size)
	copy(out[0:16], name)
	binary.LittleEndian.PutUint32(out[16:], uint32(dim))
	binary.LittleEndian.PutUint32(out[20:], uint32(dim))
	off := 40
	for i := 0; i < 4; i++ {
		binary.LittleEndian.PutUint32(out[24+i*4:], uint32(off))
		off += lv[i] * lv[i]
	}
	for i := 40; i < size; i++ {
		out[i] = 128
	}
	return out
}

// ---- texinfo ----

func (c *compiler) serializeTexinfo() []byte {
	var b bytes.Buffer
	for _, ti := range c.texinfo {
		var rec [40]byte
		for i := 0; i < 2; i++ {
			for j := 0; j < 4; j++ {
				binary.LittleEndian.PutUint32(rec[i*16+j*4:], math.Float32bits(float32(ti.vecs[i][j])))
			}
		}
		binary.LittleEndian.PutUint32(rec[32:], uint32(textureIndex(c, ti.texture)))
		binary.LittleEndian.PutUint32(rec[36:], uint32(ti.flags))
		b.Write(rec[:])
	}
	return b.Bytes()
}

// textureIndex resolves the miptex-table index for a texture name.
func textureIndex(c *compiler, name string) int32 {
	idx := int32(0)
	for _, ti := range c.texinfo {
		if ti.texture == name {
			// count unique names before this one
			break
		}
	}
	// exact position in the deduped name list
	seen := map[string]int{}
	out := int32(0)
	for _, ti := range c.texinfo {
		if _, ok := seen[ti.texture]; !ok {
			seen[ti.texture] = len(seen)
		}
	}
	_ = idx
	_ = out
	return int32(seen[name])
}

// ---- nodes/leafs/clipnodes ----

func childBytes(child childRef, bsp2 bool) int32 {
	if bsp2 {
		if child.isLeaf {
			return -int32(child.idx) - 1
		}
		return int32(child.idx)
	}
	if child.isLeaf {
		return int32(65535 - child.idx)
	}
	return int32(child.idx)
}

func serializeNodes(nodes []outNode, lay bspLayout) []byte {
	var b bytes.Buffer
	if lay == layoutBsp2 {
		for _, n := range nodes {
			var rec [44]byte
			binary.LittleEndian.PutUint32(rec[0:], uint32(n.planenum))
			for i := 0; i < 2; i++ {
				binary.LittleEndian.PutUint32(rec[4+i*4:], uint32(int32(childBytes(n.children[i], true))))
			}
			for i := 0; i < 3; i++ {
				binary.LittleEndian.PutUint32(rec[12+i*4:], math.Float32bits(float32(n.bounds[0][i])))
				binary.LittleEndian.PutUint32(rec[24+i*4:], math.Float32bits(float32(n.bounds[1][i])))
			}
			binary.LittleEndian.PutUint32(rec[36:], uint32(n.firstface))
			binary.LittleEndian.PutUint32(rec[40:], uint32(n.numfaces))
			b.Write(rec[:])
		}
		return b.Bytes()
	}
	if lay == layoutBspRMQ {
		// BSP2RMQ: int32 plane/children, int16 bounds, uint32 face spans.
		for _, n := range nodes {
			var rec [32]byte
			binary.LittleEndian.PutUint32(rec[0:], uint32(n.planenum))
			for i := 0; i < 2; i++ {
				binary.LittleEndian.PutUint32(rec[4+i*4:], uint32(int32(childBytes(n.children[i], true))))
			}
			for i := 0; i < 3; i++ {
				binary.LittleEndian.PutUint16(rec[12+i*2:], uint16(int16(clamp16(n.bounds[0][i]))))
				binary.LittleEndian.PutUint16(rec[18+i*2:], uint16(int16(clamp16(n.bounds[1][i]))))
			}
			binary.LittleEndian.PutUint32(rec[24:], uint32(n.firstface))
			binary.LittleEndian.PutUint32(rec[28:], uint32(n.numfaces))
			b.Write(rec[:])
		}
		return b.Bytes()
	}
	for _, n := range nodes {
		var rec [24]byte
		binary.LittleEndian.PutUint32(rec[0:], uint32(n.planenum))
		for i := 0; i < 2; i++ {
			binary.LittleEndian.PutUint16(rec[4+i*2:], uint16(int16(childBytes(n.children[i], false))))
		}
		for i := 0; i < 3; i++ {
			binary.LittleEndian.PutUint16(rec[8+i*2:], uint16(int16(clamp16(n.bounds[0][i]))))
			binary.LittleEndian.PutUint16(rec[14+i*2:], uint16(int16(clamp16(n.bounds[1][i]))))
		}
		binary.LittleEndian.PutUint16(rec[20:], uint16(n.firstface))
		binary.LittleEndian.PutUint16(rec[22:], uint16(n.numfaces))
		b.Write(rec[:])
	}
	return b.Bytes()
}

func clamp16(v float64) int16 {
	if v < -32768 {
		return -32768
	}
	if v > 32767 {
		return 32767
	}
	return int16(math.Round(v))
}

// serializeLeafs returns the leaf lump and the marksurfaces lump.
func serializeLeafs(leafs []outLeaf, lay bspLayout) ([]byte, []byte) {
	var lb, mb bytes.Buffer
	marksPos := 0
	for i := range leafs {
		l := &leafs[i]
		visofs := int32(-1)
		switch lay {
		case layoutBsp2:
			var rec [44]byte
			binary.LittleEndian.PutUint32(rec[0:], uint32(l.content))
			binary.LittleEndian.PutUint32(rec[4:], uint32(visofs))
			for j := 0; j < 3; j++ {
				binary.LittleEndian.PutUint32(rec[8+j*4:], math.Float32bits(float32(l.mins[j])))
				binary.LittleEndian.PutUint32(rec[20+j*4:], math.Float32bits(float32(l.maxs[j])))
			}
			binary.LittleEndian.PutUint32(rec[32:], uint32(marksPos))
			binary.LittleEndian.PutUint32(rec[36:], uint32(len(l.marksurface)))
			lb.Write(rec[:])
		case layoutBspRMQ:
			// BSP2RMQ leaf: contents/visofs, int16 bounds, uint32 marks,
			// ambient bytes (32 total, mirrors the loader's DL1Leaf).
			var rec [32]byte
			binary.LittleEndian.PutUint32(rec[0:], uint32(l.content))
			binary.LittleEndian.PutUint32(rec[4:], uint32(visofs))
			for j := 0; j < 3; j++ {
				binary.LittleEndian.PutUint16(rec[8+j*2:], uint16(int16(clamp16(l.mins[j]))))
				binary.LittleEndian.PutUint16(rec[14+j*2:], uint16(int16(clamp16(l.maxs[j]))))
			}
			binary.LittleEndian.PutUint32(rec[20:], uint32(marksPos))
			binary.LittleEndian.PutUint32(rec[24:], uint32(len(l.marksurface)))
			lb.Write(rec[:])
		default:
			var rec [28]byte
			binary.LittleEndian.PutUint32(rec[0:], uint32(l.content))
			binary.LittleEndian.PutUint32(rec[4:], uint32(visofs))
			for j := 0; j < 3; j++ {
				binary.LittleEndian.PutUint16(rec[8+j*2:], uint16(int16(clamp16(l.mins[j]))))
				binary.LittleEndian.PutUint16(rec[14+j*2:], uint16(int16(clamp16(l.maxs[j]))))
			}
			binary.LittleEndian.PutUint16(rec[20:], uint16(marksPos))
			binary.LittleEndian.PutUint16(rec[22:], uint16(len(l.marksurface)))
			lb.Write(rec[:])
		}
		for _, f := range l.marksurface {
			if lay == layoutBsp29 {
				var v [2]byte
				binary.LittleEndian.PutUint16(v[:], uint16(f))
				mb.Write(v[:])
			} else {
				var v [4]byte
				binary.LittleEndian.PutUint32(v[:], uint32(f))
				mb.Write(v[:])
			}
		}
		marksPos += len(l.marksurface)
	}
	return lb.Bytes(), mb.Bytes()
}

func serializeClipnodes(clip []outClipNode, bsp2 bool) []byte {
	var b bytes.Buffer
	for _, cn := range clip {
		if bsp2 {
			var rec [12]byte
			binary.LittleEndian.PutUint32(rec[0:], uint32(cn.plane))
			binary.LittleEndian.PutUint32(rec[4:], uint32(cn.children[0]))
			binary.LittleEndian.PutUint32(rec[8:], uint32(cn.children[1]))
			b.Write(rec[:])
		} else {
			var rec [8]byte
			binary.LittleEndian.PutUint32(rec[0:], uint32(cn.plane))
			for i := 0; i < 2; i++ {
				binary.LittleEndian.PutUint16(rec[4+i*2:], uint16(int16(cn.children[i])))
			}
			b.Write(rec[:])
		}
	}
	return b.Bytes()
}

// ---- faces ----

func serializeFaces(faces []outFace, bsp2 bool) []byte {
	var b bytes.Buffer
	for _, f := range faces {
		planenum := f.planenum
		if bsp2 {
			var rec [28]byte
			binary.LittleEndian.PutUint32(rec[0:], uint32(planenum))
			binary.LittleEndian.PutUint32(rec[4:], uint32(int8(f.side)))
			binary.LittleEndian.PutUint32(rec[8:], uint32(f.firstEdge))
			binary.LittleEndian.PutUint32(rec[12:], uint32(int16(f.numEdges)))
			binary.LittleEndian.PutUint32(rec[16:], uint32(f.texinfo))
			copy(rec[20:24], f.styles[:])
			binary.LittleEndian.PutUint32(rec[24:], uint32(f.lightOfs))
			b.Write(rec[:])
		} else {
			var rec [20]byte
			binary.LittleEndian.PutUint16(rec[0:], uint16(planenum))
			binary.LittleEndian.PutUint16(rec[2:], uint16(uint8(f.side)))
			binary.LittleEndian.PutUint32(rec[4:], uint32(f.firstEdge))
			binary.LittleEndian.PutUint16(rec[8:], uint16(int16(f.numEdges)))
			binary.LittleEndian.PutUint16(rec[10:], uint16(f.texinfo))
			copy(rec[12:16], f.styles[:])
			binary.LittleEndian.PutUint32(rec[16:], uint32(f.lightOfs))
			b.Write(rec[:])
		}
	}
	return b.Bytes()
}

// ---- edges / surfedges / vertexes ----

func serializeEdges(edges [][2]int32, bsp2 bool) []byte {
	var b bytes.Buffer
	for _, e := range edges {
		if bsp2 {
			var rec [8]byte
			binary.LittleEndian.PutUint32(rec[0:], uint32(e[0]))
			binary.LittleEndian.PutUint32(rec[4:], uint32(e[1]))
			b.Write(rec[:])
		} else {
			var rec [4]byte
			binary.LittleEndian.PutUint16(rec[0:], uint16(e[0]))
			binary.LittleEndian.PutUint16(rec[2:], uint16(e[1]))
			b.Write(rec[:])
		}
	}
	return b.Bytes()
}

func serializeInts32(v []int32) []byte {
	var b bytes.Buffer
	for _, x := range v {
		var rec [4]byte
		binary.LittleEndian.PutUint32(rec[:], uint32(x))
		b.Write(rec[:])
	}
	return b.Bytes()
}

func vertexBytes(vertexes []vec3) []byte {
	var b bytes.Buffer
	for _, v := range vertexes {
		var rec [12]byte
		binary.LittleEndian.PutUint32(rec[0:], math.Float32bits(float32(v[0])))
		binary.LittleEndian.PutUint32(rec[4:], math.Float32bits(float32(v[1])))
		binary.LittleEndian.PutUint32(rec[8:], math.Float32bits(float32(v[2])))
		b.Write(rec[:])
	}
	return b.Bytes()
}

// ---- models ----

func serializeModels(models []modelOut, bsp2 bool) []byte {
	// DModel layout: mins f32[3] @0, maxs @12, origin @24, headnode[4] @36,
	// visleafs @52, firstface @56, numfaces @60 (64 bytes).
	var b bytes.Buffer
	for _, mo := range models {
		var rec [64]byte
		for i := 0; i < 3; i++ {
			binary.LittleEndian.PutUint32(rec[i*4:], math.Float32bits(float32(mo.mins[i])))
			binary.LittleEndian.PutUint32(rec[12+i*4:], math.Float32bits(float32(mo.maxs[i])))
			binary.LittleEndian.PutUint32(rec[24+i*4:], math.Float32bits(float32(mo.origin[i])))
		}
		head := int32(0)
		if !mo.root.isLeaf {
			head = int32(mo.root.idx)
		}
		binary.LittleEndian.PutUint32(rec[36:], uint32(head))        // headnode[0]
		binary.LittleEndian.PutUint32(rec[40:], uint32(mo.clipRoot)) // headnode[1]
		binary.LittleEndian.PutUint32(rec[44:], 0)
		binary.LittleEndian.PutUint32(rec[48:], 0)
		binary.LittleEndian.PutUint32(rec[52:], uint32(mo.visLeafs))
		binary.LittleEndian.PutUint32(rec[56:], uint32(mo.firstFace))
		binary.LittleEndian.PutUint32(rec[60:], uint32(mo.numFaces))
		b.Write(rec[:])
	}
	return b.Bytes()
}

// writeBSP assembles the header + lumps. Header: int32 version followed by
// 15 (offset, length) pairs — 4 + 15*8 = 124 bytes.
func writeBSP(lumps [][]byte, lay bspLayout) ([]byte, error) {
	if len(lumps) != 15 {
		return nil, fmt.Errorf("expected 15 lumps, got %d", len(lumps))
	}
	const headerSize = 4 + 15*8
	var b bytes.Buffer
	var header [headerSize]byte
	version := int32(bsp.BSPVersion)
	switch lay {
	case layoutBsp2:
		version = bsp.BSP2Version_BSP2
	case layoutBspRMQ:
		version = bsp.BSP2Version_2PSB
	}
	binary.LittleEndian.PutUint32(header[0:], uint32(version))

	offset := uint32(headerSize)
	for i, lump := range lumps {
		binary.LittleEndian.PutUint32(header[4+i*8:], offset)
		binary.LittleEndian.PutUint32(header[8+i*8:], uint32(len(lump)))
		offset += uint32(len(lump))
	}
	b.Write(header[:])
	for _, lump := range lumps {
		b.Write(lump)
	}
	return b.Bytes(), nil
}

func ReadBSPLumps(r io.ReaderAt) (int32, [][]byte, error) {
	var version int32
	if err := binary.Read(io.NewSectionReader(r, 0, 4), binary.LittleEndian, &version); err != nil {
		return 0, nil, err
	}
	var entries [15][8]byte
	raw := make([]byte, 15*8)
	if _, err := io.ReadFull(io.NewSectionReader(r, 4, 15*8), raw); err != nil {
		return 0, nil, err
	}
	for i := range entries {
		copy(entries[i][:], raw[i*8:])
	}
	lumps := make([][]byte, 15)
	for i := range lumps {
		ofs := int32(binary.LittleEndian.Uint32(entries[i][0:]))
		ln := int32(binary.LittleEndian.Uint32(entries[i][4:]))
		if ln <= 0 {
			continue
		}
		buf := make([]byte, ln)
		if _, err := r.ReadAt(buf, int64(ofs)); err != nil {
			return 0, nil, err
		}
		lumps[i] = buf
	}
	return version, lumps, nil
}

// WriteBSP assembles one file image from raw lumps with the given version.
func WriteBSP(lumps [][]byte, version int32) ([]byte, error) {
	if len(lumps) != 15 {
		return nil, fmt.Errorf("expected 15 lumps, got %d", len(lumps))
	}
	const headerSize = 4 + 15*8
	var b bytes.Buffer
	var header [headerSize]byte
	binary.LittleEndian.PutUint32(header[0:], uint32(version))
	offset := uint32(headerSize)
	for i, lump := range lumps {
		binary.LittleEndian.PutUint32(header[4+i*8:], offset)
		binary.LittleEndian.PutUint32(header[8+i*8:], uint32(len(lump)))
		offset += uint32(len(lump))
	}
	b.Write(header[:])
	for _, lump := range lumps {
		b.Write(lump)
	}
	return b.Bytes(), nil
}
