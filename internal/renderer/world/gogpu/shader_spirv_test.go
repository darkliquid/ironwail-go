//go:build gogpu && !cgo

package gogpu

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gogpu/naga"
)

// TestShaderSPIRVValidation compiles all WGSL shaders to SPIR-V and
// validates with spirv-val. Catches naga codegen bugs that crash NVIDIA.
func TestShaderSPIRVValidation(t *testing.T) {
	spirvVal, err := exec.LookPath("spirv-val")
	if err != nil {
		t.Skip("spirv-val not found, skipping SPIR-V validation")
	}

	shaders := map[string]string{
		"alias_vertex":   AliasVertexShaderWGSL,
		"alias_fragment": AliasFragmentShaderWGSL,
		"sprite_vertex":  SpriteVertexShaderWGSL,
		"sprite_frag":    SpriteFragmentShaderWGSL,
		"decal_vertex":   DecalVertexShaderWGSL,
		"decal_fragment": DecalFragmentShaderWGSL,
	}

	tmpDir := t.TempDir()

	for name, wgsl := range shaders {
		t.Run(name, func(t *testing.T) {
			spirvBytes, err := naga.Compile(wgsl)
			if err != nil {
				t.Fatalf("naga.Compile failed: %v", err)
			}
			if len(spirvBytes) < 4 || len(spirvBytes)%4 != 0 {
				t.Fatalf("invalid SPIR-V size: %d", len(spirvBytes))
			}
			if magic := binary.LittleEndian.Uint32(spirvBytes[:4]); magic != 0x07230203 {
				t.Fatalf("invalid SPIR-V magic: 0x%08X", magic)
			}

			spvFile := filepath.Join(tmpDir, name+".spv")
			if err := os.WriteFile(spvFile, spirvBytes, 0644); err != nil {
				t.Fatalf("write SPIR-V: %v", err)
			}

			cmd := exec.Command(spirvVal, spvFile)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Errorf("spirv-val FAILED:\n%s", string(output))
			} else {
				t.Logf("spirv-val PASSED (%d bytes)", len(spirvBytes))
			}
		})
	}
}
