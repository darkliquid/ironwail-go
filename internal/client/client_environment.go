package client

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/darkliquid/ironwail-go/pkg/types"
)

// LightStyleConfig controls lightstyle animation behavior. These correspond
// to the C Ironwail cvars r_flatlightstyles and r_lerplightstyles.
type LightStyleConfig struct {
	// FlatLightStyles controls flattening:
	//   0 = dynamic animation (default)
	//   1 = use average brightness (static)
	//   2 = use peak brightness (static)
	FlatLightStyles int
	// LerpLightStyles controls interpolation between animation frames:
	//   0 = no interpolation (snap to frame)
	//   1 = interpolate, but skip abrupt changes (default)
	//   2 = always interpolate
	LerpLightStyles int
	// DynamicLights enables dynamic light animation. When false, lightstyles
	// use their average value (equivalent to r_flatlightstyles=1).
	DynamicLights bool
}

// DefaultLightStyleConfig returns the default configuration matching
// C Ironwail's default cvar values.
func DefaultLightStyleConfig() LightStyleConfig {
	return LightStyleConfig{
		FlatLightStyles: 0,
		LerpLightStyles: 1,
		DynamicLights:   true,
	}
}

// LightStyleValues evaluates the current lightstyle scalars for the client clock.
func (c *Client) LightStyleValues() [256]float32 {
	return c.LightStyleValuesWithConfig(DefaultLightStyleConfig())
}

// LightStyleValuesWithConfig evaluates lightstyle brightness with the given
// animation configuration, matching C R_AnimateLight() behavior.
func (c *Client) LightStyleValuesWithConfig(cfg LightStyleConfig) [256]float32 {
	var out [256]float32
	for i := range out {
		out[i] = 1
	}
	if c == nil {
		return out
	}
	for i, style := range c.LightStyles {
		out[i] = evalLightStyleValue(style, c.Time, cfg)
	}
	return out
}

// CurrentFog evaluates the client's active fog state at the current client clock.
func (c *Client) CurrentFog() (density float32, color types.Vec3) {
	if c == nil {
		return 0, types.Vec3{}
	}

	targetDensity := float32(c.FogDensity) / 255
	targetColor := types.Vec3{
		X: float32(c.FogColor[0]) / 255,
		Y: float32(c.FogColor[1]) / 255,
		Z: float32(c.FogColor[2]) / 255,
	}
	if c.fogFloatValid {
		targetDensity = c.FogDensityVal
		targetColor = c.FogColorVal
	}

	if c.fogFadeDone > c.Time && c.fogFadeTime > 0 {
		f := float32((c.fogFadeDone - c.Time) / float64(c.fogFadeTime))
		density = f*c.fogOldDensity + (1-f)*targetDensity
		color = c.fogOldColor.Scale(f).Add(targetColor.Scale(1 - f))
	} else {
		density = targetDensity
		color = targetColor
	}

	clampRound := func(v float32) float32 {
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		return float32(math.Round(float64(v*255))) / 255
	}
	color.X = clampRound(color.X)
	color.Y = clampRound(color.Y)
	color.Z = clampRound(color.Z)
	return density, color
}

func (c *Client) SetFogState(density byte, color [3]byte, time float32) {
	if c == nil {
		return
	}
	oldDensity, oldColor := c.CurrentFog()
	c.fogOldDensity = oldDensity
	c.fogOldColor = oldColor
	c.FogDensity = density
	c.FogColor = color
	c.FogDensityVal = float32(density) / 255
	c.FogColorVal = types.Vec3{
		X: float32(color[0]) / 255,
		Y: float32(color[1]) / 255,
		Z: float32(color[2]) / 255,
	}
	c.fogFloatValid = false
	c.FogTime = time
	c.fogFadeTime = time
	c.fogFadeDone = c.Time + float64(time)
	c.fogConfigured = true
}

func (c *Client) SetFogStateFloat(density float32, color types.Vec3, time float32) {
	if c == nil {
		return
	}
	oldDensity, oldColor := c.CurrentFog()
	c.fogOldDensity = oldDensity
	c.fogOldColor = oldColor
	c.FogDensityVal = density
	c.FogColorVal = color
	c.FogDensity = fogByteFromFloat(density)
	c.FogColor = [3]byte{
		fogByteFromFloat(color.X),
		fogByteFromFloat(color.Y),
		fogByteFromFloat(color.Z),
	}
	c.fogFloatValid = true
	c.FogTime = time
	c.fogFadeTime = time
	c.fogFadeDone = c.Time + float64(time)
	c.fogConfigured = true
}

func (c *Client) FogValues() (density float32, color types.Vec3) {
	if c == nil {
		return 0, types.Vec3{}
	}
	if c.fogFloatValid {
		return c.FogDensityVal, c.FogColorVal
	}
	return float32(c.FogDensity) / 255, types.Vec3{
		X: float32(c.FogColor[0]) / 255,
		Y: float32(c.FogColor[1]) / 255,
		Z: float32(c.FogColor[2]) / 255,
	}
}

func (c *Client) ApplyWorldspawnFogDefaults(entities []byte) {
	if c == nil || c.fogConfigured || len(entities) == 0 {
		return
	}

	density, color, ok := worldspawnFogStateFromEntities(entities)
	if !ok {
		return
	}

	c.FogDensityVal = density
	c.FogColorVal = color
	c.FogDensity = fogByteFromFloat(density)
	c.FogColor = [3]byte{
		fogByteFromFloat(color.X),
		fogByteFromFloat(color.Y),
		fogByteFromFloat(color.Z),
	}
	c.fogFloatValid = true
	c.FogTime = 0
	c.fogOldDensity = density
	c.fogOldColor = color
	c.fogFadeDone = 0
	c.fogFadeTime = 0
	c.fogConfigured = true
}

func worldspawnFogStateFromEntities(entities []byte) (float32, types.Vec3, bool) {
	if len(entities) == 0 {
		return 0, types.Vec3{}, false
	}

	entity, ok := firstEntityLumpObject(string(entities))
	if !ok {
		return 0, types.Vec3{}, false
	}

	fields := parseEntityFields(entity)
	if !strings.EqualFold(strings.TrimSpace(fields["classname"]), "worldspawn") {
		return 0, types.Vec3{}, false
	}

	const defaultGray = 0.3
	density := float32(0)
	color := types.Vec3{X: defaultGray, Y: defaultGray, Z: defaultGray}

	value, ok := fields["fog"]
	if !ok {
		value, ok = fields["_fog"]
	}
	if ok {
		var parsedDensity, red, green, blue float32
		parsedDensity = density
		red, green, blue = color.X, color.Y, color.Z
		if n, _ := fmt.Sscanf(value, "%f %f %f %f", &parsedDensity, &red, &green, &blue); n >= 1 {
			density = parsedDensity
			color = types.Vec3{X: red, Y: green, Z: blue}
		}
	}

	return density, color, true
}

func parseEntityFields(entity string) map[string]string {
	fields := make(map[string]string)
	pos := 0
	for {
		key, next, ok := nextQuotedEntityToken(entity, pos)
		if !ok {
			break
		}
		value, nextValue, ok := nextQuotedEntityToken(entity, next)
		if !ok {
			break
		}
		key = strings.TrimPrefix(key, "_")
		fields[strings.ToLower(key)] = value
		pos = nextValue
	}
	return fields
}

func firstEntityLumpObject(data string) (string, bool) {
	start := strings.IndexByte(data, '{')
	if start < 0 {
		return "", false
	}
	end := strings.IndexByte(data[start+1:], '}')
	if end < 0 {
		return "", false
	}
	return data[start+1 : start+1+end], true
}

func nextQuotedEntityToken(data string, pos int) (string, int, bool) {
	start := strings.IndexByte(data[pos:], '"')
	if start < 0 {
		return "", pos, false
	}
	start += pos
	end := strings.IndexByte(data[start+1:], '"')
	if end < 0 {
		return "", pos, false
	}
	end += start + 1
	return data[start+1 : end], end + 1, true
}

func fogByteFromFloat(value float32) byte {
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}
	return byte(math.Round(float64(value * 255)))
}

// lightstyleNormalBrightness is the brightness of character 'm' (normal light).
const lightstyleNormalBrightness = float32('m' - 'a') // 12

func evalLightStyleValue(style LightStyle, timeSeconds float64, cfg LightStyleConfig) float32 {
	if style.Length <= 0 || style.Map == "" {
		return 1
	}

	// Flat lightstyle modes use precomputed average/peak.
	if cfg.FlatLightStyles == 2 {
		return float32(style.Peak-'a') / lightstyleNormalBrightness
	}
	if cfg.FlatLightStyles == 1 || !cfg.DynamicLights {
		return float32(style.Average-'a') / lightstyleNormalBrightness
	}

	// Dynamic animation: 10 frames per second, matching C cl.time * 10.0.
	f := timeSeconds * 10.0
	base := math.Floor(f)
	frac := float32(f - base)
	if cfg.LerpLightStyles == 0 {
		frac = 0
	}

	idx := int(base) % style.Length
	if idx < 0 {
		idx += style.Length
	}
	next := (idx + 1) % style.Length

	k := float32(style.Map[idx] - 'a')
	n := float32(style.Map[next] - 'a')

	// Skip interpolation for abrupt changes (e.g. flickering) unless
	// r_lerplightstyles >= 2, matching C behavior.
	if cfg.LerpLightStyles < 2 {
		abruptThreshold := lightstyleNormalBrightness / 2 // 6
		diff := k - n
		if diff < 0 {
			diff = -diff
		}
		if diff >= abruptThreshold {
			n = k
		}
	}

	return (k + (n-k)*frac) / lightstyleNormalBrightness
}

func (c *Client) SetLightStyle(i int, style string) error {
	if i < 0 || i >= len(c.LightStyles) {
		return errors.New("lightstyle index out of range")
	}
	ls := &c.LightStyles[i]
	ls.Map = style
	ls.Length = len(style)
	if ls.Length == 0 {
		ls.Average = 'm'
		ls.Peak = 'm'
		return nil
	}
	total := 0
	peak := byte('a')
	for j := 0; j < len(style); j++ {
		ch := style[j]
		total += int(ch - 'a')
		if ch > peak {
			peak = ch
		}
	}
	ls.Peak = peak
	ls.Average = byte(total/ls.Length) + 'a'
	return nil
}
