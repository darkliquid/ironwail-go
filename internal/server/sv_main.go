package server

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/model"
	"github.com/ironwail/ironwail-go/internal/qc"
)

func normalizeServerMapName(mapName string) string {
	name := strings.TrimSpace(mapName)
	if name == "" {
		return ""
	}
	name = strings.TrimPrefix(strings.ToLower(name), "maps/")
	if idx := strings.LastIndex(strings.ToLower(name), ".bsp"); idx >= 0 && idx+4 == len(name) {
		name = name[:idx]
	}
	return filepath.Base(name)
}

const (
	ProtocolNetQuake  = 15
	ProtocolFitzQuake = 666
	ProtocolRMQ       = 999
	// Note: MaxDatagram and MaxSignonBuffers are defined in types.go
	DatagramMTU   = 1400
	SignonSize    = 31500
	NumSpawnParms = 16
	NumPingTimes  = 16

	// Spawnflag bits stored in EntVars.SpawnFlags, used to filter entities
	// by difficulty and game mode during map loading. Matches the C
	// definitions in pr_edict.c.
	spawnFlagNotEasy       = 0x100 // Don't spawn on Easy difficulty
	spawnFlagNotMedium     = 0x200 // Don't spawn on Medium difficulty
	spawnFlagNotHard       = 0x400 // Don't spawn on Hard/Nightmare difficulty
	spawnFlagNotDeathmatch = 0x800 // Don't spawn in Deathmatch mode
)

var LocalModels [MaxModels][8]byte

// init precomputes *0..*n inline BSP submodel names used by Quake's local model convention.
func init() {
	for i := 0; i < MaxModels; i++ {
		copy(LocalModels[i][:], fmt.Sprintf("*%d", i))
	}
}

// resetLightStyles initializes all dynamic lightstyle slots to "m" (normal brightness baseline).
func resetLightStyles(values *[64]string) {
	for i := range values {
		values[i] = "m"
	}
}

func globalEffectBitSupported(vm *qc.VM, bit int, names ...string) bool {
	for _, name := range names {
		ofs := vm.FindGlobal(name)
		if ofs < 0 {
			continue
		}
		if int(vm.GFloat(ofs)) == bit {
			return true
		}
	}
	return false
}

func (s *Server) mapCheckEnabled() bool {
	// C parity intent: map-check diagnostics become active when either map_checks
	// or developer is enabled. Keep this deliberately tiny until full sv_main
	// parity work lands.
	if cvar.FloatValue("map_checks") != 0 {
		return true
	}
	return cvar.FloatValue("developer") != 0
}

func (s *Server) mapCheckReportf(format string, args ...any) bool {
	msg := fmt.Sprintf(format, args...)
	reported := false
	if s != nil && s.DebugTelemetry != nil {
		if s.DebugTelemetry.LogEventf(DebugEventPhysics, s.QCVM, -1, nil, "mapcheck %s", msg) {
			reported = true
		}
	}
	if s != nil && s.Static != nil {
		for _, client := range s.Static.Clients {
			if !s.SV_IsLocalClient(client) {
				continue
			}
			s.SV_ClientPrintf(client, "%s\n", msg)
			reported = true
		}
	}
	if !reported {
		slog.Warn("mapcheck", "message", msg)
	}
	return reported
}

// SV_MapCheckThresh gates map-check diagnostics behind map_checks/developer.
//
// Minimal parity slice: this currently returns whether diagnostics are enabled
// and leaves threshold semantics to future C-parity work.
func (s *Server) SV_MapCheckThresh(threshold float32) bool {
	_ = threshold
	return s.mapCheckEnabled()
}

// SV_PrintMapCheck emits a single map-check diagnostic line when map checks are enabled.
func (s *Server) SV_PrintMapCheck(format string, args ...any) bool {
	if !s.SV_MapCheckThresh(0) {
		return false
	}
	return s.mapCheckReportf(format, args...)
}

// SV_PrintMapChecklist emits the header plus entries for map-check diagnostics.
func (s *Server) SV_PrintMapChecklist(header string, checks ...string) int {
	if !s.SV_MapCheckThresh(0) {
		return 0
	}
	printed := 0
	if strings.TrimSpace(header) != "" {
		s.mapCheckReportf("%s", header)
		printed++
	}
	for _, check := range checks {
		if strings.TrimSpace(check) == "" {
			continue
		}
		s.mapCheckReportf("- %s", check)
		printed++
	}
	return printed
}

func (s *Server) detectEffectsMaskFromQC() int {
	mask := defaultEffectsMask
	if s == nil || s.QCVM == nil {
		return mask
	}

	if !globalEffectBitSupported(s.QCVM, EffectQuadLight, "EF_QEX_QUADLIGHT", "EF_QUADLIGHT") {
		mask &^= EffectQuadLight
	}
	if !globalEffectBitSupported(s.QCVM, EffectPentaLight, "EF_QEX_PENTALIGHT", "EF_PENTALIGHT") {
		mask &^= EffectPentaLight
	}
	if !globalEffectBitSupported(s.QCVM, EffectCandleLight, "EF_QEX_CANDLELIGHT", "EF_CANDLELIGHT") {
		mask &^= EffectCandleLight
	}

	return mask
}

// Init prepares a fresh runtime server state: client slots, world edicts, caches, and buffers.
func (s *Server) Init(maxClients int) error {
	if maxClients <= 0 {
		return fmt.Errorf("maxClients must be > 0")
	}
	if maxClients > MaxClients {
		return fmt.Errorf("maxClients %d exceeds limit %d", maxClients, MaxClients)
	}

	s.Active = false
	s.Paused = false
	s.LoadGame = false
	s.PreserveSpawnParms = false
	s.State = ServerStateLoading
	s.Name = ""
	s.ModelName = ""
	s.WorldModel = nil
	s.Time = 1
	s.FrameTime = 0.1
	s.EffectsMask = defaultEffectsMask

	if s.MaxEdicts <= 0 {
		s.MaxEdicts = MaxEdicts
	}

	s.Datagram = NewMessageBuffer(MaxDatagram)
	s.ReliableDatagram = NewMessageBuffer(MaxDatagram)
	s.devStats = DevStats{}
	s.devPeak = DevStats{}
	s.SoundPrecache = make([]string, MaxSounds)
	s.ModelPrecache = make([]string, MaxModels)
	s.modelCache = make(map[string]cachedModelInfo)
	s.StaticEntities = nil
	s.StaticSounds = nil
	resetLightStyles(&s.LightStyles)

	s.Static = &ServerStatic{
		MaxClients:        maxClients,
		MaxClientsLimit:   maxClients,
		Clients:           make([]*Client, maxClients),
		ChangeLevelIssued: false,
	}

	s.Edicts = make([]*Edict, maxClients+1)
	for i := range s.Edicts {
		s.Edicts[i] = &Edict{Vars: &EntVars{}, Scale: 16}
	}
	s.NumEdicts = len(s.Edicts)

	for i := 0; i < maxClients; i++ {
		clientEdict := s.Edicts[i+1]
		s.Static.Clients[i] = &Client{
			Edict:        clientEdict,
			Message:      NewMessageBuffer(MaxDatagram),
			Name:         "unconnected",
			EntityStates: make(map[int]EntityState),
		}
	}

	return nil
}

// Shutdown tears down active server state so a new map/server can start from clean memory.
func (s *Server) Shutdown() {
	s.Active = false
	s.Paused = false
	s.State = ServerStateLoading
	s.Name = ""
	s.ModelName = ""
	s.WorldModel = nil
	s.WorldTree = nil
	s.Edicts = nil
	s.NumEdicts = 0
	s.Static = nil
	s.SoundPrecache = nil
	s.ModelPrecache = nil
	s.modelCache = nil
	s.FileSystem = nil
	s.StaticEntities = nil
	s.StaticSounds = nil
	s.SignonBuffers = nil
	s.Signon = nil
	resetLightStyles(&s.LightStyles)
	if s.Datagram != nil {
		s.Datagram.Clear()
	}
	if s.ReliableDatagram != nil {
		s.ReliableDatagram.Clear()
	}
}

// SpawnServer loads BSP assets, resets world state, builds entities, and enters active simulation.
func (s *Server) SpawnServer(mapName string, vfs *fs.FileSystem) error {
	if s.Static == nil {
		return errors.New("server not initialized")
	}
	if vfs == nil {
		return errors.New("filesystem is nil")
	}
	mapName = normalizeServerMapName(mapName)
	if mapName == "" {
		return errors.New("map name is empty")
	}

	if s.Active {
		s.SendReconnect()
	}

	s.Active = false
	s.Paused = false
	s.State = ServerStateLoading
	s.Time = 1
	s.FileSystem = vfs
	s.modelCache = make(map[string]cachedModelInfo)
	if s.Static != nil {
		s.Static.ChangeLevelIssued = false
	}
	resetLightStyles(&s.LightStyles)

	s.Name = mapName
	s.ModelName = fmt.Sprintf("maps/%s.bsp", s.Name)

	bspData, litData, err := vfs.LoadMapBSPAndLit(s.ModelName)
	if err != nil {
		return fmt.Errorf("load map %q: %w", s.ModelName, err)
	}

	tree, err := bsp.LoadTree(bytes.NewReader(bspData))
	if err != nil {
		return fmt.Errorf("parse map %q: %w", s.ModelName, err)
	}
	if err := bsp.ApplyLitFile(tree, litData); err != nil {
		slog.Warn("ignoring invalid .lit sidecar", "map", s.ModelName, "error", err)
	}
	bspFile, err := bsp.Load(bytes.NewReader(bspData))
	if err != nil {
		return fmt.Errorf("parse collision bsp %q: %w", s.ModelName, err)
	}

	worldModel := worldModelFromBSPTree(s.ModelName, tree)
	populateWorldModelCollision(worldModel, tree, bspFile)
	s.WorldModel = worldModel
	s.WorldTree = tree

	if s.Static != nil {
		keep := s.Static.MaxClients + 1
		if keep < 1 {
			keep = 1
		}
		if keep < len(s.Edicts) {
			for i := keep; i < len(s.Edicts); i++ {
				s.Edicts[i] = nil
			}
			s.Edicts = s.Edicts[:keep]
		}
		s.NumEdicts = len(s.Edicts)
	}

	if s.Edicts[0] == nil {
		s.Edicts[0] = &Edict{Vars: &EntVars{}, Scale: 16}
	}
	world := s.Edicts[0]
	world.Free = false
	world.Vars = &EntVars{}
	world.Vars.ModelIndex = 1
	world.Vars.Solid = float32(SolidBSP)
	world.Vars.MoveType = float32(MoveTypePush)
	world.Vars.ClassName = 0
	world.Vars.Model = 0

	s.ModelPrecache[0] = ""
	s.ModelPrecache[1] = s.ModelName
	for i := 1; i < len(tree.Models) && i+1 < len(s.ModelPrecache); i++ {
		s.ModelPrecache[i+1] = string(bytes.TrimRight(LocalModels[i][:], "\x00"))
	}
	if s.FindModel("progs/player.mdl") == 0 {
		for i := 1; i < len(s.ModelPrecache); i++ {
			if s.ModelPrecache[i] != "" {
				continue
			}
			s.ModelPrecache[i] = "progs/player.mdl"
			break
		}
	}
	s.StaticEntities = nil
	s.StaticSounds = nil

	// Initialize the spatial partition tree before entity loading.
	// QC spawn functions (called during loadMapEntities) invoke builtins
	// like setmodel/setsize that call LinkEdict, which requires the area
	// tree to exist. C Ironwail calls SV_ClearWorld() before ED_LoadFromFile().
	s.ClearWorld()

	if s.QCVM != nil {
		if world.Vars.Model == 0 {
			world.Vars.Model = s.QCVM.AllocString(s.ModelName)
		}
		if world.Vars.ClassName == 0 {
			world.Vars.ClassName = s.QCVM.AllocString("worldspawn")
		}
	}

	// Push QC globals (skill, mapname, deathmatch, coop) and the world
	// entity to the VM before spawning map entities. QC spawn functions
	// read these globals to decide behavior.
	s.syncQCVMState()

	// Cache QC field offsets for alpha/scale (used every frame in entity updates).
	if s.QCVM != nil {
		s.QCFieldAlpha = s.QCVM.FindField("alpha")
		s.QCFieldScale = s.QCVM.FindField("scale")
		s.QCFieldGravity = s.QCVM.FindField("gravity")
		s.EffectsMask = s.detectEffectsMaskFromQC()
	} else {
		s.EffectsMask = defaultEffectsMask
	}

	s.suppressTouchQC = true
	defer func() {
		s.suppressTouchQC = false
	}()

	if err := s.loadMapEntities(string(tree.Entities)); err != nil {
		return fmt.Errorf("parse map entities %q: %w", s.ModelName, err)
	}

	s.LinkEdict(world, false)

	s.Active = true
	s.State = ServerStateActive
	s.FrameTime = 0.1
	s.Physics()
	s.Physics()
	s.suppressTouchQC = false

	// Populate signon buffers with static entities and ambient sounds.
	// These are shared across all connecting clients.
	if err := s.buildSignonBuffers(); err != nil {
		return fmt.Errorf("build signon buffers: %w", err)
	}

	slog.Info("server spawned map start", "map", mapName)

	return nil
}

// loadMapEntities parses the BSP entity lump and instantiates edicts from textual key/value blocks.
//
// This is the Go equivalent of C Ironwail's ED_LoadFromFile(). After parsing
// each entity's key-value pairs, it filters by skill/deathmatch flags and
// calls the QC spawn function matching the entity's classname (e.g.,
// "trigger_teleport" → QC function trigger_teleport()). Without this dispatch
// step, map entities would have no touch functions, think routines, or solid
// types — making triggers, doors, items, and monsters non-functional.
func (s *Server) loadMapEntities(raw string) error {
	if strings.Trim(raw, " \t\r\n\x00") == "" {
		return nil
	}
	maxClients := 0
	if s.Static != nil {
		maxClients = s.Static.MaxClients
	}
	em := &EntityManager{
		edicts:     s.Edicts,
		vm:         s.QCVM,
		maxEdicts:  s.MaxEdicts,
		numEdicts:  s.NumEdicts,
		maxClients: maxClients,
		freeTime:   make([]float32, max(s.MaxEdicts, len(s.Edicts))),
	}

	// Read skill and deathmatch cvars for entity filtering.
	skill := 1
	if skillCV := cvar.Get("skill"); skillCV != nil {
		skill = int(skillCV.Float + 0.5)
		if skill < 0 {
			skill = 0
		} else if skill > 3 {
			skill = 3
		}
	}
	isDeathmatch := cvar.FloatValue("deathmatch") != 0
	noMonsters := cvar.FloatValue("nomonsters") != 0

	inhibited := 0
	telemetryEnabled := s.DebugTelemetry != nil && s.DebugTelemetry.EventsEnabled()
	remaining := raw
	for entIndex := 0; ; entIndex++ {
		remaining = strings.TrimLeft(remaining, " \t\r\n\x00")
		if remaining == "" {
			break
		}

		entNum := entIndex
		if entIndex > 0 {
			ent := s.AllocEdict()
			if ent == nil {
				return fmt.Errorf("no free edict for map entity %d", entIndex)
			}
			entNum = s.NumForEdict(ent)
			em.edicts = s.Edicts
			em.numEdicts = s.NumEdicts
		}

		next, err := em.ED_ParseEdict(remaining, entNum)
		if err != nil {
			return err
		}
		remaining = next

		ent := s.EdictNum(entNum)
		if ent == nil || ent.Vars == nil {
			continue
		}

		// Entity 0 is the worldspawn — it gets special handling and
		// its spawn function runs like any other entity.
		if s.QCVM == nil {
			continue
		}

		// Resolve the classname string from the QC string table.
		className := s.QCVM.GetString(ent.Vars.ClassName)
		if className == "" {
			slog.Warn("entity has no classname", "entNum", entNum)
			s.FreeEdict(ent)
			continue
		}

		// Filter entities by skill level and deathmatch flags, matching
		// C Ironwail's ED_LoadFromFile (pr_edict.c:1527-1549).
		spawnFlags := int(ent.Vars.SpawnFlags)
		if isDeathmatch {
			if spawnFlags&spawnFlagNotDeathmatch != 0 {
				s.FreeEdict(ent)
				inhibited++
				continue
			}
		} else {
			if skill == 0 && spawnFlags&spawnFlagNotEasy != 0 {
				s.FreeEdict(ent)
				inhibited++
				continue
			}
			if skill == 1 && spawnFlags&spawnFlagNotMedium != 0 {
				s.FreeEdict(ent)
				inhibited++
				continue
			}
			if skill >= 2 && spawnFlags&spawnFlagNotHard != 0 {
				s.FreeEdict(ent)
				inhibited++
				continue
			}
		}

		// Skip monsters if nomonsters cvar is set.
		if noMonsters && strings.HasPrefix(className, "monster_") {
			s.FreeEdict(ent)
			inhibited++
			continue
		}

		// Find the QC function matching the classname (e.g. "trigger_teleport").
		funcIdx := s.QCVM.FindFunction(className)
		if funcIdx < 0 {
			slog.Warn("no spawn function for entity", "classname", className, "entNum", entNum)
			s.FreeEdict(ent)
			continue
		}

		// Push parsed Go-backed entity fields to the QCVM so the spawn function
		// can read them (origin, angles, target, etc.). ED_ParseEdict already
		// cleared reused VM storage and wrote any QC-only map fields directly into
		// the VM, so do not clear here or those progs-defined fields would be lost
		// before spawn.
		s.ensureQCVMEdictStorage()
		syncEdictToQCVM(s.QCVM, entNum, ent)

		// Set QC globals and execute the spawn function.
		if err := s.ReserveSignonSpace(512); err != nil {
			return fmt.Errorf("reserve signon space for %q: %w", className, err)
		}
		if telemetryEnabled && strings.HasPrefix(className, "trigger_") {
			s.DebugTelemetry.LogEventf(DebugEventTrigger, s.QCVM, entNum, ent,
				"spawn trigger qc begin classname=%q targetname=%q target=%q touch=%d solid=%d origin=(%.1f %.1f %.1f)",
				className,
				s.QCVM.GetString(ent.Vars.TargetName),
				s.QCVM.GetString(ent.Vars.Target),
				ent.Vars.Touch,
				int(ent.Vars.Solid),
				ent.Vars.Origin[0], ent.Vars.Origin[1], ent.Vars.Origin[2],
			)
		}
		s.QCVM.SetGlobal("self", entNum)
		s.QCVM.SetGlobal("time", s.Time)
		if err := s.executeQCFunction(funcIdx); err != nil {
			slog.Error("spawn function failed", "classname", className, "entNum", entNum, "err", err)
			s.FreeEdict(ent)
			continue
		}

		// Pull QC-modified fields back to Go (solid, touch, think, etc.).
		syncEdictFromQCVM(s.QCVM, entNum, ent)
		if telemetryEnabled && strings.HasPrefix(className, "trigger_") {
			s.DebugTelemetry.LogEventf(DebugEventTrigger, s.QCVM, entNum, ent,
				"spawn trigger qc end classname=%q targetname=%q target=%q touch=%d solid=%d absmin=(%.1f %.1f %.1f) absmax=(%.1f %.1f %.1f)",
				className,
				s.QCVM.GetString(ent.Vars.TargetName),
				s.QCVM.GetString(ent.Vars.Target),
				ent.Vars.Touch,
				int(ent.Vars.Solid),
				ent.Vars.AbsMin[0], ent.Vars.AbsMin[1], ent.Vars.AbsMin[2],
				ent.Vars.AbsMax[0], ent.Vars.AbsMax[1], ent.Vars.AbsMax[2],
			)
		}
		s.LinkEdict(ent, false)
		if telemetryEnabled && strings.HasPrefix(className, "trigger_") {
			linkState := "linked"
			if ent.AreaPrev == nil {
				linkState = "unlinked"
			}
			s.DebugTelemetry.LogEventf(DebugEventTrigger, s.QCVM, entNum, ent,
				"spawn trigger relink classname=%q link=%s solid=%d touch=%d absmin=(%.1f %.1f %.1f) absmax=(%.1f %.1f %.1f)",
				className, linkState, int(ent.Vars.Solid), ent.Vars.Touch,
				ent.Vars.AbsMin[0], ent.Vars.AbsMin[1], ent.Vars.AbsMin[2],
				ent.Vars.AbsMax[0], ent.Vars.AbsMax[1], ent.Vars.AbsMax[2],
			)
		}
	}

	if inhibited > 0 {
		slog.Info("entities inhibited by skill/deathmatch filtering", "count", inhibited)
	}
	s.NumEdicts = len(s.Edicts)
	return nil
}

// worldModelFromBSPTree adapts parsed BSP tree data into the runtime model.Model expected by engine subsystems.
func worldModelFromBSPTree(modelName string, tree *bsp.Tree) *model.Model {
	m := &model.Model{
		Name:      modelName,
		Type:      model.ModBrush,
		NumLeafs:  len(tree.Leafs),
		NumNodes:  len(tree.Nodes),
		Entities:  string(tree.Entities),
		NumPlanes: len(tree.Planes),
	}

	if len(tree.Models) > 0 {
		m.Mins = tree.Models[0].BoundsMin
		m.Maxs = tree.Models[0].BoundsMax
		m.ClipMins = m.Mins
		m.ClipMaxs = m.Maxs
		m.ClipBox = true
	}
	m.NumSubModels = len(tree.Models)
	m.SubModels = append([]bsp.DModel(nil), tree.Models...)

	m.Planes = make([]model.MPlane, len(tree.Planes))
	for i, p := range tree.Planes {
		m.Planes[i] = model.MPlane{
			Normal: p.Normal,
			Dist:   p.Dist,
			Type:   uint8(p.Type),
		}
	}

	m.Nodes = make([]model.MNode, len(tree.Nodes))
	for i, n := range tree.Nodes {
		m.Nodes[i] = model.MNode{
			Contents: int(bsp.ContentsEmpty),
			MinMaxs: [6]float32{
				n.BoundsMin[0], n.BoundsMin[1], n.BoundsMin[2],
				n.BoundsMax[0], n.BoundsMax[1], n.BoundsMax[2],
			},
			FirstSurface: n.FirstFace,
			NumSurfaces:  n.NumFaces,
		}
		if int(n.PlaneNum) >= 0 && int(n.PlaneNum) < len(m.Planes) {
			m.Nodes[i].Plane = &m.Planes[n.PlaneNum]
		}
	}

	for i, n := range tree.Nodes {
		for side := 0; side < 2; side++ {
			child := n.Children[side]
			if !child.IsLeaf && child.Index >= 0 && child.Index < len(m.Nodes) {
				m.Nodes[i].Children[side] = &m.Nodes[child.Index]
			}
		}
	}

	for i := range m.Hulls {
		m.Hulls[i].FirstClipNode = -1
		m.Hulls[i].LastClipNode = -1
	}

	return m
}

var brushHullClipBounds = [model.MaxMapHulls]struct {
	mins [3]float32
	maxs [3]float32
}{
	0: {},
	1: {mins: [3]float32{-16, -16, -24}, maxs: [3]float32{16, 16, 32}},
	2: {mins: [3]float32{-32, -32, -24}, maxs: [3]float32{32, 32, 64}},
}

// populateWorldModelCollision builds movement hulls/clipnodes so SV_Move can trace against map geometry.
func populateWorldModelCollision(m *model.Model, tree *bsp.Tree, file *bsp.File) {
	if m == nil || tree == nil || len(m.Planes) == 0 || len(tree.Models) == 0 {
		return
	}

	m.Hulls[0] = buildNodeHull(tree, m.Planes, int(tree.Models[0].HeadNode[0]))

	clipNodes := bspClipNodesToModel(file)
	if len(clipNodes) == 0 {
		return
	}

	m.ClipNodes = clipNodes
	for hullNum := 1; hullNum <= 2; hullNum++ {
		headNode := int(tree.Models[0].HeadNode[hullNum])
		if headNode < 0 {
			continue
		}
		m.Hulls[hullNum] = model.Hull{
			ClipNodes:     clipNodes,
			Planes:        m.Planes,
			FirstClipNode: headNode,
			LastClipNode:  len(clipNodes) - 1,
			ClipMins:      brushHullClipBounds[hullNum].mins,
			ClipMaxs:      brushHullClipBounds[hullNum].maxs,
		}
	}
}

// buildNodeHull converts BSP nodes/leaves into a hull clipnode graph for player/world collision tracing.
func buildNodeHull(tree *bsp.Tree, planes []model.MPlane, headNode int) model.Hull {
	if tree == nil || len(tree.Nodes) == 0 || headNode < 0 || headNode >= len(tree.Nodes) {
		return model.Hull{FirstClipNode: -1, LastClipNode: -1}
	}

	clipNodes := make([]model.MClipNode, len(tree.Nodes))
	for i, node := range tree.Nodes {
		clipNodes[i].PlaneNum = int(node.PlaneNum)
		for side, child := range node.Children {
			if child.IsLeaf {
				if child.Index >= 0 && child.Index < len(tree.Leafs) {
					clipNodes[i].Children[side] = int(tree.Leafs[child.Index].Contents)
				} else {
					clipNodes[i].Children[side] = bsp.ContentsSolid
				}
				continue
			}
			clipNodes[i].Children[side] = child.Index
		}
	}

	return model.Hull{
		ClipNodes:     clipNodes,
		Planes:        planes,
		FirstClipNode: headNode,
		LastClipNode:  len(clipNodes) - 1,
	}
}

// bspClipNodesToModel normalizes BSP clipnode lump variants into model.MClipNode runtime format.
func bspClipNodesToModel(file *bsp.File) []model.MClipNode {
	if file == nil {
		return nil
	}

	switch clipNodes := file.Clipnodes.(type) {
	case []bsp.DSClipNode:
		out := make([]model.MClipNode, len(clipNodes))
		for i, node := range clipNodes {
			out[i] = model.MClipNode{
				PlaneNum: int(node.PlaneNum),
				Children: [2]int{int(node.Children[0]), int(node.Children[1])},
			}
		}
		return out
	case []bsp.DLClipNode:
		out := make([]model.MClipNode, len(clipNodes))
		for i, node := range clipNodes {
			out[i] = model.MClipNode{
				PlaneNum: int(node.PlaneNum),
				Children: [2]int{int(node.Children[0]), int(node.Children[1])},
			}
		}
		return out
	default:
		return nil
	}
}

// modelBounds resolves bounding boxes for world and inline BSP models for SetModel/LinkEdict updates.
func (s *Server) modelBounds(modelName string) (mins, maxs [3]float32, ok bool) {
	if modelName == "" {
		return mins, maxs, true
	}

	if wm := s.WorldModel; wm != nil {
		if modelName == s.ModelName {
			clipMins := wm.CollisionClipMins()
			clipMaxs := wm.CollisionClipMaxs()
			if wm.ModelType() == int(model.ModBrush) && (wm.IsClipBox() || clipMins != [3]float32{} || clipMaxs != [3]float32{}) {
				return clipMins, clipMaxs, true
			}
			if s.WorldTree != nil && len(s.WorldTree.Models) > 0 {
				return s.WorldTree.Models[0].BoundsMin, s.WorldTree.Models[0].BoundsMax, true
			}
		}

		if len(modelName) > 1 && modelName[0] == '*' {
			idx, err := strconv.Atoi(modelName[1:])
			if err == nil && idx >= 0 {
				if s.WorldTree != nil && idx < len(s.WorldTree.Models) {
					sub := s.WorldTree.Models[idx]
					return sub.BoundsMin, sub.BoundsMax, true
				}
			}
		}
	}

	if info, exists := s.modelCache[modelName]; exists && info.known {
		return info.mins, info.maxs, true
	}

	return mins, maxs, false
}

type cachedModelInfo struct {
	mins  [3]float32
	maxs  [3]float32
	known bool
}

func spriteBounds(sprite *model.MSprite) ([3]float32, [3]float32) {
	halfWidth := float32(sprite.MaxWidth) * 0.5
	halfHeight := float32(sprite.MaxHeight) * 0.5
	return [3]float32{-halfWidth, -halfWidth, -halfHeight}, [3]float32{halfWidth, halfWidth, halfHeight}
}

func (s *Server) cacheModelInfo(modelName string) (cachedModelInfo, error) {
	if modelName == "" || (len(modelName) > 0 && modelName[0] == '*') {
		return cachedModelInfo{known: true}, nil
	}
	if info, ok := s.modelCache[modelName]; ok {
		return info, nil
	}
	if s.FileSystem == nil {
		return cachedModelInfo{}, fmt.Errorf("filesystem unavailable while loading %q", modelName)
	}
	file, _, err := s.FileSystem.OpenFile(modelName)
	if err != nil {
		return cachedModelInfo{}, err
	}
	defer file.Close()

	var info cachedModelInfo
	switch filepath.Ext(modelName) {
	case ".mdl":
		m, err := model.LoadAliasModel(file)
		if err != nil {
			return cachedModelInfo{}, err
		}
		info = cachedModelInfo{mins: m.Mins, maxs: m.Maxs, known: true}
	case ".spr":
		sprite, err := model.LoadSprite(file)
		if err != nil {
			return cachedModelInfo{}, err
		}
		info.mins, info.maxs = spriteBounds(sprite)
		info.known = true
	case ".bsp":
		tree, err := bsp.LoadTree(file)
		if err != nil {
			return cachedModelInfo{}, err
		}
		if len(tree.Models) > 0 {
			info = cachedModelInfo{mins: tree.Models[0].BoundsMin, maxs: tree.Models[0].BoundsMax, known: true}
		} else {
			info = cachedModelInfo{known: true}
		}
	default:
		return cachedModelInfo{}, fmt.Errorf("unsupported model type %q", modelName)
	}

	if s.modelCache == nil {
		s.modelCache = make(map[string]cachedModelInfo)
	}
	s.modelCache[modelName] = info
	return info, nil
}

// ProtocolFlags control coordinate/angle precision in network messages.
// Bit positions match C Ironwail's PRFL_* defines in protocol.h.
type ProtocolFlags uint32

const (
	ProtocolFlagShortAngle  ProtocolFlags = 1 << 1 // PRFL_SHORTANGLE: 16-bit angles
	ProtocolFlagFloatAngle  ProtocolFlags = 1 << 2 // PRFL_FLOATANGLE: 32-bit angles
	ProtocolFlag24BitCoord  ProtocolFlags = 1 << 3 // PRFL_24BITCOORD: 24-bit coords
	ProtocolFlagFloatCoord  ProtocolFlags = 1 << 4 // PRFL_FLOATCOORD: 32-bit coords
	ProtocolFlagEdictScale  ProtocolFlags = 1 << 5 // PRFL_EDICTSCALE: entity scale
	ProtocolFlagAlphaSanity ProtocolFlags = 1 << 6 // PRFL_ALPHASANITY: alpha cleanup
	ProtocolFlagInt32Coord  ProtocolFlags = 1 << 7 // PRFL_INT32COORD: 32-bit int coords
)

// ProtocolFlags returns the protocol flags for the current server.
// For FitzQuake protocol (666), protocolflags is 0 (default 16-bit coords, 8-bit angles).
// Protocol flags are only meaningful for RMQ protocol (999).
func (s *Server) ProtocolFlags() ProtocolFlags {
	if s != nil && s.Protocol == ProtocolRMQ {
		return ProtocolFlagInt32Coord | ProtocolFlagShortAngle
	}
	return 0
}
