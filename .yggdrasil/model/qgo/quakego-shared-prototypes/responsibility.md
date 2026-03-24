# Responsibility

`qgo/quakego-shared-prototypes` owns the shared `quakego` support files whose control flow still depends on QuakeC-style forward declarations. It keeps the package-level prototype surface in one place so cross-file references are easy to audit without changing runtime behavior.

This node is responsible for declaration organization in core support files such as combat, doors, plats, and weapons. It is not responsible for changing gameplay semantics, monster AI logic that still keeps local forward declarations, or `qgo/compiler` lowering behavior.
