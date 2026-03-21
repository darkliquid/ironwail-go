package renderer

import "github.com/ironwail/ironwail-go/internal/model"

func aliasHeaderFromModel(hdr *model.AliasHeader) *AliasHeader {
	if hdr == nil {
		return nil
	}
	frames := make([]AliasFrame, len(hdr.Frames))
	for i, frame := range hdr.Frames {
		frames[i] = AliasFrame{
			FirstPose: frame.FirstPose,
			NumPoses:  frame.NumPoses,
			Interval:  float64(frame.Interval),
		}
	}
	return &AliasHeader{
		NumFrames:    hdr.NumFrames,
		Flags:        hdr.Flags,
		Frames:       frames,
		PoseVertType: hdr.PoseVertType,
	}
}

func (r *Renderer) ensureAliasStateLocked(entity AliasModelEntity) *AliasEntity {
	if entity.EntityKey == AliasViewModelEntityKey {
		created := false
		if r.viewModelAliasState == nil {
			r.viewModelAliasState = &AliasEntity{}
			created = true
		}
		return seedAliasState(r.viewModelAliasState, entity, created)
	}

	if r.aliasEntityStates == nil {
		r.aliasEntityStates = make(map[int]*AliasEntity)
	}
	state, ok := r.aliasEntityStates[entity.EntityKey]
	if !ok {
		state = &AliasEntity{}
		r.aliasEntityStates[entity.EntityKey] = state
	}
	return seedAliasState(state, entity, !ok)
}

func seedAliasState(state *AliasEntity, entity AliasModelEntity, created bool) *AliasEntity {
	flags := entity.LerpFlags
	if created || state.ModelID != entity.ModelID {
		flags |= LerpResetAnim | LerpResetMove
	}
	preserved := state.LerpFlags & (LerpResetAnim2 | LerpFinish)
	state.Frame = entity.Frame
	state.LerpFlags = preserved | flags
	state.LerpFinish = entity.LerpFinish
	state.Origin = entity.Origin
	state.Angles = entity.Angles
	state.ModelID = entity.ModelID
	state.SkinNum = entity.SkinNum
	state.ColorMap = entity.ColorMap
	state.IsPlayer = entity.IsPlayer
	return state
}

func (r *Renderer) pruneAliasStatesLocked(entities []AliasModelEntity) {
	if len(r.aliasEntityStates) == 0 {
		return
	}
	keep := make(map[int]struct{}, len(entities))
	for _, entity := range entities {
		if entity.EntityKey == AliasViewModelEntityKey {
			continue
		}
		keep[entity.EntityKey] = struct{}{}
	}
	for key := range r.aliasEntityStates {
		if _, ok := keep[key]; !ok {
			delete(r.aliasEntityStates, key)
		}
	}
}
