package qc

import (
	"fmt"
	"io"
)

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

	// State.
	loaded bool
}

// NewCSQC creates a new CSQC instance with a fresh VM.
func NewCSQC() *CSQC {
	return &CSQC{
		VM:             NewVM(),
		initFunc:       -1,
		shutdownFunc:   -1,
		drawHudFunc:    -1,
		drawScoresFunc: -1,
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

	if err := c.VM.LoadProgs(r); err != nil {
		return fmt.Errorf("csqc: load progs: %w", err)
	}

	c.initFunc = c.VM.FindFunction("CSQC_Init")
	c.shutdownFunc = c.VM.FindFunction("CSQC_Shutdown")
	c.drawHudFunc = c.VM.FindFunction("CSQC_DrawHud")
	c.drawScoresFunc = c.VM.FindFunction("CSQC_DrawScores")

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
func (c *CSQC) CallDrawHud(virtSizeX, virtSizeY float32, showScores bool) error {
	if !c.loaded {
		return fmt.Errorf("csqc: not loaded")
	}
	if c.drawHudFunc < 0 {
		return fmt.Errorf("csqc: required function CSQC_DrawHud not found")
	}

	c.VM.SetGVector(OFSParm0, [3]float32{virtSizeX, virtSizeY, 0})
	if showScores {
		c.VM.SetGFloat(OFSParm1, 1)
	} else {
		c.VM.SetGFloat(OFSParm1, 0)
	}

	if err := c.VM.ExecuteProgram(c.drawHudFunc); err != nil {
		return fmt.Errorf("csqc: call CSQC_DrawHud: %w", err)
	}
	return nil
}

// CallDrawScores calls CSQC_DrawScores when available.
// Parameters: (vector virtSize, float showScores).
func (c *CSQC) CallDrawScores(virtSizeX, virtSizeY float32, showScores bool) error {
	if !c.loaded {
		return fmt.Errorf("csqc: not loaded")
	}
	if c.drawScoresFunc < 0 {
		return nil
	}

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

// Unload resets CSQC state and replaces the VM with a fresh instance.
func (c *CSQC) Unload() {
	c.VM = NewVM()
	c.initFunc = -1
	c.shutdownFunc = -1
	c.drawHudFunc = -1
	c.drawScoresFunc = -1
	c.loaded = false
}
