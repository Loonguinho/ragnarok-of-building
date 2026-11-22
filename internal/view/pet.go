package view

import (
	"sort"
	"strconv"
	"strings"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

// petView is the Pet row of the Buffs window: the chosen pet (by its egg),
// its intimacy tier and what it gives.
type petView struct {
	Egg      int
	Tier     string      // intimacy tier option value (see PetTiers)
	Options  [][2]string // egg ID, name; sorted by name
	Gives    string      // the pet's bonuses at this intimacy, readable
	Intimacy int
}

// PetTiers are the intimacy tiers a pet script tells apart, as select
// options: the value is the intimacy set when picked.
var PetTiers = [][2]string{
	{"1000", "Leal (900–1000)"}, {"750", "Cordial (750–899)"}, {"250", "Neutro (250–749)"},
	{"100", "Tímido (100–249)"}, {"1", "Arisco (1–99)"},
}

// PetTier is the PetTiers value whose range holds intimacy.
func PetTier(intimacy int) string {
	for _, t := range PetTiers {
		lo, _ := strconv.Atoi(t[0])
		if t[0] == "1000" {
			lo = 900
		}
		if intimacy >= lo {
			return t[0]
		}
	}
	return "1"
}

func petWindow(b calc.Build, r calc.Result, c *gamedata.Catalog, l string) petView {
	v := petView{Egg: b.Pet, Intimacy: b.PetIntimacy, Tier: PetTier(b.PetIntimacy)}
	for egg := range c.Pets {
		if it := c.Items[egg]; it != nil {
			v.Options = append(v.Options, [2]string{strconv.Itoa(egg), it.Name.Get(l)})
		}
	}
	sort.Slice(v.Options, func(i, j int) bool { return v.Options[i][1] < v.Options[j][1] })
	var parts []string
	for _, m := range []map[string]int{r.PetBonus, r.PetExtra} {
		for _, k := range sortedKeys(m) {
			if m[k] == 0 {
				continue
			}
			lb, val := bonusText(k, m[k], c, l)
			parts = append(parts, strings.TrimSpace(lb+" "+val))
		}
	}
	v.Gives = strings.Join(parts, ", ")
	return v
}
