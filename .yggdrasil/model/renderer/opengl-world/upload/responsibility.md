# Responsibility

`renderer/opengl-world/upload` owns the extracted OpenGL world helper slices that build or mutate world resources before draw submission.

It covers:

- CPU-side world and submodel geometry/lightmap/render-data assembly
- diffuse/fullbright/embedded-sky texture upload planning and application helpers
- lightmap dirty/recomposition/upload helpers
- raw world mesh upload helpers
- mechanical GL teardown helpers used during world cleanup

It does not own renderer-root lifecycle ordering, fallback policy, or cache-field assignment. Those remain in the parent `renderer/opengl-world` node.
