package compiler

import (
	"go/token"

	"github.com/ironwail/ironwail-go/internal/qc"
)

// CodeGen converts an IRProgram into QCVM statements, functions, and defs.
type CodeGen struct {
	globals    *GlobalAllocator
	strings    *StringTable
	statements []qc.DStatement
	functions  []qc.DFunction
	globalDefs []qc.DDef
	fieldDefs  []qc.DDef
	errors     ErrorList

	// Per-function state
	vregMap      map[VReg]uint16 // Virtual register -> global offset
	nextVReg     VReg
	localOfs     uint16 // Start of function locals
	localSize    uint16 // Number of local slots used
	labelTargets map[int]string // statement index -> label name for branch patching
}

// NewCodeGen creates a new code generator.
func NewCodeGen(globals *GlobalAllocator, strings *StringTable) *CodeGen {
	return &CodeGen{
		globals: globals,
		strings: strings,
	}
}

// Generate processes the IR program and produces QCVM output.
func (cg *CodeGen) Generate(prog *IRProgram) (*EmitInput, error) {
	// Sentinel statement 0 (error trap)
	cg.statements = append(cg.statements, qc.DStatement{Op: uint16(qc.OPDone)})

	// Sentinel function 0 (empty)
	cg.functions = append(cg.functions, qc.DFunction{})

	// Register globals from IR
	for i := range prog.Globals {
		g := &prog.Globals[i]
		slots := slotsForType(g.Type)
		g.Offset = cg.globals.AllocGlobal(g.Name, slots)

		switch g.Type {
		case EvFloat:
			cg.globals.SetFloat(g.Offset, g.InitFloat)
		case EvString:
			ofs := cg.strings.Intern(g.InitStr)
			cg.globals.SetInt(g.Offset, ofs)
		case EvVector:
			cg.globals.SetVector(g.Offset, g.InitVec)
		}

		cg.globalDefs = append(cg.globalDefs, qc.DDef{
			Type: uint16(g.Type),
			Ofs:  g.Offset,
			Name: cg.strings.Intern(g.Name),
		})
	}

	// Register field defs
	for _, f := range prog.Fields {
		cg.fieldDefs = append(cg.fieldDefs, qc.DDef{
			Type: uint16(f.Type),
			Ofs:  f.Offset,
			Name: cg.strings.Intern(f.Name),
		})
	}

	// Generate functions
	for i := range prog.Functions {
		cg.generateFunc(&prog.Functions[i])
	}

	if err := cg.errors.Err(); err != nil {
		return nil, err
	}

	numFields := int32(0)
	if len(prog.Fields) > 0 {
		last := prog.Fields[len(prog.Fields)-1]
		numFields = int32(last.Offset) + int32(slotsForType(last.Type))
	}

	return &EmitInput{
		Statements: cg.statements,
		GlobalDefs: cg.globalDefs,
		FieldDefs:  cg.fieldDefs,
		Functions:  cg.functions,
		Strings:    cg.strings.Bytes(),
		Globals:    cg.globals.Data(),
		NumFields:  numFields,
	}, nil
}

func (cg *CodeGen) generateFunc(fn *IRFunc) {
	nameOfs := cg.strings.Intern(fn.QCName)

	if fn.IsBuiltin {
		df := qc.DFunction{
			FirstStatement: int32(-fn.BuiltinNum),
			Name:           nameOfs,
			NumParms:       int32(len(fn.Params)),
		}
		for i, p := range fn.Params {
			if i >= qc.MaxParms {
				break
			}
			df.ParmSize[i] = byte(slotsForType(p.Type))
		}
		cg.functions = append(cg.functions, df)
		return
	}

	// Allocate function-local address space
	cg.vregMap = make(map[VReg]uint16)
	cg.labelTargets = make(map[int]string)
	cg.nextVReg = 0
	cg.localOfs = cg.globals.NextOffset()
	cg.localSize = 0

	// Allocate parameters — the VM copies OFS_PARM values here on entry.
	// The first len(fn.Params) entries in fn.Locals correspond to params.
	parmStart := cg.localOfs
	for i, p := range fn.Params {
		slots := slotsForType(p.Type)
		ofs := cg.allocLocal(slots)
		// Map the corresponding local's VReg to this param offset
		if i < len(fn.Locals) {
			cg.vregMap[fn.Locals[i].VReg] = ofs
		}
	}

	// Map remaining locals (skip the param entries already mapped above)
	for _, l := range fn.Locals[len(fn.Params):] {
		slots := slotsForType(l.Type)
		ofs := cg.allocLocal(slots)
		cg.vregMap[l.VReg] = ofs
	}

	// First pass: emit statements, collecting label positions
	firstStmt := int32(len(cg.statements))
	labels := make(map[string]int) // label name -> statement index

	for _, inst := range fn.Body {
		if inst.IsLabel() {
			labels[inst.LabelName()] = len(cg.statements)
			continue
		}
		cg.emitInst(&inst)
	}

	// Second pass: patch branch targets
	noPos := token.Position{}
	for i := int(firstStmt); i < len(cg.statements); i++ {
		stmt := &cg.statements[i]
		op := qc.Opcode(stmt.Op)

		switch op {
		case qc.OPGoto:
			if target, ok := cg.labelTargets[i]; ok {
				if dest, ok := labels[target]; ok {
					stmt.A = uint16(int16(dest - i))
				} else {
					cg.errors.Addf(noPos, "undefined label: %s", target)
				}
			}
		case qc.OPIF, qc.OPIFNot:
			if target, ok := cg.labelTargets[i]; ok {
				if dest, ok := labels[target]; ok {
					stmt.B = uint16(int16(dest - i))
				} else {
					cg.errors.Addf(noPos, "undefined label: %s", target)
				}
			}
		}
	}

	df := qc.DFunction{
		FirstStatement: firstStmt,
		ParmStart:      int32(parmStart),
		Locals:         int32(cg.localSize),
		Name:           nameOfs,
		NumParms:       int32(len(fn.Params)),
	}
	for i, p := range fn.Params {
		if i >= qc.MaxParms {
			break
		}
		df.ParmSize[i] = byte(slotsForType(p.Type))
	}

	cg.functions = append(cg.functions, df)
}

func (cg *CodeGen) emitInst(inst *IRInst) {
	op := inst.Op

	switch op {
	case qc.OPGoto:
		idx := len(cg.statements)
		cg.statements = append(cg.statements, qc.DStatement{Op: uint16(op)})
		cg.labelTargets[idx] = inst.Label
		return

	case qc.OPIF, qc.OPIFNot:
		idx := len(cg.statements)
		cg.statements = append(cg.statements, qc.DStatement{
			Op: uint16(op),
			A:  cg.resolveVReg(inst.A),
		})
		cg.labelTargets[idx] = inst.Label
		return

	case qc.OPCall0, qc.OPCall1, qc.OPCall2, qc.OPCall3,
		qc.OPCall4, qc.OPCall5, qc.OPCall6, qc.OPCall7, qc.OPCall8:
		cg.statements = append(cg.statements, qc.DStatement{
			Op: uint16(qc.OPCall0 + qc.Opcode(inst.ArgCount)),
			A:  cg.resolveVReg(inst.A),
		})
		return

	case qc.OPReturn, qc.OPDone:
		cg.statements = append(cg.statements, qc.DStatement{
			Op: uint16(op),
			A:  cg.resolveVReg(inst.A),
			B:  cg.resolveVReg(inst.B),
			C:  cg.resolveVReg(inst.C),
		})
		return
	}

	// Default 3-address instruction
	cg.statements = append(cg.statements, qc.DStatement{
		Op: uint16(op),
		A:  cg.resolveVReg(inst.A),
		B:  cg.resolveVReg(inst.B),
		C:  cg.resolveVReg(inst.C),
	})
}

func (cg *CodeGen) resolveVReg(v VReg) uint16 {
	if v == VRegInvalid {
		return 0
	}
	if ofs, ok := cg.vregMap[v]; ok {
		return ofs
	}
	// Direct global offset encoded as VReg
	return uint16(v)
}

func (cg *CodeGen) allocLocal(slots uint16) uint16 {
	ofs := cg.localOfs + cg.localSize
	cg.localSize += slots
	cg.globals.AllocAnon(slots)
	return ofs
}
