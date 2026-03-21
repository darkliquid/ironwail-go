// What: Temporary entity (TE_*) parsing and state tests.
// Why: Validates the creation and management of short-lived effects like beams.
// Where in C: cl_tent.c

package client

import (
	"bytes"
	"testing"

	inet "github.com/ironwail/ironwail-go/internal/net"
)

// TestParseTempEntityBeamStoresBeamState verifies that lightning beams are correctly parsed and stored in the client state.
// Why: Lightning beams must be tracked by entity ID to allow the server to update or terminate them.
// Where in C: cl_tent.c, CL_ParseTEnt.
func TestParseTempEntityBeamStoresBeamState(t *testing.T) {
	c := NewClient()
	c.Time = 5
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(inet.SVCTempEntity))
	msg.WriteByte(byte(inet.TE_LIGHTNING1))
	writeShort(msg, 42)
	writeCoord(msg, 1)
	writeCoord(msg, 2)
	writeCoord(msg, 3)
	writeCoord(msg, 31)
	writeCoord(msg, 32)
	writeCoord(msg, 33)
	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}

	beam, ok := findBeamByEntity(c, 42)
	if !ok {
		t.Fatal("expected beam slot for entity 42")
	}
	if beam.model != "progs/bolt.mdl" {
		t.Fatalf("beam model = %q, want progs/bolt.mdl", beam.model)
	}
	if beam.start != [3]float32{1, 2, 3} {
		t.Fatalf("beam start = %v, want [1 2 3]", beam.start)
	}
	if beam.end != [3]float32{31, 32, 33} {
		t.Fatalf("beam end = %v, want [31 32 33]", beam.end)
	}
	if beam.endTime != 5.2 {
		t.Fatalf("beam endtime = %v, want 5.2", beam.endTime)
	}
}

// TestUpdateTempEntitiesSkipsExpiredBeams ensures that beams whose duration has elapsed are not processed for rendering.
// Why: Temporary entities are short-lived and must be automatically cleaned up to avoid visual clutter and memory leaks.
// Where in C: cl_tent.c, CL_UpdateTEnts.
func TestUpdateTempEntitiesSkipsExpiredBeams(t *testing.T) {
	c := NewClient()
	c.Time = 10
	c.beams[0] = beamState{
		entity:  1,
		typ:     inet.TE_LIGHTNING1,
		model:   "progs/bolt.mdl",
		endTime: 9,
		start:   [3]float32{0, 0, 0},
		end:     [3]float32{90, 0, 0},
	}

	c.UpdateTempEntities()
	if got := len(c.BeamSegments); got != 0 {
		t.Fatalf("BeamSegments len = %d, want 0", got)
	}
}

// TestUpdateTempEntitiesGeneratesBeamSegments verifies that long beams are subdivided into multiple segments for rendering.
// Why: Quake renders lightning beams as a series of 30-unit segments to allow for slight jitter and proper clipping.
// Where in C: cl_tent.c, CL_UpdateTEnts.
func TestUpdateTempEntitiesGeneratesBeamSegments(t *testing.T) {
	c := NewClient()
	c.Time = 1
	c.beams[0] = beamState{
		entity:  7,
		typ:     inet.TE_LIGHTNING2,
		model:   "progs/bolt2.mdl",
		endTime: 1.2,
		start:   [3]float32{0, 0, 0},
		end:     [3]float32{90, 0, 0},
	}

	c.UpdateTempEntities()
	if got := len(c.BeamSegments); got != 3 {
		t.Fatalf("BeamSegments len = %d, want 3", got)
	}
	if got := c.BeamSegments[0].Origin; got != [3]float32{0, 0, 0} {
		t.Fatalf("segment 0 origin = %v, want [0 0 0]", got)
	}
	if got := c.BeamSegments[1].Origin; got != [3]float32{30, 0, 0} {
		t.Fatalf("segment 1 origin = %v, want [30 0 0]", got)
	}
	if got := c.BeamSegments[2].Origin; got != [3]float32{60, 0, 0} {
		t.Fatalf("segment 2 origin = %v, want [60 0 0]", got)
	}
}

// TestParseTempEntitySpikeAppendsCanonicalImpactSound ensures that spike impacts trigger the appropriate sound effects.
// Why: Audio feedback for projectile impacts is essential for game feel and situational awareness.
// Where in C: cl_tent.c, CL_ParseTEnt.
func TestParseTempEntitySpikeAppendsCanonicalImpactSound(t *testing.T) {
	c := NewClient()
	p := NewParser(c)

	msg := bytes.NewBuffer(nil)
	msg.WriteByte(byte(inet.SVCTempEntity))
	msg.WriteByte(byte(inet.TE_SPIKE))
	writeCoord(msg, 1)
	writeCoord(msg, 2)
	writeCoord(msg, 3)
	msg.WriteByte(0xFF)

	if err := p.ParseServerMessage(msg.Bytes()); err != nil {
		t.Fatalf("ParseServerMessage() error = %v", err)
	}
	if got := len(c.SoundEvents); got != 1 {
		t.Fatalf("SoundEvents len = %d, want 1", got)
	}
	sound := c.SoundEvents[0].SoundName
	switch sound {
	case "weapons/tink1.wav", "weapons/ric1.wav", "weapons/ric2.wav", "weapons/ric3.wav":
	default:
		t.Fatalf("SoundName = %q, want canonical spike impact sound", sound)
	}
}

func findBeamByEntity(c *Client, entity int) (beamState, bool) {
	for i := range c.beams {
		if c.beams[i].entity == entity && c.beams[i].model != "" {
			return c.beams[i], true
		}
	}
	return beamState{}, false
}
