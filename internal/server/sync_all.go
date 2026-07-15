package server

// syncAllFromQCVM copies ALL entity fields from the QCVM byte array to the
// Go EntVars structs for every active entity. This is the nuclear option:
// instead of selectively syncing pushers vs non-pushers with snapshot/diff/
// restore, we simply copy everything after every QC callback.
//
// This eliminates the entire class of "forgot to sync in this path" bugs
// because there's only one sync function and it's called everywhere.
//
// Performance: O(numEdicts * numFields). For ~700 entities and ~105 fields,
// this is ~73K float copies per call — negligible compared to the QC
// execution itself.
func (s *Server) syncAllFromQCVM() {
	if s == nil || s.QCVM == nil {
		return
	}
	limit := s.NumEdicts
	if limit > len(s.Edicts) {
		limit = len(s.Edicts)
	}
	for entNum := 0; entNum < limit; entNum++ {
		ent := s.Edicts[entNum]
		if ent == nil || ent.Free || ent.Vars == nil {
			continue
		}
		oldSolid := ent.Vars.Solid
		oldModel := ent.Vars.Model
		oldModelIndex := ent.Vars.ModelIndex

		s.syncEdictFromQCVM(entNum, ent)

		// Relink only when the entity's area-list membership would change
		// (solid type or model). Don't relink for origin changes — the QC
		// callback already set AbsMin/AbsMax directly, and relinking would
		// overwrite those with recalculated (fat-1) values. This matches
		// C where SV_TouchLinks doesn't call SV_LinkEdict after callbacks.
		if entNum != 0 && (ent.Vars.Solid != oldSolid || ent.Vars.Model != oldModel || ent.Vars.ModelIndex != oldModelIndex) {
			s.LinkEdict(ent, false)
		}
	}
}

// syncAllToQCVM copies ALL entity fields from the Go EntVars structs to the
// QCVM byte array for every active entity. Called before every QC callback
// to ensure QC sees the latest Go-side state.
func (s *Server) syncAllToQCVM() {
	if s == nil || s.QCVM == nil {
		return
	}
	limit := s.NumEdicts
	if limit > len(s.Edicts) {
		limit = len(s.Edicts)
	}
	for entNum := 0; entNum < limit; entNum++ {
		ent := s.Edicts[entNum]
		if ent == nil || ent.Free || ent.Vars == nil {
			continue
		}
		s.syncEdictToQCVM(entNum, ent)
	}
}
