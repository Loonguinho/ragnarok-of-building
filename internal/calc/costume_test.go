package calc

import (
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

// TestCostumeStonePositions: a costume keeps its slot's stone in any position (a
// lower costume's sits in the 3rd) and the older stones that name no slot;
// another slot's stone goes.
func TestCostumeStonePositions(t *testing.T) {
	c := novice()
	c.Items[31844] = &gamedata.Item{ID: 31844, Type: "Armor", Locations: []string{CosLow}}
	c.Items[29434] = &gamedata.Item{ID: 29434, Type: "Card", SubType: "Enchant", Locations: []string{CosLow}}
	c.Items[29432] = &gamedata.Item{ID: 29432, Type: "Card", SubType: "Enchant", Locations: []string{CosTop}}
	c.Items[29046] = &gamedata.Item{ID: 29046, Type: "Card", SubType: "Enchant"}
	for _, tc := range []struct{ in, want [4]int }{
		{[4]int{0, 0, 29434, 0}, [4]int{0, 0, 29434, 0}},
		{[4]int{0, 0, 29046, 0}, [4]int{0, 0, 29046, 0}},
		{[4]int{29432, 0, 0, 0}, [4]int{}},
	} {
		b := Build{Job: 0, BaseLv: 1, JobLv: 1, Equip: map[string]Equip{CosLow: {Item: 31844, Cards: tc.in}}}
		Normalize(&b, c)
		if got := b.Equip[CosLow].Cards; got != tc.want {
			t.Errorf("stones %v kept as %v, want %v", tc.in, got, tc.want)
		}
	}
}
