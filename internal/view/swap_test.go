package view

import (
	"testing"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

func TestSwapItem(t *testing.T) {
	c := testCatalog()
	c.Items[1202] = &gamedata.Item{ID: 1202, Name: gamedata.Text{"pt": "Faca Boa"}, Slots: 1, Type: "Weapon", SubType: "Dagger",
		Locations: []string{"Right_Hand"}, Attack: 60, WeaponLv: 1, Refineable: true}
	c.Items[1160] = &gamedata.Item{ID: 1160, Name: gamedata.Text{"pt": "Espadão"}, Type: "Weapon", SubType: "2hSword",
		Locations: []string{"Both_Hand"}, Attack: 100, WeaponLv: 1}
	c.Items[2101] = &gamedata.Item{ID: 2101, Name: gamedata.Text{"pt": "Escudo"}, Type: "Armor", Locations: []string{"Left_Hand"}, Defense: 5}
	c.Items[5001] = &gamedata.Item{ID: 5001, Name: gamedata.Text{"pt": "Elmo"}, Type: "Armor", Locations: []string{"Head_Top", "Head_Mid"}, Defense: 3}
	c.Items[5002] = &gamedata.Item{ID: 5002, Name: gamedata.Text{"pt": "Óculos"}, Type: "Armor", Locations: []string{"Head_Mid"}, Defense: 1}
	c.Monsters = map[int]*gamedata.Monster{1002: {ID: 1002, Name: "Poring", Level: 1, HP: 5000, Size: "Medium",
		Race: "Plant", Element: "Water", ElementLv: 1, Class: "Normal", DamageTaken: 100}}
	s, err := newTestServer(Options{Catalog: c})
	if err != nil {
		t.Fatal(err)
	}
	b := calc.Build{Job: 0, BaseLv: 3, JobLv: 1, Stats: [6]int{1, 1, 1, 1, 1, 1}, Target: 1002, Skills: map[int]int{1: 9},
		Equip: map[string]calc.Equip{calc.Weapon: {Item: 1201, Refine: 7, Cards: [4]int{4002, 4002, 0, 0}}, calc.Shield: {Item: 2101},
			calc.HeadMid: {Item: 5002}}}
	calc.Normalize(&b, c)

	nb := s.swapItem(b, calc.Weapon, 1202, -1)
	if w := nb.Equip[calc.Weapon]; w.Item != 1202 || w.Refine != 7 || w.Cards != [4]int{4002, 0, 0, 0} {
		t.Errorf("weapon swap keeps refine and the cards that fit: %+v", w)
	}
	if nb := s.swapItem(b, calc.Weapon, 1160, -1); nb.Equip[calc.Shield].Item != 0 {
		t.Error("a two-handed weapon drops the shield")
	}
	if nb := s.swapItem(b, calc.HeadTop, 5001, -1); nb.Equip[calc.HeadTop].Item != 5001 || nb.Equip[calc.HeadMid].Item != 5001 {
		t.Errorf("a helm covering top+mid replaces the glasses: %+v", nb.Equip)
	}
	if nb := s.swapItem(b, calc.Weapon, 4002, 2); nb.Equip[calc.Weapon].Cards[2] != 4002 {
		t.Errorf("card swap: %+v", nb.Equip[calc.Weapon])
	}

	// The picker shows each result's damage change; the tooltip, the details.
	gains := s.pickGains(b, calc.Weapon, s.pickItems(calc.Weapon, false, -1, 0, 0, "faca", "pt"), -1, "pt")
	if !gains[1202].Better {
		t.Errorf("picker gains: %+v", gains)
	}
	sv := (&Views{s: s}).SwapCompare(b, calc.Weapon, 1202, -1, "pt")
	if sv.Expected == nil || len(sv.Lines) == 0 || sv.Current != "Faca" {
		t.Errorf("swap compare: %+v", sv)
	}
}

// Of two classes with the same name only the transcendent one is listed,
// unless the build already uses the other.
func TestHiddenJobs(t *testing.T) {
	c := testCatalog()
	tree := map[int]int{0: 1}
	stats := []int{1}
	c.Jobs[4054] = &gamedata.Job{ID: 4054, Name: gamedata.Text{"pt": "Cavaleiro Rúnico"}, Tree: tree, Parent: 7}
	c.Jobs[4060] = &gamedata.Job{ID: 4060, Name: gamedata.Text{"pt": "Cavaleiro Rúnico"}, Tree: tree, Parent: 7, Upper: true}
	for _, id := range []int{4054, 4060} {
		c.Jobs[id].BaseHP, c.Jobs[id].MaxBaseLv, c.Jobs[id].MaxJobLv = stats, 1, 1
	}
	s, err := newTestServer(Options{Catalog: c})
	if err != nil {
		t.Fatal(err)
	}
	if h := s.hiddenJobs("pt", 0); !h[4054] || h[4060] {
		t.Errorf("hidden %v, want only the non-transcendent 4054", h)
	}
	if h := s.hiddenJobs("pt", 4054); h[4054] {
		t.Error("the build's own class must stay listed")
	}
	// Only 3rd and 4th classes are listed (plus the build's own).
	c.Jobs[7].EquipTier, c.Jobs[4060].EquipTier, c.Jobs[4054].EquipTier = "Normal", "Third_Upper", "Third"
	if h := s.hiddenJobs("pt", 0); !h[7] || h[4060] {
		t.Errorf("hidden %v: want the 2nd class 7 hidden, 4060 listed", h)
	}
	if h := s.hiddenJobs("pt", 7); h[7] {
		t.Error("a 2nd class build keeps its class listed")
	}
}

func TestSwapIntoEmptyBuild(t *testing.T) {
	s, _ := newTestServer(Options{Catalog: testCatalog()})
	nb := s.swapItem(calc.Build{Job: 0, BaseLv: 3, JobLv: 1, Stats: [6]int{1, 1, 1, 1, 1, 1}}, calc.Weapon, 1201, -1)
	if nb.Equip[calc.Weapon].Item != 1201 {
		t.Errorf("equip into an empty build: %+v", nb.Equip)
	}
}
