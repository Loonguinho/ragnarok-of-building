package calc

import (
	"slices"
	"strconv"
	"strings"

	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/script"
)

// ConsumablePrefix keys a consumable in Build.Buffs ("use:12418" = 1).
const ConsumablePrefix = "use:"

// Consumables with rules of their own (latam-ro-calc calculator-controller.ts).
const (
	hpPotionL       = 12424 // HP Increase Potion (L): MaxHP +2500 + BaseLv*10/3
	spPotionL       = 12427 // SP Increase Potion (L): MaxSP +(5 + BaseLv/10)%
	battlePill      = 12791
	superBattlePill = 12792 // replaces the regular Battle Pill
)

// ConsumableKey is the Build.Buffs key of a consumable.
func ConsumableKey(item int) string { return ConsumablePrefix + strconv.Itoa(item) }

// Consumables lists the items the Buffs window offers as consumables: those
// with LATAM bonuses (Item.LatamUse) and the HP Increase Potion (L).
func Consumables(c *gamedata.Catalog) []*gamedata.Item {
	var out []*gamedata.Item
	for id, it := range c.Items {
		if len(it.LatamUse) > 0 || id == hpPotionL {
			out = append(out, it)
		}
	}
	slices.SortFunc(out, func(a, b *gamedata.Item) int { return strings.Compare(a.Name.Get("pt"), b.Name.Get("pt")) })
	return out
}

func applyConsumables(b *Build, c *gamedata.Catalog, env *latamEnv, bonus, extra map[string]int) {
	for k := range b.Buffs {
		s, ok := strings.CutPrefix(k, ConsumablePrefix)
		if !ok {
			continue
		}
		id, _ := strconv.Atoi(s)
		it := c.Items[id]
		if it == nil || (id == battlePill && b.Buffs[ConsumableKey(superBattlePill)] > 0) {
			continue
		}
		res := script.Result{Bonus: map[string]int{}, Extra: map[string]int{}}
		vals := env.evalScript(it.LatamUse, id, "", false, 0, false)
		switch id {
		case spPotionL:
			vals["spPercent"] = float64(5 + b.BaseLv/10)
		case hpPotionL:
			res.Bonus["bMaxHP"] += 2500 + b.BaseLv*10/3
		}
		addLatam(c, vals, &res)
		addAll(bonus, res.Bonus)
		addAll(extra, res.Extra)
	}
}
