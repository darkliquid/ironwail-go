package renderer

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/renderer/pipeline"
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

func TestUpdateWorldDepthFormat(t *testing.T) {
	origRenderer := worldDepthTextureFormat
	origPipeline := pipeline.WorldDepthTextureFormat
	defer func() {
		worldDepthTextureFormat = origRenderer
		pipeline.SetWorldDepthTextureFormat(origPipeline)
	}()

	r := &Renderer{}

	// Feature absent -> Depth24PlusStencil8
	r.updateWorldDepthFormat(0)
	if worldDepthTextureFormat != gputypes.TextureFormatDepth24PlusStencil8 {
		t.Fatalf("worldDepthTextureFormat = %v, want Depth24PlusStencil8", worldDepthTextureFormat)
	}
	if pipeline.WorldDepthTextureFormat != gputypes.TextureFormatDepth24PlusStencil8 {
		t.Fatalf("pipeline.WorldDepthTextureFormat = %v, want Depth24PlusStencil8", pipeline.WorldDepthTextureFormat)
	}

	// Feature present -> Depth32FloatStencil8
	r.updateWorldDepthFormat(gputypes.Features(gputypes.FeatureDepth32FloatStencil8))
	if worldDepthTextureFormat != gputypes.TextureFormatDepth32FloatStencil8 {
		t.Fatalf("worldDepthTextureFormat = %v, want Depth32FloatStencil8", worldDepthTextureFormat)
	}
	if pipeline.WorldDepthTextureFormat != gputypes.TextureFormatDepth32FloatStencil8 {
		t.Fatalf("pipeline.WorldDepthTextureFormat = %v, want Depth32FloatStencil8", pipeline.WorldDepthTextureFormat)
	}
}
