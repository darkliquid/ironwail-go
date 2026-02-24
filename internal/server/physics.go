package server

import (
	"math"
)

// CheckVelocity ensures entity velocity is within valid bounds.
func (s *Server) CheckVelocity(ent *Edict) {
	for i := 0; i < 3; i++ {
		if math.IsNaN(float64(ent.Vars.Velocity[i])) {
			ent.Vars.Velocity[i] = 0
		}
		if math.IsNaN(float64(ent.Vars.Origin[i])) {
			ent.Vars.Origin[i] = 0
		}
		if ent.Vars.Velocity[i] > s.MaxVelocity {
			ent.Vars.Velocity[i] = s.MaxVelocity
		} else if ent.Vars.Velocity[i] < -s.MaxVelocity {
			ent.Vars.Velocity[i] = -s.MaxVelocity
		}
	}
}

// RunThink executes the entity's think function if its nextthink time has been reached.
func (s *Server) RunThink(ent *Edict) bool {
	thinkTime := ent.Vars.NextThink
	if thinkTime <= 0 || thinkTime > s.Time+s.FrameTime {
		return true
	}

	if thinkTime < s.Time {
		thinkTime = s.Time
	}

	ent.OldThinkTime = thinkTime
	ent.OldFrame = ent.Vars.Frame
	ent.Vars.NextThink = 0

	s.QCVM.Time = float64(thinkTime)
	s.QCVM.SetGlobal("self", s.NumForEdict(ent))
	s.QCVM.SetGlobal("other", 0)
	if ent.Vars.Think != 0 {
		s.QCVM.ExecuteFunction(int(ent.Vars.Think))
	}

	return !ent.Free
}

// Impact runs touch functions for two entities that have collided.
func (s *Server) Impact(e1, e2 *Edict) {
	oldSelf := s.QCVM.GetGlobalInt("self")
	oldOther := s.QCVM.GetGlobalInt("other")

	s.QCVM.Time = float64(s.Time)

	if e1.Vars.Touch != 0 && e1.Vars.Solid != float32(SolidNot) {
		s.QCVM.SetGlobal("self", s.NumForEdict(e1))
		s.QCVM.SetGlobal("other", s.NumForEdict(e2))
		s.QCVM.ExecuteFunction(int(e1.Vars.Touch))
	}

	if e2.Vars.Touch != 0 && e2.Vars.Solid != float32(SolidNot) {
		s.QCVM.SetGlobal("self", s.NumForEdict(e2))
		s.QCVM.SetGlobal("other", s.NumForEdict(e1))
		s.QCVM.ExecuteFunction(int(e2.Vars.Touch))
	}

	s.QCVM.SetGlobalInt("self", oldSelf)
	s.QCVM.SetGlobalInt("other", oldOther)
}

// ClipVelocity slides off an impacting surface.
func ClipVelocity(in, normal [3]float32, overbounce float32) ([3]float32, int) {
	var backoff float32
	var change float32
	blocked := 0

	if normal[2] > 0 {
		blocked |= 1
	}
	if normal[2] == 0 {
		blocked |= 2
	}

	backoff = float32(float64(in[0])*float64(normal[0])+
		float64(in[1])*float64(normal[1])+
		float64(in[2])*float64(normal[2])) * overbounce

	var out [3]float32
	for i := 0; i < 3; i++ {
		change = normal[i] * backoff
		out[i] = in[i] - change
		if out[i] > -StopEpsilon && out[i] < StopEpsilon {
			out[i] = 0
		}
	}

	return out, blocked
}









