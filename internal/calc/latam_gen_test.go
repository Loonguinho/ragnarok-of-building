package calc

import (
	"math"
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

func magusCatalog() *gamedata.Catalog {
	return &gamedata.Catalog{Jobs: map[int]*gamedata.Job{4255: {ID: 4255, Code: "JT_ARCH_MAGE", Parent: -1}}}
}

// Expected ratios from latam-ro-calc's ArchMage.damage-components.spec.ts
// (the client's own table, base 100, no SPL).
func TestGeneratedMagusRatios(t *testing.T) {
	cases := []struct {
		code string
		lv   int
		want float64
	}{
		{"AG_ASTRAL_STRIKE", 10, 18300},
		{"AG_CRIMSON_ARROW", 5, 1500},
		{"AG_CRYSTAL_IMPACT", 5, 6750},
	}
	for _, c := range cases {
		f := FormulaFor(magusCatalog(), 4255, c.code)
		if f == nil {
			t.Errorf("%s: no generated formula", c.code)
			continue
		}
		if !f.Magic {
			t.Errorf("%s: should be magic", c.code)
		}
		got := f.Ratio(FormulaInput{Lv: c.lv, BaseLv: 100, Catalog: magusCatalog(), States: map[string]int{}})
		if math.Abs(got-c.want) > 0.01 {
			t.Errorf("%s Lv%d = %.2f%%, want %.0f%%", c.code, c.lv, got, c.want)
		}
	}
}

// Every generated formula must evaluate without panicking on empty input.
func TestGeneratedFormulasEvaluate(t *testing.T) {
	n := 0
	for job, m := range latamByJob {
		for code, f := range m {
			n++
			for lv := 1; lv <= 10; lv++ {
				in := FormulaInput{Lv: lv, BaseLv: 200, States: map[string]int{}, Monster: &gamedata.Monster{Race: "Brute"}}
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Errorf("%s/%s Lv%d panicked: %v", job, code, lv, r)
						}
					}()
					v := f.Ratio(in)
					if math.IsNaN(v) || math.IsInf(v, 0) {
						t.Errorf("%s/%s Lv%d = %v", job, code, lv, v)
					}
					if f.Hits != nil {
						f.Hits(in)
					}
					if f.CanCrit != nil {
						f.CanCrit(in)
					}
				}()
			}
		}
	}
	if n < 200 {
		t.Errorf("only %d generated formulas registered", n)
	}
}

// Hand-computed magic pipeline: status MATK 1000, weapon MATK 200 lv4
// (variance 80), skill 1000%, neutral vs neutral, no defenses:
// avg (1000+200) x 10 = 12000.
func TestMagicPipeline(t *testing.T) {
	r := Result{MATKMin: 1000, WeaponMATK: 200, WeaponLv: 4, Bonuses: map[string]int{}, Extra: map[string]int{}}
	f := &SkillFormula{Code: "X", Magic: true, Element: "Fire", CritRate: 1, CritDmg: 1,
		Ratio: func(FormulaInput) float64 { return 1000 }}
	m := target(0, 0, 0)
	c := dmgCatalog()
	d := Damage(Build{}, r, c, m, Attack{SkillCode: "X", Formula: f})
	if d.Avg != 12000 || d.Min != 11200 || d.Max != 12800 || d.HitChance != 100 || d.Crit != 0 {
		t.Errorf("magic min/avg/max %d/%d/%d hit %v crit %d", d.Min, d.Avg, d.Max, d.HitChance, d.Crit)
	}
	// MDEF 100 + soft MDEF (10+0)/4=2: 12000 x (1100/2000) = 6600 - 2 = 6598.
	m2 := target(0, 0, 10)
	m2.MDEF = 100
	if d := Damage(Build{}, r, c, m2, Attack{SkillCode: "X", Formula: f}); d.Avg != 6598 {
		t.Errorf("with MDEF: %d, want 6598", d.Avg)
	}
	// Fire vs Water lv1 = 50% (table in dmgCatalog).
	m3 := target(0, 0, 0)
	m3.Element = "Water"
	if d := Damage(Build{}, r, c, m3, Attack{SkillCode: "X", Formula: f}); d.Avg != 6000 {
		t.Errorf("fire vs water: %d, want 6000", d.Avg)
	}
}
