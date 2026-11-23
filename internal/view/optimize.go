package view

import (
	"context"
	"maps"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

// Optimizer, part 1: for every card slot (and every enchant position in use)
// of the equipped items, try each card that fits, one at a time, and rank them
// by the expected damage against the damage window's target with its attack.

const (
	optTop     = 5               // suggestions per position
	optTimeout = 8 * time.Second // the rest of the positions are skipped after this
)

// OptOptions say what the optimizer tries and what it raises.
type OptOptions struct {
	Goal                   string // GoalDamage (default), GoalPhysDef, GoalMagDef
	Items, Cards, Enchants bool
	Refine, Grade          int // of the items it suggests (those that take them)
	Traits                 bool // also search trait distributions (4th classes)
}

type optOption struct {
	Item     itemView
	Kind     string // "item", "card" or "enchant"
	Refine   int    // items: the refine and grade it was measured at
	Grade    int
	Value    string // expected damage
	Gain     string // "+18,4%"
	Skill    string // skill enchants: the skill the gain is measured on
	Slot     string
	Index    int
	Positive bool
}

type optGroup struct {
	Kind        string // "item", "card" or "enchant"
	Slot, Label string
	Index       int
	Enchant     bool
	Current     *itemView
	Options     []optOption
	// With no options: Best = the current card/enchant already gives the
	// most; otherwise nothing in the pool adds damage to this attack, and
	// SkillOnly says the pool is skill enchants (pick that skill to see them).
	Best      bool
	SkillOnly bool
	// SkillHeld: the position holds an enchant for this learned skill; a
	// swap to another skill's enchant can't be judged without knowing the
	// main skill, so none is suggested.
	SkillHeld string
}

type optResult struct {
	Goal     string
	NoTarget bool
	NoItems  bool
	Base     string
	Target   string
	Attack   string
	Groups   []optGroup
	NoEnch   []string // enchanted items the client's enchant list doesn't cover
	Tested   int
	Partial  bool // stopped by the time limit
	// Traits: the best trait distribution found, with its enchants; nil when
	// none beats the build's.
	Traits *TraitCol
}

// optPosition is one card position to optimize.
type optPosition struct {
	item    bool     // the slot's item itself (index -1)
	slots   []string // linked slots (one headgear covering several)
	label   string
	index   int
	enchant bool
	current int
	pool    []*gamedata.Item
}

// optCards lists the cards once (the catalog never changes).
func (s *Server) optCards() []*gamedata.Item {
	s.optOnce.Do(func() {
		for _, it := range s.opt.Catalog.Items {
			if it.Type == "Card" && it.SubType != "Enchant" && it.Name.Get("pt") != "" {
				s.optCardList = append(s.optCardList, it)
			}
		}
		sort.Slice(s.optCardList, func(i, j int) bool { return s.optCardList[i].ID < s.optCardList[j].ID })
	})
	return s.optCardList
}

// optExpected is the objective: expected damage per use of the attack. The
// build must already be normalized (Normalize is not safe to run in parallel
// on builds sharing maps).
func (s *Server) optExpected(b calc.Build) float64 {
	r := calc.Compute(b, s.opt.Catalog)
	return s.damageView(b, r, "pt").Dmg.Expected
}

func (s *Server) optimize(ctx context.Context, b calc.Build, enchants bool, l string) optResult {
	return s.optimizeWith(ctx, b, OptOptions{Goal: GoalDamage, Cards: true, Enchants: enchants}, l)
}

func (s *Server) optimizeWith(ctx context.Context, b calc.Build, o OptOptions, l string) optResult {
	c := s.opt.Catalog
	if o.Goal == "" {
		o.Goal = GoalDamage
	}
	enchants := o.Enchants
	// A copy: Normalize and the swaps write to the maps, the caller's.
	b = cloneBuild(b)
	calc.Normalize(&b, c)
	res := optResult{Goal: o.Goal}
	m := c.Monster(b.Target)
	if m == nil {
		res.NoTarget = true
		return res
	}
	res.Target = m.NameIn(l)
	res.Attack = tr(l, "Ataque básico")
	if sk := c.Skills[b.Atk.Skill]; sk != nil {
		res.Attack = sk.Name.Get(l) + " " + tr(l, "Nv.") + " " + strconv.Itoa(b.Atk.Level)
	}
	base := s.optScore(b, o.Goal)
	res.Base = fmtInt(int64(base + 0.5))

	cards := s.optCards()
	var pos []optPosition
	if o.Items {
		job := c.Jobs[b.Job]
		for _, info := range append(append([]slotInfo{}, equipSlots...), shadowSlots...) {
			var pool []*gamedata.Item
			for _, it := range c.Items {
				if it.Type != "" && it.Type != "Card" && calc.Fits(it, info.Key) && job.CanEquip(it) && it.Name.Get(l) != "" {
					pool = append(pool, it)
				}
			}
			sort.Slice(pool, func(i, j int) bool { return pool[i].ID < pool[j].ID })
			if len(pool) > 0 {
				pos = append(pos, optPosition{item: true, slots: []string{info.Key}, label: info.Label, index: -1,
					current: b.Equip[info.Key].Item, pool: pool})
			}
		}
	}
	done := map[string]bool{}
	for _, info := range append(append([]slotInfo{}, equipSlots...), shadowSlots...) {
		e, ok := b.Equip[info.Key]
		it := c.Items[e.Item]
		if !ok || it == nil || done[info.Key] {
			continue
		}
		linked := []string{info.Key}
		if strings.HasPrefix(info.Key, "Head_") {
			for _, h := range []string{calc.HeadTop, calc.HeadMid, calc.HeadLow} {
				if h != info.Key && b.Equip[h].Item == e.Item {
					linked = append(linked, h)
				}
			}
		}
		for _, k := range linked {
			done[k] = true
		}
		var pool []*gamedata.Item
		for _, cd := range cards {
			if calc.CardFits(cd, info.Key) {
				pool = append(pool, cd)
			}
		}
		// Enchants: only what the client's enchant list gives this item.
		ench := c.Enchants[it.ID]
		for i := 0; i < 4; i++ {
			p := optPosition{slots: linked, label: info.Label, index: i, current: e.Cards[i]}
			switch {
			case i < it.Slots:
				if !o.Cards {
					continue
				}
				p.pool = pool
			case enchants && len(ench[i]) > 0:
				p.enchant = true
				for _, id := range ench[i] {
					if ei := c.Items[id]; ei != nil {
						p.pool = append(p.pool, ei)
					}
				}
			default:
				if enchants && ench == nil && e.Cards[i] != 0 {
					if name := it.Name.Get(l); !slices.Contains(res.NoEnch, name) {
						res.NoEnch = append(res.NoEnch, name)
					}
				}
				continue
			}
			pos = append(pos, p)
		}
	}
	if len(pos) == 0 {
		res.NoItems = true
		return res
	}

	// Evaluate every (position, card) in parallel.
	type job struct{ p, k int }
	vals := make([][]float64, len(pos))
	var jobs []job
	for pi, p := range pos {
		vals[pi] = make([]float64, len(p.pool))
		for k := range p.pool {
			vals[pi][k] = -1
			jobs = append(jobs, job{pi, k})
		}
	}
	ctx, cancel := context.WithTimeout(ctx, optTimeout)
	defer cancel()
	ch := make(chan job)
	var wg sync.WaitGroup
	for w := 0; w < max(1, runtime.NumCPU()-1); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				p := pos[j.p]
				if p.item {
					if p.pool[j.k].ID == p.current {
						continue // the item itself (at another refine) is not a swap
					}
					vals[j.p][j.k] = s.optScore(s.swapAt(b, p.slots[0], p.pool[j.k].ID, o.Refine, o.Grade), o.Goal)
					continue
				}
				nb := b
				nb.Equip = maps.Clone(b.Equip)
				for _, k := range p.slots {
					e := nb.Equip[k]
					e.Cards[p.index] = p.pool[j.k].ID
					nb.Equip[k] = e
				}
				vals[j.p][j.k] = s.optScore(nb, o.Goal)
			}
		}()
	}
feed:
	for _, j := range jobs {
		select {
		case ch <- j:
		case <-ctx.Done():
			res.Partial = true
			break feed
		}
	}
	close(ch)
	wg.Wait()

	pct := func(v float64) string {
		if base <= 0 {
			return ""
		}
		g := (v/base - 1) * 100
		txt := strconv.FormatFloat(g, 'f', 1, 64)
		if l != "en" {
			txt = strings.Replace(txt, ".", ",", 1)
		}
		if g >= 0 {
			txt = "+" + txt
		}
		return txt + "%"
	}
	for pi, p := range pos {
		label := tr(l, p.label) + " · " + tr(l, "Carta") + " " + strconv.Itoa(p.index+1)
		kind := "card"
		switch {
		case p.item:
			label, kind = tr(l, p.label), "item"
		case p.enchant:
			label, kind = tr(l, p.label)+" · "+tr(l, "Encantamento")+" (slot "+strconv.Itoa(p.index+1)+")", "enchant"
		}
		g := optGroup{Kind: kind, Slot: p.slots[0], Label: label, Index: p.index, Enchant: p.enchant}
		if ci := c.Items[p.current]; p.current != 0 && ci != nil {
			v := viewItem(ci, l)
			g.Current = &v
		}
		idx := make([]int, 0, len(p.pool))
		for k, v := range vals[pi] {
			if v >= 0 {
				res.Tested++
				if v > base*1.0005 && p.pool[k].ID != p.current {
					idx = append(idx, k)
				}
			}
		}
		sort.SliceStable(idx, func(a, z int) bool { return vals[pi][idx[a]] > vals[pi][idx[z]] })
		seen := map[string]bool{}
		for _, k := range idx {
			if len(g.Options) >= optTop {
				break
			}
			// Tiers of one card/enchant ("… 10", "… 9") show only the best.
			fam := optFamily(p.pool[k].Name.Get(l))
			if seen[fam] {
				continue
			}
			seen[fam] = true
			opt := optOption{Item: viewItem(p.pool[k], l), Kind: kind, Value: fmtInt(int64(vals[pi][k] + 0.5)),
				Gain: pct(vals[pi][k]), Slot: g.Slot, Index: p.index, Positive: true}
			if p.item {
				// What it was measured at: Normalize drops what the item can't take.
				if p.pool[k].Refineable {
					opt.Refine = o.Refine
				}
				if p.pool[k].Gradable {
					opt.Grade = o.Grade
				}
			}
			g.Options = append(g.Options, opt)
		}
		// Skill enchants that don't touch the chosen attack: measure each on
		// the learned skill it boosts.
		if len(g.Options) == 0 && p.enchant && o.Goal == GoalDamage {
			if heldID, held := s.optHeldSkill(b, p.current, l); heldID != 0 {
				if heldID != b.Atk.Skill { // for the attack itself it is simply the best
					g.SkillHeld = held
				}
			} else {
				g.Options = s.optSkillEnchants(b, p, l)
			}
		}
		if len(g.Options) == 0 {
			// Keep the position listed, saying why nothing is suggested.
			g.Best = p.current != 0
			if !g.Best && p.enchant {
				g.SkillOnly = true
				for _, it := range p.pool {
					if !strings.Contains(it.Script, "bSkillAtk") {
						g.SkillOnly = false
						break
					}
				}
			}
		}
		res.Groups = append(res.Groups, g)
	}
	// Biggest gains first.
	sort.SliceStable(res.Groups, func(a, z int) bool {
		return gainOf(res.Groups[a]) > gainOf(res.Groups[z])
	})
	return res
}

// optSkillCode finds the skills an enchant boosts (bonus2 bSkillAtk,"CODE",n).
var optSkillCode = regexp.MustCompile(`bSkillAtk\s*,\s*"([A-Z0-9_]+)"`)

// optHeldSkill is the learned skill (ID, name) the enchant id boosts; 0 if none.
func (s *Server) optHeldSkill(b calc.Build, id int, l string) (int, string) {
	c := s.opt.Catalog
	it := c.Items[id]
	if it == nil {
		return 0, ""
	}
	for _, m := range optSkillCode.FindAllStringSubmatch(it.Script, -1) {
		for sid, lv := range b.Skills {
			if sk := c.Skills[sid]; sk != nil && lv > 0 && sk.Code == m[1] {
				return sid, sk.Name.Get(l)
			}
		}
	}
	return 0, ""
}

// optSkillEnchants ranks a position's skill enchants by the gain each gives
// to the learned skill it boosts (that skill as the attack, at its learned
// level), best first. Runs after the parallel pass; there are few of them.
func (s *Server) optSkillEnchants(b calc.Build, p optPosition, l string) []optOption {
	c := s.opt.Catalog
	learned := map[string]int{} // skill code -> ID, learned skills only
	for id, lv := range b.Skills {
		if sk := c.Skills[id]; sk != nil && lv > 0 {
			learned[sk.Code] = id
		}
	}
	type hit struct {
		opt  optOption
		gain float64
	}
	var hits []hit
	bases := map[int]float64{}
	for _, it := range p.pool {
		if it.ID == p.current {
			continue
		}
		for _, m := range optSkillCode.FindAllStringSubmatch(it.Script, -1) {
			id, ok := learned[m[1]]
			if !ok {
				continue
			}
			sb := b
			sb.Atk = calc.AttackChoice{Skill: id, Level: b.Skills[id]}
			if _, ok := bases[id]; !ok {
				bases[id] = s.optExpected(sb)
			}
			base := bases[id]
			sb.Equip = maps.Clone(b.Equip)
			for _, k := range p.slots {
				e := sb.Equip[k]
				e.Cards[p.index] = it.ID
				sb.Equip[k] = e
			}
			v := s.optExpected(sb)
			if base <= 0 || v <= base*1.0005 {
				continue
			}
			g := v / base
			hits = append(hits, hit{optOption{Item: viewItem(it, l), Value: fmtInt(int64(v + 0.5)), Gain: pctOf(g, l),
				Skill: c.Skills[id].Name.Get(l), Slot: p.slots[0], Index: p.index, Positive: true}, g})
			break
		}
	}
	sort.SliceStable(hits, func(a, z int) bool { return hits[a].gain > hits[z].gain })
	var out []optOption
	seen := map[string]bool{}
	for _, h := range hits {
		if len(out) >= optTop {
			break
		}
		if fam := optFamily(h.opt.Item.Name); !seen[fam] {
			seen[fam] = true
			out = append(out, h.opt)
		}
	}
	return out
}

// pctOf formats a damage ratio as a gain ("+12,5%").
func pctOf(ratio float64, l string) string {
	txt := strconv.FormatFloat((ratio-1)*100, 'f', 1, 64)
	if l != "en" {
		txt = strings.Replace(txt, ".", ",", 1)
	}
	return "+" + txt + "%"
}

var optTier = regexp.MustCompile(`\s+(\+?\d+|[IVX]+|(?i:nv|lv)\.?\s*\d+)$`)

// optFamily strips a trailing tier ("Insígnia do Talento 10" -> "Insígnia do Talento").
func optFamily(name string) string {
	return strings.ToLower(optTier.ReplaceAllString(strings.TrimSpace(name), ""))
}

func gainOf(g optGroup) float64 {
	if len(g.Options) == 0 {
		return 0
	}
	f, _ := strconv.ParseFloat(strings.NewReplacer("+", "", "%", "", ",", ".").Replace(g.Options[0].Gain), 64)
	return f
}
