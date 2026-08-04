// This file belongs to the Debug subsystem: debug telemetry, trigger touch debugging, and multiplayer debug logging.

package server

import (
	"fmt"

	"github.com/darkliquid/ironwail-go/internal/console"
	"github.com/darkliquid/ironwail-go/internal/qc"
)

// debugTriggerEnabled returns true when sv_debug_trigger is set. When enabled,
// the server prints trigger/entity activation info to the in-game console so
// players can see exactly which triggers fire, what they target, and whether
// the target's use function is invoked.
func (s *Server) debugTriggerEnabled() bool {
	return debugTriggerCVar != nil && debugTriggerCVar.Bool()
}

// qcEntString resolves a QCVM string field (by field offset) for an entity to
// a Go string, returning "" if the entity or string is invalid.
func (s *Server) qcEntString(entNum int, fieldOfs int) string {
	if s.QCVM == nil || entNum < 0 || entNum >= s.QCVM.NumEdicts {
		return ""
	}
	return s.QCVM.String(s.QCVM.EString(entNum, fieldOfs))
}

// qcFuncName resolves a QC function table index to a human-readable name
// like "plat_go_down[#42]" or "#0" if the index is invalid.
func (s *Server) qcFuncName(funcIdx int32) string {
	if s.QCVM == nil || s.DebugTelemetry == nil {
		return fmt.Sprintf("#%d", funcIdx)
	}
	return s.DebugTelemetry.FormatQCFunction(s.QCVM, funcIdx)
}

// debugTriggerTouch logs a trigger touch dispatch to the console. Called from
// touchLinks when a SOLID_TRIGGER entity's touch callback is about to fire.
func (s *Server) debugTriggerTouch(source string, touch, other *Edict) {
	if !s.debugTriggerEnabled() {
		return
	}
	touchNum := s.NumForEdict(touch)
	otherNum := s.NumForEdict(other)

	touchClass := s.qcEntString(touchNum, qc.EntFieldClassName)
	touchTargetName := s.qcEntString(touchNum, qc.EntFieldTargetName)
	touchTarget := s.qcEntString(touchNum, qc.EntFieldTarget)
	touchUseFn := s.qcFuncName(touch.Use(s))
	touchThinkFn := s.qcFuncName(touch.Think(s))

	otherClass := s.qcEntString(otherNum, qc.EntFieldClassName)

	// Read extension fields via accessor methods (cached offsets)
	thCheckAttack := touch.ThCheckAttack(s)
	customFlags := int32(touch.CustomFlags(s))
	state := touch.State(s)
	wait := touch.Wait(s)
	nextThink := touch.NextThink(s)
	enemyNum := touch.Enemy(s)

	// Resolve enemy classname for context
	enemyClass := ""
	if enemyNum > 0 {
		enemyClass = s.qcEntString(int(enemyNum), qc.EntFieldClassName)
	}

	otherOrg := other.Origin(s)
	console.Printf("trigger [%s] ent=%d classname=%q targetname=%q target=%q touch_fn=%d use_fn=%s think_fn=%s\n",
		source, touchNum, touchClass, touchTargetName, touchTarget, touch.Touch(s), touchUseFn, touchThinkFn)
	console.Printf("  → other ent=%d classname=%q origin=(%.1f %.1f %.1f)\n",
		otherNum, otherClass, otherOrg[0], otherOrg[1], otherOrg[2])
	console.Printf("  → th_checkattack=%d(%s) customflags=%d state=%.1f wait=%.3f nextthink=%.3f time=%.3f enemy=%d(%q)\n",
		thCheckAttack, s.qcFuncName(thCheckAttack), customFlags, state, wait, nextThink, s.Time, enemyNum, enemyClass)

	if touchTarget != "" {
		console.Printf("  → target %q: searching entities...\n", touchTarget)
	}
}

// debugTriggerFind logs a Find hook call when searching for entities by
// targetname. This shows whether SUB_UseTargets can find its targets.
func (s *Server) debugTriggerFind(fieldOfs int, match string, result int) {
	if !s.debugTriggerEnabled() {
		return
	}
	fieldName := "targetname"
	if fieldOfs != qc.EntFieldTargetName {
		fieldName = fmt.Sprintf("ofs_%d", fieldOfs)
	}
	if result == 0 {
		console.Printf("  find(%s=%q) → NOT FOUND\n", fieldName, match)
	} else {
		ent := s.EdictNum(result)
		cls := s.qcEntString(result, qc.EntFieldClassName)
		useFn := ""
		thinkFn := ""
		nextThink := float32(0)
		ltime := float32(0)
		vel := [3]float32{}
		movetype := ""
		if ent != nil {
			useFn = s.qcFuncName(ent.Use(s))
			thinkFn = s.qcFuncName(ent.Think(s))
			nextThink = ent.NextThink(s)
			ltime = ent.LTime(s)
			vel = ent.Velocity(s)
			movetype = fmt.Sprintf("mt=%d", int(ent.MoveType(s)))
		}
		console.Printf("  find(%s=%q) → ent=%d classname=%q %s use_fn=%s think_fn=%s nextthink=%.3f ltime=%.3f velocity=(%.1f %.1f %.1f)\n",
			fieldName, match, result, cls, movetype, useFn, thinkFn,
			nextThink, ltime, vel[0], vel[1], vel[2])
	}
}
