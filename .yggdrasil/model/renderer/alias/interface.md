# Interface

## Main consumers

- renderer roots that need backend-neutral alias animation or mesh shaping

## Contracts

- `SetupAliasFrame`, `SetupEntityTransform`, batching helpers, and mesh builders accept backend-neutral DTOs plus model metadata and return pure CPU results
- mesh helpers expose shared alias vertex interpolation and Euler rotation behavior without depending on renderer-owned cache or submission state
- callers must adapt backend-local alias pose/reference storage into the helper package and keep any submission/resource ownership outside this node

