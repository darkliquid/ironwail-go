# Interface

## Main consumers

- runtime code that selects or drives the OpenGL backend

## Contracts

- this node fulfills the package-level backend contract for the OpenGL path
- it must coordinate with shared canvas and world/entity helpers without leaking backend specifics upward
