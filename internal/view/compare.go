package view

import (
	"maps"
	"math"
	"sort"
	"strings"

	"ragnarok-of-building/internal/calc"
)

// Item comparison: the build as it is next to the build with other items in
// one slot, each column with its own refine, grade and item effects (procs)
// turned on or off, and the stats and damage of every column.

// CompareCand is a column: an item for the slot at refine and grade. Fx
// turns item effects on or off in that column only (absent: as the build
// has them).
type CompareCand struct {
	Item, Refine, Grade int
	Fx                  map[string]bool
	// Cards are the column's cards and enchants; nil keeps what the swap
	// carries over from the equipped item.
	Cards *[4]int
}

// ComparePos is a card or enchant position of a column's item, with what
// can go there.
type ComparePos struct {
	Index   int
	Enchant bool
	Current int // 0: empty
	Options []CompareOpt
}

// CompareOpt is a card or enchant that fits a position.
type CompareOpt struct {
	ID   int
	Name string
}

// CompareFx is an effect of the column's item (or of its cards).
type CompareFx struct {
	Key, Label, Gives string
	ItemID            int
	On                bool
}

// CompareCol is a computed column.
type CompareCol struct {
	Item                 *ItemView // nil: nothing in the slot
	Refine, Grade        int
	Refineable, Gradable bool
	Effects              []CompareFx
	Positions            []ComparePos
}

// CompareRow is a stat in every column; Diffs and Better compare each
// column with the first ("" and 0 for the first and for equal values).
type CompareRow struct {
	Label  string
	Values []string
	Diffs  []string
	Better []int // 1 better, -1 worse, 0 same
	Main   bool  // the expected damage
}

// CompareView is the comparison.
type CompareView struct {
	Target string // "" without a monster in the damage window
	Cols   []CompareCol
	Rows   []CompareRow
}

// cloneBuild copies the maps a computation writes to.
func cloneBuild(b calc.Build) calc.Build {
	b.Equip, b.Skills, b.Buffs = maps.Clone(b.Equip), maps.Clone(b.Skills), maps.Clone(b.Buffs)
	b.Atk.States = maps.Clone(b.Atk.States)
	if b.Equip == nil {
		b.Equip = map[string]calc.Equip{}
	}
	return b
}

// applyFx sets the column's effect switches on its build.
func applyFx(b *calc.Build, fx map[string]bool) {
	if len(fx) == 0 {
		return
	}
	if b.Buffs == nil {
		b.Buffs = map[string]int{}
	}
	for k, on := range fx {
		if on {
			b.Buffs[k] = 1
		} else {
			delete(b.Buffs, k)
		}
	}
}

// CompareBuild is a column's build: the current one (cand.Item == 0) or
// with cand's item in the slot; with the column's effects applied. It is
// what "Equipar" puts in place.
func (v *Views) CompareBuild(b calc.Build, slot string, cand CompareCand) calc.Build {
	var nb calc.Build
	if cand.Item == 0 {
		nb = cloneBuild(b)
	} else {
		nb = v.s.swapAt(b, slot, cand.Item, cand.Refine, cand.Grade)
	}
	if cand.Item == 0 && cand.Cards != nil { // the worn item with other cards: its refine and grade too
		for k, e := range nb.Equip {
			if k == slot || (strings.HasPrefix(slot, "Head_") && strings.HasPrefix(k, "Head_") && e.Item == nb.Equip[slot].Item) {
				e.Refine, e.Grade = cand.Refine, cand.Grade
				nb.Equip[k] = e
			}
		}
	}
	if cand.Cards != nil {
		item := nb.Equip[slot].Item
		for k, e := range nb.Equip {
			if k == slot || (strings.HasPrefix(slot, "Head_") && strings.HasPrefix(k, "Head_") && e.Item == item) {
				e.Cards = *cand.Cards
				nb.Equip[k] = e
			}
		}
	}
	applyFx(&nb, cand.Fx)
	calc.Normalize(&nb, v.s.opt.Catalog)
	return nb
}

// comparePositions lists the card slots and the enchant positions the
// client's list opens for the item, as the slot window does.
func (s *Server) comparePositions(slot string, e calc.Equip, cards []CompareOpt, l string) []ComparePos {
	c := s.opt.Catalog
	it := c.Items[e.Item]
	if it == nil {
		return nil
	}
	n := min(it.Slots, 4)
	for k := range c.Enchants[it.ID] {
		n = max(n, min(k+1, 4))
	}
	var out []ComparePos
	for i := range n {
		p := ComparePos{Index: i, Enchant: i >= it.Slots, Current: e.Cards[i]}
		if p.Enchant {
			for _, id := range c.Enchants[it.ID][i] {
				if ci := c.Items[id]; ci != nil && !calc.EnchantBlocked(it.ID, i, e.Cards, id) {
					p.Options = append(p.Options, CompareOpt{id, ci.Name.Get(l)})
				}
			}
		} else {
			p.Options = cards
		}
		out = append(out, p)
	}
	return out
}

// compareCards are the cards that fit the slot, by name.
func (s *Server) compareCards(slot, l string) []CompareOpt {
	var out []CompareOpt
	for _, cd := range s.optCards() {
		if calc.CardFits(cd, slot) {
			out = append(out, CompareOpt{cd.ID, cd.Name.Get(l)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Compare computes the columns: cands[0] is usually the build as it is
// (Item 0).
func (v *Views) Compare(b calc.Build, slot string, cands []CompareCand, l string) CompareView {
	s := v.s
	c := s.opt.Catalog
	var out CompareView
	m := c.Monster(b.Target)
	if m != nil {
		out.Target = m.NameIn(l)
	}
	var builds []calc.Build
	cards := s.compareCards(slot, l)
	for _, cand := range cands {
		nb := v.CompareBuild(b, slot, cand)
		r := calc.Compute(nb, c)
		e := nb.Equip[slot]
		col := CompareCol{Refine: e.Refine, Grade: e.Grade}
		if it := c.Items[e.Item]; it != nil {
			iv := viewItem(it, l)
			col.Item, col.Refineable, col.Gradable = &iv, it.Refineable, it.Gradable
		}
		// The effects of the slot's item and of its cards.
		own := map[int]bool{e.Item: true}
		for _, cd := range e.Cards {
			own[cd] = true
		}
		for _, row := range itemEffectRows(r, c, l) {
			if own[row.ItemID] && row.ItemID != 0 {
				col.Effects = append(col.Effects, CompareFx{Key: row.Key, Label: row.Label, Gives: row.Gives, ItemID: row.ItemID, On: row.Value > 0})
			}
		}
		col.Positions = s.comparePositions(slot, e, cards, l)
		out.Cols = append(out.Cols, col)
		builds = append(builds, nb)
	}
	out.Rows = s.compareRows(builds, l)
	return out
}

// compareRows are the numbers that differ between the builds, each against
// the first, with the expected damage on top.
func (s *Server) compareRows(builds []calc.Build, l string) []CompareRow {
	if len(builds) == 0 {
		return nil
	}
	m := s.opt.Catalog.Monster(builds[0].Target)
	type colStats struct {
		vals     []float64
		expected float64
	}
	var labels []string
	var stats []colStats
	for _, nb := range builds {
		r := calc.Compute(nb, s.opt.Catalog)
		var cs colStats
		labels = labels[:0]
		add := func(label string, x float64) {
			labels = append(labels, label)
			cs.vals = append(cs.vals, x)
		}
		add("ATQ", float64(r.StatusATK+r.EquipATK))
		add("ATQM", float64(r.MATKMax+r.EquipMATK))
		add("P.ATQ", float64(r.PATK))
		add("S.ATQM", float64(r.SMATK))
		add("Precisão", float64(r.HIT))
		add("Crítico", float64(int(r.CRIT)))
		add("C.RATE", float64(r.CRATE))
		add("ASPD", float64(r.ASPD))
		add("Esquiva", float64(r.FLEE))
		add("DEF", float64(r.HardDEF+r.SoftDEF))
		add("DEFM", float64(r.HardMDEF+r.SoftMDEF))
		add("RES", float64(r.RES))
		add("MRES", float64(r.MRES))
		add("HP", float64(r.HP))
		add("SP", float64(r.SP))
		for i, n := range statNames {
			add(n, float64(r.Stats[i].Base+r.Stats[i].Bonus))
		}
		for i, n := range traitNames {
			add(n, float64(r.Traits[i].Base+r.Traits[i].Bonus))
		}
		if m != nil {
			cs.expected = s.damageView(nb, r, l).Dmg.Expected
		}
		stats = append(stats, cs)
	}
	if len(stats) == 0 {
		return nil
	}
	var out []CompareRow
	if m != nil {
		row := CompareRow{Label: tr(l, "Dano esperado"), Main: true}
		e0 := stats[0].expected
		for i, cs := range stats {
			row.Values = append(row.Values, fmtInt(int64(cs.expected+0.5)))
			d, better := "", 0
			if i > 0 && e0 > 0 {
				ratio := cs.expected / e0
				switch {
				case ratio > 1.0005:
					d, better = pctChange(ratio, l), 1
				case ratio < 0.9995:
					d, better = pctChange(ratio, l), -1
				}
			}
			row.Diffs = append(row.Diffs, d)
			row.Better = append(row.Better, better)
		}
		out = append(out, row)
	}
	for k, label := range labels {
		same := true
		for _, cs := range stats[1:] {
			same = same && cs.vals[k] == stats[0].vals[k]
		}
		if same {
			continue
		}
		row := CompareRow{Label: tr(l, label)}
		for i, cs := range stats {
			x := cs.vals[k]
			row.Values = append(row.Values, fmtInt(int64(math.Round(x))))
			d, better := "", 0
			if i > 0 && x != stats[0].vals[k] {
				diff := int64(math.Round(x - stats[0].vals[k]))
				d = fmtInt(diff)
				if diff > 0 {
					d, better = "+"+d, 1
				} else {
					better = -1
				}
			}
			row.Diffs = append(row.Diffs, d)
			row.Better = append(row.Better, better)
		}
		out = append(out, row)
	}
	return out
}
