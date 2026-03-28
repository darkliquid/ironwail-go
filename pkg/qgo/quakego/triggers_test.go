package quakego

import (
	"testing"

	"github.com/darkliquid/ironwail-go/pkg/qgo/quake"
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake/engine"
)

func resetTriggerGlobals() {
	Self = nil
	Other = nil
	World = nil
	Activator = nil
	DamageAttacker = nil
	MsgEntity = nil
	FoundSecrets = 0
	ForceRetouch = 0
	Time = 0
	VForward = quake.MakeVec3(0, 0, 0)
	StringNull = ""
}

func TestTriggerMultipleSetupTouchPath(t *testing.T) {
	resetTriggerGlobals()
	engine.ResetBackend()
	defer engine.ResetBackend()

	self := &quake.Entity{
		Model:      "progs/trigger.mdl",
		Wait:       0,
		Health:     0,
		SpawnFlags: 0,
		Sounds:     3,
	}
	Self = self

	var setModelEntity *quake.Entity
	var setModelValue string
	engine.SetBackend(engine.Backend{
		SetModel: func(e *quake.Entity, m string) {
			setModelEntity = e
			setModelValue = m
		},
		PrecacheSound: func(s string) string { return s },
	})

	trigger_multiple()

	if self.Use == nil {
		t.Fatalf("Use not set")
	}
	if self.Touch == nil {
		t.Fatalf("Touch not set for touch trigger")
	}
	if self.Wait != 0.2 {
		t.Fatalf("Wait=%v want 0.2", self.Wait)
	}
	if self.Solid != SOLID_TRIGGER {
		t.Fatalf("Solid=%v want SOLID_TRIGGER", self.Solid)
	}
	if self.MoveType != MOVETYPE_NONE {
		t.Fatalf("MoveType=%v want MOVETYPE_NONE", self.MoveType)
	}
	if self.Model != StringNull {
		t.Fatalf("Model=%q want StringNull", self.Model)
	}
	if setModelEntity != self || setModelValue != "progs/trigger.mdl" {
		t.Fatalf("SetModel called with (%p,%q), want (%p,%q)", setModelEntity, setModelValue, self, "progs/trigger.mdl")
	}
	if self.Noise != "misc/trigger1.wav" {
		t.Fatalf("Noise=%q want misc/trigger1.wav", self.Noise)
	}

	Activator = nil
	Other = &quake.Entity{ClassName: "player"}
	World = &quake.Entity{Model: "maps/start.bsp"}
	Time = 3
	self.Touch()
	if Activator != Other {
		t.Fatalf("touch callback did not dispatch to trigger path")
	}
}

func TestMultiTouchDispatchesToTrigger(t *testing.T) {
	resetTriggerGlobals()
	engine.ResetBackend()
	defer engine.ResetBackend()

	self := &quake.Entity{
		ClassName: "trigger_multiple",
		Wait:      0.5,
		MoveDir:   quake.MakeVec3(1, 0, 0),
	}
	other := &quake.Entity{
		ClassName: "player",
		Angles:    quake.MakeVec3(0, 0, 0),
	}
	Self = self
	Other = other
	World = &quake.Entity{Model: "maps/start.bsp"}
	Time = 10

	engine.SetBackend(engine.Backend{
		MakeVectors: func(ang quake.Vec3) { VForward = quake.MakeVec3(1, 0, 0) },
	})

	multi_touch()

	if Activator != other {
		t.Fatalf("Activator=%p want %p", Activator, other)
	}
	if self.TakeDamage != DAMAGE_NO {
		t.Fatalf("TakeDamage=%v want DAMAGE_NO", self.TakeDamage)
	}
	if self.NextThink != 10.5 {
		t.Fatalf("NextThink=%v want 10.5", self.NextThink)
	}
	if self.Think == nil {
		t.Fatalf("Think not set after touch dispatch")
	}
}

func TestTriggerOnceDelegatesToMultiple(t *testing.T) {
	resetTriggerGlobals()
	engine.ResetBackend()
	defer engine.ResetBackend()

	self := &quake.Entity{
		Model:      "progs/trigger.mdl",
		SpawnFlags: 0,
	}
	Self = self
	engine.SetBackend(engine.Backend{
		SetModel:      func(*quake.Entity, string) {},
		PrecacheSound: func(s string) string { return s },
	})

	trigger_once()

	if self.Wait != -1 {
		t.Fatalf("Wait=%v want -1", self.Wait)
	}
	if self.Use == nil {
		t.Fatalf("Use not set")
	}
	if self.Touch == nil {
		t.Fatalf("Touch not set")
	}
}

func TestMultiUseUsesActivator(t *testing.T) {
	resetTriggerGlobals()
	engine.ResetBackend()
	defer engine.ResetBackend()

	self := &quake.Entity{
		ClassName: "trigger_multiple",
		Wait:      1,
	}
	activator := &quake.Entity{ClassName: "player"}
	Self = self
	Activator = activator
	World = &quake.Entity{Model: "maps/start.bsp"}
	Time = 2

	multi_use()

	if Other != activator {
		t.Fatalf("Other=%p want %p", Other, activator)
	}
	if self.NextThink != 3 {
		t.Fatalf("NextThink=%v want 3", self.NextThink)
	}
	if self.Think == nil {
		t.Fatalf("Think not set by use path")
	}
}

func TestTriggerMultipleUseThinkBridgeThroughEntityCallbacks(t *testing.T) {
	resetTriggerGlobals()
	engine.ResetBackend()
	defer engine.ResetBackend()

	self := &quake.Entity{
		ClassName: "trigger_multiple",
		Model:     "progs/trigger.mdl",
		Wait:      1,
		Health:    10,
	}
	activator := &quake.Entity{ClassName: "player"}
	Self = self
	Activator = activator
	World = &quake.Entity{Model: "maps/start.bsp"}
	Time = 4

	engine.SetBackend(engine.Backend{
		SetModel:      func(*quake.Entity, string) {},
		PrecacheSound: func(s string) string { return s },
	})

	trigger_multiple()

	if self.Use == nil {
		t.Fatalf("Use not assigned by trigger_multiple")
	}
	self.Use()

	if Other != activator {
		t.Fatalf("Other=%p want %p", Other, activator)
	}
	if self.Think == nil {
		t.Fatalf("Think not assigned after Use callback")
	}
	if self.NextThink != 5 {
		t.Fatalf("NextThink=%v want 5", self.NextThink)
	}
	if self.TakeDamage != DAMAGE_NO {
		t.Fatalf("TakeDamage=%v want DAMAGE_NO before wait think", self.TakeDamage)
	}

	self.Think()

	if self.Health != 10 {
		t.Fatalf("Health=%v want 10 after wait think", self.Health)
	}
	if self.TakeDamage != DAMAGE_YES {
		t.Fatalf("TakeDamage=%v want DAMAGE_YES after wait think", self.TakeDamage)
	}
	if self.Solid != SOLID_BBOX {
		t.Fatalf("Solid=%v want SOLID_BBOX after wait think", self.Solid)
	}
}
