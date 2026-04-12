package quakego

import (
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake"
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake/engine"
)

func PlayerDeathThink() {
	var forward float32

	if (int(Self.Flags) & FL_ONGROUND) != 0 {
		forward = engine.Vlen(Self.Velocity)
		forward = forward - 20
		if forward <= 0 {
			Self.Velocity = quake.MakeVec3(0, 0, 0)
		} else {
			Self.Velocity = engine.Normalize(Self.Velocity).Mul(forward)
		}
	}

	if Self.SpawnDeferred != 0 {
		spot := SelectSpawnPoint(FALSE)
		if spot != nil || Time >= Self.SpawnDeferred {
			respawn()
		}
		return
	}

	if Self.DeadFlag == float32(DEAD_DEAD) {
		if Self.Button2 != 0 || Self.Button1 != 0 || Self.Button0 != 0 {
			return
		}
		Self.DeadFlag = float32(DEAD_RESPAWNABLE)
		return
	}

	if Self.Button2 == 0 && Self.Button1 == 0 && Self.Button0 == 0 {
		return
	}

	Self.Button0 = 0
	Self.Button1 = 0
	Self.Button2 = 0
	respawn()
}

func PlayerJump() {
	if (int(Self.Flags) & FL_WATERJUMP) != 0 {
		return
	}

	if Self.WaterLevel >= 2 {
		if int(Self.WaterType) == CONTENT_WATER {
			Self.Velocity[2] = 100
		} else if int(Self.WaterType) == CONTENT_SLIME {
			Self.Velocity[2] = 80
		} else {
			Self.Velocity[2] = 50
		}

		if Self.SwimFlag < Time {
			Self.SwimFlag = Time + 1
			if engine.Random() < 0.5 {
				engine.Sound(Self, int(CHAN_BODY), "misc/water1.wav", 1, ATTN_NORM)
			} else {
				engine.Sound(Self, int(CHAN_BODY), "misc/water2.wav", 1, ATTN_NORM)
			}
		}
		return
	}

	if (int(Self.Flags) & FL_ONGROUND) == 0 {
		return
	}

	if (int(Self.Flags) & FL_JUMPRELEASED) == 0 {
		return
	}

	Self.Flags = Self.Flags - float32(int(Self.Flags)&FL_JUMPRELEASED)
	Self.Flags = Self.Flags - float32(FL_ONGROUND)

	Self.Button2 = 0
	engine.Sound(Self, int(CHAN_BODY), "player/plyrjmp8.wav", 1, ATTN_NORM)
	Self.Velocity[2] = Self.Velocity[2] + 270
}

func WaterMove() {
	if int(Self.MoveType) == MOVETYPE_NOCLIP {
		return
	}

	if Self.Health < 0 {
		return
	}

	if Self.WaterLevel != 3 {
		if Self.AirFinished < Time {
			engine.Sound(Self, int(CHAN_VOICE), "player/gasp2.wav", 1, ATTN_NORM)
		} else if Self.AirFinished < Time+9 {
			engine.Sound(Self, int(CHAN_VOICE), "player/gasp1.wav", 1, ATTN_NORM)
		}
		Self.AirFinished = Time + 12
		Self.Dmg = 2
	} else if Self.AirFinished < Time {
		if Self.PainFinished < Time {
			Self.Dmg = Self.Dmg + 2
			if Self.Dmg > 15 {
				Self.Dmg = 10
			}
			T_Damage(Self, World, World, Self.Dmg)
			Self.PainFinished = Time + 1
		}
	}

	if Self.WaterLevel == 0 {
		if (int(Self.Flags) & FL_INWATER) != 0 {
			engine.Sound(Self, int(CHAN_BODY), "misc/outwater.wav", 1, ATTN_NORM)
			Self.Flags = Self.Flags - float32(FL_INWATER)
		}
		return
	}

	if int(Self.WaterType) == CONTENT_LAVA {
		if Self.DmgTime < Time {
			if Self.RadSuitFinished > Time {
				Self.DmgTime = Time + 1
			} else {
				Self.DmgTime = Time + 0.2
			}
			T_Damage(Self, World, World, 10*Self.WaterLevel)
		}
	} else if int(Self.WaterType) == CONTENT_SLIME {
		if Self.DmgTime < Time && Self.RadSuitFinished < Time {
			Self.DmgTime = Time + 1
			T_Damage(Self, World, World, 4*Self.WaterLevel)
		}
	}

	if (int(Self.Flags) & FL_INWATER) == 0 {
		if int(Self.WaterType) == CONTENT_LAVA {
			engine.Sound(Self, int(CHAN_BODY), "player/inlava.wav", 1, ATTN_NORM)
		}
		if int(Self.WaterType) == CONTENT_WATER {
			engine.Sound(Self, int(CHAN_BODY), "player/inh2o.wav", 1, ATTN_NORM)
		}
		if int(Self.WaterType) == CONTENT_SLIME {
			engine.Sound(Self, int(CHAN_BODY), "player/slimbrn2.wav", 1, ATTN_NORM)
		}
		Self.Flags = Self.Flags + float32(FL_INWATER)
		Self.DmgTime = 0
	}

	if (int(Self.Flags) & FL_WATERJUMP) == 0 {
		Self.Velocity = Self.Velocity.Sub(Self.Velocity.Mul(0.8 * Self.WaterLevel * Frametime))
	}
}

func CheckWaterJump() {
	var start, end quake.Vec3

	makevectorsfixed(Self.Angles)
	start = Self.Origin
	start[2] = start[2] + 8
	VForward[2] = 0
	engine.Normalize(VForward)
	end = start.Add(VForward.Mul(24))
	engine.Traceline(start, end, TRUE, Self)

	if TraceFraction < 1 {
		start[2] = start[2] + Self.Maxs[2] - 8
		end = start.Add(VForward.Mul(24))
		Self.MoveDir = TracePlaneNormal.Mul(-50)
		engine.Traceline(start, end, TRUE, Self)

		if TraceFraction == 1 {
			Self.Flags = float32(int(Self.Flags) | FL_WATERJUMP)
			Self.Velocity[2] = 225
			Self.Flags = Self.Flags - float32(int(Self.Flags)&FL_JUMPRELEASED)
			Self.TeleportTime = Time + 2
			return
		}
	}
}

func PlayerPreThink() {
	if IntermissionRunning != 0 {
		IntermissionThink()
		return
	}

	if Self.ViewOfs == quake.MakeVec3(0, 0, 0) {
		return
	}

	engine.MakeVectors(Self.VAngle)

	if Deathmatch != 0 || Coop != 0 {
		CheckRules()
	}

	WaterMove()

	if Self.WaterLevel == 2 {
		CheckWaterJump()
	}

	if Self.DeadFlag >= float32(DEAD_DEAD) {
		PlayerDeathThink()
		return
	}

	if Self.DeadFlag == float32(DEAD_DYING) {
		return
	}

	if Self.Button2 != 0 {
		PlayerJump()
	} else {
		Self.Flags = float32(int(Self.Flags) | FL_JUMPRELEASED)
	}

	if Time < Self.PauseTime {
		Self.Velocity = quake.MakeVec3(0, 0, 0)
	}

	if Time > Self.AttackFinished && Self.CurrentAmmo == 0 && int(Self.Weapon) != IT_AXE {
		Self.Weapon = W_BestWeapon()
		W_SetCurrentAmmo()
	}
}

func CheckPowerups() {
	if Self.Health <= 0 {
		return
	}

	if Self.InvisibleFinished != 0 {
		if Self.InvisibleSound < Time {
			engine.Sound(Self, int(CHAN_AUTO), "items/inv3.wav", 0.5, ATTN_IDLE)
			Self.InvisibleSound = Time + ((engine.Random() * 3) + 1)
		}

		if Self.InvisibleFinished < Time+3 {
			if Self.InvisibleTime == 1 {
				engine.SPrint(Self, "$qc_ring_fade")
				engine.StuffCmd(Self, "bf\n")
				engine.Sound(Self, int(CHAN_AUTO), "items/inv2.wav", 1, ATTN_NORM)
				Self.InvisibleTime = Time + 1
			}
			if Self.InvisibleTime < Time {
				Self.InvisibleTime = Time + 1
				engine.StuffCmd(Self, "bf\n")
			}
		}

		if Self.InvisibleFinished < Time {
			Self.Items = Self.Items - float32(IT_INVISIBILITY)
			Self.InvisibleFinished = 0
			Self.InvisibleTime = 0
		}

		Self.Frame = 0
		Self.ModelIndex = ModelIndexEyes
	} else {
		Self.ModelIndex = ModelIndexPlayer
	}

	if Self.InvincibleFinished != 0 {
		if Self.InvincibleFinished < Time+3 {
			if Self.InvincibleTime == 1 {
				engine.SPrint(Self, "$qc_protection_fade")
				engine.StuffCmd(Self, "bf\n")
				engine.Sound(Self, int(CHAN_AUTO), "items/protect2.wav", 1, ATTN_NORM)
				Self.InvincibleTime = Time + 1
			}
			if Self.InvincibleTime < Time {
				Self.InvincibleTime = Time + 1
				engine.StuffCmd(Self, "bf\n")
			}
		}

		if Self.InvincibleFinished < Time {
			Self.Items = Self.Items - float32(IT_INVULNERABILITY)
			Self.InvincibleTime = 0
			Self.InvincibleFinished = 0
		}
		if Self.InvincibleFinished > Time {
			Self.Effects = float32(int(Self.Effects) | EF_PENTALIGHT)
		} else {
			Self.Effects = float32(int(Self.Effects) - (int(Self.Effects) & EF_PENTALIGHT))
		}
	}

	if Self.SuperDamageFinished != 0 {
		if Self.SuperDamageFinished < Time+3 {
			if Self.SuperTime == 1 {
				engine.SPrint(Self, "$qc_quad_fade")
				engine.StuffCmd(Self, "bf\n")
				engine.Sound(Self, int(CHAN_AUTO), "items/damage2.wav", 1, ATTN_NORM)
				Self.SuperTime = Time + 1
			}
			if Self.SuperTime < Time {
				Self.SuperTime = Time + 1
				engine.StuffCmd(Self, "bf\n")
			}
		}

		if Self.SuperDamageFinished < Time {
			Self.Items = Self.Items - float32(IT_QUAD)
			Self.SuperDamageFinished = 0
			Self.SuperTime = 0
		}
		if Self.SuperDamageFinished > Time {
			Self.Effects = float32(int(Self.Effects) | EF_QUADLIGHT)
		} else {
			Self.Effects = float32(int(Self.Effects) - (int(Self.Effects) & EF_QUADLIGHT))
		}
	}

	if Self.RadSuitFinished != 0 {
		Self.AirFinished = Time + 12
		if Self.RadSuitFinished < Time+3 {
			if Self.RadTime == 1 {
				engine.SPrint(Self, "$qc_biosuit_fade")
				engine.StuffCmd(Self, "bf\n")
				engine.Sound(Self, int(CHAN_AUTO), "items/suit2.wav", 1, ATTN_NORM)
				Self.RadTime = Time + 1
			}
			if Self.RadTime < Time {
				Self.RadTime = Time + 1
				engine.StuffCmd(Self, "bf\n")
			}
		}

		if Self.RadSuitFinished < Time {
			Self.Items = Self.Items - float32(IT_SUIT)
			Self.RadTime = 0
			Self.RadSuitFinished = 0
		}
	}
}

func CheckHealthRot() {
	if (int(Self.Items) & IT_SUPERHEALTH) == 0 {
		return
	}

	if Self.HealthRotNextCheck > Time {
		return
	}

	if Self.Health > Self.MaxHealth {
		Self.Health = Self.Health - 1
		Self.HealthRotNextCheck = Time + 1
		return
	}

	Self.Items = Self.Items - float32(int(Self.Items)&IT_SUPERHEALTH)
	Self.HealthRotNextCheck = 0
}

func PlayerPostThink() {
	if Self.ViewOfs == quake.MakeVec3(0, 0, 0) {
		return
	}

	if Self.DeadFlag != 0 {
		return
	}

	W_WeaponFrame()

	if (Self.JumpFlag < -300) && (int(Self.Flags)&FL_ONGROUND) != 0 && (Self.Health > 0) {
		if int(Self.WaterType) == CONTENT_WATER {
			engine.Sound(Self, int(CHAN_BODY), "player/h2ojump.wav", 1, ATTN_NORM)
		} else if Self.JumpFlag < -650 {
			T_Damage(Self, World, World, 5)
			engine.Sound(Self, int(CHAN_VOICE), "player/land2.wav", 1, ATTN_NORM)
			if Self.Health <= 5 {
				Self.DeathType = "falling"
			}
		} else {
			engine.Sound(Self, int(CHAN_VOICE), "player/land.wav", 1, ATTN_NORM)
		}
		Self.JumpFlag = 0
	}

	if (int(Self.Flags) & FL_ONGROUND) == 0 {
		Self.JumpFlag = Self.Velocity[2]
	}

	CheckPowerups()
	CheckHealthRot()
}

func ClientConnect() {
	engine.Bprint(quake.Sprintf("$qc_entered %s", Self.NetName))
	if IntermissionRunning != 0 {
		ExitIntermission()
	}
}

func ClientDisconnect() {
	if Gameover != 0 {
		return
	}

	engine.Bprint(quake.Sprintf("$qc_left_game %s %s", Self.NetName, engine.Ftos(Self.Frags)))
	engine.Sound(Self, int(CHAN_BODY), "player/tornoff2.wav", 1, ATTN_NONE)
	Self.Effects = 0
	set_suicide_frame()
}
