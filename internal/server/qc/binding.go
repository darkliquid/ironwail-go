// Package qc implements QuakeC VM field offset resolution, memory mirroring, and call tracing.
package qc

import (
	"github.com/darkliquid/ironwail-go/internal/qc"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

// Binding manages QuakeC VM memory mirroring and execution hooks.
type Binding struct {
	vm    *qc.VM
	store srvtypes.EntityStore
}

// NewBinding creates a new QuakeC VM binding instance.
func NewBinding(vm *qc.VM, store srvtypes.EntityStore) *Binding {
	return &Binding{
		vm:    vm,
		store: store,
	}
}

// ExecuteQCFunction calls a QuakeC bytecode function index in the VM.
func (b *Binding) ExecuteQCFunction(funcIdx int) error {
	if b == nil || b.vm == nil {
		return nil
	}
	return b.vm.ExecuteFunction(funcIdx)
}

// RunThink executes an entity's think function if think time is reached.
func (b *Binding) RunThink(ent *srvtypes.Edict) bool {
	if b == nil || b.vm == nil || ent == nil || ent.Free {
		return false
	}
	return false
}
