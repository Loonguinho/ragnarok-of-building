package calc

import (
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

// TestPortedSkillTables: the hand-ported formulas give the client's table
// (base level 100, total of the hits where the client prints a total).
func TestPortedSkillTables(t *testing.T) {
	for _, tc := range []struct {
		job, code  string
		lv         int
		ratio      float64
		hits, alts int
		weapon     string
	}{
		{"JT_WIZARD", "WZ_JUPITEL", 10, 100, 12, 0, ""},
		{"JT_WIZARD", "WZ_VERMILION", 10, 1400, 1, 0, ""},
		{"JT_WIZARD", "WZ_STORMGUST", 10, 570, 10, 0, ""},
		{"JT_WIZARD", "WZ_EARTHSPIKE", 5, 200, 5, 0, ""},
		{"JT_SAGE", "WZ_HEAVENDRIVE", 5, 125, 5, 0, ""},
		{"JT_WIZARD", "WZ_WATERBALL", 5, 250, 25, 0, ""},
		{"JT_WIZARD", "WZ_METEOR", 9, 125, 5, 0, ""},
		{"JT_WIZARD", "WZ_FIREPILLAR", 10, 20, 12, 0, ""},
		{"JT_WIZARD_H", "HW_NAPALMVULCAN", 5, 350, 5, 0, ""},
		{"JT_PRIEST", "PR_MAGNUS", 10, 100, 10, 0, ""},
		{"JT_WARLOCK", "WL_TETRAVORTEX", 10, 4800, 4, 3, ""},
		{"JT_ARCH_MAGE", "AG_VIOLENT_QUAKE", 1, 1400, 1, 0, ""},
		{"JT_TAEKWON", "TK_STORMKICK", 7, 300, 1, 0, ""},
		{"JT_TAEKWON", "TK_TURNKICK", 7, 400, 1, 0, ""},
		{"JT_TAEKWON", "TK_JUMPKICK", 7, 100, 1, 0, ""},
		{"JT_RANGER", "RA_ARROWSTORM", 5, 1100, 1, 0, ""},
		{"JT_RANGER", "RA_WUGSTRIKE", 5, 1000, 1, 0, ""},
		{"JT_BARD", "BA_MUSICALSTRIKE", 5, 310, 1, 0, ""},
		{"JT_BARD_H", "CG_ARROWVULCAN", 10, 1500, 1, 0, ""},
		{"JT_MINSTREL", "WM_METALICSOUND", 10, 1200, 1, 0, ""},
		{"JT_ROGUE", "RG_RAID", 5, 800, 1, 0, ""},
		{"JT_ROGUE", "RG_BACKSTAP", 10, 350, 1, 0, "Bow"},
		{"JT_ROGUE", "RG_BACKSTAP", 10, 700, 2, 0, "Dagger"},
		{"JT_KAGEROU", "KO_JYUMONJIKIRI", 5, 1000, 1, 1, ""},
		{"JT_SURA", "SR_TIGERCANNON", 10, 0, 1, 1, ""},
		{"JT_DO_SUMMONER", "SU_SV_STEMSPEAR", 1, 700, 1, 0, ""},
	} {
		f := latamByJob[tc.job][tc.code]
		if f == nil {
			t.Errorf("%s %s: no formula", tc.job, tc.code)
			continue
		}
		in := FormulaInput{Lv: tc.lv, BaseLv: 100, Weapon: tc.weapon, Monster: &gamedata.Monster{Race: "Brute", Element: "Neutral"}}
		h := 1
		if f.Hits != nil {
			h = f.Hits(in)
		}
		if got := f.Ratio(in); got != tc.ratio || h != tc.hits || len(f.Alts) != tc.alts {
			t.Errorf("%s Nv %d: %v%% x %d (%d alts), want %v%% x %d (%d alts)", tc.code, tc.lv, got, h, len(f.Alts), tc.ratio, tc.hits, tc.alts)
		}
	}
	// Garra de Tigre: 30% of the HP + 15% of the SP, ÷ 4; + 240/Nv + 40 per
	// monster level after everything.
	f := latamByJob["JT_SURA"]["SR_TIGERCANNON"]
	in := FormulaInput{Lv: 10, BaseLv: 200, MaxHP: 40000, MaxSP: 2000, Monster: &gamedata.Monster{Level: 100}}
	if r, fl := f.Ratio(in), f.Flat(in); r != (12000+300)/4*2 || fl != 6400 {
		t.Errorf("Garra de Tigre: %v%% + %d", r, fl)
	}
}

// TestDragonBreathTerm: the two terms latam-ro-calc pinned on a recording
// (POW 113, P.ATK 54 -> 250; POW 118, P.ATK 86 -> 304).
func TestDragonBreathTerm(t *testing.T) {
	if a, b := dragonBreathTerm(5, 10, 113, 54), dragonBreathTerm(5, 10, 118, 86); a != 250 || b != 304 {
		t.Errorf("terms %d and %d, want 250 and 304", a, b)
	}
	if got := dragonBreathTerm(5, 0, 113, 54); got != 140 {
		t.Errorf("without Aura Draconiana: %d, want 140", got)
	}
}

// TestPhysicalSkillElement: a physical skill's own element scales weapon and
// equipment ATK, not the weapon's element; status ATK stays neutral.
func TestPhysicalSkillElement(t *testing.T) {
	c := dmgCatalog()
	c.AttrFix[1]["Neutral"]["Water"], c.AttrFix[1]["Fire"]["Neutral"] = 100, 100
	m := target(0, 0, 0)
	m.Element = "Water"
	f := &SkillFormula{Code: "X", Ratio: func(FormulaInput) float64 { return 100 }}
	plain := Damage(Build{}, baseResult(), c, m, Attack{SkillCode: "X", Formula: f})
	g := *f
	g.Element = "Fire" // 50% on Water
	fire := Damage(Build{}, baseResult(), c, m, Attack{SkillCode: "X", Formula: &g})
	if fire.Element != "Fire" || fire.Avg >= plain.Avg {
		t.Errorf("fire skill: %s %d, neutral %d", fire.Element, fire.Avg, plain.Avg)
	}
	// A different endow does not override the skill's element.
	if d := Damage(Build{}, baseResult(), c, m, Attack{SkillCode: "X", Formula: &g, Element: "Neutral"}); d.Avg != fire.Avg {
		t.Errorf("with an endow: %d, want %d", d.Avg, fire.Avg)
	}
}

// TestTargetAndElementStates: Remoção Sombria Total lowers RES, the spirit
// charms and Cores do Arco-Íris set the skill's element.
func TestTargetAndElementStates(t *testing.T) {
	m := target(0, 0, 0)
	m.RES = 100
	b := Build{}
	plain := Damage(b, baseResult(), dmgCatalog(), m, Attack{}).Avg
	b.Atk.States = map[string]int{stateKey("Strip Shadow"): 5}
	m50 := *m
	m50.RES = 50
	if got, want := Damage(b, baseResult(), dmgCatalog(), m, Attack{}).Avg, Damage(Build{}, baseResult(), dmgCatalog(), &m50, Attack{}).Avg; got != want || got <= plain {
		t.Errorf("Remoção Sombria: %d, want %d (> %d)", got, want, plain)
	}
	for _, tc := range []struct {
		job, code, state string
		v                int
		want             string
	}{
		{"JT_KAGEROU", "KO_KAIHOU", "Spirit Charm", 3, "Wind"},
		{"JT_SPIRIT_HANDLER", "SH_HYUN_ROKS_BREEZE", "Colors of Hynrok", 5, "Dark"},
		{"JT_DO_SUMMONER", "SU_CN_METEOR", "Colors of Hynrok", 0, "Neutral"},
		{"JT_RUNE_KNIGHT", "RK_DRAGONBREATH", "Turisus Runestone", 1, "Holy"},
	} {
		f := latamByJob[tc.job][tc.code]
		if got := f.ElementOf(FormulaInput{Lv: 1, States: map[string]int{stateKey(tc.state): tc.v}}); got != tc.want {
			t.Errorf("%s: %s, want %s", tc.code, got, tc.want)
		}
	}
}
