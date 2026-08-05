package game

import (
	"encoding/json"
	"testing"
)

func TestFormatViewpointJSON(t *testing.T) {
	g := &Game{}

	jsonStr, vp := g.formatViewpointJSON([]string{"test-view", "Custom", "description"})
	if jsonStr == "" {
		t.Fatalf("formatViewpointJSON returned empty string")
	}

	if vp.ID != "test-view" {
		t.Fatalf("ID = %s, want test-view", vp.ID)
	}

	if vp.Description != "Custom description" {
		t.Fatalf("Description = %s, want 'Custom description'", vp.Description)
	}

	if vp.Map != "start" {
		t.Fatalf("Map = %s, want start", vp.Map)
	}

	if vp.Tag != "id1" && vp.Tag != "" {
		t.Fatalf("Tag = %s, want id1 or default", vp.Tag)
	}

	var parsed viewpointJSON
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("failed to parse output JSON: %v", err)
	}

	if parsed.ID != vp.ID {
		t.Fatalf("parsed ID %s != vp ID %s", parsed.ID, vp.ID)
	}
}
