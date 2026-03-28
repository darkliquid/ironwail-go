package quakego

import (
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake"
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake/engine"
)

var (
	EnemyVisible float32
	EnemyInfront float32
	EnemyRange   float32
	EnemyYaw     float32
)

func knight_attack() {
	len := engine.Vlen(Self.Enemy.Origin.Add(Self.Enemy.ViewOfs).Sub(Self.Origin.Add(Self.ViewOfs)))

	if len < 80 {
		knight_atk1()
	} else {
		knight_runatk1()
	}
}

func CheckAttack() float32 {
	var spot1, spot2 quake.Vec3
	var targ *quake.Entity
	var chance float32

	targ = Self.Enemy

	spot1 = Self.Origin.Add(Self.ViewOfs)
	spot2 = targ.Origin.Add(targ.ViewOfs)

	engine.Traceline(spot1, spot2, FALSE, Self)

	if TraceEnt != targ {
		return FALSE
	}

	if TraceInOpen != 0 && TraceInWater != 0 {
		return FALSE
	}

	if EnemyRange == RANGE_MELEE {
		if Self.ThMelee != nil {
			if Self.ClassName == "monster_knight" {
				knight_attack()
			} else {
				Self.ThMelee()
			}
			return TRUE
		}
	}

	if Self.ThMissile == nil {
		return FALSE
	}

	if Time < Self.AttackFinished {
		return FALSE
	}

	if EnemyRange == RANGE_FAR {
		return FALSE
	}

	if EnemyRange == RANGE_MELEE {
		chance = 0.9
		Self.AttackFinished = 0
	} else if EnemyRange == RANGE_NEAR {
		if Self.ThMelee != nil {
			chance = 0.2
		} else {
			chance = 0.4
		}
	} else if EnemyRange == RANGE_MID {
		if Self.ThMelee != nil {
			chance = 0.05
		} else {
			chance = 0.1
		}
	} else {
		chance = 0
	}

	if engine.Random() < chance {
		Self.ThMissile()
		SUB_AttackFinished(2 * engine.Random())
		return TRUE
	}

	return FALSE
}

func ai_face() {
	Self.IdealYaw = engine.Vectoyaw(Self.Enemy.Origin.Sub(Self.Origin))
	engine.ChangeYaw()
}

func ai_charge(d float32) {
	ai_face()
	engine.MoveToGoal(d)
}

func ai_charge_side() {
	var dtemp quake.Vec3
	var heading float32

	Self.IdealYaw = engine.Vectoyaw(Self.Enemy.Origin.Sub(Self.Origin))
	engine.ChangeYaw()

	engine.MakeVectors(Self.Angles) // use MakeVectors instead of makevectorsfixed
	dtemp = Self.Enemy.Origin.Sub(VRight.Mul(30))
	heading = engine.Vectoyaw(dtemp.Sub(Self.Origin))

	engine.WalkMove(heading, 20)
}

func ai_melee() {
	var delta quake.Vec3
	var ldmg float32

	if Self.Enemy == nil {
		return
	}

	delta = Self.Enemy.Origin.Sub(Self.Origin)

	if engine.Vlen(delta) > 60 {
		return
	}

	ldmg = (engine.Random() + engine.Random() + engine.Random()) * 3
	T_Damage(Self.Enemy, Self, Self, ldmg)
}

func ai_melee_side() {
	var delta quake.Vec3
	var ldmg float32

	if Self.Enemy == nil {
		return
	}

	ai_charge_side()

	delta = Self.Enemy.Origin.Sub(Self.Origin)

	if engine.Vlen(delta) > 60 {
		return
	}

	if CanDamage(Self.Enemy, Self) == 0 {
		return
	}

	ldmg = (engine.Random() + engine.Random() + engine.Random()) * 3
	T_Damage(Self.Enemy, Self, Self, ldmg)
}

func SoldierCheckAttack() float32 {
	var spot1, spot2 quake.Vec3
	var targ *quake.Entity
	var chance float32

	targ = Self.Enemy

	spot1 = Self.Origin.Add(Self.ViewOfs)
	spot2 = targ.Origin.Add(targ.ViewOfs)

	engine.Traceline(spot1, spot2, FALSE, Self)

	if TraceInOpen != 0 && TraceInWater != 0 {
		return FALSE
	}

	if TraceEnt != targ {
		return FALSE
	}

	if Time < Self.AttackFinished {
		return FALSE
	}

	if EnemyRange == RANGE_FAR {
		return FALSE
	}

	if EnemyRange == RANGE_MELEE {
		chance = 0.9
	} else if EnemyRange == RANGE_NEAR {
		chance = 0.4
	} else if EnemyRange == RANGE_MID {
		chance = 0.05
	} else {
		chance = 0
	}

	if engine.Random() < chance {
		Self.ThMissile()
		SUB_AttackFinished(1 + engine.Random())

		if engine.Random() < 0.3 {
			if Self.Lefty != 0 {
				Self.Lefty = 0
			} else {
				Self.Lefty = 1
			}
		}

		return TRUE
	}

	return FALSE
}

func ShamCheckAttack() float32 {
	var spot1, spot2 quake.Vec3
	var targ *quake.Entity

	if EnemyRange == RANGE_MELEE {
		if CanDamage(Self.Enemy, Self) != 0 {
			Self.AttackState = AS_MELEE
			return TRUE
		}
	}

	if Time < Self.AttackFinished {
		return FALSE
	}

	if EnemyVisible == 0 {
		return FALSE
	}

	targ = Self.Enemy

	spot1 = Self.Origin.Add(Self.ViewOfs)
	spot2 = targ.Origin.Add(targ.ViewOfs)

	if engine.Vlen(spot1.Sub(spot2)) > 600 {
		return FALSE
	}

	engine.Traceline(spot1, spot2, FALSE, Self)

	if TraceInOpen != 0 && TraceInWater != 0 {
		return FALSE
	}

	if TraceEnt != targ {
		return FALSE
	}

	if EnemyRange == RANGE_FAR {
		return FALSE
	}

	Self.AttackState = AS_MISSILE
	SUB_AttackFinished(2 + 2*engine.Random())
	return TRUE
}

func OgreCheckAttack() float32 {
	var spot1, spot2 quake.Vec3
	var targ *quake.Entity

	if EnemyRange == RANGE_MELEE {
		if CanDamage(Self.Enemy, Self) != 0 {
			Self.AttackState = AS_MELEE
			return TRUE
		}
	}

	if Time < Self.AttackFinished {
		return FALSE
	}

	if EnemyVisible == 0 {
		return FALSE
	}

	targ = Self.Enemy

	spot1 = Self.Origin.Add(Self.ViewOfs)
	spot2 = targ.Origin.Add(targ.ViewOfs)

	engine.Traceline(spot1, spot2, FALSE, Self)

	if TraceInOpen != 0 && TraceInWater != 0 {
		return FALSE
	}

	if TraceEnt != targ {
		return FALSE
	}

	if Time < Self.AttackFinished {
		return FALSE
	}

	if EnemyRange == RANGE_FAR {
		return FALSE
	}

	Self.AttackState = AS_MISSILE
	SUB_AttackFinished(1 + 2*engine.Random())
	return TRUE
}
