package view

import (
	"slices"
	"strings"

	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/script"
)

// The sets of the items the LATAM client does not have yet, in the
// descriptions of the enchants they are made with: the client's text (Dom
// Celestial, Dom Infernal) lists only the sets whose pieces it has, and the
// preview enchants (Essência...) have English texts.

var langs = []string{"pt", "es", "en"}

const (
	setSep    = "--------------------------"
	setHeader = "Conjuntos que ainda não chegaram ao LATAM"
)

// completeSetTexts appends the sets that are missing to the enchants'
// descriptions. Doing it again changes nothing.
func completeSetTexts(c *gamedata.Catalog) {
	if c == nil || len(c.Combos) == 0 {
		return
	}
	done := map[int]bool{}
	for _, cb := range c.Combos {
		if !slices.ContainsFunc(cb.Items, func(id int) bool { return c.Items[id] != nil && c.Items[id].Preview }) {
			continue
		}
		for _, id := range cb.Items {
			it := c.Items[id]
			if it == nil || done[id] || it.SubType != "Enchant" {
				continue
			}
			done[id] = true
			if it.Desc == nil {
				it.Desc = gamedata.Lines{}
			}
			for _, l := range langs {
				if sets := setTexts(c, id, l); len(sets) > 0 && !slices.Contains(it.Desc[l], setTitle(l)) {
					// A new slice: the catalog's may be shared.
					var text []string
					if it.Preview {
						text = []string{setTitle(l)} // instead of the English text
					} else {
						text = append(slices.Clone(it.Desc[l]), setSep, setTitle(l))
					}
					it.Desc[l] = append(text, sets...)
				}
			}
		}
	}
}

func setTitle(l string) string { return "^CC3D3D" + tr(l, setHeader) + "^000000" }

// setTexts are the lines of the sets with preview pieces that id is part
// of. The sets of a worn enchant with another one are told by the other
// (Essência), as in the client.
func setTexts(c *gamedata.Catalog, id int, l string) []string {
	var out []string
	for _, ci := range c.CombosWith(id) {
		cb := c.Combos[ci]
		preview := false
		var names []string
		other := false
		for _, p := range cb.Items {
			it := c.Items[p]
			if it == nil {
				preview = false
				names = nil
				break
			}
			preview = preview || it.Preview
			if p == id {
				continue
			}
			other = other || it.SubType == "Enchant"
			names = append(names, it.Name.Get(l))
		}
		if !preview || len(names) == 0 || other && !c.Items[id].Preview {
			continue
		}
		res := script.Eval(cb.Script, script.Context{BaseLv: 250, JobLv: 50, Param: map[string]int{}})
		var bonus []string
		add := func(k string, v int) {
			lb, val := bonusText(k, v, c, l)
			if s := "^0000FF" + strings.TrimSpace(lb+" "+val) + ".^000000"; !slices.Contains(bonus, s) {
				bonus = append(bonus, s) // two codes of one skill read the same
			}
		}
		for _, k := range sortedKeys(res.Extra) {
			add(k, res.Extra[k])
		}
		for _, k := range sortedKeys(res.Bonus) {
			add(k, res.Bonus[k])
		}
		if len(bonus) == 0 {
			continue
		}
		out = append(out, setSep, "^FA4E09"+tr(l, "Conjunto")+"^000000")
		for _, n := range names {
			out = append(out, "^FA4E09"+n+"^000000")
		}
		out = append(out, bonus...)
	}
	return out
}
