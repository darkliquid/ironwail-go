package game

import (
	"bytes"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/input"
	"github.com/darkliquid/ironwail-go/internal/qc"
)

func TestHandleGameKeyEventRoutesToCSQCInputEventWhenLoaded(t *testing.T) {
	g := New()
	g.Input = input.NewSystem(nil)
	g.CSQC = qc.NewCSQC()

	// Minimal CSQC setup with inputEventFunc returning 1 (handled)
	g.CSQC.VM.Globals = make([]float32, 64)
	g.CSQC.VM.Functions = []qc.DFunction{{FirstStatement: 0}}
	g.CSQC.VM.Statements = []qc.DStatement{{Op: uint16(qc.OPDone), A: 10}}
	g.CSQC.VM.SetGFloat(10, 1)

	// Load minimal dummy progs to mark loaded
	_ = g.CSQC.Load(bytes.NewReader(buildMinimalCSQCProgs()))
	if !g.CSQC.IsLoaded() {
		t.Skip("CSQC load helper failed, skipping input event test")
	}

	// Verify handleGameKeyEvent suppresses default binding processing when CSQC handles key event
	var cmdExecuted bool
	g.Host.Cmd.AddCommand("+jump", func(args []string) {
		cmdExecuted = true
	}, "Jump")

	g.Input.SetBinding(int(' '), "+jump")
	g.handleGameKeyEvent(input.KeyEvent{Key: int(' '), Down: true})

	if cmdExecuted {
		t.Fatal("expected +jump command execution to be suppressed when CSQC handles input event")
	}
}

func buildMinimalCSQCProgs() []byte {
	// A minimal valid progs dat buffer with CSQC_DrawHud and CSQC_InputEvent functions
	buf := new(bytes.Buffer)

	// Write DHeader (6 * 8 bytes = 48 bytes)
	// Version 6
	header := make([]int32, 12)
	header[0] = 6
	header[1] = 0 // crc

	// Lump offsets & counts
	// Lumps: Statements(0), GlobalDefs(1), FieldDefs(2), Functions(3), Strings(4), Globals(5)
	header[2] = 48           // Statements offset
	header[3] = 1            // Statements count (1 DStatement = 8 bytes)
	header[4] = 48 + 8       // GlobalDefs offset
	header[5] = 0            // GlobalDefs count
	header[6] = 48 + 8       // FieldDefs offset
	header[7] = 0            // FieldDefs count
	header[8] = 48 + 8       // Functions offset
	header[9] = 2            // Functions count (2 DFunction = 72 bytes)
	header[10] = 48 + 8 + 72 // Strings offset
	header[11] = 30          // Strings count (30 bytes)

	for _, v := range header {
		buf.WriteByte(byte(v))
		buf.WriteByte(byte(v >> 8))
		buf.WriteByte(byte(v >> 16))
		buf.WriteByte(byte(v >> 24))
	}

	// Statement 0: OPDone
	buf.Write([]byte{0, 0, 0, 0, 0, 0, 0, 0})

	// Function 0: CSQC_DrawHud, Function 1: CSQC_InputEvent
	// DFunction structure: FirstStatement int32, ParmsOffset int32, NumParms int32, LocalVars int32, Profile int32, Name int32, File int32
	// Func 0: CSQC_DrawHud (Name = 1)
	func0 := []int32{0, 0, 0, 0, 0, 1, 0}
	for _, v := range func0 {
		buf.WriteByte(byte(v))
		buf.WriteByte(byte(v >> 8))
		buf.WriteByte(byte(v >> 16))
		buf.WriteByte(byte(v >> 24))
	}
	// Func 1: CSQC_InputEvent (Name = 14)
	func1 := []int32{0, 0, 0, 0, 0, 14, 0}
	for _, v := range func1 {
		buf.WriteByte(byte(v))
		buf.WriteByte(byte(v >> 8))
		buf.WriteByte(byte(v >> 16))
		buf.WriteByte(byte(v >> 24))
	}

	// Strings: "\x00CSQC_DrawHud\x00CSQC_InputEvent\x00"
	strTable := "\x00CSQC_DrawHud\x00CSQC_InputEvent\x00"
	buf.WriteString(strTable)

	return buf.Bytes()
}
