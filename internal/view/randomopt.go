package view

import (
	"sort"

	"ragnarok-of-building/internal/gamedata"
)

// randomOptList is the random options an item position can roll, sorted by
// name, with their value range when the item's options are known; every
// option otherwise.
func randomOptList(c *gamedata.Catalog, item, pos int, l string) []optLine {
	var list []optLine
	if allowed := c.ItemOpts[item].At(pos); allowed != nil {
		for _, a := range allowed {
			if o := c.RandomOpts[a.ID]; o != nil {
				list = append(list, optLine{ID: a.ID, Min: a.Min, Max: a.Max, Label: o.Label(l, 0)})
			}
		}
	} else {
		for id, o := range c.RandomOpts {
			list = append(list, optLine{ID: id, Label: o.Label(l, 0)})
		}
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Label != list[j].Label {
			return list[i].Label < list[j].Label
		}
		return list[i].ID < list[j].ID
	})
	return list
}
