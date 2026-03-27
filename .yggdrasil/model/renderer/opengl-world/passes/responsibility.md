# Responsibility

`renderer/opengl-world/passes` owns the extracted OpenGL helpers that shape world draw work immediately before final root orchestration.

It covers:

- CPU-side world-face bucketing/classification
- repeated draw-call submission loops
- repeated world-program/model/VAO bind setup
- bbox probe/debug helpers
- sprite vertex flattening and final sprite draw helper submission

It does not own renderer-root camera/state snapshots, pass ordering, or cache lookup.
