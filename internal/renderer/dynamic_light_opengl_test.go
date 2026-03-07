//go:build opengl || cgo
// +build opengl cgo

package renderer

import (
	"math"
	"testing"
)

func TestDynamicLightCreation(t *testing.T) {
	light := DynamicLight{
		Position:   [3]float32{10, 20, 30},
		Radius:     100,
		Color:      [3]float32{1, 0.5, 0},
		Brightness: 1.5,
		Lifetime:   5.0,
		Age:        0,
		Type:       0,
	}

	if light.Position[0] != 10 || light.Position[1] != 20 || light.Position[2] != 30 {
		t.Errorf("position not set correctly")
	}
	if light.Radius != 100 {
		t.Errorf("radius not set correctly")
	}
	if light.IsAlive() != true {
		t.Errorf("new light should be alive")
	}
}

func TestDynamicLightExpiry(t *testing.T) {
	light := DynamicLight{
		Lifetime: 5.0,
		Age:      0,
	}

	if !light.IsAlive() {
		t.Errorf("light at age 0 should be alive")
	}

	light.Age = 4.9
	if !light.IsAlive() {
		t.Errorf("light at age 4.9 (< 5.0) should be alive")
	}

	light.Age = 5.0
	if light.IsAlive() {
		t.Errorf("light at age 5.0 (== lifetime) should not be alive")
	}

	light.Age = 10.0
	if light.IsAlive() {
		t.Errorf("light at age 10.0 (> lifetime) should not be alive")
	}
}

func TestDynamicLightFadeMultiplier(t *testing.T) {
	light := DynamicLight{
		Lifetime: 10.0,
		Age:      0,
	}

	// At age 0, fade should be 1.0
	fade := light.FadeMultiplier()
	if fade != 1.0 {
		t.Errorf("expected fade 1.0 at age 0, got %f", fade)
	}

	// At age 5 (halfway), fade should be 0.5
	light.Age = 5.0
	fade = light.FadeMultiplier()
	if fade != 0.5 {
		t.Errorf("expected fade 0.5 at age 5, got %f", fade)
	}

	// At age 10 (end of life), fade should be 0.0
	light.Age = 10.0
	fade = light.FadeMultiplier()
	if fade != 0.0 {
		t.Errorf("expected fade 0.0 at age 10, got %f", fade)
	}

	// Beyond lifetime should clamp to 0
	light.Age = 15.0
	fade = light.FadeMultiplier()
	if fade != 0.0 {
		t.Errorf("expected fade 0.0 beyond lifetime, got %f", fade)
	}
}

func TestEvalLightContributionAtCenter(t *testing.T) {
	light := DynamicLight{
		Position:   [3]float32{0, 0, 0},
		Radius:     100,
		Color:      [3]float32{1, 1, 1},
		Brightness: 1.0,
		Lifetime:   10.0,
		Age:        0,
	}

	// At light center, falloff should be 1.0
	point := [3]float32{0, 0, 0}
	contrib := evalLightContribution(&light, point)

	expectedBrightness := float32(1.0) // brightness * falloff * fade = 1.0 * 1.0 * 1.0
	expectedContrib := [3]float32{expectedBrightness, expectedBrightness, expectedBrightness}

	if contrib[0] != expectedContrib[0] || contrib[1] != expectedContrib[1] || contrib[2] != expectedContrib[2] {
		t.Errorf("expected %v at light center, got %v", expectedContrib, contrib)
	}
}

func TestEvalLightContributionFalloff(t *testing.T) {
	light := DynamicLight{
		Position:   [3]float32{0, 0, 0},
		Radius:     100,
		Color:      [3]float32{1, 1, 1},
		Brightness: 1.0,
		Lifetime:   10.0,
		Age:        0,
	}

	// At 50 units away (halfway), falloff should be 0.5
	point := [3]float32{50, 0, 0}
	contrib := evalLightContribution(&light, point)

	expectedBrightness := float32(0.5) // brightness * falloff * fade = 1.0 * 0.5 * 1.0
	tolerance := float32(1e-6)

	if math.Abs(float64(contrib[0]-expectedBrightness)) > 1e-6 {
		t.Errorf("expected brightness %f at 50 units, got %f", expectedBrightness, contrib[0])
	}

	// At edge of light (100 units), falloff should ~1.0 - 1.0 = 0.0
	point = [3]float32{100, 0, 0}
	contrib = evalLightContribution(&light, point)

	if float64(contrib[0]) > float64(tolerance) {
		t.Errorf("expected brightness ~0 at light edge, got %f", contrib[0])
	}

	// Beyond light (150 units), contribution should be zero
	point = [3]float32{150, 0, 0}
	contrib = evalLightContribution(&light, point)

	if contrib[0] != 0 || contrib[1] != 0 || contrib[2] != 0 {
		t.Errorf("expected zero contribution outside light radius, got %v", contrib)
	}
}

func TestEvalLightContributionWithBrightness(t *testing.T) {
	light := DynamicLight{
		Position:   [3]float32{0, 0, 0},
		Radius:     100,
		Color:      [3]float32{1, 0.5, 0.25},
		Brightness: 2.0,
		Lifetime:   10.0,
		Age:        0,
	}

	// At center with brightness 2.0
	point := [3]float32{0, 0, 0}
	contrib := evalLightContribution(&light, point)

	expected := [3]float32{2.0, 1.0, 0.5} // color * brightness * falloff * fade = color * 2.0 * 1.0 * 1.0
	tolerance := float32(1e-6)

	for i := range contrib {
		if math.Abs(float64(contrib[i]-expected[i])) > float64(tolerance) {
			t.Errorf("expected color %v at center, got %v", expected, contrib)
			break
		}
	}
}

func TestEvalLightContributionWithFade(t *testing.T) {
	light := DynamicLight{
		Position:   [3]float32{0, 0, 0},
		Radius:     100,
		Color:      [3]float32{1, 1, 1},
		Brightness: 1.0,
		Lifetime:   10.0,
		Age:        5.0, // halfway through life, fade = 0.5
	}

	// At center with 50% fade
	point := [3]float32{0, 0, 0}
	contrib := evalLightContribution(&light, point)

	expectedBrightness := float32(0.5) // brightness * falloff * fade = 1.0 * 1.0 * 0.5
	tolerance := float32(1e-6)

	if math.Abs(float64(contrib[0]-expectedBrightness)) > float64(tolerance) {
		t.Errorf("expected brightness %f with 50%% fade, got %f", expectedBrightness, contrib[0])
	}
}

func TestGLLightPoolCreation(t *testing.T) {
	pool := NewGLLightPool(256)

	if pool == nil {
		t.Errorf("pool should not be nil")
	}
	if pool.maxLights != 256 {
		t.Errorf("expected max lights 256, got %d", pool.maxLights)
	}
	if pool.ActiveCount() != 0 {
		t.Errorf("new pool should have 0 active lights")
	}
}

func TestGLLightPoolDefaultCapacity(t *testing.T) {
	pool := NewGLLightPool(0)
	if pool.maxLights != 512 {
		t.Errorf("default max lights should be 512, got %d", pool.maxLights)
	}
}

func TestGLLightPoolSpawnLight(t *testing.T) {
	pool := NewGLLightPool(2)

	light1 := DynamicLight{Position: [3]float32{0, 0, 0}, Radius: 100}
	light2 := DynamicLight{Position: [3]float32{100, 100, 100}, Radius: 50}
	light3 := DynamicLight{Position: [3]float32{200, 200, 200}, Radius: 75}

	// First two lights should succeed
	if !pool.SpawnLight(light1) {
		t.Errorf("expected first spawn to succeed")
	}
	if !pool.SpawnLight(light2) {
		t.Errorf("expected second spawn to succeed")
	}

	if pool.ActiveCount() != 2 {
		t.Errorf("expected 2 active lights, got %d", pool.ActiveCount())
	}

	// Third light should fail (pool full)
	if pool.SpawnLight(light3) {
		t.Errorf("expected third spawn to fail (pool full)")
	}

	if pool.ActiveCount() != 2 {
		t.Errorf("pool should still have 2 lights after failed spawn")
	}
}

func TestGLLightPoolUpdate(t *testing.T) {
	pool := NewGLLightPool(10)

	light1 := DynamicLight{Lifetime: 5.0, Age: 0}
	light2 := DynamicLight{Lifetime: 10.0, Age: 0}

	pool.SpawnLight(light1)
	pool.SpawnLight(light2)

	if pool.ActiveCount() != 2 {
		t.Errorf("expected 2 lights before update")
	}

	// Update with 6 seconds elapsed
	pool.UpdateAndFilter(6.0)

	// light1 should have expired (age 6 > lifetime 5)
	// light2 should still be alive (age 6 < lifetime 10)
	if pool.ActiveCount() != 1 {
		t.Errorf("expected 1 light after filtering (one expired), got %d", pool.ActiveCount())
	}

	// Check that remaining light is light2
	if len(pool.lights) > 0 && pool.lights[0].Lifetime != 10.0 {
		t.Errorf("expected remaining light to be light2 (lifetime 10)")
	}
}

func TestGLLightPoolMultipleUpdates(t *testing.T) {
	pool := NewGLLightPool(10)

	light := DynamicLight{Lifetime: 10.0, Age: 0}
	pool.SpawnLight(light)

	// Update incrementally
	pool.UpdateAndFilter(2.0)
	if pool.ActiveCount() != 1 {
		t.Errorf("light should be alive after 2s")
	}

	pool.UpdateAndFilter(3.0)
	if pool.ActiveCount() != 1 {
		t.Errorf("light should be alive after 5s total")
	}

	pool.UpdateAndFilter(6.0)
	if pool.ActiveCount() != 0 {
		t.Errorf("light should be expired after 11s total")
	}
}

func TestEvaluateLightsAtPoint(t *testing.T) {
	pool := NewGLLightPool(10)

	// Spawn two lights
	light1 := DynamicLight{
		Position:   [3]float32{0, 0, 0},
		Radius:     100,
		Color:      [3]float32{1, 0, 0}, // Red
		Brightness: 1.0,
		Lifetime:   10.0,
		Age:        0,
	}
	light2 := DynamicLight{
		Position:   [3]float32{50, 0, 0},
		Radius:     100,
		Color:      [3]float32{0, 1, 0}, // Green
		Brightness: 1.0,
		Lifetime:   10.0,
		Age:        0,
	}

	pool.SpawnLight(light1)
	pool.SpawnLight(light2)

	// At point (25, 0, 0): equal distance from both lights
	// Distance from light1: 25, falloff = 1 - 25/100 = 0.75
	// Distance from light2: 25, falloff = 1 - 25/100 = 0.75
	// Expected: (0.75 * red) + (0.75 * green) = (0.75, 0.75, 0)

	point := [3]float32{25, 0, 0}
	result := pool.EvaluateLightsAtPoint(point)

	expectedR := float32(0.75)
	expectedG := float32(0.75)
	expectedB := float32(0.0)

	tolerance := float32(1e-6)
	if math.Abs(float64(result[0]-expectedR)) > float64(tolerance) ||
		math.Abs(float64(result[1]-expectedG)) > float64(tolerance) ||
		math.Abs(float64(result[2]-expectedB)) > float64(tolerance) {
		t.Errorf("expected (%.2f, %.2f, %.2f), got (%.2f, %.2f, %.2f)",
			expectedR, expectedG, expectedB, result[0], result[1], result[2])
	}
}

func TestAccumLightsPerFace(t *testing.T) {
	pool := NewGLLightPool(10)

	light := DynamicLight{
		Position:   [3]float32{0, 0, 0},
		Radius:     100,
		Color:      [3]float32{1, 1, 1},
		Brightness: 1.0,
		Lifetime:   10.0,
		Age:        0,
	}
	pool.SpawnLight(light)

	faces := []WorldFace{
		{
			FirstIndex:    0,
			NumIndices:    6,
			TextureIndex:  0,
			LightmapIndex: 0,
			Flags:         0,
			Center:        [3]float32{0, 0, 0}, // At light center
		},
		{
			FirstIndex:    6,
			NumIndices:    6,
			TextureIndex:  0,
			LightmapIndex: 0,
			Flags:         0,
			Center:        [3]float32{50, 0, 0}, // 50 units away
		},
		{
			FirstIndex:    12,
			NumIndices:    6,
			TextureIndex:  0,
			LightmapIndex: 0,
			Flags:         0,
			Center:        [3]float32{200, 0, 0}, // Outside light radius
		},
	}

	accum := pool.AccumLightsPerFace(faces)

	if len(accum) != 3 {
		t.Errorf("expected 3 accum entries, got %d", len(accum))
	}

	// Face 0 should have brightness 1.0 (at center)
	if accum[0].Light[0] != 1.0 {
		t.Errorf("expected brightness 1.0 for face at light center, got %f", accum[0].Light[0])
	}

	// Face 1 should have brightness 0.5 (halfway)
	if math.Abs(float64(accum[1].Light[0]-0.5)) > 1e-6 {
		t.Errorf("expected brightness 0.5 for face at 50 units, got %f", accum[1].Light[0])
	}

	// Face 2 should have brightness 0 (outside radius)
	if accum[2].Light[0] != 0 || accum[2].Light[1] != 0 || accum[2].Light[2] != 0 {
		t.Errorf("expected brightness 0 for face outside radius, got %v", accum[2].Light)
	}
}

func TestClear(t *testing.T) {
	pool := NewGLLightPool(10)

	light1 := DynamicLight{Position: [3]float32{0, 0, 0}, Radius: 100}
	light2 := DynamicLight{Position: [3]float32{100, 100, 100}, Radius: 50}

	pool.SpawnLight(light1)
	pool.SpawnLight(light2)

	if pool.ActiveCount() != 2 {
		t.Errorf("expected 2 lights before clear")
	}

	pool.Clear()

	if pool.ActiveCount() != 0 {
		t.Errorf("expected 0 lights after clear")
	}
}

func TestApplyLightToColorAdditive(t *testing.T) {
	baseColor := [3]float32{0.5, 0.5, 0.5}
	lightContrib := [3]float32{0.3, 0.2, 0.1}

	result := ApplyLightToColor(baseColor, lightContrib, LightModeAdditive)

	expected := [3]float32{0.8, 0.7, 0.6}
	for i := range result {
		if result[i] != expected[i] {
			t.Errorf("additive blend: expected %v, got %v", expected, result)
			break
		}
	}
}

func TestApplyLightToColorAdditiveClamp(t *testing.T) {
	baseColor := [3]float32{0.8, 0.8, 0.8}
	lightContrib := [3]float32{0.5, 0.5, 0.5}

	result := ApplyLightToColor(baseColor, lightContrib, LightModeAdditive)

	// Should clamp to 1.0
	expected := [3]float32{1.0, 1.0, 1.0}
	for i := range result {
		if result[i] != expected[i] {
			t.Errorf("additive clamp: expected %v, got %v", expected, result)
			break
		}
	}
}

func TestApplyLightToColorModulate(t *testing.T) {
	baseColor := [3]float32{0.5, 0.5, 0.5}
	lightContrib := [3]float32{0.3, 0.2, 0.1}

	result := ApplyLightToColor(baseColor, lightContrib, LightModeModulate)

	// Light acts as multiplier: baseColor * (1 + lightContrib)
	expected := [3]float32{
		0.5 * 1.3,
		0.5 * 1.2,
		0.5 * 1.1,
	}
	tolerance := float32(1e-6)
	for i := range result {
		if math.Abs(float64(result[i]-expected[i])) > float64(tolerance) {
			t.Errorf("modulate mode: expected %v, got %v", expected, result)
			break
		}
	}
}

func TestApplyLightToColorBlend(t *testing.T) {
	baseColor := [3]float32{0.2, 0.3, 0.4}
	lightContrib := [3]float32{1.0, 1.0, 1.0}

	result := ApplyLightToColor(baseColor, lightContrib, LightModeBlend)

	// Blend mode with white light should move towards white
	// alpha = (1 + 1 + 1) / 3 = 1.0
	// result = base * (1 - 1.0) + light * 1.0 = light
	expected := [3]float32{1.0, 1.0, 1.0}
	tolerance := float32(1e-6)
	for i := range result {
		if math.Abs(float64(result[i]-expected[i])) > float64(tolerance) {
			t.Errorf("blend mode: expected %v, got %v", expected, result)
			break
		}
	}
}

func BenchmarkEvalLightContribution(b *testing.B) {
	light := DynamicLight{
		Position:   [3]float32{0, 0, 0},
		Radius:     100,
		Color:      [3]float32{1, 1, 1},
		Brightness: 1.0,
		Lifetime:   10.0,
		Age:        0,
	}
	point := [3]float32{50, 50, 50}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = evalLightContribution(&light, point)
	}
}

func BenchmarkEvaluateLightsAtPoint(b *testing.B) {
	pool := NewGLLightPool(64)

	// Add 64 lights
	for i := 0; i < 64; i++ {
		light := DynamicLight{
			Position:   [3]float32{float32(i), float32(i), float32(i)},
			Radius:     100,
			Color:      [3]float32{1, 1, 1},
			Brightness: 1.0,
			Lifetime:   10.0,
			Age:        0,
		}
		pool.SpawnLight(light)
	}

	point := [3]float32{50, 50, 50}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pool.EvaluateLightsAtPoint(point)
	}
}

func BenchmarkAccumLightsPerFace(b *testing.B) {
	pool := NewGLLightPool(64)

	// Add 64 lights
	for i := 0; i < 64; i++ {
		light := DynamicLight{
			Position:   [3]float32{float32(i * 10), 0, 0},
			Radius:     100,
			Color:      [3]float32{1, 1, 1},
			Brightness: 1.0,
			Lifetime:   10.0,
			Age:        0,
		}
		pool.SpawnLight(light)
	}

	// Create 256 faces
	faces := make([]WorldFace, 256)
	for i := 0; i < 256; i++ {
		faces[i] = WorldFace{
			Center: [3]float32{float32(i*5) - 500, 0, 0},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pool.AccumLightsPerFace(faces)
	}
}
