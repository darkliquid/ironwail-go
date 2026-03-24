# Internals

## Logic

The node keeps low-risk structural cleanup focused on package-level forward declarations. `prototypes.go` groups the declarations by source file so readers can locate shared cross-file hooks quickly while leaving the implementation functions in their original gameplay files.

## Constraints

The declarations must remain package-level `var` function slots because the translated QuakeC code assigns implementations after declaration order has been established. Replacing them with direct function declarations or broader API rewrites would risk changing compiler assumptions and is intentionally out of scope.

## Decisions

- Chose a dedicated `prototypes.go` over spreading declarations across eight support files because the work item asked for a contained structural improvement and this keeps the cleanup mechanical.
- Chose not to consolidate every monster-specific prototype block in the package because limiting the first pass to shared support files avoids a large blast radius while still improving organization.
