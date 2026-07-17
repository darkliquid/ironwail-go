package quakego

import (
	"testing"

	"quake"
)

func TestWSetCurrentAmmoSelectsBestWeaponWhenUnset(t *testing.T) {
	Self = &quake.Entity{
		Items:      IT_SHOTGUN | IT_AXE,
		AmmoShells: 25,
	}
	t.Cleanup(func() { Self = nil })

	W_SetCurrentAmmo()

	if got := int(Self.Weapon); got != IT_SHOTGUN {
		t.Fatalf("weapon = %d, want shotgun", got)
	}
	if got := Self.WeaponModel; got != "progs/v_shot.mdl" {
		t.Fatalf("weapon model = %q, want shotgun viewmodel", got)
	}
	if got := Self.CurrentAmmo; got != 25 {
		t.Fatalf("current ammo = %v, want 25 shells", got)
	}
}
