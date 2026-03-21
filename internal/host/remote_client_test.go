package host

import (
	"reflect"
	"strings"
	"testing"

	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cvar"
	inet "github.com/ironwail/ironwail-go/internal/net"
)

var (
	testRemoteServerSignOnMsg1 = []byte{
		byte(inet.SVCServerInfo),
		0x9a, 0x02, 0x00, 0x00,
		0x04,
		0x00,
		'U', 'n', 'i', 't', ' ', 'T', 'e', 's', 't', ' ', 'M', 'a', 'p', 0,
		'm', 'a', 'p', 's', '/', 's', 't', 'a', 'r', 't', '.', 'b', 's', 'p', 0,
		'p', 'r', 'o', 'g', 's', '/', 'p', 'l', 'a', 'y', 'e', 'r', '.', 'm', 'd', 'l', 0,
		0,
		'm', 'i', 's', 'c', '/', 'n', 'u', 'l', 'l', '.', 'w', 'a', 'v', 0,
		0,
		byte(inet.SVCCDTrack), 0x02, 0x02,
		byte(inet.SVCSetView), 0x01, 0x00,
		byte(inet.SVCSignOnNum), 0x01,
		0xff,
	}
	testRemoteServerSignOnMsg2  = []byte{byte(inet.SVCSignOnNum), 0x02, 0xff}
	testRemoteServerSignOnMsg3  = []byte{byte(inet.SVCSignOnNum), 0x03, 0xff}
	testRemoteFirstServerUpdate = []byte{byte(inet.SVCTime), 0x00, 0x00, 0x00, 0x00, 0xff}
)

func TestCmdConnectRemoteAutoSignonCompletesWithoutManualCommands(t *testing.T) {
	h := NewHost()
	registerHostCVars()
	oldName := cvar.StringValue(clientNameCVar)
	oldColor := cvar.IntValue(clientColorCVar)
	cvar.Set(clientNameCVar, "Ranger")
	cvar.SetInt(clientColorCVar, 0x23)
	t.Cleanup(func() {
		cvar.Set(clientNameCVar, oldName)
		cvar.SetInt(clientColorCVar, oldColor)
	})

	console := &mockConsole{}
	subs := &Subsystems{
		Console: console,
		Client:  newLocalLoopbackClient(),
	}

	loop := inet.NewLoopback()
	if err := loop.Init(); err != nil {
		t.Fatalf("loopback init failed: %v", err)
	}
	clientSocket := loop.Connect()
	serverSocket := loop.CheckNewConnections()
	if serverSocket == nil {
		t.Fatal("server socket missing after loopback connect")
	}

	oldFactory := remoteClientFactory
	remoteClientFactory = func(address string) (Client, error) {
		rc := newRemoteDatagramClient(clientSocket)
		if err := rc.Init(); err != nil {
			return nil, err
		}
		return rc, nil
	}
	t.Cleanup(func() {
		remoteClientFactory = oldFactory
	})

	h.CmdConnect("remote.test:26000", subs)
	if h.ClientState() != caConnected {
		t.Fatalf("client state after connect = %v, want %v", h.ClientState(), caConnected)
	}

	var gotSignonCommands []string
	runFrame := func(msg []byte) {
		if sent := inet.SendUnreliableMessage(serverSocket, msg); sent != 1 {
			t.Fatalf("failed to send server message, code=%d", sent)
		}
		if err := subs.Client.ReadFromServer(); err != nil {
			t.Fatalf("ReadFromServer failed: %v", err)
		}
		if state := ActiveClientState(subs); state != nil {
			h.SetSignOns(state.Signon)
		}
		h.SetClientState(subs.Client.State())
		if err := subs.Client.SendCommand(); err != nil {
			t.Fatalf("SendCommand failed: %v", err)
		}
		for {
			msgType, payload := inet.GetMessage(serverSocket)
			if msgType == 0 {
				break
			}
			if msgType == 2 && len(payload) > 1 && payload[0] == byte(inet.CLCStringCmd) {
				gotSignonCommands = append(gotSignonCommands, strings.TrimSuffix(string(payload[1:]), "\x00"))
			}
		}
	}

	runFrame(testRemoteServerSignOnMsg1)
	runFrame(testRemoteServerSignOnMsg2)
	runFrame(testRemoteServerSignOnMsg3)
	runFrame(testRemoteFirstServerUpdate)

	if want := []string{"prespawn", "name \"Ranger\"", "color 2 3", "spawn", "begin"}; !reflect.DeepEqual(gotSignonCommands, want) {
		t.Fatalf("signon command sequence = %v, want %v", gotSignonCommands, want)
	}
	if got := h.ClientState(); got != caActive {
		t.Fatalf("host client state = %v, want %v", got, caActive)
	}
	if got := h.SignOns(); got != cl.Signons {
		t.Fatalf("host signons = %d, want %d", got, cl.Signons)
	}
	clientState := ActiveClientState(subs)
	if clientState == nil {
		t.Fatal("active client state is nil")
	}
	if got := clientState.State; got != cl.StateActive {
		t.Fatalf("client state = %v, want %v", got, cl.StateActive)
	}
	if got := clientState.Signon; got != cl.Signons {
		t.Fatalf("client signon = %d, want %d", got, cl.Signons)
	}
}

func TestRemoteClientSendCommandIncludesSpawnArgsInStageTwoReply(t *testing.T) {
	loop := inet.NewLoopback()
	if err := loop.Init(); err != nil {
		t.Fatalf("loopback init failed: %v", err)
	}
	clientSocket := loop.Connect()
	serverSocket := loop.CheckNewConnections()
	if serverSocket == nil {
		t.Fatal("server socket missing after loopback connect")
	}

	rc := newRemoteDatagramClient(clientSocket)
	rc.spawnArgs = "coop 1"
	if err := rc.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	rc.inner.Signon = 2

	if err := rc.SendCommand(); err != nil {
		t.Fatalf("SendCommand failed: %v", err)
	}

	var got []string
	for {
		msgType, payload := inet.GetMessage(serverSocket)
		if msgType == 0 {
			break
		}
		if msgType == 2 && len(payload) > 1 && payload[0] == byte(inet.CLCStringCmd) {
			got = append(got, strings.TrimSuffix(string(payload[1:]), "\x00"))
		}
	}

	if want := []string{"name \"player\"", "color 0 0", "spawn coop 1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("stage-two signon commands = %v, want %v", got, want)
	}
}

func TestRemoteDatagramClientResetConnectionStateClearsClient(t *testing.T) {
	rc := newRemoteDatagramClient(nil)
	rc.inner.State = cl.StateActive
	rc.inner.Signon = cl.Signons
	rc.inner.LevelName = "stale"
	rc.lastSignonReply = 3

	if err := rc.ResetConnectionState(); err != nil {
		t.Fatalf("ResetConnectionState failed: %v", err)
	}

	if got := rc.inner.State; got != cl.StateConnected {
		t.Fatalf("client state = %v, want %v", got, cl.StateConnected)
	}
	if got := rc.inner.Signon; got != 0 {
		t.Fatalf("client signon = %d, want 0", got)
	}
	if got := rc.inner.LevelName; got != "" {
		t.Fatalf("client level name = %q, want cleared", got)
	}
	if got := rc.lastSignonReply; got != 0 {
		t.Fatalf("lastSignonReply = %d, want 0", got)
	}
}
