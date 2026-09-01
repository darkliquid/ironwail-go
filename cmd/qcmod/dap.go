package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/qc/dap"
)

type qcmodTarget struct {
	world *vmWorld
}

func (t *qcmodTarget) VM() *qc.VM {
	if t.world == nil {
		return nil
	}
	return t.world.vm
}

func (t *qcmodTarget) EdictCount() int {
	if t.world == nil || t.world.vm == nil {
		return 0
	}
	if t.world.vm.NumEdicts > 0 {
		return t.world.vm.NumEdicts
	}
	return 1
}

func (t *qcmodTarget) GetEdictFloat(entNum, offset int) float32 {
	if t.world == nil || t.world.vm == nil {
		return 0
	}
	return t.world.vm.EFloat(entNum, offset)
}

func (t *qcmodTarget) GetEdictString(entNum, offset int) string {
	if t.world == nil || t.world.vm == nil {
		return ""
	}
	sidx := t.world.vm.EInt(entNum, offset)
	return t.world.vm.String(sidx)
}

func (t *qcmodTarget) GetEdictVector(entNum, offset int) [3]float32 {
	if t.world == nil || t.world.vm == nil {
		return [3]float32{}
	}
	v := t.world.vm.EVector(entNum, offset)
	return [3]float32{v.X, v.Y, v.Z}
}

func (t *qcmodTarget) GetEdictClassName(entNum int) string {
	return t.GetEdictString(entNum, qc.EntFieldClassName)
}

func (t *qcmodTarget) FieldNames() map[string]int {
	return map[string]int{
		"classname": qc.EntFieldClassName,
		"origin":    qc.EntFieldOrigin,
		"angles":    qc.EntFieldAngles,
		"velocity":  qc.EntFieldVelocity,
		"health":    qc.EntFieldHealth,
	}
}

func runDAP(args []string, stdout, stderr io.Writer) int {
	addr := "127.0.0.1:2345"
	if len(args) > 0 {
		addr = args[0]
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	stopCh := make(chan struct{})
	go func() {
		<-sigCh
		close(stopCh)
	}()

	return runDAPServer(addr, stdout, stderr, stopCh)
}

func runDAPServer(addr string, stdout, stderr io.Writer, stopCh <-chan struct{}) int {
	w, err := newVMWorld(nil)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod dap: %v\n", err)
		return 1
	}

	target := &qcmodTarget{world: w}
	srv, err := dap.NewServer(addr, target)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod dap: %v\n", err)
		return 1
	}
	defer srv.Close()

	_, _ = fmt.Fprintf(stdout, "qcmod DAP server listening on %s (press Ctrl+C to stop)\n", srv.Addr().String())

	<-stopCh
	return 0
}
