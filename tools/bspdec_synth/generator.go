// Command bspdec_synth generates deterministic synthetic Quake maps on an
// 8-unit lattice (spec section 9.6 P6): room-grammar worldspawn solids that
// compile sealed with the pinned qbsp, each carrying its BRUSHLIST oracle.
package synth // import "github.com/darkliquid/ironwail-go/tools/bspdec_synth"

import (
	"fmt"
	"io"
	"math"
	"math/rand"

	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// Mapper-convention lattice and slab sizes (all multiples of grid).
const (
	grid        = 8  // master lattice
	blockoutMin = 64 // room walls/floors
	blockoutMax = 32 // thick blockout slabs
	detailK     = 16 // detail walls (pillars)
	detailS     = 8  // thin detail
	trimK       = 8  // baseboard thickness
	trimS       = 8  // baseboard height (4 would leave the 8-unit lattice)
	stairTread  = 16 // stair tread depth
	stairRise   = 16 // stair rise per step
)

// Generator produces the n-th map of a seed's deterministic sequence. All
// coordinates are multiples of grid, so emitted maps recompile identically.
type Generator struct {
	r     *rand.Rand
	seed  int64
	count int
	grid  int
}

// NewGenerator seeds a deterministic sequence of `count` maps.
func NewGenerator(seed int64, count int) *Generator {
	return &Generator{r: rand.New(rand.NewSource(seed)), seed: seed, count: count, grid: grid}
}

// snapGrid rounds to the nearest lattice multiple.
func snapGrid(v float64, g int) float64 {
	s := float64(g)
	return math.Round(v/s) * s
}

// randBetween returns a lattice multiple in [lo, hi] (both multiples of grid).
func (g *Generator) randBetween(lo, hi float64) float64 {
	steps := int((hi - lo) / grid)
	if steps < 1 {
		return lo
	}
	return lo + grid*float64(g.r.Intn(steps+1))
}

// room is a hollow interior plus its shell thickness.
type room struct {
	x0, y0, z0, x1, y1, z1, wallT float64 // hollow interior; shell thickness wallT
}

func (r room) depth() float64 { return r.y1 - r.y0 }

// GenMap builds map n of the sequence (map id synth-<seed>-<n>): a sealed
// row of 2-3 rooms connected by door-framed corridors, with mapper-convention
// details (stairs, trims, pillars, a water pool). Worldspawn carries all
// geometry; info_player_start sits in the first room's interior.
func (g *Generator) GenMap(n int) *mapfile.Map {
	r := rand.New(rand.NewSource(g.seed*1000003 + int64(n)))
	gr := &Generator{r: r, seed: g.seed, count: g.count, grid: g.grid}

	ws := mapfile.Entity{Epairs: []mapfile.Epair{
		{Key: "classname", Value: "worldspawn"},
		{Key: "wad", Value: ""},
		{Key: "_name", Value: fmt.Sprintf("synth-%d-%d", g.seed, n)},
	}}

	// Rooms share a row: same y0/z0 so corridors run straight along x.
	y0 := gr.randBetween(0, 256)
	z0 := gr.randBetween(0, 128)
	nrooms := 2 + r.Intn(2)
	x := gr.randBetween(0, 128)
	var rooms []room
	for i := 0; i < nrooms; i++ {
		rm := gr.randomRoom(x, y0, z0)
		rooms = append(rooms, rm)
		x = rm.x1 + rm.wallT + gr.randBetween(64, 192)
	}
	for i, rm := range rooms {
		ws.Brushes = append(ws.Brushes, gr.shellFor(rm, i > 0, i+1 < nrooms)...)
	}
	for i := 0; i+1 < len(rooms); i++ {
		ws.Brushes = append(ws.Brushes, gr.connectRooms(rooms[i], rooms[i+1])...)
	}
	if nrooms >= 2 {
		ws.Brushes = append(ws.Brushes, gr.placeStairs(rooms[1])...)
		ws.Brushes = append(ws.Brushes, gr.placeTrims(rooms[1])...)
		ws.Brushes = append(ws.Brushes, gr.placeDetails(rooms[1])...)
	}
	if nrooms >= 3 {
		ws.Brushes = append(ws.Brushes, gr.placeLiquids(rooms[2])...)
	}

	m := &mapfile.Map{Entities: []mapfile.Entity{
		ws,
		{
			Epairs: []mapfile.Epair{
				{Key: "classname", Value: "info_player_start"},
				{Key: "origin", Value: gr.playerOrigin(rooms[0])},
			},
		},
	}}
	return m
}

func (g *Generator) playerOrigin(r0 room) string {
	cx := r0.x0 + (r0.x1-r0.x0)/2
	cy := r0.y0 + (r0.y1-r0.y0)/2
	cz := r0.z0 + 32
	return fmt.Sprintf("%d %d %d", int(cx), int(cy), int(cz))
}

// randomRoom picks interior spans on the lattice: widths/depths multiples of
// 32 (>= 16), height multiples of 32, shell thickness 32 or 64.
func (g *Generator) randomRoom(x, y0, z0 float64) room {
	width := 96 + 32*float64(g.r.Intn(3))
	depth := 96 + 32*float64(g.r.Intn(3))
	height := 64 + 32*float64(g.r.Intn(3))
	wt := 32.0
	if g.r.Intn(2) == 0 {
		wt = blockoutMin
	}
	return room{
		x0: x, y0: y0, z0: z0,
		x1: x + width, y1: y0 + depth, z1: z0 + height,
		wallT: wt,
	}
}

// room builds one axis-aligned box brush with outward faces (the slabBrush
// winding convention from internal/qbsp/mapfile_test.go, compile-proven).
func (g *Generator) room(x0, y0, z0, x1, y1, z1 float64, tex string) mapfile.MapBrush {
	mins := mapfile.Vec3{X: x0, Y: y0, Z: z0}
	maxs := mapfile.Vec3{X: x1, Y: y1, Z: z1}
	faces := [][3]mapfile.Vec3{
		{vc(maxs.X, mins.Y, mins.Z), vc(maxs.X, mins.Y, maxs.Z), vc(maxs.X, maxs.Y, mins.Z)}, // +x
		{vc(mins.X, maxs.Y, mins.Z), vc(mins.X, maxs.Y, maxs.Z), vc(mins.X, mins.Y, maxs.Z)}, // -x
		{vc(mins.X, maxs.Y, mins.Z), vc(maxs.X, maxs.Y, mins.Z), vc(mins.X, maxs.Y, maxs.Z)}, // +y
		{vc(mins.X, mins.Y, mins.Z), vc(mins.X, mins.Y, maxs.Z), vc(maxs.X, mins.Y, mins.Z)}, // -y
		{vc(mins.X, mins.Y, maxs.Z), vc(mins.X, maxs.Y, maxs.Z), vc(maxs.X, mins.Y, maxs.Z)}, // +z
		{vc(mins.X, mins.Y, mins.Z), vc(maxs.X, mins.Y, mins.Z), vc(mins.X, maxs.Y, mins.Z)}, // -z
	}
	br := mapfile.MapBrush{}
	for _, pts := range faces {
		f := mapfile.MapFace{Points: pts, TexName: tex}
		if p, length := mapfile.PlaneFromPoints(pts[0], pts[1], pts[2]); length > 0.01 {
			f.Normal, f.Dist = p.Normal, p.Dist
		}
		br.Faces = append(br.Faces, f)
	}
	return br
}

func vc(x, y, z float64) mapfile.Vec3 { return mapfile.Vec3{X: x, Y: y, Z: z} }

// shellFor builds a room's slabs: floor/ceiling/north/south always; the west
// and east walls are omitted where a door frame (connectRooms) takes over.
func (g *Generator) shellFor(rm room, openWest, openEast bool) []mapfile.MapBrush {
	t := rm.wallT
	brushes := []mapfile.MapBrush{
		g.room(rm.x0-t, rm.y0-t, rm.z0-t, rm.x1+t, rm.y1+t, rm.z0, "mt_floor"), // floor
		g.room(rm.x0-t, rm.y0-t, rm.z1, rm.x1+t, rm.y1+t, rm.z1+t, "mt_floor"), // ceiling
		g.room(rm.x0, rm.y0-t, rm.z0-t, rm.x1, rm.y0, rm.z1+t, "mt_rock"),      // north
		g.room(rm.x0, rm.y1, rm.z0-t, rm.x1, rm.y1+t, rm.z1+t, "mt_rock"),      // south
	}
	if !openWest {
		brushes = append(brushes, g.room(rm.x0-t, rm.y0-t, rm.z0-t, rm.x0, rm.y1+t, rm.z1+t, "mt_rock"))
	}
	if !openEast {
		brushes = append(brushes, g.room(rm.x1, rm.y0-t, rm.z0-t, rm.x1+t, rm.y1+t, rm.z1+t, "mt_rock"))
	}
	return brushes
}

// connectRooms seals the gap between two row neighbours with a straight
// corridor tube plus door-framed facing walls: the door opening spans the
// tube's inner width (min depth minus both tube walls) and the shared
// ceiling height, with lower/upper wall beams and a side stub around it.
func (g *Generator) connectRooms(a, b room) []mapfile.MapBrush {
	const wt = float64(blockoutMax) // corridor shell thickness
	z1c := math.Min(a.z1, b.z1)
	dmin := math.Min(a.depth(), b.depth())

	// door opening spans the tube's inner width (min depth minus both tube
	// side walls) at the shared ceiling height
	doorY1 := a.y0 + dmin - wt

	// door-framed east wall of a
	east := []mapfile.MapBrush{
		g.room(a.x1, a.y0, a.z0-a.wallT, a.x1+a.wallT, a.y1, a.z0, "mt_rock"), // lower beam
		g.room(a.x1, a.y0, z1c, a.x1+a.wallT, a.y1, a.z1+a.wallT, "mt_rock"),  // upper beam
		g.room(a.x1, doorY1, a.z0, a.x1+a.wallT, a.y1, z1c, "mt_rock"),        // side stub
	}
	// door-framed west wall of b
	west := []mapfile.MapBrush{
		g.room(b.x0-b.wallT, b.y0, b.z0-b.wallT, b.x0, b.y1, b.z0, "mt_rock"),
		g.room(b.x0-b.wallT, b.y0, z1c, b.x0, b.y1, b.z1+b.wallT, "mt_rock"),
		g.room(b.x0-b.wallT, doorY1, b.z0, b.x0, b.y1, z1c, "mt_rock"),
	}

	// corridor tube between the two inner wall faces
	tube := []mapfile.MapBrush{
		g.room(a.x1, a.y0, a.z0-wt, b.x0, a.y0+dmin, a.z0, "mt_floor"),          // floor
		g.room(a.x1, a.y0, z1c, b.x0, a.y0+dmin, z1c+wt, "mt_floor"),            // ceiling
		g.room(a.x1, a.y0, a.z0-wt, b.x0, a.y0+wt, z1c+wt, "mt_rock"),           // south side
		g.room(a.x1, a.y0+dmin-wt, a.z0-wt, b.x0, a.y0+dmin, z1c+wt, "mt_rock"), // north side
	}
	out := make([]mapfile.MapBrush, 0, len(east)+len(west)+len(tube))
	out = append(out, east...)
	out = append(out, west...)
	out = append(out, tube...)
	return out
}

// placeStairs adds an ascending stair flight against a room wall (floor
// level rises by 16 per step, tread depth 16, width 32).
func (g *Generator) placeStairs(rm room) []mapfile.MapBrush {
	const n = 4
	const sx = 16.0 // step width
	x0 := rm.x0 + 8
	steps := make([]mapfile.MapBrush, 0, n)
	for k := 0; k < n; k++ {
		sf := float64(k)
		steps = append(steps, g.room(x0, rm.y0+8+sf*stairTread, rm.z0+sf*stairRise,
			x0+sx, rm.y0+8+(sf+1)*stairTread, rm.z0+(sf+1)*stairRise, "mt_rock"))
	}
	return steps
}

// placeTrims lays 8x8 baseboard strips along the inner floor edges, inset
// by a fixed pad so they never degenerate on small interiors.
func (g *Generator) placeTrims(rm room) []mapfile.MapBrush {
	const pad = 16.0
	return []mapfile.MapBrush{
		g.room(rm.x0+pad, rm.y0+pad, rm.z0, rm.x1-pad, rm.y0+pad+trimK, rm.z0+trimS, "mt_rock"), // south strip
		g.room(rm.x0+pad, rm.y1-pad-trimK, rm.z0, rm.x1-pad, rm.y1-pad, rm.z0+trimS, "mt_rock"), // north strip
		g.room(rm.x1-pad-trimK, rm.y0+pad, rm.z0, rm.x1-pad, rm.y1-pad, rm.z0+trimS, "mt_rock"), // east strip
	}
}

// placeDetails adds 16x16 pillars rising to the room ceiling, centered in
// the interior so they fit any generated room.
func (g *Generator) placeDetails(rm room) []mapfile.MapBrush {
	w := rm.x1 - rm.x0
	d := rm.y1 - rm.y0
	p := func(px, py float64) mapfile.MapBrush {
		return g.room(rm.x0+px, rm.y0+py, rm.z0, rm.x0+px+detailK, rm.y0+py+detailK, rm.z1, "mt_rock")
	}
	return []mapfile.MapBrush{
		p(w/2-8, 16), // mid-west pillar
		p(16, d/2-8), // mid-north pillar
	}
}

// placeLiquids adds a flat *water1 pool at a room's floor (content comes
// from the texture prefix, so the brush stays a closed solid box).
func (g *Generator) placeLiquids(rm room) []mapfile.MapBrush {
	return []mapfile.MapBrush{
		g.room(rm.x0+16, rm.y0+16, rm.z0, rm.x0+80, rm.y0+80, rm.z0+16, "*water1"),
	}
}

// Emit writes the map with the generator's lattice snap.
func (g *Generator) Emit(m *mapfile.Map, w io.Writer) error {
	return mapfile.Write(w, m, mapfile.WriteOptions{GridSnap: g.grid})
}
