package client

// SendMove / SendCmd / SignonReply tests split from client_test.go.

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	inet "github.com/darkliquid/ironwail-go/internal/net"
)

func TestSendMoveNotConnected(t *testing.T) {
	c := NewClient()
	c.State = StateDisconnected

	var sent []byte
	sendFunc := func(data []byte) error {
		sent = data
		return nil
	}

	err := c.SendCmd(sendFunc)
	if err != nil {
		t.Fatalf("SendCmd() error = %v, want nil", err)
	}
	if sent != nil {
		t.Fatalf("SendCmd sent data while disconnected")
	}
}

func TestSendStringCmdEncodesOpcodeAndPayload(t *testing.T) {
	c := NewClient()

	msg, err := c.SendStringCmd("prespawn")
	if err != nil {
		t.Fatalf("SendStringCmd error = %v", err)
	}
	if len(msg) < 2 {
		t.Fatalf("message too short: %d", len(msg))
	}
	if msg[0] != byte(inet.CLCStringCmd) {
		t.Fatalf("opcode = %d, want %d", msg[0], inet.CLCStringCmd)
	}
	if got := string(msg[1:]); got != "prespawn\x00" {
		t.Fatalf("payload = %q, want %q", got, "prespawn\\x00")
	}
}

func TestSendStringCmdPreservesNewlineOnlyPayload(t *testing.T) {
	c := NewClient()

	msg, err := c.SendStringCmd("\n")
	if err != nil {
		t.Fatalf("SendStringCmd error = %v", err)
	}
	if len(msg) < 2 {
		t.Fatalf("message too short: %d", len(msg))
	}
	if msg[0] != byte(inet.CLCStringCmd) {
		t.Fatalf("opcode = %d, want %d", msg[0], inet.CLCStringCmd)
	}
	if got := string(msg[1:]); got != "\n\x00" {
		t.Fatalf("payload = %q, want %q", got, "\n\x00")
	}
}

func TestHandleSignonReplyAcceptsSpawnArgs(t *testing.T) {
	c := NewClient()
	c.setState(StateConnected)
	c.Signon = 1

	if err := c.HandleSignonReply("spawn coop 1"); err != nil {
		t.Fatalf("HandleSignonReply with spawn args failed: %v", err)
	}
	if c.Signon != 2 {
		t.Fatalf("signon = %d, want 2", c.Signon)
	}
}

func TestSendMovePacking(t *testing.T) {
	c := NewClient()
	c.Protocol = inet.PROTOCOL_NETQUAKE
	c.Time = 1.234

	cmd := &UserCmd{
		ViewAngles: [3]float32{10.0, 45.0, 0.0},
		Forward:    200.0,
		Side:       50.0,
		Up:         0.0,
		Buttons:    3, // attack + jump
		Impulse:    7,
	}

	data, err := c.SendMove(cmd)
	if err != nil {
		t.Fatalf("SendMove() error = %v", err)
	}
	if len(data) == 0 {
		t.Fatal("SendMove() returned empty data")
	}

	// Parse the message back
	if data[0] != byte(inet.CLCMove) {
		t.Fatalf("first byte = %d, want %d (CLCMove)", data[0], inet.CLCMove)
	}

	// Verify we can parse the message
	buf := &bytes.Buffer{}
	buf.Write(data)

	// Read opcode
	opcode, _ := buf.ReadByte()
	if opcode != byte(inet.CLCMove) {
		t.Fatalf("opcode = %d, want %d", opcode, inet.CLCMove)
	}

	// Read time
	var timeVal float32
	_ = binary.Read(buf, binary.LittleEndian, &timeVal)
	if math.Abs(float64(timeVal-1.234)) > 0.001 {
		t.Fatalf("time = %f, want 1.234", timeVal)
	}

	// Read angles (8-bit for NetQuake)
	angle0, _ := buf.ReadByte()
	angle1, _ := buf.ReadByte()
	angle2, _ := buf.ReadByte()

	// Convert back to degrees
	gotAngle0 := float32(angle0) * 360.0 / 256.0
	gotAngle1 := float32(angle1) * 360.0 / 256.0
	gotAngle2 := float32(angle2) * 360.0 / 256.0

	if math.Abs(float64(gotAngle0-10.0)) > 2.0 {
		t.Fatalf("angle[0] = %f, want ~10.0", gotAngle0)
	}
	if math.Abs(float64(gotAngle1-45.0)) > 2.0 {
		t.Fatalf("angle[1] = %f, want ~45.0", gotAngle1)
	}
	if math.Abs(float64(gotAngle2)) > 2.0 {
		t.Fatalf("angle[2] = %f, want ~0.0", gotAngle2)
	}

	// Read movement
	var forward, side, up int16
	_ = binary.Read(buf, binary.LittleEndian, &forward)
	_ = binary.Read(buf, binary.LittleEndian, &side)
	_ = binary.Read(buf, binary.LittleEndian, &up)

	if forward != 200 {
		t.Fatalf("forward = %d, want 200", forward)
	}
	if side != 50 {
		t.Fatalf("side = %d, want 50", side)
	}
	if up != 0 {
		t.Fatalf("up = %d, want 0", up)
	}

	// Read buttons and impulse
	buttons, _ := buf.ReadByte()
	impulse, _ := buf.ReadByte()

	if buttons != 3 {
		t.Fatalf("buttons = %d, want 3", buttons)
	}
	if impulse != 7 {
		t.Fatalf("impulse = %d, want 7", impulse)
	}
}

func TestSendMoveWithShortAngles(t *testing.T) {
	c := NewClient()
	c.Protocol = inet.PROTOCOL_FITZQUAKE
	c.ProtocolFlags = inet.PRFL_SHORTANGLE
	c.Time = 2.5

	cmd := &UserCmd{
		ViewAngles: [3]float32{15.5, 180.25, 5.0},
		Forward:    150.0,
		Side:       -75.0,
		Up:         10.0,
		Buttons:    1, // attack only
		Impulse:    0,
	}

	data, err := c.SendMove(cmd)
	if err != nil {
		t.Fatalf("SendMove() error = %v", err)
	}

	// Verify message is longer (16-bit angles take more space)
	// NetQuake: 1 + 4 + 3 + 6 + 2 = 16 bytes
	// FitzQuake with short angles: 1 + 4 + 6 + 6 + 2 = 19 bytes
	if len(data) < 19 {
		t.Fatalf("message length = %d, want >= 19 for 16-bit angles", len(data))
	}
}

func TestSendCmdDuringSignOn(t *testing.T) {
	c := NewClient()
	c.Protocol = inet.PROTOCOL_NETQUAKE
	c.State = StateConnected
	c.Signon = 2 // Not yet complete
	c.MoveMessages = 2
	c.ViewAngles = [3]float32{0, 90, 0}

	var sentData []byte
	sendFunc := func(data []byte) error {
		sentData = make([]byte, len(data))
		copy(sentData, data)
		return nil
	}

	err := c.SendCmd(sendFunc)
	if err != nil {
		t.Fatalf("SendCmd() error = %v", err)
	}

	if len(sentData) == 0 {
		t.Fatal("SendCmd() did not send data during signon")
	}

	// Should send empty move (no movement values)
	// Parse and verify it's mostly zeros except angles
	buf := bytes.NewBuffer(sentData)
	_, _ = buf.ReadByte() // opcode
	var timeVal float32
	_ = binary.Read(buf, binary.LittleEndian, &timeVal)
	_, _ = buf.ReadByte() // angle 0
	_, _ = buf.ReadByte() // angle 1
	_, _ = buf.ReadByte() // angle 2

	var forward, side, up int16
	_ = binary.Read(buf, binary.LittleEndian, &forward)
	_ = binary.Read(buf, binary.LittleEndian, &side)
	_ = binary.Read(buf, binary.LittleEndian, &up)

	if forward != 0 || side != 0 || up != 0 {
		t.Fatalf("movement during signon = (%d,%d,%d), want (0,0,0)", forward, side, up)
	}
}

func TestSendCmdAfterSignOn(t *testing.T) {
	c := NewClient()
	c.Protocol = inet.PROTOCOL_NETQUAKE
	c.State = StateActive
	c.Signon = Signons // Complete
	c.MoveMessages = 2
	c.Time = 5.0

	// Simulate accumulated input
	c.PendingCmd = UserCmd{
		ViewAngles: [3]float32{5.0, 270.0, 0.0},
		Forward:    300.0,
		Side:       -100.0,
		Up:         0.0,
	}
	c.InputAttack.State = 3
	c.InputJump.State = 3
	c.InImpulse = 10

	var sentData []byte
	sendFunc := func(data []byte) error {
		sentData = make([]byte, len(data))
		copy(sentData, data)
		return nil
	}

	err := c.SendCmd(sendFunc)
	if err != nil {
		t.Fatalf("SendCmd() error = %v", err)
	}

	if len(sentData) == 0 {
		t.Fatal("SendCmd() did not send data after signon")
	}
	if c.CommandCount != 1 {
		t.Fatalf("CommandCount = %d, want 1 after sending one command", c.CommandCount)
	}

	// Verify real command was sent
	buf := bytes.NewBuffer(sentData)
	_, _ = buf.ReadByte() // opcode
	var timeVal float32
	_ = binary.Read(buf, binary.LittleEndian, &timeVal)
	_, _ = buf.ReadByte() // angles
	_, _ = buf.ReadByte()
	_, _ = buf.ReadByte()

	var forward, side, up int16
	_ = binary.Read(buf, binary.LittleEndian, &forward)
	_ = binary.Read(buf, binary.LittleEndian, &side)
	_ = binary.Read(buf, binary.LittleEndian, &up)

	if forward != 300 {
		t.Fatalf("forward = %d, want 300", forward)
	}
	if side != -100 {
		t.Fatalf("side = %d, want -100", side)
	}

	buttons, _ := buf.ReadByte()
	impulse, _ := buf.ReadByte()

	if buttons != 3 {
		t.Fatalf("buttons = %d, want 3", buttons)
	}
	if impulse != 10 {
		t.Fatalf("impulse = %d, want 10", impulse)
	}

	// Verify Cmd was updated
	if c.Cmd.Forward != 300 {
		t.Fatalf("c.Cmd.Forward = %f, want 300", c.Cmd.Forward)
	}
}

func TestSendCmdRateLimit(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.Signon = Signons
	c.MoveMessages = 0 // Should skip first 2

	sendCount := 0
	sendFunc := func(data []byte) error {
		sendCount++
		return nil
	}

	// First call - should skip (MoveMessages = 0)
	err := c.SendCmd(sendFunc)
	if err != nil {
		t.Fatalf("SendCmd(1) error = %v", err)
	}
	if sendCount != 0 {
		t.Fatalf("sent on first call, want skip")
	}
	if c.MoveMessages != 1 {
		t.Fatalf("MoveMessages = %d, want 1", c.MoveMessages)
	}

	// Second call - should skip (MoveMessages = 1)
	err = c.SendCmd(sendFunc)
	if err != nil {
		t.Fatalf("SendCmd(2) error = %v", err)
	}
	if sendCount != 0 {
		t.Fatalf("sent on second call, want skip")
	}
	if c.MoveMessages != 2 {
		t.Fatalf("MoveMessages = %d, want 2", c.MoveMessages)
	}

	// Third call - should send
	c.PendingCmd.Forward = 100
	err = c.SendCmd(sendFunc)
	if err != nil {
		t.Fatalf("SendCmd(3) error = %v", err)
	}
	if sendCount != 1 {
		t.Fatalf("sendCount = %d, want 1 after third call", sendCount)
	}
}

func TestBuildPendingMoveLatchesAndClearsOneShotInputs(t *testing.T) {
	c := NewClient()
	c.PendingCmd = UserCmd{
		ViewAngles: [3]float32{1, 2, 3},
		Forward:    120,
		Side:       -40,
		Up:         8,
		Msec:       16,
	}
	c.InputAttack.State = 3 // down + impulse down
	c.InputJump.State = 3   // down + impulse down
	c.InImpulse = 7

	cmd1 := c.BuildPendingMove()
	if cmd1.Buttons != 3 {
		t.Fatalf("first cmd buttons = %d, want 3", cmd1.Buttons)
	}
	if cmd1.Impulse != 7 {
		t.Fatalf("first cmd impulse = %d, want 7", cmd1.Impulse)
	}
	if c.InImpulse != 0 {
		t.Fatalf("InImpulse after first build = %d, want 0", c.InImpulse)
	}

	cmd2 := c.BuildPendingMove()
	if cmd2.Buttons != 3 {
		t.Fatalf("second cmd buttons = %d, want 3 while keys remain held", cmd2.Buttons)
	}
	if cmd2.Impulse != 0 {
		t.Fatalf("second cmd impulse = %d, want 0", cmd2.Impulse)
	}
}

func TestSendMoveNilClient(t *testing.T) {
	var c *Client
	data, err := c.SendMove(&UserCmd{})
	if err != nil {
		t.Fatalf("SendMove(nil client) error = %v, want nil", err)
	}
	if data != nil {
		t.Fatalf("SendMove(nil client) returned data")
	}
}

func TestSendCmdNetworkError(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.Signon = Signons
	c.MoveMessages = 2

	expectedErr := fmt.Errorf("network down")
	sendFunc := func(data []byte) error {
		return expectedErr
	}

	err := c.SendCmd(sendFunc)
	if err == nil {
		t.Fatal("SendCmd() error = nil, want error")
	}
	// Should return the error but not panic
}

// Integration test: SendCmd with mock network socket
func TestSendCmdIntegrationWithSocket(t *testing.T) {
	// Setup client in active state
	c := NewClient()
	c.State = StateActive
	c.Signon = Signons
	c.Protocol = inet.PROTOCOL_NETQUAKE
	c.MoveMessages = 2
	c.Time = 3.5

	// Setup input command
	c.PendingCmd = UserCmd{
		ViewAngles: [3]float32{10.0, 90.0, 0.0},
		Forward:    250.0,
		Side:       -50.0,
		Up:         0.0,
	}
	c.InputAttack.State = 3
	c.InImpulse = 5

	// Mock network send function that captures data
	var sentMessages [][]byte
	sendFunc := func(data []byte) error {
		captured := make([]byte, len(data))
		copy(captured, data)
		sentMessages = append(sentMessages, captured)
		return nil
	}

	// Send command
	err := c.SendCmd(sendFunc)
	if err != nil {
		t.Fatalf("SendCmd() error = %v", err)
	}

	// Verify exactly one message was sent
	if len(sentMessages) != 1 {
		t.Fatalf("sent %d messages, want 1", len(sentMessages))
	}

	// Parse the message
	msg := sentMessages[0]
	if len(msg) < 16 {
		t.Fatalf("message too short: %d bytes", len(msg))
	}

	// Verify it's a CLCMove message
	if msg[0] != byte(inet.CLCMove) {
		t.Fatalf("message type = %d, want %d (CLCMove)", msg[0], inet.CLCMove)
	}

	// Parse time
	timeBytes := msg[1:5]
	timeBits := binary.LittleEndian.Uint32(timeBytes)
	timeVal := math.Float32frombits(timeBits)
	if math.Abs(float64(timeVal-3.5)) > 0.01 {
		t.Fatalf("time = %f, want 3.5", timeVal)
	}

	// Parse angles (8-bit for NetQuake)
	angle0 := float32(msg[5]) * 360.0 / 256.0
	angle1 := float32(msg[6]) * 360.0 / 256.0
	_ = msg[7] // angle2 (roll), not checked in this test

	if math.Abs(float64(angle0-10.0)) > 2.0 {
		t.Errorf("angle0 = %f, want ~10.0", angle0)
	}
	if math.Abs(float64(angle1-90.0)) > 2.0 {
		t.Errorf("angle1 = %f, want ~90.0", angle1)
	}

	// Parse movement
	forward := int16(binary.LittleEndian.Uint16(msg[8:10]))
	side := int16(binary.LittleEndian.Uint16(msg[10:12]))
	up := int16(binary.LittleEndian.Uint16(msg[12:14]))

	if forward != 250 {
		t.Errorf("forward = %d, want 250", forward)
	}
	if side != -50 {
		t.Errorf("side = %d, want -50", side)
	}
	if up != 0 {
		t.Errorf("up = %d, want 0", up)
	}

	// Parse buttons and impulse
	buttons := msg[14]
	impulse := msg[15]

	if buttons != 1 {
		t.Errorf("buttons = %d, want 1", buttons)
	}
	if impulse != 5 {
		t.Errorf("impulse = %d, want 5", impulse)
	}

	// Verify client command was updated
	if c.Cmd.Forward != 250.0 {
		t.Errorf("client Cmd not updated")
	}
}

func TestAccumulateCmdSetsPerCommandMsec(t *testing.T) {
	c := NewClient()
	c.State = StateActive
	c.Signon = Signons

	c.AccumulateCmd(0.016)
	if c.PendingCmd.Msec != 16 {
		t.Fatalf("PendingCmd.Msec = %d, want 16", c.PendingCmd.Msec)
	}

	c.AccumulateCmd(1.0)
	if c.PendingCmd.Msec != 255 {
		t.Fatalf("PendingCmd.Msec clamp = %d, want 255", c.PendingCmd.Msec)
	}
	if c.CommandCount != 0 {
		t.Fatalf("CommandCount = %d, want 0 until a command is actually sent", c.CommandCount)
	}
}

func TestBaseMoveReturnsEmptyCommandBeforeSignonComplete(t *testing.T) {
	c := NewClient()
	c.Signon = Signons - 1
	c.InputForward.State = 1
	c.InputRight.State = 1
	c.ForwardSpeed = 200
	c.SideSpeed = 350

	var cmd UserCmd
	c.BaseMove(&cmd)
	if cmd.Forward != 0 || cmd.Side != 0 || cmd.Up != 0 {
		t.Fatalf("cmd = %+v, want zero movement before signon", cmd)
	}
}

func TestAdjustAnglesSkipsIntermissionCutsceneWithActiveViewEntity(t *testing.T) {
	c := NewClient()
	c.FixAngle = true
	c.Intermission = 1
	c.ViewEntity = 1
	c.ViewAngles = [3]float32{10, 20, 0}
	c.InputRight.State = 1

	c.AdjustAngles(0.1)

	if got := c.ViewAngles; got != [3]float32{10, 20, 0} {
		t.Fatalf("ViewAngles after intermission AdjustAngles = %v, want unchanged", got)
	}
}
