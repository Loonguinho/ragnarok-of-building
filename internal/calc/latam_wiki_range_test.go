package calc

import (
	"strings"
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

// TestRangedSkills: the listed skills are ranged whatever the weapon.
func TestRangedSkills(t *testing.T) {
	for _, code := range strings.Fields(rangedSkills) {
		n := 0
		for _, m := range latamByJob {
			if f := m[code]; f != nil {
				n++
				if f.Melee == nil && !f.Magic || f.Melee != nil && f.Melee(FormulaInput{Weapon: "Knuckle"}) {
					t.Errorf("%s follows the weapon", code)
				}
			}
		}
		if n == 0 {
			t.Errorf("%s: no formula", code)
		}
	}
}

// TestInquisitor: Punho Labareda's races, and Fogueira Espiritual's 20% of
// the maximum HP and its hits by spheres.
func TestInquisitor(t *testing.T) {
	f := latamByJob["JT_INQUISITOR"]["IQ_MASSIVE_F_BLASTER"]
	for race, want := range map[string]float64{"Brute": 23000, "Demon": 23000, "Demihuman": 21500, "Insect": 21500} {
		if got := f.Ratio(FormulaInput{Lv: 10, BaseLv: 100, Monster: &gamedata.Monster{Race: race}}); got != want {
			t.Errorf("Punho Labareda against %s: %v, want %v", race, got, want)
		}
	}
	fb := latamByJob["JT_INQUISITOR"]["IQ_THIRD_FLAME_BOMB"]
	if got := fb.Ratio(FormulaInput{Lv: 5, BaseLv: 200, MaxHP: 100000, Traits: [6]int{100}}); got != (3250+1000+20000)*2 {
		t.Errorf("Fogueira Espiritual: %v", got)
	}
}
