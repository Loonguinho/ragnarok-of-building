package calc

import (
	"math"
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

func skyCatalog() *gamedata.Catalog {
	return &gamedata.Catalog{Skills: map[int]*gamedata.Skill{
		5463: {ID: 5463, Code: "SKE_SKY_MASTERY", MaxLv: 10},
		5464: {ID: 5464, Code: "SKE_WAR_BOOK_MASTERY", MaxLv: 10},
	}}
}

func TestLatamRisingMoonMatchesRecording(t *testing.T) {
	// latam-ro-calc measured on a recording: naked at base 238, POW 111,
	// Sky Mastery 10, Rising Moon Lv1 → ratio 3.053 (base 1.283 x 2.38).
	in := FormulaInput{Lv: 1, BaseLv: 238, Catalog: skyCatalog(), Learned: map[int]int{5463: 10}}
	in.Traits[POW] = 111
	got := LatamFormulas["SKE_RISING_MOON"].Ratio(in)
	if math.Abs(got-3053.54) > 0.01 {
		t.Errorf("Rising Moon = %.2f%%, want 3053.54%%", got)
	}
}

func TestLatamSkyEmperorRules(t *testing.T) {
	in := FormulaInput{Lv: 5, BaseLv: 250, Catalog: skyCatalog(), Learned: map[int]int{5463: 10}, States: map[string]int{}}
	in.Traits[POW] = 100
	// (500 + 5 x (400 + 10x5) + 100x3) x 2.5 = 7625
	if got := LatamFormulas["SKE_RISING_SUN"].Ratio(in); got != 7625 {
		t.Errorf("Rising Sun = %v, want 7625", got)
	}
	// Midnight Kick is stronger in the Midnight space (and in Celestial Unity).
	normal := LatamFormulas["SKE_MIDNIGHT_KICK"].Ratio(in)
	in.States["ske_space"] = csMidnight
	if mid := LatamFormulas["SKE_MIDNIGHT_KICK"].Ratio(in); mid <= normal {
		t.Errorf("midnight %v should exceed normal %v", mid, normal)
	}
	// Noon Blast can only crit in the Noon space (or Unity).
	nb := LatamFormulas["SKE_NOON_BLAST"]
	if nb.CanCrit(in) {
		t.Error("Noon Blast crits outside Noon")
	}
	in.States["ske_space"] = csUnity
	if !nb.CanCrit(in) {
		t.Error("Noon Blast should crit in Celestial Unity")
	}
	// All in the Sky: 3 full hits vs Demi-Humans and Demons, otherwise 1.
	ais := LatamFormulas["SKE_ALL_IN_THE_SKY"]
	in.Monster = &gamedata.Monster{Race: "Demihuman"}
	if ais.Hits(in) != 3 {
		t.Error("All in the Sky vs Demi-Human should hit 3 times")
	}
	in.Monster = &gamedata.Monster{Race: "Brute"}
	if ais.Hits(in) != 1 {
		t.Error("All in the Sky vs Brute should hit once")
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
	// Rising Sun 7625% on the hand-computed base (325 avg), no DEF:
	// 325 x 76.25 = 24781; +15% bSkillAtk after DEF = 28498.
	r := baseResult()
	r.Extra["bSkillAtk SKE_RISING_SUN"] = 15
	in := FormulaInput{Lv: 5, BaseLv: 250, Catalog: skyCatalog(), Learned: map[int]int{5463: 10}}
	in.Traits[POW] = 100
	d := Damage(Build{}, r, dmgCatalog(), target(0, 0, 0), Attack{
		SkillCode: "SKE_RISING_SUN", Formula: LatamFormulas["SKE_RISING_SUN"], Input: in,
	})
	if d.Ratio != 7625 || d.Avg != 28498 {
		t.Errorf("ratio %v avg %d, want 7625 and 28498", d.Ratio, d.Avg)
	}
	if d.Crit != 0 {
		t.Error("Rising Sun cannot crit")
	}
}
