package alias

import (
	"fmt"
	"math"
)

const (
	LerpResetAnim = 1 << iota
	LerpResetAnim2
	LerpResetMove
	LerpMoveStep
	LerpFinish
)

const (
	ModNoLerp = 256
)

type AliasFrame struct {
	FirstPose int
	NumPoses  int
	Interval  float64
}

type AliasHeader struct {
	NumFrames int
	Flags     int
	Frames    []AliasFrame

	PoseVertType int
	NumBones     int
}

type AliasEntity struct {
	Frame int

	LerpTime   float64
	LerpStart  float64
	LerpFinish float64

	PreviousPose int
	CurrentPose  int

	MoveLerpStart float64

	LerpFlags int

	Origin         [3]float32
	Angles         [3]float32
	PreviousOrigin [3]float32
	CurrentOrigin  [3]float32
	PreviousAngles [3]float32
	CurrentAngles  [3]float32

	ModelID  string
	SkinNum  int
	ColorMap uint32
	IsPlayer bool
}

type LerpData struct {
	Pose1 int
	Pose2 int
	Blend float32

	Origin [3]float32
	Angles [3]float32
}

// SetupAliasFrame computes alias-model frame interpolation data (old/new keyframes plus lerp factor), producing smooth animation from Quake's discrete baked poses.
func SetupAliasFrame(e *AliasEntity, hdr *AliasHeader, timeSeconds float64, lerpModels bool, demoPlayback bool, demoSpeed float64) (LerpData, error) {
	var out LerpData
	if e == nil || hdr == nil {
		return out, fmt.Errorf("nil entity or alias header")
	}
	if hdr.NumFrames <= 0 || len(hdr.Frames) < hdr.NumFrames {
		return out, fmt.Errorf("invalid alias frame table")
	}

	frame := e.Frame
	if frame >= hdr.NumFrames || frame < 0 {
		frame = 0
	}

	frameInfo := hdr.Frames[frame]
	if frameInfo.NumPoses <= 0 {
		return out, fmt.Errorf("invalid pose count for frame %d", frame)
	}

	posenum := frameInfo.FirstPose
	if frameInfo.NumPoses > 1 {
		e.LerpTime = frameInfo.Interval
		if e.LerpTime <= 0 {
			e.LerpTime = 0.1
		}
		posenum += int(timeSeconds/e.LerpTime) % frameInfo.NumPoses
	} else {
		e.LerpTime = 0.1
	}

	if e.LerpFlags&LerpResetAnim != 0 {
		e.LerpStart = 0
		e.PreviousPose = posenum
		e.CurrentPose = posenum
		e.LerpFlags &^= LerpResetAnim
	} else if e.CurrentPose != posenum {
		if e.LerpFlags&LerpResetAnim2 != 0 {
			e.LerpStart = 0
			e.PreviousPose = posenum
			e.CurrentPose = posenum
			e.LerpFlags &^= LerpResetAnim2
		} else {
			e.LerpStart = timeSeconds
			e.PreviousPose = e.CurrentPose
			e.CurrentPose = posenum
		}
	}

	shouldLerp := lerpModels && !(hdr.Flags&ModNoLerp != 0)
	if shouldLerp {
		s := 1.0
		if demoPlayback && demoSpeed < 0 {
			s = -1.0
		}

		if e.LerpFlags&LerpFinish != 0 && frameInfo.NumPoses == 1 {
			out.Blend = clamp01(float32((timeSeconds - e.LerpStart) / (e.LerpFinish - e.LerpStart)))
		} else {
			out.Blend = clamp01(float32((timeSeconds - e.LerpStart) / e.LerpTime * s))
		}

		if out.Blend == 1.0 {
			e.PreviousPose = e.CurrentPose
		}

		out.Pose1 = e.PreviousPose
		out.Pose2 = e.CurrentPose
	} else {
		out.Blend = 1
		out.Pose1 = posenum
		out.Pose2 = posenum
	}

	return out, nil
}

// SetupEntityTransform builds the model matrix from entity origin and Euler angles, placing monsters/items in world space before view/projection transforms.
func SetupEntityTransform(e *AliasEntity, timeSeconds float64, lerpMove bool, isViewEntity bool, chaseActive bool, demoPlayback bool, demoSpeed float64) (origin [3]float32, angles [3]float32) {
	if e == nil {
		return origin, angles
	}

	if e.LerpFlags&LerpResetMove != 0 {
		e.MoveLerpStart = 0
		e.PreviousOrigin = e.Origin
		e.CurrentOrigin = e.Origin
		e.PreviousAngles = e.Angles
		e.CurrentAngles = e.Angles
		e.LerpFlags &^= LerpResetMove
	} else if e.Origin != e.CurrentOrigin || e.Angles != e.CurrentAngles {
		e.MoveLerpStart = timeSeconds
		e.PreviousOrigin = e.CurrentOrigin
		e.CurrentOrigin = e.Origin
		e.PreviousAngles = e.CurrentAngles
		e.CurrentAngles = e.Angles
	}

	if lerpMove && !isViewEntity && e.LerpFlags&LerpMoveStep != 0 {
		s := 1.0
		if demoPlayback && demoSpeed < 0 {
			s = -1.0
		}

		var blend float32
		if e.LerpFlags&LerpFinish != 0 {
			blend = clamp01(float32((timeSeconds - e.MoveLerpStart) / (e.LerpFinish - e.MoveLerpStart)))
		} else {
			blend = clamp01(float32((timeSeconds - e.MoveLerpStart) / 0.1 * s))
		}

		for i := 0; i < 3; i++ {
			d := e.CurrentOrigin[i] - e.PreviousOrigin[i]
			origin[i] = e.PreviousOrigin[i] + d*blend
		}

		for i := 0; i < 3; i++ {
			d := e.CurrentAngles[i] - e.PreviousAngles[i]
			if d > 180 {
				d -= 360
			}
			if d < -180 {
				d += 360
			}
			angles[i] = e.PreviousAngles[i] + d*blend
		}
	} else {
		origin = e.Origin
		angles = e.Angles
	}

	if chaseActive && isViewEntity {
		angles[0] *= 0.3
	}

	return origin, angles
}

type AliasInstance struct {
	WorldMatrix [12]float32
	LightColor  [3]float32
	Alpha       float32
	Pose1       int32
	Pose2       int32
	Blend       float32
}

type AliasBatchKey struct {
	ModelID  string
	SkinNum  int
	ColorMap uint32
	IsPlayer bool
}

type AliasBatch struct {
	maxInstances int
	count        int
	key          AliasBatchKey
	instances    []AliasInstance
}

// NewAliasBatch allocates batching storage for alias draw calls so entities sharing state can be submitted with fewer API transitions.
func NewAliasBatch(maxInstances int) *AliasBatch {
	if maxInstances <= 0 {
		maxInstances = 256
	}
	return &AliasBatch{maxInstances: maxInstances}
}

// Count reports how many batched entries are currently queued, useful for deciding when to flush before state or texture changes.
func (b *AliasBatch) Count() int {
	if b == nil {
		return 0
	}
	return b.count
}

// CanAdd checks batch capacity limits before appending geometry, preventing overflow and preserving contiguous upload/write patterns.
func (b *AliasBatch) CanAdd(key AliasBatchKey) bool {
	if b == nil {
		return false
	}
	if b.count == 0 {
		return true
	}
	if b.count >= b.maxInstances {
		return false
	}
	if b.key.ModelID != key.ModelID || b.key.SkinNum != key.SkinNum {
		return false
	}
	if b.key.IsPlayer {
		return false
	}
	return true
}

// Add appends a surface/lightmap block into an allocator or batch structure, centralizing bounds/capacity checks before write.
func (b *AliasBatch) Add(key AliasBatchKey, instance AliasInstance) bool {
	if !b.CanAdd(key) {
		return false
	}
	if b.count == 0 {
		b.key = key
	}
	if len(b.instances) <= b.count {
		b.instances = append(b.instances, instance)
	} else {
		b.instances[b.count] = instance
	}
	b.count++
	return true
}

// Flush submits the queued alias batch to the GPU and resets counters, trading many small draws for a single larger draw whenever possible.
func (b *AliasBatch) Flush() []AliasInstance {
	if b == nil || b.count == 0 {
		return nil
	}
	out := make([]AliasInstance, b.count)
	copy(out, b.instances[:b.count])
	b.count = 0
	b.key = AliasBatchKey{}
	return out
}

// MatrixTranspose4x3 converts Quake-style transform layout into the matrix packing expected by the active shader path.
func MatrixTranspose4x3(in [16]float32) [12]float32 {
	return [12]float32{
		in[0], in[4], in[8],
		in[1], in[5], in[9],
		in[2], in[6], in[10],
		in[3], in[7], in[11],
	}
}

// clamp01 performs its step in this part of the renderer; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func clamp01(v float32) float32 {
	if math.IsNaN(float64(v)) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
