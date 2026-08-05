package qc

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/qc"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

type mockStore struct{}

func (m *mockStore) EdictNum(num int) *srvtypes.Edict { return nil }
func (m *mockStore) AllocEdict() *srvtypes.Edict      { return nil }
func (m *mockStore) FreeEdict(ed *srvtypes.Edict)    {}
func (m *mockStore) GetNumEdicts() int               { return 0 }
func (m *mockStore) GetMaxEdicts() int               { return 0 }

func TestNewBinding(t *testing.T) {
	vm := &qc.VM{}
	store := &mockStore{}
	b := NewBinding(vm, store)
	if b == nil {
		t.Fatal("expected non-nil Binding")
	}

	if b.RunThink(nil) {
		t.Fatal("expected RunThink(nil) to return false")
	}
}
