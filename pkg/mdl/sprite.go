package mdl

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	// SpriteIdent is the magic number for Quake sprite models ("IDSP" in little-endian).
	SpriteIdent = ('P' << 24) + ('S' << 16) + ('D' << 8) + 'I'
	// SpriteVersion is the supported sprite version.
	SpriteVersion = 1

	// Sprite frame types
	SpriteFrameSingle = 0
	SpriteFrameGroup  = 1
	SpriteFrameAngled = 2

	// IDSpriteHeader is a legacy alias for SpriteIdent.
	IDSpriteHeader = SpriteIdent
)

// SpriteHeader represents the on-disk sprite header.
type SpriteHeader struct {
	Ident          int32
	Version        int32
	Type           int32
	BoundingRadius float32
	Width          int32
	Height         int32
	NumFrames      int32
	BeamLength     float32
	SyncType       int32
}

type dSpriteFrame struct {
	Origin [2]int32
	Width  int32
	Height int32
}

type dSpriteGroup struct {
	NumFrames int32
}

type dSpriteInterval struct {
	Interval float32
}

type dSpriteFrameType struct {
	Type int32
}

// SpriteFrame represents a single sprite frame.
type SpriteFrame struct {
	Width, Height         int
	Up, Down, Left, Right float32
	SMax, TMax            float32 // Image might be padded
	Pixels                []byte
}

// SpriteGroup represents a group of animated sprite frames.
type SpriteGroup struct {
	NumFrames int
	Intervals []float32
	Frames    []*SpriteFrame
}

// SpriteFrameDesc describes a sprite frame with its type.
type SpriteFrameDesc struct {
	Type     int // SpriteFrameSingle, SpriteFrameGroup, SpriteFrameAngled
	FramePtr any // *SpriteFrame or *SpriteGroup
}

// Sprite represents an in-memory sprite model.
type Sprite struct {
	Type           int
	MaxWidth       int
	MaxHeight      int
	NumFrames      int
	SyncType       SyncType
	BeamLength     float32
	BoundingRadius float32
	Frames         []SpriteFrameDesc
}

// LoadSprite reads and parses a Quake .spr file from r.
func LoadSprite(r io.Reader) (*Sprite, error) {
	var header SpriteHeader
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("failed to read sprite header: %w", err)
	}

	if header.Ident != SpriteIdent {
		return nil, fmt.Errorf("invalid sprite ident: got 0x%08x, expected 0x%08x", header.Ident, SpriteIdent)
	}

	if header.Version != SpriteVersion {
		return nil, fmt.Errorf("unsupported sprite version: got %d, expected %d", header.Version, SpriteVersion)
	}

	if header.NumFrames < 1 {
		return nil, fmt.Errorf("invalid sprite frame count: %d", header.NumFrames)
	}

	if header.Width <= 0 || header.Height <= 0 {
		return nil, fmt.Errorf("invalid sprite dimensions: %dx%d", header.Width, header.Height)
	}

	sprite := &Sprite{
		Type:           int(header.Type),
		MaxWidth:       int(header.Width),
		MaxHeight:      int(header.Height),
		NumFrames:      int(header.NumFrames),
		SyncType:       SyncType(header.SyncType),
		BeamLength:     header.BeamLength,
		BoundingRadius: header.BoundingRadius,
		Frames:         make([]SpriteFrameDesc, int(header.NumFrames)),
	}

	for i := 0; i < sprite.NumFrames; i++ {
		var frameType dSpriteFrameType
		if err := binary.Read(r, binary.LittleEndian, &frameType); err != nil {
			return nil, fmt.Errorf("failed to read sprite frame type %d: %w", i, err)
		}

		sprite.Frames[i].Type = int(frameType.Type)

		switch frameType.Type {
		case SpriteFrameSingle:
			frame, err := loadSpriteFrame(r)
			if err != nil {
				return nil, fmt.Errorf("failed to load sprite frame %d: %w", i, err)
			}
			sprite.Frames[i].FramePtr = frame
		case SpriteFrameGroup, SpriteFrameAngled:
			group, err := loadSpriteGroup(r, int(frameType.Type))
			if err != nil {
				return nil, fmt.Errorf("failed to load sprite group %d: %w", i, err)
			}
			sprite.Frames[i].FramePtr = group
		default:
			return nil, fmt.Errorf("invalid sprite frame type %d at frame %d", frameType.Type, i)
		}
	}

	return sprite, nil
}

func loadSpriteFrame(r io.Reader) (*SpriteFrame, error) {
	var dframe dSpriteFrame
	if err := binary.Read(r, binary.LittleEndian, &dframe); err != nil {
		return nil, fmt.Errorf("failed to read frame header: %w", err)
	}

	if dframe.Width <= 0 || dframe.Height <= 0 {
		return nil, fmt.Errorf("invalid frame dimensions: %dx%d", dframe.Width, dframe.Height)
	}

	size := int64(dframe.Width) * int64(dframe.Height)
	if size <= 0 || size > int64(^uint(0)>>1) {
		return nil, fmt.Errorf("invalid frame pixel size: %d", size)
	}

	pixels := make([]byte, int(size))
	if _, err := io.ReadFull(r, pixels); err != nil {
		return nil, fmt.Errorf("failed to read frame pixel data: %w", err)
	}

	width := int(dframe.Width)
	height := int(dframe.Height)
	originX := int(dframe.Origin[0])
	originY := int(dframe.Origin[1])

	return &SpriteFrame{
		Width:  width,
		Height: height,
		Up:     float32(originY),
		Down:   float32(originY - height),
		Left:   float32(originX),
		Right:  float32(width + originX),
		SMax:   float32(width) / float32(padConditional(width)),
		TMax:   float32(height) / float32(padConditional(height)),
		Pixels: pixels,
	}, nil
}

func loadSpriteGroup(r io.Reader, frameType int) (*SpriteGroup, error) {
	var dgroup dSpriteGroup
	if err := binary.Read(r, binary.LittleEndian, &dgroup); err != nil {
		return nil, fmt.Errorf("failed to read group header: %w", err)
	}

	if dgroup.NumFrames <= 0 {
		return nil, fmt.Errorf("invalid group frame count: %d", dgroup.NumFrames)
	}

	if frameType == SpriteFrameAngled && dgroup.NumFrames != 8 {
		return nil, fmt.Errorf("angled group requires 8 frames, got %d", dgroup.NumFrames)
	}

	numFrames := int(dgroup.NumFrames)
	intervals := make([]float32, numFrames)
	for i := 0; i < numFrames; i++ {
		var interval dSpriteInterval
		if err := binary.Read(r, binary.LittleEndian, &interval); err != nil {
			return nil, fmt.Errorf("failed to read group interval %d: %w", i, err)
		}
		if interval.Interval <= 0 {
			return nil, fmt.Errorf("invalid group interval at index %d: %f", i, interval.Interval)
		}
		intervals[i] = interval.Interval
	}

	frames := make([]*SpriteFrame, numFrames)
	for i := 0; i < numFrames; i++ {
		frame, err := loadSpriteFrame(r)
		if err != nil {
			return nil, fmt.Errorf("failed to read group frame %d: %w", i, err)
		}
		frames[i] = frame
	}

	return &SpriteGroup{
		NumFrames: numFrames,
		Intervals: intervals,
		Frames:    frames,
	}, nil
}

func padConditional(size int) int {
	if size <= 0 {
		return 1
	}

	padded := 1
	for padded < size {
		padded <<= 1
	}

	return padded
}
