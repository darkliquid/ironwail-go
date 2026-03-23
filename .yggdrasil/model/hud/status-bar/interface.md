# Interface

## Main consumers

- `hud/runtime`, which constructs one `StatusBar` and invokes either classic or QuakeWorld drawing depending on HUD style.

## Main surface

- `NewStatusBar`
- `Draw`
- `DrawQuakeWorld`

## Contracts

- Viewsize controls whether inventory and/or the main status bar render at all.
- Deathmatch scoreboard mode overrides the usual status layout when scores are shown or the player is dead.
- Expansion-pack flags in `State` (`ModHipnotic`, `ModRogue`) redirect several icon and inventory paths.
