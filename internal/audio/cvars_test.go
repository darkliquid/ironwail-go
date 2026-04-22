package audio

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

func TestRegisterCVarsUsesCanonicalFilterQualityDefault(t *testing.T) {
	cv := cvar.NewCVarSystem()
	RegisterCVars(cv)

	filterQ := cv.Get("snd_filterquality")
	if filterQ == nil {
		t.Fatal("snd_filterquality not registered")
	}
	if got := filterQ.DefaultValue; got != "5" {
		t.Fatalf("snd_filterquality default = %q, want %q", got, "5")
	}
}

func TestUpdateFromCVarsAppliesFilterQuality(t *testing.T) {
	cv := cvar.NewCVarSystem()
	RegisterCVars(cv)

	tests := []struct {
		name          string
		filterQuality string
		wantQuality   int
	}{
		{name: "valid quality", filterQuality: "3", wantQuality: 3},
		{name: "out of range falls back", filterQuality: "99", wantQuality: 5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mixer := NewMixer()
			sys := &System{
				initialized: true,
				mixer:       mixer,
			}

			cv.Set("volume", "0.4")
			cv.Set("snd_filterquality", tc.filterQuality)

			sys.UpdateFromCVars(cv)

			if got := mixer.Volume(); got != 0.4 {
				t.Fatalf("volume = %v, want 0.4", got)
			}
			if got := mixer.filterQuality; got != tc.wantQuality {
				t.Fatalf("filter quality = %d, want %d", got, tc.wantQuality)
			}
		})
	}
}
