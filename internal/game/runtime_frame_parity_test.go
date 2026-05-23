package game

import (
	"testing"

	cl "github.com/darkliquid/ironwail-go/internal/client"
)

func TestParseParityAnglesEnv(t *testing.T) {
	t.Parallel()

	got, ok := parseParityAnglesEnv("-28.82 24.43 0")
	if !ok {
		t.Fatal("parseParityAnglesEnv returned !ok, want ok")
	}
	want := [3]float32{-28.82, 24.43, 0}
	if got != want {
		t.Fatalf("parseParityAnglesEnv = %v, want %v", got, want)
	}

	if _, ok := parseParityAnglesEnv("bad values"); ok {
		t.Fatal("parseParityAnglesEnv accepted invalid input")
	}
}

func TestApplyParityViewAnglesOverride(t *testing.T) {
	t.Setenv("PARITY_RUN", "1")
	t.Setenv("PARITY_ANGLES", "-28.82 24.43 0")

	g := &Game{
		Client: &cl.Client{
			State:       cl.StateActive,
			ViewAngles:  [3]float32{1, 2, 3},
			MViewAngles: [2][3]float32{{4, 5, 6}, {7, 8, 9}},
			PendingCmd:  cl.UserCmd{ViewAngles: [3]float32{10, 11, 12}},
		},
	}

	g.applyParityViewAnglesOverride()

	want := [3]float32{-28.82, 24.43, 0}
	if g.Client.ViewAngles != want {
		t.Fatalf("Client.ViewAngles = %v, want %v", g.Client.ViewAngles, want)
	}
	if g.Client.MViewAngles[0] != want || g.Client.MViewAngles[1] != want {
		t.Fatalf("Client.MViewAngles = %v, want both %v", g.Client.MViewAngles, want)
	}
	if g.Client.PendingCmd.ViewAngles != want {
		t.Fatalf("Client.PendingCmd.ViewAngles = %v, want %v", g.Client.PendingCmd.ViewAngles, want)
	}
}
