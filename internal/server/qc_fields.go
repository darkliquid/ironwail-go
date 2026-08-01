package server

// cacheQCFieldOffsets looks up extension field offsets from the loaded
// progs.dat FieldDefs table. Called during server spawn after progs.dat
// is loaded. Standard fields use compile-time EntField* constants; these
// extension fields are mod-specific and must be resolved at runtime.
func (s *Server) cacheQCFieldOffsets() {
	if s.QCVM == nil {
		s.EffectsMask = defaultEffectsMask
		return
	}
	s.QCFieldAlpha = s.QCVM.FindField("alpha")
	s.QCFieldScale = s.QCVM.FindField("scale")
	s.QCFieldGravity = s.QCVM.FindField("gravity")
	s.QCFieldState = s.QCVM.FindField("state")
	s.QCFieldWait = s.QCVM.FindField("wait")
	s.QCFieldSpeed = s.QCVM.FindField("speed")
	s.QCFieldCustomFlags = s.QCVM.FindField("customflags")
	s.QCFieldThCheckAttack = s.QCVM.FindField("th_checkattack")
	s.QCFieldMap = s.QCVM.FindField("map")
	s.EffectsMask = s.detectEffectsMaskFromQC()
}
