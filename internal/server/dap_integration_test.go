package server

import (
	"net"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/qc/dap"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

func TestServerImplementsDAPTarget(t *testing.T) {
	var target dap.Target = &Server{}
	if target.EdictCount() != 0 {
		t.Fatalf("Expected 0 edicts, got %d", target.EdictCount())
	}
}

func TestServerStartDAPListener(t *testing.T) {
	s := newPhysicsTestServer()
	srv, err := s.StartDAPServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("StartDAPServer failed: %v", err)
	}
	defer func() { _ = srv.Close() }()

	if !strings.Contains(DAPServerStatus(), "listening on") {
		t.Fatalf("DAPServerStatus() = %q, expected listening message", DAPServerStatus())
	}

	conn, err := net.Dial("tcp", srv.Addr().String())
	if err != nil {
		t.Fatalf("Dial DAP server failed: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if err := dap.WriteMessage(conn, dap.Request{
		Message: dap.Message{Seq: 1, Type: "request"},
		Command: "initialize",
	}); err != nil {
		t.Fatal(err)
	}

	payload, err := dap.ReadMessage(conn)
	if err != nil {
		t.Fatal(err)
	}
	var resp dap.Response
	if err := dap.DecodeMessage(payload, &resp); err != nil || !resp.Success {
		t.Fatalf("Init failed: %v, resp=%+v", err, resp)
	}
}

func TestServerDAPTargetInspection(t *testing.T) {
	s := newPhysicsTestServer()
	if s.VM() == nil {
		t.Fatal("Expected non-nil VM()")
	}
	if s.EdictCount() != 1 {
		t.Fatalf("Expected EdictCount() == 1, got %d", s.EdictCount())
	}

	fn := s.FieldNames()
	if _, ok := fn["classname"]; !ok {
		t.Fatal("Expected classname in FieldNames")
	}

	cname := s.GetEdictClassName(0)
	if cname != "" { // freshly allocated edict 0 has no classname string yet
		t.Logf("Edict 0 classname: %q", cname)
	}

	// Set string in VM and check GetEdictString
	strIdx := s.QCVM.AllocString("test_entity")
	s.QCVM.SetEString(0, qc.EntFieldClassName, int32(strIdx))
	if got := s.GetEdictClassName(0); got != "test_entity" {
		t.Fatalf("GetEdictClassName(0) = %q, want 'test_entity'", got)
	}

	s.QCVM.SetEFloat(0, qc.EntFieldHealth, 100.0)
	if got := s.GetEdictFloat(0, qc.EntFieldHealth); got != 100.0 {
		t.Fatalf("GetEdictFloat(0, Health) = %v, want 100.0", got)
	}

	s.QCVM.SetEVector(0, qc.EntFieldOrigin, qtypes.Vec3{X: 1, Y: 2, Z: 3})
	if got := s.GetEdictVector(0, qc.EntFieldOrigin); got != [3]float32{1, 2, 3} {
		t.Fatalf("GetEdictVector(0, Origin) = %v, want [1, 2, 3]", got)
	}

	StopDAPServer()
	if DAPServerStatus() != "DAP debug server is inactive" {
		t.Fatalf("DAPServerStatus() = %q after stop, want inactive", DAPServerStatus())
	}
}
