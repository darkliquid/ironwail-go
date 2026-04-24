// Command uvdump inspects an alias model inside a Quake PAK file and prints
// its skin dimensions, vertex/triangle counts, and unique UV coordinates for
// debugging texture mapping issues.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"sort"

	"github.com/darkliquid/ironwail-go/internal/model"
)

func main() {
pak, _ := os.ReadFile("/home/darkliquid/Games/Heroic/Quake Enhanced/id1/pak0.pak")
dirofs := binary.LittleEndian.Uint32(pak[4:])
dirlen := binary.LittleEndian.Uint32(pak[8:])
target := os.Args[1]
for i := uint32(0); i < dirlen/64; i++ {
e := pak[dirofs+i*64:]
name := string(bytes.TrimRight(e[:56], "\x00"))
if name != target {
continue
}
ofs := binary.LittleEndian.Uint32(e[56:])
ln := binary.LittleEndian.Uint32(e[60:])
m, err := model.LoadAliasModel(bytes.NewReader(pak[ofs : ofs+ln]))
if err != nil {
fmt.Println("load err:", err)
return
}
h := m.AliasHeader
fmt.Printf("skin=%dx%d verts=%d tris=%d poses=%d\n", h.SkinWidth, h.SkinHeight, h.NumVerts, h.NumTris, h.NumPoses)
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
fmt.Printf("unique UVs emitted: %d\n", len(uvSet))
		// Per-FacesFront statistics
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
			fmt.Printf("FacesFront=%d: tris=%d  leftHalf=%d  rightHalf=%d\n", ff, s.tris, s.leftHalf, s.rightHalf)
		}
type kv struct {
u, v  float32
count int
}
var all []kv
for k, c := range uvSet {
all = append(all, kv{k.u, k.v, c})
}
// Count UVs that are exactly on edge (u>=1 or v>=1)
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
fmt.Printf("UVs on/over [0,1] edge: %d  strict overflow: %d\n", edge, overflow)
// Print top distribution buckets
sort.Slice(all, func(i, j int) bool { return all[i].u < all[j].u })
fmt.Println("First 10 UVs (sorted by u):")
for i := 0; i < 10 && i < len(all); i++ {
fmt.Printf("  u=%.4f v=%.4f  cnt=%d\n", all[i].u, all[i].v, all[i].count)
}
fmt.Println("Last 10 UVs (sorted by u):")
for i := len(all) - 10; i < len(all); i++ {
if i < 0 {
continue
}
fmt.Printf("  u=%.4f v=%.4f  cnt=%d\n", all[i].u, all[i].v, all[i].count)
}
// Distribution of u*skinwidth -0.5 (rounded): should correspond to raw S or S+skinwidth/2
// Histogram of UV "s-pixel"
hist := map[int]int{}
for _, k := range all {
spx := int(k.u*float32(h.SkinWidth) - 0.5)
hist[spx/16]++ // bucket by 16 px
}
var buckets []int
for b := range hist {
buckets = append(buckets, b)
}
sort.Ints(buckets)
fmt.Println("S-pixel distribution (16px buckets):")
for _, b := range buckets {
fmt.Printf("  [%d..%d]: %d\n", b*16, (b+1)*16-1, hist[b])
}
return
}
}
