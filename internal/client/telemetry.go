package client

import "github.com/darkliquid/ironwail-go/pkg/types"

type LerpTelemetryReason uint8

const (
	LerpTelemetryReasonNormal LerpTelemetryReason = iota
	LerpTelemetryReasonF0
	LerpTelemetryReasonTimeDemo
	LerpTelemetryReasonFastServer
	LerpTelemetryReasonNoLerp
	LerpTelemetryReasonGapClamp
	LerpTelemetryReasonFracLT0
	LerpTelemetryReasonFracGT1
)

func (r LerpTelemetryReason) String() string {
	switch r {
	case LerpTelemetryReasonF0:
		return "f0"
	case LerpTelemetryReasonTimeDemo:
		return "timedemo"
	case LerpTelemetryReasonFastServer:
		return "fastserver"
	case LerpTelemetryReasonNoLerp:
		return "nolerp"
	case LerpTelemetryReasonGapClamp:
		return "gap_clamp"
	case LerpTelemetryReasonFracLT0:
		return "frac_lt_0"
	case LerpTelemetryReasonFracGT1:
		return "frac_gt_1"
	default:
		return "normal"
	}
}

type LerpTelemetry struct {
	TimeBefore       float64
	TimeAfter        float64
	OldTime          float64
	MTime0Before     float64
	MTime1Before     float64
	MTime0After      float64
	MTime1After      float64
	FrameDeltaBefore float64
	FrameDeltaAfter  float64
	RawFrac          float64
	HasRawFrac       bool
	Frac             float64
	GapClamped       bool
	TimeSnapped      bool
	Reason           LerpTelemetryReason
}

type PredictionReplayTelemetry struct {
	FrameTime                float64
	EntityNum                int
	EntityFound              bool
	Valid                    bool
	ServerBaseOrigin         types.Vec3
	ServerBaseVelocity       types.Vec3
	ServerBaseChanged        bool
	PreviousPredictedOrigin  types.Vec3
	RebasedPredictedOrigin   types.Vec3
	RebasedPredictedVelocity types.Vec3
	OutputPredictedOrigin    types.Vec3
	OutputPredictedVelocity  types.Vec3
	CommandCountBeforeAck    int
	CommandCountAfterAck     int
	ReplayedCommandCount     int
	UsedPendingCmdFallback   bool
	PendingCmd               UserCmd
	OldestReplayedCmd        UserCmd
	NewestReplayedCmd        UserCmd
	HasReplayedCmds          bool
}
