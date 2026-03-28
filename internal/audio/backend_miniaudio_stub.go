//go:build !(amd64 && (linux || windows))

package audio

func NewMiniaudioBackend() Backend {
	return nil
}
