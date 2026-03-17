package renderer

// DynamicLight represents a temporary point light source in the world.
// Dynamic lights fade over time and are used for explosions, beams, and other transient effects.
type DynamicLight struct {
	// Position is the light source center in world space [X, Y, Z]
	Position [3]float32

	// Radius is the distance at which light falloff reaches zero
	Radius float32

	// Color is the RGB light color in linear space [0-1]
	Color [3]float32

	// Brightness is the light intensity multiplier, typically 1.0-2.0
	Brightness float32

	// Lifetime is the total lifespan of the light in seconds
	Lifetime float32

	// Age is the current age of the light in seconds (incremented per frame)
	Age float32

	// Type identifies the light source for optional filtering
	Type int

	// EntityKey is the entity number this light is bound to, enabling per-entity
	// slot reuse (mirrors C's CL_AllocDlight key parameter). Zero means no key.
	EntityKey int
}

// IsAlive returns true if the light has not yet expired.
func (l *DynamicLight) IsAlive() bool {
	return l.Age < l.Lifetime
}

// FadeMultiplier returns a brightness fade factor as the light ages.
func (l *DynamicLight) FadeMultiplier() float32 {
	if l.Lifetime <= 0 {
		return 1.0
	}
	remaining := 1.0 - (l.Age / l.Lifetime)
	if remaining < 0 {
		remaining = 0
	}
	return remaining
}
