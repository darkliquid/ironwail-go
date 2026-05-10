package renderer

import skyimpl "github.com/darkliquid/ironwail-go/internal/renderer/sky"

var (
	skyboxFaceSuffixes     = skyimpl.SkyboxFaceSuffixes
	skyboxCubemapFaceOrder = skyimpl.SkyboxCubemapFaceOrder
)

type externalSkyboxFace = skyimpl.ExternalSkyboxFace
type externalSkyboxRenderMode = skyimpl.ExternalSkyboxRenderMode
type externalSkyboxWind = skyimpl.ExternalSkyboxWind

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

func loadExternalSkyboxFaces(baseName string, loadFile func(string) ([]byte, error)) (faces [6]externalSkyboxFace, loaded int) {
	return skyimpl.LoadExternalSkyboxFaces(baseName, loadFile)
}

func loadExternalSkyboxWind(baseName string, loadFile func(string) ([]byte, error)) (externalSkyboxWind, bool) {
	return skyimpl.LoadExternalSkyboxWind(baseName, loadFile)
}

func externalSkyboxCubemapEligible(faces [6]externalSkyboxFace, loaded int) bool {
	return skyimpl.ExternalSkyboxCubemapEligible(faces, loaded)
}

func externalSkyboxCubemapFaceSize(faces [6]externalSkyboxFace, loaded int) (int, bool) {
	return skyimpl.ExternalSkyboxCubemapFaceSize(faces, loaded)
}
