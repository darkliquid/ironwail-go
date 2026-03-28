// Package compiler implements a Go-to-QCVM compiler.
package compiler

import "github.com/darkliquid/ironwail-go/internal/qc"

// opcodeForStore returns the STORE opcode for a given QC type.
func opcodeForStore(t qc.EType) qc.Opcode {
	switch t {
	case EvFloat:
		return qc.OPStoreF
	case EvVector:
		return qc.OPStoreV
	case EvString:
		return qc.OPStoreS
	case EvEntity:
		return qc.OPStoreEnt
	case EvField:
		return qc.OPStoreFld
	case EvFunction:
		return qc.OPStoreFNC
	default:
		return qc.OPStoreF
	}
}

// opcodeForLoad returns the LOAD opcode for a given QC type.
func opcodeForLoad(t qc.EType) qc.Opcode {
	switch t {
	case EvFloat:
		return qc.OPLoadF
	case EvVector:
		return qc.OPLoadV
	case EvString:
		return qc.OPLoadS
	case EvEntity:
		return qc.OPLoadEnt
	case EvField:
		return qc.OPLoadFld
	case EvFunction:
		return qc.OPLoadFNC
	default:
		return qc.OPLoadF
	}
}

// opcodeForStoreP returns the STOREP (store-to-pointer) opcode for a given QC type.
func opcodeForStoreP(t qc.EType) qc.Opcode {
	switch t {
	case EvFloat:
		return qc.OPStorePF
	case EvVector:
		return qc.OPStorePV
	case EvString:
		return qc.OPStorePS
	case EvEntity:
		return qc.OPStorePEnt
	case EvField:
		return qc.OPStorePFld
	case EvFunction:
		return qc.OPStorePFNC
	default:
		return qc.OPStorePF
	}
}

// opcodeForNot returns the NOT opcode for a given QC type.
func opcodeForNot(t qc.EType) qc.Opcode {
	switch t {
	case EvFloat:
		return qc.OPNotF
	case EvVector:
		return qc.OPNotV
	case EvString:
		return qc.OPNotS
	case EvEntity:
		return qc.OPNotEnt
	case EvFunction:
		return qc.OPNotFNC
	default:
		return qc.OPNotF
	}
}

// opcodeForEq returns the equality opcode for a given QC type.
func opcodeForEq(t qc.EType) qc.Opcode {
	switch t {
	case EvFloat:
		return qc.OPEqF
	case EvVector:
		return qc.OPEqV
	case EvString:
		return qc.OPEqS
	case EvEntity:
		return qc.OPEqE
	case EvFunction:
		return qc.OPEqFNC
	default:
		return qc.OPEqF
	}
}

// opcodeForNe returns the inequality opcode for a given QC type.
func opcodeForNe(t qc.EType) qc.Opcode {
	switch t {
	case EvFloat:
		return qc.OPNeF
	case EvVector:
		return qc.OPNeV
	case EvString:
		return qc.OPNeS
	case EvEntity:
		return qc.OPNeE
	case EvFunction:
		return qc.OPNeFNC
	default:
		return qc.OPNeF
	}
}

// QC type aliases used throughout the compiler to avoid importing qc everywhere.
const (
	EvVoid     = qc.EvVoid
	EvString   = qc.EvString
	EvFloat    = qc.EvFloat
	EvVector   = qc.EvVector
	EvEntity   = qc.EvEntity
	EvField    = qc.EvField
	EvFunction = qc.EvFunction
	EvPointer  = qc.EvPointer
)

// slotsForType returns the number of global slots a type occupies.
func slotsForType(t qc.EType) uint16 {
	if t == EvVector {
		return 3
	}
	return 1
}
