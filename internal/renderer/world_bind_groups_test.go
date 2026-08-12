package renderer

import (
	"os"
	"strings"
	"testing"
)

// TestWorldBindGroupLayoutFitsBrowserCap guards the bind-group consolidation:
// browsers cap pipeline layouts at 4 bind groups and reject render passes
// that bind group indices beyond the pipeline layout. The world pipeline
// layout, the external-sky pipeline layout, and every world shader must stay
// within groups 0-3.
func TestWorldBindGroupLayoutFitsBrowserCap(t *testing.T) {
	// 1. No world shader may reference group(4) or higher.
	for name, src := range map[string]string{
		"world_fragment":     worldFragmentShaderWGSL,
		"world_alpha_frag":   worldAlphaTestFragmentShaderWGSL,
		"world_turbulent":    worldTurbulentFragmentShaderWGSL,
		"world_sky":          worldSkyFragmentShaderWGSL,
		"world_sky_external": worldSkyExternalFaceFragmentShaderWGSL,
		"cluster_compute":    worldClusterComputeShaderWGSL,
	} {
		if strings.Contains(src, "@group(4)") {
			t.Errorf("%s references @group(4); browser WebGPU caps at 4 groups (0-3)", name)
		}
	}

	// 2. The pipeline layout arrays must chain at most 4 bind group layouts.
	src, err := os.ReadFile("pipeline/world_pipelines.go")
	if err != nil {
		t.Fatalf("ReadFile(pipeline/world_pipelines.go): %v", err)
	}
	lines := strings.Split(string(src), "\n")
	for i, line := range lines {
		if !strings.Contains(line, "BindGroupLayouts: []*wgpu.BindGroupLayout{") {
			continue
		}
		var depth, count int
		for j := i; j < len(lines); j++ {
			depth += strings.Count(lines[j], "{") - strings.Count(lines[j], "}")
			count += strings.Count(lines[j], ",")
			if depth <= 0 {
				break
			}
		}
		if count > 4 {
			t.Errorf("pipeline/world_pipelines.go:%d chains %d bind group layouts, browser cap is 4", i+1, count)
		}
	}
}
