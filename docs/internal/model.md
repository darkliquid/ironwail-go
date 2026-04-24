# Package model

## Purpose

The `model` package defines the in-memory representations and loading logic for all Quake model formats. This includes world geometry (Brush/BSP models), animated character models (Alias/MDL), and 2D billboard effects (Sprites). It provides a unified vocabulary for these formats so that collision and rendering systems can treat them consistently.

## Key Types & Interfaces

- **`Model`**: The top-level struct representing a loaded asset. It contains common fields like bounds and flags, as well as type-specific data (BSP, Alias, or Sprite).
- **`AliasHeader`**: Contains the metadata for MDL models, including vertices, triangles, skins, and animation frames.
- **`MSprite`**: Represents a sprite model with one or more frames.
- **`Hull`**: Used for collision detection, containing clipnodes and planes.
- **`MSurface`**: Represents a single renderable surface on a brush model, including texture and lightmap information.

## Core Workflow

1.  **Loading**: Models are loaded from their respective file formats (BSP, MDL, SPR). The `MDLReader` and `LoadMDL` functions handle the complex bit-packed and compressed nature of Quake's character models.
2.  **Representation**: Data is expanded into optimized in-memory structures. For example, compressed vertex bytes in an MDL are stored in `Poses`, and `AliasHeader` maintains the relationships between frames and poses.
3.  **Animation Resolution**: For animated models, methods like `ResolveSkinFrame` determine which concrete frame or skin should be displayed based on the current game time.

## Integration

- **`internal/bsp`**: Provides the raw data structures for world models.
- **`internal/renderer`**: Consumes the `Model` and its sub-structures (`MSurface`, `AliasHeader`) to build vertex buffers and issue draw calls.
- **`internal/server`**: Uses the `Hull` and `ClipNode` data for physics and collision detection.

## Learning Tips

- **Vertex Compression**: Look at `TriVertX` and `DecodeVertex` in `mdl.go` to see how Quake saves space by storing vertices as 8-bit offsets from a scale/origin.
- **Collision Hulls**: Study the `Hull` struct and how it differs from visual geometry, which is key to understanding how Quake's movement physics works.
- **Animation Groups**: Note how frames can be grouped (`AliasGroup`) for randomized or timed animations within a single model.
