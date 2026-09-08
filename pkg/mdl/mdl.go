package mdl

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/darkliquid/ironwail-go/pkg/types"
)

const (
	// Ident is the magic number for MDL files ("IDPO" in little-endian).
	Ident = 0x4F504449
	// Version is the expected version number for Quake 1 MDL files.
	Version = 6
	// OnSeam is a flag for vertices on the texture seam.
	OnSeam = 0x0020
	// FacesFront is a flag for front-facing triangles.
	FacesFront = 0x0010

	// Model limit constants
	MaxVerts  = 0x7fff
	MaxFrames = 1024
	MaxTris   = 4096
	MaxSkins  = 32

	// Legacy aliases
	MDLIdent       = Ident
	MDLVersion     = Version
	MDLOnSeam      = OnSeam
	MDLFacesFront  = FacesFront
	MaxAliasVerts  = MaxVerts
	MaxAliasFrames = MaxFrames
	MaxAliasTris   = MaxTris
)

// FrameType distinguishes between single frames and frame groups.
type FrameType int

const (
	FrameSingle FrameType = 0
	FrameGroup  FrameType = 1

	// Legacy aliases
	AliasSingle = FrameSingle
	AliasGroup  = FrameGroup
)

// AliasFrameType is a type alias for backwards compatibility.
type AliasFrameType = FrameType

// SkinType distinguishes between single skins and skin groups.
type SkinType int

const (
	SkinSingle SkinType = 0
	SkinGroup  SkinType = 1

	// Legacy aliases
	AliasSkinSingle = SkinSingle
	AliasSkinGroup  = SkinGroup
)

// AliasSkinType is a type alias for backwards compatibility.
type AliasSkinType = SkinType

// SyncType determines how model animations are synchronized.
type SyncType int

const (
	SyncSTSync      SyncType = 0 // Synchronized animation
	SyncSTRand      SyncType = 1 // Random animation
	SyncSTFrameTime SyncType = 2 // Sync to frame changes

	// Legacy aliases
	STSync      = SyncSTSync
	STRand      = SyncSTRand
	STFrameTime = SyncSTFrameTime
)

// Header represents the on-disk MDL file header.
type Header struct {
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

// MDLHeader is an alias for Header.
type MDLHeader = Header

// STVert represents an on-disk skin texture vertex coordinate.
type STVert struct {
	OnSeam int32
	S      int32
	T      int32
}

// Triangle represents an on-disk triangle definition.
type Triangle struct {
	FacesFront int32
	VertIndex  [3]int32
}

// DTriangle is an alias for Triangle.
type DTriangle = Triangle

// TriVertX represents a compressed vertex position and normal index.
type TriVertX struct {
	V                [3]byte
	LightNormalIndex byte
}

// FrameDesc describes an alias model animation frame.
type FrameDesc struct {
	FirstPose int
	NumPoses  int
	Interval  float32
	BBoxMin   [4]byte // trivertx_t
	BBoxMax   [4]byte // trivertx_t
	Frame     int
	Name      [16]byte
}

// AliasFrameDesc is an alias for FrameDesc.
type AliasFrameDesc = FrameDesc

// SkinDesc describes a logical alias skin entry and the flat skin-frame
// range it owns inside File.Skins.
type SkinDesc struct {
	FirstFrame int
	NumFrames  int
	Intervals  []float32
}

// AliasSkinDesc is an alias for SkinDesc.
type AliasSkinDesc = SkinDesc

// Bounds stores bounding boxes for an alias model.
type Bounds struct {
	Mins, Maxs   types.Vec3
	YMins, YMaxs types.Vec3 // Yaw-rotated bounds
	RMins, RMaxs types.Vec3 // Pitch/roll-rotated bounds (spherical)
}

// File represents a loaded, parsed Quake Alias model (.mdl).
type File struct {
	Ident          int
	Version        int
	Scale          types.Vec3
	ScaleOrigin    types.Vec3
	BoundingRadius float32
	EyePosition    types.Vec3
	NumSkins       int
	SkinWidth      int
	SkinHeight     int
	NumVerts       int
	NumTris        int
	NumFrames      int
	SyncType       SyncType
	Flags          int
	Size           float32
	NumVertsVBO    int
	NumPoses       int
	PoseVertType   int // PV_QUAKE1, PV_IQM, PV_MD3
	Skins          [][]byte
	SkinDescs      []SkinDesc
	STVerts        []STVert
	Triangles      []Triangle
	Poses          [][]TriVertX
	Frames         []FrameDesc
	Bounds         Bounds
}

// AliasHeader is an alias for File for backwards compatibility with internal/model.
type AliasHeader = File

// On-disk format types
type DAliasFrame struct {
	BBoxMin TriVertX
	BBoxMax TriVertX
	Name    [16]byte
}

type DAliasGroup struct {
	NumFrames int32
	BBoxMin   TriVertX
	BBoxMax   TriVertX
}

type DAliasInterval struct {
	Interval float32
}

type DAliasSkinGroup struct {
	NumSkins int32
}

type DAliasSkinInterval struct {
	Interval float32
}

type DAliasFrameType struct {
	Type int32
}

type DAliasSkinType struct {
	Type int32
}

// Reader reads MDL (alias model) files.
type Reader struct {
	r io.ReadSeeker
}

// NewReader creates a new MDL reader.
func NewReader(r io.ReadSeeker) *Reader {
	return &Reader{r: r}
}

// ReadHeader reads and validates the MDL header.
func (r *Reader) ReadHeader() (*Header, error) {
	var header Header
	if err := binary.Read(r.r, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("failed to read MDL header: %w", err)
	}

	if header.Ident != Ident {
		return nil, fmt.Errorf("invalid MDL ident: got 0x%08x, expected 0x%08x", header.Ident, Ident)
	}

	if header.Version != Version {
		return nil, fmt.Errorf("unsupported MDL version: got %d, expected %d", header.Version, Version)
	}

	return &header, nil
}

// ReadSTVerts reads the skin texture vertices.
func (r *Reader) ReadSTVerts(count int) ([]STVert, error) {
	verts := make([]STVert, count)
	if err := binary.Read(r.r, binary.LittleEndian, &verts); err != nil {
		return nil, fmt.Errorf("failed to read ST verts: %w", err)
	}
	return verts, nil
}

// ReadTriangles reads the triangle definitions.
func (r *Reader) ReadTriangles(count int) ([]Triangle, error) {
	tris := make([]Triangle, count)
	if err := binary.Read(r.r, binary.LittleEndian, &tris); err != nil {
		return nil, fmt.Errorf("failed to read triangles: %w", err)
	}
	return tris, nil
}

// ReadSkin reads a single skin texture.
func (r *Reader) ReadSkin(width, height int) ([]byte, error) {
	size := width * height
	data := make([]byte, size)
	if _, err := io.ReadFull(r.r, data); err != nil {
		return nil, fmt.Errorf("failed to read skin: %w", err)
	}
	return data, nil
}

const defaultFrameInterval = 0.1

// Load reads and parses a complete Quake .mdl file from r.
func Load(r io.ReadSeeker) (*File, error) {
	var header Header
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("failed to read alias header: %w", err)
	}

	if header.Ident != Ident {
		return nil, fmt.Errorf("invalid MDL ident: got 0x%08x, expected 0x%08x", header.Ident, Ident)
	}
	if header.Version != Version {
		return nil, fmt.Errorf("unsupported MDL version: got %d, expected %d", header.Version, Version)
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
	if numVerts > MaxVerts {
		return nil, fmt.Errorf("alias model has too many vertices: %d (max %d)", numVerts, MaxVerts)
	}
	if numTris <= 0 {
		return nil, fmt.Errorf("alias model has no triangles")
	}
	if numFrames < 1 {
		return nil, fmt.Errorf("invalid number of frames: %d", numFrames)
	}

	// 1. Skins
	skins, skinDescs, err := readSkins(r, numSkins, int(header.SkinWidth), int(header.SkinHeight))
	if err != nil {
		return nil, err
	}

	// 2. STVerts
	stVerts := make([]STVert, numVerts)
	if err := binary.Read(r, binary.LittleEndian, &stVerts); err != nil {
		return nil, fmt.Errorf("failed to read ST verts: %w", err)
	}

	// 3. Triangles
	triangles := make([]Triangle, numTris)
	if err := binary.Read(r, binary.LittleEndian, &triangles); err != nil {
		return nil, fmt.Errorf("failed to read triangles: %w", err)
	}

	// 4. Frames & Poses
	frames, poses, numPoses, bounds, err := readFrames(r, numFrames, numVerts, header.Scale, header.ScaleOrigin)
	if err != nil {
		return nil, err
	}

	file := &File{
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
		Bounds:         bounds,
	}

	return file, nil
}

func readSkins(r io.Reader, numSkins, skinWidth, skinHeight int) ([][]byte, []SkinDesc, error) {
	skinSize := skinWidth * skinHeight
	if skinSize < 0 {
		return nil, nil, fmt.Errorf("invalid skin dimensions: %dx%d", skinWidth, skinHeight)
	}

	skins := make([][]byte, 0, numSkins)
	skinDescs := make([]SkinDesc, 0, numSkins)
	for i := 0; i < numSkins; i++ {
		var skinType DAliasSkinType
		if err := binary.Read(r, binary.LittleEndian, &skinType); err != nil {
			return nil, nil, fmt.Errorf("failed to read skin type %d: %w", i, err)
		}

		switch SkinType(skinType.Type) {
		case SkinSingle:
			skin := make([]byte, skinSize)
			if _, err := io.ReadFull(r, skin); err != nil {
				return nil, nil, fmt.Errorf("failed to read single skin %d: %w", i, err)
			}
			firstFrame := len(skins)
			skins = append(skins, skin)
			skinDescs = append(skinDescs, SkinDesc{FirstFrame: firstFrame, NumFrames: 1})
		case SkinGroup:
			var group DAliasSkinGroup
			if err := binary.Read(r, binary.LittleEndian, &group); err != nil {
				return nil, nil, fmt.Errorf("failed to read skin group %d: %w", i, err)
			}

			n := int(group.NumSkins)
			if n < 1 {
				return nil, nil, fmt.Errorf("invalid number of grouped skins for skin %d: %d", i, n)
			}

			intervals := make([]DAliasSkinInterval, n)
			if err := binary.Read(r, binary.LittleEndian, &intervals); err != nil {
				return nil, nil, fmt.Errorf("failed to read skin group intervals %d: %w", i, err)
			}

			firstFrame := len(skins)
			skinIntervals := make([]float32, n)
			for skinIndex := 0; skinIndex < n; skinIndex++ {
				skin := make([]byte, skinSize)
				if _, err := io.ReadFull(r, skin); err != nil {
					return nil, nil, fmt.Errorf("failed to read grouped skin %d:%d: %w", i, skinIndex, err)
				}
				skins = append(skins, skin)
				skinIntervals[skinIndex] = intervals[skinIndex].Interval
			}
			skinDescs = append(skinDescs, SkinDesc{
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

func readFrames(r io.Reader, numFrames, numVerts int, scale, origin types.Vec3) ([]FrameDesc, [][]TriVertX, int, Bounds, error) {
	frames := make([]FrameDesc, 0, numFrames)
	poses := make([][]TriVertX, 0, numFrames)
	bounds := Bounds{
		Mins: types.Vec3{X: math.MaxFloat32, Y: math.MaxFloat32, Z: math.MaxFloat32},
		Maxs: types.Vec3{X: -math.MaxFloat32, Y: -math.MaxFloat32, Z: -math.MaxFloat32},
	}
	poseCount := 0
	var yawRadiusSquared float32
	var radiusSquared float32

	for i := 0; i < numFrames; i++ {
		var frameType DAliasFrameType
		if err := binary.Read(r, binary.LittleEndian, &frameType); err != nil {
			return nil, nil, 0, Bounds{}, fmt.Errorf("failed to read frame type %d: %w", i, err)
		}

		switch FrameType(frameType.Type) {
		case FrameSingle:
			var frame DAliasFrame
			if err := binary.Read(r, binary.LittleEndian, &frame); err != nil {
				return nil, nil, 0, Bounds{}, fmt.Errorf("failed to read single frame %d: %w", i, err)
			}

			poseVerts := make([]TriVertX, numVerts)
			if err := binary.Read(r, binary.LittleEndian, &poseVerts); err != nil {
				return nil, nil, 0, Bounds{}, fmt.Errorf("failed to read single frame pose verts %d: %w", i, err)
			}
			poses = append(poses, poseVerts)
			updateBounds(poseVerts, scale, origin, &bounds, &yawRadiusSquared, &radiusSquared)

			frames = append(frames, FrameDesc{
				FirstPose: poseCount,
				NumPoses:  1,
				Interval:  defaultFrameInterval,
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
			if poseCount > MaxFrames {
				return nil, nil, 0, Bounds{}, fmt.Errorf("too many alias poses: %d (max %d)", poseCount, MaxFrames)
			}

		case FrameGroup:
			var group DAliasGroup
			if err := binary.Read(r, binary.LittleEndian, &group); err != nil {
				return nil, nil, 0, Bounds{}, fmt.Errorf("failed to read frame group %d: %w", i, err)
			}

			n := int(group.NumFrames)
			if n < 1 {
				return nil, nil, 0, Bounds{}, fmt.Errorf("invalid number of grouped frames for frame %d: %d", i, n)
			}

			intervals := make([]DAliasInterval, n)
			if err := binary.Read(r, binary.LittleEndian, &intervals); err != nil {
				return nil, nil, 0, Bounds{}, fmt.Errorf("failed to read frame intervals for frame %d: %w", i, err)
			}

			firstPose := poseCount
			for poseIndex := 0; poseIndex < n; poseIndex++ {
				var pose DAliasFrame
				if err := binary.Read(r, binary.LittleEndian, &pose); err != nil {
					return nil, nil, 0, Bounds{}, fmt.Errorf("failed to read group pose %d for frame %d: %w", poseIndex, i, err)
				}

				poseVerts := make([]TriVertX, numVerts)
				if err := binary.Read(r, binary.LittleEndian, &poseVerts); err != nil {
					return nil, nil, 0, Bounds{}, fmt.Errorf("failed to read group pose verts %d for frame %d: %w", poseIndex, i, err)
				}
				poses = append(poses, poseVerts)
				updateBounds(poseVerts, scale, origin, &bounds, &yawRadiusSquared, &radiusSquared)

				poseCount++
				if poseCount > MaxFrames {
					return nil, nil, 0, Bounds{}, fmt.Errorf("too many alias poses: %d (max %d)", poseCount, MaxFrames)
				}
			}

			frames = append(frames, FrameDesc{
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
			return nil, nil, 0, Bounds{}, fmt.Errorf("invalid frame type %d for frame %d", frameType.Type, i)
		}
	}

	if poseCount == 0 {
		return nil, nil, 0, Bounds{}, fmt.Errorf("alias model has no poses")
	}

	radius := float32(math.Sqrt(float64(radiusSquared)))
	bounds.RMins = types.Vec3{X: -radius, Y: -radius, Z: -radius}
	bounds.RMaxs = types.Vec3{X: radius, Y: radius, Z: radius}

	yawRadius := float32(math.Sqrt(float64(yawRadiusSquared)))
	bounds.YMins = types.Vec3{X: -yawRadius, Y: -yawRadius, Z: bounds.Mins.Z}
	bounds.YMaxs = types.Vec3{X: yawRadius, Y: yawRadius, Z: bounds.Maxs.Z}

	return frames, poses, poseCount, bounds, nil
}

func updateBounds(verts []TriVertX, scale, origin types.Vec3, bounds *Bounds, yawRadiusSquared, radiusSquared *float32) {
	for _, v := range verts {
		decoded := DecodeVertex(v, scale, origin)
		if decoded.X < bounds.Mins.X {
			bounds.Mins.X = decoded.X
		}
		if decoded.X > bounds.Maxs.X {
			bounds.Maxs.X = decoded.X
		}
		if decoded.Y < bounds.Mins.Y {
			bounds.Mins.Y = decoded.Y
		}
		if decoded.Y > bounds.Maxs.Y {
			bounds.Maxs.Y = decoded.Y
		}
		if decoded.Z < bounds.Mins.Z {
			bounds.Mins.Z = decoded.Z
		}
		if decoded.Z > bounds.Maxs.Z {
			bounds.Maxs.Z = decoded.Z
		}

		dist := decoded.X*decoded.X + decoded.Y*decoded.Y
		if dist > *yawRadiusSquared {
			*yawRadiusSquared = dist
		}

		dist += decoded.Z * decoded.Z
		if dist > *radiusSquared {
			*radiusSquared = dist
		}
	}
}

// ResolveSkinFrame maps a logical skin selection and time value to a concrete
// flattened skin-frame index inside Skins.
func (f *File) ResolveSkinFrame(skinNum int, timeSeconds float64) int {
	if f == nil || len(f.Skins) == 0 {
		return 0
	}

	descCount := len(f.SkinDescs)
	if descCount == 0 {
		if skinNum < 0 {
			skinNum = 0
		}
		return skinNum % len(f.Skins)
	}
	if skinNum < 0 {
		skinNum = 0
	}
	skinNum %= descCount
	desc := f.SkinDescs[skinNum]
	if desc.NumFrames <= 1 {
		return clampSkinFrame(desc.FirstFrame, len(f.Skins))
	}
	if len(desc.Intervals) >= desc.NumFrames {
		fullInterval := float64(desc.Intervals[desc.NumFrames-1])
		if fullInterval > 0 {
			target := math.Mod(timeSeconds, fullInterval)
			if target < 0 {
				target += fullInterval
			}
			for i, interval := range desc.Intervals[:desc.NumFrames] {
				if float64(interval) > target {
					return clampSkinFrame(desc.FirstFrame+i, len(f.Skins))
				}
			}
			return clampSkinFrame(desc.FirstFrame+desc.NumFrames-1, len(f.Skins))
		}
	}

	frame := desc.FirstFrame + int(timeSeconds*10)%desc.NumFrames
	if frame < desc.FirstFrame {
		frame = desc.FirstFrame
	}
	return clampSkinFrame(frame, len(f.Skins))
}

func clampSkinFrame(index, count int) int {
	if count <= 0 {
		return 0
	}
	if index < 0 {
		return 0
	}
	if index >= count {
		return count - 1
	}
	return index
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
