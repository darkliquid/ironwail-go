package quakego

import (
	"github.com/ironwail/ironwail-go/pkg/qgo/quake"
	"github.com/ironwail/ironwail-go/pkg/qgo/quake/engine"
)

// Prototyped elsewhere
var (
)

func info_null() {
	engine.Remove(Self)
}

func info_notnull() {}

var START_OFF float32 = 1

func light_use() {
	if (int(Self.SpawnFlags) & int(START_OFF)) != 0 {
		engine.LightStyle(Self.Style, "m")
		Self.SpawnFlags = Self.SpawnFlags - START_OFF
	} else {
		engine.LightStyle(Self.Style, "a")
		Self.SpawnFlags = Self.SpawnFlags + START_OFF
	}
}

func light() {
	if Self.TargetName == "" {
		engine.Remove(Self)
		return
	}

	if Self.Style >= 32 {
		Self.Use = light_use
		if (int(Self.SpawnFlags) & int(START_OFF)) != 0 {
			engine.LightStyle(Self.Style, "a")
		} else {
			engine.LightStyle(Self.Style, "m")
		}
	}
}

func light_fluoro() {
	if Self.Style >= 32 {
		Self.Use = light_use
		if (int(Self.SpawnFlags) & int(START_OFF)) != 0 {
			engine.LightStyle(Self.Style, "a")
		} else {
			engine.LightStyle(Self.Style, "m")
		}
	}

	engine.PrecacheSound("ambience/fl_hum1.wav")
	engine.Ambientsound(Self.Origin, "ambience/fl_hum1.wav", 0.5, ATTN_STATIC)
}

func light_fluorospark() {
	if Self.Style == 0 {
		Self.Style = 10
	}

	engine.PrecacheSound("ambience/buzz1.wav")
	engine.Ambientsound(Self.Origin, "ambience/buzz1.wav", 0.5, ATTN_STATIC)
}

func light_globe() {
	engine.PrecacheModel("progs/s_light.spr")
	engine.SetModel(Self, "progs/s_light.spr")
	engine.MakeStatic(Self)
}

func FireAmbient() {
	engine.PrecacheSound("ambience/fire1.wav")
	engine.Ambientsound(Self.Origin, "ambience/fire1.wav", 0.5, ATTN_STATIC)
}

func light_torch_small_walltorch() {
	engine.PrecacheModel("progs/flame.mdl")
	engine.SetModel(Self, "progs/flame.mdl")
	FireAmbient()
	engine.MakeStatic(Self)
}

func light_flame_large_yellow() {
	engine.PrecacheModel("progs/flame2.mdl")
	engine.SetModel(Self, "progs/flame2.mdl")
	Self.Frame = 1
	FireAmbient()
	engine.MakeStatic(Self)
}

func light_flame_small_yellow() {
	engine.PrecacheModel("progs/flame2.mdl")
	engine.SetModel(Self, "progs/flame2.mdl")
	FireAmbient()
	engine.MakeStatic(Self)
}

func light_flame_small_white() {
	engine.PrecacheModel("progs/flame2.mdl")
	engine.SetModel(Self, "progs/flame2.mdl")
	FireAmbient()
	engine.MakeStatic(Self)
}

func misc_fireball() {
	engine.PrecacheModel("progs/lavaball.mdl")
	Self.ClassName = "fireball"
	Self.NextThink = Time + (engine.Random() * 5)
	Self.Think = fire_fly
	Self.NetName = "$qc_lava_ball"
	Self.KillString = "$qc_ks_lavaball"

	if Self.Speed == 0 {
		Self.Speed = 1000
	}
}

func fire_fly() {
	fireball := engine.Spawn()
	fireball.Solid = SOLID_TRIGGER
	fireball.MoveType = MOVETYPE_TOSS
	fireball.Velocity = quake.MakeVec3(0, 0, 1000)
	fireball.Velocity[0] = (engine.Random() * 100) - 50
	fireball.Velocity[1] = (engine.Random() * 100) - 50
	fireball.Velocity[2] = Self.Speed + (engine.Random() * 200)
	fireball.ClassName = "fireball"
	engine.SetModel(fireball, "progs/lavaball.mdl")
	engine.SetSize(fireball, quake.MakeVec3(0, 0, 0), quake.MakeVec3(0, 0, 0))
	engine.SetOrigin(fireball, Self.Origin)
	fireball.NextThink = Time + 5
	fireball.Think = SUB_Remove
	fireball.Touch = fire_touch

	Self.NetName = "$qc_lava_ball"
	Self.KillString = "$qc_ks_lavaball"

	Self.NextThink = Time + (engine.Random() * 5) + 3
	Self.Think = fire_fly
}

func fire_touch() {
	T_Damage(Other, Self, Self, 20)
	engine.Remove(Self)
}

func barrel_explode() {
	T_RadiusDamage(Self, Self.Enemy, 160, World)
	engine.Sound(Self, int(CHAN_VOICE), "weapons/r_exp3.wav", 1, ATTN_NORM)
	engine.Particle(Self.Origin, quake.MakeVec3(0, 0, 0), 75, 255)

	Self.Origin[2] = Self.Origin[2] + 32
	BecomeExplosion()
}

func barrel_detonate() {
	Self.ClassName = "explo_box"
	Self.TakeDamage = DAMAGE_NO
	Self.Think = barrel_explode
	Self.NextThink = Self.LTime + 0.3
}

func misc_explobox() {
	var oldz float32

	if Self.Mdl == "" {
		Self.Mdl = "maps/b_explob.bsp"
	}

	Self.Solid = SOLID_BSP
	Self.MoveType = MOVETYPE_PUSH
	engine.PrecacheModel(Self.Mdl)
	engine.SetModel(Self, Self.Mdl)
	engine.PrecacheSound("weapons/r_exp3.wav")
	Self.Health = 20
	Self.ThDie = barrel_detonate
	Self.TakeDamage = DAMAGE_AIM
	Self.NetName = "$qc_exploding_barrel"
	Self.KillString = "$qc_ks_blew_up"

	Self.Origin[2] = Self.Origin[2] + 2
	oldz = Self.Origin[2]
	engine.DropToFloor()

	if oldz-Self.Origin[2] > 250 {
		engine.Dprint("explobox fell out of level at ")
		engine.Dprint(engine.Vtos(Self.Origin))
		engine.Dprint("\n")
		engine.Remove(Self)
	}
}

func misc_explobox2() {
	Self.Mdl = "maps/b_exbox2.bsp"
	misc_explobox()
}

var (
	SPAWNFLAG_SUPERSPIKE float32 = 1
	SPAWNFLAG_LASER      float32 = 2
)

func spikeshooter_use() {
	if (int(Self.SpawnFlags) & int(SPAWNFLAG_LASER)) != 0 {
		engine.Sound(Self, int(CHAN_VOICE), "enforcer/enfire.wav", 1, ATTN_NORM)
		LaunchLaser(Self.Origin, Self.MoveDir)
	} else {
		engine.Sound(Self, int(CHAN_VOICE), "weapons/spike2.wav", 1, ATTN_NORM)
		launch_spike(Self.Origin, Self.MoveDir)
		Newmis.Velocity = Self.MoveDir.Mul(500)

		if (int(Self.SpawnFlags) & int(SPAWNFLAG_SUPERSPIKE)) != 0 {
			Newmis.Touch = superspike_touch
		}
	}
}

func shooter_think() {
	spikeshooter_use()
	Self.NextThink = Time + Self.Wait
	Newmis.Velocity = Self.MoveDir.Mul(500)
}

func trap_spikeshooter() {
	SetMovedir()
	Self.Use = spikeshooter_use
	Self.NetName = "$qc_spike_trap"
	Self.KillString = "$qc_ks_spiked"

	if (int(Self.SpawnFlags) & int(SPAWNFLAG_LASER)) != 0 {
		engine.PrecacheModel2("progs/laser.mdl")
		engine.PrecacheSound2("enforcer/enfire.wav")
		engine.PrecacheSound2("enforcer/enfstop.wav")
	} else {
		engine.PrecacheSound("weapons/spike2.wav")
	}
}

func trap_shooter() {
	trap_spikeshooter()

	if Self.Wait == 0 {
		Self.Wait = 1
	}

	Self.NextThink = Self.NextThink + Self.Wait + Self.LTime
	Self.Think = shooter_think
}

func air_bubbles() {
	if Deathmatch != 0 {
		engine.Remove(Self)
		return
	}

	engine.PrecacheModel("progs/s_bubble.spr")
	Self.NextThink = Time + 1
	Self.Think = make_bubbles
}

func make_bubbles() {
	bubble := engine.Spawn()
	engine.SetModel(bubble, "progs/s_bubble.spr")
	engine.SetOrigin(bubble, Self.Origin)
	bubble.MoveType = MOVETYPE_NOCLIP
	bubble.Solid = SOLID_NOT
	bubble.Velocity = quake.MakeVec3(0, 0, 15)
	bubble.NextThink = Time + 0.5
	bubble.Think = bubble_bob
	bubble.Touch = bubble_remove
	bubble.ClassName = "bubble"
	bubble.Frame = 0
	bubble.Cnt = 0
	engine.SetSize(bubble, quake.MakeVec3(-8, -8, -8), quake.MakeVec3(8, 8, 8))
	Self.NextThink = Time + engine.Random() + 0.5
	Self.Think = make_bubbles
}

func bubble_split() {
	bubble := engine.Spawn()
	engine.SetModel(bubble, "progs/s_bubble.spr")
	engine.SetOrigin(bubble, Self.Origin)
	bubble.MoveType = MOVETYPE_NOCLIP
	bubble.Solid = SOLID_NOT
	bubble.Velocity = Self.Velocity
	bubble.NextThink = Time + 0.5
	bubble.Think = bubble_bob
	bubble.Touch = bubble_remove
	bubble.ClassName = "bubble"
	bubble.Frame = 1
	bubble.Cnt = 10
	engine.SetSize(bubble, quake.MakeVec3(-8, -8, -8), quake.MakeVec3(8, 8, 8))
	Self.Frame = 1
	Self.Cnt = 10

	if Self.WaterLevel != 3 {
		engine.Remove(Self)
	}
}

func bubble_remove() {
	if Other.ClassName == Self.ClassName {
		return
	}
	engine.Remove(Self)
}

func bubble_bob() {
	var rnd1, rnd2, rnd3 float32

	Self.Cnt = Self.Cnt + 1
	if Self.Cnt == 4 {
		bubble_split()
	}

	if Self.Cnt == 20 {
		engine.Remove(Self)
	}

	rnd1 = Self.Velocity[0] + (-10 + (engine.Random() * 20))
	rnd2 = Self.Velocity[1] + (-10 + (engine.Random() * 20))
	rnd3 = Self.Velocity[2] + 10 + engine.Random()*10

	if rnd1 > 10 {
		rnd1 = 5
	}
	if rnd1 < -10 {
		rnd1 = -5
	}
	if rnd2 > 10 {
		rnd2 = 5
	}
	if rnd2 < -10 {
		rnd2 = -5
	}
	if rnd3 < 10 {
		rnd3 = 15
	}
	if rnd3 > 30 {
		rnd3 = 25
	}

	Self.Velocity[0] = rnd1
	Self.Velocity[1] = rnd2
	Self.Velocity[2] = rnd3

	Self.NextThink = Time + 0.5
	Self.Think = bubble_bob
}

func viewthing() {
	Self.MoveType = MOVETYPE_NONE
	Self.Solid = SOLID_NOT
	engine.PrecacheModel("progs/player.mdl")
	engine.SetModel(Self, "progs/player.mdl")
}

func func_wall_use() {
	Self.Frame = 1 - Self.Frame
}

func func_wall() {
	Self.Angles = quake.MakeVec3(0, 0, 0)
	Self.ClassName = "func_wall"
	Self.MoveType = MOVETYPE_PUSH
	Self.Solid = SOLID_BSP
	Self.Use = func_wall_use
	engine.SetModel(Self, Self.Model)
}

func func_illusionary() {
	Self.Angles = quake.MakeVec3(0, 0, 0)
	Self.MoveType = MOVETYPE_NONE
	Self.Solid = SOLID_NOT
	engine.SetModel(Self, Self.Model)
	engine.MakeStatic(Self)
}

func func_episodegate() {
	if (int(ServerFlags) & int(Self.SpawnFlags)) == 0 {
		return
	}

	Self.Angles = quake.MakeVec3(0, 0, 0)
	Self.MoveType = MOVETYPE_PUSH
	Self.Solid = SOLID_BSP
	Self.Use = func_wall_use
	engine.SetModel(Self, Self.Model)
}

func func_bossgate() {
	if (int(ServerFlags) & 15) == 15 {
		return
	}

	Self.Angles = quake.MakeVec3(0, 0, 0)
	Self.MoveType = MOVETYPE_PUSH
	Self.Solid = SOLID_BSP
	Self.Use = func_wall_use
	engine.SetModel(Self, Self.Model)
}

func ambient_suck_wind() {
	engine.PrecacheSound("ambience/suck1.wav")
	engine.Ambientsound(Self.Origin, "ambience/suck1.wav", 1, ATTN_STATIC)
}

func ambient_drone() {
	engine.PrecacheSound("ambience/drone6.wav")
	engine.Ambientsound(Self.Origin, "ambience/drone6.wav", 0.5, ATTN_STATIC)
}

func ambient_flouro_buzz() {
	engine.PrecacheSound("ambience/buzz1.wav")
	engine.Ambientsound(Self.Origin, "ambience/buzz1.wav", 1, ATTN_STATIC)
}

func ambient_drip() {
	engine.PrecacheSound("ambience/drip1.wav")
	engine.Ambientsound(Self.Origin, "ambience/drip1.wav", 0.5, ATTN_STATIC)
}

func ambient_comp_hum() {
	engine.PrecacheSound("ambience/comp1.wav")
	engine.Ambientsound(Self.Origin, "ambience/comp1.wav", 1, ATTN_STATIC)
}

func ambient_thunder() {
	engine.PrecacheSound("ambience/thunder1.wav")
	engine.Ambientsound(Self.Origin, "ambience/thunder1.wav", 0.5, ATTN_STATIC)
}

func ambient_light_buzz() {
	engine.PrecacheSound("ambience/fl_hum1.wav")
	engine.Ambientsound(Self.Origin, "ambience/fl_hum1.wav", 0.5, ATTN_STATIC)
}

func ambient_swamp1() {
	engine.PrecacheSound("ambience/swamp1.wav")
	engine.Ambientsound(Self.Origin, "ambience/swamp1.wav", 0.5, ATTN_STATIC)
}

func ambient_swamp2() {
	engine.PrecacheSound("ambience/swamp2.wav")
	engine.Ambientsound(Self.Origin, "ambience/swamp2.wav", 0.5, ATTN_STATIC)
}

func noise_think() {
	Self.NextThink = Time + 0.5
	engine.Sound(Self, 1, "enforcer/enfire.wav", 1, ATTN_NORM)
	engine.Sound(Self, 2, "enforcer/enfstop.wav", 1, ATTN_NORM)
	engine.Sound(Self, 3, "enforcer/sight1.wav", 1, ATTN_NORM)
	engine.Sound(Self, 4, "enforcer/sight2.wav", 1, ATTN_NORM)
	engine.Sound(Self, 5, "enforcer/sight3.wav", 1, ATTN_NORM)
	engine.Sound(Self, 6, "enforcer/sight4.wav", 1, ATTN_NORM)
	engine.Sound(Self, 7, "enforcer/pain1.wav", 1, ATTN_NORM)
}

func misc_noisemaker() {
	engine.PrecacheSound2("enforcer/enfire.wav")
	engine.PrecacheSound2("enforcer/enfstop.wav")
	engine.PrecacheSound2("enforcer/sight1.wav")
	engine.PrecacheSound2("enforcer/sight2.wav")
	engine.PrecacheSound2("enforcer/sight3.wav")
	engine.PrecacheSound2("enforcer/sight4.wav")
	engine.PrecacheSound2("enforcer/pain1.wav")
	engine.PrecacheSound2("enforcer/pain2.wav")
	engine.PrecacheSound2("enforcer/death1.wav")
	engine.PrecacheSound2("enforcer/idle1.wav")

	Self.NextThink = Time + 0.1 + engine.Random()
	Self.Think = noise_think
}
