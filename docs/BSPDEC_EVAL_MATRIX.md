# bspdec eval matrix (snapshot 2026-09-10)

Regenerate with `mise run bspdec-eval && mise run bspdec-report -data dataset/bspdec`.

| slice | pairs | error-free | mean voxel IoU | brush model |
| --- | --- | --- | --- | --- |
| synthetic (BRUSHLIST direct path) | 1000 | 1000 | 1.0000 | 1 model/brush |
| classic (dm1, e1m8) | 2 | 0 | 0.000 | CSG-fidelity caveats |

Classic rows (error text from the known CSG-fidelity beads xxy.6xi/.aeh):

| pkg | map | voxel IoU | brushDelta | error |
| --- | --- | --- | --- | --- |
| dm1 | dm1 | 0.000 | +0 | self-check: entity 0 brush 37: only 2 faces |
| e1m8 | e1m8 | 0.000 | +0 | self-check: entity 0 brush 294: face 2 point 608 400 -608 in |
