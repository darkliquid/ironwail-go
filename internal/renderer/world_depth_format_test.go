package renderer

import (
	"testing"

	"github.com/gogpu/gputypes"
)

// TestWorldDepthFormatForFeatures pins the depth-format fallback used when a
// device does not expose the depth32float-stencil8 feature (strict-validating
// browsers). Desktop adapters keep Depth32FloatStencil8 for NVIDIA parity;
// feature-less devices fall back to the universally-available
// Depth24PlusStencil8.
func TestWorldDepthFormatForFeatures(t *testing.T) {
	cases := []struct {
		name     string
		features gputypes.Features
		want     gputypes.TextureFormat
	}{
		{
			name:     "depth32float-stencil8 present",
			features: gputypes.Features(gputypes.FeatureDepth32FloatStencil8),
			want:     gputypes.TextureFormatDepth32FloatStencil8,
		},
		{
			name:     "feature absent falls back to depth24plus-stencil8",
			features: 0,
			want:     gputypes.TextureFormatDepth24PlusStencil8,
		},
		{
			name:     "unrelated features still fall back",
			features: gputypes.Features(gputypes.FeatureTextureCompressionBC),
			want:     gputypes.TextureFormatDepth24PlusStencil8,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := worldDepthFormatForFeatures(tc.features); got != tc.want {
				t.Fatalf("worldDepthFormatForFeatures(%v) = %v, want %v", tc.features, got, tc.want)
			}
		})
	}
}
