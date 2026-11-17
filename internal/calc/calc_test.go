package calc

import (
	"path/filepath"
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

func TestStatCost(t *testing.T) {
	cases := map[int]int{1: 2, 10: 2, 11: 3, 99: 11, 100: 16, 104: 16, 105: 20, 120: 32, 129: 36}
	for x, want := range cases {
		if got := StatCost(x); got != want {
			t.Errorf("StatCost(%d) = %d, want %d", x, got, want)
		}
	}
	if StatSpent(1) != 0 || StatSpent(2) != 2 || StatSpent(12) != 2*10+3 {
		t.Errorf("StatSpent wrong: %d %d %d", StatSpent(1), StatSpent(2), StatSpent(12))
	}
}

// novice is a minimal catalog matching rAthena's Novice tables at level 1.
func novice() *gamedata.Catalog {
	return &gamedata.Catalog{
		StatPoints: []int{48, 51, 54},
		Jobs: map[int]*gamedata.Job{0: {
			ID: 0, MaxBaseLv: 3, MaxJobLv: 10,
			BaseHP: []int{40, 46, 53}, BaseSP: []int{11, 12, 13},
			JobBonus: make([][12]int, 11),
			BaseASPD: map[string]int{"Fist": 40, "Dagger": 55, "Shield": 10},
		}},
		Items: map[int]*gamedata.Item{
			1201: {ID: 1201, Type: "Weapon", SubType: "Dagger", Locations: []string{"Right_Hand"}, Attack: 17, WeaponLv: 1, Refineable: true},
			2301: {ID: 2301, Type: "Armor", Locations: []string{"Armor"}, Defense: 10, ArmorLv: 1, Refineable: true},
			4001: {ID: 4001, Type: "Card", Locations: []string{"Armor"}, Script: "bonus bLuk,2;\nbonus bFlee2,1;"},
			5000: {ID: 5000, Type: "Armor", Locations: []string{"Head_Top", "Head_Mid"}, Defense: 3,
				Script: "bonus bMaxHPrate,10;\nbonus2 bAddRace,RC_All,5;"},
		},
		Refine: map[string]map[int][]int{
			"Weapon": {1: {0, 200, 400, 600, 800, 1000, 1200, 1400}},
			"Armor":  {1: {0, 100, 200, 300, 400, 600, 800, 1000}},
		},
	}
}

func TestNoviceLevel1(t *testing.T) {
	c := novice()
	b := Build{Job: 0, BaseLv: 1, JobLv: 1, Stats: [6]int{1, 1, 1, 1, 1, 1}}
	Normalize(&b, c)
	r := Compute(b, c)
	check := func(name string, got, want int) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %d, want %d", name, got, want)
		}
	}
	check("points", r.Points, 48)
	check("HP", r.HP, 40) // 40 * 1.01
	check("SP", r.SP, 11)
	check("HIT", r.HIT, 177)
	check("FLEE", r.FLEE, 102)
	check("ASPD", r.ASPD, 156)
	check("status ATK", r.StatusATK, 1)
	check("soft DEF", r.SoftDEF, 1)
	check("soft MDEF", r.SoftMDEF, 1)
	check("perfect dodge", r.FLEE2, 1)
	if r.CRIT < 1.29 || r.CRIT > 1.31 {
		t.Errorf("CRIT = %v, want 1.3", r.CRIT)
	}
}

func TestEquipment(t *testing.T) {
	c := novice()
	b := Build{Job: 0, BaseLv: 3, JobLv: 1, Stats: [6]int{1, 1, 1, 1, 1, 1}, Equip: map[string]Equip{
		Weapon:  {Item: 1201, Refine: 7},
		Armor:   {Item: 2301, Refine: 5, Cards: [4]int{4001}},
		HeadTop: {Item: 5000},
		HeadMid: {Item: 5000}, // same upper+mid headgear: counted once
	}}
	Normalize(&b, c)
	r := Compute(b, c)
	if r.WeaponType != "Dagger" {
		t.Errorf("weapon type %q", r.WeaponType)
	}
	if r.EquipATK != 17+14 {
		t.Errorf("equip ATK = %d, want 31 (17 + 7*2 refine)", r.EquipATK)
	}
	if r.HardDEF != 10+3+6 {
		t.Errorf("hard DEF = %d, want 19 (armor 10 + head 3 + refine 6)", r.HardDEF)
	}
	if r.Stats[LUK].Bonus != 2 {
		t.Errorf("LUK bonus = %d, want 2 from Poring Card", r.Stats[LUK].Bonus)
	}
	if r.FLEE2 != (3+10)/10+1 {
		t.Errorf("perfect dodge = %d", r.FLEE2)
	}
	// 53 * 1.01 = 53.53 -> +10% (trunc 5) = 58
	if r.HP != 58 {
		t.Errorf("HP = %d, want 58", r.HP)
	}
	if r.SkippedScriptEffects != 0 || r.Extra["bAddRace RC_All"] != 5 {
		t.Errorf("skipped = %d, extra = %v; want the bonus2 recorded once as an extra", r.SkippedScriptEffects, r.Extra)
	}
	if r.ASPD != 156+40-55 {
		t.Errorf("ASPD with dagger = %d", r.ASPD)
	}
}

func TestNormalizeClamps(t *testing.T) {
	c := novice()
	b := Build{Job: 0, BaseLv: 999, JobLv: -5, Stats: [6]int{0, 500, 1, 1, 1, 1},
		Equip: map[string]Equip{"Nope": {Item: 1201}, Weapon: {Item: 99999}, Armor: {Item: 2301, Refine: 99}}}
	Normalize(&b, c)
	if b.BaseLv != 3 || b.JobLv != 1 || b.Stats[0] != 1 || b.Stats[1] != 130 {
		t.Errorf("not clamped: %+v", b)
	}
	if len(b.Equip) != 1 || b.Equip[Armor].Refine != 20 {
		t.Errorf("equip not cleaned: %+v", b.Equip)
	}
}

func TestOverBudget(t *testing.T) {
	c := novice()
	b := Build{Job: 0, BaseLv: 1, JobLv: 1, Stats: [6]int{30, 1, 1, 1, 1, 1}}
	Normalize(&b, c)
	if r := Compute(b, c); r.Points >= 0 || len(r.Warnings) == 0 {
		t.Errorf("expected over budget warning, points %d", r.Points)
	}
}

// Real data sanity check (skipped when the catalog has not been generated).
func TestRealDragonKnight(t *testing.T) {
	c, err := gamedata.Load(filepath.Join("..", "..", "data", "gen", "catalog.json.gz"))
	if err != nil || len(c.StatPoints) == 0 {
		t.Skip("catalog with rAthena data not generated")
	}
	b := Build{Job: 4252, BaseLv: 275, JobLv: 60, Stats: [6]int{130, 100, 100, 1, 100, 1}}
	Normalize(&b, c)
	r := Compute(b, c)
	t.Logf("DK 275/60: HP %d SP %d AP %d ATK %d+%d HIT %d FLEE %d ASPD %d points %d traits %d",
		r.HP, r.SP, r.AP, r.StatusATK, r.EquipATK, r.HIT, r.FLEE, r.ASPD, r.Points, r.TraitPoints)
	if r.HP < 50000 || r.AP == 0 || r.TraitPoints <= 0 {
		t.Errorf("implausible: %+v", r)
	}
}
