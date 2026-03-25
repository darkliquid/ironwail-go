# Internals

## Logic

The `core` node mixes several categories of low-level helpers because they all survive from the same compatibility-oriented subset of the old Quake common layer. `SizeBuf` tracks current size and read position over a single backing slice, `COM_Parse` and friends preserve the original text-tokenizing style with global token state, and path/hash helpers provide small reusable utilities without introducing new package dependencies. `Link` and `BitArray` remain tiny allocation-free helpers for subsystems that want intrusive lists or packed-bit state.

## Constraints

- `ComToken` and argv globals create deliberate shared mutable state.
- `SizeBuf.WriteAngle` and `WriteAngle16` encode angles with simple truncation, which is a parity-sensitive detail callers must understand.
- Some helpers appear lightly used or future-facing, so their long-term boundary rationale is not fully proven by current call sites.

## Decisions

### Preserve C-style helpers where compatibility outweighs idiomatic redesign

Observed decision:
- The port keeps several global/stateful utility surfaces (`ComToken`, argv checks, intrusive links) instead of rewriting everything into stateless abstractions.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Existing Quake-style parsing and utility call patterns remain easy to port, but the package retains some non-idiomatic shared state and mixed concerns.

## Audit Notes (internal/common parity vs C Ironwail)

### Scope and method

- Audited `internal/common/common.go` and `internal/common/common_parse_newline.go` against `Quake/common.c` and `Quake/common.h`.
- Evidence-driven only: entries below are direct code-observable mismatches or matches.

### Confirmed parity gaps

1. `COM_CheckParmNext` case-sensitivity mismatch:
   - Go uses `strings.EqualFold` (case-insensitive) in `internal/common/common.go`.
   - C uses `Q_strcmp` in `COM_CheckParmNext` (case-sensitive) in `Quake/common.c`.

2. `COM_ParseEx` overflow mode mismatch:
   - C enforces fixed token buffer size (`com_token[1024]`) and uses `mode` (`CPE_NOTRUNC` vs `CPE_ALLOWTRUNC`) to return `NULL` or truncate.
   - Go defines `CPE_Mode` but does not enforce a token-size limit, so the mode does not materially affect parsing behavior.

3. `COM_AddExtension` matching rule mismatch:
   - C compares extension identity using `COM_FileGetExtension(path)` and `strcmp(...)` (exact extension value).
   - Go uses `HasSuffix(strings.ToLower(path), strings.ToLower(extension))`, which is path-suffix based and case-insensitive.

4. `COM_FileBase` short-name fallback mismatch:
   - C returns `"?model?"` when basename length before dot is `< 2`.
   - Go returns any non-empty basename (including single-character names), and only falls back for empty or `"."`.

5. Angle write rounding mismatch:
   - C `MSG_WriteAngle` / `MSG_WriteAngle16` use `Q_rint(...)` before quantization.
   - Go `WriteAngle` / `WriteAngle16` use direct `int(...)` truncation.

### Parity-correct areas (confirmed)

- `COM_Parse`/`COM_ParseEx` main tokenization behavior for whitespace, `//` and `/* */` comments, quoted strings, and single-character tokens is structurally aligned.
- `COM_ParseIntNewline`, `COM_ParseFloatNewline`, and `COM_ParseStringNewline` behavior is aligned with C intent (parse value/token, then consume trailing whitespace including newline).
- `COM_HashString` / `COM_HashBlock` use the same FNV-1a constants and update order as C.

### Benign divergences (observed)

- `SizeBuf.GetSpace` error path differs from C's fatal-error style (`Host_Error`/`Sys_Error`) by returning `nil`/`false`; this appears to be an intentional Go safety/ergonomics adaptation rather than an accidental parser/path/hash parity defect.
