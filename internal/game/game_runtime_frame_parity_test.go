package game

import (
	"testing"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

func TestParseParityAnglesEnv(t *testing.T) {
	t.Parallel()

	got, ok := parseParityAnglesEnv("-28.82 24.43 0")
	if !ok {
		t.Fatal("parseParityAnglesEnv returned !ok, want ok")
	}
	want := types.Vec3{X: -28.82, Y: 24.43, Z: 0}
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
			ViewAngles:  types.Vec3{X: 1, Y: 2, Z: 3},
			MViewAngles: [2]types.Vec3{{X: 4, Y: 5, Z: 6}, {X: 7, Y: 8, Z: 9}},
			PendingCmd:  cl.UserCmd{ViewAngles: types.Vec3{X: 10, Y: 11, Z: 12}},
		},
	}

	g.applyParityViewAnglesOverride()

	want := types.Vec3{X: -28.82, Y: 24.43, Z: 0}
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
