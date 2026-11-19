package web

import (
	"strconv"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

type buffGroup struct {
	Name   string
	Items  []buffItem
	Active int  // buffs on in this group (shown on the closed group)
	Open   bool // the group starts open: support buffs, or any buff on
}

type buffItem struct {
	Key, Label string
	Excl       string      // only one of the same Excl group at a time
	State      bool        // a class buff: kept in the attack states (data-atk-state)
	Choices    [][2]string // selector; nil = checkbox or number
	Max        int         // > 0: number input
	Value      int
	ValueStr   string
}

// buffGroups lists the buffs window rows with the build's values. Buffs
// tied to a skill take its name from the client, in the page's language.
func buffGroups(b calc.Build, c *gamedata.Catalog, l string) []buffGroup {
	skills := map[string]*gamedata.Skill{}
	if c != nil {
		for _, s := range c.Skills {
			skills[s.Code] = s
		}
	}
	var out []buffGroup
	for _, d := range calc.Buffs {
		if len(out) == 0 || out[len(out)-1].Name != d.Group {
			out = append(out, buffGroup{Name: d.Group, Open: d.Group == "Suporte"})
		}
		label := tr(l, d.Label)
		if s := skills[d.Skill]; s != nil && s.Name.Get(l) != "" {
			label = s.Name.Get(l)
		}
		v := b.Buffs[d.Key]
		g := &out[len(out)-1]
		g.Items = append(g.Items, buffItem{Key: d.Key, Label: label, Excl: d.Excl, Choices: d.Choices, Max: d.Max, Value: v, ValueStr: strconv.Itoa(v)})
		if v > 0 {
			g.Active++
			g.Open = true
		}
		if d.Group == "Próprios" && d.Key == "concentration" && c != nil {
			// The class's own buffs (generated from latam-ro-calc) join this group.
			for _, cb := range calc.ClassBuffsFor(c, b.Job) {
				label := cb.Label
				if s := skills[cb.Skill]; s != nil && s.Name.Get(l) != "" {
					label = s.Name.Get(l)
				}
				v := b.Atk.States[cb.Key]
				it := buffItem{Key: cb.Key, Label: label, State: true, Value: v, ValueStr: strconv.Itoa(v)}
				if !cb.Toggle {
					it.Choices = cb.Choices
				}
				g.Items = append(g.Items, it)
				if v > 0 {
					g.Active++
					g.Open = true
				}
			}
		}
	}
	return out
}
