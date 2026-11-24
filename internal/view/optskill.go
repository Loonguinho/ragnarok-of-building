package view

import (
	"context"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

// The optimizer's skill cards and card sets: the cards that boost the attack
// chosen are listed even when general cards beat them, and the sets of cards
// are tried whole.

const (
	optSkillMax = 3 // skill cards listed per position, besides the best ones
	optSetMax   = 5 // card sets listed, besides the attack's own
)

// boostsSkill reports a card or enchant with a bonus for the skill code.
func boostsSkill(it *gamedata.Item, code string) bool {
	if it == nil || code == "" {
		return false
	}
	src := it.Script
	if src == "" {
		src = it.DescScript
	}
	return codeIn(src, code)
}

func codeIn(script, code string) bool {
	for _, m := range optSkillCode.FindAllStringSubmatch(script, -1) {
		if m[1] == code {
			return true
		}
	}
	return false
}

// attackCode is the code of the attack's skill ("" for the basic attack).
func (s *Server) attackCode(b calc.Build) string {
	if sk := s.opt.Catalog.Skills[b.Atk.Skill]; sk != nil {
		return sk.Code
	}
	return ""
}

// skillCards are the cards of a position that boost the attack's skill and
// are not among the options yet; those that only pay off at a higher refine
// or grade come with Later.
func (s *Server) skillCards(p optPosition, vals, later []float64, base float64, have []optOption, code string, pct func(float64) string, tag, l string) []optOption {
	if p.item || p.opts != nil || p.enchant || code == "" {
		return nil
	}
	var idx []int
	for k, it := range p.pool {
		listed := slices.ContainsFunc(have, func(o optOption) bool { return o.Item.ID == it.ID })
		if listed || it.ID == p.current || !boostsSkill(it, code) || vals[k] <= base*1.0005 && later[k] <= 1.003 {
			continue
		}
		idx = append(idx, k)
	}
	sort.SliceStable(idx, func(a, z int) bool { return vals[idx[a]] > vals[idx[z]] })
	var out []optOption
	for _, k := range idx[:min(len(idx), optSkillMax)] {
		out = append(out, optOption{Item: viewItem(p.pool[k], l), Kind: "card", Value: fmtInt(int64(vals[k] + 0.5)), Gain: pct(vals[k]),
			Slot: p.slots[0], Index: p.index, Positive: true, Tag: tag, Later: vals[k] <= base*1.0005})
	}
	return out
}

// cardSets tries every set of cards that fits the items worn, each card in
// the position where the whole scores best. The sets that boost the
// attack's skill are tagged and listed even when others beat them; missing
// names those of them that need a card slot the items do not have.
func (s *Server) cardSets(ctx context.Context, b calc.Build, o OptOptions, tag, l string) (opts []optOption, tested int, missing []string) {
	c := s.opt.Catalog
	base := s.optScore(b, o.Goal)
	if base <= 0 {
		return nil, 0, nil
	}
	code := s.attackCode(b)
	// The card positions there are.
	type place struct {
		slot string
		idx  int
	}
	var places []place
	done := map[string]bool{}
	for _, info := range append(append([]slotInfo{}, equipSlots...), shadowSlots...) {
		host := c.Items[b.Equip[info.Key].Item]
		if host == nil || done[info.Key] {
			continue
		}
		for _, k := range linkedSlots(b, info.Key) {
			done[k] = true
		}
		for i := range min(host.Slots, 4) {
			places = append(places, place{info.Key, i})
		}
	}
	type set struct {
		cards []int
		skill bool
		parts []ComboPart
		score float64
	}
	var sets []set
	seen := map[string]bool{}
next:
	for _, cb := range c.Combos {
		if len(cb.Items) < 2 || len(cb.Items) > 4 {
			continue
		}
		skill := codeIn(cb.Script, code)
		key := ""
		var names, noSlot []string
		ids := slices.Clone(cb.Items)
		slices.Sort(ids)
		for _, id := range ids {
			it := c.Items[id]
			if it == nil || it.Type != "Card" || it.SubType == "Enchant" || it.Name.Get("pt") == "" || o.NoMVP && it.MVPCard {
				continue next
			}
			names = append(names, it.Name.Get(l))
			if !slices.ContainsFunc(places, func(p place) bool { return calc.CardFits(it, p.slot) }) {
				noSlot = append(noSlot, it.Name.Get(l))
			}
			skill = skill || boostsSkill(it, code)
			key += it.Name.Get("pt") + "|"
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		if len(noSlot) > 0 {
			if skill && code != "" && len(missing) < optSkillMax {
				missing = append(missing, strings.Join(names, " + ")+" ("+tr(l, "falta slot para")+" "+strings.Join(noSlot, ", ")+")")
			}
			continue
		}
		sets = append(sets, set{cards: ids, skill: skill && code != ""})
	}
	// place puts the cards one after the other, trying every position left.
	var try func(nb calc.Build, cards []int, used map[place]bool, parts []ComboPart, st *set)
	try = func(nb calc.Build, cards []int, used map[place]bool, parts []ComboPart, st *set) {
		if ctx.Err() != nil {
			return
		}
		if len(cards) == 0 {
			if sc := s.optScore(nb, o.Goal); sc > st.score {
				st.score, st.parts = sc, slices.Clone(parts)
			}
			return
		}
		id := cards[0]
		it := c.Items[id]
		// Already worn: it stays where it is.
		for _, p := range places {
			if !used[p] && b.Equip[p.slot].Cards[p.idx] == id {
				u := maps.Clone(used)
				u[p] = true
				try(nb, cards[1:], u, parts, st)
				return
			}
		}
		for _, p := range places {
			if used[p] || !calc.CardFits(it, p.slot) {
				continue
			}
			u := maps.Clone(used)
			u[p] = true
			nx := editSlots(nb, linkedSlots(nb, p.slot), func(e *calc.Equip) { e.Cards[p.idx] = id })
			part := ComboPart{Slot: p.slot, Idx: p.idx, Card: id, CardName: it.Name.Get(l), SlotLabel: tr(l, slotLabel(p.slot))}
			try(nx, cards[1:], u, append(slices.Clone(parts), part), st)
		}
	}
	runPar(ctx, len(sets), func(i int) { try(b, sets[i].cards, map[place]bool{}, nil, &sets[i]) })
	tested = len(sets)
	sort.SliceStable(sets, func(i, j int) bool { return sets[i].score > sets[j].score })
	n, sk := 0, 0
	for i, st := range sets {
		if st.score <= base*1.0005 || len(st.parts) < 2 {
			continue // one card left to place: it is in its own position
		}
		mine := st.skill && sk < optSkillMax
		if n >= optSetMax && !mine {
			continue
		}
		op := optOption{Item: viewItem(c.Items[st.parts[0].Card], l), Kind: "set", Value: fmtInt(int64(st.score + 0.5)),
			Gain: pctOf(st.score/base, l), Slot: "cards", Index: i, Parts: st.parts, Positive: true}
		if st.skill {
			op.Tag = tag
			sk++
		}
		n++
		opts = append(opts, op)
	}
	return opts, tested, missing
}

// multiTop is how many cards are combined in an item with several slots.
const multiTop = 10

// multiCards tries the cards of an item with several card slots together, a
// card more than once included: each position alone does not show what two
// of one card give.
func (s *Server) multiCards(ctx context.Context, b calc.Build, pos []optPosition, vals, later [][]float64, o OptOptions, code, tag, l string) (groups []optGroup, tested int) {
	c := s.opt.Catalog
	base := s.optScore(b, o.Goal)
	if base <= 0 {
		return nil, 0
	}
	for pi, p := range pos {
		host := c.Items[b.Equip[p.slots[0]].Item]
		if p.item || p.enchant || p.opts != nil || p.index != 0 || host == nil || host.Slots < 2 {
			continue
		}
		n := min(host.Slots, 4)
		// The best cards alone, and the attack's own.
		idx := make([]int, 0, len(p.pool))
		for k := range p.pool {
			if vals[pi][k] > base*1.0005 {
				idx = append(idx, k)
			}
		}
		sort.SliceStable(idx, func(a, z int) bool { return vals[pi][idx[a]] > vals[pi][idx[z]] })
		var cards []int
		mine := map[int]bool{}
		for i, k := range idx {
			it := p.pool[k]
			if boostsSkill(it, code) {
				mine[it.ID] = true
			}
			if i < multiTop || mine[it.ID] && len(mine) <= optSkillMax {
				cards = append(cards, it.ID)
			}
		}
		for k, it := range p.pool { // those that only pay off at a higher refine
			if boostsSkill(it, code) && !mine[it.ID] && later[pi][k] > 1.003 && len(mine) < optSkillMax {
				mine[it.ID] = true
				cards = append(cards, it.ID)
			}
		}
		// What is there now may stay in some of the slots.
		for i := range n {
			if cd := b.Equip[p.slots[0]].Cards[i]; cd != 0 && !slices.Contains(cards, cd) {
				cards = append(cards, cd)
			}
		}
		type combo struct {
			cards []int
			score float64
		}
		var list []combo
		var build func(from int, cur []int)
		build = func(from int, cur []int) {
			if len(cur) == n {
				list = append(list, combo{cards: slices.Clone(cur)})
				return
			}
			for i := from; i < len(cards); i++ {
				build(i, append(cur, cards[i]))
			}
		}
		build(0, nil)
		fill := func(ids []int) func(e *calc.Equip) {
			return func(e *calc.Equip) {
				for i, id := range ids {
					e.Cards[i] = id
				}
			}
		}
		runPar(ctx, len(list), func(i int) {
			list[i].score = s.optScore(editSlots(b, p.slots, fill(list[i].cards)), o.Goal)
		})
		tested += len(list)
		sort.SliceStable(list, func(i, j int) bool { return list[i].score > list[j].score })
		label := tr(l, p.label) + " · " + strconv.Itoa(n) + " " + tr(l, "cartas juntas")
		g := optGroup{Kind: "set", Slot: p.slots[0], Label: label}
		own, best := 0, 0
		for _, cb := range list {
			all := len(mine) > 0 // only the attack's cards
			some := false
			count := map[int]int{}
			for _, id := range cb.cards {
				all = all && mine[id]
				some = some || mine[id]
				count[id]++
			}
			gains := cb.score > base*1.0005
			tagged := (all || some && own == 0 && gains) && own < optSkillMax
			if !gains && !all || best >= optTop && !tagged {
				continue
			}
			best++
			var names []string
			var parts []ComboPart
			done := map[int]bool{}
			for i, id := range cb.cards {
				cn := c.Items[id].Name.Get(l)
				parts = append(parts, ComboPart{Slot: p.slots[0], Idx: i, Card: id, CardName: cn, SlotLabel: tr(l, p.label)})
				if done[id] {
					continue
				}
				done[id] = true
				if count[id] > 1 {
					cn = strconv.Itoa(count[id]) + "× " + cn
				}
				names = append(names, cn)
			}
			op := optOption{Item: viewItem(c.Items[cb.cards[0]], l), Kind: "set", Name: strings.Join(names, " + "),
				Value: fmtInt(int64(cb.score + 0.5)), Gain: strings.Replace(pctOf(cb.score/base, l), "+-", "-", 1),
				Slot: p.slots[0], Index: len(g.Options), Parts: parts, Positive: gains, Later: !gains}
			if slices.ContainsFunc(cb.cards, func(id int) bool { return scalesWithItem(c.Items[id]) }) {
				op.Steps = s.stepsOf(ctx, b, p.slots, fill(cb.cards), o.Goal, l)
			}
			if tagged {
				op.Tag = tag
				own++
			}
			g.Options = append(g.Options, op)
		}
		if len(g.Options) > 0 {
			groups = append(groups, g)
		}
	}
	return groups, tested
}
