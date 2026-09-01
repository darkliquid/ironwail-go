package dap

import "github.com/darkliquid/ironwail-go/internal/qc"

// Target represents an engine or simulation target being debugged.
type Target interface {
	VM() *qc.VM
	EdictCount() int
	GetEdictFloat(entNum, offset int) float32
	GetEdictString(entNum, offset int) string
	GetEdictVector(entNum, offset int) [3]float32
	GetEdictClassName(entNum int) string
	FieldNames() map[string]int
}
