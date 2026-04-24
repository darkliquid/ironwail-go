# BSP Package

## Purpose
The `bsp` package is responsible for reading and parsing Quake Binary Space Partitioning (BSP) map files. These files contain the level geometry, textures, lighting data, and visibility information. The package translates the raw bytes of a BSP file into strongly typed Go structures used by the server for collision and the renderer for drawing the world.

## Key Types & Interfaces
- **`File`**: The main structure representing a parsed BSP file, containing all its lumps (planes, vertexes, nodes, faces, etc.).
- **`Reader`**: A helper used to decode the BSP format from an `io.ReadSeeker`.
- **`DModel`**: Represents a sub-model within the map (e.g., doors, platforms, or the world itself).
- **`DPlane`, `DSNode`, `DSLeaf`**: Core structures of the BSP tree used for spatial partitioning and visibility determination.
- **`DSFace`, `Texinfo`**: Define how geometry is rendered and how textures are mapped to surfaces.

## Core Workflow
1. **Header Parsing**: The `Reader` first reads the `DHeader`, which identifies the BSP version and the locations (offsets and lengths) of the 15 data "lumps".
2. **Lump Loading**: Each lump is read into memory. Some are used as raw bytes (like lighting or visibility), while others are parsed into slices of typed structs (like planes or vertexes).
3. **Version Handling**: Since there are multiple BSP formats (Standard Quake, BSP2, Quake 64), the package handles the variations in structure sizes and field types (e.g., 16-bit vs 32-bit indices for nodes and edges).
4. **Structural Mapping**: The parsed data is organized into a `File` struct, which serves as the "source of truth" for the map's geometry and metadata.

## Integration
- **Server**: Uses the BSP tree (`Nodes`, `Leafs`, `Clipnodes`) and `Planes` for movement and collision detection.
- **Renderer**: Uses `Faces`, `Vertexes`, `Edges`, and `Texinfo` to build the visual representation of the map.
- **Model System**: The world is treated as the first model (index 0) in the BSP file.

## Learning Tips
- **BSP Versions**: The package supports original Quake (v29), BSP2 (used for large maps), and Quake 64. Compare the different Node and Leaf structures (e.g., `DSNode` vs `DL2Node`) to see how the format evolved.
- **Lump-based Architecture**: BSP files are a great example of a "lump-based" format, similar to WAD files. Understanding how the header points to various data sections is fundamental to Quake modding.
- **Coordinate Systems**: Pay attention to the use of `[3]float32` for points and normals, and how Quake's coordinate system (Z-up) is reflected in the data.
