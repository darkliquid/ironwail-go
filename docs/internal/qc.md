# Package `qc`

## Purpose
The `qc` package implements the QuakeC Virtual Machine (VM). QuakeC is the domain-specific scripting language used to define Quake's game logic, monster AI, and player interactions. This package loads the compiled `progs.dat` files, interprets the bytecode, and provides a bridge between the scripts and the Go-based engine.

## Key Types & Interfaces
- **`VM`**: The core state of the virtual machine, including the function table, statement (bytecode) array, globals, and the string table.
- **`GlobalVars`**: A typed view into the VM's global variable array, mapping fixed offsets to familiar QuakeC variables like `self`, `other`, `time`, and `trace_fraction`.
- **`EntVars`**: A typed view into an entity's (edict's) fields, such as `origin`, `velocity`, `health`, and `classname`.
- **`BuiltinFunc`**: A function signature for Go-native functions that are exposed to QuakeC (e.g., `print`, `spawn`, `traceline`).
- **`CSQC`**: A specialized wrapper for the Client-Side QuakeC VM, which handles HUD rendering and client-side logic.

## Core Workflow
1. **Loading**: `LoadProgs` reads a `.dat` file, populating the VM's statements, functions, and initial global/entity definitions.
2. **Execution**: The engine calls `ExecuteProgram` with a function index (like the `StartFrame` entry point).
3. **Interpreter Loop**: Inside `exec.go`, the VM iterates through bytecode statements. Each statement consists of an opcode and up to three operands (usually offsets into the globals or entity fields).
4. **Builtins**: When a statement calls a negative function index, the VM dispatches to a Go function registered in the `Builtins` array, allowing QuakeC to perform "engine-level" tasks like physics or networking.

## Integration
- **Server**: The primary user of the `qc` package. The server runs the "SSQC" (Server-Side QuakeC) to drive the game simulation.
- **Renderer/HUD**: In mods that support it, the client runs "CSQC" to draw custom HUD elements or handle client-side effects.
- **Cvar System**: The VM can interact with engine cvars via builtins like `cvar()` and `cvar_set()`.

## Learning Tips
- **Interpreter Loop**: Read `internal/qc/exec.go` to see how the core switch-statement for opcodes is implemented. It's the most performance-critical part of the VM.
- **Data Layout**: Compare `internal/qc/types.go` with the original `progdefs.h` or `progdefs.q1` to see how the engine maintains bit-perfect compatibility with QuakeC data layouts.
- **Builtin Bridge**: Check `internal/qc/builtins.go` to see how Go functions are mapped to VM callbacks.

## Tests

### VM & Execution (`exec_test.go`)

**`TestExecuteProgramDivByZeroBehaviorMatrixMatchesC`** — 7 cases of float division by ±0: `1/+0→+Inf`, `-1/+0→-Inf`, `0/+0→NaN`, etc. QC programs sometimes divide by zero intentionally (early-exit patterns); the result must match C IEEE behavior for demo compatibility. Asserts both the result register and `OFSReturn`.

**`TestExecuteProgramOPAddressAllowsWorldEntityFieldStores`** — `OPAddress(self=0, fieldOfs=health)` followed by `OPStorePF(42, ptrOfs)` must store 42 in world entity's health field. The world entity (index 0) has special status; this test ensures the address/store pipeline doesn't reject it.

**`TestExecuteProgramCallRunsCalleeFirstStatement`** — Main calls callee via `OPCall0`; callee's first statement stores 42 into `targetOfs`. Asserts both the returned value and `OFSReturn` are 42. Verifies the `FirstStatement` offset is used correctly (the inner loop adjusts by −1 before the first increment).

**`TestExecuteProgramCallCopiesParametersIntoCalleeLocals`** — Main stores 37 into `OFSParm0` and calls callee with `OPCall1`. Callee reads its local parm slot and copies it to `capturedOfs`. Verifies parm-copy semantics work correctly for single-parm calls.

**`TestExecuteProgramNestedCallRestoresCallerLocals`** — Caller stores 99 into `sharedLocalOfs`, calls callee which writes 7 to the same slot. After callee returns, caller reads `sharedLocalOfs` into `resultOfs`. Asserts `resultOfs==99`. Validates the caller local save/restore mechanism that prevents callee clobber.

**`TestExecuteProgramOPReturnCopiesThreeSlotsToOFSReturn`** — Callee executes `OPReturn(returnVecOfs)` where `returnVecOfs={11,22,33}`. Asserts `OFSReturn` vector is `{11,22,33}` (full 12-byte copy). Verifies `OPReturn` (as opposed to `OPDone`) correctly copies the full vector to the return register.

**`TestProfileResultsAccumulatesPerFunction`** — Runs a two-function program (main calls callee twice), calls `ProfileResults(10)`, and checks both functions have non-zero counts. Then calls `ProfileResults` again and asserts it's empty (counters reset). Validates the profiling infrastructure used by the `profile` console command.

**`TestExecuteProgramRunawayLoopProtectionMatchesC`** — An infinite `OPGoto 0` loop must produce a `"runaway loop error"` error. Safety net preventing QC bugs from hanging the engine. Asserts `XStatement==0` after the error.

**`TestExecuteProgramRunawayLoopLimitConstantMatchesC`** — Asserts `runawayLoopLimit == 0x1000000`. This is a parity constant with the C engine; changing it would break expected behavior for mods that know the limit.

**`TestExecuteProgramRunawayLoopLimitOverrideUsesVMFixture`** — Sets `vm.RunawayLoopLimit=3`; `statementBudgetLimit()` must return 3, and the infinite loop triggers at that limit. Enables faster test execution by lowering the limit in test fixtures.

**`TestExecuteProgramTraceCallEventsNested`** — Installs `TraceCallFunc`, runs a program where main calls callee. Asserts 4 events in order: `enter/1/0`, `enter/2/1`, `leave/2/1`, `leave/1/0`. Also asserts `XFunction==nil` and `XFunctionIndex==-1` after execution. Validates the trace hook lifecycle.

**`TestExecuteProgramTraceCallEventsBuiltin`** — Same but callee is a builtin (negative function index). Asserts the `"builtin"` phase event with `FunctionIndex==-1`.

### Hook Isolation (`vm_hooks_isolation_test.go`)

**`TestVMServerHooksIsolation`** — Creates two VMs `a` and `b`, installs different `BroadcastPrint` hooks on each, fires both, and asserts each received only its own message. Then clears `a`'s hooks and asserts `b`'s are unaffected. Before this was fixed, a package-level `serverBuiltinHooks` global caused all VMs to share hooks, so a CSQC and server VM would cross-contaminate each other's callbacks.

### General Builtins (`builtins_test.go`)

**`TestLocalizedTextMessageDecodesEscapedControlCharacters`** — `localizedTextMessage` must decode `\\n`, `\\t`, `\\"`, `\\\\` to real control characters. QC-generated text messages use escape sequences that must be decoded before display.

**`TestWriteStringBuiltinDecodesEscapedNewlines`** — Sets parm1 to `"line1\nline2"` (escaped), calls `writeStringBuiltin`, and asserts the `WriteString` server hook receives the real-newline version. The server sends text to clients via write builtins; decode must happen before transmission.

**`TestRegisterBuiltinsIncludesNoopExtensionProbes`** — After `RegisterBuiltins`, slots 99 and 100 (extension probe builtins) must zero `OFSReturn` (returning 0 to indicate "not supported"). QC mods probe for extensions by calling builtin 99/100 and checking the return; a missing builtin breaks any mod using this pattern.

**`TestModBuiltinWarnsOnZeroDivisor`** — `modBuiltin` with b=0 must return 0 and print `"PF_mod: mod by zero\n"`. Division by zero in QC would propagate NaN/Inf; a warning helps mappers debug their scripts.

**`TestModBuiltinBehaviorMatrixMatchesC`** — Tests 9 cases of integer/float modulo including sign combinations, exact zero, and ±negative-zero divisors. The Go `%` operator uses truncated division; the C implementation must match exactly for demo parity.

**`TestSoundBuiltinScalesVolumeAndPreservesZeroAttenuation`** — Calls `sound` with parm3=1.0 (max volume) and parm4=0 (no attenuation). Asserts `gotVolume==255` and `gotAttenuation==0`. Volume is stored as a byte [0,255] in the network protocol; forgetting to scale would result in near-silence.

**`TestTraceBuiltinsToggleVMTraceFlag`** — `traceOnBuiltin` sets `vm.Trace=true`; `traceOffBuiltin` sets it back to false. The developer must be able to toggle QC trace mode from QC code.

**`TestCoredumpBuiltinPrintsAllAllocatedEntities`** — With `NumEdicts=3`, `coredumpBuiltin` prints `"entity 0\n"`, `"entity 1\n"`, `"entity 2\n"` in order. `coredump()` is a QC debugging tool; wrong output range or order would confuse debugging.

**`TestSpawnAllocatesEntity`** — `spawn()` returns entity index 1 and increments `NumEdicts` from 1 to 2. Every new entity in the game is allocated this way; an off-by-one would place the entity in the wrong slot.

**`TestRemoveClearsEntityData`** — Sets health=99 and origin={1,2,3} on entity 1, calls `remove(1)`, asserts both fields are zero. Removed entities must be zeroed so the next `spawn` gets a clean slot.

**`TestSetOriginUpdatesAbsBounds`** — Entity 1 has mins=(-1,-2,-3), maxs=(4,5,6). `setorigin(1, {10,20,30})` must update absmin=origin+mins=(9,18,27) and absmax=origin+maxs=(14,25,36). The world clipper uses absmin/absmax for broad-phase collision; stale bounds cause missed or phantom collisions.

**`TestSetSizeUpdatesSizeAndAbsBounds`** — Sets origin={10,20,30}, calls `setsize(1, {-1,-2,-3}, {4,5,6})`, asserts mins, maxs, size=(5,7,9), absmin, absmax. Size and absolute bounds must be consistent; inconsistency causes invisible clipping.

**`TestSetModelStoresModelAndModelIndex`** — `setmodel(1, "progs/test.mdl")` must store the string in the entity's model field and set modelindex=1. The renderer looks up `modelindex` to find what to draw; a wrong index renders the wrong mesh.

**`TestPrecacheBuiltinsFallbackToCSQCHooks`** — Without server hooks, `precacheSound` and `precacheModel` fall back to CSQC client hooks and return the input string. In CSQC mode, precache builtins must use the client-side registry, not the server-side one.

**`TestBuiltinsUseServerHooksWhenConfigured`** — Installs non-nil server hooks for every builtin, calls each one, and asserts each hook was called exactly once. The server hooks are the integration point between QC execution and the engine; a builtin that silently ignores a set hook would break gameplay.

### Math Builtins (`builtins_math_test.go`)

**`TestRegisterBuiltinsCanonicalMappings`** — After `RegisterBuiltins`, a hardcoded list of standard slot numbers must all be non-nil. QC mods call builtins by number (e.g., `#16` = `lightstyle`); a missing slot causes a runtime panic or silent no-op.

**`TestMathCVarAndLocalCmdBuiltins`** — Tests `rint`, `floor`, `ceil`, `fabs`, `cvar`, `cvar_set`, and `localcmd` in a single sweep. These are among the most frequently called QC builtins for game logic (health rounding, reading/writing game config, running console commands).

**`TestVectoyawBuiltinMatchesQuakeYaw`** — Table-driven: `{1,0,0}→0°`, `{0,1,0}→90°`, `{-1,0,0}→180°`, `{0,-1,0}→270°`, `{1,1,0}→45°`, zero→0°. `vectoyaw` is used for AI navigation; wrong yaw would make monsters face the wrong direction.

**`TestVectoanglesBuiltinUsesQuakeYawConvention`** and **`TestVectoanglesBuiltinVerticalCasesMatchC`** — Validates Quake's right-handed yaw convention and the degenerate up/down cases that match C special-casing for rockets flying vertically.

**`TestMakevectorsMatchesQuakeAngleVectors`** — Three angle sets (pure yaw, pitch+yaw, pitch+yaw+roll) must produce `v_forward/v_right/v_up` globals matching `qtypes.AngleVectors`. `makevectors` populates globals used by weapons and AI for aiming.

**`TestNormalizeBuiltinReturnsUnitVector`** and **`TestNormalizeBuiltinZeroVector`** — `normalize({3,4,0})=={0.6,0.8,0}` and `normalize({0,0,0})=={0,0,0}` (no panic). Normalization is used throughout QC for direction vectors; a non-unit result would scale forces or render artefacts.

**`TestMathBuiltins`** — Tests `sin`, `cos`, `sqrt`, `tan`, `asin`, `acos`, `atan`, and `atan2` against known values. QC mods use these for custom physics and lighting effects; inaccurate results would cause visible jitter.

**`TestMinMaxBoundPow`** — Tests `min`, `max`, `bound`, `pow`, and their variadic forms. QC staples for clamping health, damage, and movement values.

**`TestStringBuiltins`** — Tests `strlen`, `strcat`, `substring` (with negative offsets), `stov`, `stof`, `etos`, `chr2str`, `strzone`, `str2chr`. String builtins power HUD text, centerprint, and player name formatting.

**`TestSearchBuiltinsFallback`** — Without server hooks, the fallback `find`, `findfloat`, `nextent`, and `findradius` walk the entity list in-process and return correct results. In headless tests (without a full server), these builtins must still work for QC unit tests and CSQC.

**`TestRandomBuiltinDistribution`** — Calls `random()` 1000 times with `sv_gameplayfix_random=1` and asserts every value is in the open interval (0, 1). The fix ensures the result is never exactly 0 or 1, preventing edge cases in spawn tables.

**`TestRandomBuiltinMatchesCompatSequence`** and **`TestRandomBuiltinLegacyFormulaWhenGameplayFixDisabled`** — The first 3 values from the RNG must exactly match hardcoded sequences for both the fixed and legacy formulas. Demo playback requires byte-for-byte identical RNG sequences; any deviation breaks replay.

**`TestRandomBuiltinUsesInjectedCompatRNGState`** and **`TestRandomBuiltinTraceParityDeltaFromUpstreamRandDraw`** — Verify that an injected `compatrand.RNG` (with one draw pre-consumed) produces the expected shifted sequence. The server may pre-consume one RNG draw for another purpose; the VM must use the shared injected state exactly.

### CSQC (`csqc_test.go`)

**`TestNewCSQCCreatesValidInstance`** — `NewCSQC()` returns a non-nil struct with a non-nil VM and `IsLoaded()==false`. Validates initial construction.

**`TestCSQCLoadFailsWithInvalidData`** — `Load(nil bytes)` returns a non-nil error and leaves `IsLoaded()==false`. Prevents a half-initialized CSQC from being used.

**`TestCSQCFunctionIndicesStartAtMinusOne`** and **`TestCSQCGlobalsStartAtMinusOne`** — All tracked function indices and global offsets start at -1. These are resolved from the progs by name after load; -1 means "not found yet" and prevents accidental calls before loading.

**`TestCSQCSyncGlobalsUnloadedNoPanic`** — `SyncGlobals` on an unloaded CSQC must not panic. Guards against early frame calls before progs are ready.

**`TestCSQCPrecacheModelReturnsStableIndices`**, **`TestCSQCPrecacheSoundReturnsStableIndices`**, and **`TestCSQCPrecachePicReturnsStableIndices`** — First call returns 1; a second unique call returns 2; a duplicate returns 1. Network protocols encode references as indices; duplicates must map to the same index.

**`TestCSQCPrecachedModelsReturnsRegistrationOrder`** — `PrecachedModels()` returns models in registration order. The server sends the list as an ordered array; order must match.

**`TestCSQCUnloadResetsPrecacheRegistries`** — After `Unload()`, all three precache registries are empty and the next call returns index 1 again. Maps must start with a clean CSQC state.

**`TestCSQCSyncGlobalsUsesRealtimeForCltime`** — `SyncGlobals({RealTime:10.5})` must set `cltime` global to 10.5 (realtime, not server time) and `OFSTime` to the server time. Verifies correct global mapping for time variables.

**`TestCSQCCallDrawHudUsesProgramReturnValue`** and **`TestCSQCCallDrawHudClearsStaleReturnBeforeExecute`** — `CallDrawHud(false)` returns false and `CallDrawHud(true)` returns true based on the progs return value. Also verifies that a pre-set stale `OFSReturn=1` is cleared before execution, so a previous call's return value cannot contaminate the next.

### CSQC Draw/Client Builtins (`builtins_csqc_test.go`)

**`TestCSQCDrawBuiltinsNoHooks`** — With no draw hooks installed, all draw builtins (`iscachedpic`, `precache_pic`, `drawgetimagesize`, `drawcharacter`, `drawrawstring`, `drawpic`, `drawfill`, `stringwidth`, `drawsetcliparea`, `drawresetcliparea`, `drawsubpic`) return 0 or zero-vectors without panicking. Guards against nil dereferences.

**`TestCSQCDrawBuiltinsUseHooks`** — With all hooks installed, tests each draw builtin with exact argument values and asserts the hook closure is called with exactly those arguments. Validates the builtin-to-hook translation layer for every CSQC draw primitive.

**`TestCSQCClientBuiltinsNoHooks`** — With no client hooks, `getstati`, `getstatf`, `getstats`, `getplayerkeyvalue`, and `registercommand` return zero/empty without panicking.

**`TestCSQCClientBuiltinsUseHooks`** — With all client hooks, validates: `getstati(7)=123`, `getstatf(9,0,0)=42.5`, `getstatf(9,4,3)=5` (bitfield), `getstats(11)="weapon_supershotgun"`, `getplayerkeyvalue(2,"name")="ranger"`, `registercommand("cl_cmd_test")` calls the hook.
