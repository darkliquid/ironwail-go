package main

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/qc/dap"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

func TestQCModDAPCommand(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// Run in background with dynamic port
	stopCh := make(chan struct{})
	go func() {
		runDAPServer("127.0.0.1:23499", stdout, stderr, stopCh)
	}()
	defer close(stopCh)

	var conn net.Conn
	var err error
	for i := 0; i < 100; i++ {
		time.Sleep(50 * time.Millisecond)
		conn, err = net.Dial("tcp", "127.0.0.1:23499")
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("Failed connecting to qcmod dap: %v (stderr: %s)", err, stderr.String())
	}
	defer conn.Close()

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
		t.Fatalf("qcmod DAP initialize failed: %v", err)
	}
}

func TestQCModTarget(t *testing.T) {
	w, err := newVMWorld(nil)
	if err != nil {
		t.Fatalf("newVMWorld failed: %v", err)
	}
	target := &qcmodTarget{world: w}

	var dTarget dap.Target = target
	if dTarget.VM() == nil {
		t.Fatal("expected non-nil VM")
	}
	if dTarget.EdictCount() <= 0 {
		t.Fatalf("expected EdictCount > 0, got %d", dTarget.EdictCount())
	}

	fields := dTarget.FieldNames()
	if _, ok := fields["classname"]; !ok {
		t.Error("expected classname field in FieldNames")
	}

	w.vm.SetEFloat(0, qc.EntFieldHealth, 100.0)
	if got := dTarget.GetEdictFloat(0, qc.EntFieldHealth); got != 100.0 {
		t.Fatalf("GetEdictFloat = %v, want 100.0", got)
	}

	sidx := w.vm.AllocString("info_player_start")
	w.vm.SetEString(0, qc.EntFieldClassName, int32(sidx))
	if got := dTarget.GetEdictString(0, qc.EntFieldClassName); got != "info_player_start" {
		t.Fatalf("GetEdictString = %q, want info_player_start", got)
	}
	if got := dTarget.GetEdictClassName(0); got != "info_player_start" {
		t.Fatalf("GetEdictClassName = %q, want info_player_start", got)
	}

	w.vm.SetEVector(0, qc.EntFieldOrigin, qtypes.Vec3{X: 10, Y: 20, Z: 30})
	if got := dTarget.GetEdictVector(0, qc.EntFieldOrigin); got != [3]float32{10, 20, 30} {
		t.Fatalf("GetEdictVector = %v, want [10, 20, 30]", got)
	}

	// Nil safety checks
	nilTarget := &qcmodTarget{}
	if nilTarget.VM() != nil {
		t.Error("expected nil VM for empty target")
	}
	if nilTarget.EdictCount() != 0 {
		t.Errorf("expected 0 EdictCount, got %d", nilTarget.EdictCount())
	}
	if nilTarget.GetEdictFloat(0, 0) != 0 {
		t.Error("expected 0 float for nil target")
	}
	if nilTarget.GetEdictString(0, 0) != "" {
		t.Error("expected empty string for nil target")
	}
	if nilTarget.GetEdictVector(0, 0) != [3]float32{} {
		t.Error("expected zero vector for nil target")
	}
}

func TestQCModDAPServerListenError(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	stopCh := make(chan struct{})
	close(stopCh)

	code := runDAPServer("invalid-address-99999:port", stdout, stderr, stopCh)
	if code != 1 {
		t.Fatalf("expected exit code 1 for invalid address, got %d", code)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("qcmod dap:")) {
		t.Fatalf("expected error message in stderr, got: %s", stderr.String())
	}
}
