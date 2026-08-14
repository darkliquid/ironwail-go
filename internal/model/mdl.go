package model

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/darkliquid/ironwail-go/pkg/types"
)

const (
	// MDLIdent is the magic number for MDL files ("IDPO" in little-endian).
	MDLIdent = 0x4F504449
	// MDLVersion is the expected version number for MDL files.
	MDLVersion = 6
	// MDLOnSeam is a flag for vertices on the seam.
	MDLOnSeam = 0x0020
	// MDLFacesFront is a flag for triangles facing front.
	MDLFacesFront = 0x0010
)

// AliasFrameType distinguishes between single frames and frame groups.
type AliasFrameType int

const (
	AliasSingle AliasFrameType = 0
	AliasGroup  AliasFrameType = 1
)

// AliasSkinType distinguishes between single skins and skin groups.
type AliasSkinType int

const (
	AliasSkinSingle AliasSkinType = 0
	AliasSkinGroup  AliasSkinType = 1
)

// MDLHeader represents the on-disk MDL file header.
type MDLHeader struct {
	Ident          int32
	Version        int32
	Scale          types.Vec3
	ScaleOrigin    types.Vec3
	BoundingRadius float32
	EyePosition    types.Vec3
	NumSkins       int32
	SkinWidth      int32
	SkinHeight     int32
	NumVerts       int32
	NumTris        int32
	NumFrames      int32
	SyncType       int32
	Flags          int32
	Size           float32
}

// STVert represents an on-disk skin texture vertex.
type STVert struct {
	OnSeam int32
	S      int32
	T      int32
}

// DTriangle represents an on-disk triangle definition.
type DTriangle struct {
	FacesFront int32
	VertIndex  [3]int32
}

// TriVertX represents a compressed vertex position and normal.
type TriVertX struct {
	V                [3]byte
	LightNormalIndex byte
}

// DAliasFrame represents an on-disk single frame.
type DAliasFrame struct {
	BBoxMin TriVertX
	BBoxMax TriVertX
	Name    [16]byte
}

// DAliasGroup represents an on-disk frame group header.
type DAliasGroup struct {
	NumFrames int32
	BBoxMin   TriVertX
	BBoxMax   TriVertX
}

// DAliasInterval represents an on-disk frame interval.
type DAliasInterval struct {
	Interval float32
}

// DAliasSkinGroup represents an on-disk skin group header.
type DAliasSkinGroup struct {
	NumSkins int32
}

// DAliasSkinInterval represents an on-disk skin interval.
type DAliasSkinInterval struct {
	Interval float32
}

// DAliasFrameType represents an on-disk frame type marker.
type DAliasFrameType struct {
	Type int32
}

// DAliasSkinType represents an on-disk skin type marker.
type DAliasSkinType struct {
	Type int32
}

// MDLReader reads MDL (alias model) files.
type MDLReader struct {
	r io.ReadSeeker
}

// NewMDLReader creates a new MDL reader.
func NewMDLReader(r io.ReadSeeker) *MDLReader {
	return &MDLReader{r: r}
}

// ReadHeader reads and validates the MDL header.
func (r *MDLReader) ReadHeader() (*MDLHeader, error) {
	var header MDLHeader
	if err := binary.Read(r.r, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("failed to read MDL header: %w", err)
	}

	if header.Ident != MDLIdent {
		return nil, fmt.Errorf("invalid MDL ident: got 0x%08x, expected 0x%08x", header.Ident, MDLIdent)
	}

	if header.Version != MDLVersion {
		return nil, fmt.Errorf("unsupported MDL version: got %d, expected %d", header.Version, MDLVersion)
	}

	return &header, nil
}

// ReadSTVerts reads the skin texture vertices.
func (r *MDLReader) ReadSTVerts(count int) ([]STVert, error) {
	verts := make([]STVert, count)
	if err := binary.Read(r.r, binary.LittleEndian, &verts); err != nil {
		return nil, fmt.Errorf("failed to read ST verts: %w", err)
	}
	return verts, nil
}

// ReadTriangles reads the triangle definitions.
func (r *MDLReader) ReadTriangles(count int) ([]DTriangle, error) {
	tris := make([]DTriangle, count)
	if err := binary.Read(r.r, binary.LittleEndian, &tris); err != nil {
		return nil, fmt.Errorf("failed to read triangles: %w", err)
	}
	return tris, nil
}

// ReadSkin reads a single skin texture.
func (r *MDLReader) ReadSkin(width, height int) ([]byte, error) {
	size := width * height
	data := make([]byte, size)
	if _, err := io.ReadFull(r.r, data); err != nil {
		return nil, fmt.Errorf("failed to read skin: %w", err)
	}
	return data, nil
}

// LoadMDL loads a complete MDL file and returns an AliasHeader.
func LoadMDL(r io.ReadSeeker) (*AliasHeader, error) {
	reader := NewMDLReader(r)

	header, err := reader.ReadHeader()
	if err != nil {
		return nil, err
	}

	// Read ST verts
	stVerts, err := reader.ReadSTVerts(int(header.NumVerts))
	if err != nil {
		return nil, fmt.Errorf("failed to load ST verts: %w", err)
	}

	// Read triangles
	tris, err := reader.ReadTriangles(int(header.NumTris))
	if err != nil {
		return nil, fmt.Errorf("failed to load triangles: %w", err)
	}

	// Skip skins for now (we just need the structure)
	skinSize := int(header.SkinWidth) * int(header.SkinHeight)
	for i := 0; i < int(header.NumSkins); i++ {
		var skinType DAliasSkinType
		if err := binary.Read(r, binary.LittleEndian, &skinType); err != nil {
			return nil, fmt.Errorf("failed to read skin type: %w", err)
		}

		if skinType.Type == int32(AliasSkinSingle) {
			if _, err := r.Seek(int64(skinSize), io.SeekCurrent); err != nil {
				return nil, fmt.Errorf("failed to skip skin: %w", err)
			}
		} else {
			var group DAliasSkinGroup
			if err := binary.Read(r, binary.LittleEndian, &group); err != nil {
				return nil, fmt.Errorf("failed to read skin group: %w", err)
			}
			// Skip intervals and skins
			skipSize := int64(group.NumSkins)*(4+int64(skinSize)) + int64(group.NumSkins)*4
			if _, err := r.Seek(skipSize, io.SeekCurrent); err != nil {
				return nil, fmt.Errorf("failed to skip skin group: %w", err)
			}
		}
	}

	// Read frames
	frames := make([]AliasFrameDesc, 0, header.NumFrames)
	poseCount := 0

	for len(frames) < int(header.NumFrames) {
		var frameType DAliasFrameType
		if err := binary.Read(r, binary.LittleEndian, &frameType); err != nil {
			return nil, fmt.Errorf("failed to read frame type: %w", err)
		}

		if frameType.Type == int32(AliasSingle) {
			var frame DAliasFrame
			if err := binary.Read(r, binary.LittleEndian, &frame); err != nil {
				return nil, fmt.Errorf("failed to read frame: %w", err)
			}

			// Skip pose vertices
			poseSize := int64(header.NumVerts) * 4
			if _, err := r.Seek(poseSize, io.SeekCurrent); err != nil {
				return nil, fmt.Errorf("failed to skip pose verts: %w", err)
			}

			frames = append(frames, AliasFrameDesc{
				FirstPose: poseCount,
				NumPoses:  1,
				Interval:  0.1,
				BBoxMin:   [4]byte{frame.BBoxMin.V[0], frame.BBoxMin.V[1], frame.BBoxMin.V[2], frame.BBoxMin.LightNormalIndex},
				BBoxMax:   [4]byte{frame.BBoxMax.V[0], frame.BBoxMax.V[1], frame.BBoxMax.V[2], frame.BBoxMax.LightNormalIndex},
				Frame:     len(frames),
				Name:      [16]byte(frame.Name),
			})
			poseCount++
		} else {
			var group DAliasGroup
			if err := binary.Read(r, binary.LittleEndian, &group); err != nil {
				return nil, fmt.Errorf("failed to read frame group: %w", err)
			}

			// Read intervals
			intervals := make([]DAliasInterval, group.NumFrames)
			if err := binary.Read(r, binary.LittleEndian, &intervals); err != nil {
				return nil, fmt.Errorf("failed to read frame intervals: %w", err)
			}

			// Read all poses
			for i := 0; i < int(group.NumFrames); i++ {
				var bboxMin, bboxMax TriVertX
				if err := binary.Read(r, binary.LittleEndian, &bboxMin); err != nil {
					return nil, fmt.Errorf("failed to read pose bbox: %w", err)
				}
				if err := binary.Read(r, binary.LittleEndian, &bboxMax); err != nil {
					return nil, fmt.Errorf("failed to read pose bbox: %w", err)
				}

				// Skip pose vertices
				poseSize := int64(header.NumVerts) * 4
				if _, err := r.Seek(poseSize, io.SeekCurrent); err != nil {
					return nil, fmt.Errorf("failed to skip pose verts: %w", err)
				}
			}

			// Create frame entry for the group
			frames = append(frames, AliasFrameDesc{
				FirstPose: poseCount,
				NumPoses:  int(group.NumFrames),
				Interval:  intervals[0].Interval,
				Frame:     len(frames),
			})
			poseCount += int(group.NumFrames)
		}
	}

	// Build the alias header
	alias := &AliasHeader{
		Ident:          int(header.Ident),
		Version:        int(header.Version),
		Scale:          header.Scale,
		ScaleOrigin:    header.ScaleOrigin,
		BoundingRadius: header.BoundingRadius,
		EyePosition:    header.EyePosition,
		NumSkins:       int(header.NumSkins),
		SkinWidth:      int(header.SkinWidth),
		SkinHeight:     int(header.SkinHeight),
		NumVerts:       int(header.NumVerts),
		NumTris:        int(header.NumTris),
		NumFrames:      int(header.NumFrames),
		SyncType:       SyncType(header.SyncType),
		Flags:          int(header.Flags),
		Size:           header.Size,
		NumPoses:       poseCount,
		PoseVertType:   0, // PV_QUAKE1
		Frames:         frames,
	}

	_ = stVerts
	_ = tris

	return alias, nil
}

// DecodeVertex decodes a compressed vertex to world coordinates.
func DecodeVertex(v TriVertX, scale, origin types.Vec3) types.Vec3 {
	return types.Vec3{
		X: origin.X + float32(v.V[0])*scale.X,
		Y: origin.Y + float32(v.V[1])*scale.Y,
		Z: origin.Z + float32(v.V[2])*scale.Z,
	}
}

// NormalsTable contains the precomputed normal vectors for MDL models.
var NormalsTable = [...]types.Vec3{
	{X: -0.525731, Y: 0.000000, Z: 0.850651},
	{X: -0.442863, Y: 0.238856, Z: 0.864188},
	{X: -0.295242, Y: 0.000000, Z: 0.955423},
	{X: -0.309017, Y: 0.500000, Z: 0.809017},
	{X: -0.162460, Y: 0.262866, Z: 0.951056},
	{X: 0.000000, Y: 0.000000, Z: 1.000000},
	{X: 0.000000, Y: 0.850651, Z: 0.525731},
	{X: -0.147621, Y: 0.716567, Z: 0.681718},
	{X: 0.147621, Y: 0.716567, Z: 0.681718},
	{X: 0.000000, Y: 0.525731, Z: 0.850651},
	{X: 0.309017, Y: 0.500000, Z: 0.809017},
	{X: 0.525731, Y: 0.000000, Z: 0.850651},
	{X: 0.295242, Y: 0.000000, Z: 0.955423},
	{X: 0.442863, Y: 0.238856, Z: 0.864188},
	{X: 0.162460, Y: 0.262866, Z: 0.951056},
	{X: -0.681718, Y: 0.147621, Z: 0.716567},
	{X: -0.809017, Y: 0.309017, Z: 0.500000},
	{X: -0.587785, Y: 0.425325, Z: 0.688191},
	{X: -0.850651, Y: 0.525731, Z: 0.000000},
	{X: -0.864188, Y: 0.442863, Z: 0.238856},
	{X: -0.716567, Y: 0.681718, Z: 0.147621},
	{X: -0.688191, Y: 0.587785, Z: 0.425325},
	{X: -0.500000, Y: 0.809017, Z: 0.309017},
	{X: -0.238856, Y: 0.864188, Z: 0.442863},
	{X: -0.425325, Y: 0.688191, Z: 0.587785},
	{X: -0.716567, Y: 0.681718, Z: -0.147621},
	{X: -0.500000, Y: 0.809017, Z: -0.309017},
	{X: -0.525731, Y: 0.850651, Z: 0.000000},
	{X: 0.000000, Y: 0.850651, Z: -0.525731},
	{X: -0.238856, Y: 0.864188, Z: -0.442863},
	{X: 0.000000, Y: 0.955423, Z: -0.295242},
	{X: -0.262866, Y: 0.951056, Z: -0.162460},
	{X: 0.000000, Y: 1.000000, Z: 0.000000},
	{X: 0.000000, Y: 0.955423, Z: 0.295242},
	{X: -0.262866, Y: 0.951056, Z: 0.162460},
	{X: 0.238856, Y: 0.864188, Z: 0.442863},
	{X: 0.262866, Y: 0.951056, Z: 0.162460},
	{X: 0.500000, Y: 0.809017, Z: 0.309017},
	{X: 0.262866, Y: 0.951056, Z: -0.162460},
	{X: 0.238856, Y: 0.864188, Z: -0.442863},
	{X: 0.262866, Y: 0.951056, Z: -0.162460},
	{X: 0.500000, Y: 0.809017, Z: -0.309017},
	{X: 0.850651, Y: 0.525731, Z: 0.000000},
	{X: 0.716567, Y: 0.681718, Z: 0.147621},
	{X: 0.716567, Y: 0.681718, Z: -0.147621},
	{X: 0.525731, Y: 0.850651, Z: 0.000000},
	{X: 0.425325, Y: 0.688191, Z: 0.587785},
	{X: 0.864188, Y: 0.442863, Z: 0.238856},
	{X: 0.688191, Y: 0.587785, Z: 0.425325},
	{X: 0.809017, Y: 0.309017, Z: 0.500000},
	{X: 0.681718, Y: 0.147621, Z: 0.716567},
	{X: 0.587785, Y: 0.425325, Z: 0.688191},
	{X: 0.955423, Y: 0.295242, Z: 0.000000},
	{X: 1.000000, Y: 0.000000, Z: 0.000000},
	{X: 0.951056, Y: 0.162460, Z: 0.262866},
	{X: 0.850651, Y: -0.525731, Z: 0.000000},
	{X: 0.955423, Y: -0.295242, Z: 0.000000},
	{X: 0.864188, Y: -0.442863, Z: 0.238856},
	{X: 0.951056, Y: -0.162460, Z: 0.262866},
	{X: 0.809017, Y: -0.309017, Z: 0.500000},
	{X: 0.681718, Y: -0.147621, Z: 0.716567},
	{X: 0.850651, Y: 0.000000, Z: 0.525731},
	{X: 0.864188, Y: 0.442863, Z: -0.238856},
	{X: 0.809017, Y: 0.309017, Z: -0.500000},
	{X: 0.951056, Y: 0.162460, Z: -0.262866},
	{X: 0.525731, Y: 0.000000, Z: -0.850651},
	{X: 0.681718, Y: 0.147621, Z: -0.716567},
	{X: 0.681718, Y: -0.147621, Z: -0.716567},
	{X: 0.850651, Y: 0.000000, Z: -0.525731},
	{X: 0.809017, Y: -0.309017, Z: -0.500000},
	{X: 0.864188, Y: -0.442863, Z: -0.238856},
	{X: 0.951056, Y: -0.162460, Z: -0.262866},
	{X: 0.147621, Y: 0.716567, Z: -0.681718},
	{X: 0.309017, Y: 0.500000, Z: -0.809017},
	{X: 0.425325, Y: 0.688191, Z: -0.587785},
	{X: 0.442863, Y: 0.238856, Z: -0.864188},
	{X: 0.587785, Y: 0.425325, Z: -0.688191},
	{X: 0.688191, Y: 0.587785, Z: -0.425325},
	{X: -0.147621, Y: 0.716567, Z: -0.681718},
	{X: -0.309017, Y: 0.500000, Z: -0.809017},
	{X: 0.000000, Y: 0.525731, Z: -0.850651},
	{X: -0.525731, Y: 0.000000, Z: -0.850651},
	{X: -0.442863, Y: 0.238856, Z: -0.864188},
	{X: -0.295242, Y: 0.000000, Z: -0.955423},
	{X: -0.162460, Y: 0.262866, Z: -0.951056},
	{X: 0.000000, Y: 0.000000, Z: -1.000000},
	{X: 0.295242, Y: 0.000000, Z: -0.955423},
	{X: 0.162460, Y: 0.262866, Z: -0.951056},
	{X: -0.442863, Y: -0.238856, Z: -0.864188},
	{X: -0.309017, Y: -0.500000, Z: -0.809017},
	{X: -0.162460, Y: -0.262866, Z: -0.951056},
	{X: 0.000000, Y: -0.850651, Z: -0.525731},
	{X: -0.147621, Y: -0.716567, Z: -0.681718},
	{X: 0.147621, Y: -0.716567, Z: -0.681718},
	{X: 0.000000, Y: -0.525731, Z: -0.850651},
	{X: 0.309017, Y: -0.500000, Z: -0.809017},
	{X: 0.442863, Y: -0.238856, Z: -0.864188},
	{X: 0.162460, Y: -0.262866, Z: -0.951056},
	{X: 0.238856, Y: -0.864188, Z: -0.442863},
	{X: 0.500000, Y: -0.809017, Z: -0.309017},
	{X: 0.425325, Y: -0.688191, Z: -0.587785},
	{X: 0.716567, Y: -0.681718, Z: -0.147621},
	{X: 0.688191, Y: -0.587785, Z: -0.425325},
	{X: 0.587785, Y: -0.425325, Z: -0.688191},
	{X: 0.000000, Y: -0.955423, Z: -0.295242},
	{X: 0.000000, Y: -1.000000, Z: 0.000000},
	{X: 0.262866, Y: -0.951056, Z: -0.162460},
	{X: 0.000000, Y: -0.850651, Z: 0.525731},
	{X: 0.000000, Y: -0.955423, Z: 0.295242},
	{X: 0.262866, Y: -0.951056, Z: 0.162460},
	{X: 0.262866, Y: -0.951056, Z: 0.162460},
	{X: 0.500000, Y: -0.809017, Z: 0.309017},
	{X: 0.716567, Y: -0.681718, Z: 0.147621},
	{X: 0.525731, Y: -0.850651, Z: 0.000000},
	{X: -0.238856, Y: -0.864188, Z: -0.442863},
	{X: -0.500000, Y: -0.809017, Z: -0.309017},
	{X: -0.262866, Y: -0.951056, Z: -0.162460},
	{X: -0.850651, Y: -0.525731, Z: 0.000000},
	{X: -0.716567, Y: -0.681718, Z: -0.147621},
	{X: -0.716567, Y: -0.681718, Z: 0.147621},
	{X: -0.525731, Y: -0.850651, Z: 0.000000},
	{X: -0.500000, Y: -0.809017, Z: 0.309017},
	{X: -0.238856, Y: -0.864188, Z: 0.442863},
	{X: -0.262866, Y: -0.951056, Z: 0.162460},
	{X: -0.864188, Y: -0.442863, Z: 0.238856},
	{X: -0.809017, Y: -0.309017, Z: 0.500000},
	{X: -0.688191, Y: -0.587785, Z: 0.425325},
	{X: -0.681718, Y: -0.147621, Z: 0.716567},
	{X: -0.442863, Y: -0.238856, Z: 0.864188},
	{X: -0.587785, Y: -0.425325, Z: 0.688191},
	{X: -0.309017, Y: -0.500000, Z: 0.809017},
	{X: -0.147621, Y: -0.716567, Z: 0.681718},
	{X: -0.425325, Y: -0.688191, Z: 0.587785},
	{X: -0.162460, Y: -0.262866, Z: 0.951056},
	{X: 0.442863, Y: -0.238856, Z: 0.864188},
	{X: 0.162460, Y: -0.262866, Z: 0.951056},
	{X: 0.309017, Y: -0.500000, Z: 0.809017},
	{X: 0.147621, Y: -0.716567, Z: 0.681718},
	{X: 0.000000, Y: -0.525731, Z: 0.850651},
	{X: 0.425325, Y: -0.688191, Z: 0.587785},
	{X: 0.587785, Y: -0.425325, Z: 0.688191},
	{X: 0.688191, Y: -0.587785, Z: 0.425325},
	{X: -0.955423, Y: 0.295242, Z: 0.000000},
	{X: -0.951056, Y: 0.162460, Z: 0.262866},
	{X: -1.000000, Y: 0.000000, Z: 0.000000},
	{X: -0.850651, Y: 0.000000, Z: 0.525731},
	{X: -0.955423, Y: -0.295242, Z: 0.000000},
	{X: -0.951056, Y: -0.162460, Z: 0.262866},
	{X: -0.864188, Y: 0.442863, Z: -0.238856},
	{X: -0.951056, Y: 0.162460, Z: -0.262866},
	{X: -0.809017, Y: 0.309017, Z: -0.500000},
	{X: -0.864188, Y: -0.442863, Z: -0.238856},
	{X: -0.951056, Y: -0.162460, Z: -0.262866},
	{X: -0.809017, Y: -0.309017, Z: -0.500000},
	{X: -0.681718, Y: 0.147621, Z: -0.716567},
	{X: -0.681718, Y: -0.147621, Z: -0.716567},
	{X: -0.850651, Y: 0.000000, Z: -0.525731},
	{X: -0.688191, Y: 0.587785, Z: -0.425325},
	{X: -0.587785, Y: 0.425325, Z: -0.688191},
	{X: -0.425325, Y: 0.688191, Z: -0.587785},
	{X: -0.425325, Y: -0.688191, Z: -0.587785},
	{X: -0.587785, Y: -0.425325, Z: -0.688191},
	{X: -0.688191, Y: -0.587785, Z: -0.425325},
}

// GetNormal returns the normal vector for the given normal index.
func GetNormal(index byte) types.Vec3 {
	if int(index) >= len(NormalsTable) {
		return types.Vec3{X: 0, Y: 0, Z: 1}
	}
	return NormalsTable[index]
}

// Float32FromBits converts a uint32 to a float32 using IEEE 754 representation.
func Float32FromBits(b uint32) float32 {
	return math.Float32frombits(b)
}
