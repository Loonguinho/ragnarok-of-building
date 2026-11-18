package calc

import (
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

func TestItemCombos(t *testing.T) {
	c := &gamedata.Catalog{
		Items: map[int]*gamedata.Item{
			2601: {ID: 2601, Type: "Armor", Locations: []string{"Left_Accessory", "Right_Accessory"}},
			2101: {ID: 2101, Type: "Armor", Locations: []string{"Left_Hand"}},
			4001: {ID: 4001, Type: "Card", Locations: []string{"Left_Hand"}},
		},
		Combos: []gamedata.Combo{
			{Items: []int{2601, 2601}, Script: "bonus bStr,5;"},     // a pair of rings
			{Items: []int{2101, 4001}, Script: "bonus bMaxHP,100;"}, // shield + its card
			{Items: []int{2101, 9999}, Script: "bonus bLuk,50;"},    // missing item
		},
		Jobs: map[int]*gamedata.Job{0: {ID: 0, MaxBaseLv: 99, MaxJobLv: 10, Parent: -1}},
	}
	b := Build{Job: 0, BaseLv: 1, JobLv: 1, Stats: [6]int{1, 1, 1, 1, 1, 1}, Equip: map[string]Equip{
		AccRight: {Item: 2601}, Shield: {Item: 2101, Cards: [4]int{4001}},
	}}
	bonus, extra := map[string]int{}, map[string]int{}
	skipped := 0
	got, _ := applyCombos(c, &b, nil, map[int]bool{}, bonus, extra, &skipped)
	if len(got) != 1 || bonus["bMaxHP"] != 100 || bonus["bStr"] != 0 {
		t.Fatalf("one ring: combos %v, bonus %v", got, bonus)
	}
	b.Equip[AccLeft] = Equip{Item: 2601}
	bonus = map[string]int{}
	got, eff := applyCombos(c, &b, nil, map[int]bool{}, bonus, extra, &skipped)
	if len(got) != 2 || bonus["bStr"] != 5 {
		t.Fatalf("two rings: combos %v, bonus %v", got, bonus)
	}
	// What each combo adds, on its own.
	if len(eff) != 2 {
		t.Fatalf("effects: %+v", eff)
	}
	sum := map[string]int{}
	for _, e := range eff {
		addAll(sum, e.Bonus)
	}
	if sum["bStr"] != 5 || sum["bMaxHP"] != 100 {
		t.Errorf("effects add up to %v", sum)
	}
}
