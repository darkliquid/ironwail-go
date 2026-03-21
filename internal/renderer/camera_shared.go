package renderer

// projectionFOVForCamera returns the horizontal FOV that should feed the
// projection matrix after backend-independent camera modifiers are applied.
func projectionFOVForCamera(camera CameraState) float32 {
	fov := camera.FOV
	if camera.WaterwarpFOV {
		fov = ApplyWaterwarpFOV(fov, camera.Time)
	}
	return fov
}
