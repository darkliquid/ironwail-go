package game

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/host"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/server"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

func createSyntheticBSPTreeForFaceTrace() *bsp.Tree {
	var texBuf bytes.Buffer
	_ = binary.Write(&texBuf, binary.LittleEndian, uint32(1))
	_ = binary.Write(&texBuf, binary.LittleEndian, uint32(8))

	var name [16]byte
	copy(name[:], "wall01")
	texBuf.Write(name[:])
	_ = binary.Write(&texBuf, binary.LittleEndian, uint32(64))
	_ = binary.Write(&texBuf, binary.LittleEndian, uint32(64))
	_ = binary.Write(&texBuf, binary.LittleEndian, uint32(40))
	_ = binary.Write(&texBuf, binary.LittleEndian, uint32(40+64*64))
	_ = binary.Write(&texBuf, binary.LittleEndian, uint32(40+64*64+32*32))
	_ = binary.Write(&texBuf, binary.LittleEndian, uint32(40+64*64+32*32+16*16))
	texBuf.Write(make([]byte, 64*64+32*32+16*16+8*8))

	tree := &bsp.Tree{
		Planes: []bsp.DPlane{
			{Normal: types.Vec3{X: 1, Y: 0, Z: 0}, Dist: 100.0, Type: 0},
		},
		Vertexes: []bsp.DVertex{
			{Point: types.Vec3{X: 100, Y: -50, Z: -50}},
			{Point: types.Vec3{X: 100, Y: 50, Z: -50}},
			{Point: types.Vec3{X: 100, Y: 50, Z: 50}},
			{Point: types.Vec3{X: 100, Y: -50, Z: 50}},
		},
		Edges: []bsp.TreeEdge{
			{V: [2]uint32{0, 1}},
			{V: [2]uint32{1, 2}},
			{V: [2]uint32{2, 3}},
			{V: [2]uint32{3, 0}},
		},
		Surfedges: []int32{0, 1, 2, 3},
		Texinfo: []bsp.Texinfo{
			{
				Vecs: [2][4]float32{
					{0, 1, 0, 0},
					{0, 0, 1, 0},
				},
				Miptex: 0,
				Flags:  0,
			},
		},
		Faces: []bsp.TreeFace{
			{
				PlaneNum:  0,
				Side:      1,
				FirstEdge: 0,
				NumEdges:  4,
				Texinfo:   0,
				Styles:    [4]byte{0, 255, 255, 255},
				LightOfs:  0,
			},
		},
		Models: []bsp.DModel{
			{
				FirstFace: 0,
				NumFaces:  1,
			},
		},
		TextureData: texBuf.Bytes(),
	}
	return tree
}

func TestTraceCrosshairFace(t *testing.T) {
	g := New()
	s := &server.Server{
		WorldTree: createSyntheticBSPTreeForFaceTrace(),
	}
	g.Server = s

	t.Run("DirectHit", func(t *testing.T) {
		origin := types.Vec3{X: 0, Y: 0, Z: 0}
		forward := types.Vec3{X: 1, Y: 0, Z: 0}

		hit, ok := g.traceCrosshairFace(origin, forward)
		if !ok {
			t.Fatalf("expected face hit, got none")
		}

		if hit.faceIndex != 0 {
			t.Errorf("faceIndex = %d, want 0", hit.faceIndex)
		}
		if hit.distance != 100.0 {
			t.Errorf("distance = %f, want 100.0", hit.distance)
		}
		if hit.hitPos != (types.Vec3{X: 100, Y: 0, Z: 0}) {
			t.Errorf("hitPos = %v, want (100, 0, 0)", hit.hitPos)
		}
		if hit.texName != "wall01" {
			t.Errorf("texName = %q, want %q", hit.texName, "wall01")
		}
	})

	t.Run("MissRayPointingAway", func(t *testing.T) {
		origin := types.Vec3{X: 0, Y: 0, Z: 0}
		forward := types.Vec3{X: -1, Y: 0, Z: 0}

		_, ok := g.traceCrosshairFace(origin, forward)
		if ok {
			t.Fatalf("expected miss when pointing away from wall, got hit")
		}
	})

	t.Run("MissOutsidePolygon", func(t *testing.T) {
		origin := types.Vec3{X: 0, Y: 200, Z: 0}
		forward := types.Vec3{X: 1, Y: 0, Z: 0}

		_, ok := g.traceCrosshairFace(origin, forward)
		if ok {
			t.Fatalf("expected miss when ray is outside polygon bounds, got hit")
		}
	})
}

func TestCmdCamDebugRunsWithoutPanic(t *testing.T) {
	g := New()
	h := host.NewHost()
	if err := h.Init(&host.InitParams{BaseDir: t.TempDir(), UserDir: t.TempDir()}, &host.Subsystems{}); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	g.Host = h
	g.Client = &client.Client{
		ViewEntity: 1,
		Entities: map[int]inet.EntityState{
			1: {Origin: types.Vec3{X: 0, Y: 0, Z: 0}},
		},
	}
	g.Server = &server.Server{
		WorldTree: createSyntheticBSPTreeForFaceTrace(),
	}

	// Executing cmdCamDebug should dump info without panicking
	g.cmdCamDebug(nil)
}
