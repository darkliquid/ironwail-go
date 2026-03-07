package client

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"

	inet "github.com/ironwail/ironwail-go/internal/net"
)

type DemoFrame struct {
	FileOffset       int64
	Intermission     int
	ForceUnderwater  bool
	SerializedEvents int
}

type DemoState struct {
	File       *os.File
	Reader     *bufio.Reader
	Writer     *bufio.Writer
	Playback   bool
	Recording  bool
	Paused     bool
	Speed      float32
	BaseSpeed  float32
	TimeDemo   bool
	FrameIndex int
	CDTrack    int
	Filename   string

	Frames []DemoFrame
}

// NewDemoState creates a new demo state with default values
func NewDemoState() *DemoState {
	return &DemoState{
		Speed:     1.0,
		BaseSpeed: 1.0,
	}
}

// StartDemoRecording opens a demo file for recording
func (d *DemoState) StartDemoRecording(filename string, cdtrack int) error {
	if d.Recording {
		return fmt.Errorf("already recording a demo")
	}
	if d.Playback {
		return fmt.Errorf("cannot record during playback")
	}

	// Ensure demos directory exists
	demoDir := "demos"
	if err := os.MkdirAll(demoDir, 0755); err != nil {
		return fmt.Errorf("failed to create demos directory: %w", err)
	}

	// Add .dem extension if not present
	if filepath.Ext(filename) == "" {
		filename = filename + ".dem"
	}

	// Create full path
	fullPath := filepath.Join(demoDir, filename)

	// Open file for writing
	f, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create demo file %s: %w", fullPath, err)
	}

	d.File = f
	d.Writer = bufio.NewWriter(f)
	d.Recording = true
	d.CDTrack = cdtrack
	d.Filename = fullPath

	// Write CD track number header
	if _, err := fmt.Fprintf(d.Writer, "%d\n", cdtrack); err != nil {
		d.StopRecording()
		return fmt.Errorf("failed to write demo header: %w", err)
	}

	return nil
}

// StopRecording stops demo recording and closes the file
func (d *DemoState) StopRecording() error {
	if !d.Recording {
		return nil
	}

	var err error
	if d.Writer != nil {
		err = d.Writer.Flush()
	}
	if d.File != nil {
		if closeErr := d.File.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}

	d.File = nil
	d.Writer = nil
	d.Recording = false

	return err
}

// WriteDemoFrame writes a single demo frame (message with view angles)
func (d *DemoState) WriteDemoFrame(messageData []byte, viewAngles [3]float32) error {
	if !d.Recording || d.Writer == nil {
		return fmt.Errorf("not recording")
	}

	// Write message size (4 bytes, little endian)
	msgSize := int32(len(messageData))
	if err := binary.Write(d.Writer, binary.LittleEndian, msgSize); err != nil {
		return fmt.Errorf("failed to write message size: %w", err)
	}

	// Write view angles (3 floats, 12 bytes, little endian)
	for i := 0; i < 3; i++ {
		if err := binary.Write(d.Writer, binary.LittleEndian, viewAngles[i]); err != nil {
			return fmt.Errorf("failed to write view angle %d: %w", i, err)
		}
	}

	// Write message data
	if _, err := d.Writer.Write(messageData); err != nil {
		return fmt.Errorf("failed to write message data: %w", err)
	}

	return nil
}

func (d *DemoState) WriteDisconnectTrailer(viewAngles [3]float32) error {
	return d.WriteDemoFrame([]byte{inet.SVCDisconnect}, viewAngles)
}

// StartDemoPlayback opens a demo file for playback
func (d *DemoState) StartDemoPlayback(filename string) error {
	if d.Recording {
		return fmt.Errorf("cannot playback while recording")
	}
	if d.Playback {
		return fmt.Errorf("already playing back a demo")
	}

	// Add .dem extension if not present
	if filepath.Ext(filename) == "" {
		filename = filename + ".dem"
	}

	// Try demos directory first
	fullPath := filepath.Join("demos", filename)
	f, err := os.Open(fullPath)
	if err != nil {
		// Try current directory
		f, err = os.Open(filename)
		if err != nil {
			return fmt.Errorf("failed to open demo file %s: %w", filename, err)
		}
		fullPath = filename
	}

	d.File = f
	d.Reader = bufio.NewReader(f)
	d.Filename = fullPath

	// Read CD track header
	var cdtrack int
	if _, err := fmt.Fscanf(d.Reader, "%d\n", &cdtrack); err != nil {
		d.StopPlayback()
		return fmt.Errorf("failed to read demo header: %w", err)
	}
	d.CDTrack = cdtrack

	d.Playback = true
	d.Paused = false
	d.Speed = 1.0
	if d.BaseSpeed == 0 {
		d.BaseSpeed = 1.0
	}
	d.FrameIndex = 0
	d.Frames = nil

	return nil
}

// StopPlayback stops demo playback and closes the file
func (d *DemoState) StopPlayback() error {
	if !d.Playback {
		return nil
	}

	var err error
	if d.File != nil {
		err = d.File.Close()
	}

	d.File = nil
	d.Reader = nil
	d.Playback = false
	d.Paused = false
	d.Speed = 1.0
	d.Frames = nil
	d.FrameIndex = 0

	return err
}

// ReadDemoFrame reads one frame from the demo file
// Returns the message data, view angles, and any error
// Returns io.EOF when the demo ends
func (d *DemoState) ReadDemoFrame() (messageData []byte, viewAngles [3]float32, err error) {
	if !d.Playback || d.Reader == nil {
		return nil, viewAngles, fmt.Errorf("not playing back")
	}

	// Read message size
	var msgSize int32
	if err := binary.Read(d.Reader, binary.LittleEndian, &msgSize); err != nil {
		return nil, viewAngles, err
	}

	// Validate message size
	if msgSize < 0 || msgSize > 65536 { // MAX_MSGLEN
		return nil, viewAngles, fmt.Errorf("invalid message size: %d", msgSize)
	}

	// Read view angles
	for i := 0; i < 3; i++ {
		if err := binary.Read(d.Reader, binary.LittleEndian, &viewAngles[i]); err != nil {
			return nil, viewAngles, fmt.Errorf("failed to read view angle: %w", err)
		}
	}

	// Read message data
	messageData = make([]byte, msgSize)
	if _, err := io.ReadFull(d.Reader, messageData); err != nil {
		return nil, viewAngles, fmt.Errorf("failed to read message data: %w", err)
	}

	d.FrameIndex++

	return messageData, viewAngles, nil
}

// Client demo helper methods

func (c *Client) ClearSignons() {
	c.Signon = 0
}

func (c *Client) AdvanceTime(demo *DemoState, frametime float64) {
	c.OldTime = c.Time
	if demo != nil && demo.Playback {
		speed := float64(demo.Speed)
		if demo.Paused {
			speed = 0
		}
		c.Time += speed * frametime
		return
	}
	c.Time += frametime
}

func (c *Client) FinishDemoFrame() {
}

func (c *Client) StopPlayback(demo *DemoState) {
	if demo == nil || !demo.Playback {
		return
	}
	if demo.File != nil {
		_ = demo.File.Close()
	}
	demo.File = nil
	demo.Playback = false
	demo.Paused = false
	demo.Speed = 1
	demo.Frames = nil
	demo.FrameIndex = 0
	c.State = StateDisconnected
}
