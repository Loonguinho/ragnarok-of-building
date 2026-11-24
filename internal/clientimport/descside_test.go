package clientimport

import (
	"slices"
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

func TestDescSides(t *testing.T) {
	item := func(typ, sub, line string, locs ...string) *gamedata.Item {
		return &gamedata.Item{Type: typ, SubType: sub, Locations: locs, Desc: map[string][]string{"pt": {line}}}
	}
	c := &gamedata.Catalog{Items: map[int]*gamedata.Item{
		1: item("Card", "", "Equipa em: ^777777Aces. Esquerdo^000000", "Both_Accessory"),
		2: item("Armor", "", "Tipo: ^777777Aces. direito^000000", "Right_Accessory"),
		3: item("Armor", "", "Tipo: Acessório", "Left_Accessory"),
		4: item("ShadowGear", "", "Posição: Aces. Direito", "Shadow_Left_Accessory"),
		5: item("Card", "Enchant", "Equipa em: Acessório"),
	}}
	if n := DescSides(c); n != 2 {
		t.Errorf("changed %d, want 2", n)
	}
	for id, want := range map[int][]string{1: {"Left_Accessory"}, 2: {"Right_Accessory"}, 3: {"Both_Accessory"}, 4: {"Shadow_Left_Accessory"}, 5: nil} {
		if got := c.Items[id].Locations; !slices.Equal(got, want) {
			t.Errorf("%d: %v, want %v", id, got, want)
		}
	}
}
