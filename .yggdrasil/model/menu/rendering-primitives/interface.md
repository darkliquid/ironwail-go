# Interface

## Main consumers

- the various menu page drawing functions in `menu_main.go`, `menu_game.go`, and `menu_options.go`.

## Main surface

- `drawMenuTextBox`
- `translateSetupPlayerPic`
- `drawPlaqueAndTitle`
- `drawCursor`, `drawArrowCursor`
- `drawText`

## Contracts

- Drawing assumes Quake menu-space coordinates and `DrawMenuPic` / `DrawMenuCharacter` semantics.
- Unsupported runes in menu text fall back to `'?'` and bright text is expressed by adding 128 to character codes.
- Player preview translation remaps Quake shirt/pants palette ranges without altering geometry.
