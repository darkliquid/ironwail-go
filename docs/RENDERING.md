# Rendering Pipeline Comparison (OpenGL)

This document provides a detailed comparison of the OpenGL rendering implementations.

## 1. OpenGL Context and Versioning

- **Ironwail (C)**: Uses a mix of legacy OpenGL (Fixed Function Pipeline) and modern OpenGL (Shaders/VBOs). It often targets OpenGL 3.x Compatibility Profile for compatibility with older hardware.
- **Ironwail-Go (Go)**: Strictly uses OpenGL 4.6 Core Profile. This allows for modern techniques like bindless textures and compute shaders but requires newer hardware.

## 2. World and Entity Rendering

### C-Implementation (`gl_rmain.c`)
- **BSP**: Traversals are done through `R_RecursiveWorldNode`. Surfaces are drawn using `GL_DrawSurfaces`.
- **Entities**: Alias models (.mdl) and sprites (.spr) are drawn through `R_DrawAliasModel` and `R_DrawSpriteModel`.
- **Lighting**: Lightmaps are updated dynamically for dynamic lights.

### Go-Implementation (`renderer_opengl.go`, `world_runtime_opengl.go`)
- **BSP**: Handled in `DrawBSPWorld` which utilizes `WorldRuntime` for efficient GPU-side rendering.
- **Entities**: Entities are collected in `collectAliasEntities()` and `collectSpriteEntities()` and drawn through the `DrawContext` using shaders.
- **Lighting**: Modern implementation utilizing UBOs and dynamic lighting shaders.

## 3. Comparison of Features

| Feature | Ironwail (C) | Ironwail-Go (Go) |
| :--- | :--- | :--- |
| **Shaders** | Optional or GLSL 1.x-3.x | GLSL 4.10 (Core) |
| **VBO/VAO** | VBOs used for some geometry | VBO/VAO used for all geometry |
| **Texture Management** | `gl_texmgr.c` (Manual binding) | Modern texture management with descriptors |
| **2D Overlay** | `SCR_Draw2D` (legacy GL) | `init2DRenderer` (Shader-based 2D) |

## 4. Key Parity Objectives

- **Visual Fidelity**: Parity is ensured by matching the Quake colormap and palette processing.
- **PVS and Frustum Culling**: Both implementations use the BSP's Potentially Visible Set (PVS) to cull non-visible areas.
- **Texture Filtering**: Parity is maintained by respecting the `gl_texturemode` (e.g., `GL_NEAREST_MIPMAP_LINEAR`).
