package qc

import (
	"bytes"
	"reflect"
	"testing"
)

func TestNewCSQCCreatesValidInstance(t *testing.T) {
	csqc := NewCSQC()
	if csqc == nil {
		t.Fatal("NewCSQC() returned nil")
	}
	if csqc.VM == nil {
		t.Fatal("NewCSQC().VM is nil")
	}
	if csqc.IsLoaded() {
		t.Fatal("new CSQC instance should not be loaded")
	}
}

func TestCSQCLoadFailsWithInvalidData(t *testing.T) {
	csqc := NewCSQC()

	err := csqc.Load(bytes.NewReader(nil))
	if err == nil {
		t.Fatal("Load() expected error for invalid data")
	}
	if csqc.IsLoaded() {
		t.Fatal("CSQC should not be marked loaded after failed Load()")
	}
}

func TestCSQCIsLoadedFalseBeforeLoad(t *testing.T) {
	csqc := NewCSQC()
	if csqc.IsLoaded() {
		t.Fatal("IsLoaded() = true, want false before Load()")
	}
}

func TestCSQCFunctionIndicesStartAtMinusOne(t *testing.T) {
	csqc := NewCSQC()

	if csqc.initFunc != -1 {
		t.Fatalf("initFunc = %d, want -1", csqc.initFunc)
	}
	if csqc.shutdownFunc != -1 {
		t.Fatalf("shutdownFunc = %d, want -1", csqc.shutdownFunc)
	}
	if csqc.drawHudFunc != -1 {
		t.Fatalf("drawHudFunc = %d, want -1", csqc.drawHudFunc)
	}
	if csqc.drawScoresFunc != -1 {
		t.Fatalf("drawScoresFunc = %d, want -1", csqc.drawScoresFunc)
	}
}

func TestCSQCGlobalsStartAtMinusOne(t *testing.T) {
	csqc := NewCSQC()

	if csqc.globals.cltime != -1 {
		t.Fatalf("globals.cltime = %d, want -1", csqc.globals.cltime)
	}
	if csqc.globals.clframetime != -1 {
		t.Fatalf("globals.clframetime = %d, want -1", csqc.globals.clframetime)
	}
	if csqc.globals.maxclients != -1 {
		t.Fatalf("globals.maxclients = %d, want -1", csqc.globals.maxclients)
	}
	if csqc.globals.intermission != -1 {
		t.Fatalf("globals.intermission = %d, want -1", csqc.globals.intermission)
	}
	if csqc.globals.intermissionTime != -1 {
		t.Fatalf("globals.intermissionTime = %d, want -1", csqc.globals.intermissionTime)
	}
	if csqc.globals.playerLocalNum != -1 {
		t.Fatalf("globals.playerLocalNum = %d, want -1", csqc.globals.playerLocalNum)
	}
	if csqc.globals.playerLocalEntNum != -1 {
		t.Fatalf("globals.playerLocalEntNum = %d, want -1", csqc.globals.playerLocalEntNum)
	}
	if csqc.globals.viewAngles != -1 {
		t.Fatalf("globals.viewAngles = %d, want -1", csqc.globals.viewAngles)
	}
	if csqc.globals.clientCommandFrame != -1 {
		t.Fatalf("globals.clientCommandFrame = %d, want -1", csqc.globals.clientCommandFrame)
	}
	if csqc.globals.serverCommandFrame != -1 {
		t.Fatalf("globals.serverCommandFrame = %d, want -1", csqc.globals.serverCommandFrame)
	}
}

func TestCSQCSyncGlobalsUnloadedNoPanic(t *testing.T) {
	csqc := NewCSQC()
	csqc.SyncGlobals(CSQCFrameState{})
}

func TestCSQCFrameStateZeroValueSafe(t *testing.T) {
	var state CSQCFrameState
	if state.Time != 0 {
		t.Fatalf("state.Time = %f, want 0", state.Time)
	}
}

func TestCSQCPrecacheModelReturnsStableIndices(t *testing.T) {
	csqc := NewCSQC()

	if got := csqc.PrecacheModel("progs/soldier.mdl"); got != 1 {
		t.Fatalf("first model index = %d, want 1", got)
	}
	if got := csqc.PrecacheModel("progs/ogre.mdl"); got != 2 {
		t.Fatalf("second model index = %d, want 2", got)
	}
	if got := csqc.PrecacheModel("progs/soldier.mdl"); got != 1 {
		t.Fatalf("duplicate model index = %d, want 1", got)
	}
}

func TestCSQCPrecacheSoundReturnsStableIndices(t *testing.T) {
	csqc := NewCSQC()

	if got := csqc.PrecacheSound("weapons/rocket1i.wav"); got != 1 {
		t.Fatalf("first sound index = %d, want 1", got)
	}
	if got := csqc.PrecacheSound("weapons/sgun1.wav"); got != 2 {
		t.Fatalf("second sound index = %d, want 2", got)
	}
	if got := csqc.PrecacheSound("weapons/rocket1i.wav"); got != 1 {
		t.Fatalf("duplicate sound index = %d, want 1", got)
	}
}

func TestCSQCPrecachePicReturnsStableIndices(t *testing.T) {
	csqc := NewCSQC()

	if got := csqc.PrecachePic("gfx/conback.lmp"); got != 1 {
		t.Fatalf("first pic index = %d, want 1", got)
	}
	if got := csqc.PrecachePic("gfx/menuplyr.lmp"); got != 2 {
		t.Fatalf("second pic index = %d, want 2", got)
	}
	if got := csqc.PrecachePic("gfx/conback.lmp"); got != 1 {
		t.Fatalf("duplicate pic index = %d, want 1", got)
	}
}

func TestCSQCPrecachedModelsReturnsRegistrationOrder(t *testing.T) {
	csqc := NewCSQC()
	csqc.PrecacheModel("progs/player.mdl")
	csqc.PrecacheModel("progs/flame.mdl")

	want := []string{"progs/player.mdl", "progs/flame.mdl"}
	if got := csqc.PrecachedModels(); !reflect.DeepEqual(got, want) {
		t.Fatalf("PrecachedModels() = %v, want %v", got, want)
	}
}

func TestCSQCUnloadResetsPrecacheRegistries(t *testing.T) {
	csqc := NewCSQC()
	csqc.PrecacheModel("progs/player.mdl")
	csqc.PrecacheSound("misc/menu2.wav")
	csqc.PrecachePic("gfx/pause.lmp")

	csqc.Unload()

	if len(csqc.precachedModels) != 0 || len(csqc.precachedSounds) != 0 || len(csqc.precachedPics) != 0 {
		t.Fatalf("expected precache registries to be reset on Unload")
	}
	if got := csqc.PrecacheModel("progs/new.mdl"); got != 1 {
		t.Fatalf("model index after Unload = %d, want 1", got)
	}
	if got := csqc.PrecacheSound("misc/new.wav"); got != 1 {
		t.Fatalf("sound index after Unload = %d, want 1", got)
	}
	if got := csqc.PrecachePic("gfx/new.lmp"); got != 1 {
		t.Fatalf("pic index after Unload = %d, want 1", got)
	}
}

func TestCSQCSyncGlobalsUsesRealtimeForCltime(t *testing.T) {
	csqc := NewCSQC()
	csqc.VM.Globals = make([]float32, 32)
	csqc.VM.Progs = &DProgs{NumGlobals: 32}
	csqc.loaded = true
	csqc.globals.cltime = 4

	csqc.SyncGlobals(CSQCFrameState{
		RealTime:  10.5,
		Time:      2.25,
		FrameTime: 0.05,
	})

	if got := csqc.VM.GFloat(4); got != 10.5 {
		t.Fatalf("cltime global = %v, want realtime 10.5", got)
	}
	if got := csqc.VM.GFloat(OFSTime); got != 2.25 {
		t.Fatalf("time global = %v, want client time 2.25", got)
	}
}
