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

// MarkMVPCardsByName covers cards no rAthena monster drops (their monster is
// only an ID there, its stats from latam-ro-calc): "Ultra Limacina Card" is
// an MVP card when the monster named "Ultra Limacina" is an MVP.
func MarkMVPCardsByName(c *gamedata.Catalog, db *rathena.DB) int {
	dropped := map[int]bool{}
	for _, m := range db.Mobs {
		for _, id := range m.Cards {
			dropped[id] = true
		}
	}
	// Every monster of that name must be an MVP (one MVP variant of a
	// mini-boss, like the special Deviling, doesn't make its card one).
	mvpName, other := map[string]bool{}, map[string]bool{}
	for _, m := range c.Monsters {
		if n := strings.ToLower(m.Name); m.MVP {
			mvpName[n] = true
		} else {
			other[n] = true
		}
	}
	n := 0
	for id, it := range c.Items {
		if it.Type != "Card" || it.MVPCard || dropped[id] {
			continue
		}
		name, ok := strings.CutSuffix(it.Name.Get("en"), " Card")
		if key := strings.ToLower(name); ok && mvpName[key] && !other[key] {
			it.MVPCard = true
			n++
		}
	}
	return n
}
