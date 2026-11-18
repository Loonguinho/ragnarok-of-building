package web

import (
	"sort"

	"ragnarok-of-building/internal/calc"
)

// skillRow is one line of the damage window's table: an attack the build
// has, computed the same way the window computes the chosen one.
type skillRow struct {
	ID, Level  int // ID 0 = basic attack
	Name       string
	Hits       int
	Avg, Crit  int64
	Expected   float64
	HitsToKill int
	Selected   bool
	FromDesc   bool // the % comes from the skill description, not a formula
}

// skillTable computes every learned damaging skill (at its learned level)
// and the basic attack against the target, best expected damage first. A
// skill counts as damaging when it has a LATAM formula or its description
// gives a %. The chosen attack's states and element are kept.
func (s *Server) skillTable(b calc.Build, r calc.Result, l string) []skillRow {
	c := s.opt.Catalog
	if c.Monster(b.Target) == nil {
		return nil
	}
	ids := []int{0}
	for id, lv := range b.Skills {
		sk := c.Skills[id]
		if sk == nil || lv <= 0 || sk.Free() {
			continue
		}
		if _, ok := sk.RatioAt(lv); ok || calc.FormulaFor(c, b.Job, sk.Code) != nil {
			ids = append(ids, id)
		}
	}
	var rows []skillRow
	for _, id := range ids {
		nb := b
		nb.Atk = calc.AttackChoice{Skill: id, Level: b.Skills[id], Element: b.Atk.Element, States: b.Atk.States}
		if id == b.Atk.Skill { // the chosen one keeps its own settings
			nb.Atk = b.Atk
		}
		d := s.damageView(nb, r, l)
		if d.Dmg.Avg <= 0 && id != 0 {
			continue
		}
		row := skillRow{ID: id, Level: nb.Atk.Level, Name: tr(l, "Ataque básico"), Hits: d.Hits,
			Avg: int64(d.Dmg.Avg), Crit: int64(d.Dmg.Crit), Expected: d.Dmg.Expected, HitsToKill: d.Dmg.HitsToKill,
			Selected: id == b.Atk.Skill, FromDesc: d.RatioSource == "descrição"}
		if sk := c.Skills[id]; sk != nil {
			row.Name = sk.Name.Get(l)
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Expected > rows[j].Expected })
	return rows
}
