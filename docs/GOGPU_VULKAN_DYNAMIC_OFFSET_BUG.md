# GitHub Issue: [hal/vulkan] Dynamic uniform buffer offsets ignored because CreateBindGroupLayout creates static VK_DESCRIPTOR_TYPE_UNIFORM_BUFFER

**Repository:** `github.com/gogpu/wgpu`  
**Affects:** `v0.32.1` (and earlier pure-Go Vulkan HAL versions)

---

## Title

`[hal/vulkan] Dynamic uniform buffer offsets ignored because CreateBindGroupLayout creates static VK_DESCRIPTOR_TYPE_UNIFORM_BUFFER`

---

## Description

In the pure-Go Vulkan backend (`hal/vulkan`), buffer bindings configured with `HasDynamicOffset: true` are created with static descriptor types (`VK_DESCRIPTOR_TYPE_UNIFORM_BUFFER` / `VK_DESCRIPTOR_TYPE_STORAGE_BUFFER`) instead of their dynamic counterparts (`VK_DESCRIPTOR_TYPE_UNIFORM_BUFFER_DYNAMIC` / `VK_DESCRIPTOR_TYPE_STORAGE_BUFFER_DYNAMIC`).

When draw or compute passes bind a bind group with dynamic offsets using `SetBindGroup(index, bg, []uint32{dynamicOffset})`, `RenderPassEncoder.SetBindGroup` passes `pDynamicOffsets` to `vkCmdBindDescriptorSets`. However, per the Vulkan specification, dynamic offsets are only applied to descriptor bindings that were declared with `_DYNAMIC` descriptor types. Because the descriptor set layout was created as static `VK_DESCRIPTOR_TYPE_UNIFORM_BUFFER`, the Vulkan driver ignores the dynamic offsets, and shaders unconditionally read from offset 0 on the GPU.

Other backends (Metal in `hal/metal/encoder.go:756` and GLES in `hal/gles/command.go:1378`) handle `HasDynamicOffset` properly.

---

## Root Cause Analysis

1. **`hal/vulkan/device.go:952-958` (`CreateBindGroupLayout`):**
   ```go
   case entry.Buffer != nil:
       binding.DescriptorType = bufferBindingTypeToVk(entry.Buffer.Type)
   ```
   `entry.Buffer.HasDynamicOffset` is never checked when configuring the descriptor set layout binding.

2. **`hal/vulkan/convert.go:287-298` (`bufferBindingTypeToVk`):**
   ```go
   func bufferBindingTypeToVk(bindingType gputypes.BufferBindingType) vk.DescriptorType {
       switch bindingType {
       case gputypes.BufferBindingTypeUniform:
           return vk.DescriptorTypeUniformBuffer // Never returns vk.DescriptorTypeUniformBufferDynamic
       case gputypes.BufferBindingTypeStorage, gputypes.BufferBindingTypeReadOnlyStorage:
           return vk.DescriptorTypeStorageBuffer // Never returns vk.DescriptorTypeStorageBufferDynamic
       default:
           return vk.DescriptorTypeUniformBuffer
       }
   }
   ```

3. **`hal/vulkan/descriptor.go:224` & `hal/vulkan/device.go:1091` (`CreateBindGroup`):**
   The descriptor pool allocation and descriptor writes are also hardcoded to `vk.DescriptorTypeUniformBuffer` and `vk.DescriptorTypeStorageBuffer`.

---

## Standalone Reproduction Code

Save as `main.go` and run with `go run main.go`:

```go
package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	_ "github.com/gogpu/wgpu/hal/vulkan"
)

const computeShaderWGSL = `
struct Uniforms {
	val: f32,
}

@group(0) @binding(0) var<uniform> u: Uniforms;
@group(0) @binding(1) var<storage, read_write> outBuf: array<f32>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
	outBuf[0] = u.val;
}
`

func main() {
	instance, err := wgpu.CreateInstance(&wgpu.InstanceDescriptor{
		Backends: wgpu.BackendsAll,
	})
	if err != nil || instance == nil {
		fmt.Printf("Failed to create wgpu instance: %v\n", err)
		os.Exit(1)
	}
	defer instance.Release()

	adapter, err := instance.RequestAdapter(nil)
	if err != nil {
		fmt.Printf("RequestAdapter failed: %v\n", err)
		os.Exit(1)
	}
	defer adapter.Release()

	fmt.Printf("Adapter: %s (Backend: %v)\n", adapter.Info().Name, adapter.Info().Backend)

	device, err := adapter.RequestDevice(nil)
	if err != nil {
		fmt.Printf("RequestDevice failed: %v\n", err)
		os.Exit(1)
	}
	defer device.Release()
	queue := device.Queue()

	// 1. Create a uniform buffer with 2 slots of 256 bytes
	// Slot 0 (offset 0): val = 100.0
	// Slot 1 (offset 256): val = 42.0
	uniformBuf, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "Uniform Buffer",
		Size:  512,
		Usage: gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		fmt.Printf("Create uniform buffer failed: %v\n", err)
		os.Exit(1)
	}
	defer uniformBuf.Release()

	var data0 [256]byte
	binary.LittleEndian.PutUint32(data0[0:4], math.Float32bits(100.0))
	_ = queue.WriteBuffer(uniformBuf, 0, data0[:])

	var data1 [256]byte
	binary.LittleEndian.PutUint32(data1[0:4], math.Float32bits(42.0))
	_ = queue.WriteBuffer(uniformBuf, 256, data1[:])

	// 2. Create storage output buffer and readback buffer
	outBuf, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "Output Buffer",
		Size:  16,
		Usage: gputypes.BufferUsageStorage | gputypes.BufferUsageCopySrc,
	})
	if err != nil {
		fmt.Printf("Create output buffer failed: %v\n", err)
		os.Exit(1)
	}
	defer outBuf.Release()

	readbackBuf, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "Readback Buffer",
		Size:  16,
		Usage: gputypes.BufferUsageMapRead | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		fmt.Printf("Create readback buffer failed: %v\n", err)
		os.Exit(1)
	}
	defer readbackBuf.Release()

	// 3. Create BindGroupLayout with HasDynamicOffset: true for binding 0
	bgl, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "Repro BGL",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageCompute,
				Buffer: &gputypes.BufferBindingLayout{
					Type:             gputypes.BufferBindingTypeUniform,
					HasDynamicOffset: true,
					MinBindingSize:   16,
				},
			},
			{
				Binding:    1,
				Visibility: gputypes.ShaderStageCompute,
				Buffer: &gputypes.BufferBindingLayout{
					Type:             gputypes.BufferBindingTypeStorage,
					HasDynamicOffset: false,
					MinBindingSize:   16,
				},
			},
		},
	})
	if err != nil {
		fmt.Printf("CreateBindGroupLayout failed: %v\n", err)
		os.Exit(1)
	}
	defer bgl.Release()

	// 4. Create BindGroup with buffer binding size 256
	bg, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "Repro BG",
		Layout: bgl,
		Entries: []wgpu.BindGroupEntry{
			{
				Binding: 0,
				Buffer:  uniformBuf,
				Offset:  0,
				Size:    256,
			},
			{
				Binding: 1,
				Buffer:  outBuf,
				Offset:  0,
				Size:    16,
			},
		},
	})
	if err != nil {
		fmt.Printf("CreateBindGroup failed: %v\n", err)
		os.Exit(1)
	}
	defer bg.Release()

	// 5. Create Compute Pipeline
	shaderModule, err := device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label: "Repro Shader",
		WGSL:  computeShaderWGSL,
	})
	if err != nil {
		fmt.Printf("CreateShaderModule failed: %v\n", err)
		os.Exit(1)
	}
	defer shaderModule.Release()

	pl, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "Repro PL",
		BindGroupLayouts: []*wgpu.BindGroupLayout{bgl},
	})
	if err != nil {
		fmt.Printf("CreatePipelineLayout failed: %v\n", err)
		os.Exit(1)
	}
	defer pl.Release()

	computePipeline, err := device.CreateComputePipeline(&wgpu.ComputePipelineDescriptor{
		Label:      "Repro Compute Pipeline",
		Layout:     pl,
		Module:     shaderModule,
		EntryPoint: "main",
	})
	if err != nil {
		fmt.Printf("CreateComputePipeline failed: %v\n", err)
		os.Exit(1)
	}
	defer computePipeline.Release()

	// 6. Record Compute Pass with dynamic offset 256
	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "Repro Encoder"})
	if err != nil {
		fmt.Printf("CreateCommandEncoder failed: %v\n", err)
		os.Exit(1)
	}

	computePass, err := encoder.BeginComputePass(&wgpu.ComputePassDescriptor{Label: "Repro Pass"})
	if err != nil {
		fmt.Printf("BeginComputePass failed: %v\n", err)
		os.Exit(1)
	}
	computePass.SetPipeline(computePipeline)
	// Bind with dynamic offset 256 (slot 1, value 42.0)
	computePass.SetBindGroup(0, bg, []uint32{256})
	computePass.Dispatch(1, 1, 1)
	_ = computePass.End()

	encoder.CopyBufferToBuffer(outBuf, 0, readbackBuf, 0, 16)
	cmdBuf, err := encoder.Finish()
	if err != nil {
		fmt.Printf("Finish failed: %v\n", err)
		os.Exit(1)
	}
	defer cmdBuf.Release()

	_, _ = queue.Submit(cmdBuf)

	// 7. Read back result
	pending, err := readbackBuf.MapAsync(wgpu.MapModeRead, 0, 16)
	if err != nil {
		fmt.Printf("MapAsync error: %v\n", err)
		os.Exit(1)
	}

	_ = device.Poll(wgpu.PollWait)
	if err := pending.Wait(context.Background()); err != nil {
		fmt.Printf("pending.Wait failed: %v\n", err)
		os.Exit(1)
	}

	mapped, err := readbackBuf.MappedRange(0, 16)
	if err != nil {
		fmt.Printf("MappedRange failed: %v\n", err)
		os.Exit(1)
	}
	bytes := mapped.Bytes()
	gotVal := math.Float32frombits(binary.LittleEndian.Uint32(bytes[0:4]))
	_ = readbackBuf.Unmap()

	fmt.Println("--------------------------------------------------")
	fmt.Printf("Dynamic offset requested: 256 (expected value: 42.0)\n")
	fmt.Printf("Slot 0 offset 0 value:    100.0\n")
	fmt.Printf("GPU evaluated value:      %f\n", gotVal)
	fmt.Println("--------------------------------------------------")

	switch gotVal {
	case 42.0:
		fmt.Println("SUCCESS: Dynamic uniform offset was applied correctly.")
	case 100.0:
		fmt.Println("FAILURE / BUG CONFIRMED: Dynamic uniform offset was IGNORED (GPU read from offset 0).")
	default:
		fmt.Printf("UNEXPECTED: Got value %f\n", gotVal)
	}
}
```

---

## Output / Behavior

### Expected Behavior:
The compute shader reads the uniform data from dynamic offset 256 (`val = 42.0`):
```text
Adapter: NVIDIA GeForce RTX 3060 (Backend: Vulkan)
--------------------------------------------------
Dynamic offset requested: 256 (expected value: 42.0)
Slot 0 offset 0 value:    100.0
GPU evaluated value:      42.000000
--------------------------------------------------
SUCCESS: Dynamic uniform offset was applied correctly.
```

### Actual Behavior:
The compute shader reads the uniform data from offset 0 (`val = 100.0`):
```text
Adapter: NVIDIA GeForce RTX 3060 (Backend: Vulkan)
--------------------------------------------------
Dynamic offset requested: 256 (expected value: 42.0)
Slot 0 offset 0 value:    100.0
GPU evaluated value:      100.000000
--------------------------------------------------
FAILURE / BUG CONFIRMED: Dynamic uniform offset was IGNORED (GPU read from offset 0).
```

---

## Suggested Fix

1. Update `bufferBindingTypeToVk(bindingType gputypes.BufferBindingType, hasDynamicOffset bool) vk.DescriptorType` in `hal/vulkan/convert.go` to return:
   - `vk.DescriptorTypeUniformBufferDynamic` when `hasDynamicOffset == true` and type is `BufferBindingTypeUniform`.
   - `vk.DescriptorTypeStorageBufferDynamic` when `hasDynamicOffset == true` and type is storage / read-only storage.
2. In `hal/vulkan/device.go` (`CreateBindGroupLayout`), pass `entry.Buffer.HasDynamicOffset` to `bufferBindingTypeToVk`.
3. In `DescriptorCounts` and `hal/vulkan/descriptor.go`, track counts for dynamic descriptor types in `vk.DescriptorPoolSize`.
4. In `hal/vulkan/device.go` (`CreateBindGroup`), use the matching descriptor type when writing descriptor sets.
