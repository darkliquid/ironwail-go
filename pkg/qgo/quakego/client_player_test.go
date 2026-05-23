package quakego

import (
	"os"
	"strings"
	"testing"
)

func TestCheckWaterJumpAssignsNormalizedForwardVector(t *testing.T) {
	// QuakeC normalize(v) returns a new vector, so the translated gameplay code
	// must assign it before tracing 24 units ahead for the water-jump probe.
	data, err := os.ReadFile("client_player.go")
	if err != nil {
		t.Fatalf("read client_player.go: %v", err)
	}
	if !strings.Contains(string(data), "VForward = engine.Normalize(VForward)") {
		t.Fatal("CheckWaterJump must assign engine.Normalize(VForward) before tracing")
	}
}
