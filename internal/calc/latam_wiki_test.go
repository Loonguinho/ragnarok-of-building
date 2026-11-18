package calc

import (
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

// Values from bROWiki's per-level tables (base level 100, no stats).
func TestWikiFormulasMatchTables(t *testing.T) {
	cases := []struct {
		job, code, variant string
		want               map[int]float64
	}{
		{"JT_ABYSS_CHASER", "ABC_ABYSS_DAGGER", "", map[int]float64{1: 600, 5: 2600}},
		{"JT_SHADOW_CROSS", "SHC_IMPACT_CRATER", "", map[int]float64{1: 80, 5: 400}},
		{"JT_HYPER_NOVICE", "HN_HELLS_DRIVE", "", map[int]float64{1: 2200, 10: 8500}},
		{"JT_HYPER_NOVICE", "HN_METEOR_STORM_BUSTER", "Queda", map[int]float64{1: 620, 3: 1260}},
		{"JT_HYPER_NOVICE", "HN_METEOR_STORM_BUSTER", "Explosão", map[int]float64{1: 610, 3: 930}},
		{"JT_SHINKIRO", "SS_SHIMIRU", "", map[int]float64{1: 700, 5: 3500}},
		{"JT_ALCHEMIST", "CR_ACIDDEMONSTRATION", "", map[int]float64{1: 200, 10: 2000}},
		{"JT_GENETIC", "GN_CARTCANNON", "", map[int]float64{1: 250, 5: 1250}},
		{"JT_GENETIC", "GN_SPORE_EXPLOSION", "", map[int]float64{1: 600, 10: 2400}},
		{"JT_IMPERIAL_GUARD", "IG_SHIELD_SHOOTING", "", map[int]float64{1: 2700, 5: 11900}},
		{"JT_ROYAL_GUARD", "LG_SHIELDPRESS", "", map[int]float64{1: 200, 10: 2000}},
		{"JT_CRUSADER", "PA_SHIELDCHAIN", "", map[int]float64{1: 500, 5: 1300}},
		{"JT_MECHANIC", "NC_ARMSCANNON", "", map[int]float64{1: 750, 5: 2150}},
		{"JT_MECHANIC", "NC_BOOSTKNUCKLE", "", map[int]float64{1: 340, 5: 1300}},
		{"JT_MECHANIC", "NC_POWERSWING", "", map[int]float64{1: 400, 10: 1300}},
		{"JT_MECHANIC", "NC_VULCANARM", "", map[int]float64{1: 230, 3: 690}},
		{"JT_REBELLION", "RL_D_TAIL", "", map[int]float64{1: 700, 10: 2500}},
		{"JT_SORCERER", "SO_POISON_BUSTER", "", map[int]float64{1: 1300, 5: 2500}},
		{"JT_SURA", "SR_KNUCKLEARROW", "", map[int]float64{1: 600, 10: 1500}},
		{"JT_WANDERER", "WM_SEVERE_RAINSTORM", "", map[int]float64{1: 100, 5: 500}},
	}
	for _, c := range cases {
		f := latamByJob[c.job][c.code]
		if f == nil {
			t.Errorf("%s/%s: no formula", c.job, c.code)
			continue
		}
		for _, v := range f.Variants() {
			if c.variant != "" && v.Variant != c.variant {
				continue
			}
			if v.Source != wikiSource {
				t.Errorf("%s: source %q, want bROWiki", c.code, v.Source)
			}
			for lv, want := range c.want {
				in := FormulaInput{Lv: lv, BaseLv: 100, States: map[string]int{}, Monster: &gamedata.Monster{}}
				if got := v.Ratio(in); got != want {
					t.Errorf("%s %s Nv%d = %v, want %v", c.code, v.Variant, lv, got, want)
				}
			}
			break
		}
	}
}

func TestWikiHammerCoinsAndShield(t *testing.T) {
	h := latamByJob["JT_REBELLION"]["RL_HAMMER_OF_GOD"]
	in := FormulaInput{Lv: 10, BaseLv: 100, Stack: 5, States: map[string]int{}}
	// (1000 + 5 coins x 150) and x 400 when marked
	if got := h.Pick(0).Ratio(in); got != 1750 {
		t.Errorf("God's Hammer = %v, want 1750", got)
	}
	if got := h.Pick(1).Ratio(in); got != 3000 {
		t.Errorf("God's Hammer marked = %v, want 3000", got)
	}
	// Shield Press: (2000 + STR 100 + weight 250) x 2 + VIT 100 x refine 10
	sp := latamByJob["JT_ROYAL_GUARD"]["LG_SHIELDPRESS"]
	in = FormulaInput{Lv: 10, BaseLv: 200, ShieldWt: 2500, ShieldRefine: 10}
	in.Stats[STR], in.Stats[VIT] = 100, 100
	if got := sp.Ratio(in); got != 5700 {
		t.Errorf("Shield Press = %v, want 5700", got)
	}
}
