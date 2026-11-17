package calc

import (
	"math"
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

// skyCatalog has the Sky Emperor job and the masteries the formulas read.
func skyCatalog() *gamedata.Catalog {
	return &gamedata.Catalog{
		Jobs: map[int]*gamedata.Job{4302: {ID: 4302, Code: "JT_SKY_EMPEROR", Parent: -1}},
		Skills: map[int]*gamedata.Skill{
			5463: {ID: 5463, Code: "SKE_SKY_MASTERY", MaxLv: 10, Name: gamedata.Text{"en": "Sky Mastery"}},
			5464: {ID: 5464, Code: "SKE_WAR_BOOK_MASTERY", MaxLv: 10, Name: gamedata.Text{"en": "War Book Mastery"}},
		},
	}
}

func sky(t *testing.T, code string) *SkillFormula {
	t.Helper()
	f := FormulaFor(skyCatalog(), 4302, code)
	if f == nil {
		t.Fatalf("no generated formula for %s", code)
	}
	return f
}

func TestLatamRisingMoonCurrentFormula(t *testing.T) {
	// LATAM uses the 2022 rebalance since the 28/10/2026 client:
	// celestial(700, 450, 5, 3) = (700 + Lv x (450 + Sky Mastery x 5) + POW x 3) x BaseLv/100.
	// (latam-ro-calc's recording at base 238 / POW 111 / Sky Mastery 10 measured
	// 3053.54% before that client, with the old 600 + 300/Lv formula.)
	in := FormulaInput{Lv: 1, BaseLv: 238, Catalog: skyCatalog(), Learned: map[int]int{5463: 10}}
	in.Traits[POW] = 111
	got := sky(t, "SKE_RISING_MOON").Ratio(in)
	if math.Abs(got-3648.54) > 0.01 {
		t.Errorf("Rising Moon = %.2f%%, want 3648.54%%", got)
	}
	if !sky(t, "SKE_RISING_MOON").Rebalanced {
		t.Error("Rising Moon should come from the 2022 rebalance")
	}
}

func TestLatamSkyEmperorRules(t *testing.T) {
	in := FormulaInput{Lv: 5, BaseLv: 250, Catalog: skyCatalog(), Learned: map[int]int{5463: 10}, States: map[string]int{}}
	in.Traits[POW] = 100
	// celestial(500, 600, 5, 5): (500 + 5 x (600 + 10x5) + 100x5) x 2.5 = 10625
	if got := sky(t, "SKE_RISING_SUN").Ratio(in); got != 10625 {
		t.Errorf("Rising Sun = %v, want 10625", got)
	}
	space := stateKey("_SkyEmperor_Celestial_Space")
	normal := sky(t, "SKE_MIDNIGHT_KICK").Ratio(in)
	in.States[space] = 5 // Midnight
	if mid := sky(t, "SKE_MIDNIGHT_KICK").Ratio(in); mid <= normal {
		t.Errorf("midnight %v should exceed normal %v", mid, normal)
	}
	nb := sky(t, "SKE_NOON_BLAST")
	in.States[space] = 0
	if nb.CanCrit(in) {
		t.Error("Noon Blast crits outside Noon")
	}
	in.States[space] = 7 // Celestial Unity counts as every space
	if !nb.CanCrit(in) {
		t.Error("Noon Blast should crit in Celestial Unity")
	}
	ais := sky(t, "SKE_ALL_IN_THE_SKY")
	in.Monster = &gamedata.Monster{Race: "Demihuman"}
	if ais.Hits(in) != 3 {
		t.Error("All in the Sky vs Demi-Human should hit 3 times")
	}
	in.Monster = &gamedata.Monster{Race: "Brute"}
	if ais.Hits(in) != 1 {
		t.Error("All in the Sky vs Brute should hit once")
	}
}

func TestPerClassLookup(t *testing.T) {
	// The same skill code resolves to the closest class's formula.
	c := &gamedata.Catalog{Jobs: map[int]*gamedata.Job{
		4060: {ID: 4060, Code: "JT_RUNE_KNIGHT_H", Parent: -1},
		4252: {ID: 4252, Code: "JT_DRAGON_KNIGHT", Parent: 4060},
	}}
	regJob("JT_RUNE_KNIGHT_H", &SkillFormula{Code: "TEST_SKILL", Name: "rk"})
	regJob("JT_DRAGON_KNIGHT", &SkillFormula{Code: "TEST_SKILL", Name: "dk"})
	if f := FormulaFor(c, 4252, "TEST_SKILL"); f == nil || f.Name != "dk" {
		t.Errorf("DK got %+v", f)
	}
	if f := FormulaFor(c, 4060, "TEST_SKILL"); f == nil || f.Name != "rk" {
		t.Errorf("RK got %+v", f)
	}
}

func TestWarBookMastery(t *testing.T) {
	c := skyCatalog()
	b := Build{Skills: map[int]int{5464: 10}}
	bonus := map[string]int{}
	LatamPassives(b, c, "Book", bonus)
	if bonus["bPAtk"] != 12 || bonus["bHit"] != 30 {
		t.Errorf("with book: %v, want P.ATK 12 HIT 30", bonus)
	}
	bonus = map[string]int{}
	LatamPassives(b, c, "Knuckle", bonus)
	if len(bonus) != 0 {
		t.Errorf("without a book: %v", bonus)
	}
}

func TestDamageWithFormulaAndSkillBonus(t *testing.T) {
	// Rising Sun 10625% on the hand-computed base (325 avg), no DEF:
	// floor(325 x 106.25) = 34531; +15% bSkillAtk after DEF = 39710.
	r := baseResult()
	r.Extra["bSkillAtk SKE_RISING_SUN"] = 15
	in := FormulaInput{Lv: 5, BaseLv: 250, Catalog: skyCatalog(), Learned: map[int]int{5463: 10}}
	in.Traits[POW] = 100
	f := sky(t, "SKE_RISING_SUN")
	d := Damage(Build{}, r, dmgCatalog(), target(0, 0, 0), Attack{SkillCode: "SKE_RISING_SUN", Formula: f, Input: in})
	if d.Ratio != 10625 || d.Avg != 39710 {
		t.Errorf("ratio %v avg %d, want 10625 and 39710", d.Ratio, d.Avg)
	}
	if d.Crit != 0 {
		t.Error("Rising Sun cannot crit")
	}
}
