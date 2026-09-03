package transpile

import (
	"strings"
	"testing"
)

func transpile(t *testing.T, src string) string {
	t.Helper()
	out, err := Transpile(src, Options{})
	if err != nil {
		t.Fatalf("Transpile: %v", err)
	}
	return out
}

// The fixtures below are drawn from the real rerelease triggers.qc.

func TestTranspilesFunctionDefinitionAndFieldTruthiness(t *testing.T) {
	out := transpile(t, `void() trigger_reactivate =
{
	if (self.max_health)
		self.solid = SOLID_BBOX;
	else
		self.solid = SOLID_TRIGGER;
};
`)
	for _, want := range []string{
		"func trigger_reactivate() {",
		"if Self.MaxHealth != 0 {",
		"Self.Solid = SOLID_BBOX",
		"} else {",
		"Self.Solid = SOLID_TRIGGER",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q, got:\n%s", want, out)
		}
	}
}

func TestTranspilesFloatReturnAndStringComparison(t *testing.T) {
	out := transpile(t, `float() CheckValidTouch =
{
	if (other.classname != "player")
		return FALSE;
	if (other.health <= 0)
		return FALSE;
	return TRUE;
};
`)
	for _, want := range []string{
		"func CheckValidTouch() float32 {",
		`if Other.ClassName != "player" {`,
		"return False",
		"return True",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q, got:\n%s", want, out)
		}
	}
}

func TestTranspilesBitwiseFlagCondition(t *testing.T) {
	out := transpile(t, `void() InitTrigger =
{
	if (self.spawnflags & SPAWNFLAG_TRIGGER_FIRST)
	{
		self.think1 = self.use;
	}
};
`)
	if !strings.Contains(out, "(int(Self.SpawnFlags) & int(SPAWNFLAG_TRIGGER_FIRST)) != 0") {
		t.Errorf("bitwise flag condition not lowered, got:\n%s", out)
	}
}

func TestTranspilesEngineBuiltinCall(t *testing.T) {
	out := transpile(t, `void() InitTrigger =
{
	setmodel (self, self.model);
};
`)
	if !strings.Contains(out, "engine.SetModel(Self, Self.Model)") {
		t.Errorf("builtin call not mapped, got:\n%s", out)
	}
}

func TestTranspilesLocalsAndArithmetic(t *testing.T) {
	out := transpile(t, `void() sums =
{
	local float total;
	local float i;
	total = total + i;
};
`)
	for _, want := range []string{"var total float32", "var i float32", "total = total + i"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q, got:\n%s", want, out)
		}
	}
}

func TestTranspilesVectorLiteralAndWhile(t *testing.T) {
	out := transpile(t, `void() v =
{
	while (self.angles != '0 0 0')
		SetMovedir ();
};
`)
	if !strings.Contains(out, "Self.Angles != quake.MakeVec3(0, 0, 0)") {
		t.Errorf("vector literal not mapped, got:\n%s", out)
	}
	if !strings.Contains(out, "for Self.Angles != quake.MakeVec3(0, 0, 0) {") {
		t.Errorf("while not lowered to for, got:\n%s", out)
	}
}

func TestTranspilesGlobalWithInitialiser(t *testing.T) {
	out := transpile(t, `float force_retouch = 2;
string message_null;
`)
	if !strings.Contains(out, "var ForceRetouch float32 = 2") {
		t.Errorf("global initialiser missing, got:\n%s", out)
	}
	if !strings.Contains(out, "var MessageNull string") {
		t.Errorf("global missing, got:\n%s", out)
	}
}

func TestTranspilesFieldDeclarationAsComment(t *testing.T) {
	out := transpile(t, ".float max_health;\n")
	if !strings.Contains(out, "already defined on quake.Entity") {
		t.Errorf("standard field should be commented, got:\n%s", out)
	}
}

func TestTranspilesEntityFieldStringTruthiness(t *testing.T) {
	out := transpile(t, `void() noise_check =
{
	if (self.noise)
		sound (self, CHAN_VOICE, self.noise, 1, ATTN_NORM);
};
`)
	if !strings.Contains(out, "Self.Noise != StringNull") {
		t.Errorf("string truthiness not applied, got:\n%s", out)
	}
	if !strings.Contains(out, "engine.Sound(Self, CHAN_VOICE, Self.Noise, 1, ATTN_NORM)") {
		t.Errorf("builtin call not mapped, got:\n%s", out)
	}
}

func TestTranspilesNegationAndNot(t *testing.T) {
	out := transpile(t, `void() neg =
{
	if (!self.health)
		self.health = 100;
	self.nextthink = time + -0.5;
};
`)
	if !strings.Contains(out, "Self.Health == 0") {
		t.Errorf("negated truthiness not applied, got:\n%s", out)
	}
	if !strings.Contains(out, "Time + -0.5") {
		t.Errorf("unary minus lost, got:\n%s", out)
	}
}

func TestTranspilesAndOrChains(t *testing.T) {
	out := transpile(t, `void() chain =
{
	if (self.health && other.classname == "player" || self.count)
		self.solid = SOLID_NOT;
};
`)
	// QC `&&` (rerelease) short-circuits; `||` likewise. Comparisons pass
	// through as-is.
	if !strings.Contains(out, "Self.Health != 0 && Other.ClassName == \"player\" || Self.Count != 0") {
		t.Errorf("and/or chain not mapped, got:\n%s", out)
	}
}

func TestTranspileUnknownTopLevelResyncs(t *testing.T) {
	// The FGD block comment content is skipped by the lexer, but a stray
	// unsupported top-level construct must not abort the rest of the file.
	out := transpile(t, `void() before = { };
this is not valid qc;
void() after = { };
`)
	if !strings.Contains(out, "func before()") || !strings.Contains(out, "func after()") {
		t.Errorf("parser failed to resync past unsupported construct, got:\n%s", out)
	}
}
