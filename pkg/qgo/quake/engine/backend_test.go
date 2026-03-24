package engine

import (
	"math"
	"testing"

	"github.com/ironwail/ironwail-go/pkg/qgo/quake"
)

func TestBackendHooks(t *testing.T) {
	ResetBackend()
	defer ResetBackend()

	ent := &quake.Entity{}
	spawned := &quake.Entity{Health: 42}
	found := &quake.Entity{ClassName: "target"}

	calls := map[string]int{}
	SetBackend(Backend{
		SetOrigin: func(e *quake.Entity, org quake.Vec3) {
			calls["SetOrigin"]++
			e.Origin = org
		},
		Random: func() float32 {
			calls["Random"]++
			return 0.25
		},
		Spawn: func() *quake.Entity {
			calls["Spawn"]++
			return spawned
		},
		Find: func(_ *quake.Entity, field string, match string) *quake.Entity {
			calls["Find"]++
			if field == "classname" && match == "target" {
				return found
			}
			return nil
		},
		PrecacheSound: func(s string) string {
			calls["PrecacheSound"]++
			return "cached:" + s
		},
		Vtos: func(v quake.Vec3) string {
			calls["Vtos"]++
			return "vtos"
		},
		SetSpawnParms: func(e *quake.Entity) {
			calls["SetSpawnParms"]++
			e.Frags = 99
		},
	})

	SetOrigin(ent, quake.MakeVec3(1, 2, 3))
	if got, want := ent.Origin, (quake.Vec3{1, 2, 3}); got != want {
		t.Fatalf("SetOrigin hook origin=%v want=%v", got, want)
	}
	if got := Random(); got != 0.25 {
		t.Fatalf("Random hook=%v want=0.25", got)
	}
	if got := Spawn(); got != spawned {
		t.Fatalf("Spawn hook returned %p want %p", got, spawned)
	}
	if got := Find(nil, "classname", "target"); got != found {
		t.Fatalf("Find hook returned %p want %p", got, found)
	}
	if got := PrecacheSound("weapons/shot.wav"); got != "cached:weapons/shot.wav" {
		t.Fatalf("PrecacheSound hook=%q", got)
	}
	if got := Vtos(quake.MakeVec3(0, 1, 2)); got != "vtos" {
		t.Fatalf("Vtos hook=%q", got)
	}
	SetSpawnParms(ent)
	if got := ent.Frags; got != 99 {
		t.Fatalf("SetSpawnParms hook Frags=%v want=99", got)
	}

	for _, name := range []string{"SetOrigin", "Random", "Spawn", "Find", "PrecacheSound", "Vtos", "SetSpawnParms"} {
		if calls[name] != 1 {
			t.Fatalf("%s calls=%d want=1", name, calls[name])
		}
	}
}

func TestBackendDefaultsAndReset(t *testing.T) {
	ResetBackend()
	if got := Spawn(); got != nil {
		t.Fatalf("default Spawn=%p want nil", got)
	}
	if got := Random(); got != 0 {
		t.Fatalf("default Random=%v want 0", got)
	}
	if got := PrecacheModel("progs/player.mdl"); got != "progs/player.mdl" {
		t.Fatalf("default PrecacheModel=%q", got)
	}

	SetBackend(Backend{Random: func() float32 { return 0.5 }})
	if got := CRandom(); math.Abs(float64(got)) > 1e-6 {
		t.Fatalf("CRandom with hook=%v want 0", got)
	}

	ResetBackend()
	if got := Random(); got != 0 {
		t.Fatalf("Random after reset=%v want 0", got)
	}
}
