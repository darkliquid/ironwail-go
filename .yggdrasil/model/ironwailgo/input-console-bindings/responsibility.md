# Responsibility

## Purpose

`ironwailgo/input-console-bindings` owns runtime input routing and the command-package side of binds/console/chat/gameplay actions.

## Owns

- Key-destination coordination between gameplay, menu, console, and message/chat input.
- Mouse-grab policy and forced button release when leaving gameplay input.
- Chat input, text-edit repeat behavior, and menu mouse forwarding.
- Gameplay bind/command registration and runtime command handlers that live in the command package.

## Does not own

- Renderer/input backend implementation.
- Camera/view calculations beyond consuming input state.
