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
	return n > 1
}

// applyCombos adds the scripts of every item combo (rAthena item_combos.yml)
// whose items are all equipped, cards included. An item listed twice needs
// two copies (e.g. a pair of rings); an item worn in several slots (upper +
// middle headgear) counts once. Returns the active sets.
func applyCombos(c *gamedata.Catalog, b *Build, param map[string]int, equipIDs map[int]bool,
	bonus, extra map[string]int, skipped *int) [][]int {
	if len(c.Combos) == 0 {
		return nil
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
	var cand []int
	for id := range have {
		cand = append(cand, c.CombosWith(id)...)
	}
	slices.Sort(cand)
	cand = slices.Compact(cand)
	var active [][]int
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
		res := script.Eval(cb.Script, script.Context{BaseLv: b.BaseLv, JobLv: b.JobLv, Param: param, Class: b.Job, EquipIDs: equipIDs})
		addAll(bonus, res.Bonus)
		addAll(extra, res.Extra)
		*skipped += res.Skipped
		active = append(active, cb.Items)
	}
	return active
}
