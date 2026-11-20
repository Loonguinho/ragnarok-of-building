package view

import (
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

func TestBonusText(t *testing.T) {
	c := testCatalog()
	c.Skills[9001] = &gamedata.Skill{ID: 9001, Code: "MG_FIREBOLT", Name: gamedata.Text{"pt": "Lanças de Fogo", "en": "Fire Bolt"}}
	cases := []struct {
		k, l       string
		v          int
		label, val string
	}{
		{"bSPLossRate 5", "pt", 10000, "Perde 5 SP a cada", "10 s"},
		{"bSPLossRate 5", "en", 10000, "Lose 5 SP every", "10 s"},
		{"bAutoSpell MG_FIREBOLT 3", "pt", 50, "Chance de conjurar Lanças de Fogo Nv. 3", "5%"},
		{"bAddClass Class_Boss", "pt", 15, "Dano físico vs chefes", "+15%"},
		{"bAtkEle Ele_Fire", "pt", 1, "Elemento do ataque: Fogo", ""},
		{"bSkillCooldown MG_FIREBOLT", "pt", -1500, "Recarga de Lanças de Fogo", "-1.5 s"},
		{"bStr", "pt", 3, "FOR", "+3"},
		{"bAddRace RC_DemiHuman", "pt", 10, "Dano físico vs Humanoide", "+10%"},
	}
	for _, cs := range cases {
		lb, v := bonusText(cs.k, cs.v, c, cs.l)
		if lb != cs.label || v != cs.val {
			t.Errorf("bonusText(%q, %d, %s) = %q %q, want %q %q", cs.k, cs.v, cs.l, lb, v, cs.label, cs.val)
		}
	}
}
