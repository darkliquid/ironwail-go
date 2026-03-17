package renderer

// worldLiquidAlphaSettings stores per-liquid-type alpha values read from console
// variables (r_wateralpha, r_lavaalpha, r_slimealpha, r_telealpha). These control
// the transparency of water, lava, slime, and teleporter surfaces during the world
// liquid render passes, allowing mappers and players to configure liquid visibility.
type worldLiquidAlphaSettings struct {
	water float32
	lava  float32
	slime float32
	tele  float32
}
