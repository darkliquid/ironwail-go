# Responsibility

`qgo/compiler-tests` captures the evidence used to validate the reverse-engineered compiler. The tests assert header layout, opcode selection, allocator behavior, synthetic package definitions, string interning, and—most importantly—round-trip compatibility by compiling fixtures, loading them into `internal/qc.VM`, and executing the generated functions.

This node is not responsible for implementing compiler behavior. Its job is to pin down what the compiler is expected to emit and which sample programs demonstrate that behavior.

It also maintains a narrow structural parity smoke that validates shallow `progs.dat` structural signals (core section/header sanity, function-shape expectations, and required opcode presence) for arithmetic/controlflow fixtures so regressions are caught beyond byte-ordering checks.
It maintains a deterministic VM-visible parity smoke harness over selected arithmetic/controlflow fixture calls, comparing QCVM returns against native-Go fixture semantics for `Add`, `Max`, and `Sum` so lowering drift is detected early.
Allocator-focused tests in this node are responsible for preserving the package baseline assumption that compiler globals begin after QCVM/system slots (`qc.OFSMsgEntity + 1`), ensuring qgo slices compare against the same reserved-global model as runtime.
It also owns explicit deferred-scope evidence for struct literals: `Vec3` composite literals remain supported while non-`Vec3` struct literals are intentionally deferred and must fail with a stable diagnostic contract.
