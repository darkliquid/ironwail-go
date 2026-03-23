# Internals

## Logic

`protocol.go` is a contract catalog rather than a state machine. It groups protocol versions, Quake/FitzQuake/RMQ flags, server-to-client and client-to-server message identifiers, temp-entity events, sound packet flags, baseline/update flags, and helper functions for alpha encoding/decoding. This single file is referenced by multiple subsystems that need to agree on exact numeric values and bit layouts.

## Constraints

- Constants are externally observable across the network and therefore much less flexible than internal implementation details.
- The file mixes multiple protocol generations (classic Quake, FitzQuake, RMQ, rerelease additions), so consumers must know which subsets apply in which runtime contexts.
- Encoding helpers such as the alpha converters are part of the protocol contract, not just convenience wrappers.

## Decisions

### Keep all network wire constants in one canonical file

Observed decision:
- The package centralizes protocol identifiers and compact encoding helpers in `protocol.go` instead of spreading them across client, server, and serializer code.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Cross-subsystem agreement on wire values is easier to maintain, but the file becomes broad and has to cover multiple protocol eras in one place.
