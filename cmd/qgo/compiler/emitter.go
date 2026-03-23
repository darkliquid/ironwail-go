package compiler

import (
	"bytes"
	"encoding/binary"
	"math"

	"github.com/ironwail/ironwail-go/internal/qc"
)

// EmitInput collects all data needed to produce a progs.dat binary.
type EmitInput struct {
	Statements []qc.DStatement
	GlobalDefs []qc.DDef
	FieldDefs  []qc.DDef
	Functions  []qc.DFunction
	Strings    []byte   // Raw string table
	Globals    []uint32 // Raw global data
	NumFields  int32    // Entity field count
}

// Emit serializes a compiled program into the progs.dat binary format.
func Emit(in *EmitInput) ([]byte, error) {
	var buf bytes.Buffer

	// Calculate section sizes
	stmtSize := len(in.Statements) * 8 // 4 x uint16
	gdefSize := len(in.GlobalDefs) * 8 // uint16 + uint16 + int32
	fdefSize := len(in.FieldDefs) * 8  // same
	funcSize := len(in.Functions) * 36 // DFunction is 36 bytes
	strSize := len(in.Strings)
	_ = len(in.Globals) * 4 // uint32 each; size used implicitly via binary.Write

	// Header is 60 bytes (DProgs has 15 int32 fields)
	headerSize := 60
	offset := int32(headerSize)

	header := qc.DProgs{
		Version:       qc.ProgVersion,
		CRC:           int32(qc.ProgHeaderCRC),
		Statements:    offset,
		NumStatements: int32(len(in.Statements)),
	}
	offset += int32(stmtSize)

	header.GlobalDefs = offset
	header.NumGlobalDefs = int32(len(in.GlobalDefs))
	offset += int32(gdefSize)

	header.FieldDefs = offset
	header.NumFieldDefs = int32(len(in.FieldDefs))
	offset += int32(fdefSize)

	header.Functions = offset
	header.NumFunctions = int32(len(in.Functions))
	offset += int32(funcSize)

	header.Strings = offset
	header.NumStrings = int32(strSize)
	offset += int32(strSize)

	header.Globals = offset
	header.NumGlobals = int32(len(in.Globals))
	header.EntityFields = in.NumFields

	// Write header
	if err := binary.Write(&buf, binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	// Write statements
	for _, s := range in.Statements {
		if err := binary.Write(&buf, binary.LittleEndian, &s); err != nil {
			return nil, err
		}
	}

	// Write global definitions
	for _, d := range in.GlobalDefs {
		if err := binary.Write(&buf, binary.LittleEndian, &d); err != nil {
			return nil, err
		}
	}

	// Write field definitions
	for _, d := range in.FieldDefs {
		if err := binary.Write(&buf, binary.LittleEndian, &d); err != nil {
			return nil, err
		}
	}

	// Write functions
	for _, f := range in.Functions {
		if err := binary.Write(&buf, binary.LittleEndian, &f); err != nil {
			return nil, err
		}
	}

	// Write string table
	buf.Write(in.Strings)

	// Write globals
	for _, g := range in.Globals {
		if err := binary.Write(&buf, binary.LittleEndian, g); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

// float32ToUint32 converts a float32 to its raw uint32 bits.
func float32ToUint32(f float32) uint32 {
	return math.Float32bits(f)
}
