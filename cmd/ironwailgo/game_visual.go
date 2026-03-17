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
	if gameHost == nil || gameSubs == nil || gameSubs.Client == nil || gameClient == nil {
		return
	}

	demo := gameHost.DemoState()
	if demo == nil || !demo.Recording {
		return
	}

	source, ok := gameSubs.Client.(interface{ LastServerMessage() []byte })
	if !ok {
		return
	}
	message := source.LastServerMessage()
	if len(message) == 0 {
		return
	}

	if err := demo.WriteDemoFrame(message, gameClient.ViewAngles); err != nil {
		slog.Warn("failed to record demo frame", "error", err)
	}
}

func resetRuntimeVisualState() {
	if gameRenderer == nil {
		gameParticles = nil
		gameDecalMarks = nil
		particleRNG = nil
		particleTime = 0
		skyboxNameKey = ""
		return
	}

	gameParticles = renderer.NewParticleSystem(renderer.MaxParticles)
	gameDecalMarks = renderer.NewDecalMarkSystem()
	particleRNG = rand.New(rand.NewSource(1))
	particleTime = 0
	skyboxNameKey = ""
}

func syncRuntimeVisualEffects(dt float64, transientEvents cl.TransientEvents) {
	if gameParticles == nil && gameDecalMarks == nil && gameRenderer == nil {
		return
	}

	if gameClient == nil || gameClient.State != cl.StateActive {
		if gameRenderer != nil {
			gameRenderer.ClearDynamicLights()
		}
		if (gameParticles != nil && gameParticles.ActiveCount() > 0) || (gameDecalMarks != nil && gameDecalMarks.ActiveCount() > 0) {
			resetRuntimeVisualState()
		}
		return
	}

	// Update v_blend color shifts: decay damage/bonus, compute powerup, sync contents tint.
	// runtimeCameraLeafContents was updated by syncRuntimeAmbientAudio earlier this frame.
	// Mirrors C view.c:V_UpdateBlend() + V_SetContentsColor().
	gameClient.SetContentsColor(runtimeCameraLeafContents)
	gameClient.UpdateBlend(dt)

	// Update damage kick angles if damage was recently taken.
	// Mirrors C Ironwail V_ParseDamage damage kick calculation (view.c:329-345).
	if gameClient.DamageTaken > 0 || gameClient.DamageSaved > 0 {
		if entityOrigin, ok := runtimeAuthoritativePlayerOrigin(); ok {
			var entityAngles [3]float32
			// Get player entity angles from ViewEntity.
			if gameClient.ViewEntity != 0 {
				if state, ok := gameClient.Entities[gameClient.ViewEntity]; ok {
					entityAngles = state.Angles
				}
			} else if state, ok := gameClient.Entities[0]; ok {
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
			gameClient.CalculateDamageKick(entityOrigin, entityAngles, kickTime, kickRoll, kickPitch)
		}
	}

	oldTime := particleTime
	particleTime += float32(dt)

	// Interpolate entity positions/angles from double-buffered network origins.
	// Must run before any entity collection so rendered positions are lerped.
	if gameClient != nil {
		gameClient.RelinkEntities()
	}

	particleEvents := transientEvents.ParticleEvents
	tempEntities := transientEvents.TempEntities
	effectSources := collectEntityEffectSources()

	if gameRenderer != nil {
		gameRenderer.UpdateLights(float32(dt))
		renderer.EmitDynamicLights(gameRenderer.SpawnDynamicLight, tempEntities)
		renderer.EmitEntityEffectLights(gameRenderer.SpawnKeyedDynamicLight, effectSources)
	}
	if gameParticles != nil {
		renderer.EmitClientEffects(gameParticles, particleEvents, tempEntities, particleRNG, particleTime)
		renderer.EmitEntityEffectParticles(gameParticles, effectSources, particleTime)
		gameParticles.RunParticles(particleTime, oldTime, 800)
	}
	if gameDecalMarks != nil {
		gameDecalMarks.Run(particleTime)
		renderer.EmitDecalMarks(gameDecalMarks, tempEntities, particleRNG, particleTime)
	}
}

func syncRuntimeSkybox() {
	if gameRenderer == nil {
		skyboxNameKey = ""
		return
	}
	skyboxName := ""
	if gameClient != nil && gameClient.State == cl.StateActive {
		skyboxName = gameClient.SkyboxName
	}
	if skyboxName == skyboxNameKey {
		return
	}
	skyboxNameKey = skyboxName
	if skyboxName == "" || gameSubs == nil || gameSubs.Files == nil {
		gameRenderer.SetExternalSkybox("", nil)
		return
	}
	gameRenderer.SetExternalSkybox(skyboxName, gameSubs.Files.LoadFile)
}

// updateHUDFromServer pushes current player/client state into the HUD.
func updateHUDFromServer() {
	if gameHUD == nil {
		return
	}

	if gameClient != nil {
		shells, nails, rockets, cells := gameClient.AmmoCounts()
		gameHUD.SetState(hud.State{
			Health:        gameClient.Health(),
			Armor:         gameClient.Armor(),
			Ammo:          gameClient.Ammo(),
			WeaponModel:   gameClient.WeaponModelIndex(),
			ActiveWeapon:  gameClient.ActiveWeapon(),
			Shells:        shells,
			Nails:         nails,
			Rockets:       rockets,
			Cells:         cells,
			Items:         gameClient.Items,
			ModHipnotic:   gameModDir == "hipnotic",
			ModRogue:      gameModDir == "rogue",
			GameType:      gameClient.GameType,
			MaxClients:    gameClient.MaxClients,
			ShowScores:    gameShowScores && gameClient.MaxClients > 1,
			Scoreboard:    buildHUDScoreboard(gameClient),
			Intermission:  gameClient.Intermission,
			CompletedTime: gameClient.CompletedTime,
			Time:          gameClient.Time,
			CenterPrint:   gameClient.CenterPrint,
			CenterPrintAt: gameClient.CenterPrintAt,
			LevelName:     gameClient.LevelName,
			Secrets:       gameClient.Stats[inet.StatSecrets],
			TotalSecrets:  gameClient.Stats[inet.StatTotalSecrets],
			Monsters:      gameClient.Stats[inet.StatMonsters],
			TotalMonsters: gameClient.Stats[inet.StatTotalMonsters],
		})
		return
	}

	if gameServer == nil {
		return
	}
	ent := gameServer.EdictNum(1)
	if ent == nil {
		return
	}
	gameHUD.SetState(hud.State{
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
