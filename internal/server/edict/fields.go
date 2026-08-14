package edict

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/darkliquid/ironwail-go/internal/qc"
	types "github.com/darkliquid/ironwail-go/internal/server/types"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

type fieldDefInfo struct {
	ofs   int
	eType qc.EType
}

// fieldDef looks up a QC field definition by name and returns both its
// offset in the VM edict data and its declared type. Matching is
// case-insensitive and underscore-insensitive to mirror Quake's field lookup.
func (em *Manager) fieldDef(keyName string) (int, qc.EType, bool) {
	if em == nil || em.vm == nil {
		return 0, 0, false
	}
	if em.fieldDefMap == nil {
		em.fieldDefMap = make(map[string]fieldDefInfo, len(em.vm.FieldDefs))
		for _, def := range em.vm.FieldDefs {
			norm := normalizeFieldName(em.vm.String(def.Name))
			em.fieldDefMap[norm] = fieldDefInfo{
				ofs:   int(def.Ofs),
				eType: qc.EType(def.Type &^ qc.DefSaveGlobal),
			}
		}
	}
	normalized := normalizeFieldName(keyName)
	if info, ok := em.fieldDefMap[normalized]; ok {
		return info.ofs, info.eType, true
	}

	// Fallback to default offsets for standard entvars fields when
	// FieldDefs doesn't contain the field (e.g. minimal test VMs).
	if ofs, ok := em.defaultOffsets[normalized]; ok {
		fieldType := qc.EvFloat
		if _, isString := StringEntFieldNames[normalized]; isString {
			fieldType = qc.EvString
		} else if isVectorField(normalized) {
			fieldType = qc.EvVector
		}
		return ofs, fieldType, true
	}
	return 0, 0, false
}

// isVectorField returns true for standard entvars fields that are vec3 types.
var vectorEntFieldNames = map[string]struct{}{
	normalizeFieldName("AbsMin"):     {},
	normalizeFieldName("AbsMax"):     {},
	normalizeFieldName("Origin"):     {},
	normalizeFieldName("OldOrigin"):  {},
	normalizeFieldName("Velocity"):   {},
	normalizeFieldName("Angles"):     {},
	normalizeFieldName("AVelocity"):  {},
	normalizeFieldName("PunchAngle"): {},
	normalizeFieldName("Mins"):       {},
	normalizeFieldName("Maxs"):       {},
	normalizeFieldName("Size"):       {},
	normalizeFieldName("ViewOfs"):    {},
	normalizeFieldName("VAngle"):     {},
	normalizeFieldName("MoveDir"):    {},
}

func isVectorField(normalized string) bool {
	_, ok := vectorEntFieldNames[normalized]
	return ok
}

var (
	normalizedNamesMu    sync.RWMutex
	normalizedNamesCache = make(map[string]string, 512)
)

// normalizeFieldName strips underscores and lowercases the input string to
// produce a canonical form suitable for case-insensitive,
// underscore-insensitive field name matching. This mirrors the original
// Quake engine's forgiving key-name lookup, allowing map editors and mods
// to use "ClassName", "classname", "class_name", etc. interchangeably.
func normalizeFieldName(name string) string {
	normalizedNamesMu.RLock()
	if n, ok := normalizedNamesCache[name]; ok {
		normalizedNamesMu.RUnlock()
		return n
	}
	normalizedNamesMu.RUnlock()

	n := strings.ToLower(strings.ReplaceAll(name, "_", ""))
	normalizedNamesMu.Lock()
	normalizedNamesCache[name] = n
	normalizedNamesMu.Unlock()
	return n
}

var vec3Replacer = strings.NewReplacer("\t", " ", "\n", " ", "\r", " ")

// parseVec3 parses a space-separated "x y z" string (as found in map entity
// definitions) into a types.Vec3 vector. Quake's entity parser is lenient
// here: missing or empty components decode as 0 and extra components are
// ignored, matching the original C atof-based parsing.
func parseVec3(raw string) (qtypes.Vec3, error) {
	var out qtypes.Vec3
	normalized := vec3Replacer.Replace(strings.TrimSpace(raw))
	if normalized == "" {
		return out, nil
	}
	parts := strings.Split(normalized, " ")
	component := 0
	for _, part := range parts {
		if component >= 3 {
			break
		}
		if part == "" {
			component++
			continue
		}
		v, err := strconv.ParseFloat(part, 32)
		if err != nil {
			return qtypes.Vec3{}, err
		}
		switch component {
		case 0:
			out.X = float32(v)
		case 1:
			out.Y = float32(v)
		case 2:
			out.Z = float32(v)
		}
		component++
	}
	return out, nil
}

// parseFloat32 parses a single string token into a float32, trimming
// surrounding whitespace first. Used for scalar entity fields such as
// health, speed, and delay that are stored as float32 in EntVars. Empty
// values decode to 0 to match Quake's atof-based entity parsing.
func parseFloat32(raw string) (float32, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	v, err := strconv.ParseFloat(raw, 32)
	if err != nil {
		return 0, err
	}
	return float32(v), nil
}

// parseInt32 parses a string into a base-10 int32 value, trimming
// surrounding whitespace. Used for integer-valued entity fields such as
// entity numbers (EvEntity), function indices (EvFunction), spawnflags,
// and bit-flag fields. Empty values decode to 0 to match Quake's atoi-based
// entity parsing.
func parseInt32(raw string) (int32, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(v), nil
}

// parseEdictFieldValue sets a single field on an edict's EntVars from a
// key-value pair read from map entity data. It normalises the key, looks up
// the matching struct field index in entVarsFieldIndex, and uses reflection
// to write the parsed value.
//
// Type-specific handling:
//   - float32 fields: parsed directly as floats via parseFloat32.
//   - int32 fields: dispatched through several layers of type resolution:
//     1. If the field is in StringEntFieldNames, the value is allocated in the
//     VM string table and the returned index is stored.
//     2. Otherwise, the VM's compiled field definitions are consulted via
//     fieldType. Depending on the QC type (EvString, EvField, EvFunction,
//     EvEntity, etc.) the value is resolved appropriately (string alloc,
//     field offset lookup, function index lookup, or integer parse).
//     3. As a last resort, the value is parsed as a plain integer; if that
//     also fails, parseStringFallbackInt32 produces a deterministic FNV-1a
//     hash so the field is never left uninitialised.
//   - types.Vec3: parsed as "x y z" vec3 via parseVec3.
//
// After setting either the "mins" or "maxs" field, the entity's Size vector
// is automatically recalculated as (Maxs - Mins) on each axis. This keeps
// Size consistent whenever a bounding box corner changes, which is required
// by the physics code (SV_Physics, SV_ClipMoveToEntity) that relies on Size
// being the delta (Maxs − Mins) and does not recompute it on the fly.
func (em *Manager) parseEdictFieldValue(edict *types.Edict, entNum int, keyName, value string) error {
	if edict == nil {
		return fmt.Errorf("nil edict")
	}

	// Always parse into QCVM edict storage if VM is available
	if err := em.parseQCVMEdictFieldValue(entNum, keyName, value); err != nil {
		return err
	}

	// Recalculate entity bounding-box Size whenever either corner changes.
	// The physics code (SV_Physics, SV_ClipMoveToEntity) relies on Size
	// being the delta (Maxs − Mins) and does not recompute it on the fly.
	if normalizeFieldName(keyName) == "mins" || normalizeFieldName(keyName) == "maxs" {
		if em.vm != nil && em.vm.EdictSize > 28 {
			mins := em.vm.EVector(edict.Num, qc.EntFieldMins)
			maxs := em.vm.EVector(edict.Num, qc.EntFieldMaxs)
			em.vm.SetEVector(edict.Num, qc.EntFieldSize, maxs.Sub(mins))
		}
	}

	return nil
}

func (em *Manager) parseQCVMEdictFieldValue(entNum int, keyName, value string) error {
	if em == nil || em.vm == nil {
		return nil
	}

	fieldOfs, fieldType, ok := em.fieldDef(keyName)
	if !ok {
		return nil
	}

	switch fieldType {
	case qc.EvString:
		em.vm.SetEString(entNum, fieldOfs, em.vm.AllocString(value))
	case qc.EvFloat:
		f, err := parseFloat32(value)
		if err != nil {
			return err
		}
		em.vm.SetEFloat(entNum, fieldOfs, f)
	case qc.EvVector:
		vec, err := parseVec3(value)
		if err != nil {
			return err
		}
		em.vm.SetEVector(entNum, fieldOfs, vec)
	case qc.EvField:
		if resolvedOfs := em.vm.FindField(value); resolvedOfs >= 0 {
			em.vm.SetEInt(entNum, fieldOfs, int32(resolvedOfs))
		}
	case qc.EvFunction:
		if funcNum := em.vm.FindFunction(value); funcNum >= 0 {
			em.vm.SetEInt(entNum, fieldOfs, int32(funcNum))
		}
	case qc.EvEntity, qc.EvPointer, qc.EvExtInteger:
		i, err := parseInt32(value)
		if err != nil {
			return err
		}
		em.vm.SetEInt(entNum, fieldOfs, i)
	}

	return nil
}

// parseGlobalValue sets a single QuakeC global variable from a key-value
// pair encountered during map or savegame loading. It scans the VM's
// GlobalDefs for a matching name and dispatches by the declared QC type:
//   - EvVector: parsed as "x y z" and written via SetGVector.
//   - EvString: allocated in the VM string table and stored as an index.
//   - EvEntity / EvField / EvFunction / EvPointer / EvExtInteger: parsed as
//     a plain int32 and stored via SetGInt.
//   - Everything else (typically EvFloat): parsed as float32 via SetGFloat.
//
// Unrecognised key names are silently ignored; parse errors cause the
// key to be skipped rather than aborting the entire load, matching the
// original engine's lenient behaviour.
func (em *Manager) parseGlobalValue(vm *qc.VM, keyName, value string) {
	for _, def := range vm.GlobalDefs {
		if vm.String(def.Name) != keyName {
			continue
		}

		ofs := int(def.Ofs)
		etype := qc.EType(def.Type &^ qc.DefSaveGlobal)

		switch etype {
		case qc.EvVector:
			vec, err := parseVec3(value)
			if err != nil {
				return
			}
			vm.SetGVector(ofs, vec)
		case qc.EvString:
			vm.SetGInt(ofs, vm.AllocString(value))
		case qc.EvEntity, qc.EvField, qc.EvFunction, qc.EvPointer, qc.EvExtInteger:
			i, err := parseInt32(value)
			if err != nil {
				return
			}
			vm.SetGInt(ofs, i)
		default:
			f, err := parseFloat32(value)
			if err != nil {
				return
			}
			vm.SetGFloat(ofs, f)
		}

		return
	}
}
