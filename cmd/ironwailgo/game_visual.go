package main

import (
	"log/slog"
	"math/rand"
	"sort"
	"strings"

	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/hud"
	inet "github.com/ironwail/ironwail-go/internal/net"
	"github.com/ironwail/ironwail-go/internal/renderer"
)

func applyDemoPlaybackViewAngles(clientState *cl.Client, viewAngles [3]float32) {
	if clientState == nil {
		return
	}
	clientState.MViewAngles[1] = clientState.MViewAngles[0]
	clientState.MViewAngles[0] = viewAngles
	clientState.ViewAngles = viewAngles
}

func shouldReadNextDemoMessage(clientState *cl.Client, demo *cl.DemoState) bool {
	if clientState == nil || demo == nil {
		return true
	}
	if demo.RewindBackstop() {
		return false
	}
	if demo.TimeDemo {
		return true
	}
	if clientState.Signon < cl.Signons {
		return true
	}
	if demo.Speed > 0 {
		return clientState.Time > clientState.MTime[0]
	}
	if demo.Speed < 0 {
		return clientState.Time < clientState.MTime[0]
	}
	return false
}

func recordRuntimeDemoFrame() {
	if g.Host == nil || g.Subs == nil || g.Subs.Client == nil || g.Client == nil {
		return
	}

	demo := g.Host.DemoState()
	if demo == nil || !demo.Recording {
		return
	}

	source, ok := g.Subs.Client.(interface{ LastServerMessage() []byte })
	if !ok {
		return
	}
	message := source.LastServerMessage()
	if len(message) == 0 {
		return
	}

	if err := demo.WriteDemoFrame(message, g.Client.ViewAngles); err != nil {
		slog.Warn("failed to record demo frame", "error", err)
	}
}

func resetRuntimeVisualState() {
	globalViewCalc = viewCalcState{}

	if g.Renderer == nil {
		g.Particles = nil
		g.DecalMarks = nil
		g.ParticleRNG = nil
		g.ParticleTime = 0
		g.RuntimeBeams = nil
		g.SkyboxNameKey = ""
		return
	}

	g.Particles = renderer.NewParticleSystem(renderer.MaxParticles)
	g.DecalMarks = renderer.NewDecalMarkSystem()
	g.ParticleRNG = rand.New(rand.NewSource(1))
	g.ParticleTime = 0
	g.RuntimeBeams = nil
	g.SkyboxNameKey = ""
}

func syncRuntimeVisualEffects(dt float64, transientEvents cl.TransientEvents) {
	if g.Particles == nil && g.DecalMarks == nil && g.Renderer == nil {
		return
	}

	if g.Client == nil || g.Client.State != cl.StateActive {
		g.RuntimeBeams = nil
		if (g.Particles != nil && g.Particles.ActiveCount() > 0) || (g.DecalMarks != nil && g.DecalMarks.ActiveCount() > 0) {
			resetRuntimeVisualState()
		}
		return
	}

	// Update v_blend color shifts: decay damage/bonus, compute powerup, sync contents tint.
	// g.CameraLeafContents was updated by syncRuntimeAmbientAudio earlier this frame.
	// Mirrors C view.c:V_UpdateBlend() + V_SetContentsColor().
	g.Client.SetContentsColor(g.CameraLeafContents)
	g.Client.UpdateBlend(dt)

	// Update damage kick angles if damage was recently taken.
	// Mirrors C Ironwail V_ParseDamage damage kick calculation (view.c:329-345).
	if g.Client.DamageTaken > 0 || g.Client.DamageSaved > 0 {
		if entityOrigin, ok := runtimeAuthoritativePlayerOrigin(); ok {
			var entityAngles [3]float32
			// Get player entity angles from ViewEntity.
			if g.Client.ViewEntity != 0 {
				if state, ok := g.Client.Entities[g.Client.ViewEntity]; ok {
					entityAngles = state.Angles
				}
			} else if state, ok := g.Client.Entities[0]; ok {
				entityAngles = state.Angles
			}
			// Get cvar values.
			kickTime := float32(0.5)
			kickRoll := float32(0.6)
			kickPitch := float32(0.6)
			if cv := cvar.Get("v_kicktime"); cv != nil {
				kickTime = cv.Float32()
			}
			if cv := cvar.Get("v_kickroll"); cv != nil {
				kickRoll = cv.Float32()
			}
			if cv := cvar.Get("v_kickpitch"); cv != nil {
				kickPitch = cv.Float32()
			}
			g.Client.CalculateDamageKick(entityOrigin, entityAngles, kickTime, kickRoll, kickPitch)
		}
	}

	oldTime := g.ParticleTime
	g.ParticleTime += float32(dt)

	// Update scope zoom transition after relink, matching C CL_RelinkEntities
	// calling SCR_UpdateZoom() post-velocity interpolation.
	g.Zoom, g.ZoomDir, _ = renderer.UpdateZoom(g.Zoom, g.ZoomDir, currentZoomSpeed(), float32(g.ParticleTime-float32(dt)), g.ParticleTime)

	particleEvents := transientEvents.ParticleEvents
	tempEntities := transientEvents.TempEntities
	g.RuntimeBeams = transientEvents.BeamSegments

	// Trail events are emitted by RelinkEntities based on model flags,
	// so collect them from the Client after relinking (not from TransientEvents).
	var trailEvents []cl.TrailEvent
	if g.Client != nil {
		trailEvents = g.Client.TrailEvents
		g.Client.TrailEvents = nil
	}

	if g.Particles != nil {
		effectSources := collectEntityEffectSources()
		renderer.EmitClientEffects(g.Particles, particleEvents, trailEvents, tempEntities, g.ParticleRNG, g.ParticleTime)
		renderer.EmitEntityEffectParticles(g.Particles, effectSources, g.ParticleTime)
		g.Particles.RunParticles(g.ParticleTime, oldTime, 800)
	}
	if g.DecalMarks != nil {
		g.DecalMarks.Run(g.ParticleTime)
		renderer.EmitDecalMarks(g.DecalMarks, tempEntities, g.ParticleRNG, g.ParticleTime)
	}
}

func currentZoomSpeed() float32 {
	if cv := cvar.Get("zoom_speed"); cv != nil {
		return cv.Float32()
	}
	return 8
}

func syncRuntimeSkybox() {
	if g.Renderer == nil {
		g.SkyboxNameKey = ""
		return
	}
	skyboxName := ""
	if g.Client != nil && g.Client.State == cl.StateActive {
		skyboxName = g.Client.SkyboxName
	}
	g.SkyboxNameKey = skyboxName
}

func applyRuntimeRendererVisualEffects(dt float64, transientEvents cl.TransientEvents) {
	if g.Renderer == nil {
		return
	}

	if g.Client == nil || g.Client.State != cl.StateActive {
		g.Renderer.ClearDynamicLights()
		return
	}

	g.Renderer.UpdateLights(float32(dt))
	renderer.EmitDynamicLights(g.Renderer.SpawnDynamicLight, transientEvents.TempEntities)
	renderer.EmitEntityEffectLights(g.Renderer.SpawnKeyedDynamicLight, collectEntityEffectSources())
}

func applyRuntimeRendererSkybox() {
	if g.Renderer == nil {
		return
	}
	skyboxName := g.SkyboxNameKey
	if skyboxName == "" || g.Subs == nil || g.Subs.Files == nil {
		g.Renderer.SetExternalSkybox("", nil)
		return
	}
	g.Renderer.SetExternalSkybox(skyboxName, g.Subs.Files.LoadFile)
}

// updateHUDFromServer pushes current player/client state into the HUD.
func updateHUDFromServer() {
	if g.HUD == nil {
		return
	}

	if g.Client != nil {
		shells, nails, rockets, cells := g.Client.AmmoCounts()
		g.HUD.SetState(hud.State{
			Health:        g.Client.Health(),
			Armor:         g.Client.Armor(),
			Ammo:          g.Client.Ammo(),
			WeaponModel:   g.Client.WeaponModelIndex(),
			ActiveWeapon:  g.Client.ActiveWeapon(),
			Shells:        shells,
			Nails:         nails,
			Rockets:       rockets,
			Cells:         cells,
			Items:         g.Client.Items,
			ModHipnotic:   g.ModDir == "hipnotic",
			ModRogue:      g.ModDir == "rogue",
			GameType:      g.Client.GameType,
			MaxClients:    g.Client.MaxClients,
			ShowScores:    g.ShowScores && g.Client.MaxClients > 1,
			Scoreboard:    buildHUDScoreboard(g.Client),
			Paused:        g.Client.Paused,
			InCutscene:    g.Client.InCutscene(),
			Intermission:  g.Client.Intermission,
			CompletedTime: g.Client.CompletedTime,
			Time:          g.Client.Time,
			CenterPrint:   g.Client.CenterPrint,
			CenterPrintAt: g.Client.CenterPrintAt,
			FaceAnimUntil: g.Client.FaceAnimUntil,
			LevelName:     g.Client.LevelName,
			Secrets:       g.Client.Stats[inet.StatSecrets],
			TotalSecrets:  g.Client.Stats[inet.StatTotalSecrets],
			Monsters:      g.Client.Stats[inet.StatMonsters],
			TotalMonsters: g.Client.Stats[inet.StatTotalMonsters],
		})
		return
	}

	if g.Server == nil {
		return
	}
	ent := g.Server.EdictNum(1)
	if ent == nil {
		return
	}
	g.HUD.SetState(hud.State{
		Health:      int(ent.Vars.Health),
		Armor:       int(ent.Vars.ArmorValue),
		Ammo:        int(ent.Vars.CurrentAmmo),
		WeaponModel: int(ent.Vars.Weapon),
	})
}

func buildHUDScoreboard(client *cl.Client) []hud.ScoreEntry {
	if client == nil || client.MaxClients <= 1 {
		return nil
	}
	rows := make([]hud.ScoreEntry, 0, client.MaxClients)
	current := client.ViewEntity - 1
	for i := 0; i < client.MaxClients; i++ {
		name := strings.TrimSpace(client.PlayerNames[i])
		if name == "" {
			continue
		}
		rows = append(rows, hud.ScoreEntry{
			ClientIndex: i,
			Name:        name,
			Frags:       client.Frags[i],
			Colors:      client.PlayerColors[i],
			IsCurrent:   i == current,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Frags == rows[j].Frags {
			return rows[i].ClientIndex < rows[j].ClientIndex
		}
		return rows[i].Frags > rows[j].Frags
	})
	return rows
}
