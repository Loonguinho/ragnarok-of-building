package clientimport

import (
	"encoding/json"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

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

// optCrit is the CRIT random option (VAR_CRITICALSUCCESSVALUE).
const optCrit = 24

// FixShadowOpts caps CRIT on shadow gear at 5: rAthena rolls 1-15, LATAM 1-5
// (bROWiki, and every roll in the recordings).
func FixShadowOpts(c *gamedata.Catalog) int {
	n := 0
	for id, spec := range c.ItemOpts {
		if it := c.Items[id]; it == nil || it.Type != "ShadowGear" {
			continue
		}
		for _, rolls := range spec.Positions {
			for i := range rolls {
				if rolls[i].ID == optCrit && rolls[i].Max > 5 {
					rolls[i].Max = 5
					n++
				}
			}
		}
	}
	return n
}

// shadowBookExtra are the rolls the shadow books (Cadernos Sombrios) add to
// the ones every shadow piece has, and shadowBookSecond the second option
// they may give (bROWiki; Dano preciso has no option in the client list).
var (
	shadowBookExtra  = []gamedata.OptRange{{ID: 219, Min: 1, Max: 5}, {ID: 231, Min: 1, Max: 5}}
	shadowBookSecond = []gamedata.OptRange{
		{ID: 243, Min: 1, Max: 5}, {ID: 244, Min: 1, Max: 5}, {ID: 245, Min: 1, Max: 5}, // POD, FEI, STA
		{ID: 246, Min: 1, Max: 5}, {ID: 247, Min: 1, Max: 5}, {ID: 248, Min: 1, Max: 5}, // SAB, CON, CRV
		{ID: 249, Min: 1, Max: 2}, {ID: 250, Min: 1, Max: 2}, // P.ATQ, S.ATQM
		{ID: 251, Min: 5, Max: 15}, {ID: 252, Min: 5, Max: 15}, // TEN, TENM
		{ID: 253, Min: 3, Max: 10}, {ID: 254, Min: 1, Max: 5}, // C.Mais, T.CRÍT
	}
)

// ShadowBooks gives the shadow pieces the books name ("Itens afetados") their
// two random options. Returns how many pieces it covers.
func ShadowBooks(c *gamedata.Catalog) int {
	byName := map[string][]int{}
	var base []gamedata.OptRange // what every shadow piece rolls
	for id, it := range c.Items {
		if it.Type != "ShadowGear" {
			continue
		}
		n := strings.TrimSpace(it.Name.Get("pt"))
		byName[n] = append(byName[n], id)
		if s := c.ItemOpts[id]; s != nil && len(s.Positions) == 1 && len(s.Positions[0]) > len(base) {
			base = s.Positions[0]
		}
	}
	n := 0
	for _, it := range c.Items {
		if !strings.HasPrefix(it.Name.Get("pt"), "Caderno") {
			continue
		}
		two, list := false, false
		for _, l := range it.Desc["pt"] {
			l = strings.TrimSpace(descColor.ReplaceAllString(l, ""))
			switch {
			case strings.Contains(l, "até 2 bônus aleatórios"):
				two = true
			case strings.Contains(l, "Itens afetados"):
				list = true
			case strings.HasPrefix(l, "---"):
				list = false
			case two && list:
				for _, id := range byName[l] {
					first := slices.Clone(base)
					if s := c.ItemOpts[id]; s != nil && len(s.Positions) > 0 {
						if len(s.Positions) > 1 {
							continue // another book's
						}
						first = slices.Clone(s.Positions[0])
					}
					first = append(first, shadowBookExtra...)
					slices.SortFunc(first, func(a, b gamedata.OptRange) int { return a.ID - b.ID })
					c.ItemOpts[id] = &gamedata.ItemOpts{Positions: [][]gamedata.OptRange{first, shadowBookSecond}}
					n++
				}
			}
		}
	}
	return n
}
