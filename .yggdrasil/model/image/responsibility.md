# Responsibility

## Purpose

`image` owns low-level Quake image/container decoding and lightweight transcoding helpers: WAD2 lumps, `QPic` pictures, `MipTex` textures, palette handling, TGA loading, and PNG/TGA export utilities.

## Owns

- WAD2 header/lump parsing and canonical lump-name lookup.
- `QPic` parsing and sub-picture extraction.
- Quake palette loading and indexed-to-RGBA conversion helpers.
- `MipTex` parsing plus mip-level extraction.
- Supported TGA decoding and debug/export helpers (`LoadPNG`, `WritePNG`, `WriteTGA`).
- Image-level cleanup helpers such as `AlphaEdgeFix`.

## Does not own

- Filesystem search/mount policy for locating assets.
- GPU upload or renderer/backend-specific texture management.
- HUD/menu/layout policy that decides how decoded assets are presented.
- Full BSP parsing beyond interpreting raw miptexture blobs.
