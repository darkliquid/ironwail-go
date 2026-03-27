# Responsibility

`renderer/opengl-world/sky` owns the extracted OpenGL helpers specific to sky rendering.

It covers:

- embedded-sky texture/frame resolution helpers
- sky-pass GL submission helpers
- external skybox cubemap and face upload helpers

It does not own root skybox loading/orchestration, external-sky mode selection, or renderer field assignment. Those stay in the parent `renderer/opengl-world` node.
