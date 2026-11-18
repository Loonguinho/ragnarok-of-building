package clientimport

import (
	"strings"

	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/rathena"
)

// markMVPCards flags the cards only MVPs drop: dropped by an MVP that is not
// an event copy (E_..., which drop ordinary cards too) and by no ordinary
// monster. Boss-class droppers (instance versions of an MVP) don't count
// against it.
func markMVPCards(c *gamedata.Catalog, db *rathena.DB) int {
	mvp, normal := map[int]bool{}, map[int]bool{}
	for _, m := range db.Mobs {
		for _, id := range m.Cards {
			switch {
			case m.MVP && !strings.HasPrefix(m.Aegis, "E_"):
				mvp[id] = true
			case !m.MVP && m.Class != "Boss":
				normal[id] = true
			}
		}
	}
	n := 0
	for id := range mvp {
		if it := c.Items[id]; it != nil && !normal[id] {
			it.MVPCard = true
			n++
		}
	}
	return n
}
