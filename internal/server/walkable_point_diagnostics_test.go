package server

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/model"
	"github.com/ironwail/ironwail-go/internal/qc"
	"github.com/ironwail/ironwail-go/internal/testutil"
)

const maxWalkableFailureSamples = 8

var (
	walkablePlayerMins = [3]float32{-16, -16, -24}
	walkablePlayerMaxs = [3]float32{16, 16, 32}
)

type walkableHullSummary struct {
	Index         int
	FirstClipNode int
	LastClipNode  int
	ClipNodeCount int
	PlaneCount    int
	ClipMins      [3]float32
	ClipMaxs      [3]float32
}

type walkableWorldSummary struct {
	ModelName   string
	BoundsMin   [3]float32
	BoundsMax   [3]float32
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
	Start           [3]float32
	StartContents   int
	End             [3]float32
	TraceFraction   float32
	TraceStartSolid bool
	TraceAllSolid   bool
	TraceEndPos     [3]float32
	Lifted          [3]float32
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
			hull.ClipMins[0], hull.ClipMins[1], hull.ClipMins[2],
			hull.ClipMaxs[0], hull.ClipMaxs[1], hull.ClipMaxs[2]))
	}

	sb := strings.Builder{}
	fmt.Fprintf(&sb, "world model=%q bounds=(%.1f %.1f %.1f)->(%.1f %.1f %.1f) tree(models=%d nodes=%d leafs=%d faces=%d) headnodes=%v collisionModel=%v hulls=[%s] samples=%d reasons={%s}",
		d.World.ModelName,
		d.World.BoundsMin[0], d.World.BoundsMin[1], d.World.BoundsMin[2],
		d.World.BoundsMax[0], d.World.BoundsMax[1], d.World.BoundsMax[2],
		d.World.TreeModels, d.World.TreeNodes, d.World.TreeLeafs, d.World.TreeFaces,
		d.World.HeadNodes, d.World.CollisionOK, strings.Join(hulls, "; "),
		d.SamplesTried, strings.Join(reasons, ", "))

	if d.ChosenSample != nil {
		chosen := d.ChosenSample
		fmt.Fprintf(&sb, " chosen=[xi=%d yi=%d start=(%.1f %.1f %.1f) end=(%.1f %.1f %.1f) traceEnd=(%.1f %.1f %.1f) lifted=(%.1f %.1f %.1f)]",
			chosen.XI, chosen.YI,
			chosen.Start[0], chosen.Start[1], chosen.Start[2],
			chosen.End[0], chosen.End[1], chosen.End[2],
			chosen.TraceEndPos[0], chosen.TraceEndPos[1], chosen.TraceEndPos[2],
			chosen.Lifted[0], chosen.Lifted[1], chosen.Lifted[2])
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
				failure.Start[0], failure.Start[1], failure.Start[2],
				failure.StartContents,
				failure.TraceFraction,
				failure.TraceStartSolid,
				failure.TraceAllSolid,
				failure.Lifted[0], failure.Lifted[1], failure.Lifted[2],
				failure.LiftedContents)
		}
		sb.WriteString("]")
	}
	return sb.String()
}

func collectWalkableWorldSummary(s *Server, mins, maxs [3]float32) walkableWorldSummary {
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

func validateWalkableSample(s *Server, sample walkableSampleFailure) ([3]float32, bool, walkableSampleFailure) {
	sample.StartContents = s.PointContents(sample.Start)
	if sample.StartContents == bsp.ContentsSolid {
		sample.Reason = "start-in-solid"
		return [3]float32{}, false, sample
	}

	trace := s.Move(sample.Start, walkablePlayerMins, walkablePlayerMaxs, sample.End, MoveNormal, nil)
	sample.TraceFraction = trace.Fraction
	sample.TraceStartSolid = trace.StartSolid
	sample.TraceAllSolid = trace.AllSolid
	sample.TraceEndPos = trace.EndPos
	if trace.StartSolid {
		sample.Reason = "trace-startsolid"
		return [3]float32{}, false, sample
	}
	if trace.AllSolid {
		sample.Reason = "trace-allsolid"
		return [3]float32{}, false, sample
	}
	if trace.Fraction == 1 {
		sample.Reason = "trace-no-floor-hit"
		return [3]float32{}, false, sample
	}

	return validateWalkableStandingOrigin(s, trace.EndPos, sample)
}

func validateWalkableStandingOrigin(s *Server, origin [3]float32, sample walkableSampleFailure) ([3]float32, bool, walkableSampleFailure) {
	sample.Lifted = origin
	sample.LiftedContents = s.PointContents(origin)
	if sample.LiftedContents != bsp.ContentsEmpty {
		sample.Reason = "lifted-point-not-empty"
		return [3]float32{}, false, sample
	}

	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.Origin = origin
	ent.Vars.Mins = walkablePlayerMins
	ent.Vars.Maxs = walkablePlayerMaxs
	ent.Vars.Size = [3]float32{32, 32, 56}
	ent.Vars.Solid = float32(SolidSlideBox)
	ent.Vars.AbsMin = [3]float32{origin[0] - 16, origin[1] - 16, origin[2] - 24}
	ent.Vars.AbsMax = [3]float32{origin[0] + 16, origin[1] + 16, origin[2] + 32}
	if blocker := s.SV_TestEntityPosition(ent); blocker != nil {
		sample.Reason = "lifted-entity-blocked"
		return [3]float32{}, false, sample
	}
	if !s.CheckBottom(ent) {
		sample.Reason = "lifted-no-bottom"
		return [3]float32{}, false, sample
	}
	if supportsStationaryMoveStep(s, origin) {
		return origin, true, sample
	}
	sample.Reason = "lifted-not-stationary"
	return [3]float32{}, false, sample
}

func validateWalkableStandingPoint(s *Server, origin [3]float32, sample walkableSampleFailure) ([3]float32, bool, walkableSampleFailure) {
	sample.Start = origin
	sample.StartContents = s.PointContents(origin)
	if sample.StartContents == bsp.ContentsSolid {
		sample.Reason = "start-in-solid"
		return [3]float32{}, false, sample
	}
	if pos, ok, sample := validateWalkableStandingOrigin(s, origin, sample); ok {
		return pos, true, sample
	}
	return validateWalkableSpawnPoint(s, origin, sample)
}

func validateWalkableSpawnPoint(s *Server, origin [3]float32, sample walkableSampleFailure) ([3]float32, bool, walkableSampleFailure) {
	worldMins, _, ok := s.modelBounds(s.ModelName)
	if !ok {
		sample.Reason = "model-bounds-unavailable"
		return [3]float32{}, false, sample
	}

	var lastFailure walkableSampleFailure
	for _, zOffset := range []float32{0, 1, stepSize, 24, 32, 48, 64} {
		start := origin
		start[2] += zOffset
		sample.Start = start
		sample.End = [3]float32{start[0], start[1], worldMins[2] - 256}
		if pos, ok, validated := validateWalkableSample(s, sample); ok {
			return pos, true, validated
		} else {
			lastFailure = validated
		}
	}

	if lastFailure.Reason == "" {
		sample.Reason = "spawn-no-floor"
		return [3]float32{}, false, sample
	}
	return [3]float32{}, false, lastFailure
}

func findGroundedWalkableNearOrigin(s *Server, origin [3]float32, sample walkableSampleFailure) ([3]float32, bool, walkableSampleFailure) {
	worldMins, _, _ := s.modelBounds(s.ModelName)
	endZ := worldMins[2] - 256
	standingZOffsets := []float32{0, 1, -1, 8, -8, 16, -16, 24, -24}
	for radius := float32(0); radius <= 192; radius += 8 {
		for dx := -radius; dx <= radius; dx += 8 {
			for dy := -radius; dy <= radius; dy += 8 {
				if radius != 0 {
					onEdgeX := dx == -radius || dx == radius
					onEdgeY := dy == -radius || dy == radius
					if !onEdgeX && !onEdgeY {
						continue
					}
				}
				for _, standingZOffset := range standingZOffsets {
					candidate := [3]float32{origin[0] + dx, origin[1] + dy, origin[2] + standingZOffset}
					sample.Start = candidate
					if pos, ok, sample := validateWalkableStandingOrigin(s, candidate, sample); ok {
						return pos, true, sample
					}
				}
			}
		}
	}

	startZOffsets := []float32{48, 64, 96, 128, 192, 256, 320}
	for radius := float32(0); radius <= 192; radius += 16 {
		for dx := -radius; dx <= radius; dx += 16 {
			for dy := -radius; dy <= radius; dy += 16 {
				if radius != 0 {
					onEdgeX := dx == -radius || dx == radius
					onEdgeY := dy == -radius || dy == radius
					if !onEdgeX && !onEdgeY {
						continue
					}
				}
				for _, startZOffset := range startZOffsets {
					start := [3]float32{origin[0] + dx, origin[1] + dy, origin[2] + startZOffset}
					sample.Start = start
					sample.End = [3]float32{start[0], start[1], endZ}
					if pos, ok, sample := validateWalkableSample(s, sample); ok {
						return pos, true, sample
					}
				}
			}
		}
	}

	sample.Reason = "spawn-no-floor"
	return [3]float32{}, false, sample
}

func supportsStationaryMoveStep(s *Server, pos [3]float32) bool {
	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.Origin = pos
	ent.Vars.Mins = walkablePlayerMins
	ent.Vars.Maxs = walkablePlayerMaxs
	ent.Vars.Size = [3]float32{32, 32, 56}
	ent.Vars.Solid = float32(SolidSlideBox)
	ent.Vars.MoveType = float32(MoveTypeStep)
	ent.Vars.Flags = float32(FlagOnGround)
	ent.Vars.AbsMin = [3]float32{pos[0] - 16, pos[1] - 16, pos[2] - 24}
	ent.Vars.AbsMax = [3]float32{pos[0] + 16, pos[1] + 16, pos[2] + 32}
	s.Edicts = append(s.Edicts, ent)
	s.NumEdicts = len(s.Edicts)
	defer func() {
		UnlinkEdict(ent)
		s.Edicts = s.Edicts[:len(s.Edicts)-1]
		s.NumEdicts = len(s.Edicts)
	}()
	s.LinkEdict(ent, false)
	if blocker := s.SV_TestEntityPosition(ent); blocker != nil {
		return false
	}
	original := ent.Vars.Origin
	if !s.MoveStep(ent, [3]float32{}, true) {
		return false
	}
	return ent.Vars.Origin == original
}

var quotedEntityPairRE = regexp.MustCompile(`"([^"]+)"\s+"([^"]*)"`)

func findSpawnOriginsFromEntityLump(s *Server) [][3]float32 {
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
	origins := make([][3]float32, 0, len(blocks))
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

func findSpawnOriginFromEntityLump(s *Server) ([3]float32, bool) {
	origins := findSpawnOriginsFromEntityLump(s)
	if len(origins) == 0 {
		return [3]float32{}, false
	}
	return origins[0], true
}

func TestFindSpawnOriginFromEntityLumpParsesInfoPlayerStart(t *testing.T) {
	s := NewServer()
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
	if want := [3]float32{544, 288, 32}; got != want {
		t.Fatalf("findSpawnOriginFromEntityLump() = %v, want %v", got, want)
	}
}

func TestFindSpawnOriginsFromEntityLumpEnumeratesSupportedSpawnClasses(t *testing.T) {
	s := NewServer()
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
	want := [][3]float32{
		{544, 288, 32},
		{1, 2, 3},
		{10, 11, 12},
		{4, 5, 6},
		{7, 8, 9},
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

func findWalkablePointWithDiagnostics(s *Server) ([3]float32, bool, walkablePointDiagnostics) {
	mins, maxs, ok := s.modelBounds(s.ModelName)
	diag := walkablePointDiagnostics{
		World:        collectWalkableWorldSummary(s, mins, maxs),
		ReasonCounts: make(map[string]int, 8),
	}
	if !ok {
		diag.ReasonCounts["model-bounds-unavailable"] = 1
		return [3]float32{}, false, diag
	}

	if spawn := s.findLocalSpawnPoint(); spawn != nil && spawn.Vars != nil {
		sample := walkableSampleFailure{
			XI: -1,
			YI: -1,
		}
		if pos, ok, sample := validateWalkableSpawnPoint(s, spawn.Vars.Origin, sample); ok {
			diag.setChosen(sample)
			return pos, true, diag
		} else {
			diag.addFailure(sample)
		}
		return [3]float32{}, false, diag
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
		x := mins[0] + (maxs[0]-mins[0])*(float32(xi)/16)
		for yi := 1; yi < 15; yi++ {
			y := mins[1] + (maxs[1]-mins[1])*(float32(yi)/16)
			for zi := 0; zi < 16; zi++ {
				z := maxs[2] - (maxs[2]-mins[2])*(float32(zi)/16) - 8
				sample := walkableSampleFailure{
					XI: xi,
					YI: yi,
				}
				sample.Start = [3]float32{x, y, z}
				sample.End = [3]float32{x, y, mins[2] - 256}
				if pos, ok, sample := validateWalkableSample(s, sample); ok {
					diag.setChosen(sample)
					return pos, true, diag
				} else {
					diag.addFailure(sample)
				}
			}
		}
	}

	return [3]float32{}, false, diag
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
			{Normal: [3]float32{0, 0, 1}, Dist: 128, Type: 2},
			{Normal: [3]float32{0, 0, 1}, Dist: 0, Type: 2},
		},
		ClipNodes: []model.MClipNode{
			{PlaneNum: 0, Children: [2]int{bsp.ContentsSolid, 1}},
			{PlaneNum: 1, Children: [2]int{bsp.ContentsEmpty, bsp.ContentsSolid}},
		},
		FirstClipNode: 0,
		LastClipNode:  1,
		ClipMins:      [3]float32{-512, -512, -512},
		ClipMaxs:      [3]float32{512, 512, 512},
	}

	m.Type = model.ModBrush
	m.Hulls[0] = hull
	m.Hulls[1] = hull
	m.Hulls[1].ClipMins = walkablePlayerMins
	m.Hulls[1].ClipMaxs = walkablePlayerMaxs
	m.Hulls[2] = hull
	m.Hulls[2].ClipMins = [3]float32{-32, -32, -24}
	m.Hulls[2].ClipMaxs = [3]float32{32, 32, 64}
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
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.WorldModel = createSyntheticSealedRoomWorldModel()
	s.ModelName = "synthetic-room.bsp"
	s.WorldTree = &bsp.Tree{
		Entities: []byte("{\n\"classname\" \"info_player_start\"\n\"origin\" \"32 0 24\"\n}\n"),
	}
	if len(s.Edicts) > 0 && s.Edicts[0] != nil && s.Edicts[0].Vars != nil {
		s.Edicts[0].Vars.Solid = float32(SolidBSP)
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
	if got := s.PointContents([3]float32{32, 0, maxs[2] - 8}); got != bsp.ContentsSolid {
		t.Fatalf("top-slice contents = %d, want solid", got)
	}

	pos, ok, diag := findWalkablePointWithDiagnostics(s)
	if !ok {
		t.Fatalf("findWalkablePointWithDiagnostics returned no point: %s", diag.String())
	}
	if pos[2] < mins[2] || pos[2] > maxs[2] {
		t.Fatalf("walkable point z=%v out of world bounds %v..%v", pos[2], mins[2], maxs[2])
	}
	if got := s.PointContents(pos); got == bsp.ContentsSolid {
		t.Fatalf("walkable point contents = %d, want non-solid; %s", got, diag.String())
	}
}

func TestFindWalkablePointUsesSpawnpointOnStartMap(t *testing.T) {
	s := newStartMapDiagnosticsServer(t)
	spawn := s.findLocalSpawnPoint()
	if spawn == nil || spawn.Vars == nil {
		t.Fatal("expected local spawnpoint entity on start map")
	}

	pos, ok, diag := findWalkablePointWithDiagnostics(s)
	if ok != true {
		t.Fatalf("findWalkablePointWithDiagnostics returned no point: %s", diag.String())
	}
	if pos == ([3]float32{}) {
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
	if diag.ChosenSample.Start[0] != spawn.Vars.Origin[0] || diag.ChosenSample.Start[1] != spawn.Vars.Origin[1] {
		t.Fatalf("chosen sample start = %v, want spawnpoint x/y %v; %s", diag.ChosenSample.Start, spawn.Vars.Origin, diag.String())
	}

	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.Origin = pos
	ent.Vars.Mins = [3]float32{-16, -16, -24}
	ent.Vars.Maxs = [3]float32{16, 16, 32}
	ent.Vars.Solid = float32(SolidSlideBox)
	s.LinkEdict(ent, false)
	if blocker := s.SV_TestEntityPosition(ent); blocker != nil {
		t.Fatalf("spawnpoint-derived walkable point blocked by %+v; %s", blocker, diag.String())
	}
}

func TestFindWalkablePointTriesMultipleEntityLumpSpawnCandidates(t *testing.T) {
	s := NewServer()
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
	if len(s.Edicts) > 0 && s.Edicts[0] != nil && s.Edicts[0].Vars != nil {
		s.Edicts[0].Vars.Solid = float32(SolidBSP)
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
	if len(s.Edicts) > 0 && s.Edicts[0] != nil && s.Edicts[0].Vars != nil {
		s.Edicts[0].Vars.Solid = float32(SolidBSP)
	}
	s.ClearWorld()

	pos, ok, diag := findWalkablePointWithDiagnostics(s)
	if ok != true {
		t.Fatalf("findWalkablePointWithDiagnostics returned no point: %s", diag.String())
	}
	if pos[0] != 32 || pos[1] != 0 {
		t.Fatalf("findWalkablePointWithDiagnostics() = %v, want fallback on second spawn column; %s", pos, diag.String())
	}
	if pos[2] < -24 || pos[2] > 32 {
		t.Fatalf("findWalkablePointWithDiagnostics() z=%v, want standing height near second spawn column; %s", pos[2], diag.String())
	}
	if diag.SamplesTried < 2 || len(diag.FailedSamples) == 0 {
		t.Fatalf("expected at least one rejected earlier spawn candidate, got diagnostics: %s", diag.String())
	}
	if diag.ChosenSample == nil {
		t.Fatalf("expected chosen sample in diagnostics, got: %s", diag.String())
	}
	if diag.ChosenSample.Start[0] != 32 || diag.ChosenSample.Start[1] != 0 {
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
		x := mins[0] + (maxs[0]-mins[0])*(float32(sample[0])/16)
		y := mins[1] + (maxs[1]-mins[1])*(float32(sample[1])/16)
		start := [3]float32{x, y, maxs[2] - 8}
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
	start[2] += stepSize
	end := [3]float32{start[0], start[1], worldMins[2] - 256}

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
