package quakego

import (
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake"
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake/engine"
)

type combatEntity quake.Entity

func asCombatEntity(ent *quake.Entity) *combatEntity {
	return (*combatEntity)(ent)
}

func (ce *combatEntity) entity() *quake.Entity {
	return (*quake.Entity)(ce)
}

func CanDamage(targ, inflictor *quake.Entity) float32 {
	return asCombatEntity(targ).canDamage(inflictor)
}

func (ce *combatEntity) canDamage(inflictor *quake.Entity) float32 {
	targ := ce.entity()

	if targ.MoveType == MOVETYPE_PUSH {
		engine.Traceline(inflictor.Origin, targ.AbsMin.Add(targ.AbsMax).Mul(0.5), TRUE, Self)
		if TraceFraction == 1 {
			return TRUE
		}
		if TraceEnt == targ {
			return TRUE
		}
		return FALSE
	}

	engine.Traceline(inflictor.Origin, targ.Origin, TRUE, Self)
	if TraceFraction == 1 {
		return TRUE
	}

	engine.Traceline(inflictor.Origin, targ.Origin.Add(quake.MakeVec3(15, 15, 0)), TRUE, Self)
	if TraceFraction == 1 {
		return TRUE
	}

	engine.Traceline(inflictor.Origin, targ.Origin.Add(quake.MakeVec3(-15, -15, 0)), TRUE, Self)
	if TraceFraction == 1 {
		return TRUE
	}

	engine.Traceline(inflictor.Origin, targ.Origin.Add(quake.MakeVec3(-15, 15, 0)), TRUE, Self)
	if TraceFraction == 1 {
		return TRUE
	}

	engine.Traceline(inflictor.Origin, targ.Origin.Add(quake.MakeVec3(15, -15, 0)), TRUE, Self)
	if TraceFraction == 1 {
		return TRUE
	}

	return FALSE
}

func Killed(targ, attacker *quake.Entity) {
	asCombatEntity(targ).killed(attacker)
}

func (ce *combatEntity) killed(attacker *quake.Entity) {
	targ := ce.entity()

	oself := Self
	Self = targ

	if Self.Health < -99 {
		Self.Health = -99
	}

	if Self.MoveType == MOVETYPE_PUSH || Self.MoveType == MOVETYPE_NONE {
		if Self.ThDie != nil {
			Self.ThDie()
		}
		Self = oself
		return
	}

	Self.Enemy = attacker
	if (int(Self.Flags) & FL_MONSTER) != 0 {
		KilledMonsters = KilledMonsters + 1
		engine.WriteByte(MSG_ALL, float32(SVC_KILLEDMONSTER))
	}

	ClientObituary(Self, attacker)

	if Self.ThDie != nil {
		Self.ThDie()
	}
	Self = oself
}

func T_Damage(targ, inflictor, attacker *quake.Entity, damage float32) {
	asCombatEntity(targ).takeDamage(inflictor, attacker, damage)
}

func (ce *combatEntity) takeDamage(inflictor, attacker *quake.Entity, damage float32) {
	targ := ce.entity()

	var save, take float32
	var dir, knockback quake.Vec3

	if targ.TakeDamage == 0 {
		return
	}

	// check for armor
	save = engine.Ceil(targ.ArmorValue * 0.5)
	if save >= damage {
		save = damage - 1
	}
	if save < 0 {
		save = 0
	}

	take = damage - save
	targ.ArmorValue = targ.ArmorValue - save

	if (int(targ.Flags) & FL_CLIENT) != 0 {
		targ.DmgTake = targ.DmgTake + take
		targ.DmgSave = targ.DmgSave + save
		targ.DmgInflictor = inflictor
	}

	if (inflictor != World) && (targ.MoveType != MOVETYPE_PUSH) && (targ.MoveType != MOVETYPE_NONE) {
		dir = targ.Origin.Add(targ.Mins.Add(targ.Maxs).Mul(0.5)).Sub(inflictor.Origin.Add(inflictor.Mins.Add(inflictor.Maxs).Mul(0.5)))
		dir = engine.Normalize(dir)
		knockback = dir.Mul(damage * 10)
		targ.Velocity = targ.Velocity.Add(knockback)
	}

	if (int(targ.Flags) & FL_GODMODE) != 0 {
		return
	}

	targ.Health = targ.Health - take

	if targ.Health <= 0 {
		ce.killed(attacker)
		return
	}

	oldSelf := Self
	Self = targ
	if (int(Self.Flags)&FL_MONSTER) != 0 && attacker != World {
		if (Self.Enemy != attacker) && (attacker != Self) {
			if Self.Enemy.ClassName == "player" {
				Self.OldEnemy = Self.Enemy
			}
			Self.Enemy = attacker
			FoundTarget()
		}
	}

	if Self.ThPain != nil {
		Self.ThPain(attacker, take)
	}

	Self = oldSelf
}

func T_RadiusDamage(inflictor, attacker *quake.Entity, damage float32, ignore *quake.Entity) {
	var points float32
	var head *quake.Entity
	var org quake.Vec3

	head = engine.FindRadius(inflictor.Origin, damage+40)

	for head != nil {
		if head != ignore {
			if head.TakeDamage != 0 {
				org = head.Origin.Add(head.Mins.Add(head.Maxs).Mul(0.5))
				points = 0.5 * engine.Vlen(inflictor.Origin.Sub(org))

				if points < 0 {
					points = 0
				}

				points = damage - points

				if head == attacker {
					points = points * 0.5
				}

				if points > 0 {
					if CanDamage(head, inflictor) != 0 {
						if head.ClassName == "monster_shambler" {
							T_Damage(head, inflictor, attacker, points*0.5)
						} else {
							T_Damage(head, inflictor, attacker, points)
						}
					}
				}
			}
		}
		head = head.Chain
	}
}

func T_BeamDamage(attacker *quake.Entity, damage float32) {
	var points float32
	var head *quake.Entity

	head = engine.FindRadius(attacker.Origin, damage+40)

	for head != nil {
		if head.TakeDamage != 0 {
			points = 0.5 * engine.Vlen(attacker.Origin.Sub(head.Origin))

			if points < 0 {
				points = 0
			}

			points = damage - points

			if head == attacker {
				points = points * 0.5
			}

			if points > 0 {
				if CanDamage(head, attacker) != 0 {
					T_Damage(head, attacker, attacker, points)
				}
			}
		}
		head = head.Chain
	}
}
