package compiler

import (
	"strings"
	"testing"
)

func TestBuiltinNameRegistry_RegistersCanonicalNameAndNumber(t *testing.T) {
	t.Parallel()

	if got, ok := builtinDirectiveRegistry.numberForName("BPRINT"); !ok || got != 23 {
		t.Fatalf("numberForName(BPRINT) = (%d, %v), want (23, true)", got, ok)
	}
	if got, ok := builtinDirectiveRegistry.nameForNumber(77); !ok || got != "precache_file2" {
		t.Fatalf("nameForNumber(77) = (%q, %v), want (%q, true)", got, ok, "precache_file2")
	}
	if _, ok := builtinDirectiveRegistry.nameForNumber(79); ok {
		t.Fatal("nameForNumber(79) unexpectedly resolved an unmapped compiler builtin name")
	}
	if got, ok := builtinDirectiveRegistry.numberForName("print"); !ok || got != 24 {
		t.Fatalf("numberForName(print) = (%d, %v), want (24, true)", got, ok)
	}
	if got, ok := builtinDirectiveRegistry.nameForNumber(24); !ok || got != "sprint" {
		t.Fatalf("nameForNumber(24) = (%q, %v), want (%q, true)", got, ok, "sprint")
	}
}

func TestBuiltinNameRegistry_RejectsDuplicateNames(t *testing.T) {
	t.Parallel()

	_, err := newBuiltinNameRegistry([]builtinNameRegistration{
		{Name: "spawn", Number: 14},
		{Name: "SPAWN", Number: 114},
	})
	if err == nil {
		t.Fatal("expected duplicate-name validation failure")
	}
	if !strings.Contains(err.Error(), `duplicates builtin 114 already registered for name "spawn"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuiltinNameRegistry_RejectsDuplicateNumbers(t *testing.T) {
	t.Parallel()

	_, err := newBuiltinNameRegistry([]builtinNameRegistration{
		{Name: "spawn", Number: 14},
		{Name: "spawn_alias", Number: 14},
	})
	if err == nil {
		t.Fatal("expected duplicate-number validation failure")
	}
	if !strings.Contains(err.Error(), `duplicates builtin 14 already registered for name "spawn"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuiltinNameRegistry_AllowsAliasForCanonicalBuiltinNumber(t *testing.T) {
	t.Parallel()

	registry, err := newBuiltinNameRegistry([]builtinNameRegistration{
		{Name: "sprint", Number: 24, Canonical: true},
		{Name: "print", Number: 24},
	})
	if err != nil {
		t.Fatalf("newBuiltinNameRegistry returned error: %v", err)
	}
	if got, ok := registry.numberForName("print"); !ok || got != 24 {
		t.Fatalf("numberForName(print) = (%d, %v), want (24, true)", got, ok)
	}
	if got, ok := registry.nameForNumber(24); !ok || got != "sprint" {
		t.Fatalf("nameForNumber(24) = (%q, %v), want (%q, true)", got, ok, "sprint")
	}
}

func TestBuiltinNameRegistry_RejectsSecondCanonicalForSameNumber(t *testing.T) {
	t.Parallel()

	_, err := newBuiltinNameRegistry([]builtinNameRegistration{
		{Name: "sprint", Number: 24, Canonical: true},
		{Name: "print", Number: 24, Canonical: true},
	})
	if err == nil {
		t.Fatal("expected duplicate-canonical-number validation failure")
	}
	if !strings.Contains(err.Error(), `duplicates builtin 24 already registered for canonical name "sprint"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuiltinNameRegistry_RejectsMissingName(t *testing.T) {
	t.Parallel()

	_, err := newBuiltinNameRegistry([]builtinNameRegistration{
		{Name: "", Number: 14},
	})
	if err == nil {
		t.Fatal("expected missing-name validation failure")
	}
	if !strings.Contains(err.Error(), "missing name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuiltinNameRegistry_RejectsOutOfRangeNumber(t *testing.T) {
	t.Parallel()

	_, err := newBuiltinNameRegistry([]builtinNameRegistration{
		{Name: "too_high", Number: 1280},
	})
	if err == nil {
		t.Fatal("expected out-of-range validation failure")
	}
	if !strings.Contains(err.Error(), "outside valid range 1..1279") {
		t.Fatalf("unexpected error: %v", err)
	}
}
