// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/loc"
	"github.com/darkliquid/ironwail-go/internal/qc"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
	"github.com/darkliquid/ironwail-go/internal/testutil"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

func TestE1M1NightmareCombatAndMovement(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	loc.Init(baseDir, "english")

	vfs := fs.NewFileSystem()
	if err := vfs.Init(baseDir, "id1"); err != nil {
		t.Fatalf("init filesystem: %v", err)
	}
	defer vfs.Close()

	s := NewServer()
	s.Datagram = NewMessageBuffer(MaxDatagram)
	s.Static = &ServerStatic{Clients: []*Client{{Active: true, Message: NewMessageBuffer(MaxDatagram)}}}
	newServerTestVM(s, 1024)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}

	// Set skill to 3 (Nightmare)
	s.CVar.Set("skill", "3")

	progsData, err := vfs.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("load progs.dat: %v", err)
	}
	if err := s.QCVM.LoadProgs(bytes.NewReader(progsData)); err != nil {
		t.Fatalf("LoadProgs: %v", err)
	}
	qc.RegisterBuiltins(s.QCVM)

	if err := s.SpawnServer("e1m1", vfs); err != nil {
		t.Fatalf("spawn server e1m1: %v", err)
	}

	t.Logf("e1m1 spawned with %d edicts", s.NumEdicts)

	// Enumerate monsters
	type monsterInfo struct {
		num       int
		classname string
		origin    qtypes.Vec3
		absmin    qtypes.Vec3
		absmax    qtypes.Vec3
		solid     int
		takedmg   int
		flags     uint32
	}
	var monsters []monsterInfo
	for i := 1; i < s.NumEdicts; i++ {
		e := s.Edicts[i]
		if e == nil || e.Free {
			continue
		}
		cname := s.String(e.ClassName(s))
		flags := uint32(e.Flags(s))
		if flags&uint32(srvtypes.FlagMonster) != 0 || (len(cname) > 8 && cname[:8] == "monster_") {
			monsters = append(monsters, monsterInfo{
				num:       i,
				classname: cname,
				origin:    e.Origin(s),
				absmin:    e.AbsMin(s),
				absmax:    e.AbsMax(s),
				solid:     int(e.Solid(s)),
				takedmg:   int(e.TakeDamage(s)),
				flags:     flags,
			})
		}
	}
	t.Logf("Found %d monsters in e1m1 nightmare", len(monsters))
	for idx, m := range monsters {
		t.Logf("Monster #%d: num=%d class=%s origin=%v solid=%d takedmg=%d flags=0x%x absmin=%v absmax=%v",
			idx, m.num, m.classname, m.origin, m.solid, m.takedmg, m.flags, m.absmin, m.absmax)
	}

	// Setup player edict at info_player_start (480, -352, 88)
	player := s.Edicts[1]
	player.Free = false
	player.SetClassName(s, s.QCVM.AllocString("player"))
	player.SetSolid(s, float32(SolidSlideBox))
	player.SetMoveType(s, float32(MoveTypeWalk))
	player.SetOrigin(s, qtypes.Vec3{X: 480, Y: -352, Z: 88})
	player.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -24})
	player.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 32})
	player.SetAbsMin(s, qtypes.Vec3{X: 464, Y: -368, Z: 64})
	player.SetAbsMax(s, qtypes.Vec3{X: 496, Y: -336, Z: 120})
	player.SetAngles(s, qtypes.Vec3{X: 0, Y: 90, Z: 0})
	player.SetVAngle(s, qtypes.Vec3{X: 0, Y: 90, Z: 0})
	s.LinkEdict(player, true)

	// Test tracing from player position towards the first few monsters
	for idx, m := range monsters {
		mEnt := s.EdictNum(m.num)
		mCenter := m.origin.Add(mEnt.Mins(s).Add(mEnt.Maxs(s)).Scale(0.5))

		// Move player near the monster with direct line of sight (100 units away)
		playerPos := mCenter.Add(qtypes.Vec3{X: 0, Y: -100, Z: 0})
		playerEyes := playerPos.Add(qtypes.Vec3{Z: 16})

		// Trace directly at the monster
		tr := s.SV_Move(playerEyes, qtypes.Vec3{}, qtypes.Vec3{}, mCenter, MoveType(MoveNormal), player)
		hitNum := -1
		if tr.Entity != nil {
			hitNum = s.NumForEdict(tr.Entity)
		}
		t.Logf("Monster #%d (ent=%d, class=%s) direct trace: hitEnt=%d, frac=%.3f, allsolid=%t, startsolid=%t",
			idx, m.num, m.classname, hitNum, tr.Fraction, tr.AllSolid, tr.StartSolid)

		// Test shotgun pellet trace with weapon origin (offset by 10 units forward)
		fwd := mCenter.Sub(playerEyes).Normalize()
		src := playerPos.Add(fwd.Scale(10))
		src.Z = playerPos.Z - 24 + 56*0.7 // Self.AbsMin[2] + Self.Size[2]*0.7
		end := src.Add(fwd.Scale(2048))

		trShot := s.SV_Move(src, qtypes.Vec3{}, qtypes.Vec3{}, end, MoveType(MoveNormal), player)
		hitShotNum := -1
		if trShot.Entity != nil {
			hitShotNum = s.NumForEdict(trShot.Entity)
		}
		t.Logf("Monster #%d ent=%d shotgun trace: hitEnt=%d, frac=%.3f, allsolid=%t, startsolid=%t",
			idx, m.num, hitShotNum, trShot.Fraction, trShot.AllSolid, trShot.StartSolid)
	}
}

func TestE1M1ActualShotgunFiring(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	loc.Init(baseDir, "english")

	vfs := fs.NewFileSystem()
	if err := vfs.Init(baseDir, "id1"); err != nil {
		t.Fatalf("init filesystem: %v", err)
	}
	defer vfs.Close()

	s := NewServer()
	s.Datagram = NewMessageBuffer(MaxDatagram)
	s.Static = &ServerStatic{Clients: []*Client{{Active: true, Message: NewMessageBuffer(MaxDatagram)}}}
	newServerTestVM(s, 1024)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}

	RegisterSvdbgCVars(s.CVar)
	SetSvdbgEmit(func(line string) { t.Log(line) })

	s.CVar.Set("skill", "3")
	s.CVar.Set("sv_debug_combat", "2")

	progsData, err := vfs.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("load progs.dat: %v", err)
	}
	if err := s.QCVM.LoadProgs(bytes.NewReader(progsData)); err != nil {
		t.Fatalf("LoadProgs: %v", err)
	}
	qc.RegisterBuiltins(s.QCVM)

	if err := s.SpawnServer("e1m1", vfs); err != nil {
		t.Fatalf("spawn server e1m1: %v", err)
	}

	// Find Monster #0 (army grunt at 616 72 40)
	var grunt *Edict
	for i := 1; i < s.NumEdicts; i++ {
		e := s.Edicts[i]
		if e != nil && !e.Free && s.String(e.ClassName(s)) == "monster_army" {
			grunt = e
			break
		}
	}
	if grunt == nil {
		t.Fatal("monster_army not found")
	}

	initialHealth := grunt.Health(s)
	t.Logf("Found grunt #%d: origin=%v health=%v solid=%v takedmg=%v",
		s.NumForEdict(grunt), grunt.Origin(s), initialHealth, grunt.Solid(s), grunt.TakeDamage(s))

	// Setup player
	player := s.Edicts[1]
	player.Free = false
	player.SetClassName(s, s.QCVM.AllocString("player"))
	player.SetSolid(s, float32(SolidSlideBox))
	player.SetMoveType(s, float32(MoveTypeWalk))
	player.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -24})
	player.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 32})
	player.SetSize(s, qtypes.Vec3{X: 32, Y: 32, Z: 56})

	// Place player 150 units in front of grunt, facing +X (grunt is at X=616, Y=72, Z=40)
	player.SetOrigin(s, qtypes.Vec3{X: 466, Y: 72, Z: 40})
	player.SetAbsMin(s, qtypes.Vec3{X: 450, Y: 56, Z: 16})
	player.SetAbsMax(s, qtypes.Vec3{X: 482, Y: 88, Z: 72})
	player.SetAngles(s, qtypes.Vec3{X: 0, Y: 0, Z: 0})
	player.SetVAngle(s, qtypes.Vec3{X: 0, Y: 0, Z: 0})
	player.SetWeapon(s, 1) // IT_SHOTGUN = 1
	player.SetAmmoShells(s, 25)
	player.SetCurrentAmmo(s, 25)
	s.LinkEdict(player, true)

	// Execute W_Attack
	fnIdx := s.QCVM.FindFunction("W_Attack")
	t.Logf("W_Attack function index = %d", fnIdx)
	if fnIdx < 0 {
		t.Fatal("W_Attack not found")
	}

	s.QCVM.SetGInt(qc.OFSSelf, int32(s.NumForEdict(player)))
	s.QCVM.SetGFloat(qc.OFSTime, 1.0)
	if err := s.QCVM.ExecuteProgram(fnIdx); err != nil {
		t.Fatalf("ExecuteProgram(W_Attack): %v", err)
	}

	afterHealth := grunt.Health(s)
	t.Logf("Grunt health before=%v, after=%v (damage=%v)", initialHealth, afterHealth, initialHealth-afterHealth)

	// Now test with Monster #7 (elevated on bridge at 648, 736, 104)
	var bridgeGrunt *Edict
	for i := 1; i < s.NumEdicts; i++ {
		e := s.Edicts[i]
		if e != nil && !e.Free && s.String(e.ClassName(s)) == "monster_army" && e.Origin(s).Z > 80 {
			bridgeGrunt = e
			break
		}
	}
	if bridgeGrunt != nil {
		bInitHealth := bridgeGrunt.Health(s)
		t.Logf("Found bridgeGrunt #%d: origin=%v health=%v", s.NumForEdict(bridgeGrunt), bridgeGrunt.Origin(s), bInitHealth)

		// Place player at (400, 576, 40) in room looking towards bridge grunt (648, 736, 104)
		player.SetOrigin(s, qtypes.Vec3{X: 400, Y: 576, Z: 40})
		player.SetAbsMin(s, qtypes.Vec3{X: 384, Y: 560, Z: 16})
		player.SetAbsMax(s, qtypes.Vec3{X: 416, Y: 592, Z: 72})
		player.SetCurrentAmmo(s, 25)

		// Vector to grunt: dx = 248, dy = 160, dz = 64
		// Yaw: atan2(dy, dx) = atan2(160, 248) = 32.8 degrees
		// Pitch: -atan2(dz, sqrt(dx*dx + dy*dy)) = -atan2(64, 295.1) = -12.2 degrees
		yaw := float32(32.8)
		pitch := float32(-12.2)

		player.SetAngles(s, qtypes.Vec3{X: 0, Y: yaw, Z: 0})
		player.SetVAngle(s, qtypes.Vec3{X: pitch, Y: yaw, Z: 0})
		s.LinkEdict(player, true)

		t.Logf("--- Firing at bridge grunt with aimed pitch=%.1f yaw=%.1f ---", pitch, yaw)
		s.QCVM.SetGInt(qc.OFSSelf, int32(s.NumForEdict(player)))
		s.QCVM.SetGFloat(qc.OFSTime, 3.0)
		if err := s.QCVM.ExecuteProgram(fnIdx); err != nil {
			t.Fatalf("ExecuteProgram(W_Attack): %v", err)
		}
		bAfterHealth := bridgeGrunt.Health(s)
		t.Logf("Bridge grunt health before=%v, after=%v (damage=%v)", bInitHealth, bAfterHealth, bInitHealth-bAfterHealth)

		// 1. Direct pitch-aimed shot should hit
		if bInitHealth-bAfterHealth <= 0 {
			t.Fatalf("expected directly aimed shot to hit bridge grunt, dealt 0 damage")
		}

		// 2. Test pitch=0 (horizontal look) with sv_aim=1.0 (autoaim disabled)
		bridgeGrunt.SetHealth(s, 30)
		player.SetVAngle(s, qtypes.Vec3{X: 0, Y: yaw, Z: 0}) // pitch = 0!
		player.SetCurrentAmmo(s, 25)
		s.CVar.Set("sv_aim", "1")
		t.Logf("--- Firing at bridge grunt with pitch=0, sv_aim=1 ---")
		if err := s.QCVM.ExecuteProgram(fnIdx); err != nil {
			t.Fatalf("ExecuteProgram(W_Attack): %v", err)
		}
		dmgNoAutoaim := 30 - bridgeGrunt.Health(s)
		t.Logf("Bridge grunt health after horizontal shot (sv_aim=1) = %v (damage=%v)", bridgeGrunt.Health(s), dmgNoAutoaim)
		if dmgNoAutoaim != 0 {
			t.Fatalf("expected horizontal shot with sv_aim=1 to miss elevated bridge grunt, got damage=%v", dmgNoAutoaim)
		}

		// 3. Test pitch=0 (horizontal look) with sv_aim=0.93 (classic Quake autoaim)
		bridgeGrunt.SetHealth(s, 30)
		player.SetVAngle(s, qtypes.Vec3{X: 0, Y: yaw, Z: 0}) // pitch = 0!
		player.SetCurrentAmmo(s, 25)
		s.CVar.Set("sv_aim", "0.93")
		t.Logf("--- Firing at bridge grunt with pitch=0, sv_aim=0.93 ---")
		if err := s.QCVM.ExecuteProgram(fnIdx); err != nil {
			t.Fatalf("ExecuteProgram(W_Attack): %v", err)
		}
		dmgAutoaim := 30 - bridgeGrunt.Health(s)
		t.Logf("Bridge grunt health after horizontal shot (sv_aim=0.93) = %v (damage=%v)", bridgeGrunt.Health(s), dmgAutoaim)
		if dmgAutoaim <= 0 {
			t.Fatalf("expected horizontal shot with sv_aim=0.93 to hit elevated bridge grunt, got damage=0")
		}

		// 4. Test pitch=0 with default sv_aim (classic Quake 0.93 default)
		bridgeGrunt.SetHealth(s, 30)
		player.SetVAngle(s, qtypes.Vec3{X: 0, Y: yaw, Z: 0})
		player.SetCurrentAmmo(s, 25)
		s.CVar.Set("sv_aim", "0.93")
		t.Logf("--- Firing at bridge grunt with pitch=0, default sv_aim ---")
		if err := s.QCVM.ExecuteProgram(fnIdx); err != nil {
			t.Fatalf("ExecuteProgram(W_Attack): %v", err)
		}
		dmgDefault := 30 - bridgeGrunt.Health(s)
		t.Logf("Bridge grunt health after horizontal shot (default sv_aim) = %v (damage=%v)", bridgeGrunt.Health(s), dmgDefault)
		if dmgDefault <= 0 {
			t.Fatalf("expected horizontal shot with default sv_aim to hit elevated bridge grunt, got damage=0")
		}
	}
}

