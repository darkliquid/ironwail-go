# Interface

## Main consumers

- backend implementations
- HUD/menu/draw systems that depend on consistent canvas semantics

## Main surface

- `RenderContext`
- `Backend`
- `Config`
- canvas and transform-related types

## Contracts

- canvas transforms define the logical coordinate spaces used by 2D drawing
- config and backend interfaces are the stable package contract for renderer creation and runtime control
