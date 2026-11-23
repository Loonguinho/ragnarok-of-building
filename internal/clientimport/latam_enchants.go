package clientimport

import (
	_ "embed"
	"encoding/json"
	"slices"

	"ragnarok-of-building/internal/gamedata"
)

// latamEnchantsJSON is latam-ro-calc's enchant table (MIT), dumped by
// tools/latamgen/enchantgen.ts: the pools by AegisName, positions = card
// indexes 0-3, plus the weapons Malangdo's Snow enchants.
//
//go:embed latam_enchants.json
var latamEnchantsJSON []byte

type latamEnchantTable struct {
	ByName   map[string][][]string `json:"byName"`
	Malangdo struct {
		Weapons []int    `json:"weapons"`
		Pool    []string `json:"pool"`
	} `json:"malangdo"`
}

// addLatamEnchants gives the items the client's enchant list leaves out
// (Brasão de Schmidt, Botas Temporais, the Good and Evil gear...) the
// enchants latam-ro-calc lists for them. An item the client lists keeps
// the client's list. It returns how many items it filled.
func addLatamEnchants(c *gamedata.Catalog, aegis map[string]int) int {
	var t latamEnchantTable
	if json.Unmarshal(latamEnchantsJSON, &t) != nil {
		return 0
	}
	ids := func(names []string) []int {
		var out []int
		for _, n := range names {
			if id, ok := aegis[n]; ok && c.Items[id] != nil && !slices.Contains(out, id) {
				out = append(out, id)
			}
		}
		slices.Sort(out)
		return out
	}
	have := func(id int) bool { return len(c.Enchants[id]) > 0 }
	n := 0
	set := func(item int, pos map[int][]int) {
		if c.Items[item] == nil || have(item) || len(pos) == 0 {
			return
		}
		c.Enchants[item] = pos
		n++
	}
	names := make([]string, 0, len(t.ByName))
	for name := range t.ByName {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		item, ok := aegis[name]
		if !ok {
			continue
		}
		pos := map[int][]int{}
		for k, list := range t.ByName[name] {
			if e := ids(list); len(e) > 0 {
				pos[k] = e
			}
		}
		set(item, pos)
	}
	pool := ids(t.Malangdo.Pool)
	for _, w := range t.Malangdo.Weapons {
		if it := c.Items[w]; it != nil && len(pool) > 0 {
			if it.Slots >= 3 {
				set(w, map[int][]int{3: pool})
			} else {
				set(w, map[int][]int{2: pool, 3: pool})
			}
		}
	}
	return n
}
