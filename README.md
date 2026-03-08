# Ironwail Go

Ironwail Go is an exercise in porting the entire [Ironwail][1] Quake codebase
from C to Go, for the purposes of learning and education. It is an experiment
to get more experience with agentic coding and furthermore to learn more about
the Quake engine, game programming and indulge in a bit of nostalgia from my
school days of hacking together Quake mods and maps.

## Did you say agentic coding? Is this just AI slop?

Yes and no. A large portion of the codebase has been written entirely by AI
agents converting the C code to Go. However, I've been fairly hands on in
terms of planning and guiding that work, as well as reviewing and making
manual changes of my own.

## Differences from Ironwail

Well, apart from the obvious that this is Go, rather than C, I'm building this
with the following changes:

- OpenGL/CGO as the default gameplay renderer/runtime
- gogpu/WebGPU as a secondary backend for non-parity experimentation
- Dividing the codebase up into packages
- Use Go stdlib for as much as possible, rather than custom implementations of
  things from the original C codebase

Additionally, I'm trying to build it as readable as possible, with extensive
commenting and to keep as much of the codebase in Go as practical. The
canonical OpenGL renderer currently requires CGo bindings, but the gameplay and
engine logic remain in Go and can still be understood without diving deeply
into C engine code.

## Building

The toolchain is built around [mise][2] which provides both the tooling and
the tasks for running tests, builds, etc.

You can see what tasks are available to run using `mise tasks`

The canonical parity/build path is the CGO/OpenGL runtime:

- `mise run test`
- `mise run build-cgo`
- `mise run smoke-cgo-map-start`

The gogpu tasks remain available for secondary-backend work, but they are no
longer the primary parity gate.

[1]:https://github.com/andrei-drexler/ironwail
[2]:https://mise.jdx.dev
