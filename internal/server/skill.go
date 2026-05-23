package server

import "strconv"

func (s *Server) normalizeSkillCVarForSpawn() int {
	skill := 1
	if skillCV := s.CVar.Get("skill"); skillCV != nil {
		skill = int(skillCV.Float + 0.5)
		if skill < 0 {
			skill = 0
		} else if skill > 3 {
			skill = 3
		}
		s.CVar.Set("skill", strconv.Itoa(skill))
	}
	return skill
}
