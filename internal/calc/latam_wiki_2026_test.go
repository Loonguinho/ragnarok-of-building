package calc

import (
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

// TestRebalance2026: the added skills match their bROWiki pages: the table
// at base level 100 with no POW, POW × 7 (× 10 in the Eureka/Aura forms and
// on Laser Triplo), Baforada's HP/SP factor, and the hits.
func TestRebalance2026(t *testing.T) {
	up := func(state string, in FormulaInput) FormulaInput {
		in.States = map[string]int{stateKey(state): 1}
		return in
	}
	for _, tc := range []struct {
		job, code string
		in        FormulaInput
		ratio     float64
		hits      int
	}{
		{"JT_DRAGON_KNIGHT", "DK_DRAGONIC_BREATH", FormulaInput{Lv: 1, BaseLv: 100}, 400, 1},
		// Nv 10: 3.550% + 50 x 0,01 x (80.000/8 + 4.000/4) + POW 100 x 7, at base 200.
		{"JT_DRAGON_KNIGHT", "DK_DRAGONIC_BREATH", FormulaInput{Lv: 10, BaseLv: 200, MaxHP: 80000, MaxSP: 4000, Traits: [6]int{100}}, (3550 + 5500 + 700) * 2, 1},
		{"JT_DRAGON_KNIGHT", "DK_DRAGONIC_BREATH", up("Dragonic Aura", FormulaInput{Lv: 10, BaseLv: 100, MaxHP: 80000, MaxSP: 4000, Traits: [6]int{100}}), 3550 + 7700 + 1000, 1},
		{"JT_MEISTER", "MT_SPARK_BLASTER", FormulaInput{Lv: 10, BaseLv: 100}, 7750, 2},
		{"JT_MEISTER", "MT_TRIPLE_LASER", FormulaInput{Lv: 2, BaseLv: 100, Traits: [6]int{10}}, 1500 + 100, 3},
		{"JT_MEISTER", "MT_MIGHTY_SMASH", FormulaInput{Lv: 5, BaseLv: 100, Traits: [6]int{10}}, 1600 + 70, 3},
		{"JT_MEISTER", "MT_MIGHTY_SMASH", up("Axe Stomp", FormulaInput{Lv: 5, BaseLv: 100, Traits: [6]int{10}}), 1650 + 70, 5},
		{"JT_BIOLO", "BO_EXPLOSIVE_POWDER", up("Research Report", FormulaInput{Lv: 2, BaseLv: 100}), 1500, 5},
		{"JT_BIOLO", "BO_MAYHEMIC_THORNS", FormulaInput{Lv: 6, BaseLv: 100, Traits: [6]int{10}}, 1700 + 70, 3},
		{"JT_BIOLO", "BO_MAYHEMIC_THORNS", up("Research Report", FormulaInput{Lv: 10, BaseLv: 100, Traits: [6]int{10}}), 3250 + 100, 5},
		{"JT_BIOLO", "BO_ACIDIFIED_ZONE_FIRE", FormulaInput{Lv: 5, BaseLv: 200, Traits: [6]int{10}}, (2000 + 10) * 2, 7},
		{"JT_BIOLO", "BO_ACIDIFIED_ZONE_FIRE", up("Research Report", FormulaInput{Lv: 5, BaseLv: 100, Traits: [6]int{10}, Monster: &gamedata.Monster{Race: "Plant"}}), (2000 + 15) * 1.5, 7},
	} {
		f := latamByJob[tc.job][tc.code]
		if f == nil {
			t.Errorf("%s: no formula", tc.code)
			continue
		}
		h := 1
		if f.Hits != nil {
			h = f.Hits(tc.in)
		}
		if got := f.Ratio(tc.in); got != tc.ratio || h != tc.hits || len(f.Alts) != 0 {
			t.Errorf("%s Nv %d %v: %v%% x %d (%s, %d alts), want %v%% x %d", tc.code, tc.in.Lv, tc.in.States, got, h, f.Source, len(f.Alts), tc.ratio, tc.hits)
		}
	}
	// Sopro do Dragão is not Baforada Draconiana (the generator mixed them).
	if f := latamByJob["JT_DRAGON_KNIGHT"]["RK_DRAGONBREATH"]; f != nil || latamByJob["JT_RUNE_KNIGHT"]["RK_DRAGONBREATH"].Name != "Dragon Breath" {
		t.Error("Sopro do Dragão: wrong formula")
	}
}

// TestWikiCoefficientFixes: formulas whose attribute terms were checked
// against bROWiki (tools/browiki COMPARE.md, coefficients).
func TestWikiCoefficientFixes(t *testing.T) {
	ratio := func(job, code, variant string, in FormulaInput) float64 {
		for _, v := range latamByJob[job][code].Variants() {
			if variant == "" || v.Variant == variant {
				return v.Ratio(in)
			}
		}
		t.Fatalf("%s: no variant %q", code, variant)
		return 0
	}
	in := func(lv, base int, stats, traits [6]int) FormulaInput {
		return FormulaInput{Lv: lv, BaseLv: base, Stats: stats, Traits: traits, States: map[string]int{}}
	}
	for _, tc := range []struct {
		name string
		got  float64
		want float64
	}{
		{"Infiltrar: CON x 3", ratio("JT_SHINKIRO", "SS_SHIMIRU", "", in(5, 200, [6]int{}, [6]int{4: 10})), (3500 + 30) * 2},
		{"Golpe da Palma: FOR x 2 x base", ratio("JT_MONK_H", "CH_PALMSTRIKE", "", in(5, 200, [6]int{STR: 100}, [6]int{})), (1000 + 200) * 2},
		{"Impacto Sísmico visível", ratio("JT_SURA", "SR_EARTHSHAKER", "Alvo visível", in(5, 200, [6]int{STR: 100}, [6]int{})), (1500 + 300) * 2},
		{"Impacto Sísmico oculto", ratio("JT_SURA", "SR_EARTHSHAKER", "Alvo oculto", in(5, 200, [6]int{STR: 100}, [6]int{})), (2000 + 500) * 2},
		{"Zona Gravitacional: zona", ratio("JT_HYPER_NOVICE", "HN_GROUND_GRAVITATION", "Zona (1º dano)", in(10, 100, [6]int{}, [6]int{SPL: 10})), 18000 + 50},
		{"Zona Gravitacional: gravidade", ratio("JT_HYPER_NOVICE", "HN_GROUND_GRAVITATION", "Gravidade (contínuo)", in(10, 100, [6]int{}, [6]int{SPL: 10})), 7800 + 20},
	} {
		if tc.got != tc.want {
			t.Errorf("%s: %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

// TestWiki2026CritHalf: the client's text for the 2026 skills that can
// crit says "A chance é a metade de CRIT do usuário": half the chance, the
// whole crit damage.
func TestWiki2026CritHalf(t *testing.T) {
	for _, tc := range []struct{ job, code string }{
		{"JT_MEISTER", "MT_TRIPLE_LASER"},
		{"JT_BIOLO", "BO_MAYHEMIC_THORNS"},
	} {
		f := latamByJob[tc.job][tc.code]
		if f == nil {
			t.Errorf("%s: no formula", tc.code)
			continue
		}
		if f.CanCrit == nil || f.CritRate != 0.5 || (f.CritDmg != 0 && f.CritDmg != 1) {
			t.Errorf("%s: can crit %v, crit rate part %v, crit damage part %v; want 0.5 and the whole damage", tc.code, f.CanCrit != nil, f.CritRate, f.CritDmg)
		}
	}
}
