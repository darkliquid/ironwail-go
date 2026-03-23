# Internals

## Logic

This node turns authoritative server state into Quake/FitzQuake/RMQ wire messages. It maintains signon buffers for initial world state, shared unreliable datagrams for transient events, and per-client reliable payloads for serverinfo, stats, and entity updates. Encoding paths must select the correct extended or legacy representation based on protocol limits. Non-entity intermission stats are refreshed from QuakeC globals before serialization so spawn packets and later reliable `svc_updatestat` messages match C Ironwail's `host_cmd.c` behavior.

## Constraints

- Entity/model/sound index width and coordinate encoding vary by protocol and must stay aligned with client expectations.
- Omission/update rules for packet entities are parity-sensitive because clients infer lifecycle from those deltas.
- Message buffers enforce MTU/signon size ceilings and may intentionally skip non-essential updates when capacity is exhausted.

## Decisions

### Centralized protocol-gated serialization layer

Observed decision:
- The package keeps protocol branching close to message emission instead of scattering it across gameplay code.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Protocol compatibility is easier to audit and test, even though the serializer remains tightly coupled to authoritative server state.
