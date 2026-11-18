package web

import (
	"strconv"

	"ragnarok-of-building/internal/calc"
)

type buffGroup struct {
	Name  string
	Items []buffItem
}

type buffItem struct {
	Key, Label string
	Choices    [][2]string // selector; nil = checkbox or number
	Max        int         // > 0: number input
	Value      int
	ValueStr   string
}

// buffGroups lists the buffs window rows with the build's values.
func buffGroups(b calc.Build) []buffGroup {
	var out []buffGroup
	for _, d := range calc.Buffs {
		if len(out) == 0 || out[len(out)-1].Name != d.Group {
			out = append(out, buffGroup{Name: d.Group})
		}
		v := b.Buffs[d.Key]
		g := &out[len(out)-1]
		g.Items = append(g.Items, buffItem{Key: d.Key, Label: d.Label, Choices: d.Choices, Max: d.Max, Value: v, ValueStr: strconv.Itoa(v)})
	}
	return out
}
