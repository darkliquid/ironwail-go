//go:build gogpu

package renderer

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gogpu/naga"
)

// TestWorldShaderSPIRVValidation compiles all world WGSL shaders to SPIR-V
// via naga and validates them with spirv-val. This catches NVIDIA driver
// crashes caused by invalid SPIR-V that AMD/Mesa silently accepts.
func TestWorldShaderSPIRVValidation(t *testing.T) {
	spirvVal, err := exec.LookPath("spirv-val")
	if err != nil {
		t.Skip("spirv-val not found, skipping SPIR-V validation")
	}

	shaders := map[string]string{
		"world_vertex":       worldVertexShaderWGSL,
		"world_fragment":     worldFragmentShaderWGSL,
		"sky_vertex":         worldSkyVertexShaderWGSL,
		"sky_fragment":       worldSkyFragmentShaderWGSL,
		"sky_external_frag":  worldSkyExternalFaceFragmentShaderWGSL,
		"turbulent_fragment": worldTurbulentFragmentShaderWGSL,
	}

	tmpDir := t.TempDir()

	for name, wgsl := range shaders {
		t.Run(name, func(t *testing.T) {
			spirvBytes, err := naga.Compile(wgsl)
			if err != nil {
				t.Fatalf("naga.Compile failed: %v", err)
			}
			if len(spirvBytes) == 0 {
				t.Fatalf("naga.Compile returned empty SPIR-V")
			}
			if len(spirvBytes)%4 != 0 {
				t.Fatalf("SPIR-V size not aligned to 4 bytes: %d", len(spirvBytes))
			}

			// Check SPIR-V magic number
			if len(spirvBytes) >= 4 {
				magic := binary.LittleEndian.Uint32(spirvBytes[:4])
				if magic != 0x07230203 {
					t.Fatalf("Invalid SPIR-V magic: 0x%08X (expected 0x07230203)", magic)
				}
			}

			// Write to file and validate with spirv-val
			spvFile := filepath.Join(tmpDir, name+".spv")
			if err := os.WriteFile(spvFile, spirvBytes, 0644); err != nil {
				t.Fatalf("Failed to write SPIR-V file: %v", err)
			}

			cmd := exec.Command(spirvVal, spvFile)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Errorf("spirv-val FAILED for %s:\n%s", name, string(output))
			} else {
				t.Logf("spirv-val PASSED for %s (%d bytes)", name, len(spirvBytes))
			}
		})
	}
}
