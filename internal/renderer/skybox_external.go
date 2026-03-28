package renderer

import skyimpl "github.com/darkliquid/ironwail-go/internal/renderer/sky"

var (
	skyboxFaceSuffixes     = skyimpl.SkyboxFaceSuffixes
	skyboxFaceExts         = skyimpl.SkyboxFaceExts
	skyboxCubemapFaceOrder = skyimpl.SkyboxCubemapFaceOrder
)

type externalSkyboxFace = skyimpl.ExternalSkyboxFace
type externalSkyboxRenderMode = skyimpl.ExternalSkyboxRenderMode

const (
	externalSkyboxRenderEmbedded externalSkyboxRenderMode = skyimpl.ExternalSkyboxRenderEmbedded
	externalSkyboxRenderCubemap  externalSkyboxRenderMode = skyimpl.ExternalSkyboxRenderCubemap
	externalSkyboxRenderFaces    externalSkyboxRenderMode = skyimpl.ExternalSkyboxRenderFaces
)

func selectExternalSkyboxRenderMode(loaded int, cubemapEligible bool) externalSkyboxRenderMode {
	return skyimpl.SelectExternalSkyboxRenderMode(loaded, cubemapEligible)
}

func normalizeSkyboxBaseName(name string) string {
	return skyimpl.NormalizeSkyboxBaseName(name)
}

func skyboxFaceSearchPaths(baseName, suffix string) []string {
	return skyimpl.SkyboxFaceSearchPaths(baseName, suffix)
}

func decodeSkyboxImage(path string, data []byte) (rgba []byte, width, height int, ok bool) {
	return skyimpl.DecodeSkyboxImage(path, data)
}

func loadExternalSkyboxFaces(baseName string, loadFile func(string) ([]byte, error)) (faces [6]externalSkyboxFace, loaded int) {
	return skyimpl.LoadExternalSkyboxFaces(baseName, loadFile)
}

func loadSkyboxFileCandidate(candidate string, loadFile func(string) ([]byte, error)) ([]byte, error) {
	return skyimpl.LoadSkyboxFileCandidate(candidate, loadFile)
}

func externalSkyboxCubemapEligible(faces [6]externalSkyboxFace, loaded int) bool {
	return skyimpl.ExternalSkyboxCubemapEligible(faces, loaded)
}

func externalSkyboxCubemapFaceSize(faces [6]externalSkyboxFace, loaded int) (int, bool) {
	return skyimpl.ExternalSkyboxCubemapFaceSize(faces, loaded)
}
