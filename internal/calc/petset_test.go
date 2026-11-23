package calc

import (
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

// A LATAM set line whose partner is a pet egg (Cesta de Mascotes) counts
// with that pet out, and only with it.
func TestPetEggSetLine(t *testing.T) {
	c := novice()
	c.Items[9121] = &gamedata.Item{ID: 9121, Type: "PetEgg"}
	c.Pets = map[int]*gamedata.Pet{9121: {Mob: 20571, Egg: 9121, Script: "bonus bLuk,0;"}}
	c.Items[410599] = &gamedata.Item{ID: 410599, Type: "Armor", Locations: []string{HeadMid},
		Latam: map[string][]string{"range": {"EQUIP_ID[9121||9137]10"}}, LatamKeys: []string{"bLongAtkRate"}}
	long := func(pet int) int {
		b := Build{Job: 0, BaseLv: 1, JobLv: 1, Stats: [6]int{1, 1, 1, 1, 1, 1}, Pet: pet, PetIntimacy: 1000,
			Equip: map[string]Equip{HeadMid: {Item: 410599}}}
		Normalize(&b, c)
		return Compute(b, c).Bonuses["bLongAtkRate"]
	}
	if got := long(9121); got != 10 {
		t.Errorf("with the pet: ranged +%d, want +10", got)
	}
	if got := long(0); got != 0 {
		t.Errorf("without a pet: ranged +%d, want 0", got)
	}
}
