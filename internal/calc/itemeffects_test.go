package calc

import (
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

func effectCatalog() *gamedata.Catalog {
	return &gamedata.Catalog{
		Jobs: map[int]*gamedata.Job{1: {ID: 1, Code: "JT_SWORDMAN", Parent: -1}},
		Items: map[int]*gamedata.Item{
			// rAthena autobonus from +11, more with an enchant grade.
			470071: {ID: 470071, Type: "Armor", Locations: []string{Shoes}, Refineable: true,
				Script: `if (getrefine()>=11) { autobonus "{ .@g = getenchantgrade(); bonus bStr,175+(.@g>=1?25:0); }",1,10000,BF_WEAPON; }`},
			// only a latam-ro-calc chance line.
			4581: {ID: 4581, Type: "Card", LatamProcs: map[string][]string{"agi": {"44"}}},
			1201: {ID: 1201, Type: "Weapon", SubType: "Dagger", Locations: []string{Weapon}},
		},
	}
}

func TestItemEffects(t *testing.T) {
	c := effectCatalog()
	b := Build{Job: 1, BaseLv: 99, JobLv: 50, Stats: [6]int{1, 1, 1, 1, 1, 1},
		Equip: map[string]Equip{Shoes: {Item: 470071, Refine: 11, Grade: 1}, Weapon: {Item: 1201, Cards: [4]int{4581}}}}
	r := Compute(b, c)
	if len(r.ItemEffects) != 2 {
		t.Fatalf("effects = %+v", r.ItemEffects)
	}
	got := map[int]map[string]int{}
	for _, ef := range r.ItemEffects {
		if ef.On {
			t.Errorf("%d on without being asked", ef.Item)
		}
		got[ef.Item] = ef.Bonus
	}
	if got[470071]["bStr"] != 200 || got[4581]["bAgi"] != 44 {
		t.Errorf("effects: %v", got)
	}
	str := r.Stats[STR].Bonus
	b.Buffs = map[string]int{ItemEffectKey(470071): 1}
	if r2 := Compute(b, c); r2.Stats[STR].Bonus != str+200 {
		t.Errorf("boots on: STR bonus %d, want %d", r2.Stats[STR].Bonus, str+200)
	}
	// Below +11 the boots have nothing to offer.
	b.Equip[Shoes] = Equip{Item: 470071, Refine: 9}
	for _, ef := range Compute(b, c).ItemEffects {
		if ef.Item == 470071 {
			t.Errorf("boots +9 offer %v", ef.Bonus)
		}
	}
}
