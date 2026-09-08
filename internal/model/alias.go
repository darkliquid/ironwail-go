package model

import (
	"io"

	"github.com/darkliquid/ironwail-go/internal/engine/arena"
	"github.com/darkliquid/ironwail-go/pkg/mdl"
)

// LoadAliasModel loads an alias model (.mdl) from r.
func LoadAliasModel(r io.ReadSeeker) (*Model, error) {
	return LoadAliasModelWithArena(r, nil)
}

// LoadAliasModelWithArena loads an alias model (.mdl) from r, optionally using the provided arena.
func LoadAliasModelWithArena(r io.ReadSeeker, ar *arena.Arena) (*Model, error) {
	file, err := mdl.Load(r)
	if err != nil {
		return nil, err
	}

	if ar != nil {
		if len(file.SkinDescs) > 0 {
			descs := arena.Alloc[AliasSkinDesc](ar, len(file.SkinDescs))
			copy(descs, file.SkinDescs)
			file.SkinDescs = descs
		}
		if len(file.STVerts) > 0 {
			verts := arena.Alloc[STVert](ar, len(file.STVerts))
			copy(verts, file.STVerts)
			file.STVerts = verts
		}
		if len(file.Triangles) > 0 {
			tris := arena.Alloc[DTriangle](ar, len(file.Triangles))
			copy(tris, file.Triangles)
			file.Triangles = tris
		}
		if len(file.Frames) > 0 {
			frames := arena.Alloc[AliasFrameDesc](ar, len(file.Frames))
			copy(frames, file.Frames)
			file.Frames = frames
		}
	}

	model := &Model{
		Type:        ModAlias,
		NumFrames:   file.NumFrames,
		SyncType:    SyncType(file.SyncType),
		Flags:       file.Flags,
		Mins:        file.Bounds.Mins,
		Maxs:        file.Bounds.Maxs,
		YMins:       file.Bounds.YMins,
		YMaxs:       file.Bounds.YMaxs,
		RMins:       file.Bounds.RMins,
		RMaxs:       file.Bounds.RMaxs,
		AliasHeader: file,
	}

	return model, nil
}
