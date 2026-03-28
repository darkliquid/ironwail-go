package compiler

import (
	"fmt"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

type builtinNameRegistration struct {
	Name      string
	Number    int
	Canonical bool
}

type builtinNameRegistry struct {
	byName            map[string]int
	byNumber          map[int]string
	canonicalByNumber map[int]bool
}

func mustNewBuiltinNameRegistry(entries []builtinNameRegistration) builtinNameRegistry {
	registry, err := newBuiltinNameRegistry(entries)
	if err != nil {
		panic(err)
	}
	return registry
}

func newBuiltinNameRegistry(entries []builtinNameRegistration) (builtinNameRegistry, error) {
	registry := builtinNameRegistry{
		byName:            make(map[string]int, len(entries)),
		byNumber:          make(map[int]string, len(entries)),
		canonicalByNumber: make(map[int]bool, len(entries)),
	}

	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			return builtinNameRegistry{}, fmt.Errorf("builtin registry entry missing name for builtin %d", entry.Number)
		}
		if entry.Number <= 0 || entry.Number >= qc.MaxBuiltins {
			return builtinNameRegistry{}, fmt.Errorf("builtin registry entry %q has builtin %d outside valid range 1..%d", name, entry.Number, qc.MaxBuiltins-1)
		}

		normalized := strings.ToLower(name)
		if existing, exists := registry.byName[normalized]; exists {
			return builtinNameRegistry{}, fmt.Errorf("builtin registry entry %q duplicates builtin %d already registered for name %q", name, entry.Number, registry.byNumber[existing])
		}
		if existing, exists := registry.byNumber[entry.Number]; exists {
			if entry.Canonical {
				return builtinNameRegistry{}, fmt.Errorf("builtin registry entry %q duplicates builtin %d already registered for canonical name %q", name, entry.Number, existing)
			}
			if !registry.canonicalByNumber[entry.Number] {
				return builtinNameRegistry{}, fmt.Errorf("builtin registry entry %q duplicates builtin %d already registered for name %q", name, entry.Number, existing)
			}
			registry.byName[normalized] = entry.Number
			continue
		}

		registry.byName[normalized] = entry.Number
		registry.byNumber[entry.Number] = name
		registry.canonicalByNumber[entry.Number] = entry.Canonical
	}

	return registry, nil
}

func (r builtinNameRegistry) numberForName(name string) (int, bool) {
	number, ok := r.byName[strings.ToLower(strings.TrimSpace(name))]
	return number, ok
}

func (r builtinNameRegistry) nameForNumber(number int) (string, bool) {
	name, ok := r.byNumber[number]
	return name, ok
}

var builtinDirectiveRegistry = mustNewBuiltinNameRegistry([]builtinNameRegistration{
	{Name: "setorigin", Number: 2, Canonical: true},
	{Name: "setmodel", Number: 3, Canonical: true},
	{Name: "setsize", Number: 4, Canonical: true},
	{Name: "sound", Number: 8, Canonical: true},
	{Name: "spawn", Number: 14, Canonical: true},
	{Name: "remove", Number: 15, Canonical: true},
	{Name: "traceline", Number: 16, Canonical: true},
	{Name: "checkclient", Number: 17, Canonical: true},
	{Name: "find", Number: 18, Canonical: true},
	{Name: "precache_sound", Number: 19, Canonical: true},
	{Name: "precache_model", Number: 20, Canonical: true},
	{Name: "stuffcmd", Number: 21, Canonical: true},
	{Name: "findradius", Number: 22, Canonical: true},
	{Name: "bprint", Number: 23, Canonical: true},
	{Name: "sprint", Number: 24, Canonical: true},
	{Name: "print", Number: 24},
	{Name: "dprint", Number: 25, Canonical: true},
	{Name: "ftos", Number: 26, Canonical: true},
	{Name: "vtos", Number: 27, Canonical: true},
	{Name: "eprint", Number: 31, Canonical: true},
	{Name: "walkmove", Number: 32, Canonical: true},
	{Name: "droptofloor", Number: 34, Canonical: true},
	{Name: "lightstyle", Number: 35, Canonical: true},
	{Name: "rint", Number: 36, Canonical: true},
	{Name: "floor", Number: 37, Canonical: true},
	{Name: "ceil", Number: 38, Canonical: true},
	{Name: "checkbottom", Number: 40, Canonical: true},
	{Name: "pointcontents", Number: 41, Canonical: true},
	{Name: "fabs", Number: 43, Canonical: true},
	{Name: "aim", Number: 44, Canonical: true},
	{Name: "cvar", Number: 45, Canonical: true},
	{Name: "localcmd", Number: 46, Canonical: true},
	{Name: "nextent", Number: 47, Canonical: true},
	{Name: "particle", Number: 48, Canonical: true},
	{Name: "changeyaw", Number: 49, Canonical: true},
	{Name: "vectoangles", Number: 51, Canonical: true},
	{Name: "writebyte", Number: 52, Canonical: true},
	{Name: "writechar", Number: 53, Canonical: true},
	{Name: "writeshort", Number: 54, Canonical: true},
	{Name: "writelong", Number: 55, Canonical: true},
	{Name: "writecoord", Number: 56, Canonical: true},
	{Name: "writeangle", Number: 57, Canonical: true},
	{Name: "writestring", Number: 58, Canonical: true},
	{Name: "writeentity", Number: 59, Canonical: true},
	{Name: "sin", Number: 60, Canonical: true},
	{Name: "cos", Number: 61, Canonical: true},
	{Name: "sqrt", Number: 62, Canonical: true},
	{Name: "etos", Number: 65, Canonical: true},
	{Name: "movetogoal", Number: 67, Canonical: true},
	{Name: "precache_file", Number: 68, Canonical: true},
	{Name: "makestatic", Number: 69, Canonical: true},
	{Name: "changelevel", Number: 70, Canonical: true},
	{Name: "cvar_set", Number: 72, Canonical: true},
	{Name: "centerprint", Number: 73, Canonical: true},
	{Name: "ambientsound", Number: 74, Canonical: true},
	{Name: "precache_model2", Number: 75, Canonical: true},
	{Name: "precache_sound2", Number: 76, Canonical: true},
	{Name: "precache_file2", Number: 77, Canonical: true},
	{Name: "setspawnparms", Number: 78, Canonical: true},
	{Name: "local_sound", Number: 80, Canonical: true},
})
