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
	// ASPD items without a LATAM consumable script (latam-ro-calc
	// aspd-potion-list.ts and calculator-controller.ts applyGuaranaCandy).
	guaranaCandy  = 12414 // Aumentar Agilidade Nv 5 + Poção da Concentração
	salamineJuice = 12437 // Suco Celular Enriquecido: ASPD +10%, stacks with the potions
	goldPotion    = 12684 // Poção de Ouro: fixed ASPD bonus 3 (× AGI ÷ 200), stacks
)

// ASPD potions: a fixed bonus × AGI ÷ 200 (rAthena SC_ASPDPOTION*, in
// status_calc_aspd's fixed group), LATAM values 4/6/9 (latam-ro-calc
// aspd-potion-list.ts). They replace one another: the strongest counts.
var aspdPotions = []struct{ Item, Fixed, Legacy int }{
	{657, 9, 20}, // Poção da Fúria Selvagem
	{656, 6, 15}, // Poção do Despertar
	{645, 4, 10}, // Poção da Concentração
}

// AspdPotionExcl is the exclusive group of the ASPD potions (one at a time).
const AspdPotionExcl = "aspd_potion"

// ConsumableExcl is the exclusive group of a consumable ("" = none).
func ConsumableExcl(item int) string {
	if aspdPotionFixed(item) > 0 {
		return AspdPotionExcl
	}
	return ""
}

func aspdPotionFixed(item int) int {
	for _, p := range aspdPotions {
		if p.Item == item {
			return p.Fixed
		}
	}
	return 0
}

// activeAspdPotion is the strongest ASPD potion on (0 = none).
func activeAspdPotion(b *Build) int {
	for _, p := range aspdPotions {
		if b.Buffs[ConsumableKey(p.Item)] > 0 {
			return p.Item
		}
	}
	return 0
}

// migrateAspdPotion turns the old "aspd_potion" selector (10/15/20) into
// the potion's consumable key.
func migrateAspdPotion(b *Build) {
	v, ok := b.Buffs[AspdPotionExcl]
	if !ok {
		return
	}
	delete(b.Buffs, AspdPotionExcl)
	for _, p := range aspdPotions {
		if p.Legacy == v {
			b.Buffs[ConsumableKey(p.Item)] = 1
		}
	}
}

// extraConsumables are offered without an Item.LatamUse script.
var extraConsumables = []int{hpPotionL, guaranaCandy, salamineJuice, goldPotion, 645, 656, 657}

// ConsumableKey is the Build.Buffs key of a consumable.
func ConsumableKey(item int) string { return ConsumablePrefix + strconv.Itoa(item) }

// Consumables lists the items the Buffs window offers as consumables: those
// with LATAM bonuses (Item.LatamUse) and extraConsumables.
func Consumables(c *gamedata.Catalog) []*gamedata.Item {
	var out []*gamedata.Item
	for id, it := range c.Items {
		if len(it.LatamUse) > 0 || slices.Contains(extraConsumables, id) {
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
		if it == nil || (id == battlePill && b.Buffs[ConsumableKey(superBattlePill)] > 0) ||
			(aspdPotionFixed(id) > 0 && id != activeAspdPotion(b)) {
			continue
		}
		res := script.Result{Bonus: map[string]int{}, Extra: map[string]int{}}
		vals := env.evalScript(it.LatamUse, id, "", false, 0, false)
		switch id {
		case spPotionL:
			vals["spPercent"] = float64(5 + b.BaseLv/10)
		case hpPotionL:
			res.Bonus["bMaxHP"] += 2500 + b.BaseLv*10/3
		case salamineJuice:
			res.Bonus["bAspdRate"] += 10
		case 645, 656, 657:
			res.Bonus["bAspdFixed"] += aspdPotionFixed(id)
		case goldPotion:
			res.Bonus["bAspdFixed"] += 3
		case guaranaCandy:
			// Neither part stacks with a stronger source: a chosen ASPD potion
			// replaces the Concentration one, Aumentar Agilidade Nv 5+ the buff.
			if activeAspdPotion(b) == 0 {
				res.Bonus["bAspdFixed"] += 4
			}
			if b.Buffs["incagi"] < 5 {
				res.Bonus["bAgi"] += 7
				res.Bonus["bAspdRate"] += 5
			}
		}
		addLatam(c, vals, &res)
		addAll(bonus, res.Bonus)
		addAll(extra, res.Extra)
	}
}
