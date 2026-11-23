package calc

import (
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

func starCatalog() *gamedata.Catalog {
	c := dmgCatalog()
	c.Jobs = map[int]*gamedata.Job{
		4302: {ID: 4302, Code: "JT_SKY_EMPEROR", Parent: 4239},
		4239: {ID: 4239, Code: "JT_STAR_EMPEROR", Parent: -1},
		4047: {ID: 4047, Code: "JT_STAR", Parent: -1},
	}
	c.Skills = map[int]*gamedata.Skill{424: {ID: 424, Code: "TK_POWER", MaxLv: 5}}
	return c
}

func TestStarEmperorKihopAndWrath(t *testing.T) {
	c := starCatalog()
	b := Build{Job: 4302, BaseLv: 250, Skills: map[int]int{424: 5}}
	// Kihop Nv 5: 325 + floor(325 x 85%) = 601.
	if d := Damage(b, baseResult(), c, target(0, 0, 0), Attack{}); d.Avg != 601 {
		t.Errorf("Kihop: avg %d, want 601", d.Avg)
	}
	// Fúria Lunar reaches a Medium target (HP 10000 >= 6000): +(250+0+0)/3 = 83%,
	// floor(601 x 1.83) = 1099; Fúria Solar (Small only) adds nothing.
	b.Atk.States = map[string]int{stateKey("Wrath of Sun"): 3, stateKey("Wrath of Moon"): 3}
	if d := Damage(b, baseResult(), c, target(0, 0, 0), Attack{}); d.Avg != 1099 {
		t.Errorf("Kihop + Wrath of Moon: avg %d, want 1099", d.Avg)
	}
	// Outside the Star Emperor line Kihop is not an ATK multiplier.
	b = Build{Job: 4047, BaseLv: 99, Skills: map[int]int{424: 5}}
	if d := Damage(b, baseResult(), c, target(0, 0, 0), Attack{}); d.Avg != 325 {
		t.Errorf("Star Gladiator: avg %d, want 325", d.Avg)
	}
}

func TestWarBookMasteryNotDoubled(t *testing.T) {
	// With the client's level table the passive comes from applyClassPassives;
	// the hand-written rule must not add it again.
	c := skyCatalog()
	c.Skills[5464].Effects = &gamedata.SkillEffects{Kind: "passive", Weapons: []string{"Book"}, Levels: map[int]string{10: "bonus bPAtk,12; bonus bHit,30;"}}
	bonus := map[string]int{}
	LatamPassives(Build{Skills: map[int]int{5464: 10}}, c, "Book", bonus, map[string]int{})
	if len(bonus) != 0 {
		t.Errorf("hand-written rule applied on top of the client table: %v", bonus)
	}
}

func ruleCatalog() *gamedata.Catalog {
	return &gamedata.Catalog{
		Jobs: map[int]*gamedata.Job{
			1: {ID: 1, Code: "JT_TAEKWON", Parent: -1}, 2: {ID: 2, Code: "JT_STAR", Parent: 1},
			3: {ID: 3, Code: "JT_KNIGHT_H", Parent: -1}, 4: {ID: 4, Code: "JT_RUNE_KNIGHT_H", Parent: 3},
			5: {ID: 5, Code: "JT_SHADOW_CROSS", Parent: -1}, 6: {ID: 6, Code: "JT_ABYSS_CHASER", Parent: -1},
			7: {ID: 7, Code: "JT_ARCH_MAGE", Parent: -1, Tree: map[int]int{0: 5200}},
		},
		Skills: map[int]*gamedata.Skill{
			411: {ID: 411, Code: "TK_RUN"}, 55: {ID: 55, Code: "KN_SPEARMASTERY"}, 2007: {ID: 2007, Code: "RK_DRAGONTRAINING"},
			6001: {ID: 6001, Code: "SHC_SHADOW_SENSE"}, 6002: {ID: 6002, Code: "ABC_DAGGER_AND_BOW_M"},
			5200: {ID: 5200, Code: "AG_TWOHANDSTAFF", Effects: &gamedata.SkillEffects{Kind: "passive", Levels: map[int]string{10: "bonus bSMatk,20;"}}},
		},
	}
}

func TestLatamClassRules(t *testing.T) {
	c := ruleCatalog()
	run := func(b Build, weapon string) (map[string]int, map[string]int) {
		bonus, extra := map[string]int{}, map[string]int{}
		LatamPassives(b, c, weapon, bonus, extra)
		return bonus, extra
	}
	// Corrida: +100 mastery ATK bare-handed on the Taekwon line only.
	if bn, _ := run(Build{Job: 2, Skills: map[int]int{411: 10}}, "Fist"); bn["bMasteryAtk"] != 100 {
		t.Errorf("Run bare-handed: %v", bn)
	}
	if bn, _ := run(Build{Job: 2, Skills: map[int]int{411: 10}}, "Book"); bn["bMasteryAtk"] != 0 {
		t.Errorf("Run with a book: %v", bn)
	}
	// Rune Knight (transcendent line): Dragon Training +10/lv; on a dragon +1/lv and no size penalty.
	b := Build{Job: 4, Skills: map[int]int{55: 10, 2007: 1}, Atk: AttackChoice{States: map[string]int{stateKey("Ride Dragon"): 1}}}
	if bn, _ := run(b, "2hSpear"); bn["bMasteryAtk"] != 110 || bn["bNoSizeFix"] != 1 {
		t.Errorf("Rune Knight spear: %v", bn)
	}
	// Senso das Sombras Nv 10: FLEE +100, CRIT +50 with a dagger.
	if bn, _ := run(Build{Job: 5, Skills: map[int]int{6001: 10}}, "Dagger"); bn["bFlee"] != 100 || bn["bCritical"] != 50 {
		t.Errorf("Shadow Sense: %v", bn)
	}
	// Versatilidade Nv 10 with a bow: +10/+15/+20% by size; nothing with a sword.
	if _, ex := run(Build{Job: 6, Skills: map[int]int{6002: 10}}, "Bow"); ex["bAddSize Size_Small"] != 10 || ex["bAddSize Size_Medium"] != 15 || ex["bAddSize Size_Large"] != 20 {
		t.Errorf("Dagger & Bow Mastery: %v", ex)
	}
	if _, ex := run(Build{Job: 6, Skills: map[int]int{6002: 10}}, "1hSword"); len(ex) != 0 {
		t.Errorf("Dagger & Bow Mastery with a sword: %v", ex)
	}
	// Two-Handed Staff Mastery (client table): S.MATK only with a two-handed staff.
	for weapon, want := range map[string]int{"2hStaff": 20, "Staff": 0} {
		bonus, extra := map[string]int{}, map[string]int{}
		applyClassPassives(&Build{Job: 7, Skills: map[int]int{5200: 10}}, c, weapon, bonus, extra)
		if bonus["bSMatk"] != want {
			t.Errorf("Two-Handed Staff Mastery with %s: S.MATK %d, want %d", weapon, bonus["bSMatk"], want)
		}
	}
}

// TestWrathLevelsAndMiracle: the Wrath divisor follows its level (÷9, ÷6,
// ÷3, bROWiki), and Milagre lets Fúria Estelar (with FOR) reach any target.
func TestWrathLevelsAndMiracle(t *testing.T) {
	c := starCatalog()
	b := Build{Job: 4302, BaseLv: 240}
	// No Kihop: 325 ATK. Medium target, Fúria Lunar Nv 1: +240/9 = 26%.
	b.Atk.States = map[string]int{stateKey("Wrath of Moon"): 1}
	if d := Damage(b, baseResult(), c, target(0, 0, 0), Attack{}); d.Avg != 409 {
		t.Errorf("Fúria Lunar Nv 1: avg %d, want 409", d.Avg)
	}
	// Nv 2: +240/6 = 40%.
	b.Atk.States[stateKey("Wrath of Moon")] = 2
	if d := Damage(b, baseResult(), c, target(0, 0, 0), Attack{}); d.Avg != 455 {
		t.Errorf("Fúria Lunar Nv 2: avg %d, want 455", d.Avg)
	}
	// Fúria Estelar alone does not reach a Medium target...
	b.Atk.States = map[string]int{stateKey("Wrath of"): 3}
	if d := Damage(b, baseResult(), c, target(0, 0, 0), Attack{}); d.Avg != 325 {
		t.Errorf("Fúria Estelar on Medium: avg %d, want 325", d.Avg)
	}
	// ...but does under Milagre: +(240 + FOR 48)/3 = 96%.
	b.Atk.States[stateKey("Miracle")] = 1
	if d := Damage(b, baseResult(), c, target(0, 0, 0), Attack{}); d.Avg != 637 {
		t.Errorf("Fúria Estelar + Milagre: avg %d, want 637", d.Avg)
	}
}
