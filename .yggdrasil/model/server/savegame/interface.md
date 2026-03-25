# Interface

## Main consumers

- `host/session` and command flows that trigger save/load
- tests covering portable snapshot and text save compatibility

## Main surface

- save snapshot capture APIs
- save snapshot restore APIs
- text/KEX save parse and restore helpers
- text-save gamedir compatibility validation before restore proceeds

## Contracts

- QC string indices are VM-instance-local and must be serialized as text, not raw handles.
- Precache/lightstyle/entity ordering must be restored before world relinking and later networking.
- Text/KEX save restores must reject mismatched or unavailable gamedirs instead of silently replaying a save into the wrong mod context.
- Restore must rebuild world-link state after entity reconstruction so collision/gameplay behavior resumes correctly.
