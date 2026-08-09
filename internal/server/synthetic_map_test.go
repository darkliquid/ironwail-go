package server

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/fs"
)

func TestBuildSyntheticMapProducesValidBSP(t *testing.T) {
	tree, file, err := BuildSyntheticMap()
	if err != nil {
		t.Fatalf("BuildSyntheticMap: %v", err)
	}
	if tree == nil || file == nil {
		t.Fatal("expected non-nil tree and file")
	}

	// World model exists with faces.
	if len(tree.Models) != 1 {
		t.Fatalf("expected 1 world model, got %d", len(tree.Models))
	}
	world := tree.Models[0]
	if world.NumFaces != 6 {
		t.Fatalf("expected 6 faces, got %d", world.NumFaces)
	}
	if len(tree.Faces) != 6 {
		t.Fatalf("expected 6 tree faces, got %d", len(tree.Faces))
	}

	// Nodes/leaves for leaf queries + collision.
	if len(tree.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(tree.Nodes))
	}
	if len(tree.Leafs) != 4 {
		t.Fatalf("expected 4 leaves, got %d", len(tree.Leafs))
	}

	// Interior point is empty, outside is solid.
	inside := tree.PointInLeaf([3]float32{0, 0, 48})
	if inside == nil || inside.Contents != bsp.ContentsEmpty {
		t.Fatalf("interior point should be empty, got %v", inside)
	}
	outside := tree.PointInLeaf([3]float32{0, 0, 400})
	if outside == nil || outside.Contents != bsp.ContentsSolid {
		t.Fatalf("point above ceiling should be solid, got %v", outside)
	}
	below := tree.PointInLeaf([3]float32{0, 0, -100})
	if below == nil || below.Contents != bsp.ContentsSolid {
		t.Fatalf("point below floor should be solid, got %v", below)
	}

	// Vertex/edge integrity for the renderer.
	if len(tree.Vertexes) == 0 {
		t.Fatal("no vertexes")
	}
	if len(tree.Edges) != len(tree.Surfedges) {
		t.Fatalf("edge count %d != surfedge count %d", len(tree.Edges), len(tree.Surfedges))
	}
	for si, se := range tree.Surfedges {
		if int(se) < 0 || int(se) >= len(tree.Edges) {
			t.Fatalf("surfedge %d out of range: %d", si, se)
		}
	}

	// Entity lump parses with the player start.
	ents := string(tree.Entities)
	if len(ents) == 0 || len(ents) > 4096 {
		t.Fatalf("entities lump looks wrong: %d bytes", len(ents))
	}

	// The collision file has clipnodes for hulls 1-2.
	clipNodes, ok := file.Clipnodes.([]bsp.DSClipNode)
	if !ok || len(clipNodes) != 2 {
		t.Fatalf("expected 2 DSClipNode clipnodes, got %T %d", file.Clipnodes, len(clipNodes))
	}
}

func TestSpawnServerFallsBackToSyntheticMap(t *testing.T) {
	// A filesystem with NO map data should trigger the synthetic fallback
	// instead of failing SpawnServer — but only for the synthetic demo map
	// name (the auto-start path). Arbitrary missing map names must fail.
	vfs := fs.NewFileSystem()
	cases := []string{SyntheticMapName}
	for _, mapName := range cases {
		srv := newSyntheticTestServer(t)
		err := srv.SpawnServer(mapName, vfs)
		if err != nil {
			t.Fatalf("SpawnServer(%q) with no map data: %v", mapName, err)
		}
		if !srv.SyntheticMap {
			t.Fatalf("SpawnServer(%q) should set SyntheticMap=true", mapName)
		}
		if srv.WorldTree == nil || srv.WorldModel == nil {
			t.Fatalf("SpawnServer(%q) left nil world", mapName)
		}
		// World model collision must be populated for movement.
		if cm, ok := srv.WorldModel.(interface{ ModelHull0() any }); ok && cm.ModelHull0() == nil {
			t.Fatalf("world hull 0 not populated")
		}
	}

	// A named-but-missing map (savegame reference / typo) must fail loudly.
	for _, badMap := range []string{"missingmap", "zzz_nonexistent"} {
		srv := newSyntheticTestServer(t)
		if err := srv.SpawnServer(badMap, vfs); err == nil {
			t.Fatalf("SpawnServer(%q) should fail when map is missing", badMap)
		}
	}
}

// newSyntheticTestServer builds a minimal Server wired like the physics tests
// (no QC). It is sufficient to exercise SpawnServer's map-load path.
func newSyntheticTestServer(t *testing.T) *Server {
	t.Helper()
	s := &Server{
		Gravity:      800,
		MaxVelocity:  2000,
		FrameTime:    0.1,
		CVar:         cvar.NewCVarSystem(),
		QCFieldAlpha: -1,
		QCFieldScale: -1,
		Static:       &ServerStatic{MaxClients: 1},
	}
	s.Edicts = make([]*Edict, 64)
	for i := range s.Edicts {
		e := &Edict{Num: i}
		e.Free = true
		s.Edicts[i] = e
	}
	s.Edicts[0].Free = false
	s.NumEdicts = 1
	s.MaxEdicts = 64
	s.ModelPrecache = make([]string, MaxModels)
	return s
}
