package alias

import (
	"fmt"
	"math"

	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
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

	Origin         qtypes.Vec3
	Angles         qtypes.Vec3
	PreviousOrigin qtypes.Vec3
	CurrentOrigin  qtypes.Vec3
	PreviousAngles qtypes.Vec3
	CurrentAngles  qtypes.Vec3

	ModelID  string
	SkinNum  int
	ColorMap uint32
	IsPlayer bool
}

type LerpData struct {
	Pose1 int
	Pose2 int
	Blend float32

	Origin qtypes.Vec3
	Angles qtypes.Vec3
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

	shouldLerp := lerpModels && hdr.Flags&ModNoLerp == 0
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
func SetupEntityTransform(e *AliasEntity, timeSeconds float64, lerpMove bool, isViewEntity bool, chaseActive bool, demoPlayback bool, demoSpeed float64) (origin qtypes.Vec3, angles qtypes.Vec3) {
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

		origin = e.PreviousOrigin.Add(e.CurrentOrigin.Sub(e.PreviousOrigin).Scale(blend))

		diffAngles := e.CurrentAngles.Sub(e.PreviousAngles)
		if diffAngles.X > 180 {
			diffAngles.X -= 360
		} else if diffAngles.X < -180 {
			diffAngles.X += 360
		}
		if diffAngles.Y > 180 {
			diffAngles.Y -= 360
		} else if diffAngles.Y < -180 {
			diffAngles.Y += 360
		}
		if diffAngles.Z > 180 {
			diffAngles.Z -= 360
		} else if diffAngles.Z < -180 {
			diffAngles.Z += 360
		}
		angles = e.PreviousAngles.Add(diffAngles.Scale(blend))
	} else {
		origin = e.Origin
		angles = e.Angles
	}

	if chaseActive && isViewEntity {
		angles.X *= 0.3
	}

	return origin, angles
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
