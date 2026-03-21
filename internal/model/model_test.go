package model

import (
	"bytes"
	"encoding/binary"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/testutil"
)

// TestLoadAliasModelFromPak0 tests the loading of Quake Alias Models (.mdl).
// It ensures player, monster, and item models are parsed correctly, including skins, triangles, and animation frames.
// Where in C: Mod_LoadAliasModel in model.c
func TestLoadAliasModelFromPak0(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	vfs := fs.NewFileSystem()
	err := vfs.Init(baseDir, "id1")
	testutil.AssertNoError(t, err)
	defer vfs.Close()

	mdlNames := []string{
		"progs/player.mdl",
		"progs/soldier.mdl",
		"progs/backpack.mdl",
	}

	for _, mdlName := range mdlNames {
		t.Run(mdlName, func(t *testing.T) {
			data, err := vfs.LoadFile(mdlName)
			testutil.AssertNoError(t, err)

			m, err := LoadAliasModel(bytes.NewReader(data))
			testutil.AssertNoError(t, err)

			if m.Type != ModAlias {
				t.Fatalf("model type = %v, want %v", m.Type, ModAlias)
			}
			if m.AliasHeader == nil {
				t.Fatal("alias header is nil")
			}

			a := m.AliasHeader
			if a.Ident != MDLIdent {
				t.Fatalf("ident = 0x%08x, want 0x%08x", a.Ident, MDLIdent)
			}
			if a.Version != MDLVersion {
				t.Fatalf("version = %d, want %d", a.Version, MDLVersion)
			}

			if a.NumSkins < 1 || a.NumSkins > MaxSkins {
				t.Fatalf("invalid skin count %d", a.NumSkins)
			}
			if a.NumVerts < 1 || a.NumVerts > MaxAliasVerts {
				t.Fatalf("invalid vertex count %d", a.NumVerts)
			}
			if a.NumTris < 1 {
				t.Fatalf("invalid triangle count %d", a.NumTris)
			}
			if a.NumFrames < 1 {
				t.Fatalf("invalid frame count %d", a.NumFrames)
			}
			if len(a.Frames) != a.NumFrames {
				t.Fatalf("frame descriptor count = %d, want %d", len(a.Frames), a.NumFrames)
			}
			if a.NumPoses < a.NumFrames {
				t.Fatalf("pose count = %d, expected at least %d", a.NumPoses, a.NumFrames)
			}
			if len(a.SkinDescs) != a.NumSkins {
				t.Fatalf("logical skin count = %d, want %d", len(a.SkinDescs), a.NumSkins)
			}
			if len(a.Skins) < a.NumSkins {
				t.Fatalf("flattened skin payload count = %d, want at least %d", len(a.Skins), a.NumSkins)
			}
			if len(a.STVerts) != a.NumVerts {
				t.Fatalf("st vert count = %d, want %d", len(a.STVerts), a.NumVerts)
			}
			if len(a.Triangles) != a.NumTris {
				t.Fatalf("triangle payload count = %d, want %d", len(a.Triangles), a.NumTris)
			}
			if len(a.Poses) != a.NumPoses {
				t.Fatalf("pose payload count = %d, want %d", len(a.Poses), a.NumPoses)
			}

			if m.NumFrames != a.NumFrames {
				t.Fatalf("model numframes = %d, want %d", m.NumFrames, a.NumFrames)
			}
			for axis := 0; axis < 3; axis++ {
				if m.Mins[axis] > m.Maxs[axis] {
					t.Fatalf("invalid axis %d bounds: min=%f max=%f", axis, m.Mins[axis], m.Maxs[axis])
				}
				if m.RMins[axis] > m.RMaxs[axis] {
					t.Fatalf("invalid axis %d rotational bounds: min=%f max=%f", axis, m.RMins[axis], m.RMaxs[axis])
				}
			}
		})
	}
}

func TestLoadAliasModelPreservesGroupedSkinFrames(t *testing.T) {
	var data bytes.Buffer
	write := func(value interface{}) {
		if err := binary.Write(&data, binary.LittleEndian, value); err != nil {
			t.Fatalf("binary.Write(%T): %v", value, err)
		}
	}

	write(MDLHeader{
		Ident:      MDLIdent,
		Version:    MDLVersion,
		NumSkins:   1,
		SkinWidth:  1,
		SkinHeight: 1,
		NumVerts:   3,
		NumTris:    1,
		NumFrames:  1,
	})
	write(DAliasSkinType{Type: int32(AliasSkinGroup)})
	write(DAliasSkinGroup{NumSkins: 2})
	write(DAliasSkinInterval{Interval: 0.2})
	write(DAliasSkinInterval{Interval: 0.4})
	if err := data.WriteByte(7); err != nil {
		t.Fatalf("WriteByte(group skin 0): %v", err)
	}
	if err := data.WriteByte(9); err != nil {
		t.Fatalf("WriteByte(group skin 1): %v", err)
	}
	write([3]STVert{})
	write(DTriangle{FacesFront: MDLFacesFront, VertIndex: [3]int32{0, 1, 2}})
	write(DAliasFrameType{Type: int32(AliasSingle)})
	write(DAliasFrame{})
	write([3]TriVertX{{V: [3]byte{0, 0, 0}}, {V: [3]byte{1, 0, 0}}, {V: [3]byte{0, 1, 0}}})

	m, err := LoadAliasModel(bytes.NewReader(data.Bytes()))
	testutil.AssertNoError(t, err)
	a := m.AliasHeader
	if a == nil {
		t.Fatal("alias header is nil")
	}
	if got := len(a.SkinDescs); got != 1 {
		t.Fatalf("skin desc count = %d, want 1", got)
	}
	if got := len(a.Skins); got != 2 {
		t.Fatalf("flattened skin count = %d, want 2", got)
	}
	desc := a.SkinDescs[0]
	if desc.FirstFrame != 0 || desc.NumFrames != 2 {
		t.Fatalf("skin desc = %+v, want first=0 frames=2", desc)
	}
	if len(desc.Intervals) != 2 || desc.Intervals[0] != 0.2 || desc.Intervals[1] != 0.4 {
		t.Fatalf("skin intervals = %v, want [0.2 0.4]", desc.Intervals)
	}
	if got := a.ResolveSkinFrame(0, 0.1); got != 0 {
		t.Fatalf("ResolveSkinFrame(0, 0.1) = %d, want 0", got)
	}
	if got := a.ResolveSkinFrame(0, 0.25); got != 1 {
		t.Fatalf("ResolveSkinFrame(0, 0.25) = %d, want 1", got)
	}
	if got := a.ResolveSkinFrame(0, 0.45); got != 0 {
		t.Fatalf("ResolveSkinFrame(0, 0.45) = %d, want 0", got)
	}
}

// TestLoadSpriteFromPak0 tests the loading of Sprite models (.spr).
// It ensures effects like explosions and fire are parsed correctly.
// Where in C: Mod_LoadSpriteModel in model.c
func TestLoadSpriteFromPak0(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	vfs := fs.NewFileSystem()
	err := vfs.Init(baseDir, "id1")
	testutil.AssertNoError(t, err)
	defer vfs.Close()

	files := vfs.ListFiles("progs/*.spr")
	if len(files) == 0 {
		t.Fatal("no sprite files found in pak0.pak")
	}

	var spritePath string
	for _, f := range files {
		if strings.HasSuffix(strings.ToLower(f), ".spr") {
			spritePath = f
			break
		}
	}

	if spritePath == "" {
		t.Fatal("no .spr file found in progs/ path")
	}

	data, err := vfs.LoadFile(spritePath)
	testutil.AssertNoError(t, err)

	sprite, err := LoadSprite(bytes.NewReader(data))
	testutil.AssertNoError(t, err)

	if sprite.NumFrames <= 0 {
		t.Fatalf("sprite has invalid frame count: %d", sprite.NumFrames)
	}
	if len(sprite.Frames) != sprite.NumFrames {
		t.Fatalf("sprite frame descriptor count mismatch: got %d, want %d", len(sprite.Frames), sprite.NumFrames)
	}
	if sprite.MaxWidth <= 0 || sprite.MaxHeight <= 0 {
		t.Fatalf("sprite has invalid max dimensions: %dx%d", sprite.MaxWidth, sprite.MaxHeight)
	}

	for i, frameDesc := range sprite.Frames {
		switch frameDesc.Type {
		case SpriteFrameSingle:
			frame, ok := frameDesc.FramePtr.(*MSpriteFrame)
			if !ok || frame == nil {
				t.Fatalf("frame %d expected *MSpriteFrame, got %T", i, frameDesc.FramePtr)
			}
			if frame.Width <= 0 || frame.Height <= 0 {
				t.Fatalf("frame %d has invalid dimensions: %dx%d", i, frame.Width, frame.Height)
			}
		case SpriteFrameGroup, SpriteFrameAngled:
			group, ok := frameDesc.FramePtr.(*MSpriteGroup)
			if !ok || group == nil {
				t.Fatalf("frame %d expected *MSpriteGroup, got %T", i, frameDesc.FramePtr)
			}
			if group.NumFrames <= 0 {
				t.Fatalf("group %d has invalid frame count: %d", i, group.NumFrames)
			}
			if len(group.Intervals) != group.NumFrames || len(group.Frames) != group.NumFrames {
				t.Fatalf("group %d shape mismatch: intervals=%d frames=%d numframes=%d", i, len(group.Intervals), len(group.Frames), group.NumFrames)
			}
			if frameDesc.Type == SpriteFrameAngled && group.NumFrames != 8 {
				t.Fatalf("angled group %d has %d frames, expected 8", i, group.NumFrames)
			}
			for j := 0; j < group.NumFrames; j++ {
				if group.Intervals[j] <= 0 {
					t.Fatalf("group %d interval %d is invalid: %f", i, j, group.Intervals[j])
				}
				if group.Frames[j] == nil {
					t.Fatalf("group %d frame %d is nil", i, j)
				}
			}
		default:
			t.Fatalf("frame %d has invalid frame type %d", i, frameDesc.Type)
		}
	}
}

// TestLoadSpriteRetainsSyncType tests sprite synchronization flags.
// It ensures sprites correctly follow their defined animation sync type (e.g., random start frame).
// Where in C: Mod_LoadSpriteModel in model.c
func TestLoadSpriteRetainsSyncType(t *testing.T) {
	var data bytes.Buffer
	write := func(value interface{}) {
		if err := binary.Write(&data, binary.LittleEndian, value); err != nil {
			t.Fatalf("binary.Write(%T): %v", value, err)
		}
	}

	write(int32(IDSpriteHeader))
	write(int32(SpriteVersion))
	write(int32(0))
	write(float32(1))
	write(int32(1))
	write(int32(1))
	write(int32(1))
	write(float32(0))
	write(int32(STRand))
	write(int32(SpriteFrameSingle))
	write([2]int32{0, 0})
	write(int32(1))
	write(int32(1))
	if err := data.WriteByte(7); err != nil {
		t.Fatalf("WriteByte(pixel): %v", err)
	}

	sprite, err := LoadSprite(bytes.NewReader(data.Bytes()))
	testutil.AssertNoError(t, err)
	if got := sprite.SyncType; got != STRand {
		t.Fatalf("sprite SyncType = %v, want %v", got, STRand)
	}
}
