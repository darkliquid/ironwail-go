package renderer

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/darkliquid/ironwail-go/pkg/types"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

func (r *Renderer) createWorldClusterComputePipeline(device *wgpu.Device, computeShader *wgpu.ShaderModule) (*wgpu.ComputePipeline, *wgpu.PipelineLayout, *wgpu.BindGroupLayout, error) {
	if device == nil || computeShader == nil {
		return nil, nil, nil, fmt.Errorf("invalid shader module or device")
	}

	bindGroupLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "World Cluster Compute BGL",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0, // ComputeUniforms
				Visibility: gputypes.ShaderStageCompute,
				Buffer: &gputypes.BufferBindingLayout{
					Type:             gputypes.BufferBindingTypeUniform,
					HasDynamicOffset: false,
					MinBindingSize:   0,
				},
			},
			{
				Binding:    1, // DynamicLight array
				Visibility: gputypes.ShaderStageCompute,
				Buffer: &gputypes.BufferBindingLayout{
					Type:             gputypes.BufferBindingTypeReadOnlyStorage,
					HasDynamicOffset: false,
					MinBindingSize:   0,
				},
			},
			{
				Binding:    2, // LightClusters storage texture
				Visibility: gputypes.ShaderStageCompute,
				StorageTexture: &gputypes.StorageTextureBindingLayout{
					Access:        gputypes.StorageTextureAccessWriteOnly,
					Format:        gputypes.TextureFormatRG32Uint,
					ViewDimension: gputypes.TextureViewDimension3D,
				},
			},
		},
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create compute BGL: %w", err)
	}

	pipelineLayout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "World Cluster Compute Pipeline Layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{bindGroupLayout},
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create compute pipeline layout: %w", err)
	}

	pipeline, err := device.CreateComputePipeline(&wgpu.ComputePipelineDescriptor{
		Label:      "World Cluster Compute Pipeline",
		Layout:     pipelineLayout,
		Module:     computeShader,
		EntryPoint: "cs_main",
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create compute pipeline: %w", err)
	}

	return pipeline, pipelineLayout, bindGroupLayout, nil
}

func (r *Renderer) dispatchWorldClusterCompute(device *wgpu.Device, queue *wgpu.Queue, encoder *wgpu.CommandEncoder, activeLights []DynamicLight, viewMatrix, projMatrix types.Mat4) error {
	if r.resources.WorldClusterComputePipeline == nil || r.resources.WorldClusterComputeBindGroup == nil {
		return nil
	}

	// 1. Upload dynamic lights (if not already done elsewhere, but we can do it here to ensure it's done before compute)
	ptr, lightData := encodeGoGPUWorldDynamicLights(activeLights)
	err := queue.WriteBuffer(r.resources.WorldDynamicLightsBuffer, 0, lightData)
	dynamicLightsBytesPool.Put(ptr)
	if err != nil {
		return fmt.Errorf("upload dynamic lights: %w", err)
	}

	// Calculate numLights exactly as the shader expects (active ones that fade > 0)
	numLights := uint32(0)
	if dynamicLightsEnabled() {
		for _, light := range activeLights {
			if numLights >= gogpuWorldDynamicLightBufferMax || light.Radius <= 0 {
				break
			}
			if light.Brightness*light.FadeMultiplier() > 0 {
				numLights++
			}
		}
	}

	// 2. Upload compute uniforms
	uniformBytes := make([]byte, 144)
	putFloat32s(uniformBytes[0:64], viewMatrix[:])

	// The C version uses TransposedProj, which is just the projection matrix transposed.
	transposedProj := projMatrix.Transpose()
	putFloat32s(uniformBytes[64:128], transposedProj[:])

	// zLogScale and zLogBias
	// In C, logznear and logzfar are derived from camera.
	// For Quake, usually znear=4.0, zfar=4096.0 or similar.
	// We can use fixed constants or derive from projection.
	// Ironwail-go sets camera Near=4.0.
	logznear := float32(2.0) // log2(4.0)
	logzfar := float32(14.0) // log2(16384.0) - let's assume 16384.
	zLogScale := float32(32.0) / (logzfar - logznear)
	zLogBias := -zLogScale * logznear

	binary.LittleEndian.PutUint32(uniformBytes[128:132], math.Float32bits(zLogScale))
	binary.LittleEndian.PutUint32(uniformBytes[132:136], math.Float32bits(zLogBias))
	binary.LittleEndian.PutUint32(uniformBytes[136:140], numLights)
	// padding 140:144

	if err := queue.WriteBuffer(r.resources.WorldClusterComputeUniformBuffer, 0, uniformBytes); err != nil {
		return fmt.Errorf("upload compute uniforms: %w", err)
	}

	// 3. Dispatch Compute Pass
	computePass, err := encoder.BeginComputePass(&wgpu.ComputePassDescriptor{
		Label: "World Cluster Compute Pass",
	})
	if err != nil {
		return fmt.Errorf("begin compute pass failed: %w", err)
	}

	computePass.SetPipeline(r.resources.WorldClusterComputePipeline)
	computePass.SetBindGroup(0, r.resources.WorldClusterComputeBindGroup, nil)
	// Dispatch for 32x16x32 Grid with 8x8x1 threads per group
	// (32+7)/8 = 4, (16+7)/8 = 2, 32/1 = 32
	computePass.Dispatch(4, 2, 32)
	if err := computePass.End(); err != nil {
		return fmt.Errorf("end compute pass failed: %w", err)
	}

	return nil
}
