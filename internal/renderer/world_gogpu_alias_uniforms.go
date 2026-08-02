package renderer

import (
	"encoding/binary"
	"math"

	"github.com/darkliquid/ironwail-go/internal/model"
	aliasimpl "github.com/darkliquid/ironwail-go/internal/renderer/alias"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

func appendAliasSceneUniformBytes(dst []byte, targetOffset uint32, vp types.Mat4, cameraOrigin [3]float32, alpha float32, fogColor [3]float32, fogDensity float32) []byte {
	requiredLen := int(targetOffset) + int(worldUniformAlign)
	if cap(dst) < requiredLen {
		newCap := requiredLen * 2
		buf := make([]byte, requiredLen, newCap)
		copy(buf, dst)
		dst = buf
	} else if len(dst) < requiredLen {
		dst = dst[:requiredLen]
	}
	data := dst[targetOffset : targetOffset+aliasSceneUniformBufferSize]
	matrixBytes := matrixToBytes(vp)
	copy(data[:64], matrixBytes)
	putFloat32s(data[64:76], cameraOrigin[:])
	binary.LittleEndian.PutUint32(data[76:80], math.Float32bits(worldFogUniformDensity(fogDensity)))
	putFloat32s(data[80:92], fogColor[:])
	binary.LittleEndian.PutUint32(data[92:96], math.Float32bits(alpha))
	return dst
}

func appendAliasInstanceUniformBytes(dst []byte, targetOffset uint32, pose1, pose2 int, blend, entityScale float32, scale, scaleOrigin, origin, angles [3]float32, fullAngles bool, numPoses int) []byte {
	requiredLen := int(targetOffset) + int(worldUniformAlign)
	if cap(dst) < requiredLen {
		newCap := requiredLen * 2
		buf := make([]byte, requiredLen, newCap)
		copy(buf, dst)
		dst = buf
	} else if len(dst) < requiredLen {
		dst = dst[:requiredLen]
	}
	data := dst[targetOffset : targetOffset+aliasInstanceUniformSize]
	binary.LittleEndian.PutUint32(data[0:4], math.Float32bits(float32(pose1)))
	binary.LittleEndian.PutUint32(data[4:8], math.Float32bits(float32(pose2)))
	binary.LittleEndian.PutUint32(data[8:12], math.Float32bits(blend))
	binary.LittleEndian.PutUint32(data[12:16], math.Float32bits(entityScale))
	putFloat32s(data[16:28], scale[:])
	putFloat32s(data[28:40], scaleOrigin[:])
	putFloat32s(data[40:52], origin[:])
	putFloat32s(data[52:64], angles[:])
	full := float32(0)
	if fullAngles {
		full = 1
	}
	binary.LittleEndian.PutUint32(data[64:68], math.Float32bits(full))
	binary.LittleEndian.PutUint32(data[68:72], math.Float32bits(float32(numPoses)))
	return dst
}

func aliasSceneUniformBytes(vp types.Mat4, cameraOrigin [3]float32, alpha float32, fogColor [3]float32, fogDensity float32) []byte {
	return appendAliasSceneUniformBytes(nil, 0, vp, cameraOrigin, alpha, fogColor, fogDensity)
}

func aliasVertexBytes(vertices []WorldVertex) []byte {
	return aliasVertexBytesInto(nil, vertices)
}

func aliasVertexBytesInto(dst []byte, vertices []WorldVertex) []byte {
	return appendAliasVertexBytes(dst[:0], vertices)
}

func appendAliasVertexBytes(dst []byte, vertices []WorldVertex) []byte {
	required := len(vertices) * aliasVertexStride
	start := len(dst)
	total := start + required
	if cap(dst) < total {
		newCap := total * 2
		buf := make([]byte, total, newCap)
		copy(buf, dst)
		dst = buf
	} else {
		dst = dst[:total]
	}
	data := dst[start:]
	for i, v := range vertices {
		offset := i * aliasVertexStride
		putFloat32s(data[offset:offset+12], v.Position[:])
		putFloat32s(data[offset+12:offset+20], v.TexCoord[:])
		putFloat32s(data[offset+20:offset+28], v.LightmapCoord[:])
		putFloat32s(data[offset+28:offset+40], v.Normal[:])
		putFloat32s(data[offset+40:offset+44], []float32{v.LightmapLayer})
		binary.LittleEndian.PutUint32(data[offset+44:offset+48], v.MaterialID)
	}
	return dst
}

func buildAliasVerticesInterpolatedInto(dst []WorldVertex, alias *gpuAliasModel, mdl *model.Model, pose1Index, pose2Index int, blend float32, origin, angles [3]float32, entityScale float32, fullAngles bool) []WorldVertex {
	if alias == nil || mdl == nil || mdl.AliasHeader == nil {
		return nil
	}
	return aliasimpl.BuildVerticesInterpolatedInto(
		dst,
		aliasimpl.MeshFromRefs(alias.poses, alias.refs),
		mdl.AliasHeader,
		pose1Index,
		pose2Index,
		blend,
		origin,
		angles,
		entityScale,
		fullAngles,
	)
}
