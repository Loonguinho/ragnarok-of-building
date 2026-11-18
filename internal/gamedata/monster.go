package gamedata

import (
	"sort"
	"strconv"
	"strings"
)

// Monster is a damage target (rAthena renewal mob_db).
type Monster struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`            // English (rAthena)
	Names       Text     `json:"names,omitempty"` // pt/es from the monster's card name in the client
	Level       int      `json:"lv"`
	HP          int      `json:"hp"`
	DEF         int      `json:"def"`
	MDEF        int      `json:"mdef"`
	RES         int      `json:"res,omitempty"`
	MRES        int      `json:"mres,omitempty"`
	Stats       [6]int   `json:"stats"`
	Size        string   `json:"size"`  // Small, Medium, Large
	Race        string   `json:"race"`  // Demihuman, Brute, ...
	Element     string   `json:"ele"`   // Neutral, Water, ...
	ElementLv   int      `json:"eleLv"` // 1-4
	Class       string   `json:"class"` // Normal, Boss, ...
	MVP         bool     `json:"mvp,omitempty"`
	DamageTaken int      `json:"dmgTaken"` // % of player damage taken (100 = normal)
	Modes       []string `json:"modes,omitempty"`
	// Source is where the stats came from when not rAthena ("Divine Pride").
	Source string `json:"src,omitempty"`
	// Stub: known only by ID (search result whose stats are not loaded yet).
	Stub bool `json:"-"`
}

// Monster returns a monster by ID: the catalog's, or one loaded at run time.
func (c *Catalog) Monster(id int) *Monster {
	if m := c.Monsters[id]; m != nil {
		return m
	}
	if m, ok := c.extraMobs.Load(id); ok {
		return m.(*Monster)
	}
	return nil
}

// AddMonster keeps a monster loaded at run time (safe for concurrent use).
func (c *Catalog) AddMonster(m *Monster) { c.extraMobs.Store(m.ID, m) }

// StubName is a readable name from an AegisName: "MD_BETELGEUSE" ->
// "Betelgeuse (MD)". Prefixes of two or three letters (instance variants)
// go in parentheses.
func StubName(aegis string) string {
	parts := strings.Split(strings.ToLower(aegis), "_")
	var tags, words []string
	for i, p := range parts {
		switch {
		case p == "":
		case i == 0 && len(parts) > 1 && len(p) <= 3:
			tags = append(tags, strings.ToUpper(p))
		default:
			words = append(words, strings.ToUpper(p[:1])+p[1:])
		}
	}
	name := strings.Join(words, " ")
	if len(tags) > 0 {
		name += " (" + strings.Join(tags, " ") + ")"
	}
	return name
}

// NameIn is the monster's name in a language: the local name taken from its
// card when known, else the rAthena (English) name.
func (m *Monster) NameIn(lang string) string {
	if lang != "en" {
		if n := m.Names[lang]; n != "" {
			return n
		}
		if n := m.Names["pt"]; n != "" && lang == "es" {
			return n
		}
	}
	return m.Name
}

// OtherName is the English name when the shown name differs (search hint).
func (m *Monster) OtherName(lang string) string {
	if n := m.NameIn(lang); n != m.Name {
		return m.Name
	}
	return ""
}

// SoftDEF is the renewal status DEF of a non-player: (level + VIT) / 2.
func (m *Monster) SoftDEF() int { return (m.Level + m.Stats[2]) / 2 }

// SoftMDEF is the renewal status MDEF of a non-player: (INT + level) / 4.
func (m *Monster) SoftMDEF() int { return (m.Stats[3] + m.Level) / 4 }

// SearchMonsters finds monsters by ID or name (accent/case-insensitive),
// prefix matches first, then by level.
func (c *Catalog) SearchMonsters(q string, limit int) []*Monster {
	q = Normalize(q)
	if q == "" {
		return nil
	}
	id, idErr := strconv.Atoi(q)
	q0 := id
	type hit struct {
		m    *Monster
		rank int
	}
	var hits []hit
	words := strings.Fields(q)
	for _, m := range c.Monsters {
		if idErr == nil && m.ID == id {
			hits = append(hits, hit{m, 0})
			continue
		}
		// Any of its names (English from rAthena, local from the card).
		rank := 0
		for _, name := range append([]string{m.Name}, m.Names["pt"], m.Names["es"]) {
			n := Normalize(name)
			switch {
			case n == "":
			case strings.HasPrefix(n, q):
				rank = 1
			case rank == 0 && containsAll(n, words):
				rank = 2
			}
		}
		if rank > 0 {
			hits = append(hits, hit{m, rank})
		}
	}
	// Monsters known only by ID: the loaded one, or a stub to load.
	for id, aegis := range c.MonsterStubs {
		m := c.Monster(id)
		if m == nil {
			m = &Monster{ID: id, Name: StubName(aegis), Stub: true}
		}
		rank := 0
		if idErr == nil && id == q0 {
			rank = -1
		}
		for _, name := range []string{m.Name, aegis} {
			n := Normalize(strings.ReplaceAll(name, "_", " "))
			switch {
			case rank != 0:
			case strings.HasPrefix(n, q):
				rank = 1
			case containsAll(n, words):
				rank = 2
			}
		}
		if rank != 0 {
			hits = append(hits, hit{m, max(rank, 0)})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.rank != b.rank {
			return a.rank < b.rank
		}
		if a.m.Stub != b.m.Stub {
			return b.m.Stub // monsters with stats first
		}
		if a.m.Level != b.m.Level {
			return a.m.Level < b.m.Level
		}
		return a.m.ID < b.m.ID
	})
	out := make([]*Monster, 0, min(limit, len(hits)))
	for _, h := range hits[:min(limit, len(hits))] {
		out = append(out, h.m)
	}
	return out
}
