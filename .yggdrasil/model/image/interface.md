# Interface

## Main consumers

- `draw`, which loads `gfx.wad` and turns WAD lumps into `QPic` assets for HUD/menu/console code.
- `renderer`, which parses miptextures, decodes external TGAs, and expands paletted data for upload.
- tooling/tests that need PNG/TGA export or direct format verification.

## Main surface

- `LoadWad`, `CleanupName`, `Wad`, `Lump`, `LumpType`
- `ParseQPic`, `QPic`, `(*QPic).SubPic`
- `LoadPalette`, `Palette`, `Palette.ToRGBA`
- `ParseMipTex`, `MipTex`, `(*MipTex).MipLevel`
- `LoadTGA`, `DecodeTGA`
- `LoadPNG`, `WritePNG`, `WriteTGA`, `RGBAFromPalette`
- `AlphaEdgeFix`

## Contracts

- WAD parsing only accepts `WAD2` archives and canonicalizes lump names for map lookup.
- `QPic` width and height must be non-zero and the payload must contain enough indexed pixels.
- TGA decoding only supports the subset of Quake-relevant truecolor/grayscale formats implemented in the package.
- Palette index `255` is treated as transparent only when callers explicitly request transparent expansion.
- `WritePNG` accepts either 24-bit RGB or 32-bit RGBA buffers (`bpp` 24/32) and takes an `upsidedown` flag. With `upsidedown=false` it flips rows vertically before encoding, matching Ironwail C `Image_WritePNG` semantics.
- `WriteTGA` accepts either 24-bit RGB or 32-bit RGBA buffers (`bpp` 24/32) and takes an `upsidedown` flag that controls the TGA descriptor top-origin bit (`0x20`), mirroring Ironwail C `Image_WriteTGA`.
