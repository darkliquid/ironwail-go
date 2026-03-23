package quakego

import (
	"github.com/ironwail/ironwail-go/pkg/qgo/quake"
	"github.com/ironwail/ironwail-go/pkg/qgo/quake/engine"
)

// Prototyped elsewhere
var (
	SpawnBlood       func(org, vel quake.Vec3, damage float32)
)

func W_Precache() {
	engine.PrecacheSound("weapons/r_exp3.wav")
	engine.PrecacheSound("weapons/rocket1i.wav")
	engine.PrecacheSound("weapons/sgun1.wav")
	engine.PrecacheSound("weapons/guncock.wav")
	engine.PrecacheSound("weapons/ric1.wav")
	engine.PrecacheSound("weapons/ric2.wav")
	engine.PrecacheSound("weapons/ric3.wav")
	engine.PrecacheSound("weapons/spike2.wav")
	engine.PrecacheSound("weapons/tink1.wav")
	engine.PrecacheSound("weapons/grenade.wav")
	engine.PrecacheSound("weapons/bounce.wav")
	engine.PrecacheSound("weapons/shotgn2.wav")
}

func crandom() float32 {
	return 2 * (engine.Random() - 0.5)
}

func W_FireAxe() {
	var source, org quake.Vec3

	engine.MakeVectors(Self.VAngle)
	source = Self.Origin.Add(quake.MakeVec3(0, 0, 16))
	engine.Traceline(source, source.Add(VForward.Mul(64)), FALSE, Self)
	if TraceFraction == 1.0 {
		return
	}

	org = TraceEndPos.Sub(VForward.Mul(4))

	if TraceEnt.TakeDamage != 0 {
		TraceEnt.AxHitMe = 1
		SpawnBlood(org, quake.MakeVec3(0, 0, 0), 20)
		T_Damage(TraceEnt, Self, Self, 20)
	} else {
		engine.Sound(Self, int(CHAN_WEAPON), "player/axhit2.wav", 1, ATTN_NORM)
		engine.WriteByte(MSG_BROADCAST, float32(SVC_TEMPENTITY))
		engine.WriteByte(MSG_BROADCAST, float32(TE_GUNSHOT))
		engine.WriteCoord(MSG_BROADCAST, org[0])
		engine.WriteCoord(MSG_BROADCAST, org[1])
		engine.WriteCoord(MSG_BROADCAST, org[2])
	}
}

func wall_velocity() quake.Vec3 {
	var vel quake.Vec3

	vel = engine.Normalize(Self.Velocity)
	vel = engine.Normalize(vel.Add(VUp.Mul(engine.Random() - 0.5)).Add(VRight.Mul(engine.Random() - 0.5)))
	vel = vel.Add(TracePlaneNormal.Mul(2))
	vel = vel.Mul(200)

	return vel
}

func SpawnMeatSpray(org, vel quake.Vec3) {
	missile := engine.Spawn()
	missile.Owner = Self
	missile.MoveType = MOVETYPE_BOUNCE
	missile.Solid = SOLID_NOT

	makevectorsfixed(Self.Angles)

	missile.Velocity = vel
	missile.Velocity[2] = missile.Velocity[2] + 250 + 50*engine.Random()

	missile.AVelocity = quake.MakeVec3(3000, 1000, 2000)

	missile.NextThink = Time + 1
	missile.Think = SUB_Remove

	engine.SetModel(missile, "progs/zom_gib.mdl")
	engine.SetSize(missile, quake.MakeVec3(0, 0, 0), quake.MakeVec3(0, 0, 0))
	engine.SetOrigin(missile, org)
}

func SpawnBlood_impl(org, vel quake.Vec3, damage float32) {
	engine.Particle(org, vel.Mul(0.1), 73, damage*2)
}

func init() {
	SpawnBlood = SpawnBlood_impl
}

func spawn_touchblood(damage float32) {
	var vel quake.Vec3

	vel = wall_velocity().Mul(0.2)
	SpawnBlood(Self.Origin.Add(vel.Mul(0.01)), vel, damage)
}

func SpawnChunk(org, vel quake.Vec3) {
	engine.Particle(org, vel.Mul(0.02), 0, 10)
}

var (
	multi_ent    *quake.Entity
	multi_damage float32
)

func ClearMultiDamage() {
	multi_ent = nil
	multi_damage = 0
}

func ApplyMultiDamage() {
	if multi_ent == nil {
		return
	}

	T_Damage(multi_ent, Self, Self, multi_damage)
}

func AddMultiDamage(hit *quake.Entity, damage float32) {
	if hit == nil {
		return
	}

	if hit != multi_ent {
		ApplyMultiDamage()
		multi_damage = damage
		multi_ent = hit
	} else {
		multi_damage = multi_damage + damage
	}
}

func TraceAttack(damage float32, dir quake.Vec3) {
	var vel, org quake.Vec3

	vel = engine.Normalize(dir.Add(VUp.Mul(crandom())).Add(VRight.Mul(crandom())))
	vel = vel.Add(TracePlaneNormal.Mul(2))
	vel = vel.Mul(200)

	org = TraceEndPos.Sub(dir.Mul(4))

	if TraceEnt.TakeDamage != 0 {
		SpawnBlood(org, vel.Mul(0.2), damage)
		AddMultiDamage(TraceEnt, damage)
	} else {
		engine.WriteByte(MSG_BROADCAST, float32(SVC_TEMPENTITY))
		engine.WriteByte(MSG_BROADCAST, float32(TE_GUNSHOT))
		engine.WriteCoord(MSG_BROADCAST, org[0])
		engine.WriteCoord(MSG_BROADCAST, org[1])
		engine.WriteCoord(MSG_BROADCAST, org[2])
	}
}

func FireBullets(shotcount float32, dir, spread quake.Vec3) {
	var direction quake.Vec3
	var src quake.Vec3

	engine.MakeVectors(Self.VAngle)

	src = Self.Origin.Add(VForward.Mul(10))
	src[2] = Self.AbsMin[2] + Self.Size[2]*0.7

	ClearMultiDamage()
	for shotcount > 0 {
		direction = dir.Add(VRight.Mul(crandom() * spread[0])).Add(VUp.Mul(crandom() * spread[1]))

		engine.Traceline(src, src.Add(direction.Mul(2048)), FALSE, Self)
		if TraceFraction != 1.0 {
			TraceAttack(4, direction)
		}

		shotcount = shotcount - 1
	}
	ApplyMultiDamage()
}

func W_FireShotgun() {
	var dir quake.Vec3

	engine.Sound(Self, int(CHAN_WEAPON), "weapons/guncock.wav", 1, ATTN_NORM)

	Self.PunchAngle[0] = -2

	Self.AmmoShells = Self.AmmoShells - 1
	Self.CurrentAmmo = Self.AmmoShells
	dir = engine.Aim(Self, 100000)
	FireBullets(6, dir, quake.MakeVec3(0.04, 0.04, 0))
}

func W_FireSuperShotgun() {
	var dir quake.Vec3

	if Self.CurrentAmmo == 1 {
		W_FireShotgun()
		return
	}

	engine.Sound(Self, int(CHAN_WEAPON), "weapons/shotgn2.wav", 1, ATTN_NORM)

	Self.PunchAngle[0] = -4

	Self.AmmoShells = Self.AmmoShells - 2
	Self.CurrentAmmo = Self.AmmoShells
	dir = engine.Aim(Self, 100000)
	FireBullets(14, dir, quake.MakeVec3(0.14, 0.08, 0))
}

func s_explode1() { Self.Frame = 0; Self.NextThink = Time + 0.1; Self.Think = s_explode2 }
func s_explode2() { Self.Frame = 1; Self.NextThink = Time + 0.1; Self.Think = s_explode3 }
func s_explode3() { Self.Frame = 2; Self.NextThink = Time + 0.1; Self.Think = s_explode4 }
func s_explode4() { Self.Frame = 3; Self.NextThink = Time + 0.1; Self.Think = s_explode5 }
func s_explode5() { Self.Frame = 4; Self.NextThink = Time + 0.1; Self.Think = s_explode6 }
func s_explode6() { Self.Frame = 5; Self.NextThink = Time + 0.1; Self.Think = SUB_Remove }

func BecomeExplosion() {
	Self.MoveType = MOVETYPE_NONE
	Self.Velocity = quake.MakeVec3(0, 0, 0)
	Self.Touch = SUB_Null
	engine.SetModel(Self, "progs/s_explod.spr")
	Self.Solid = SOLID_NOT
	s_explode1()
}

func T_MissileTouch() {
	var damg float32

	if Other == Self.Owner {
		return
	}

	if engine.PointContents(Self.Origin) == float32(CONTENT_SKY) {
		engine.Remove(Self)
		return
	}

	damg = 100 + engine.Random()*20

	if Other.Health != 0 {
		if Other.ClassName == "monster_shambler" {
			damg = damg * 0.5
		}
		T_Damage(Other, Self, Self.Owner, damg)
	}

	T_RadiusDamage(Self, Self.Owner, 120, Other)

	Self.Origin = Self.Origin.Sub(engine.Normalize(Self.Velocity).Mul(8))

	engine.WriteByte(MSG_BROADCAST, float32(SVC_TEMPENTITY))
	engine.WriteByte(MSG_BROADCAST, float32(TE_EXPLOSION))
	engine.WriteCoord(MSG_BROADCAST, Self.Origin[0])
	engine.WriteCoord(MSG_BROADCAST, Self.Origin[1])
	engine.WriteCoord(MSG_BROADCAST, Self.Origin[2])

	BecomeExplosion()
}

func W_FireRocket() {
	var missile *quake.Entity

	Self.AmmoRockets = Self.AmmoRockets - 1
	Self.CurrentAmmo = Self.AmmoRockets

	engine.Sound(Self, int(CHAN_WEAPON), "weapons/sgun1.wav", 1, ATTN_NORM)

	Self.PunchAngle[0] = -2

	missile = engine.Spawn()
	missile.Owner = Self
	missile.MoveType = MOVETYPE_FLYMISSILE
	missile.Solid = SOLID_BBOX
	missile.ClassName = "missile"

	engine.MakeVectors(Self.VAngle)
	missile.Velocity = engine.Aim(Self, 1000)
	missile.Velocity = missile.Velocity.Mul(1000)
	missile.Angles = engine.VectoAngles(missile.Velocity)

	missile.Touch = T_MissileTouch

	missile.NextThink = Time + 5
	missile.Think = SUB_Remove

	engine.SetModel(missile, "progs/missile.mdl")
	engine.SetSize(missile, quake.MakeVec3(0, 0, 0), quake.MakeVec3(0, 0, 0))
	engine.SetOrigin(missile, Self.Origin.Add(VForward.Mul(8)).Add(quake.MakeVec3(0, 0, 16)))
}

func LightningDamage(p1, p2 quake.Vec3, from *quake.Entity, damage float32) {
	var e1, e2 *quake.Entity
	var f quake.Vec3

	f = p2.Sub(p1)
	f = engine.Normalize(f)
	fx := 0 - f[1]
	fy := f[0]
	f[0] = fx
	f[1] = fy
	f[2] = 0
	f = f.Mul(16)

	e1 = nil
	e2 = nil

	engine.Traceline(p1, p2, FALSE, Self)
	if TraceEnt.TakeDamage != 0 {
		engine.Particle(TraceEndPos, quake.MakeVec3(0, 0, 100), 225, damage*4)
		T_Damage(TraceEnt, from, from, damage)
		if Self.ClassName == "player" {
			if Other.ClassName == "player" {
				TraceEnt.Velocity[2] = TraceEnt.Velocity[2] + 400
			}
		}
	}

	e1 = TraceEnt

	engine.Traceline(p1.Add(f), p2.Add(f), FALSE, Self)
	if TraceEnt != e1 && TraceEnt.TakeDamage != 0 {
		engine.Particle(TraceEndPos, quake.MakeVec3(0, 0, 100), 225, damage*4)
		T_Damage(TraceEnt, from, from, damage)
	}

	e2 = TraceEnt

	engine.Traceline(p1.Sub(f), p2.Sub(f), FALSE, Self)
	if TraceEnt != e1 && TraceEnt != e2 && TraceEnt.TakeDamage != 0 {
		engine.Particle(TraceEndPos, quake.MakeVec3(0, 0, 100), 225, damage*4)
		T_Damage(TraceEnt, from, from, damage)
	}
}

func W_FireLightning() {
	var org quake.Vec3
	var cells float32

	if Self.AmmoCells < 1 {
		Self.Weapon = W_BestWeapon()
		W_SetCurrentAmmo()
		return
	}

	if Self.WaterLevel > 1 {
		cells = Self.AmmoCells
		Self.AmmoCells = 0
		W_SetCurrentAmmo()
		T_RadiusDamage(Self, Self, 35*cells, World)
		return
	}

	if Self.TWidth < Time {
		engine.Sound(Self, int(CHAN_WEAPON), "weapons/lhit.wav", 1, ATTN_NORM)
		Self.TWidth = Time + 0.6
	}

	Self.PunchAngle[0] = -2
	Self.AmmoCells = Self.AmmoCells - 1
	Self.CurrentAmmo = Self.AmmoCells
	org = Self.Origin.Add(quake.MakeVec3(0, 0, 16))

	engine.Traceline(org, org.Add(VForward.Mul(600)), TRUE, Self)

	engine.WriteByte(MSG_BROADCAST, float32(SVC_TEMPENTITY))
	engine.WriteByte(MSG_BROADCAST, float32(TE_LIGHTNING2))
	engine.WriteEntity(MSG_BROADCAST, Self)
	engine.WriteCoord(MSG_BROADCAST, org[0])
	engine.WriteCoord(MSG_BROADCAST, org[1])
	engine.WriteCoord(MSG_BROADCAST, org[2])
	engine.WriteCoord(MSG_BROADCAST, TraceEndPos[0])
	engine.WriteCoord(MSG_BROADCAST, TraceEndPos[1])
	engine.WriteCoord(MSG_BROADCAST, TraceEndPos[2])

	LightningDamage(Self.Origin, TraceEndPos.Add(VForward.Mul(4)), Self, 30)
}

func GrenadeExplode() {
	T_RadiusDamage(Self, Self.Owner, 120, World)

	engine.WriteByte(MSG_BROADCAST, float32(SVC_TEMPENTITY))
	engine.WriteByte(MSG_BROADCAST, float32(TE_EXPLOSION))
	engine.WriteCoord(MSG_BROADCAST, Self.Origin[0])
	engine.WriteCoord(MSG_BROADCAST, Self.Origin[1])
	engine.WriteCoord(MSG_BROADCAST, Self.Origin[2])

	BecomeExplosion()
}

func GrenadeTouch() {
	if Other == Self.Owner {
		return
	}
	if Other.TakeDamage == float32(DAMAGE_AIM) {
		GrenadeExplode()
		return
	}

	engine.Sound(Self, int(CHAN_WEAPON), "weapons/bounce.wav", 1, ATTN_NORM)

	if Self.Velocity == quake.MakeVec3(0, 0, 0) {
		Self.AVelocity = quake.MakeVec3(0, 0, 0)
	}
}

func W_FireGrenade() {
	var missile *quake.Entity

	Self.AmmoRockets = Self.AmmoRockets - 1
	Self.CurrentAmmo = Self.AmmoRockets

	engine.Sound(Self, int(CHAN_WEAPON), "weapons/grenade.wav", 1, ATTN_NORM)

	Self.PunchAngle[0] = -2

	missile = engine.Spawn()
	missile.Owner = Self
	missile.MoveType = MOVETYPE_BOUNCE
	missile.Solid = SOLID_BBOX
	missile.ClassName = "grenade"

	engine.MakeVectors(Self.VAngle)

	if Self.VAngle[0] != 0 {
		missile.Velocity = VForward.Mul(600).Add(VUp.Mul(200)).Add(VRight.Mul(crandom() * 10)).Add(VUp.Mul(crandom() * 10))
	} else {
		missile.Velocity = engine.Aim(Self, 10000)
		missile.Velocity = missile.Velocity.Mul(600)
		missile.Velocity[2] = 200
	}

	missile.AVelocity = quake.MakeVec3(300, 300, 300)
	missile.Angles = engine.VectoAngles(missile.Velocity)
	missile.Touch = GrenadeTouch

	missile.NextThink = Time + 2.5
	missile.Think = GrenadeExplode

	engine.SetModel(missile, "progs/grenade.mdl")
	engine.SetSize(missile, quake.MakeVec3(0, 0, 0), quake.MakeVec3(0, 0, 0))
	engine.SetOrigin(missile, Self.Origin)
}

func launch_spike(org, dir quake.Vec3) {
	Newmis = engine.Spawn()
	Newmis.Owner = Self
	Newmis.MoveType = MOVETYPE_FLYMISSILE
	Newmis.Solid = SOLID_BBOX

	Newmis.Angles = engine.VectoAngles(dir)

	Newmis.Touch = spike_touch
	Newmis.ClassName = "spike"
	Newmis.Think = SUB_Remove
	Newmis.NextThink = Time + 6
	engine.SetModel(Newmis, "progs/spike.mdl")
	engine.SetSize(Newmis, VEC_ORIGIN, VEC_ORIGIN)
	engine.SetOrigin(Newmis, org)

	Newmis.Velocity = dir.Mul(1000)
}

func W_FireSuperSpikes() {
	var dir quake.Vec3

	engine.Sound(Self, int(CHAN_WEAPON), "weapons/spike2.wav", 1, ATTN_NORM)
	Self.AttackFinished = Time + 0.2
	Self.AmmoNails = Self.AmmoNails - 2
	Self.CurrentAmmo = Self.AmmoNails
	dir = engine.Aim(Self, 1000)
	launch_spike(Self.Origin.Add(quake.MakeVec3(0, 0, 16)), dir)
	Newmis.ClassName = "super_spike"
	Newmis.Touch = superspike_touch
	engine.SetModel(Newmis, "progs/s_spike.mdl")
	engine.SetSize(Newmis, VEC_ORIGIN, VEC_ORIGIN)
	Self.PunchAngle[0] = -2
}

func W_FireSpikes(ox float32) {
	var dir quake.Vec3

	engine.MakeVectors(Self.VAngle)

	if Self.AmmoNails >= 2 && int(Self.Weapon) == IT_SUPER_NAILGUN {
		W_FireSuperSpikes()
		return
	}

	if Self.AmmoNails < 1 {
		Self.Weapon = W_BestWeapon()
		W_SetCurrentAmmo()
		return
	}

	engine.Sound(Self, int(CHAN_WEAPON), "weapons/rocket1i.wav", 1, ATTN_NORM)
	Self.AttackFinished = Time + 0.2
	Self.AmmoNails = Self.AmmoNails - 1
	Self.CurrentAmmo = Self.AmmoNails
	dir = engine.Aim(Self, 1000)
	launch_spike(Self.Origin.Add(quake.MakeVec3(0, 0, 16)).Add(VRight.Mul(ox)), dir)

	Self.PunchAngle[0] = -2
}

func spike_touch() {
	if Other == Self.Owner {
		return
	}

	if Other.Solid == float32(SOLID_TRIGGER) {
		return
	}

	if engine.PointContents(Self.Origin) == float32(CONTENT_SKY) {
		engine.Remove(Self)
		return
	}

	if Other.TakeDamage != 0 {
		spawn_touchblood(9)
		T_Damage(Other, Self, Self.Owner, 9)
	} else {
		engine.WriteByte(MSG_BROADCAST, float32(SVC_TEMPENTITY))

		if Self.ClassName == "wizard_spike" {
			engine.WriteByte(MSG_BROADCAST, float32(TE_WIZSPIKE))
		} else if Self.ClassName == "knight_spike" {
			engine.WriteByte(MSG_BROADCAST, float32(TE_KNIGHTSPIKE))
		} else {
			engine.WriteByte(MSG_BROADCAST, float32(TE_SPIKE))
		}

		engine.WriteCoord(MSG_BROADCAST, Self.Origin[0])
		engine.WriteCoord(MSG_BROADCAST, Self.Origin[1])
		engine.WriteCoord(MSG_BROADCAST, Self.Origin[2])
	}

	engine.Remove(Self)
}

func superspike_touch() {
	if Other == Self.Owner {
		return
	}

	if Other.Solid == float32(SOLID_TRIGGER) {
		return
	}

	if engine.PointContents(Self.Origin) == float32(CONTENT_SKY) {
		engine.Remove(Self)
		return
	}

	if Other.TakeDamage != 0 {
		spawn_touchblood(18)
		T_Damage(Other, Self, Self.Owner, 18)
	} else {
		engine.WriteByte(MSG_BROADCAST, float32(SVC_TEMPENTITY))
		engine.WriteByte(MSG_BROADCAST, float32(TE_SUPERSPIKE))
		engine.WriteCoord(MSG_BROADCAST, Self.Origin[0])
		engine.WriteCoord(MSG_BROADCAST, Self.Origin[1])
		engine.WriteCoord(MSG_BROADCAST, Self.Origin[2])
	}

	engine.Remove(Self)
}

func W_SetCurrentAmmo() {
	player_run()
	Self.WeaponFrame = 0

	Self.Items = float32(int(Self.Items) - (int(Self.Items) & (IT_SHELLS | IT_NAILS | IT_ROCKETS | IT_CELLS)))

	switch int(Self.Weapon) {
	case IT_AXE:
		Self.CurrentAmmo = 0
		Self.WeaponModel = "progs/v_axe.mdl"
	case IT_SHOTGUN:
		Self.CurrentAmmo = Self.AmmoShells
		Self.WeaponModel = "progs/v_shot.mdl"
		Self.Items = float32(int(Self.Items) | IT_SHELLS)
	case IT_SUPER_SHOTGUN:
		Self.CurrentAmmo = Self.AmmoShells
		Self.WeaponModel = "progs/v_shot2.mdl"
		Self.Items = float32(int(Self.Items) | IT_SHELLS)
	case IT_NAILGUN:
		Self.CurrentAmmo = Self.AmmoNails
		Self.WeaponModel = "progs/v_nail.mdl"
		Self.Items = float32(int(Self.Items) | IT_NAILS)
	case IT_SUPER_NAILGUN:
		Self.CurrentAmmo = Self.AmmoNails
		Self.WeaponModel = "progs/v_nail2.mdl"
		Self.Items = float32(int(Self.Items) | IT_NAILS)
	case IT_GRENADE_LAUNCHER:
		Self.CurrentAmmo = Self.AmmoRockets
		Self.WeaponModel = "progs/v_rock.mdl"
		Self.Items = float32(int(Self.Items) | IT_ROCKETS)
	case IT_ROCKET_LAUNCHER:
		Self.CurrentAmmo = Self.AmmoRockets
		Self.WeaponModel = "progs/v_rock2.mdl"
		Self.Items = float32(int(Self.Items) | IT_ROCKETS)
	case IT_LIGHTNING:
		Self.CurrentAmmo = Self.AmmoCells
		Self.WeaponModel = "progs/v_light.mdl"
		Self.Items = float32(int(Self.Items) | IT_CELLS)
	default:
		Self.CurrentAmmo = 0
		Self.WeaponModel = StringNull
	}
}

func W_BestWeapon() float32 {
	it := int(Self.Items)

	if Self.WaterLevel <= 1 && Self.AmmoCells >= 1 && (it&IT_LIGHTNING) != 0 {
		return float32(IT_LIGHTNING)
	} else if Self.AmmoNails >= 2 && (it&IT_SUPER_NAILGUN) != 0 {
		return float32(IT_SUPER_NAILGUN)
	} else if Self.AmmoShells >= 2 && (it&IT_SUPER_SHOTGUN) != 0 {
		return float32(IT_SUPER_SHOTGUN)
	} else if Self.AmmoNails >= 1 && (it&IT_NAILGUN) != 0 {
		return float32(IT_NAILGUN)
	} else if Self.AmmoShells >= 1 && (it&IT_SHOTGUN) != 0 {
		return float32(IT_SHOTGUN)
	}

	return float32(IT_AXE)
}

func W_WantsToChangeWeapon(playerEnt *quake.Entity, old, new float32) float32 {
	playerFlags := engine.CheckPlayerEXFlags(playerEnt)
	if (int(playerFlags) & 1) != 0 { // PEF_CHANGENEVER
		return 0
	}

	if ((int(playerFlags) & 2) != 0) && old == new { // PEF_CHANGEONLYNEW
		return 0
	}

	return 1
}

func W_HasNoAmmo() float32 {
	if Self.CurrentAmmo != 0 {
		return FALSE
	}

	if int(Self.Weapon) == IT_AXE {
		return FALSE
	}

	Self.Weapon = W_BestWeapon()
	W_SetCurrentAmmo()

	return TRUE
}

func W_Attack() {
	var r float32

	if W_HasNoAmmo() != 0 {
		return
	}

	engine.MakeVectors(Self.VAngle)
	Self.ShowHostile = Time + 1

	if int(Self.Weapon) != IT_AXE {
		Self.FiredWeapon = 1
	}

	if int(Self.Weapon) == IT_AXE {
		engine.Sound(Self, int(CHAN_WEAPON), "weapons/ax1.wav", 1, ATTN_NORM)
		r = engine.Random()
		if r < 0.25 {
			player_axe1()
		} else if r < 0.5 {
			player_axeb1()
		} else if r < 0.75 {
			player_axec1()
		} else {
			player_axed1()
		}
		Self.AttackFinished = Time + 0.5
	} else if int(Self.Weapon) == IT_SHOTGUN {
		player_shot1()
		W_FireShotgun()
		Self.AttackFinished = Time + 0.5
	} else if int(Self.Weapon) == IT_SUPER_SHOTGUN {
		player_shot1()
		W_FireSuperShotgun()
		Self.AttackFinished = Time + 0.7
	} else if int(Self.Weapon) == IT_NAILGUN {
		player_nail1()
		W_FireSpikes(0)
	} else if int(Self.Weapon) == IT_SUPER_NAILGUN {
		player_nail1()
		W_FireSpikes(0)
	} else if int(Self.Weapon) == IT_GRENADE_LAUNCHER {
		player_rocket1()
		W_FireGrenade()
		Self.AttackFinished = Time + 0.6
	} else if int(Self.Weapon) == IT_ROCKET_LAUNCHER {
		player_rocket1()
		W_FireRocket()
		Self.AttackFinished = Time + 0.8
	} else if int(Self.Weapon) == IT_LIGHTNING {
		player_light1()
		Self.AttackFinished = Time + 0.1
		engine.Sound(Self, int(CHAN_AUTO), "weapons/lstart.wav", 1, ATTN_NORM)
	}
}

func W_ChangeWeapon() {
	var am, fl float32

	am = 0

	switch int(Self.Impulse) {
	case 1:
		fl = float32(IT_AXE)
	case 2:
		fl = float32(IT_SHOTGUN)
		if Self.AmmoShells < 1 {
			am = 1
		}
	case 3:
		fl = float32(IT_SUPER_SHOTGUN)
		if Self.AmmoShells < 2 {
			am = 1
		}
	case 4:
		fl = float32(IT_NAILGUN)
		if Self.AmmoNails < 1 {
			am = 1
		}
	case 5:
		fl = float32(IT_SUPER_NAILGUN)
		if Self.AmmoNails < 2 {
			am = 1
		}
	case 6:
		fl = float32(IT_GRENADE_LAUNCHER)
		if Self.AmmoRockets < 1 {
			am = 1
		}
	case 7:
		fl = float32(IT_ROCKET_LAUNCHER)
		if Self.AmmoRockets < 1 {
			am = 1
		}
	case 8:
		fl = float32(IT_LIGHTNING)
		if Self.AmmoCells < 1 {
			am = 1
		}
	}

	Self.Impulse = 0

	if (int(Self.Items) & int(fl)) == 0 {
		engine.SPrint(Self, "$qc_no_weapon")
		return
	}

	if am != 0 {
		engine.SPrint(Self, "$qc_not_enough_ammo")
		return
	}

	Self.Weapon = fl
	W_SetCurrentAmmo()
}

func CheatCommand() {
	if (Deathmatch != 0 || Coop != 0) && CheatsAllowed == 0 {
		return
	}

	Self.AmmoRockets = 100
	Self.AmmoNails = 200
	Self.AmmoShells = 100
	Self.Items = float32(int(Self.Items) | IT_AXE | IT_SHOTGUN | IT_SUPER_SHOTGUN | IT_NAILGUN | IT_SUPER_NAILGUN | IT_GRENADE_LAUNCHER | IT_ROCKET_LAUNCHER | IT_KEY1 | IT_KEY2)

	Self.AmmoCells = 200
	Self.Items = float32(int(Self.Items) | IT_LIGHTNING)

	Self.ArmorType = 0.8
	Self.ArmorValue = 200
	Self.Items = float32(int(Self.Items) - (int(Self.Items) & (IT_ARMOR1 | IT_ARMOR2 | IT_ARMOR3)) + IT_ARMOR3)

	Self.Weapon = float32(IT_ROCKET_LAUNCHER)
	Self.Impulse = 0
	W_SetCurrentAmmo()
}

func CycleWeaponCommand() {
	it := int(Self.Items)
	Self.Impulse = 0

	for {
		am := 0

		switch int(Self.Weapon) {
		case IT_LIGHTNING:
			Self.Weapon = float32(IT_AXE)
		case IT_AXE:
			Self.Weapon = float32(IT_SHOTGUN)
			if Self.AmmoShells < 1 {
				am = 1
			}
		case IT_SHOTGUN:
			Self.Weapon = float32(IT_SUPER_SHOTGUN)
			if Self.AmmoShells < 2 {
				am = 1
			}
		case IT_SUPER_SHOTGUN:
			Self.Weapon = float32(IT_NAILGUN)
			if Self.AmmoNails < 1 {
				am = 1
			}
		case IT_NAILGUN:
			Self.Weapon = float32(IT_SUPER_NAILGUN)
			if Self.AmmoNails < 2 {
				am = 1
			}
		case IT_SUPER_NAILGUN:
			Self.Weapon = float32(IT_GRENADE_LAUNCHER)
			if Self.AmmoRockets < 1 {
				am = 1
			}
		case IT_GRENADE_LAUNCHER:
			Self.Weapon = float32(IT_ROCKET_LAUNCHER)
			if Self.AmmoRockets < 1 {
				am = 1
			}
		case IT_ROCKET_LAUNCHER:
			Self.Weapon = float32(IT_LIGHTNING)
			if Self.AmmoCells < 1 {
				am = 1
			}
		}

		if (it&int(Self.Weapon)) != 0 && am == 0 {
			W_SetCurrentAmmo()
			return
		}
	}
}

func CycleWeaponReverseCommand() {
	it := int(Self.Items)
	Self.Impulse = 0

	for {
		am := 0

		switch int(Self.Weapon) {
		case IT_LIGHTNING:
			Self.Weapon = float32(IT_ROCKET_LAUNCHER)
			if Self.AmmoRockets < 1 {
				am = 1
			}
		case IT_ROCKET_LAUNCHER:
			Self.Weapon = float32(IT_GRENADE_LAUNCHER)
			if Self.AmmoRockets < 1 {
				am = 1
			}
		case IT_GRENADE_LAUNCHER:
			Self.Weapon = float32(IT_SUPER_NAILGUN)
			if Self.AmmoNails < 2 {
				am = 1
			}
		case IT_SUPER_NAILGUN:
			Self.Weapon = float32(IT_NAILGUN)
			if Self.AmmoNails < 1 {
				am = 1
			}
		case IT_NAILGUN:
			Self.Weapon = float32(IT_SUPER_SHOTGUN)
			if Self.AmmoShells < 2 {
				am = 1
			}
		case IT_SUPER_SHOTGUN:
			Self.Weapon = float32(IT_SHOTGUN)
			if Self.AmmoShells < 1 {
				am = 1
			}
		case IT_SHOTGUN:
			Self.Weapon = float32(IT_AXE)
		case IT_AXE:
			Self.Weapon = float32(IT_LIGHTNING)
			if Self.AmmoCells < 1 {
				am = 1
			}
		}

		if (it&int(Self.Weapon)) != 0 && am == 0 {
			W_SetCurrentAmmo()
			return
		}
	}
}

func ServerflagsCommand() {
	ServerFlags = ServerFlags*2 + 1
}

func QuadCheat() {
	if CheatsAllowed == 0 {
		return
	}

	Self.SuperTime = 1
	Self.SuperDamageFinished = Time + 30
	Self.Items = float32(int(Self.Items) | IT_QUAD)
	engine.Dprint("quad cheat\n")
}

func ImpulseCommands() {
	if Self.Impulse >= 1 && Self.Impulse <= 8 {
		W_ChangeWeapon()
	} else if Self.Impulse == 9 {
		CheatCommand()
	} else if Self.Impulse == 10 {
		CycleWeaponCommand()
	} else if Self.Impulse == 11 {
		ServerflagsCommand()
	} else if Self.Impulse == 12 {
		CycleWeaponReverseCommand()
	} else if Self.Impulse == 255 {
		QuadCheat()
	}

	Self.Impulse = 0
}

func W_WeaponFrame() {
	if Time < Self.AttackFinished {
		return
	}

	if Self.Impulse != 0 {
		ImpulseCommands()
	}

	if Self.Button0 != 0 {
		SuperDamageSound()
		W_Attack()
	}
}

func SuperDamageSound() {
	if Self.SuperDamageFinished > Time {
		if Self.SuperSound < Time {
			Self.SuperSound = Time + 1
			engine.Sound(Self, int(CHAN_BODY), "items/damage3.wav", 1, ATTN_NORM)
		}
	}
}
