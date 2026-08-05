package pipeline_test

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/renderer/pipeline"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

type mockResourceProvider struct{}

func (m *mockResourceProvider) Device() *wgpu.Device { return nil }
func (m *mockResourceProvider) DepthFormat() gputypes.TextureFormat {
	return pipeline.WorldDepthTextureFormat
}
func (m *mockResourceProvider) UniformBuffer() *wgpu.Buffer { return nil }

func TestPipelineResources(t *testing.T) {
	res := pipeline.NewResources()
	if res == nil {
		t.Fatalf("NewResources returned nil")
	}

	mock := &mockResourceProvider{}
	resProv, err := pipeline.NewResourcesWithProvider(mock, nil)
	if err != nil {
		t.Fatalf("NewResourcesWithProvider returned error: %v", err)
	}
	if resProv.Provider() != mock {
		t.Fatalf("Provider mismatch")
	}
	if resProv.Provider().DepthFormat() != pipeline.WorldDepthTextureFormat {
		t.Fatalf("DepthFormat mismatch")
	}
}
