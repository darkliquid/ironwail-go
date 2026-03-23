# Internals

## Logic

The package is format-centric rather than renderer-centric. `LoadWad` reads the WAD2 header, seeks to the lump directory, loads each lump payload into memory, and stores it in a canonicalized-name map. `ParseQPic` and `ParseMipTex` then interpret those raw bytes without deep copying unless a later operation like `SubPic` allocates fresh storage. Palette helpers preserve Quake's indexed-pixel model until a caller explicitly expands to RGBA. TGA decoding supports a narrow subset of image types and pixel depths, handling BGR/BGRA reordering, origin flipping, and RLE decompression before producing a standard `image.RGBA`. Export helpers move the other direction, writing straightforward PNG or uncompressed 32-bit TGA output for screenshots and debugging.

## Constraints

- The package assumes little-endian binary layouts for Quake-era formats.
- `LoadWad` eagerly stores all lumps in memory and silently lets later duplicate cleaned names overwrite earlier ones in the map.
- `ParseQPic` and `ParseMipTex` return views over the original byte buffer, so callers must respect backing-data lifetime and immutability.
- `SubPic` clamps coordinates/bounds rather than requiring strict intersection semantics.
- `AlphaEdgeFix` uses 8-neighbor averaging with toroidal wrapping so transparent border texels pick up plausible color data for filtered sampling.
- Generated `lumptype_string.go` reflects `stringer` output for the aliased lump-type constants and is not hand-maintained.

## Decisions

### Keep image decoding separate from filesystem lookup and renderer upload

Observed decision:
- The Go port isolates Quake image/container parsing into its own package instead of folding decoding into filesystem or renderer code.

Rationale:
- **unknown — inferred from code and comments, not confirmed by a developer**

Observed effect:
- Asset consumers can share one decoding layer while choosing their own loading policy, palette expansion strategy, and renderer/backend-specific upload behavior.
