package gamedata

import (
	"sort"
	"strconv"
	"strings"
)

// Monster is a damage target (rAthena renewal mob_db).
type Monster struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"` // English (rAthena)
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
	for _, m := range c.Monsters {
		n := Normalize(m.Name)
		switch {
		case idErr == nil && m.ID == id:
			hits = append(hits, hit{m, 0})
		case strings.HasPrefix(n, q):
			hits = append(hits, hit{m, 1})
		case containsAll(n, strings.Fields(q)):
			hits = append(hits, hit{m, 2})
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
