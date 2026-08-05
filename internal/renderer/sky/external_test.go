package sky

import (
	"testing"
)

func TestSelectExternalSkyboxRenderMode(t *testing.T) {
	tests := []struct {
		name            string
		loaded          int
		cubemapEligible bool
		want            ExternalSkyboxRenderMode
	}{
		{
			name:            "no skybox faces loaded",
			loaded:          0,
			cubemapEligible: false,
			want:            ExternalSkyboxRenderEmbedded,
		},
		{
			name:            "partial skybox faces loaded",
			loaded:          3,
			cubemapEligible: false,
			want:            ExternalSkyboxRenderFaces,
		},
		{
			name:            "all faces loaded but cubemap ineligible",
			loaded:          6,
			cubemapEligible: false,
			want:            ExternalSkyboxRenderFaces,
		},
		{
			name:            "all 6 faces loaded and cubemap eligible",
			loaded:          6,
			cubemapEligible: true,
			want:            ExternalSkyboxRenderCubemap,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SelectExternalSkyboxRenderMode(tt.loaded, tt.cubemapEligible)
			if got != tt.want {
				t.Errorf("SelectExternalSkyboxRenderMode(%d, %v) = %v, want %v", tt.loaded, tt.cubemapEligible, got, tt.want)
			}
		})
	}
}

func TestSkyboxConstants(t *testing.T) {
	if len(SkyboxFaceSuffixes) != 6 {
		t.Fatalf("len(SkyboxFaceSuffixes) = %d, want 6", len(SkyboxFaceSuffixes))
	}
	if len(SkyboxCubemapFaceOrder) != 6 {
		t.Fatalf("len(SkyboxCubemapFaceOrder) = %d, want 6", len(SkyboxCubemapFaceOrder))
	}
}
