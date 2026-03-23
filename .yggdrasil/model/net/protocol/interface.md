# Interface

## Main consumers

- client parsing code that switches on `SVC*` message IDs.
- server networking code that emits protocol messages and update flags.
- renderer/gameplay code that interprets protocol-side encoded fields such as alpha or effects flags.

## Main surface

- protocol version constants
- server/client message constants (`SVC*`, `CLC*`)
- temp entity, sound, baseline, update, and RMQ/FitzQuake flag constants
- alpha encoding helpers such as `ENTALPHA_OPAQUE`, `ENTALPHA_ENCODE`, `ENTALPHA_DECODE`, `ENTALPHA_TOSAVE`

## Contracts

- This file is the canonical wire schema for the package and downstream consumers.
- Numeric values must remain stable because they are part of the network contract with clients/servers.
- Helper encoders/decoders define how protocol-level compact representations map back to runtime values.
