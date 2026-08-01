package server

// Client data write tests split from sv_send_test.go.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	inet "github.com/darkliquid/ironwail-go/internal/net"
)

func TestWriteClientDataToMessage_NetQuakeOmitsExtensions(t *testing.T) {
	t.Parallel()

	vm := newTestQCVM()
	vm.StringTable = map[int32]string{}
	weaponModel := vm.AllocString("progs/v_super.mdl")

	modelPrecache := make([]string, 0x124)
	modelPrecache[0x123] = "progs/v_super.mdl"

	s := &Server{
		Protocol:      ProtocolNetQuake,
		QCVM:          vm,
		ModelPrecache: modelPrecache,
	}
	newServerTestVM(s, 8)
	ent := &Edict{
				Alpha: 0x7f,
	}
	ent.SetWeaponModel(s, weaponModel)
	ent.SetWeaponFrame(s, 0x234)
	ent.SetArmorValue(s, 0x345)
	ent.SetHealth(s, 100)
	ent.SetCurrentAmmo(s, 0x456)
	ent.SetAmmoShells(s, 0x567)
	ent.SetAmmoNails(s, 0x678)
	ent.SetAmmoRockets(s, 0x789)
	ent.SetAmmoCells(s, 0x89a)

	msg := NewMessageBuffer(512)
	s.WriteClientDataToMessage(ent, msg)

	bits, payload := decodeClientDataBitsAndPayload(t, msg.Data[:msg.Len()])

	extBits := uint32(
		inet.SU_EXTEND1 | inet.SU_EXTEND2 |
			inet.SU_WEAPON2 | inet.SU_ARMOR2 | inet.SU_AMMO2 |
			inet.SU_SHELLS2 | inet.SU_NAILS2 | inet.SU_ROCKETS2 | inet.SU_CELLS2 |
			inet.SU_WEAPONFRAME2 | inet.SU_WEAPONALPHA,
	)
	if bits&extBits != 0 {
		t.Fatalf("netquake unexpectedly set extension bits: %#x", bits&extBits)
	}

	// NetQuake payload ends after base fields only.
	if len(payload) != 16 {
		t.Fatalf("netquake payload length = %d, want 16; payload=%v", len(payload), payload)
	}
}

func TestWriteClientDataToMessage_FitzSendsWeapon2(t *testing.T) {
	t.Parallel()

	vm := newTestQCVM()
	vm.StringTable = map[int32]string{}
	weaponModel := vm.AllocString("progs/v_super.mdl")

	modelPrecache := make([]string, 0x124)
	modelPrecache[0x123] = "progs/v_super.mdl"

	s := &Server{
		Protocol:      ProtocolFitzQuake,
		QCVM:          vm,
		ModelPrecache: modelPrecache,
	}
	newServerTestVM(s, 8)
	ent := &Edict{
			}
	ent.SetWeaponModel(s, weaponModel)
	ent.SetHealth(s, 100)

	msg := NewMessageBuffer(256)
	s.WriteClientDataToMessage(ent, msg)

	bits, payload := decodeClientDataBitsAndPayload(t, msg.Data[:msg.Len()])

	if bits&inet.SU_WEAPON2 == 0 {
		t.Fatalf("missing SU_WEAPON2 bit: %#x", bits)
	}
	if bits&inet.SU_EXTEND1 == 0 {
		t.Fatalf("missing SU_EXTEND1 bit for SU_WEAPON2: %#x", bits)
	}
	if bits&inet.SU_EXTEND2 != 0 {
		t.Fatalf("unexpected SU_EXTEND2 bit: %#x", bits)
	}

	if got, want := payload[len(payload)-1], byte(0x01); got != want {
		t.Fatalf("weapon2 high byte = %#x, want %#x; payload=%v", got, want, payload)
	}
}

func TestWriteClientDataToMessage_FitzExtensionsPayloadOrder(t *testing.T) {
	t.Parallel()

	vm := newTestQCVM()
	vm.StringTable = map[int32]string{}
	weaponModel := vm.AllocString("progs/v_super.mdl")

	modelPrecache := make([]string, 0x124)
	modelPrecache[0x123] = "progs/v_super.mdl"

	s := &Server{
		Protocol:      ProtocolFitzQuake,
		QCVM:          vm,
		ModelPrecache: modelPrecache,
	}
	newServerTestVM(s, 8)
	ent := &Edict{
				Alpha: 0x7f,
	}
	ent.SetWeaponModel(s, weaponModel)
	ent.SetWeaponFrame(s, 0x234)
	ent.SetArmorValue(s, 0x345)
	ent.SetHealth(s, 100)
	ent.SetCurrentAmmo(s, 0x456)
	ent.SetAmmoShells(s, 0x567)
	ent.SetAmmoNails(s, 0x678)
	ent.SetAmmoRockets(s, 0x789)
	ent.SetAmmoCells(s, 0x89a)

	msg := NewMessageBuffer(512)
	s.WriteClientDataToMessage(ent, msg)

	bits, payload := decodeClientDataBitsAndPayload(t, msg.Data[:msg.Len()])

	required := uint32(
		inet.SU_EXTEND1 | inet.SU_EXTEND2 |
			inet.SU_WEAPON2 | inet.SU_ARMOR2 | inet.SU_AMMO2 |
			inet.SU_SHELLS2 | inet.SU_NAILS2 | inet.SU_ROCKETS2 | inet.SU_CELLS2 |
			inet.SU_WEAPONFRAME2 | inet.SU_WEAPONALPHA,
	)
	if bits&required != required {
		t.Fatalf("missing extension bits: bits=%#x required=%#x", bits, required)
	}

	got := payload[len(payload)-9:]
	want := []byte{
		0x01, // SU_WEAPON2
		0x03, // SU_ARMOR2
		0x04, // SU_AMMO2
		0x05, // SU_SHELLS2
		0x06, // SU_NAILS2
		0x07, // SU_ROCKETS2
		0x08, // SU_CELLS2
		0x02, // SU_WEAPONFRAME2
		0x7f, // SU_WEAPONALPHA
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("extension payload order mismatch:\n got: %v\nwant: %v", got, want)
	}
}

func TestWriteClientDataToMessage_FixAngleUsesVAngle(t *testing.T) {
	t.Parallel()

	s := &Server{Protocol: ProtocolFitzQuake}
	newServerTestVM(s, 8)
	ent := allocPhysicsTestEdict(s)
	ent.SetFixAngle(s, 1)
	ent.SetAngles(s, [3]float32{10, 20, 30})
	ent.SetVAngle(s, [3]float32{90, 180, 270})

	msg := NewMessageBuffer(256)
	s.WriteClientDataToMessage(ent, msg)

	data := msg.Data[:msg.Len()]
	if len(data) < 4 {
		t.Fatalf("short message: %v", data)
	}
	if got, want := data[0], byte(inet.SVCSetAngle); got != want {
		t.Fatalf("message[0] = %d, want %d", got, want)
	}

	want := NewMessageBuffer(16)
	flags := uint32(s.ProtocolFlags())
	want.WriteAngle(ent.VAngle(s)[0], flags)
	want.WriteAngle(ent.VAngle(s)[1], flags)
	want.WriteAngle(ent.VAngle(s)[2], flags)
	if got := data[1:4]; !bytes.Equal(got, want.Data[:want.Len()]) {
		t.Fatalf("setangle payload = %v, want %v", got, want.Data[:want.Len()])
	}
	if ent.FixAngle(s) != 0 {
		t.Fatalf("FixAngle = %v, want 0", ent.FixAngle(s))
	}
}

func TestWriteClientDataToMessage_SendsBaseWeaponBitmask(t *testing.T) {
	t.Parallel()

	s := &Server{Protocol: ProtocolNetQuake}
	newServerTestVM(s, 8)
	ent := &Edict{
			}
	ent.SetWeapon(s, 1 << 5)
	ent.SetHealth(s, 100)
	ent.SetCurrentAmmo(s, 5)

	msg := NewMessageBuffer(128)
	s.WriteClientDataToMessage(ent, msg)

	_, payload := decodeClientDataBitsAndPayload(t, msg.Data[:msg.Len()])
	if got, want := payload[len(payload)-1], byte(1<<5); got != want {
		t.Fatalf("active weapon byte = %#x, want %#x; payload=%v", got, want, payload)
	}
}

func TestWriteClientDataToMessage_MissionPackEncodesWeaponAsBitNumber(t *testing.T) {
	t.Parallel()

	s := &Server{
		Protocol:   ProtocolNetQuake,
		FileSystem: testGameDirFS{gameDir: "rogue"},
	}
	newServerTestVM(s, 8)
	ent := &Edict{
			}
	ent.SetWeapon(s, 1 << 5)
	ent.SetHealth(s, 100)
	ent.SetCurrentAmmo(s, 5)

	msg := NewMessageBuffer(128)
	s.WriteClientDataToMessage(ent, msg)

	_, payload := decodeClientDataBitsAndPayload(t, msg.Data[:msg.Len()])
	if got, want := payload[len(payload)-1], byte(5); got != want {
		t.Fatalf("mission-pack active weapon byte = %#x, want %#x; payload=%v", got, want, payload)
	}
}

func TestWriteClientDataToMessage_SendsFullActiveWeaponStatWhenBitmaskExceedsByte(t *testing.T) {
	t.Parallel()

	s := &Server{Protocol: ProtocolNetQuake}
	newServerTestVM(s, 8)
	ent := &Edict{
			}
	ent.SetWeapon(s, 1 << 8)
	ent.SetHealth(s, 100)
	ent.SetCurrentAmmo(s, 5)

	msg := NewMessageBuffer(128)
	s.WriteClientDataToMessage(ent, msg)

	_, payload := decodeClientDataBitsAndPayload(t, msg.Data[:msg.Len()])
	if got, want := payload[12], byte(0); got != want {
		t.Fatalf("active weapon byte = %#x, want %#x; payload=%v", got, want, payload)
	}
	want := []byte{byte(inet.SVCUpdateStat), byte(inet.StatActiveWeapon), 0x00, 0x01, 0x00, 0x00}
	if got := payload[len(payload)-len(want):]; !bytes.Equal(got, want) {
		t.Fatalf("active weapon updatestat = %v, want %v", got, want)
	}
}

func TestWriteClientDataToMessage_LogsPhysicsTelemetry(t *testing.T) {
	t.Parallel()

	lines := make([]string, 0, 4)
	s := &Server{
		Protocol: ProtocolNetQuake,
		DebugTelemetry: NewDebugTelemetryWithConfig(func() DebugTelemetryConfig {
			return DebugTelemetryConfig{
				Enabled:      true,
				EventMask:    debugEventMaskPhysics,
				EntityFilter: debugEntityFilter{all: true},
				SummaryMode:  0,
			}
		}, func(line string) {
			lines = append(lines, line)
		}),
	}
	newServerTestVM(s, 8)
	oldEnable := debugTelemetryEnableCVar
	debugTelemetryEnableCVar = s.CVar.Register("sv_debug_telemetry_test_clientdata", "1", cvar.FlagNone, "")
	t.Cleanup(func() {
		debugTelemetryEnableCVar = oldEnable
	})

	ent := &Edict{Num: 1}
	ent.SetIdealPitch(s, 7)
	ent.SetVelocity(s, [3]float32{0, 1840, 0})
	ent.SetFlags(s, FlagPartialGround)
	ent.SetViewOfs(s, [3]float32{0, 0, 22})
	ent.SetPunchAngle(s, [3]float32{105, 0, 32})
	ent.SetFixAngle(s, 1)
	ent.SetGroundEntity(s, 99)
	ent.SetTeleportTime(s, 1.25)
	ent.SetHealth(s, 100)
	s.Edicts = []*Edict{{}, ent}

	msg := NewMessageBuffer(256)
	s.WriteClientDataToMessage(ent, msg)

	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"kind=physics",
		"clientdata serialize bits=",
		"onground=false",
		"vel=(0.0 1840.0 0.0)",
		"punch=(105.0 0.0 32.0)",
		"fixangle_sent=true",
		"ground=99",
		"teleport=1.250",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in telemetry:\n%s", want, joined)
		}
	}
}

func TestWriteEntitiesToClient_SkipsEntAlphaZero(t *testing.T) {
	t.Parallel()

	vm := newTestQCVM()
	vm.SetEFloat(1, 0, 0.001) // tiny positive alpha rounds to ENTALPHA_ZERO

	ent := &Edict{
			}
	client := &Client{
		Edict: ent,
	}
	s := &Server{
		Protocol:     ProtocolFitzQuake,
		Static:       &ServerStatic{MaxClients: 1},
		QCVM:         vm,
		QCFieldAlpha: 0,
		QCFieldScale: -1,
		Edicts:       []*Edict{{}, ent},
		NumEdicts:    2,
	}
	newServerTestVM(s, 8)

	msg := NewMessageBuffer(256)
	s.writeEntitiesToClient(client, msg)

	if got := msg.Len(); got != 0 {
		t.Fatalf("writeEntitiesToClient wrote %d bytes for ENTALPHA_ZERO entity, want 0", got)
	}
	if _, ok := client.EntityStates[1]; ok {
		t.Fatal("ENTALPHA_ZERO entity should not be tracked in client.EntityStates")
	}
}

func TestWriteEntitiesToClient_DoesNotEmitRetireForFreedBaselineOnlyEntity(t *testing.T) {
	t.Parallel()

	ent := &Edict{
				Baseline: EntityState{ModelIndex: 5, Scale: inet.ENTSCALE_DEFAULT},
	}
	client := &Client{}
	s := &Server{
		Static:    &ServerStatic{MaxClients: 1},
		Edicts:    []*Edict{{}, ent},
		NumEdicts: 2,
	}
	newServerTestVM(s, 8)

	ent.Free = true
	msg := NewMessageBuffer(256)
	s.writeEntitiesToClient(client, msg)

	if got := msg.Len(); got != 0 {
		t.Fatalf("writeEntitiesToClient wrote %d retire bytes for freed baseline-only entity, want 0", got)
	}
	if _, ok := client.EntityStates[1]; ok {
		t.Fatal("freed baseline-only entity should not be tracked for sticky retire updates")
	}
}

func TestWriteEntitiesToClient_DoesNotEmitStickyRetireForOmittedTrackedEntity(t *testing.T) {
	t.Parallel()

	ent := &Edict{
				Baseline: EntityState{ModelIndex: 5, Scale: inet.ENTSCALE_DEFAULT},
	}
	client := &Client{EntityStates: map[int]EntityState{1: ent.Baseline}}
	s := &Server{
		Static:    &ServerStatic{MaxClients: 1},
		Edicts:    []*Edict{{}, ent},
		NumEdicts: 2,
	}
	newServerTestVM(s, 8)

	ent.Free = true
	first := NewMessageBuffer(256)
	s.writeEntitiesToClient(client, first)
	if got := first.Len(); got != 0 {
		t.Fatalf("writeEntitiesToClient wrote %d retire bytes for omitted tracked entity, want 0", got)
	}

	second := NewMessageBuffer(256)
	s.writeEntitiesToClient(client, second)
	if got := second.Len(); got != 0 {
		t.Fatalf("writeEntitiesToClient wrote %d sticky retire bytes for omitted tracked entity, want 0", got)
	}

	if state := client.EntityStates[1]; state.ModelIndex != ent.Baseline.ModelIndex {
		t.Fatalf("tracked omitted entity ModelIndex=%d, want preserved prior state %d", state.ModelIndex, ent.Baseline.ModelIndex)
	}
}
