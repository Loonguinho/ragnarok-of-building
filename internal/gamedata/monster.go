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
	sort.Slice(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.rank != b.rank {
			return a.rank < b.rank
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
