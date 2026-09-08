package dap

import (
	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

// Target represents an engine or simulation target being debugged.
type Target interface {
	VM() *qc.VM
	EdictCount() int
	GetEdictFloat(entNum, offset int) float32
	GetEdictString(entNum, offset int) string
	GetEdictVector(entNum, offset int) types.Vec3
	GetEdictClassName(entNum int) string
	FieldNames() map[string]int
}
