package qbsp

import (
	"github.com/darkliquid/ironwail-go/pkg/bsp"
)

// PortalFile returns the PRT1 portal file (Map ↔ leaf topology) for a
// compiled map: one record per shared facet between non-solid leaves, in
// the format vis consumes. Aliased from pkg/bsp.
type PortalFile = bsp.PortalFile

// Portal is one shared facet between two non-solid leaves, aliased from pkg/bsp.
type Portal = bsp.Portal

