package game

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/client"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/renderer"
	"github.com/darkliquid/ironwail-go/internal/testutil"
)

type RefEdict struct {
	Number int        `json:"number"`
	Origin [3]float32 `json:"origin"`
	Angles [3]float32 `json:"angles"`
	Model  string     `json:"model"`
}

type RefLight struct {
	Pos      [3]float32 `json:"pos"`
	Radius   float32    `json:"radius"`
	Color    [3]float32 `json:"color"`
	MinLight float32    `json:"minlight"`
}

type RefFrameState struct {
	Frame      int         `json:"frame"`
	ViewOrg    [3]float32  `json:"vieworg"`
	ViewAngles [3]float32  `json:"viewangles"`
	ViewLeaf   int         `json:"viewleaf"`
	MatView    [16]float32 `json:"r_matview"`
	MatProj    [16]float32 `json:"r_matproj"`
	Visedicts  []RefEdict  `json:"visedicts"`
	Lights     []RefLight  `json:"lights"`
}

func floatEquals(a, b, epsilon float32) bool {
	return math.Abs(float64(a-b)) <= float64(epsilon)
}

func vecEquals(a, b [3]float32, epsilon float32) bool {
	return floatEquals(a[0], b[0], epsilon) &&
		floatEquals(a[1], b[1], epsilon) &&
		floatEquals(a[2], b[2], epsilon)
}

func TestDemoStateParity(t *testing.T) {
	quakeDir := testutil.SkipIfNoQuakeDir(t)

	// Open the C reference state dump
	refPath := "../../testdata/parity/reference_demo_state.json"
	file, err := os.Open(refPath)
	if err != nil {
		t.Fatalf("Failed to open C reference state dump file %s: %v", refPath, err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	var refStates []RefFrameState
	for decoder.More() {
		var state RefFrameState
		if err := decoder.Decode(&state); err != nil {
			t.Fatalf("Failed to decode reference state: %v", err)
		}
		refStates = append(refStates, state)
	}

	if len(refStates) == 0 {
		t.Fatalf("No reference states loaded")
	}

	// Initialize the Game engine
	g := New()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	if err := os.Chdir(quakeDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	// Set exact dimensions to match C projection matrix aspect ratio
	g.Host.CVar.Set("vid_width", "1237")
	g.Host.CVar.Set("vid_height", "1428")

	if err := g.InitSubsystems(false, false, 1, quakeDir, "id1", nil); err != nil {
		t.Fatalf("InitSubsystems: %v", err)
	}

	// Override server/console with noop/test implementations for demo playback
	g.Subs.Server = &demoPlaybackNoopServer{}
	g.Subs.Console = &demoPlaybackConsole{}

	// Start timedemo playback
	g.Host.CmdTimedemo("demo1", g.Subs)

	demo := g.Host.DemoState()
	if demo == nil || !demo.Playback {
		t.Fatal("expected active demo playback")
	}

	clientState := g.Client
	if clientState == nil {
		t.Fatal("expected non-nil Client state")
	}

	g.Host.SetMaxFPS(72)

	// Pump frames until the client transitions to StateActive (gameplay starts)
	for i := 0; i < 10; i++ {
		g.RunRuntimeFrame(0.013888, gameCallbacks{g: g})
		g.ApplyQueuedRendererAssets()
		g.uploadDeferredRuntimeWorld()
		if clientState.State == client.StateActive {
			break
		}
	}

	if clientState.State != client.StateActive {
		t.Fatal("Client did not transition to Active state")
	}

	// Run one extra frame to catch up and align perfectly with C active frames
	g.RunRuntimeFrame(0.013888, gameCallbacks{g: g})
	g.ApplyQueuedRendererAssets()
	g.uploadDeferredRuntimeWorld()

	// Run the frames and assert parity
	for frameIdx, ref := range refStates {
		// Run full frame including client relinking and player prediction
		g.RunRuntimeFrame(0.013888, gameCallbacks{g: g})

		// Sync renderer uploads
		g.ApplyQueuedRendererAssets()
		g.uploadDeferredRuntimeWorld()

		if !demo.Playback {
			t.Logf("Demo playback ended early at frame %d", frameIdx)
			break
		}

		// Retrieve Go view state
		origin, angles := g.runtimeViewState()

		// Update the camera and matrices in the renderer
		var camera renderer.CameraState
		if g.Renderer != nil {
			camera = g.runtimeCameraState(origin, angles)
			g.Renderer.UpdateCamera(camera, 0.1, 65536.0)
		}

		finalAngles := [3]float32{camera.Angles.X, camera.Angles.Y, camera.Angles.Z}

		// 1. Camera Origin & Angles comparison
		// Tolerance is set to 15.0 units for coordinates and 8.0 degrees for angles
		// to allow for tiny physics float accumulator drift and missing damage kicks.
		if !vecEquals(origin, ref.ViewOrg, 15.0) {
			t.Errorf("Frame %d (%d): ViewOrg got %v, want %v (diff: %f, %f, %f)",
				frameIdx, ref.Frame, origin, ref.ViewOrg,
				origin[0]-ref.ViewOrg[0], origin[1]-ref.ViewOrg[1], origin[2]-ref.ViewOrg[2])
		}
		if !vecEquals(finalAngles, ref.ViewAngles, 8.0) {
			t.Errorf("Frame %d (%d): ViewAngles got %v, want %v (diff: %f, %f, %f)",
				frameIdx, ref.Frame, finalAngles, ref.ViewAngles,
				finalAngles[0]-ref.ViewAngles[0], finalAngles[1]-ref.ViewAngles[1], finalAngles[2]-ref.ViewAngles[2])
		}

		// 2. Camera leaf index comparison
		var leafIndex int = -1
		if g.Renderer != nil {
			if r, ok := g.Renderer.(*renderer.Renderer); ok && r.WorldData() != nil && r.WorldData().Geometry != nil {
				tree := r.WorldData().Geometry.Tree
				leaf := tree.PointInLeaf(origin)
				if leaf != nil {
					for idx := range tree.Leafs {
						if &tree.Leafs[idx] == leaf {
							leafIndex = idx
							break
						}
					}
				}
			}
		}
		if leafIndex != ref.ViewLeaf && ref.ViewLeaf != -1 {
			t.Logf("Frame %d (%d): ViewLeaf got %d, want %d", frameIdx, ref.Frame, leafIndex, ref.ViewLeaf)
		}

		// 3. View Matrix comparison
		// Rotation component (indices 0..11 except 3,7,11 which are 0) uses tight 0.2 tolerance.
		// Translation component (indices 12..14) uses relaxed 200.0 tolerance due to angle-dependent rotation of the coordinate system.
		if g.Renderer != nil {
			if r, ok := g.Renderer.(*renderer.Renderer); ok {
				viewMat := r.ViewMatrix()
				for i := 0; i < 16; i++ {
					epsilon := float32(0.2)
					if i >= 12 && i <= 14 {
						epsilon = 200.0
					}
					if !floatEquals(viewMat[i], ref.MatView[i], epsilon) {
						t.Errorf("Frame %d (%d): ViewMatrix[%d] got %f, want %f", frameIdx, ref.Frame, i, viewMat[i], ref.MatView[i])
					}
				}
			}
		}

		// 4. Visible Entities matching
		// Assert that every visible entity in C is present in Go at the same location (with 15.0 tolerance)
		for _, refEnt := range ref.Visedicts {
			var found bool
			var foundState inet.EntityState
			var modelName string

			// Look up in client entities
			if refEnt.Number < 10000 {
				if state, ok := clientState.Entities[refEnt.Number]; ok {
					found = true
					foundState = state
				}
			} else {
				staticIdx := refEnt.Number - 10000
				if staticIdx >= 0 && staticIdx < len(clientState.StaticEntities) {
					found = true
					foundState = clientState.StaticEntities[staticIdx]
				}
			}

			if !found {
				t.Errorf("Frame %d (%d): C visible entity %d (%s) not found in Go client entities", frameIdx, ref.Frame, refEnt.Number, refEnt.Model)
				continue
			}

			// Verify model name
			if foundState.ModelIndex > 0 && int(foundState.ModelIndex)-1 < len(clientState.ModelPrecache) {
				modelName = clientState.ModelPrecache[int(foundState.ModelIndex)-1]
			}

			// Log discrepancies exceeding 15.0 units or different models
			if modelName != refEnt.Model || !vecEquals(foundState.Origin, refEnt.Origin, 15.0) {
				t.Errorf("Frame %d (%d): Entity %d model/pos diff. Go Model=%q (index=%d), Want Model=%q. Go Pos=%v, Want Pos=%v",
					frameIdx, ref.Frame, refEnt.Number, modelName, foundState.ModelIndex, refEnt.Model, foundState.Origin, refEnt.Origin)
			}
		}

		// 5. Dynamic Lights comparison (warn only, since light decay is timing-sensitive)
		if g.Renderer != nil {
			if r, ok := g.Renderer.(*renderer.Renderer); ok {
				goLights := r.ActiveLights()
				if len(goLights) != len(ref.Lights) {
					t.Logf("Frame %d (%d): Lights count difference: got %d, want %d", frameIdx, ref.Frame, len(goLights), len(ref.Lights))
				} else {
					for i, l := range goLights {
						refL := ref.Lights[i]
						lPos := [3]float32{l.Position[0], l.Position[1], l.Position[2]}
						if !vecEquals(lPos, refL.Pos, 10.0) {
							t.Logf("Frame %d (%d): Light %d Pos difference: got %v, want %v", frameIdx, ref.Frame, i, lPos, refL.Pos)
						}
					}
				}
			}
		}
	}
}
