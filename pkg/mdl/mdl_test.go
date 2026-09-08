package mdl_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/darkliquid/ironwail-go/pkg/mdl"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

func TestLoadSyntheticMDL(t *testing.T) {
	var buf bytes.Buffer
	write := func(val any) {
		if err := binary.Write(&buf, binary.LittleEndian, val); err != nil {
			t.Fatalf("binary.Write: %v", err)
		}
	}

	// MDL Header
	write(int32(mdl.Ident))
	write(int32(mdl.Version))
	write(types.Vec3{X: 1, Y: 2, Z: 3}) // Scale
	write(types.Vec3{X: 10, Y: 20, Z: 30}) // ScaleOrigin
	write(float32(50)) // BoundingRadius
	write(types.Vec3{X: 0, Y: 0, Z: 20}) // EyePosition
	write(int32(2)) // NumSkins (1 single, 1 group with 2 frames)
	write(int32(4)) // SkinWidth
	write(int32(4)) // SkinHeight
	write(int32(3)) // NumVerts
	write(int32(1)) // NumTris
	write(int32(2)) // NumFrames (1 single, 1 group with 2 poses)
	write(int32(mdl.SyncSTSync)) // SyncType
	write(int32(0)) // Flags
	write(float32(100)) // Size

	// Skin 0: Single (type = 0)
	write(int32(mdl.SkinSingle))
	buf.Write(make([]byte, 16)) // 4x4 pixels

	// Skin 1: Group (type = 1)
	write(int32(mdl.SkinGroup))
	write(int32(2)) // 2 grouped skins
	write(float32(0.2)) // interval 0
	write(float32(0.5)) // interval 1
	buf.Write(make([]byte, 16)) // grouped skin 0
	buf.Write(make([]byte, 16)) // grouped skin 1

	// 3 STVerts
	for i := 0; i < 3; i++ {
		write(int32(0)) // onseam
		write(int32(i * 2)) // s
		write(int32(i * 3)) // t
	}

	// 1 Triangle
	write(int32(mdl.FacesFront))
	write([3]int32{0, 1, 2})

	// Frame 0: Single
	write(int32(mdl.FrameSingle))
	write(mdl.TriVertX{V: [3]byte{0, 0, 0}, LightNormalIndex: 5}) // BBoxMin
	write(mdl.TriVertX{V: [3]byte{10, 10, 10}, LightNormalIndex: 5}) // BBoxMax
	var frame0Name [16]byte
	copy(frame0Name[:], "frame_walk")
	write(frame0Name)
	// 3 pose vertices (TriVertX)
	for i := 0; i < 3; i++ {
		write(mdl.TriVertX{V: [3]byte{byte(i * 2), byte(i * 3), byte(i * 4)}, LightNormalIndex: byte(i)})
	}

	// Frame 1: Group
	write(int32(mdl.FrameGroup))
	write(int32(2)) // 2 frames in group
	write(mdl.TriVertX{V: [3]byte{0, 0, 0}, LightNormalIndex: 5})    // group BBoxMin
	write(mdl.TriVertX{V: [3]byte{12, 12, 12}, LightNormalIndex: 5}) // group BBoxMax
	write(float32(0.1))                                              // interval 0
	write(float32(0.2))                                              // interval 1
	// Pose 0 of group
	write(mdl.TriVertX{V: [3]byte{0, 0, 0}, LightNormalIndex: 5})
	write(mdl.TriVertX{V: [3]byte{10, 10, 10}, LightNormalIndex: 5})
	write([16]byte{})
	for i := 0; i < 3; i++ {
		write(mdl.TriVertX{V: [3]byte{byte(i), byte(i), byte(i)}, LightNormalIndex: 0})
	}
	// Pose 1 of group
	write(mdl.TriVertX{V: [3]byte{1, 1, 1}, LightNormalIndex: 5})
	write(mdl.TriVertX{V: [3]byte{12, 12, 12}, LightNormalIndex: 5})
	write([16]byte{})
	for i := 0; i < 3; i++ {
		write(mdl.TriVertX{V: [3]byte{byte(i + 1), byte(i + 2), byte(i + 3)}, LightNormalIndex: 1})
	}

	file, err := mdl.Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("mdl.Load failed: %v", err)
	}

	if file.Ident != mdl.Ident {
		t.Errorf("Ident = 0x%08x, want 0x%08x", file.Ident, mdl.Ident)
	}
	if file.Version != mdl.Version {
		t.Errorf("Version = %d, want %d", file.Version, mdl.Version)
	}
	if file.NumSkins != 2 {
		t.Errorf("NumSkins = %d, want 2", file.NumSkins)
	}
	if len(file.Skins) != 3 { // 1 single + 2 grouped = 3 skin textures
		t.Errorf("len(Skins) = %d, want 3", len(file.Skins))
	}
	if len(file.SkinDescs) != 2 {
		t.Errorf("len(SkinDescs) = %d, want 2", len(file.SkinDescs))
	}
	if len(file.STVerts) != 3 {
		t.Errorf("len(STVerts) = %d, want 3", len(file.STVerts))
	}
	if len(file.Triangles) != 1 {
		t.Errorf("len(Triangles) = %d, want 1", len(file.Triangles))
	}
	if file.NumFrames != 2 {
		t.Errorf("NumFrames = %d, want 2", file.NumFrames)
	}
	if file.NumPoses != 3 { // 1 single + 2 in group = 3 poses
		t.Errorf("NumPoses = %d, want 3", file.NumPoses)
	}
	if len(file.Poses) != 3 {
		t.Errorf("len(Poses) = %d, want 3", len(file.Poses))
	}

	// Check skin frame resolution
	skin0 := file.ResolveSkinFrame(0, 0.0)
	if skin0 != 0 {
		t.Errorf("ResolveSkinFrame(0) = %d, want 0", skin0)
	}
	skin1Early := file.ResolveSkinFrame(1, 0.1)
	if skin1Early != 1 {
		t.Errorf("ResolveSkinFrame(1, 0.1) = %d, want 1", skin1Early)
	}
	skin1Late := file.ResolveSkinFrame(1, 0.3)
	if skin1Late != 2 {
		t.Errorf("ResolveSkinFrame(1, 0.3) = %d, want 2", skin1Late)
	}

	// Check vertex decoding
	vDec := mdl.DecodeVertex(mdl.TriVertX{V: [3]byte{2, 3, 4}}, file.Scale, file.ScaleOrigin)
	wantDec := types.Vec3{
		X: 10 + 2*1,
		Y: 20 + 3*2,
		Z: 30 + 4*3,
	}
	if vDec != wantDec {
		t.Errorf("DecodeVertex = %v, want %v", vDec, wantDec)
	}
}

func TestLoadSyntheticSprite(t *testing.T) {
	var buf bytes.Buffer
	write := func(val any) {
		if err := binary.Write(&buf, binary.LittleEndian, val); err != nil {
			t.Fatalf("binary.Write: %v", err)
		}
	}

	// Sprite Header
	write(int32(mdl.SpriteIdent))
	write(int32(mdl.SpriteVersion))
	write(int32(0)) // Type
	write(float32(20)) // BoundingRadius
	write(int32(16)) // Width
	write(int32(16)) // Height
	write(int32(2)) // NumFrames (1 single, 1 group)
	write(float32(0)) // BeamLength
	write(int32(mdl.SyncSTSync)) // SyncType

	// Frame 0: Single
	write(int32(mdl.SpriteFrameSingle))
	write([2]int32{-8, 8}) // Origin (x, y)
	write(int32(4)) // Width
	write(int32(4)) // Height
	buf.Write(make([]byte, 16)) // 4x4 pixels

	// Frame 1: Group
	write(int32(mdl.SpriteFrameGroup))
	write(int32(2)) // 2 frames in group
	write(float32(0.1)) // interval 0
	write(float32(0.2)) // interval 1
	// Group frame 0
	write([2]int32{-4, 4})
	write(int32(2))
	write(int32(2))
	buf.Write(make([]byte, 4)) // 2x2 pixels
	// Group frame 1
	write([2]int32{-2, 2})
	write(int32(2))
	write(int32(2))
	buf.Write(make([]byte, 4)) // 2x2 pixels

	sprite, err := mdl.LoadSprite(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("mdl.LoadSprite failed: %v", err)
	}

	if sprite.MaxWidth != 16 || sprite.MaxHeight != 16 {
		t.Errorf("sprite dimensions = %dx%d, want 16x16", sprite.MaxWidth, sprite.MaxHeight)
	}
	if sprite.NumFrames != 2 {
		t.Errorf("NumFrames = %d, want 2", sprite.NumFrames)
	}
	if len(sprite.Frames) != 2 {
		t.Errorf("len(Frames) = %d, want 2", len(sprite.Frames))
	}

	frame0, ok := sprite.Frames[0].FramePtr.(*mdl.SpriteFrame)
	if !ok || frame0 == nil {
		t.Fatalf("frame 0 FramePtr is not *mdl.SpriteFrame")
	}
	if frame0.Width != 4 || frame0.Height != 4 {
		t.Errorf("frame 0 dimensions = %dx%d, want 4x4", frame0.Width, frame0.Height)
	}

	group, ok := sprite.Frames[1].FramePtr.(*mdl.SpriteGroup)
	if !ok || group == nil {
		t.Fatalf("frame 1 FramePtr is not *mdl.SpriteGroup")
	}
	if group.NumFrames != 2 || len(group.Frames) != 2 {
		t.Errorf("group frame count = %d, want 2", group.NumFrames)
	}
}

func TestMDLInvalidHeader(t *testing.T) {
	// Bad magic
	badMagic := make([]byte, 64)
	if _, err := mdl.Load(bytes.NewReader(badMagic)); err == nil {
		t.Errorf("expected error loading bad magic MDL, got nil")
	}

	// Truncated header
	if _, err := mdl.Load(bytes.NewReader([]byte{1, 2, 3})); err == nil {
		t.Errorf("expected error loading truncated MDL, got nil")
	}
}

func TestSpriteInvalidHeader(t *testing.T) {
	badMagic := make([]byte, 36)
	if _, err := mdl.LoadSprite(bytes.NewReader(badMagic)); err == nil {
		t.Errorf("expected error loading bad magic Sprite, got nil")
	}
}

func TestNormalsTable(t *testing.T) {
	if len(mdl.NormalsTable) != 163 {
		t.Errorf("len(NormalsTable) = %d, want 163", len(mdl.NormalsTable))
	}
	n0 := mdl.GetNormal(0)
	if n0 != mdl.NormalsTable[0] {
		t.Errorf("GetNormal(0) = %v, want %v", n0, mdl.NormalsTable[0])
	}
	// Out of range returns z-up
	nOOB := mdl.GetNormal(200)
	wantZ := types.Vec3{X: 0, Y: 0, Z: 1}
	if nOOB != wantZ {
		t.Errorf("GetNormal(200) = %v, want %v", nOOB, wantZ)
	}
}
