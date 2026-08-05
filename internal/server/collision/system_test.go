package collision

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

type mockWorld struct {
	model *model.Model
	tree  *bsp.Tree
}

func (m *mockWorld) GetWorldModel() srvtypes.CollisionModel {
	return m.model
}

func (m *mockWorld) GetWorldTree() *bsp.Tree {
	return m.tree
}

type mockTouch struct{}

func (m *mockTouch) TouchLinks(ent *srvtypes.Edict) {}

type mockStore struct {
	edicts []*srvtypes.Edict
}

func (m *mockStore) EdictNum(num int) *srvtypes.Edict {
	if num < 0 || num >= len(m.edicts) {
		return nil
	}
	return m.edicts[num]
}

func (m *mockStore) AllocEdict() *srvtypes.Edict {
	ed := &srvtypes.Edict{Num: len(m.edicts)}
	m.edicts = append(m.edicts, ed)
	return ed
}

func (m *mockStore) FreeEdict(ed *srvtypes.Edict) {
	if ed != nil {
		ed.Free = true
	}
}

func TestCollisionSystem_ClearWorld(t *testing.T) {
	world := &mockWorld{}
	store := &mockStore{edicts: []*srvtypes.Edict{{Num: 0}}}
	touch := &mockTouch{}

	sys := NewSystem(world, store, touch, nil)
	sys.ClearWorld()

	if got := len(sys.Areanodes()); got != AreaNodes {
		t.Fatalf("Areanodes len = %d, want %d", got, AreaNodes)
	}

	if sys.NumAreaNodes() == 0 {
		t.Fatal("NumAreaNodes = 0 after ClearWorld")
	}
}

func TestCollisionSystem_PointContents(t *testing.T) {
	world := &mockWorld{}
	store := &mockStore{edicts: []*srvtypes.Edict{{Num: 0}}}
	touch := &mockTouch{}

	sys := NewSystem(world, store, touch, nil)
	cont := sys.PointContents([3]float32{0, 0, 0})
	if cont != bsp.ContentsSolid {
		t.Fatalf("PointContents without world model = %d, want ContentsSolid (%d)", cont, bsp.ContentsSolid)
	}
}
