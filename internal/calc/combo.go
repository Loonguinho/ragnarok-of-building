package calc

import (
	"slices"

	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/script"
)

// itemScript is the rAthena script, or for LATAM-only items the one read
// from the client description (counting the lines it couldn't read).
func itemScript(it *gamedata.Item, skipped *int) string {
	if it.Script != "" {
		return it.Script
	}
	*skipped += it.DescSkipped
	return it.DescScript
}

// multiHead reports whether an item covers several headgear slots at once
// (upper + middle...), so it is stored in each of them but worn once. Items
// with alternative slots (accessories: left or right) are not.
func multiHead(it *gamedata.Item) bool {
	n := 0
	for _, l := range it.Locations {
		if l == HeadTop || l == HeadMid || l == HeadLow {
			n++
		}
	}
	c := 0 // costumes too (upper + middle visuals)
	for _, l := range it.Locations {
		if l == CosTop || l == CosMid || l == CosLow {
			c++
		}
	}
	return n > 1 || c > 1
}

// applyCombos adds the scripts of every item combo (rAthena item_combos.yml)
// whose items are all equipped, cards included. An item listed twice needs
// two copies (e.g. a pair of rings); an item worn in several slots (upper +
// middle headgear) counts once. Returns the active sets and what each adds.
func applyCombos(c *gamedata.Catalog, b *Build, ctx script.Context, bonus, extra map[string]int, skipped *int) ([][]int, []ComboEffect) {
	if len(c.Combos) == 0 {
		return nil, nil
	}
	have := map[int]int{}
	counted := map[int]bool{}
	for _, slot := range Slots {
		e, ok := b.Equip[slot]
		if !ok || e.Item == 0 {
			continue
		}
		if it := c.Items[e.Item]; it != nil && multiHead(it) {
			if counted[e.Item] {
				continue
			}
			counted[e.Item] = true
		}
		have[e.Item]++
		for _, cd := range e.Cards {
			if cd > 0 {
				have[cd]++
			}
		}
	}
	if b.Pet > 0 { // the pet's egg completes sets like an equipped item
		have[b.Pet]++
	}
	var cand []int
	for id := range have {
		cand = append(cand, c.CombosWith(id)...)
	}
	slices.Sort(cand)
	cand = slices.Compact(cand)
	var active [][]int
	var effects []ComboEffect
	for _, i := range cand {
		cb := c.Combos[i]
		need := map[int]int{}
		for _, id := range cb.Items {
			need[id]++
		}
		ok := true
		for id, n := range need {
			if have[id] < n {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		res := script.Eval(cb.Script, ctx)
		addAll(bonus, res.Bonus)
		addAll(extra, res.Extra)
		*skipped += res.Skipped
		active = append(active, cb.Items)
		effects = append(effects, ComboEffect{Items: cb.Items, Bonus: res.Bonus, Extra: res.Extra, Skipped: res.Skipped})
	}
	return active, effects
}

// ComboEffect is what one active item combo adds: status bonuses (bStr,
// bAtkRate...), damage modifiers (bonus2+), and how many of its effects
// the evaluator could not apply.
type ComboEffect struct {
	Items        []int
	Bonus, Extra map[string]int
	Skipped      int
}
