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
	LatamPassives(Build{Skills: map[int]int{5464: 10}}, c, "Book", bonus)
	if len(bonus) != 0 {
		t.Errorf("hand-written rule applied on top of the client table: %v", bonus)
	}
}
