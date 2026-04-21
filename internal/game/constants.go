package game

const (
	// Version constants
	VersionMajor = 0
	VersionMinor = 2
	VersionPatch = 0

	// RuntimeMaxPredictedXYOffset is the maximum predicted XY offset.
	RuntimeMaxPredictedXYOffset = 4.0

	// CSQC picture flags for precaching
	CSQCPicFlagAuto   uint32 = 0
	CSQCPicFlagBlock  uint32 = 1 << 9
	CSQCPicFlagNoLoad uint32 = 1 << 31
)
