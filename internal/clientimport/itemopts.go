package clientimport

import (
	"encoding/json"
	"os"
	"regexp"
	"slices"
	"strconv"

	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/rathena"
)

// ItemOptsFromRathena fills c.ItemOpts from rAthena's option groups: an item
// with several groups gets, per position, every option any of them rolls
// (with the widest value range), guaranteed positions first, then the
// MaxRandom extra ones.
func ItemOptsFromRathena(c *gamedata.Catalog, db *rathena.DB) int {
	if c.ItemOpts == nil {
		c.ItemOpts = map[int]*gamedata.ItemOpts{}
	}
	for id, groups := range db.ItemOptGroups {
		if c.Items[id] == nil {
			continue
		}
		var pos [][]gamedata.OptRange
		merge := func(i int, rs []rathena.OptRange) {
			for len(pos) <= i {
				pos = append(pos, nil)
			}
			for _, r := range rs {
				k := slices.IndexFunc(pos[i], func(o gamedata.OptRange) bool { return o.ID == r.ID })
				if k < 0 {
					pos[i] = append(pos[i], gamedata.OptRange{ID: r.ID, Min: r.Min, Max: r.Max})
					continue
				}
				pos[i][k].Min = min(pos[i][k].Min, r.Min)
				pos[i][k].Max = max(pos[i][k].Max, r.Max)
			}
		}
		for _, name := range groups {
			g := db.OptGroups[name]
			for i, s := range g.Slots {
				merge(i, s)
			}
			for k := 0; k < g.MaxRandom && len(g.Slots)+k < 5; k++ {
				merge(len(g.Slots)+k, g.Random)
			}
		}
		if len(pos) > 0 {
			for i := range pos {
				slices.SortFunc(pos[i], func(a, b gamedata.OptRange) int { return a.ID - b.ID })
			}
			c.ItemOpts[id] = &gamedata.ItemOpts{Positions: pos}
		}
	}
	return len(c.ItemOpts)
}

// latam-ro-calc's ExtraOptionTable: `  AegisName: 2,`.
var reOptCount = regexp.MustCompile(`(?m)^\s*([A-Za-z0-9_]+):\s*(\d+),`)

// ItemOptCounts reads latam-ro-calc's extra-option-table.ts (MIT): how many
// random options each LATAM item takes, checked against bROWiki. Items the
// rAthena groups already cover keep their per-position lists.
func ItemOptCounts(c *gamedata.Catalog, path string, aegis map[string]int) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if c.ItemOpts == nil {
		c.ItemOpts = map[int]*gamedata.ItemOpts{}
	}
	n := 0
	for _, m := range reOptCount.FindAllStringSubmatch(string(b), -1) {
		id, ok := aegis[m[1]]
		cnt, _ := strconv.Atoi(m[2])
		if !ok || c.Items[id] == nil || cnt <= 0 || c.ItemOpts[id] != nil {
			continue
		}
		c.ItemOpts[id] = &gamedata.ItemOpts{Any: min(cnt, 5)}
		n++
	}
	return n, nil
}

// LatamAegis adds to rAthena's AegisName -> ID map the LATAM-only items it
// lacks, from latam-ro-calc's item.json (MIT). A missing file leaves the
// rAthena map as it is.
func LatamAegis(path string, rathena map[string]int) map[string]int {
	out := make(map[string]int, len(rathena))
	for k, v := range rathena {
		out[k] = v
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	var items map[string]struct {
		ID    int    `json:"id"`
		Aegis string `json:"aegisName"`
	}
	if json.Unmarshal(b, &items) != nil {
		return out
	}
	for _, it := range items {
		if _, ok := out[it.Aegis]; !ok && it.Aegis != "" && it.ID > 0 {
			out[it.Aegis] = it.ID
		}
	}
	return out
}
