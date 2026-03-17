package host

import (
"bytes"
"testing"

cl "github.com/ironwail/ironwail-go/internal/client"
"github.com/ironwail/ironwail-go/internal/fs"
"github.com/ironwail/ironwail-go/internal/qc"
"github.com/ironwail/ironwail-go/internal/server"
"github.com/ironwail/ironwail-go/internal/testutil"
)

func TestE2ELoopbackMovement(t *testing.T) {
quakeDir := testutil.SkipIfNoQuakeDir(t)

h := NewHost()
fileSys := fs.NewFileSystem()
srv := server.NewServer()
subs := &Subsystems{
Files:   fileSys,
Console: &mockConsole{},
Server:  srv,
}
SetupLoopbackClientServer(subs, srv)

if err := h.Init(&InitParams{
BaseDir:    quakeDir,
GameDir:    "id1",
MaxClients: 1,
}, subs); err != nil {
t.Fatalf("Init: %v", err)
}
defer fileSys.Close()

progsData, err := fileSys.LoadFile("progs.dat")
if err != nil {
t.Fatalf("LoadFile(progs.dat): %v", err)
}
if err := srv.QCVM.LoadProgs(bytes.NewReader(progsData)); err != nil {
t.Fatalf("LoadProgs: %v", err)
}
qc.RegisterBuiltins(srv.QCVM)

if err := h.CmdMap("start", subs); err != nil {
t.Fatalf("CmdMap(start): %v", err)
}

clientState := LoopbackClientState(subs)
if clientState == nil {
t.Fatal("no client state")
}

t.Logf("Client: State=%v Signon=%v ViewEntity=%d", clientState.State, clientState.Signon, clientState.ViewEntity)

// Check if the client received the player entity
if ent, ok := clientState.Entities[clientState.ViewEntity]; ok {
t.Logf("Client ViewEntity %d: Origin=%v ModelIndex=%v", clientState.ViewEntity, ent.Origin, ent.ModelIndex)
} else {
t.Logf("Client ViewEntity %d: NOT found in Entities (total: %d)", clientState.ViewEntity, len(clientState.Entities))
for k, v := range clientState.Entities {
t.Logf("  Entity %d: Origin=%v Model=%v", k, v.Origin, v.ModelIndex)
}
}

// Check server entity
serverEnt := srv.Static.Clients[0].Edict
t.Logf("Server entity: Origin=%v MoveType=%v Health=%v Velocity=%v Flags=%v",
serverEnt.Vars.Origin, serverEnt.Vars.MoveType, serverEnt.Vars.Health,
serverEnt.Vars.Velocity, uint32(serverEnt.Vars.Flags))

// Check view angles and MViewAngles
t.Logf("Client MViewAngles: [0]=%v [1]=%v ViewAngles=%v",
clientState.MViewAngles[0], clientState.MViewAngles[1], clientState.ViewAngles)

// Get loopback client for sending
lc := subs.Client.(*localLoopbackClient)

// Set up forward input
clientState.ViewAngles = [3]float32{0, 0, 0}
clientState.ForwardSpeed = 400
clientState.SideSpeed = 350
clientState.InputForward.State = 1 // key held
clientState.MoveSpeedKey = 1

// Simulate frame: AccumulateCmd → SendCommand → ServerFrame → ReadFromServer
clientState.AccumulateCmd(0.1)
t.Logf("PendingCmd: Forward=%v Side=%v Up=%v ViewAngles=%v",
clientState.PendingCmd.Forward, clientState.PendingCmd.Side, clientState.PendingCmd.Up, clientState.PendingCmd.ViewAngles)

// Send to server
lc.cmdReady = true
clientState.State = cl.StateActive
clientState.Signon = cl.Signons
if err := lc.SendCommand(); err != nil {
t.Fatalf("SendCommand: %v", err)
}

// Check what the server received
t.Logf("Server LastCmd: Forward=%v Side=%v ViewAngles=%v",
srv.Static.Clients[0].LastCmd.ForwardMove, srv.Static.Clients[0].LastCmd.SideMove,
srv.Static.Clients[0].LastCmd.ViewAngles)

// Run server frame
srv.Frame(0.1)
t.Logf("Server entity after Frame: Origin=%v Velocity=%v",
serverEnt.Vars.Origin, serverEnt.Vars.Velocity)

// Read entity update from server
if err := lc.ReadFromServer(); err != nil {
t.Fatalf("ReadFromServer: %v", err)
}

// Check client entity after reading update
if ent, ok := clientState.Entities[clientState.ViewEntity]; ok {
t.Logf("Client ViewEntity after frame: Origin=%v", ent.Origin)
} else {
t.Log("Client ViewEntity STILL not found after frame")
}

// Check velocity received by client
t.Logf("Client Velocity after frame: %v", clientState.Velocity)
}
