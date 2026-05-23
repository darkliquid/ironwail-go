// Command uvdump inspects an alias model inside a Quake PAK file and prints
// its skin dimensions, vertex/triangle counts, and unique UV coordinates for
// debugging texture mapping issues.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/darkliquid/ironwail-go/internal/model"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	pakPath := ""
	target := ""
	switch len(args) {
	case 0:
		_, _ = fmt.Fprintln(stderr, "usage: uvdump <pak-file> <model-path>")
		_, _ = fmt.Fprintln(stderr, "       QUAKE_PAKPATH=/path/to/pak0.pak uvdump <model-path>")
		return 2
	case 1:
		pakPath = os.Getenv("QUAKE_PAKPATH")
		target = args[0]
	default:
		pakPath = args[0]
		target = args[1]
	}

	if pakPath == "" {
		_, _ = fmt.Fprintln(stderr, "uvdump: pak file path is required; pass it as the first argument or set QUAKE_PAKPATH")
		return 2
	}

	pak, err := os.ReadFile(pakPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "uvdump: read %s: %v\n", pakPath, err)
		return 1
	}
	if len(pak) < 12 || string(pak[:4]) != "PACK" {
		_, _ = fmt.Fprintf(stderr, "uvdump: %s is not a Quake PAK file\n", pakPath)
		return 1
	}

	dirofs := binary.LittleEndian.Uint32(pak[4:])
	dirlen := binary.LittleEndian.Uint32(pak[8:])
	if uint64(dirofs)+uint64(dirlen) > uint64(len(pak)) || dirlen%64 != 0 {
		_, _ = fmt.Fprintf(stderr, "uvdump: %s has invalid PAK directory\n", pakPath)
		return 1
	}

	for i := uint32(0); i < dirlen/64; i++ {
		e := pak[dirofs+i*64:]
		name := string(bytes.TrimRight(e[:56], "\x00"))
		if name != target {
			continue
		}
		ofs := binary.LittleEndian.Uint32(e[56:])
		ln := binary.LittleEndian.Uint32(e[60:])
		if uint64(ofs)+uint64(ln) > uint64(len(pak)) {
			_, _ = fmt.Fprintf(stderr, "uvdump: %s entry %s is out of bounds\n", pakPath, target)
			return 1
		}
		return dumpAliasUV(stdout, pak[ofs:ofs+ln])
	}

	_, _ = fmt.Fprintf(stderr, "uvdump: %s not found in %s\n", target, pakPath)
	return 1
}

func dumpAliasUV(stdout io.Writer, data []byte) int {
	m, err := model.LoadAliasModel(bytes.NewReader(data))
	if err != nil {
		_, _ = fmt.Fprintf(stdout, "load err: %v\n", err)
		return 1
	}
	h := m.AliasHeader
	_, _ = fmt.Fprintf(stdout, "skin=%dx%d verts=%d tris=%d poses=%d\n", h.SkinWidth, h.SkinHeight, h.NumVerts, h.NumTris, h.NumPoses)
	type uv struct{ u, v float32 }
	uvSet := map[uv]int{}
	for _, tri := range h.Triangles {
		for _, vi := range tri.VertIndex {
			st := h.STVerts[vi]
			s := float32(st.S) + 0.5
			if tri.FacesFront == 0 && st.OnSeam != 0 {
				s += float32(h.SkinWidth) * 0.5
			}
			u := s / float32(h.SkinWidth)
			v := (float32(st.T) + 0.5) / float32(h.SkinHeight)
			uvSet[uv{u, v}]++
		}
	}
	_, _ = fmt.Fprintf(stdout, "unique UVs emitted: %d\n", len(uvSet))

	ffStats := map[int32]struct{ tris, rightHalf, leftHalf int }{}
	for _, tri := range h.Triangles {
		st0 := h.STVerts[tri.VertIndex[0]]
		s := float32(st0.S) + 0.5
		if tri.FacesFront == 0 && st0.OnSeam != 0 {
			s += float32(h.SkinWidth) * 0.5
		}
		v := ffStats[tri.FacesFront]
		v.tris++
		if s > float32(h.SkinWidth)/2 {
			v.rightHalf++
		} else {
			v.leftHalf++
		}
		ffStats[tri.FacesFront] = v
	}
	for ff, s := range ffStats {
		_, _ = fmt.Fprintf(stdout, "FacesFront=%d: tris=%d  leftHalf=%d  rightHalf=%d\n", ff, s.tris, s.leftHalf, s.rightHalf)
	}

	type kv struct {
		u, v  float32
		count int
	}
	var all []kv
	for k, c := range uvSet {
		all = append(all, kv{k.u, k.v, c})
	}

	edge := 0
	overflow := 0
	for _, k := range all {
		if k.u >= 1.0 || k.v >= 1.0 {
			edge++
		}
		if k.u > 1.0 || k.v > 1.0 {
			overflow++
		}
	}
	_, _ = fmt.Fprintf(stdout, "UVs on/over [0,1] edge: %d  strict overflow: %d\n", edge, overflow)

	sort.Slice(all, func(i, j int) bool { return all[i].u < all[j].u })
	_, _ = fmt.Fprintln(stdout, "First 10 UVs (sorted by u):")
	for i := 0; i < 10 && i < len(all); i++ {
		_, _ = fmt.Fprintf(stdout, "  u=%.4f v=%.4f  cnt=%d\n", all[i].u, all[i].v, all[i].count)
	}
	_, _ = fmt.Fprintln(stdout, "Last 10 UVs (sorted by u):")
	for i := len(all) - 10; i < len(all); i++ {
		if i < 0 {
			continue
		}
		_, _ = fmt.Fprintf(stdout, "  u=%.4f v=%.4f  cnt=%d\n", all[i].u, all[i].v, all[i].count)
	}

	hist := map[int]int{}
	for _, k := range all {
		spx := int(k.u*float32(h.SkinWidth) - 0.5)
		hist[spx/16]++
	}
	var buckets []int
	for b := range hist {
		buckets = append(buckets, b)
	}
	sort.Ints(buckets)
	_, _ = fmt.Fprintln(stdout, "S-pixel distribution (16px buckets):")
	for _, b := range buckets {
		_, _ = fmt.Fprintf(stdout, "  [%d..%d]: %d\n", b*16, (b+1)*16-1, hist[b])
	}
	return 0
}
