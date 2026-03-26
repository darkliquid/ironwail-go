package qc

import (
	"fmt"
	"io"
)

// csqcGlobals caches offsets for CSQC-specific global variables.
// Offsets are -1 when the global is not defined in the loaded progs.
type csqcGlobals struct {
	cltime             int
	clframetime        int
	maxclients         int
	intermission       int
	intermissionTime   int
	playerLocalNum     int
	playerLocalEntNum  int
	viewAngles         int
	clientCommandFrame int
	serverCommandFrame int
}

func newCSQCGlobals() csqcGlobals {
	return csqcGlobals{
		cltime:             -1,
		clframetime:        -1,
		maxclients:         -1,
		intermission:       -1,
		intermissionTime:   -1,
		playerLocalNum:     -1,
		playerLocalEntNum:  -1,
		viewAngles:         -1,
		clientCommandFrame: -1,
		serverCommandFrame: -1,
	}
}

// CSQCFrameState holds per-frame state values synced to CSQC globals
// before each entry point call.
type CSQCFrameState struct {
	RealTime           float32
	Time               float32
	FrameTime          float32
	MaxClients         float32
	Intermission       float32
	IntermissionTime   float32
	PlayerLocalNum     float32
	PlayerLocalEntNum  float32
	ViewAngles         [3]float32
	ClientCommandFrame float32
	ServerCommandFrame float32
}

// CSQC represents a client-side QuakeC VM instance.
// It wraps a standard VM with CSQC-specific entry points and lifecycle.
type CSQC struct {
	// VM is the underlying QC virtual machine.
	VM *VM

	// CSQC entry point function indices (-1 if not found).
	initFunc       int
	shutdownFunc   int
	drawHudFunc    int
	drawScoresFunc int

	// Cached CSQC global variable offsets.
	globals csqcGlobals

	// Precache registries (resource name -> index).
	precachedModels map[string]int
	precachedSounds map[string]int
	precachedPics   map[string]int

	// State.
	loaded bool
}

// NewCSQC creates a new CSQC instance with a fresh VM.
func NewCSQC() *CSQC {
	return &CSQC{
		VM:              NewVM(),
		initFunc:        -1,
		shutdownFunc:    -1,
		drawHudFunc:     -1,
		drawScoresFunc:  -1,
		globals:         newCSQCGlobals(),
		precachedModels: make(map[string]int),
		precachedSounds: make(map[string]int),
		precachedPics:   make(map[string]int),
	}
}

// Load loads a csprogs.dat program and resolves CSQC entry points.
// CSQC_DrawHud is required and causes an error when missing.
func (c *CSQC) Load(r io.ReadSeeker) error {
	c.loaded = false
	c.initFunc = -1
	c.shutdownFunc = -1
	c.drawHudFunc = -1
	c.drawScoresFunc = -1
	c.globals = newCSQCGlobals()
	c.precachedModels = make(map[string]int)
	c.precachedSounds = make(map[string]int)
	c.precachedPics = make(map[string]int)

	if err := c.VM.LoadProgs(r); err != nil {
		return fmt.Errorf("csqc: load progs: %w", err)
	}

	c.initFunc = c.VM.FindFunction("CSQC_Init")
	c.shutdownFunc = c.VM.FindFunction("CSQC_Shutdown")
	c.drawHudFunc = c.VM.FindFunction("CSQC_DrawHud")
	c.drawScoresFunc = c.VM.FindFunction("CSQC_DrawScores")

	c.globals.cltime = c.VM.FindGlobal("cltime")
	c.globals.clframetime = c.VM.FindGlobal("clframetime")
	c.globals.maxclients = c.VM.FindGlobal("maxclients")
	c.globals.intermission = c.VM.FindGlobal("intermission")
	c.globals.intermissionTime = c.VM.FindGlobal("intermission_time")
	c.globals.playerLocalNum = c.VM.FindGlobal("player_localnum")
	c.globals.playerLocalEntNum = c.VM.FindGlobal("player_localentnum")
	c.globals.viewAngles = c.VM.FindGlobal("view_angles")
	c.globals.clientCommandFrame = c.VM.FindGlobal("clientcommandframe")
	c.globals.serverCommandFrame = c.VM.FindGlobal("servercommandframe")

	if c.drawHudFunc < 0 {
		return fmt.Errorf("csqc: required function CSQC_DrawHud not found")
	}

	c.loaded = true
	return nil
}

// IsLoaded reports whether CSQC progs are loaded and have required entry points.
func (c *CSQC) IsLoaded() bool {
	return c.loaded
}

// HasDrawScores reports whether the optional CSQC_DrawScores entry point exists.
func (c *CSQC) HasDrawScores() bool {
	return c.loaded && c.drawScoresFunc >= 0
}

// SyncGlobals writes per-frame CSQC state to globals before entry point calls.
func (c *CSQC) SyncGlobals(state CSQCFrameState) {
	if !c.loaded || c.VM == nil {
		return
	}

	if c.globals.cltime >= 0 {
		c.VM.SetGFloat(c.globals.cltime, state.RealTime)
	}
	if c.globals.clframetime >= 0 {
		c.VM.SetGFloat(c.globals.clframetime, state.FrameTime)
	}
	if c.globals.maxclients >= 0 {
		c.VM.SetGFloat(c.globals.maxclients, state.MaxClients)
	}
	if c.globals.intermission >= 0 {
		c.VM.SetGFloat(c.globals.intermission, state.Intermission)
	}
	if c.globals.intermissionTime >= 0 {
		c.VM.SetGFloat(c.globals.intermissionTime, state.IntermissionTime)
	}
	if c.globals.playerLocalNum >= 0 {
		c.VM.SetGFloat(c.globals.playerLocalNum, state.PlayerLocalNum)
	}
	if c.globals.playerLocalEntNum >= 0 {
		c.VM.SetGFloat(c.globals.playerLocalEntNum, state.PlayerLocalEntNum)
	}
	if c.globals.viewAngles >= 0 {
		c.VM.SetGVector(c.globals.viewAngles, state.ViewAngles)
	}
	if c.globals.clientCommandFrame >= 0 {
		c.VM.SetGFloat(c.globals.clientCommandFrame, state.ClientCommandFrame)
	}
	if c.globals.serverCommandFrame >= 0 {
		c.VM.SetGFloat(c.globals.serverCommandFrame, state.ServerCommandFrame)
	}

	c.VM.SetGFloat(OFSTime, state.Time)
	c.VM.SetGFloat(OFSFrameTime, state.FrameTime)
}

// CallInit calls CSQC_Init when available.
// Parameters: (float apilevel, string enginename, float engineversion).
func (c *CSQC) CallInit(engineName string, engineVersion float32) error {
	if !c.loaded {
		return fmt.Errorf("csqc: not loaded")
	}
	if c.initFunc < 0 {
		return nil
	}

	c.VM.SetGFloat(OFSParm0, 0)
	c.VM.SetGInt(OFSParm1, c.VM.AllocString(engineName))
	c.VM.SetGFloat(OFSParm2, engineVersion)

	if err := c.VM.ExecuteProgram(c.initFunc); err != nil {
		return fmt.Errorf("csqc: call CSQC_Init: %w", err)
	}
	return nil
}

// CallShutdown calls CSQC_Shutdown when available.
func (c *CSQC) CallShutdown() error {
	if !c.loaded {
		return fmt.Errorf("csqc: not loaded")
	}
	if c.shutdownFunc < 0 {
		return nil
	}

	if err := c.VM.ExecuteProgram(c.shutdownFunc); err != nil {
		return fmt.Errorf("csqc: call CSQC_Shutdown: %w", err)
	}
	return nil
}

// CallDrawHud calls CSQC_DrawHud.
// Parameters: (vector virtSize, float showScores).
// It returns whether CSQC explicitly reported that it drew the HUD.
func (c *CSQC) CallDrawHud(state CSQCFrameState, virtSizeX, virtSizeY float32, showScores bool) (bool, error) {
	if !c.loaded {
		return false, fmt.Errorf("csqc: not loaded")
	}
	if c.drawHudFunc < 0 {
		return false, fmt.Errorf("csqc: required function CSQC_DrawHud not found")
	}

	c.SyncGlobals(state)
	c.VM.SetGFloat(OFSReturn, 0)
	c.VM.SetGVector(OFSParm0, [3]float32{virtSizeX, virtSizeY, 0})
	if showScores {
		c.VM.SetGFloat(OFSParm1, 1)
	} else {
		c.VM.SetGFloat(OFSParm1, 0)
	}

	if err := c.VM.ExecuteProgram(c.drawHudFunc); err != nil {
		return false, fmt.Errorf("csqc: call CSQC_DrawHud: %w", err)
	}
	return c.VM.GFloat(OFSReturn) != 0, nil
}

// CallDrawScores calls CSQC_DrawScores when available.
// Parameters: (vector virtSize, float showScores).
func (c *CSQC) CallDrawScores(state CSQCFrameState, virtSizeX, virtSizeY float32, showScores bool) error {
	if !c.loaded {
		return fmt.Errorf("csqc: not loaded")
	}
	if c.drawScoresFunc < 0 {
		return nil
	}

	c.SyncGlobals(state)
	c.VM.SetGVector(OFSParm0, [3]float32{virtSizeX, virtSizeY, 0})
	if showScores {
		c.VM.SetGFloat(OFSParm1, 1)
	} else {
		c.VM.SetGFloat(OFSParm1, 0)
	}

	if err := c.VM.ExecuteProgram(c.drawScoresFunc); err != nil {
		return fmt.Errorf("csqc: call CSQC_DrawScores: %w", err)
	}
	return nil
}

// PrecacheModel registers a model for CSQC use.
// Returns the model index.
func (c *CSQC) PrecacheModel(name string) int {
	if idx, ok := c.precachedModels[name]; ok {
		return idx
	}
	idx := len(c.precachedModels) + 1
	c.precachedModels[name] = idx
	return idx
}

// PrecacheSound registers a sound for CSQC use.
// Returns the sound index.
func (c *CSQC) PrecacheSound(name string) int {
	if idx, ok := c.precachedSounds[name]; ok {
		return idx
	}
	idx := len(c.precachedSounds) + 1
	c.precachedSounds[name] = idx
	return idx
}

// PrecachePic registers a pic for CSQC use.
// Returns the pic index.
func (c *CSQC) PrecachePic(name string) int {
	if idx, ok := c.precachedPics[name]; ok {
		return idx
	}
	idx := len(c.precachedPics) + 1
	c.precachedPics[name] = idx
	return idx
}

func namesByIndex(registry map[string]int) []string {
	names := make([]string, len(registry))
	for name, idx := range registry {
		if idx <= 0 || idx > len(names) {
			continue
		}
		names[idx-1] = name
	}
	return names
}

// PrecachedModels returns model names in their registration order.
func (c *CSQC) PrecachedModels() []string {
	return namesByIndex(c.precachedModels)
}

// PrecachedSounds returns sound names in their registration order.
func (c *CSQC) PrecachedSounds() []string {
	return namesByIndex(c.precachedSounds)
}

// PrecachedPics returns pic names in their registration order.
func (c *CSQC) PrecachedPics() []string {
	return namesByIndex(c.precachedPics)
}

// Unload resets CSQC state and replaces the VM with a fresh instance.
func (c *CSQC) Unload() {
	c.VM = NewVM()
	c.initFunc = -1
	c.shutdownFunc = -1
	c.drawHudFunc = -1
	c.drawScoresFunc = -1
	c.globals = newCSQCGlobals()
	c.precachedModels = make(map[string]int)
	c.precachedSounds = make(map[string]int)
	c.precachedPics = make(map[string]int)
	c.loaded = false
}
