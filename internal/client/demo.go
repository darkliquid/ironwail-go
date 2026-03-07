package client

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math"
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

	playbackHostFrame int
}

// NewDemoState creates a new demo state with default values
func NewDemoState() *DemoState {
	return &DemoState{
		Speed:             1.0,
		BaseSpeed:         1.0,
		playbackHostFrame: -1,
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

func (d *DemoState) WriteInitialStateSnapshot(c *Client) error {
	if c == nil {
		return fmt.Errorf("initial demo snapshot requires client state")
	}
	if c.State == StateDisconnected || c.Signon == 0 {
		return fmt.Errorf("initial demo snapshot requires a connected client")
	}

	frames := [][]byte{
		buildInitialServerInfoFrame(c),
		buildDemoSignonFrame(2),
		buildInitialStateFrame(c),
	}
	for _, frame := range frames {
		if err := d.WriteDemoFrame(frame, c.ViewAngles); err != nil {
			return err
		}
	}
	return nil
}

func buildInitialServerInfoFrame(c *Client) []byte {
	maxClients := c.MaxClients
	if maxClients <= 0 {
		maxClients = 1
	}

	protocol := c.Protocol
	if protocol == 0 {
		protocol = inet.PROTOCOL_FITZQUAKE
	}

	msg := make([]byte, 0, 4096)
	msg = append(msg, byte(inet.SVCServerInfo))
	msg = appendLong(msg, protocol)
	if protocol == inet.PROTOCOL_RMQ {
		msg = appendLong(msg, int32(c.ProtocolFlags))
	}
	msg = append(msg, byte(maxClients))
	msg = append(msg, byte(c.GameType))
	msg = appendString(msg, c.LevelName)
	for _, model := range c.ModelPrecache {
		msg = appendString(msg, model)
	}
	msg = append(msg, 0)
	for _, sound := range c.SoundPrecache {
		msg = appendString(msg, sound)
	}
	msg = append(msg, 0)
	msg = append(msg, byte(inet.SVCCDTrack), byte(c.CDTrack), byte(c.LoopTrack))
	for _, ent := range c.StaticEntities {
		msg = appendSpawnStaticMessage(msg, ent)
	}
	for _, snd := range c.StaticSounds {
		msg = appendSpawnStaticSoundMessage(msg, snd)
	}
	msg = append(msg, byte(inet.SVCSetView))
	msg = appendShort(msg, int16(c.ViewEntity))
	msg = append(msg, byte(inet.SVCSignOnNum), 1, 0xff)
	return msg
}

func buildDemoSignonFrame(num byte) []byte {
	return []byte{byte(inet.SVCSignOnNum), num, 0xff}
}

func buildInitialStateFrame(c *Client) []byte {
	maxClients := c.MaxClients
	if maxClients <= 0 {
		maxClients = 1
	}

	msg := make([]byte, 0, 4096)
	for i := 0; i < maxClients; i++ {
		name := ""
		if c.PlayerNames != nil {
			name = c.PlayerNames[i]
		}
		msg = append(msg, byte(inet.SVCUpdateName), byte(i))
		msg = appendString(msg, name)

		frags := 0
		if c.Frags != nil {
			frags = c.Frags[i]
		}
		msg = append(msg, byte(inet.SVCUpdateFrags), byte(i))
		msg = appendShort(msg, int16(frags))

		colors := byte(0)
		if c.PlayerColors != nil {
			colors = c.PlayerColors[i]
		}
		msg = append(msg, byte(inet.SVCUpdateColors), byte(i), colors)
	}

	for i, style := range c.LightStyles {
		msg = append(msg, byte(inet.SVCLightStyle), byte(i))
		msg = appendString(msg, style.Map)
	}

	for i, stat := range c.Stats {
		msg = append(msg, byte(inet.SVCUpdateStat), byte(i))
		msg = appendLong(msg, int32(stat))
	}

	if c.SkyboxName != "" {
		msg = append(msg, byte(inet.SVCSkyBox))
		msg = appendString(msg, c.SkyboxName)
	}

	if c.FogDensity != 0 || c.FogColor != [3]byte{} || c.FogTime != 0 {
		density, color := c.CurrentFog()
		msg = append(msg, byte(inet.SVCFog))
		msg = append(msg,
			byte(math.Round(float64(density*255))),
			byte(math.Round(float64(color[0]*255))),
			byte(math.Round(float64(color[1]*255))),
			byte(math.Round(float64(color[2]*255))),
		)
		msg = appendFloat(msg, 0)
	}

	if c.Paused {
		msg = append(msg, byte(inet.SVCSetPause), 1)
	}

	msg = append(msg, byte(inet.SVCSetView))
	msg = appendShort(msg, int16(c.ViewEntity))
	msg = append(msg, byte(inet.SVCSignOnNum), 3, 0xff)
	return msg
}

func appendSpawnStaticMessage(dst []byte, ent inet.EntityState) []byte {
	extended := ent.ModelIndex > 255 || ent.Frame > 255 || ent.Alpha != inet.ENTALPHA_DEFAULT || (ent.Scale != 0 && ent.Scale != inet.ENTSCALE_DEFAULT)
	if extended {
		dst = append(dst, byte(inet.SVCSpawnStatic2))
		return appendEntityState(dst, ent, true, false, 0)
	}
	dst = append(dst, byte(inet.SVCSpawnStatic))
	return appendEntityState(dst, ent, false, false, 0)
}

func appendSpawnStaticSoundMessage(dst []byte, snd StaticSound) []byte {
	if snd.SoundIndex > 255 {
		dst = append(dst, byte(inet.SVCSpawnStaticSound2))
		for i := range snd.Origin {
			dst = appendFloat(dst, snd.Origin[i])
		}
		dst = appendShort(dst, int16(snd.SoundIndex))
		dst = append(dst, byte(snd.Volume), byte(snd.Attenuation*64))
		return dst
	}
	dst = append(dst, byte(inet.SVCSpawnStaticSound))
	for i := range snd.Origin {
		dst = appendFloat(dst, snd.Origin[i])
	}
	dst = append(dst, byte(snd.SoundIndex), byte(snd.Volume), byte(snd.Attenuation*64))
	return dst
}

func appendEntityState(dst []byte, ent inet.EntityState, extended bool, includeEntNum bool, entNum int) []byte {
	var bits byte
	if ent.ModelIndex > 255 {
		bits |= inet.BLARGEMODEL
	}
	if ent.Frame > 255 {
		bits |= inet.BLARGEFRAME
	}
	if ent.Alpha != inet.ENTALPHA_DEFAULT {
		bits |= inet.BALPHA
	}
	if ent.Scale != 0 && ent.Scale != inet.ENTSCALE_DEFAULT {
		bits |= inet.BSCALE
	}

	if extended {
		dst = append(dst, bits)
	}
	if includeEntNum {
		dst = appendShort(dst, int16(entNum))
	}
	if extended && bits&inet.BLARGEMODEL != 0 {
		dst = appendShort(dst, int16(ent.ModelIndex))
	} else {
		dst = append(dst, byte(ent.ModelIndex))
	}
	if extended && bits&inet.BLARGEFRAME != 0 {
		dst = appendShort(dst, int16(ent.Frame))
	} else {
		dst = append(dst, byte(ent.Frame))
	}
	dst = append(dst, ent.Colormap, ent.Skin)
	for i := range ent.Origin {
		dst = appendFloat(dst, ent.Origin[i])
	}
	for i := range ent.Angles {
		dst = append(dst, byte(ent.Angles[i]*256.0/360.0))
	}
	if extended && bits&inet.BALPHA != 0 {
		dst = append(dst, ent.Alpha)
	}
	if extended && bits&inet.BSCALE != 0 {
		dst = append(dst, ent.Scale)
	}
	return dst
}

func appendString(dst []byte, s string) []byte {
	dst = append(dst, s...)
	return append(dst, 0)
}

func appendShort(dst []byte, v int16) []byte {
	n := len(dst)
	dst = append(dst, 0, 0)
	binary.LittleEndian.PutUint16(dst[n:], uint16(v))
	return dst
}

func appendLong(dst []byte, v int32) []byte {
	n := len(dst)
	dst = append(dst, 0, 0, 0, 0)
	binary.LittleEndian.PutUint32(dst[n:], uint32(v))
	return dst
}

func appendFloat(dst []byte, v float32) []byte {
	n := len(dst)
	dst = append(dst, 0, 0, 0, 0)
	binary.LittleEndian.PutUint32(dst[n:], math.Float32bits(v))
	return dst
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
	d.playbackHostFrame = -1

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
	d.playbackHostFrame = -1

	return err
}

func (d *DemoState) ShouldReadFrame(hostFrame int) bool {
	if d == nil || !d.Playback {
		return false
	}
	if d.Paused || d.Speed == 0 {
		return false
	}
	if d.playbackHostFrame == hostFrame {
		return false
	}
	d.playbackHostFrame = hostFrame
	return true
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
