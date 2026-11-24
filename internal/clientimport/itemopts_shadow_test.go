package clientimport

import (
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

func TestFixShadowOpts(t *testing.T) {
	spec := func() *gamedata.ItemOpts {
		return &gamedata.ItemOpts{Positions: [][]gamedata.OptRange{{{ID: 17, Min: 1, Max: 15}, {ID: optCrit, Min: 1, Max: 15}}}}
	}
	c := &gamedata.Catalog{
		Items:    map[int]*gamedata.Item{1: {Type: "ShadowGear"}, 2: {Type: "Weapon"}},
		ItemOpts: map[int]*gamedata.ItemOpts{1: spec(), 2: spec()},
	}
	if n := FixShadowOpts(c); n != 1 {
		t.Errorf("fixed %d, want 1", n)
	}
	if s, w := c.ItemOpts[1].Positions[0], c.ItemOpts[2].Positions[0]; s[0].Max != 15 || s[1].Max != 5 || w[1].Max != 15 {
		t.Errorf("shadow %v weapon %v", s, w)
	}
}

func TestShadowBooks(t *testing.T) {
	shadow := func(n string) *gamedata.Item { return &gamedata.Item{Type: "ShadowGear", Name: gamedata.Text{"pt": n}} }
	c := &gamedata.Catalog{
		Items: map[int]*gamedata.Item{
			1: shadow("Escudo Sombrio Mega Repente"), 2: shadow("Manopla Sombria de EXP"), 3: shadow("Malha Sombria"),
			9: {Name: gamedata.Text{"pt": "Caderno Mega Repente"}, Desc: gamedata.Lines{"pt": {"^0000ffAdiciona até 2 bônus aleatórios.^000000", "---",
				"[^CD3278Itens afetados^000000]", "Escudo Sombrio Mega Repente", "Manopla Sombria de EXP", "---", "Malha Sombria"}}},
		},
		ItemOpts: map[int]*gamedata.ItemOpts{
			1: {Positions: [][]gamedata.OptRange{{{ID: 17, Min: 1, Max: 15}}}}, 2: {Any: 1},
			3: {Positions: [][]gamedata.OptRange{{{ID: 17, Min: 1, Max: 15}}}},
		},
	}
	if n := ShadowBooks(c); n != 2 {
		t.Errorf("covered %d, want 2", n)
	}
	for id, want := range map[int]int{1: 2, 2: 2, 3: 1} {
		if got := c.ItemOpts[id].Slots(); got != want {
			t.Errorf("item %d takes %d options, want %d", id, got, want)
		}
	}
	if p := c.ItemOpts[2].Positions; len(p[0]) != 3 || len(p[1]) != len(shadowBookSecond) {
		t.Errorf("rolls: %v", p)
	}
}
