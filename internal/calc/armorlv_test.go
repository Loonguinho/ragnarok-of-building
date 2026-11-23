package calc

import (
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

// A card reads the level of the armor it is in: Carta Dollocaris Binocular
// gives TEN +50 more in a level 2 armor.
func TestCardArmorLevel(t *testing.T) {
	c := novice()
	c.Items[300376] = &gamedata.Item{ID: 300376, Type: "Card", Locations: []string{"Armor"},
		Script: "bonus bRes,50; if (getequiparmorlv() == 2) { bonus bRes,50; }"}
	res := func(armorLv int) int {
		c.Items[2301].ArmorLv = armorLv
		b := Build{Job: 0, BaseLv: 1, JobLv: 1, Stats: [6]int{1, 1, 1, 1, 1, 1},
			Equip: map[string]Equip{Armor: {Item: 2301, Refine: 12, Cards: [4]int{300376}}}}
		Normalize(&b, c)
		return Compute(b, c).Bonuses["bRes"]
	}
	if got := res(2); got != 100 {
		t.Errorf("level 2 armor: TEN +%d, want +100", got)
	}
	if got := res(1); got != 50 {
		t.Errorf("level 1 armor: TEN +%d, want +50", got)
	}
}
