package quakego

import (
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake"
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake/engine"
)

var (
	SightEntity     *quake.Entity //qgo:sight_entity
	SightEntityTime float32       //qgo:sight_entity_time
)

func makevectorsfixed(ang quake.Vec3) {
	ang[0] *= -1
	engine.MakeVectors(ang)
}

func anglemod(v float32) float32 {
	for v >= 360 {
		v = v - 360
	}
	for v < 0 {
		v = v + 360
	}
	return v
}

func movetarget_f() {
	if Self.TargetName == "" {
		engine.ObjError("monster_movetarget: no targetname")
	}

	Self.Solid = SOLID_TRIGGER
	Self.Touch = t_movetarget
	engine.SetSize(Self, quake.MakeVec3(-8, -8, -8), quake.MakeVec3(8, 8, 8))
}

func path_corner() {
	movetarget_f()
}

func t_movetarget() {
	var temp *quake.Entity

	if Other.MoveTarget != Self {
		return
	}

	if Other.Enemy != nil {
		return
	}

	temp = Self
	Self = Other
	Other = temp

	if Self.ClassName == "monster_ogre" {
		engine.Sound(Self, int(CHAN_VOICE), "ogre/ogdrag.wav", 1, ATTN_IDLE)
	}

	Self.GoalEntity = engine.Find(World, "targetname", Other.Target)
	Self.MoveTarget = Self.GoalEntity
	Self.IdealYaw = engine.Vectoyaw(Self.GoalEntity.Origin.Sub(Self.Origin))
	if Self.MoveTarget == nil {
		Self.PauseTime = Time + 999999
		if Self.ThStand != nil {
			Self.ThStand()
		}
		return
	}
}

func range_func(targ *quake.Entity) float32 {
	var spot1, spot2 quake.Vec3
	var r float32
	spot1 = Self.Origin.Add(Self.ViewOfs)
	spot2 = targ.Origin.Add(targ.ViewOfs)

	r = engine.Vlen(spot1.Sub(spot2))
	if r < 120 {
		return RANGE_MELEE
	}
	if r < 500 {
		return RANGE_NEAR
	}
	if r < 1000 {
		return RANGE_MID
	}
	return RANGE_FAR
}

func visible(targ *quake.Entity) float32 {
	var spot1, spot2 quake.Vec3

	spot1 = Self.Origin.Add(Self.ViewOfs)
	spot2 = targ.Origin.Add(targ.ViewOfs)
	engine.Traceline(spot1, spot2, TRUE, Self)

	if TraceInOpen != 0 && TraceInWater != 0 {
		return FALSE
	}

	if TraceFraction == 1 {
		return TRUE
	}
	return FALSE
}

func infront(targ *quake.Entity) float32 {
	var vec quake.Vec3
	var dot float32

	makevectorsfixed(Self.Angles)
	vec = engine.Normalize(targ.Origin.Sub(Self.Origin))
	dot = vec.Dot(VForward)

	if dot > 0.3 {
		return TRUE
	}
	return FALSE
}

func HuntTarget() {
	Self.GoalEntity = Self.Enemy
	Self.Think = Self.ThRun
	Self.IdealYaw = engine.Vectoyaw(Self.Enemy.Origin.Sub(Self.Origin))
	Self.NextThink = Time + 0.1
	SUB_AttackFinished(1)
}

func SightSound() {
	if Self.ClassName == "enforcer" {
		var rsnd float32
		rsnd = engine.RInt(engine.Random() * 3)
		if rsnd == 1 {
			engine.Sound(Self, int(CHAN_VOICE), Self.Noise, 1, ATTN_NORM)
		} else if rsnd == 2 {
			engine.Sound(Self, int(CHAN_VOICE), Self.Noise1, 1, ATTN_NORM)
		} else if rsnd == 0 {
			engine.Sound(Self, int(CHAN_VOICE), Self.Noise2, 1, ATTN_NORM)
		} else {
			engine.Sound(Self, int(CHAN_VOICE), Self.Noise4, 1, ATTN_NORM)
		}
	} else {
		engine.Sound(Self, int(CHAN_VOICE), Self.Noise, 1, ATTN_NORM)
	}
}

func FoundTarget() {
	if Self.Enemy.ClassName == "player" {
		SightEntity = Self
		SightEntityTime = Time
	}

	Self.ShowHostile = Time + 1
	SightSound()
	HuntTarget()
}

func FindTarget() float32 {
	var client *quake.Entity
	var r float32

	if SightEntityTime >= Time-0.1 && (int(Self.SpawnFlags)&3) == 0 {
		client = SightEntity
		if client.Enemy == Self.Enemy {
			return TRUE
		}
	} else {
		client = engine.CheckClient()
		if client == nil {
			return FALSE
		}
	}

	if client == Self.Enemy {
		return FALSE
	}

	if (int(client.Flags) & FL_NOTARGET) != 0 {
		return FALSE
	}

	if (int(client.Items) & IT_INVISIBILITY) != 0 {
		return FALSE
	}

	r = range_func(client)
	if r == RANGE_FAR {
		return FALSE
	}

	if visible(client) == 0 {
		return FALSE
	}

	if r == RANGE_NEAR {
		if client.ShowHostile < Time && infront(client) == 0 {
			return FALSE
		}
	} else if r == RANGE_MID {
		if infront(client) == 0 {
			return FALSE
		}
	}

	Self.Enemy = client
	if Self.Enemy.ClassName != "player" {
		Self.Enemy = Self.Enemy.Enemy
		if Self.Enemy.ClassName != "player" {
			Self.Enemy = World
			return FALSE
		}
	}

	FoundTarget()
	return TRUE
}

func ai_forward(dist float32) {
	engine.WalkMove(Self.Angles[1], dist)
}

func ai_back(dist float32) {
	engine.WalkMove(Self.Angles[1]+180, dist)
}

func ai_pain(dist float32) {
	ai_back(dist)
}

func ai_painforward(dist float32) {
	engine.WalkMove(Self.IdealYaw, dist)
}

func ai_walk(dist float32) {
	Movedist = dist

	if FindTarget() != 0 {
		return
	}

	engine.MoveToGoal(dist)
}

func ai_stand() {
	if FindTarget() != 0 {
		return
	}

	if Time > Self.PauseTime {
		if Self.ThWalk != nil {
			Self.ThWalk()
		}
		return
	}
}

func ai_turn() {
	if FindTarget() != 0 {
		return
	}

	engine.ChangeYaw()
}

func ChooseTurn(dest3 quake.Vec3) {
	var dir, newdir quake.Vec3

	dir = Self.Origin.Sub(dest3)

	newdir[0] = TracePlaneNormal[1]
	newdir[1] = 0 - TracePlaneNormal[0]
	newdir[2] = 0

	if dir.Dot(newdir) > 0 {
		dir[0] = 0 - TracePlaneNormal[1]
		dir[1] = TracePlaneNormal[0]
	} else {
		dir[0] = TracePlaneNormal[1]
		dir[1] = 0 - TracePlaneNormal[0]
	}

	dir[2] = 0
	Self.IdealYaw = engine.Vectoyaw(dir)
}

func FacingIdeal() float32 {
	var delta float32

	delta = anglemod(Self.Angles[1] - Self.IdealYaw)
	if delta > 45 && delta < 315 {
		return FALSE
	}
	return TRUE
}

func CheckAnyAttack() float32 {
	if EnemyVisible == 0 {
		return FALSE
	}
	if Self.ClassName == "monster_army" {
		return SoldierCheckAttack()
	}
	if Self.ClassName == "monster_ogre" {
		return OgreCheckAttack()
	}
	if Self.ClassName == "monster_shambler" {
		return ShamCheckAttack()
	}
	if Self.ClassName == "monster_demon1" {
		return DemonCheckAttack()
	}
	if Self.ClassName == "monster_dog" {
		return DogCheckAttack()
	}
	if Self.ClassName == "monster_wizard" {
		return WizardCheckAttack()
	}
	return CheckAttack()
}

func ai_run_melee() {
	Self.IdealYaw = EnemyYaw
	engine.ChangeYaw()

	if FacingIdeal() != 0 {
		if Self.ThMelee != nil {
			Self.ThMelee()
		}
		Self.AttackState = AS_STRAIGHT
	}
}

func ai_run_missile() {
	Self.IdealYaw = EnemyYaw
	engine.ChangeYaw()
	if FacingIdeal() != 0 {
		if Self.ThMissile != nil {
			Self.ThMissile()
		}
		Self.AttackState = AS_STRAIGHT
	}
}

func ai_run_slide() {
	var ofs float32

	Self.IdealYaw = EnemyYaw
	engine.ChangeYaw()
	if Self.Lefty != 0 {
		ofs = 90
	} else {
		ofs = -90
	}

	if engine.WalkMove(Self.IdealYaw+ofs, Movedist) != 0 {
		return
	}

	Self.Lefty = 1 - Self.Lefty

	engine.WalkMove(Self.IdealYaw-ofs, Movedist)
}

func ai_run(dist float32) {
	Movedist = dist
	if Self.Enemy.Health <= 0 {
		Self.Enemy = World
		if Self.OldEnemy.Health > 0 {
			Self.Enemy = Self.OldEnemy
			HuntTarget()
		} else {
			if Self.MoveTarget != nil {
				if Self.ThWalk != nil {
					Self.ThWalk()
				}
			} else {
				if Self.ThStand != nil {
					Self.ThStand()
				}
			}
			return
		}
	}

	Self.ShowHostile = Time + 1

	EnemyVisible = visible(Self.Enemy)
	if EnemyVisible != 0 {
		Self.SearchTime = Time + 5
	}

	if Coop != 0 && Self.SearchTime < Time {
		if FindTarget() != 0 {
			return
		}
	}

	EnemyInfront = infront(Self.Enemy)
	EnemyRange = range_func(Self.Enemy)
	EnemyYaw = engine.Vectoyaw(Self.Enemy.Origin.Sub(Self.Origin))

	if Self.AttackState == AS_MISSILE {
		ai_run_missile()
		return
	}
	if Self.AttackState == AS_MELEE {
		ai_run_melee()
		return
	}

	if CheckAnyAttack() != 0 {
		return
	}

	if Self.AttackState == AS_SLIDING {
		ai_run_slide()
		return
	}

	engine.MoveToGoal(dist)
}
