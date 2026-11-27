package view

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

// Comparing whole sets (Comparar conjuntos): each column changes several
// pieces at once, so the bonuses that need them together count.

// SetPiece is one piece a column puts on.
type SetPiece struct {
	Slot string
	CompareCand
}

// GearSet is a set of the catalog the class can wear.
type GearSet struct {
	Key    string
	Name   string
	Pieces []SetPiece
	Cards  bool // the set needs cards: they go in free card positions
}

// SetPieceView is a piece as the column shows it.
type SetPieceView struct {
	Slot, Label string
	CompareCol
	Worn    []string // the cards' names (the column as it is)
	Dropped bool     // the rest of the column leaves no room for it
}

// SetCol is a column: the build as it is (no pieces) or with them.
type SetCol struct {
	Pieces []SetPieceView
	// Left are the slots the pieces empty (the shield of a two-handed
	// weapon, a headgear under a bigger one).
	Left []string
}

// SetView is the comparison.
type SetView struct {
	Target string
	Cols   []SetCol
	Rows   []CompareRow
}

// SetBuild is the build with the pieces on. The order they come in does
// not matter, except that a later piece takes the place of an earlier one
// they cannot share.
func (v *Views) SetBuild(b calc.Build, pieces []SetPiece) calc.Build {
	nb := cloneBuild(b)
	ps := slices.Clone(pieces)
	// The weapon first: it decides whether there is room for a shield.
	sort.SliceStable(ps, func(i, j int) bool { return ps[i].Slot == calc.Weapon && ps[j].Slot != calc.Weapon })
	for _, p := range ps {
		if p.Item == 0 {
			continue
		}
		nb = v.CompareBuild(nb, p.Slot, p.CompareCand)
	}
	calc.Normalize(&nb, v.s.opt.Catalog)
	return nb
}

// CompareSets computes the columns; a column without pieces is the build
// as it is.
func (v *Views) CompareSets(b calc.Build, cols [][]SetPiece, l string) SetView {
	s := v.s
	c := s.opt.Catalog
	var out SetView
	if m := c.Monster(b.Target); m != nil {
		out.Target = m.NameIn(l)
	}
	base := cloneBuild(b)
	calc.Normalize(&base, c)
	cards := map[string][]CompareOpt{}
	var builds []calc.Build
	for _, pieces := range cols {
		nb := v.SetBuild(b, pieces)
		builds = append(builds, nb)
		var col SetCol
		for _, p := range pieces {
			e := nb.Equip[p.Slot]
			pv := SetPieceView{Slot: p.Slot, Label: tr(l, slotLabel(p.Slot)), Dropped: e.Item != p.Item}
			if it := c.Items[p.Item]; it != nil {
				iv := viewItem(it, l)
				pv.Item, pv.Refineable, pv.Gradable = &iv, it.Refineable, it.Gradable
			}
			pv.Refine, pv.Grade = p.Refine, p.Grade
			if !pv.Dropped {
				pv.Refine, pv.Grade = e.Refine, e.Grade
				if _, ok := cards[p.Slot]; !ok {
					cards[p.Slot] = s.compareCards(p.Slot, l)
				}
				pv.Positions = s.comparePositions(p.Slot, e, cards[p.Slot], l)
			}
			col.Pieces = append(col.Pieces, pv)
		}
		for _, sl := range calc.Slots {
			if base.Equip[sl].Item != 0 && nb.Equip[sl].Item == 0 {
				col.Left = append(col.Left, tr(l, slotLabel(sl)))
			}
		}
		out.Cols = append(out.Cols, col)
	}
	// The column as it is shows what the others take off.
	for i := range out.Cols {
		if len(cols[i]) > 0 {
			continue
		}
		for _, sl := range calc.Slots {
			e := base.Equip[sl]
			it := c.Items[e.Item]
			if it == nil || !slices.ContainsFunc(cols, func(ps []SetPiece) bool {
				return slices.ContainsFunc(ps, func(p SetPiece) bool { return p.Slot == sl })
			}) {
				continue
			}
			iv := viewItem(it, l)
			pv := SetPieceView{Slot: sl, Label: tr(l, slotLabel(sl)), CompareCol: CompareCol{Item: &iv, Refine: e.Refine, Grade: e.Grade}}
			for _, cd := range e.Cards {
				if ci := c.Items[cd]; ci != nil {
					pv.Worn = append(pv.Worn, ci.Name.Get(l))
				}
			}
			out.Cols[i].Pieces = append(out.Cols[i].Pieces, pv)
		}
	}
	out.Rows = s.compareRows(builds, l)
	return out
}

// GearSets lists the sets of two pieces or more the class can wear at its
// level, by name.
func (v *Views) GearSets(b calc.Build, l string) []GearSet {
	s := v.s
	c := s.opt.Catalog
	job := c.Jobs[b.Job]
	wears := func(it *gamedata.Item) bool {
		return it != nil && (it.Type == "Weapon" || it.Type == "Armor" || it.Type == "ShadowGear") &&
			job.CanEquip(it) && it.EquipLvMin <= b.BaseLv && it.Name.Get(l) != ""
	}
	hosts := map[int][]int{} // enchant -> the items that take it
	for id, pos := range c.Enchants {
		seen := map[int]bool{}
		for _, list := range pos {
			for _, e := range list {
				if !seen[e] {
					seen[e] = true
					hosts[e] = append(hosts[e], id)
				}
			}
		}
	}
	takes := func(id, ench int) bool { return slices.Contains(hosts[ench], id) }
	found := map[string][]int{}
	type cardSet struct{ gear, cards []int }
	var withCards []cardSet
	for _, cb := range c.Combos {
		var gear, cards []int
		ok := true
		for _, id := range cb.Items {
			it := c.Items[id]
			switch {
			case wears(it):
				gear = append(gear, id)
			case it != nil && it.Type == "Card" && it.SubType == "Enchant":
			case it != nil && it.Type == "Card" && it.Name.Get(l) != "":
				cards = append(cards, id)
			default:
				ok = false // a piece the class does not wear
			}
		}
		switch {
		case !ok:
		case len(cards) > 0 && len(gear) > 0: // a piece and the card(s) of its set
			sort.Ints(gear)
			withCards = append(withCards, cardSet{gear, cards})
		case len(cards) == 0 && len(gear) >= 2:
			sort.Ints(gear)
			found[fmt.Sprint(gear)] = gear
		}
	}
	// An enchant of a piece that forms a set with an enchant only one other
	// item takes brings that item along (the Ambivalente boots).
	for k, gear := range found {
		full := slices.Clone(gear)
		for _, cb := range c.Combos {
			// Every other part is in the set already or is an enchant of it.
			for i, y := range cb.Items {
				if len(hosts[y]) == 0 || len(hosts[y]) > 2 {
					continue
				}
				in, linked := true, false
				for k, x := range cb.Items {
					if k == i || slices.Contains(full, x) {
						continue
					}
					ench := slices.ContainsFunc(full, func(id int) bool { return takes(id, x) })
					in, linked = in && ench, linked || ench
				}
				if !in || !linked {
					continue
				}
				for _, h := range hosts[y] {
					if wears(c.Items[h]) && !slices.Contains(full, h) {
						if _, ok := s.placeGear(append(slices.Clone(full), h)); ok {
							full = append(full, h)
						}
					}
				}
			}
		}
		if len(full) > len(gear) {
			sort.Ints(full)
			delete(found, k)
			found[fmt.Sprint(full)] = full
		}
	}
	var out []GearSet
	for k, gear := range found {
		slots, ok := s.placeGear(gear)
		if !ok {
			continue
		}
		set := GearSet{Key: k}
		var names []string
		for i, id := range gear {
			set.Pieces = append(set.Pieces, s.newPiece(b, slots[i], id))
			names = append(names, c.Items[id].Name.Get(l))
		}
		set.Name = strings.Join(names, " + ")
		out = append(out, set)
	}
	for _, cs := range withCards {
		if set, ok := s.cardSet(b, cs.gear, cs.cards, l); ok {
			out = append(out, set)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// newPiece is an item at the refine and grade of what the slot wears.
func (s *Server) newPiece(b calc.Build, slot string, id int) SetPiece {
	e := b.Equip[slot]
	return SetPiece{Slot: slot, CompareCand: CompareCand{Item: id, Refine: e.Refine, Grade: e.Grade}}
}

// NewSetPiece is newPiece for the UI.
func (v *Views) NewSetPiece(b calc.Build, slot string, id int) SetPiece {
	return v.s.newPiece(b, slot, id)
}

// SetItems are the items a slot of a custom set can take, by name.
func (v *Views) SetItems(b calc.Build, slot, l string) []CompareOpt {
	c := v.s.opt.Catalog
	job := c.Jobs[b.Job]
	var out []CompareOpt
	for _, it := range c.Items {
		if it.Type != "" && it.Type != "Card" && calc.Fits(it, slot) && job.CanEquip(it) && it.EquipLvMin <= b.BaseLv && it.Name.Get(l) != "" {
			name := it.Name.Get(l)
			if it.Slots > 0 {
				name += fmt.Sprintf(" [%d]", it.Slots)
			}
			out = append(out, CompareOpt{it.ID, name})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// placeGear gives each item its own slot; ok false when they do not go on
// together.
func (s *Server) placeGear(ids []int) ([]string, bool) {
	c := s.opt.Catalog
	slots := make([]string, len(ids))
	used := map[string]bool{}
	var put func(i int) bool
	put = func(i int) bool {
		if i == len(ids) {
			return true
		}
		it := c.Items[ids[i]]
		for _, sl := range calc.Slots {
			if sl == calc.Ammo || calc.IsCostume(sl) || !calc.Fits(it, sl) {
				continue
			}
			takes := []string{sl}
			switch {
			case strings.HasPrefix(sl, "Head_"):
				for _, h := range []string{calc.HeadTop, calc.HeadMid, calc.HeadLow} {
					if h != sl && it.HasLocation(h) {
						takes = append(takes, h)
					}
				}
			case sl == calc.Weapon && it.HasLocation("Both_Hand"):
				takes = append(takes, calc.Shield)
			}
			if slices.ContainsFunc(takes, func(t string) bool { return used[t] }) {
				continue
			}
			for _, t := range takes {
				used[t] = true
			}
			slots[i] = sl
			if put(i + 1) {
				return true
			}
			for _, t := range takes {
				delete(used, t)
			}
		}
		return false
	}
	return slots, put(0)
}

// BestSetEnchants picks the enchants of the pieces that give the most
// expected damage: the sets the enchants form (Dom + Essência...) are tried
// whole, then each position by itself. Cards stay. Without a target, or
// when the attack does no damage, the pieces come back as they are.
func (v *Views) BestSetEnchants(ctx context.Context, b calc.Build, pieces []SetPiece) []SetPiece {
	s := v.s
	c := s.opt.Catalog
	type pos struct {
		piece, idx int
		opts       []int
	}
	start := make([]SetPiece, len(pieces))
	worn := v.SetBuild(b, pieces)
	var ps []pos
	opts := map[int]bool{}
	for i, p := range pieces {
		start[i] = p
		e := worn.Equip[p.Slot]
		it := c.Items[p.Item]
		if it == nil || e.Item != p.Item {
			continue
		}
		cards := e.Cards
		start[i].Cards = &cards
		for idx := it.Slots; idx < 4; idx++ {
			if l := c.Enchants[it.ID][idx]; len(l) > 0 {
				ps = append(ps, pos{i, idx, l})
				for _, id := range l {
					opts[id] = true
				}
			}
		}
	}
	score := func(x []SetPiece) float64 { return s.optExpected(v.SetBuild(b, x)) }
	if len(ps) == 0 || score(start) <= 0 {
		return start
	}
	with := func(x []SetPiece, p pos, id int) []SetPiece {
		y := slices.Clone(x)
		cards := *y[p.piece].Cards
		cards[p.idx] = id
		y[p.piece].Cards = &cards
		return y
	}
	// The sets with two enchants or more that fit the positions together.
	var sets [][]int
	for _, cb := range c.Combos {
		var en []int
		for _, id := range cb.Items {
			if opts[id] {
				en = append(en, id)
			}
		}
		if len(en) >= 2 {
			sets = append(sets, en)
		}
	}
	place := func(x []SetPiece, en []int) ([]SetPiece, bool) {
		used := map[int]bool{}
		for _, id := range en {
			ok := false
			for pi, p := range ps {
				if !used[pi] && slices.Contains(p.opts, id) {
					x, used[pi], ok = with(x, p, id), true, true
					break
				}
			}
			if !ok {
				return nil, false
			}
		}
		return x, true
	}
	starts := [][]SetPiece{start}
	for _, en := range sets {
		if x, ok := place(start, en); ok {
			starts = append(starts, x)
		}
	}
	best, bestScore := start, score(start)
	for _, x := range starts {
		sc := score(x)
		for range 3 {
			changed := false
			for _, p := range ps {
				for _, id := range p.opts {
					if ctx.Err() != nil {
						return best
					}
					if (*x[p.piece].Cards)[p.idx] == id {
						continue
					}
					y := with(x, p, id)
					if n := score(y); n > sc*1.000001 {
						x, sc, changed = y, n, true
					}
				}
			}
			if !changed {
				break
			}
		}
		if sc > bestScore {
			best, bestScore = x, sc
		}
	}
	return best
}

// cardSet is a set of pieces and cards: the pieces in their slots, each
// card in a free card position of a piece of the set that takes it, or
// else of the item the build wears where it goes. ok false when a card
// finds no free position.
func (s *Server) cardSet(b calc.Build, gear, cards []int, l string) (GearSet, bool) {
	c := s.opt.Catalog
	slots, ok := s.placeGear(gear)
	if !ok {
		return GearSet{}, false
	}
	set := GearSet{Key: fmt.Sprint(gear, cards), Cards: true}
	var names []string
	for i, id := range gear {
		set.Pieces = append(set.Pieces, s.newPiece(b, slots[i], id))
		names = append(names, c.Items[id].Name.Get(l))
	}
	// free finds a card position the piece in slot has open.
	free := func(p *SetPiece) int {
		it := c.Items[p.Item]
		if it == nil {
			return -1
		}
		cur := [4]int{}
		if p.Cards != nil {
			cur = *p.Cards
		}
		for i := 0; i < it.Slots && i < 4; i++ {
			if cur[i] == 0 {
				return i
			}
		}
		return -1
	}
	put := func(p *SetPiece, card int) bool {
		i := free(p)
		if i < 0 {
			return false
		}
		cur := [4]int{}
		if p.Cards != nil {
			cur = *p.Cards
		}
		cur[i] = card
		p.Cards = &cur
		return true
	}
	for _, card := range cards {
		cd := c.Items[card]
		placed := false
		for i := range set.Pieces {
			if calc.CardFits(cd, set.Pieces[i].Slot) && put(&set.Pieces[i], card) {
				placed = true
				break
			}
		}
		for _, slot := range calc.Slots {
			if placed {
				break
			}
			e, worn := b.Equip[slot]
			if !worn || slices.ContainsFunc(set.Pieces, func(p SetPiece) bool { return p.Slot == slot }) || !calc.CardFits(cd, slot) {
				continue
			}
			cards := e.Cards
			p := SetPiece{Slot: slot, CompareCand: CompareCand{Item: e.Item, Refine: e.Refine, Grade: e.Grade, Cards: &cards}}
			if put(&p, card) {
				set.Pieces = append(set.Pieces, p)
				placed = true
			}
		}
		if !placed {
			return GearSet{}, false
		}
		names = append(names, cd.Name.Get(l))
	}
	set.Name = strings.Join(names, " + ")
	return set, true
}
