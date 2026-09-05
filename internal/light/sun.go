package light

import (
	"bytes"
	"fmt"
	"math"
	"strconv"

	"github.com/darkliquid/ironwail-go/internal/qbsp"
)

// Sun is a directional sky light: a `sun` entity or worldspawn sunlight/
// sun_mangle/sunlight_color keys. Sky faces receive no lightmap but act as
// the sun's source; non-sky faces accumulate sun intensity scaled by the
// cosine of the face normal vs the sun direction.
type Sun struct {
	Dir   [3]float64 // direction the light travels (from the sky)
	Value float64
	Color [3]float64
}

// ParseSun reads the sun from the BSP's entity lump: worldspawn keys
// (sunlight, sun_mangle, sunlight_color) or a `sun` entity (light,
// angles). Returns nil when no sun is defined.
func ParseSun(bspData []byte) (*Sun, error) {
	_, lumps, err := qbsp.ReadBSPLumps(bytes.NewReader(bspData))
	if err != nil {
		return nil, err
	}
	ents := parseEntities(lumps[0])
	if len(ents) == 0 {
		return nil, nil
	}
	world := ents[0]
	lightKey, angleKey, colorKey := "sunlight", "sun_mangle", "sunlight_color"
	for _, e := range ents {
		if e["classname"] == "sun" {
			world = e
			lightKey, angleKey, colorKey = "light", "angles", "color"
		}
	}
	str, ok := world[lightKey]
	if !ok {
		return nil, nil
	}
	value, err := strconv.ParseFloat(str, 64)
	if err != nil || value <= 0 {
		return nil, nil
	}
	sun := &Sun{Value: value, Color: [3]float64{255, 255, 255}, Dir: [3]float64{0, 0, -1}}
	if v, ok := world[angleKey]; ok {
		if yaw, pitch, err := parseMangle(v); err == nil {
			sun.Dir = mangleToDir(yaw, pitch)
		}
	}
	if v, ok := world[colorKey]; ok {
		var c [3]float64
		if _, err := fmt.Sscanf(v, "%f %f %f", &c[0], &c[1], &c[2]); err == nil {
			sun.Color = c
		}
	}
	return sun, nil
}

// parseMangle parses "yaw pitch" in degrees.
func parseMangle(s string) (yaw, pitch float64, err error) {
	var y, p float64
	if _, err2 := fmt.Sscanf(s, "%f %f", &y, &p); err2 != nil {
		return 0, 0, err2
	}
	return y, p, nil
}

// mangleToDir converts sun_mangle yaw/pitch (degrees, pointing INTO the
// world) to a direction vector.
func mangleToDir(yaw, pitch float64) [3]float64 {
	yr := yaw * math.Pi / 180
	pr := pitch * math.Pi / 180
	return [3]float64{
		math.Cos(yr) * math.Cos(pr),
		math.Sin(yr) * math.Cos(pr),
		math.Sin(pr),
	}
}

// SunLight computes the direct sun contribution at a sample point on a
// face: intensity * max(dot(normal, -sun.Dir), 0) * color.
func (s *Sun) SunLight(f *Face, n [3]float64, p [3]float64) (float64, float64, float64) {
	cos := -(n[0]*s.Dir[0] + n[1]*s.Dir[1] + n[2]*s.Dir[2])
	if cos <= 0 {
		return 0, 0, 0
	}
	col := s.Color
	if col == [3]float64{} {
		col = [3]float64{255, 255, 255}
	}
	sc := math.Min(s.Value*cos, 255)
	return math.Min(sc*col[0]/255, 255), math.Min(sc*col[1]/255, 255), math.Min(sc*col[2]/255, 255)
}
