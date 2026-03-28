package server

import (
	"bytes"
	"testing"

	inet "github.com/darkliquid/ironwail-go/internal/net"
)

func TestAddSignonBuffer(t *testing.T) {
	s := &Server{}

	// First buffer should succeed.
	if err := s.AddSignonBuffer(); err != nil {
		t.Fatalf("AddSignonBuffer: %v", err)
	}
	if len(s.SignonBuffers) != 1 {
		t.Fatalf("got %d buffers, want 1", len(s.SignonBuffers))
	}
	if s.Signon == nil {
		t.Fatal("Signon is nil after AddSignonBuffer")
	}
	if s.Signon != s.SignonBuffers[0] {
		t.Fatal("Signon does not point to last buffer")
	}
	if len(s.Signon.Data) != SignonSize {
		t.Fatalf("buffer size = %d, want %d", len(s.Signon.Data), SignonSize)
	}
}

func TestAddSignonBufferOverflow(t *testing.T) {
	s := &Server{}

	for i := 0; i < MaxSignonBuffers; i++ {
		if err := s.AddSignonBuffer(); err != nil {
			t.Fatalf("AddSignonBuffer %d: %v", i, err)
		}
	}

	// One more should fail.
	if err := s.AddSignonBuffer(); err == nil {
		t.Fatal("expected overflow error, got nil")
	}
}

func TestReserveSignonSpace_NilSignon(t *testing.T) {
	s := &Server{}

	// With nil signon, ReserveSignonSpace should allocate.
	if err := s.ReserveSignonSpace(10); err != nil {
		t.Fatalf("ReserveSignonSpace: %v", err)
	}
	if s.Signon == nil {
		t.Fatal("Signon still nil after ReserveSignonSpace")
	}
}

func TestReserveSignonSpace_SplitsBuffer(t *testing.T) {
	s := &Server{}
	if err := s.AddSignonBuffer(); err != nil {
		t.Fatal(err)
	}

	// Fill the buffer almost completely.
	filler := make([]byte, SignonSize-10)
	s.Signon.Write(filler)
	first := s.Signon

	// Reserve more space than remaining — should allocate new buffer.
	if err := s.ReserveSignonSpace(20); err != nil {
		t.Fatalf("ReserveSignonSpace: %v", err)
	}
	if s.Signon == first {
		t.Fatal("expected new buffer, got same one")
	}
	if len(s.SignonBuffers) != 2 {
		t.Fatalf("got %d buffers, want 2", len(s.SignonBuffers))
	}
}

func TestReserveSignonSpace_FitsInCurrent(t *testing.T) {
	s := &Server{}
	if err := s.AddSignonBuffer(); err != nil {
		t.Fatal(err)
	}
	first := s.Signon

	// Small reservation should not allocate a new buffer.
	if err := s.ReserveSignonSpace(10); err != nil {
		t.Fatalf("ReserveSignonSpace: %v", err)
	}
	if s.Signon != first {
		t.Fatal("unexpected new buffer for small reservation")
	}
	if len(s.SignonBuffers) != 1 {
		t.Fatalf("got %d buffers, want 1", len(s.SignonBuffers))
	}
}

func TestWriteSignonByte(t *testing.T) {
	s := &Server{}
	if err := s.WriteSignonByte(0x42); err != nil {
		t.Fatalf("WriteSignonByte: %v", err)
	}
	if s.Signon.Len() != 1 {
		t.Fatalf("signon len = %d, want 1", s.Signon.Len())
	}
	if s.Signon.Data[0] != 0x42 {
		t.Fatalf("signon data = 0x%02x, want 0x42", s.Signon.Data[0])
	}
}

func TestWriteSignonString(t *testing.T) {
	s := &Server{}
	if err := s.WriteSignonString("hello"); err != nil {
		t.Fatalf("WriteSignonString: %v", err)
	}
	// "hello" + null terminator = 6 bytes.
	if s.Signon.Len() != 6 {
		t.Fatalf("signon len = %d, want 6", s.Signon.Len())
	}
}

func TestSendSignonBuffers(t *testing.T) {
	s := &Server{}
	if err := s.AddSignonBuffer(); err != nil {
		t.Fatal(err)
	}
	s.Signon.WriteByte(0xAA)
	s.Signon.WriteByte(0xBB)

	// Add a second buffer.
	if err := s.AddSignonBuffer(); err != nil {
		t.Fatal(err)
	}
	s.Signon.WriteByte(0xCC)

	client := &Client{
		Message: NewMessageBuffer(MaxDatagram),
	}

	s.SendSignonBuffers(client)

	if client.Message.Len() != 3 {
		t.Fatalf("client message len = %d, want 3", client.Message.Len())
	}
	if client.Message.Data[0] != 0xAA || client.Message.Data[1] != 0xBB || client.Message.Data[2] != 0xCC {
		t.Fatalf("client message data mismatch: %v", client.Message.Data[:3])
	}
}

func TestBuildSignonBuffers_Empty(t *testing.T) {
	s := &Server{}

	if err := s.buildSignonBuffers(); err != nil {
		t.Fatalf("buildSignonBuffers: %v", err)
	}
	if len(s.SignonBuffers) < 1 {
		t.Fatal("expected at least one signon buffer")
	}
}

func TestWriteSignonData(t *testing.T) {
	s := &Server{}
	data := []byte{0x01, 0x02, 0x03, 0x04}
	if err := s.WriteSignonData(data); err != nil {
		t.Fatalf("WriteSignonData: %v", err)
	}
	if s.Signon.Len() != 4 {
		t.Fatalf("signon len = %d, want 4", s.Signon.Len())
	}
}

func TestWriteSignonShort(t *testing.T) {
	s := &Server{}
	if err := s.WriteSignonShort(0x1234); err != nil {
		t.Fatalf("WriteSignonShort: %v", err)
	}
	if s.Signon.Len() != 2 {
		t.Fatalf("signon len = %d, want 2", s.Signon.Len())
	}
}

func TestWriteSignonLong(t *testing.T) {
	s := &Server{}
	if err := s.WriteSignonLong(0x12345678); err != nil {
		t.Fatalf("WriteSignonLong: %v", err)
	}
	if s.Signon.Len() != 4 {
		t.Fatalf("signon len = %d, want 4", s.Signon.Len())
	}
}

func TestWriteSignonFloat(t *testing.T) {
	s := &Server{}
	if err := s.WriteSignonFloat(3.14); err != nil {
		t.Fatalf("WriteSignonFloat: %v", err)
	}
	if s.Signon.Len() != 4 {
		t.Fatalf("signon len = %d, want 4", s.Signon.Len())
	}
}

func TestWriteSignonCoord(t *testing.T) {
	s := &Server{}
	if err := s.WriteSignonCoord(128.5); err != nil {
		t.Fatalf("WriteSignonCoord: %v", err)
	}
	// Default FitzQuake protocol writes coords as 16-bit fixed-point (2 bytes)
	if s.Signon.Len() != 2 {
		t.Fatalf("signon len = %d, want 2", s.Signon.Len())
	}
}

func TestMultipleBufferFill(t *testing.T) {
	s := &Server{}

	// Write enough data to span multiple buffers.
	chunk := make([]byte, SignonSize-100) // leave a little room
	for i := 0; i < 3; i++ {
		if err := s.WriteSignonData(chunk); err != nil {
			t.Fatalf("WriteSignonData iteration %d: %v", i, err)
		}
	}

	if len(s.SignonBuffers) != 3 {
		t.Fatalf("got %d buffers, want 3", len(s.SignonBuffers))
	}

	// Verify client receives all data.
	client := &Client{
		Message: NewMessageBuffer(SignonSize * 4),
	}
	s.SendSignonBuffers(client)

	expectedLen := len(chunk) * 3
	if client.Message.Len() != expectedLen {
		t.Fatalf("client message len = %d, want %d", client.Message.Len(), expectedLen)
	}
}

func TestGetClientLoopbackMessageStagesOversizedSignonBuffers(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.ConnectClient(0)
	client := s.Static.Clients[0]
	client.Loopback = true
	client.SendSignon = SignonPrespawn
	client.SignonIdx = 0
	client.Message.Clear()

	first := NewMessageBuffer(MaxDatagram)
	first.Write(bytes.Repeat([]byte{byte(inet.SVCNop)}, MaxDatagram-1))
	second := NewMessageBuffer(MaxDatagram)
	second.WriteByte(byte(inet.SVCNop))
	second.WriteByte(byte(inet.SVCNop))
	s.SignonBuffers = []*MessageBuffer{first, second}

	data := s.GetClientLoopbackMessage(0)
	if len(data) != MaxDatagram {
		t.Fatalf("first loopback message len = %d, want %d", len(data), MaxDatagram)
	}
	if client.SendSignon != SignonPrespawn {
		t.Fatalf("SendSignon after first chunk = %v, want %v", client.SendSignon, SignonPrespawn)
	}
	if client.SignonIdx != 1 {
		t.Fatalf("SignonIdx after first chunk = %d, want 1", client.SignonIdx)
	}

	data = s.GetClientLoopbackMessage(0)
	if len(data) != 5 {
		t.Fatalf("second loopback message len = %d, want 5", len(data))
	}
	if got := data[0]; got != byte(inet.SVCNop) {
		t.Fatalf("second loopback first byte = %d, want SVCNop", got)
	}
	if got := data[1]; got != byte(inet.SVCNop) {
		t.Fatalf("second loopback second byte = %d, want SVCNop", got)
	}
	if got := data[2]; got != byte(inet.SVCSignOnNum) {
		t.Fatalf("second loopback third byte = %d, want SVCSignOnNum", got)
	}
	if got := data[3]; got != 2 {
		t.Fatalf("second loopback signon = %d, want 2", got)
	}
	if got := data[4]; got != 0xff {
		t.Fatalf("second loopback terminator = %#x, want 0xff", got)
	}
	if client.SendSignon != SignonSignonBufs {
		t.Fatalf("SendSignon after final chunk = %v, want %v", client.SendSignon, SignonSignonBufs)
	}
}
