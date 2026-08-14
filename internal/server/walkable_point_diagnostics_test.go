// This file belongs to the Tests subsystem: unit, integration, parity, and e2e tests for the server package.

package server

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/model"
	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/testutil"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

const maxWalkableFailureSamples = 8

var (
	walkablePlayerMins = qtypes.Vec3{X: -16, Y: -16, Z: -24}
	walkablePlayerMaxs = qtypes.Vec3{X: 16, Y: 16, Z: 32}
)

type walkableHullSummary struct {
	Index         int
	FirstClipNode int
	LastClipNode  int
	ClipNodeCount int
	PlaneCount    int
	ClipMins      qtypes.Vec3
	ClipMaxs      qtypes.Vec3
}

type walkableWorldSummary struct {
	ModelName   string
	BoundsMin   qtypes.Vec3
	BoundsMax   qtypes.Vec3
	TreeModels  int
	TreeNodes   int
	TreeLeafs   int
	TreeFaces   int
	HeadNodes   [bsp.MaxMapHulls]int32
	CollisionOK bool
	Hulls       []walkableHullSummary
}

type walkableSampleFailure struct {
	XI              int
	YI              int
	Start           qtypes.Vec3
	StartContents   int
	End             qtypes.Vec3
	TraceFraction   float32
	TraceStartSolid bool
	TraceAllSolid   bool
	TraceEndPos     qtypes.Vec3
	Lifted          qtypes.Vec3
	LiftedContents  int
	Reason          string
}

type walkablePointDiagnostics struct {
	World         walkableWorldSummary
	SamplesTried  int
	ReasonCounts  map[string]int
	FailedSamples []walkableSampleFailure
	ChosenSample  *walkableSampleFailure
}

func (d *walkablePointDiagnostics) addFailure(sample walkableSampleFailure) {
	d.SamplesTried++
	d.ReasonCounts[sample.Reason]++
	if len(d.FailedSamples) < maxWalkableFailureSamples {
		d.FailedSamples = append(d.FailedSamples, sample)
	}
}

func (d *walkablePointDiagnostics) setChosen(sample walkableSampleFailure) {
	d.SamplesTried++
	d.ReasonCounts["success"]++
	sample.Reason = "success"
	cp := sample
	d.ChosenSample = &cp
}

func (d walkablePointDiagnostics) String() string {
	reasons := make([]string, 0, len(d.ReasonCounts))
	for reason, count := range d.ReasonCounts {
		reasons = append(reasons, fmt.Sprintf("%s=%d", reason, count))
	}
	sort.Strings(reasons)
	hulls := make([]string, 0, len(d.World.Hulls))
	for _, hull := range d.World.Hulls {
		hulls = append(hulls, fmt.Sprintf("#%d clipNodes=%d planes=%d first=%d last=%d clipMins=(%.0f %.0f %.0f) clipMaxs=(%.0f %.0f %.0f)",
			hull.Index,
			hull.ClipNodeCount,
			hull.PlaneCount,
			hull.FirstClipNode,
			hull.LastClipNode,
			hull.ClipMins.X, hull.ClipMins.Y, hull.ClipMins.Z,
			hull.ClipMaxs.X, hull.ClipMaxs.Y, hull.ClipMaxs.Z))
	}

	sb := strings.Builder{}
	fmt.Fprintf(&sb, "world model=%q bounds=(%.1f %.1f %.1f)->(%.1f %.1f %.1f) tree(models=%d nodes=%d leafs=%d faces=%d) headnodes=%v collisionModel=%v hulls=[%s] samples=%d reasons={%s}",
		d.World.ModelName,
		d.World.BoundsMin.X, d.World.BoundsMin.Y, d.World.BoundsMin.Z,
		d.World.BoundsMax.X, d.World.BoundsMax.Y, d.World.BoundsMax.Z,
		d.World.TreeModels, d.World.TreeNodes, d.World.TreeLeafs, d.World.TreeFaces,
		d.World.HeadNodes, d.World.CollisionOK, strings.Join(hulls, "; "),
		d.SamplesTried, strings.Join(reasons, ", "))

	if d.ChosenSample != nil {
		chosen := d.ChosenSample
		fmt.Fprintf(&sb, " chosen=[xi=%d yi=%d start=(%.1f %.1f %.1f) end=(%.1f %.1f %.1f) traceEnd=(%.1f %.1f %.1f) lifted=(%.1f %.1f %.1f)]",
			chosen.XI, chosen.YI,
			chosen.Start.X, chosen.Start.Y, chosen.Start.Z,
			chosen.End.X, chosen.End.Y, chosen.End.Z,
			chosen.TraceEndPos.X, chosen.TraceEndPos.Y, chosen.TraceEndPos.Z,
			chosen.Lifted.X, chosen.Lifted.Y, chosen.Lifted.Z)
	}
	if len(d.FailedSamples) > 0 {
		sb.WriteString(" firstFailures=[")
		for i, failure := range d.FailedSamples {
			if i > 0 {
				sb.WriteString("; ")
			}
			fmt.Fprintf(&sb, "#%d reason=%s xi=%d yi=%d start=(%.1f %.1f %.1f) startCont=%d frac=%.3f startSolid=%v allSolid=%v lifted=(%.1f %.1f %.1f) liftedCont=%d",
				i,
				failure.Reason,
				failure.XI,
				failure.YI,
				failure.Start.X, failure.Start.Y, failure.Start.Z,
				failure.StartContents,
				failure.TraceFraction,
				failure.TraceStartSolid,
				failure.TraceAllSolid,
				failure.Lifted.X, failure.Lifted.Y, failure.Lifted.Z,
				failure.LiftedContents)
		}
		sb.WriteString("]")
	}
	return sb.String()
}

func collectWalkableWorldSummary(s *Server, mins, maxs qtypes.Vec3) walkableWorldSummary {
	summary := walkableWorldSummary{
		ModelName: s.ModelName,
		BoundsMin: mins,
		BoundsMax: maxs,
	}
	if s.WorldTree != nil {
		summary.TreeModels = len(s.WorldTree.Models)
		summary.TreeNodes = len(s.WorldTree.Nodes)
		summary.TreeLeafs = len(s.WorldTree.Leafs)
		summary.TreeFaces = len(s.WorldTree.Faces)
		if len(s.WorldTree.Models) > 0 {
			summary.HeadNodes = s.WorldTree.Models[0].HeadNode
		}
	}
	if s.WorldModel != nil {
		summary.CollisionOK = true
		hulls := make([]walkableHullSummary, 0, s.WorldModel.NumHulls())
		for i := 0; i < s.WorldModel.NumHulls(); i++ {
			hull := s.WorldModel.Hull(i)
			hulls = append(hulls, walkableHullSummary{
				Index:         i,
				FirstClipNode: hull.FirstClipNode,
				LastClipNode:  hull.LastClipNode,
				ClipNodeCount: len(hull.ClipNodes),
				PlaneCount:    len(hull.Planes),
				ClipMins:      hull.ClipMins,
				ClipMaxs:      hull.ClipMaxs,
			})
		}
		summary.Hulls = hulls
	}
	return summary
}

func validateWalkableSample(s *Server, sample walkableSampleFailure) (qtypes.Vec3, bool, walkableSampleFailure) {
	sample.StartContents = s.PointContents(sample.Start)
	if sample.StartContents != bsp.ContentsEmpty {
		sample.Reason = "start-point-not-empty"
		return qtypes.Vec3{}, false, sample
	}

	trace := s.Move(sample.Start, walkablePlayerMins, walkablePlayerMaxs, sample.End, MoveNormal, nil)
	sample.TraceFraction = trace.Fraction
	sample.TraceStartSolid = trace.StartSolid
	sample.TraceAllSolid = trace.AllSolid
	sample.TraceEndPos = trace.EndPos

	if trace.StartSolid {
		sample.Reason = "trace-startsolid"
		return qtypes.Vec3{}, false, sample
	}
	if trace.AllSolid {
		sample.Reason = "trace-allsolid"
		return qtypes.Vec3{}, false, sample
	}
	if trace.Fraction == 1 {
		sample.Reason = "trace-no-floor-hit"
		return qtypes.Vec3{}, false, sample
	}

	return validateWalkableStandingOrigin(s, trace.EndPos, sample)
}

func validateWalkableStandingOrigin(s *Server, origin qtypes.Vec3, sample walkableSampleFailure) (qtypes.Vec3, bool, walkableSampleFailure) {
	sample.Lifted = origin
	sample.LiftedContents = s.PointContents(origin)
	if sample.LiftedContents != bsp.ContentsEmpty {
		sample.Reason = "lifted-point-not-empty"
		return qtypes.Vec3{}, false, sample
	}

	ent := allocPhysicsTestEdict(s)
	ent.SetOrigin(s, origin)
	ent.SetMins(s, walkablePlayerMins)
	ent.SetMaxs(s, walkablePlayerMaxs)
	ent.SetSize(s, qtypes.Vec3{X: 32, Y: 32, Z: 56})
	ent.SetSolid(s, float32(SolidSlideBox))
	ent.SetAbsMin(s, origin.Add(qtypes.Vec3{X: -16, Y: -16, Z: -24}))
	ent.SetAbsMax(s, origin.Add(qtypes.Vec3{X: 16, Y: 16, Z: 32}))
	if blocker := s.SV_TestEntityPosition(ent); blocker != nil {
		sample.Reason = "lifted-entity-blocked"
		return qtypes.Vec3{}, false, sample
	}
	if !s.CheckBottom(ent) {
		sample.Reason = "lifted-no-bottom"
		return qtypes.Vec3{}, false, sample
	}
	if supportsStationaryMoveStep(s, origin) {
		return origin, true, sample
	}
	sample.Reason = "lifted-not-stationary"
	return qtypes.Vec3{}, false, sample
}

func validateWalkableSpawnPoint(s *Server, origin qtypes.Vec3, sample walkableSampleFailure) (qtypes.Vec3, bool, walkableSampleFailure) {
	worldMins, _, ok := s.modelBounds(s.ModelName)
	if !ok {
		sample.Reason = "model-bounds-unavailable"
		return qtypes.Vec3{}, false, sample
	}

	var lastFailure walkableSampleFailure
	for _, zOffset := range []float32{0, 1, stepSize, 24, 32, 48, 64} {
		start := origin
		start.Z += zOffset
		sample.Start = start
		sample.End = qtypes.Vec3{X: start.X, Y: start.Y, Z: worldMins.Z - 256}
		if pos, ok, validated := validateWalkableSample(s, sample); ok {
			return pos, true, validated
		} else {
			lastFailure = validated
		}
	}

	if lastFailure.Reason == "" {
		sample.Reason = "spawn-no-floor"
		return qtypes.Vec3{}, false, sample
	}
	return qtypes.Vec3{}, false, lastFailure
}

func supportsStationaryMoveStep(s *Server, pos qtypes.Vec3) bool {
	ent := allocPhysicsTestEdict(s)
	ent.SetOrigin(s, pos)
	ent.SetMins(s, walkablePlayerMins)
	ent.SetMaxs(s, walkablePlayerMaxs)
	ent.SetSize(s, qtypes.Vec3{X: 32, Y: 32, Z: 56})
	ent.SetSolid(s, float32(SolidSlideBox))
	ent.SetMoveType(s, float32(MoveTypeStep))
	ent.SetFlags(s, float32(FlagOnGround))
	ent.SetAbsMin(s, pos.Add(qtypes.Vec3{X: -16, Y: -16, Z: -24}))
	ent.SetAbsMax(s, pos.Add(qtypes.Vec3{X: 16, Y: 16, Z: 32}))
	s.Edicts = append(s.Edicts, ent)
	s.NumEdicts = len(s.Edicts)
	defer func() {
		s.UnlinkEdict(ent)
		s.Edicts = s.Edicts[:len(s.Edicts)-1]
		s.NumEdicts = len(s.Edicts)
	}()
	s.LinkEdict(ent, false)
	if blocker := s.SV_TestEntityPosition(ent); blocker != nil {
		return false
	}
	original := ent.Origin(s)
	if !s.MoveStep(ent, qtypes.Vec3{}, true) {
		return false
	}
	return ent.Origin(s) == original
}

var quotedEntityPairRE = regexp.MustCompile(`"([^"]+)"\s+"([^"]*)"`)

func findSpawnOriginsFromEntityLump(s *Server) []qtypes.Vec3 {
	if s == nil || s.WorldTree == nil || len(s.WorldTree.Entities) == 0 {
		return nil
	}

	spawnClassOrder := []string{
		"info_player_start",
		"testplayerstart",
		"info_player_coop",
		"info_player_deathmatch",
		"info_player_start2",
	}
	blocks := strings.Split(string(s.WorldTree.Entities), "{")
	origins := make([]qtypes.Vec3, 0, len(blocks))
	for _, className := range spawnClassOrder {
		for _, block := range blocks {
			if !strings.Contains(block, "\"classname\"") {
				continue
			}
			fields := make(map[string]string, 4)
			for _, match := range quotedEntityPairRE.FindAllStringSubmatch(block, -1) {
				if len(match) >= 3 {
					fields[match[1]] = match[2]
				}
			}
			if fields["classname"] != className {
				continue
			}
			originValue, ok := fields["origin"]
			if !ok {
				continue
			}
			origin, err := parseVec3(originValue)
			if err == nil {
				origins = append(origins, origin)
			}
		}
	}
	return origins
}

func findSpawnOriginFromEntityLump(s *Server) (qtypes.Vec3, bool) {
	origins := findSpawnOriginsFromEntityLump(s)
	if len(origins) == 0 {
		return qtypes.Vec3{}, false
	}
	return origins[0], true
}

func TestFindSpawnOriginFromEntityLumpParsesInfoPlayerStart(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 8)
	s.WorldTree = &bsp.Tree{
		Entities: []byte(`{
"classname" "worldspawn"
}
{
"classname" "info_player_start"
"origin" "544 288 32"
}
{
"classname" "testplayerstart"
"origin" "1 2 3"
}`),
	}

	got, ok := findSpawnOriginFromEntityLump(s)
	if !ok {
		t.Fatal("findSpawnOriginFromEntityLump() = not found, want parsed start origin")
	}
	if want := (qtypes.Vec3{X: 544, Y: 288, Z: 32}); got != want {
		t.Fatalf("findSpawnOriginFromEntityLump() = %v, want %v", got, want)
	}
}

func TestFindSpawnOriginsFromEntityLumpEnumeratesSupportedSpawnClasses(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 8)
	s.WorldTree = &bsp.Tree{
		Entities: []byte(`{
"classname" "worldspawn"
}
{
"classname" "info_player_deathmatch"
"origin" "4 5 6"
}
{
"classname" "info_player_start2"
"origin" "7 8 9"
}
{
"classname" "testplayerstart"
"origin" "1 2 3"
}
{
"classname" "info_player_start"
"origin" "544 288 32"
}
{
"classname" "info_player_coop"
"origin" "10 11 12"
}`),
	}

	got := findSpawnOriginsFromEntityLump(s)
	want := []qtypes.Vec3{
		{X: 544, Y: 288, Z: 32},
		{X: 1, Y: 2, Z: 3},
		{X: 10, Y: 11, Z: 12},
		{X: 4, Y: 5, Z: 6},
		{X: 7, Y: 8, Z: 9},
	}
	if len(got) != len(want) {
		t.Fatalf("findSpawnOriginsFromEntityLump() len=%d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("findSpawnOriginsFromEntityLump()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func findWalkablePointWithDiagnostics(s *Server) (qtypes.Vec3, bool, walkablePointDiagnostics) {
	mins, maxs, ok := s.modelBounds(s.ModelName)
	diag := walkablePointDiagnostics{
		World:        collectWalkableWorldSummary(s, mins, maxs),
		ReasonCounts: make(map[string]int, 8),
	}
	if !ok {
		diag.ReasonCounts["model-bounds-unavailable"] = 1
		return qtypes.Vec3{}, false, diag
	}

	if spawn := s.findLocalSpawnPoint(); spawn != nil {
		sample := walkableSampleFailure{
			XI: -1,
			YI: -1,
		}
		if pos, ok, sample := validateWalkableSpawnPoint(s, spawn.Origin(s), sample); ok {
			diag.setChosen(sample)
			return pos, true, diag
		} else {
			diag.addFailure(sample)
		}
		return qtypes.Vec3{}, false, diag
	}
	if spawnOrigins := findSpawnOriginsFromEntityLump(s); len(spawnOrigins) > 0 {
		for i, spawnOrigin := range spawnOrigins {
			sample := walkableSampleFailure{
				XI: -2 - i,
				YI: -2 - i,
			}
			pos, ok, validated := validateWalkableSpawnPoint(s, spawnOrigin, sample)
			if ok {
				diag.setChosen(validated)
				return pos, true, diag
			}
			diag.addFailure(validated)
		}
	}

	for xi := 1; xi < 15; xi++ {
		x := mins.X + (maxs.X-mins.X)*(float32(xi)/16)
		for yi := 1; yi < 15; yi++ {
			y := mins.Y + (maxs.Y-mins.Y)*(float32(yi)/16)
			for zi := 0; zi < 16; zi++ {
				z := maxs.Z - (maxs.Z-mins.Z)*(float32(zi)/16) - 8
				sample := walkableSampleFailure{
					XI: xi,
					YI: yi,
				}
				sample.Start = qtypes.Vec3{X: x, Y: y, Z: z}
				sample.End = qtypes.Vec3{X: x, Y: y, Z: mins.Z - 256}
				if pos, ok, sample := validateWalkableSample(s, sample); ok {
					diag.setChosen(sample)
					return pos, true, diag
				} else {
					diag.addFailure(sample)
				}
			}
		}
	}

	return qtypes.Vec3{}, false, diag
}

func newStartMapDiagnosticsServer(t *testing.T) *Server {
	t.Helper()

	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	vfs := fs.NewFileSystem()
	if err := vfs.Init(baseDir, "id1"); err != nil {
		t.Fatalf("init filesystem: %v", err)
	}
	t.Cleanup(vfs.Close)

	s := NewServer()
	newServerTestVM(s, 8)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	progsData, err := vfs.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("load progs.dat: %v", err)
	}
	if err := s.QCVM.LoadProgs(bytes.NewReader(progsData)); err != nil {
		t.Fatalf("LoadProgs: %v", err)
	}
	qc.RegisterBuiltins(s.QCVM)
	if err := s.SpawnServer("start", vfs); err != nil {
		t.Fatalf("spawn server: %v", err)
	}
	return s
}

func createSyntheticSealedRoomWorldModel() *model.Model {
	m := &model.Model{}

	hull := model.Hull{
		Planes: []model.MPlane{
			{Normal: qtypes.Vec3{Z: 1}, Dist: 128, Type: 2},
			{Normal: qtypes.Vec3{Z: 1}, Dist: 0, Type: 2},
		},
		ClipNodes: []model.MClipNode{
			{PlaneNum: 0, Children: [2]int{bsp.ContentsSolid, 1}},
			{PlaneNum: 1, Children: [2]int{bsp.ContentsEmpty, bsp.ContentsSolid}},
		},
		FirstClipNode: 0,
		LastClipNode:  1,
		ClipMins:      qtypes.Vec3{X: -512, Y: -512, Z: -512},
		ClipMaxs:      qtypes.Vec3{X: 512, Y: 512, Z: 512},
	}

	m.Type = model.ModBrush
	m.Hulls[0] = hull
	m.Hulls[1] = hull
	m.Hulls[1].ClipMins = walkablePlayerMins
	m.Hulls[1].ClipMaxs = walkablePlayerMaxs
	m.Hulls[2] = hull
	m.Hulls[2].ClipMins = qtypes.Vec3{X: -32, Y: -32, Z: -24}
	m.Hulls[2].ClipMaxs = qtypes.Vec3{X: 32, Y: 32, Z: 64}
	m.Mins = hull.ClipMins
	m.Maxs = hull.ClipMaxs
	m.ClipBox = true
	m.ClipMins = hull.ClipMins
	m.ClipMaxs = hull.ClipMaxs

	return m
}

func newSyntheticWalkableDiagnosticsServer(t *testing.T) *Server {
	t.Helper()

	s := NewServer()
	newServerTestVM(s, 8)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.WorldModel = createSyntheticSealedRoomWorldModel()
	s.ModelName = "synthetic-room.bsp"
	s.WorldTree = &bsp.Tree{
		Entities: []byte("{\n\"classname\" \"info_player_start\"\n\"origin\" \"32 0 24\"\n}\n"),
	}
	if len(s.Edicts) > 0 && s.Edicts[0] != nil {
		s.Edicts[0].SetSolid(s, float32(SolidBSP))
	}
	s.ClearWorld()
	return s
}

func TestFindWalkablePointFallsBackBelowSolidTopSliceInSyntheticWorld(t *testing.T) {
	s := newSyntheticWalkableDiagnosticsServer(t)

	mins, maxs, ok := s.modelBounds(s.ModelName)
	if !ok {
		t.Fatalf("model bounds unavailable for %q", s.ModelName)
	}
	if got := s.PointContents(qtypes.Vec3{X: 32, Y: 0, Z: maxs.Z - 8}); got != bsp.ContentsSolid {
		t.Fatalf("top-slice contents = %d, want solid", got)
	}

	pos, ok, diag := findWalkablePointWithDiagnostics(s)
	if !ok {
		t.Fatalf("findWalkablePointWithDiagnostics returned no point: %s", diag.String())
	}
	if pos.Z < mins.Z || pos.Z > maxs.Z {
		t.Fatalf("walkable point z=%v out of world bounds %v..%v", pos.Z, mins.Z, maxs.Z)
	}
	if got := s.PointContents(pos); got == bsp.ContentsSolid {
		t.Fatalf("walkable point contents = %d, want non-solid; %s", got, diag.String())
	}
}

func TestFindWalkablePointUsesSpawnpointOnStartMap(t *testing.T) {
	s := newStartMapDiagnosticsServer(t)
	spawn := s.findLocalSpawnPoint()
	if spawn == nil {
		t.Fatal("expected local spawnpoint entity on start map")
	}

	pos, ok, diag := findWalkablePointWithDiagnostics(s)
	if ok != true {
		t.Fatalf("findWalkablePointWithDiagnostics returned no point: %s", diag.String())
	}
	if pos == (qtypes.Vec3{}) {
		t.Fatalf("walkable point = zero vector, want usable spawnpoint-derived position")
	}
	if got := s.PointContents(pos); got == bsp.ContentsSolid {
		t.Fatalf("walkable point contents = %d, want non-solid; %s", got, diag.String())
	}
	if diag.ChosenSample == nil {
		t.Fatalf("expected chosen sample in diagnostics, got: %s", diag.String())
	}
	if diag.ChosenSample.XI != -1 || diag.ChosenSample.YI != -1 {
		t.Fatalf("expected helper to use findLocalSpawnPoint first, got chosen sample %+v; %s", *diag.ChosenSample, diag.String())
	}
	if diag.ChosenSample.Start.X != spawn.Origin(s).X || diag.ChosenSample.Start.Y != spawn.Origin(s).Y {
		t.Fatalf("chosen sample start = %v, want spawnpoint x/y %v; %s", diag.ChosenSample.Start, spawn.Origin(s), diag.String())
	}

	ent := allocPhysicsTestEdict(s)
	ent.SetOrigin(s, pos)
	ent.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -24})
	ent.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 32})
	ent.SetSolid(s, float32(SolidSlideBox))
	s.LinkEdict(ent, false)
	if blocker := s.SV_TestEntityPosition(ent); blocker != nil {
		t.Fatalf("spawnpoint-derived walkable point blocked by %+v; %s", blocker, diag.String())
	}
}

func TestFindWalkablePointTriesMultipleEntityLumpSpawnCandidates(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 8)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.WorldModel = createSyntheticSealedRoomWorldModel()
	s.ModelName = "synthetic-room.bsp"
	s.WorldTree = &bsp.Tree{
		Entities: []byte(`{
"classname" "info_player_start"
"origin" "0 0 256"
}
{
"classname" "info_player_coop"
"origin" "32 0 24"
}`),
	}
	if len(s.Edicts) > 0 && s.Edicts[0] != nil {
		s.Edicts[0].SetSolid(s, float32(SolidBSP))
	}
	s.ClearWorld()

	pos, ok, diag := findWalkablePointWithDiagnostics(s)
	if !ok {
		t.Fatalf("findWalkablePointWithDiagnostics returned no point: %s", diag.String())
	}
	if diag.SamplesTried < 2 {
		t.Fatalf("expected multiple candidate attempts, got diagnostics: %s", diag.String())
	}
	if got := s.PointContents(pos); got == bsp.ContentsSolid {
		t.Fatalf("walkable point contents = %d, want non-solid; %s", got, diag.String())
	}
}

func TestFindWalkablePointFallsBackAcrossEntityLumpSpawnCandidatesWithoutPakAssets(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 8)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.WorldModel = createSyntheticSealedRoomWorldModel()
	s.ModelName = "synthetic-room.bsp"
	s.WorldTree = &bsp.Tree{
		Entities: []byte(`{
"classname" "info_player_start"
"origin" "0 0 256"
}
{
"classname" "testplayerstart"
"origin" "32 0 24"
}
{
"classname" "info_player_coop"
"origin" "64 0 24"
}`),
	}
	if len(s.Edicts) > 0 && s.Edicts[0] != nil {
		s.Edicts[0].SetSolid(s, float32(SolidBSP))
	}
	s.ClearWorld()

	pos, ok, diag := findWalkablePointWithDiagnostics(s)
	if ok != true {
		t.Fatalf("findWalkablePointWithDiagnostics returned no point: %s", diag.String())
	}
	if pos.X != 32 || pos.Y != 0 {
		t.Fatalf("findWalkablePointWithDiagnostics() = %v, want fallback on second spawn column; %s", pos, diag.String())
	}
	if pos.Z < -24 || pos.Z > 32 {
		t.Fatalf("findWalkablePointWithDiagnostics() z=%v, want standing height near second spawn column; %s", pos.Z, diag.String())
	}
	if diag.SamplesTried < 2 || len(diag.FailedSamples) == 0 {
		t.Fatalf("expected at least one rejected earlier spawn candidate, got diagnostics: %s", diag.String())
	}
	if diag.ChosenSample == nil {
		t.Fatalf("expected chosen sample in diagnostics, got: %s", diag.String())
	}
	if diag.ChosenSample.Start.X != 32 || diag.ChosenSample.Start.Y != 0 {
		t.Fatalf("chosen sample start = %v, want second spawn column; %s", diag.ChosenSample.Start, diag.String())
	}
}

func TestStartMapTopSliceSamplesSolid(t *testing.T) {
	s := newStartMapDiagnosticsServer(t)

	mins, maxs, ok := s.modelBounds(s.ModelName)
	if !ok {
		t.Fatalf("model bounds unavailable for %q", s.ModelName)
	}

	for _, sample := range [][2]int{{1, 1}, {8, 8}, {14, 14}} {
		x := mins.X + (maxs.X-mins.X)*(float32(sample[0])/16)
		y := mins.Y + (maxs.Y-mins.Y)*(float32(sample[1])/16)
		start := qtypes.Vec3{X: x, Y: y, Z: maxs.Z - 8}
		if got := s.PointContents(start); got != bsp.ContentsSolid {
			t.Fatalf("PointContents(%v) = %d, want solid top-slice sample", start, got)
		}
	}
}

func TestStartMapSpawnColumnFindsFloorWithPlayerHull(t *testing.T) {
	s := newStartMapDiagnosticsServer(t)

	spawnOrigin, ok := findSpawnOriginFromEntityLump(s)
	if !ok {
		t.Fatal("spawn origin unavailable from entity lump")
	}
	worldMins, _, ok := s.modelBounds(s.ModelName)
	if !ok {
		t.Fatalf("model bounds unavailable for %q", s.ModelName)
	}

	start := spawnOrigin
	start.Z += stepSize
	end := qtypes.Vec3{X: start.X, Y: start.Y, Z: worldMins.Z - 256}

	trace := s.Move(start, walkablePlayerMins, walkablePlayerMaxs, end, MoveNormal, nil)
	if trace.StartSolid {
		t.Fatalf("spawn-column trace started solid: start=%v trace=%+v", start, trace)
	}
	if trace.AllSolid {
		t.Fatalf("spawn-column trace stayed allsolid: start=%v end=%v trace=%+v", start, end, trace)
	}
	if trace.Fraction == 1 {
		t.Fatalf("spawn-column failed to hit floor with player hull: start=%v end=%v trace=%+v", start, end, trace)
	}
	if got := s.PointContents(trace.EndPos); got == bsp.ContentsSolid {
		t.Fatalf("spawn-column end position contents = %d, want non-solid; trace=%+v", got, trace)
	}
}
