package view

import (
	"testing"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

// An item with a proc compared with its effect off and on.
func TestCompare(t *testing.T) {
	c := testCatalog()
	c.Items[1290] = &gamedata.Item{ID: 1290, Name: gamedata.Text{"pt": "Faca do Proc"}, Type: "Weapon", SubType: "Dagger",
		Locations: []string{"Right_Hand"}, Attack: 30, WeaponLv: 1, Refineable: true,
		Script: `autobonus "{ bonus bStr,100; }",100,5000,BF_WEAPON;`}
	c.Monsters = map[int]*gamedata.Monster{1002: {ID: 1002, Name: "Poring", Level: 1, HP: 5000, Size: "Medium",
		Race: "Plant", Element: "Water", ElementLv: 1, Class: "Normal", DamageTaken: 100}}
	s, err := newTestServer(Options{Catalog: c})
	if err != nil {
		t.Fatal(err)
	}
	v := &Views{s: s}
	b := calc.Build{Job: 0, BaseLv: 3, JobLv: 1, Stats: [6]int{1, 1, 1, 1, 1, 1}, Target: 1002,
		Equip: map[string]calc.Equip{calc.Weapon: {Item: 1201, Refine: 4}}}
	key := calc.ItemEffectKey(1290)
	cv := v.Compare(b, calc.Weapon, []CompareCand{
		{},
		{Item: 1290, Refine: 7},
		{Item: 1290, Refine: 7, Fx: map[string]bool{key: true}},
	}, "pt")
	if len(cv.Cols) != 3 || cv.Cols[1].Item == nil || cv.Cols[1].Item.ID != 1290 || cv.Cols[1].Refine != 7 {
		t.Fatalf("columns: %+v", cv.Cols)
	}
	if len(cv.Cols[0].Effects) != 0 || len(cv.Cols[1].Effects) != 1 || cv.Cols[1].Effects[0].On || !cv.Cols[2].Effects[0].On {
		t.Errorf("effects: %+v / %+v / %+v", cv.Cols[0].Effects, cv.Cols[1].Effects, cv.Cols[2].Effects)
	}
	rows := map[string]CompareRow{}
	for _, r := range cv.Rows {
		rows[r.Label] = r
	}
	str, ok := rows["FOR"]
	if !ok || str.Values[1] == str.Values[2] || str.Better[2] != 1 {
		t.Errorf("FOR row: %+v", str)
	}
	exp := cv.Rows[0]
	if !exp.Main || exp.Better[2] != 1 {
		t.Errorf("expected damage row: %+v", exp)
	}
	if len(b.Buffs) != 0 || b.Equip[calc.Weapon].Item != 1201 {
		t.Errorf("the caller's build changed: %+v", b)
	}
	// An enchant in the column's fourth position.
	c.Items[4700] = &gamedata.Item{ID: 4700, Name: gamedata.Text{"pt": "Força 1"}, Type: "Card", SubType: "Enchant", Script: `bonus bStr,10;`}
	c.Enchants = map[int]map[int][]int{1290: {3: {4700}}}
	cv = v.Compare(b, calc.Weapon, []CompareCand{
		{Item: 1290, Refine: 7},
		{Item: 1290, Refine: 7, Cards: &[4]int{0, 0, 0, 4700}},
	}, "pt")
	pos := cv.Cols[0].Positions
	if len(pos) != 4 || !pos[3].Enchant || len(pos[3].Options) != 1 || pos[3].Options[0].ID != 4700 || cv.Cols[1].Positions[3].Current != 4700 {
		t.Fatalf("positions: %+v", pos)
	}
	found := false
	for _, r := range cv.Rows {
		if r.Label == "FOR" {
			found = r.Diffs[1] == "+10"
		}
	}
	if !found {
		t.Errorf("the enchant's FOR +10 is missing: %+v", cv.Rows)
	}

	// Equipar takes the column as it is, effect on.
	nb := v.CompareBuild(b, calc.Weapon, CompareCand{Item: 1290, Refine: 7, Fx: map[string]bool{key: true}})
	if nb.Equip[calc.Weapon].Item != 1290 || nb.Buffs[key] != 1 {
		t.Errorf("column build: %+v", nb)
	}
}
