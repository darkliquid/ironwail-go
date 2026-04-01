module github.com/darkliquid/ironwail-go

go 1.26

require (
	github.com/Zyko0/go-sdl3 v0.0.0-20260125144524-02de3d449cb1
	github.com/ebitengine/oto/v3 v3.4.0
	github.com/go-gl/gl v0.0.0-20231021071112-07e5d0ea2e71
	github.com/go-gl/glfw/v3.3/glfw v0.0.0-20250301202403-da16c1255728
	github.com/gogpu/gogpu v0.26.0
	github.com/gogpu/gpucontext v0.11.0
	github.com/gogpu/gputypes v0.3.0
	github.com/gogpu/wgpu v0.23.2
	github.com/gotracker/playback v1.5.0
	github.com/hajimehoshi/go-mp3 v0.3.4
	github.com/jfreymuth/oggvorbis v1.0.5
	github.com/kazzmir/opus-go v1.3.0
	github.com/mewkiz/flac v1.0.13
	golang.org/x/tools v0.42.0
)

require (
	github.com/Zyko0/purego-gen v0.0.0-20250727121216-3bcd331a1e0c // indirect
	github.com/ebitengine/purego v0.10.0 // indirect
	github.com/go-webgpu/goffi v0.5.0 // indirect
	github.com/go-webgpu/webgpu v0.4.3 // indirect
	github.com/gogpu/naga v0.15.2 // indirect
	github.com/gotracker/goaudiofile v1.0.16 // indirect
	github.com/gotracker/opl2 v1.0.2 // indirect
	github.com/heucuva/comparison v1.0.0 // indirect
	github.com/heucuva/optional v0.0.1 // indirect
	github.com/icza/bitio v1.1.0 // indirect
	github.com/jfreymuth/pulse v0.1.1 // indirect
	github.com/jfreymuth/vorbis v1.0.2 // indirect
	github.com/mewkiz/pkg v0.0.0-20250417130911-3f050ff8c56d // indirect
	github.com/mewpkg/term v0.0.0-20241026122259-37a80af23985 // indirect
	golang.org/x/exp v0.0.0-20251209150349-8475f28825e9 // indirect
	golang.org/x/mod v0.33.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
)

tool golang.org/x/tools/cmd/stringer

replace github.com/ebitengine/oto/v3 => ../oto
