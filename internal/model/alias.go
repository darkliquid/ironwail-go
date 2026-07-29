package model

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/darkliquid/ironwail-go/internal/engine/arena"
)

const (
	defaultAliasFrameInterval = 0.1
)

func LoadAliasModel(r io.ReadSeeker) (*Model, error) {
	return LoadAliasModelWithArena(r, nil)
}

func LoadAliasModelWithArena(r io.ReadSeeker, ar *arena.Arena) (*Model, error) {
	var header MDLHeader
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("failed to read alias header: %w", err)
	}

	if header.Ident != MDLIdent {
		return nil, fmt.Errorf("invalid MDL ident: got 0x%08x, expected 0x%08x", header.Ident, MDLIdent)
	}
	if header.Version != MDLVersion {
		return nil, fmt.Errorf("unsupported MDL version: got %d, expected %d", header.Version, MDLVersion)
	}

	numSkins := int(header.NumSkins)
	numVerts := int(header.NumVerts)
	numTris := int(header.NumTris)
	numFrames := int(header.NumFrames)

	if numSkins < 1 || numSkins > MaxSkins {
		return nil, fmt.Errorf("invalid number of skins: %d", numSkins)
	}
	if numVerts <= 0 {
		return nil, fmt.Errorf("alias model has no vertices")
	}
	if numVerts > MaxAliasVerts {
		return nil, fmt.Errorf("alias model has too many vertices: %d (max %d)", numVerts, MaxAliasVerts)
	}
	if numTris <= 0 {
		return nil, fmt.Errorf("alias model has no triangles")
	}
	if numFrames < 1 {
		return nil, fmt.Errorf("invalid number of frames: %d", numFrames)
	}

	skins, skinDescs, err := readAliasSkins(r, ar, numSkins, int(header.SkinWidth), int(header.SkinHeight))
	if err != nil {
		return nil, err
	}

	stVerts, err := readAliasSTVerts(r, ar, numVerts)
	if err != nil {
		return nil, err
	}
	triangles, err := readAliasTriangles(r, ar, numTris)
	if err != nil {
		return nil, err
	}

	frames, poses, numPoses, bounds, err := readAliasFrames(r, ar, numFrames, numVerts, header.Scale, header.ScaleOrigin)
	if err != nil {
		return nil, err
	}

	alias := &AliasHeader{
		Ident:          int(header.Ident),
		Version:        int(header.Version),
		Scale:          header.Scale,
		ScaleOrigin:    header.ScaleOrigin,
		BoundingRadius: header.BoundingRadius,
		EyePosition:    header.EyePosition,
		NumSkins:       numSkins,
		SkinWidth:      int(header.SkinWidth),
		SkinHeight:     int(header.SkinHeight),
		NumVerts:       numVerts,
		NumTris:        numTris,
		NumFrames:      numFrames,
		SyncType:       SyncType(header.SyncType),
		Flags:          int(header.Flags),
		Size:           header.Size,
		NumPoses:       numPoses,
		PoseVertType:   0,
		Skins:          skins,
		SkinDescs:      skinDescs,
		STVerts:        stVerts,
		Triangles:      triangles,
		Poses:          poses,
		Frames:         frames,
	}

	model := &Model{
		Type:        ModAlias,
		NumFrames:   numFrames,
		SyncType:    SyncType(header.SyncType),
		Flags:       int(header.Flags),
		Mins:        bounds.mins,
		Maxs:        bounds.maxs,
		YMins:       bounds.ymins,
		YMaxs:       bounds.ymaxs,
		RMins:       bounds.rmins,
		RMaxs:       bounds.rmaxs,
		AliasHeader: alias,
	}

	return model, nil
}

type aliasBounds struct {
	mins, maxs   [3]float32
	ymins, ymaxs [3]float32
	rmins, rmaxs [3]float32
}

func initAliasBounds() aliasBounds {
	return aliasBounds{
		mins: [3]float32{math.MaxFloat32, math.MaxFloat32, math.MaxFloat32},
		maxs: [3]float32{-math.MaxFloat32, -math.MaxFloat32, -math.MaxFloat32},
	}
}

func (b *aliasBounds) expand(v [3]float32) {
	for i := 0; i < 3; i++ {
		if v[i] < b.mins[i] {
			b.mins[i] = v[i]
		}
		if v[i] > b.maxs[i] {
			b.maxs[i] = v[i]
		}
	}
}

func (b *aliasBounds) finalize(yawRadiusSquared, radiusSquared float32) {
	radius := float32(math.Sqrt(float64(radiusSquared)))
	b.rmins = [3]float32{-radius, -radius, -radius}
	b.rmaxs = [3]float32{radius, radius, radius}

	yawRadius := float32(math.Sqrt(float64(yawRadiusSquared)))
	b.ymins = [3]float32{-yawRadius, -yawRadius, b.mins[2]}
	b.ymaxs = [3]float32{yawRadius, yawRadius, b.maxs[2]}
}

func readAliasSkins(r io.ReadSeeker, ar *arena.Arena, numSkins, skinWidth, skinHeight int) ([][]byte, []AliasSkinDesc, error) {
	skinSize := skinWidth * skinHeight
	if skinSize < 0 {
		return nil, nil, fmt.Errorf("invalid skin dimensions: %dx%d", skinWidth, skinHeight)
	}

	skins := make([][]byte, 0, numSkins)
	skinDescs := arena.Alloc[AliasSkinDesc](ar, numSkins)[:0]
	for i := 0; i < numSkins; i++ {
		var skinType DAliasSkinType
		if err := binary.Read(r, binary.LittleEndian, &skinType); err != nil {
			return nil, nil, fmt.Errorf("failed to read skin type %d: %w", i, err)
		}

		switch AliasSkinType(skinType.Type) {
		case AliasSkinSingle:
			skin, err := readAliasSkinPixels(r, ar, skinSize)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to read single skin %d: %w", i, err)
			}
			firstFrame := len(skins)
			skins = append(skins, skin)
			skinDescs = append(skinDescs, AliasSkinDesc{FirstFrame: firstFrame, NumFrames: 1})
		case AliasSkinGroup:
			var group DAliasSkinGroup
			if err := binary.Read(r, binary.LittleEndian, &group); err != nil {
				return nil, nil, fmt.Errorf("failed to read skin group %d: %w", i, err)
			}

			n := int(group.NumSkins)
			if n < 1 {
				return nil, nil, fmt.Errorf("invalid number of grouped skins for skin %d: %d", i, n)
			}

			intervals := arena.Alloc[DAliasSkinInterval](ar, n)
			if err := binary.Read(r, binary.LittleEndian, &intervals); err != nil {
				return nil, nil, fmt.Errorf("failed to read skin group intervals %d: %w", i, err)
			}

			firstFrame := len(skins)
			skinIntervals := arena.Alloc[float32](ar, n)
			for skinIndex := 0; skinIndex < n; skinIndex++ {
				skin, err := readAliasSkinPixels(r, ar, skinSize)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read grouped skin %d:%d: %w", i, skinIndex, err)
				}
				skins = append(skins, skin)
				skinIntervals[skinIndex] = intervals[skinIndex].Interval
			}
			skinDescs = append(skinDescs, AliasSkinDesc{
				FirstFrame: firstFrame,
				NumFrames:  n,
				Intervals:  skinIntervals,
			})
		default:
			return nil, nil, fmt.Errorf("invalid skin type %d for skin %d", skinType.Type, i)
		}
	}

	return skins, skinDescs, nil
}

func readAliasSkinPixels(r io.Reader, ar *arena.Arena, skinSize int) ([]byte, error) {
	data := arena.Alloc[byte](ar, skinSize)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	return data, nil
}

func readAliasSTVerts(r io.Reader, ar *arena.Arena, count int) ([]STVert, error) {
	verts := arena.Alloc[STVert](ar, count)
	if err := binary.Read(r, binary.LittleEndian, &verts); err != nil {
		return nil, fmt.Errorf("failed to read ST verts: %w", err)
	}
	return verts, nil
}

func readAliasTriangles(r io.Reader, ar *arena.Arena, count int) ([]DTriangle, error) {
	tris := arena.Alloc[DTriangle](ar, count)
	if err := binary.Read(r, binary.LittleEndian, &tris); err != nil {
		return nil, fmt.Errorf("failed to read triangles: %w", err)
	}
	return tris, nil
}

func readAliasFrames(r io.Reader, ar *arena.Arena, numFrames, numVerts int, scale, origin [3]float32) ([]AliasFrameDesc, [][]TriVertX, int, aliasBounds, error) {
	frames := arena.Alloc[AliasFrameDesc](ar, numFrames)[:0]
	poses := make([][]TriVertX, 0, numFrames)
	bounds := initAliasBounds()
	poseCount := 0
	var yawRadiusSquared float32
	var radiusSquared float32

	for i := 0; i < numFrames; i++ {
		var frameType DAliasFrameType
		if err := binary.Read(r, binary.LittleEndian, &frameType); err != nil {
			return nil, nil, 0, aliasBounds{}, fmt.Errorf("failed to read frame type %d: %w", i, err)
		}

		switch AliasFrameType(frameType.Type) {
		case AliasSingle:
			var frame DAliasFrame
			if err := binary.Read(r, binary.LittleEndian, &frame); err != nil {
				return nil, nil, 0, aliasBounds{}, fmt.Errorf("failed to read single frame %d: %w", i, err)
			}

			poseVerts, err := readAliasPoseVerts(r, ar, numVerts)
			if err != nil {
				return nil, nil, 0, aliasBounds{}, fmt.Errorf("failed to read single frame pose verts %d: %w", i, err)
			}
			poses = append(poses, poseVerts)
			updateAliasBounds(poseVerts, scale, origin, &bounds, &yawRadiusSquared, &radiusSquared)

			frames = append(frames, AliasFrameDesc{
				FirstPose: poseCount,
				NumPoses:  1,
				Interval:  defaultAliasFrameInterval,
				BBoxMin: [4]byte{
					frame.BBoxMin.V[0],
					frame.BBoxMin.V[1],
					frame.BBoxMin.V[2],
					frame.BBoxMin.LightNormalIndex,
				},
				BBoxMax: [4]byte{
					frame.BBoxMax.V[0],
					frame.BBoxMax.V[1],
					frame.BBoxMax.V[2],
					frame.BBoxMax.LightNormalIndex,
				},
				Frame: i,
				Name:  frame.Name,
			})

			poseCount++
			if poseCount > MaxAliasFrames {
				return nil, nil, 0, aliasBounds{}, fmt.Errorf("too many alias poses: %d (max %d)", poseCount, MaxAliasFrames)
			}

		case AliasGroup:
			var group DAliasGroup
			if err := binary.Read(r, binary.LittleEndian, &group); err != nil {
				return nil, nil, 0, aliasBounds{}, fmt.Errorf("failed to read frame group %d: %w", i, err)
			}

			n := int(group.NumFrames)
			if n < 1 {
				return nil, nil, 0, aliasBounds{}, fmt.Errorf("invalid number of grouped frames for frame %d: %d", i, n)
			}

			intervals := arena.Alloc[DAliasInterval](ar, n)
			if err := binary.Read(r, binary.LittleEndian, &intervals); err != nil {
				return nil, nil, 0, aliasBounds{}, fmt.Errorf("failed to read frame intervals for frame %d: %w", i, err)
			}

			firstPose := poseCount
			for poseIndex := 0; poseIndex < n; poseIndex++ {
				var pose DAliasFrame
				if err := binary.Read(r, binary.LittleEndian, &pose); err != nil {
					return nil, nil, 0, aliasBounds{}, fmt.Errorf("failed to read group pose %d for frame %d: %w", poseIndex, i, err)
				}

				poseVerts, err := readAliasPoseVerts(r, ar, numVerts)
				if err != nil {
					return nil, nil, 0, aliasBounds{}, fmt.Errorf("failed to read group pose verts %d for frame %d: %w", poseIndex, i, err)
				}
				poses = append(poses, poseVerts)
				updateAliasBounds(poseVerts, scale, origin, &bounds, &yawRadiusSquared, &radiusSquared)

				poseCount++
				if poseCount > MaxAliasFrames {
					return nil, nil, 0, aliasBounds{}, fmt.Errorf("too many alias poses: %d (max %d)", poseCount, MaxAliasFrames)
				}
			}

			frames = append(frames, AliasFrameDesc{
				FirstPose: firstPose,
				NumPoses:  n,
				Interval:  intervals[0].Interval,
				BBoxMin: [4]byte{
					group.BBoxMin.V[0],
					group.BBoxMin.V[1],
					group.BBoxMin.V[2],
					group.BBoxMin.LightNormalIndex,
				},
				BBoxMax: [4]byte{
					group.BBoxMax.V[0],
					group.BBoxMax.V[1],
					group.BBoxMax.V[2],
					group.BBoxMax.LightNormalIndex,
				},
				Frame: i,
			})
		default:
			return nil, nil, 0, aliasBounds{}, fmt.Errorf("invalid frame type %d for frame %d", frameType.Type, i)
		}
	}

	if poseCount == 0 {
		return nil, nil, 0, aliasBounds{}, fmt.Errorf("alias model has no poses")
	}

	bounds.finalize(yawRadiusSquared, radiusSquared)
	return frames, poses, poseCount, bounds, nil
}

func readAliasPoseVerts(r io.Reader, ar *arena.Arena, count int) ([]TriVertX, error) {
	verts := arena.Alloc[TriVertX](ar, count)
	if err := binary.Read(r, binary.LittleEndian, &verts); err != nil {
		return nil, err
	}
	return verts, nil
}

func updateAliasBounds(verts []TriVertX, scale, origin [3]float32, bounds *aliasBounds, yawRadiusSquared, radiusSquared *float32) {
	for _, v := range verts {
		decoded := DecodeVertex(v, scale, origin)
		bounds.expand(decoded)

		dist := decoded[0]*decoded[0] + decoded[1]*decoded[1]
		if dist > *yawRadiusSquared {
			*yawRadiusSquared = dist
		}

		dist += decoded[2] * decoded[2]
		if dist > *radiusSquared {
			*radiusSquared = dist
		}
	}
}
