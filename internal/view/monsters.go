package view

import (
	"context"
	"time"

	"ragnarok-of-building/internal/divinepride"
	"ragnarok-of-building/internal/gamedata"
)

// loadTarget makes sure a monster rAthena knows only by ID (a stub) has its
// stats, fetching that one ID from Divine Pride (cached on disk) the first
// time it is chosen. It returns a note for the user when it can't.
func (s *Server) loadTarget(id int, l string) string {
	c := s.opt.Catalog
	if c == nil || id == 0 || c.Monster(id) != nil {
		return ""
	}
	aegis, ok := c.MonsterStubs[id]
	if !ok {
		return ""
	}
	if s.opt.DP == nil {
		return tr(l, "Este monstro não tem atributos no rAthena; configure DP_API_KEY no .env para buscá-los no Divine Pride.")
	}
	s.mobMu.Lock()
	defer s.mobMu.Unlock()
	if c.Monster(id) != nil {
		return ""
	}
	if s.mobFailed[id] {
		return tr(l, "Não foi possível obter os atributos deste monstro no Divine Pride.")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	b, err := s.opt.DP.Get(ctx, divinepride.Monster, id, "pt")
	var m *gamedata.Monster
	if err == nil {
		m, err = divinepride.ParseMonster(b, gamedata.StubName(aegis))
	}
	if err != nil {
		s.opt.Log.Warn("divine pride monster", "id", id, "err", err)
		if s.mobFailed == nil {
			s.mobFailed = map[int]bool{}
		}
		s.mobFailed[id] = true
		return tr(l, "Não foi possível obter os atributos deste monstro no Divine Pride.")
	}
	c.AddMonster(m)
	return ""
}
