package calc

import (
	"strings"
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

// dmgCatalog: neutral/fire/water tables and a plain target.
func dmgCatalog() *gamedata.Catalog {
	return &gamedata.Catalog{
		AttrFix: map[int]map[string]map[string]int{1: {
			"Neutral": {"Neutral": 100, "Water": 100},
			"Fire":    {"Neutral": 100, "Water": 50},
			"Water":   {"Neutral": 100, "Water": 25},
		}},
		SizeFix: map[string]map[string]int{"Knuckle": {"Small": 100, "Medium": 100, "Large": 75}},
	}
}

func target(def, vit, lv int) *gamedata.Monster {
	return &gamedata.Monster{ID: 1, Name: "Alvo", Level: lv, HP: 10000, DEF: def, Stats: [6]int{1, 1, vit, 1, 1, 1},
		Size: "Medium", Race: "Brute", Element: "Neutral", ElementLv: 1, Class: "Normal", DamageTaken: 100}
}

// Hand-computed: status 100 → 200; weapon 100 wlv1 STR 50: variance 5,
// STR bonus 25 → 120..130 (avg 125); total 320..330 (avg 325).
func baseResult() Result {
	r := Result{StatusATK: 100, WeaponATK: 100, WeaponLv: 1, WeaponType: "Dagger", HIT: 1000, CritTenths: 100}
	r.st[STR] = 50
	r.Bonuses, r.Extra = map[string]int{}, map[string]int{}
	return r
}

func TestDamageBasic(t *testing.T) {
	d := Damage(Build{}, baseResult(), dmgCatalog(), target(0, 0, 0), Attack{})
	if d.Min != 320 || d.Avg != 325 || d.Max != 330 {
		t.Errorf("min/avg/max = %d/%d/%d, want 320/325/330", d.Min, d.Avg, d.Max)
	}
	if d.Crit != 462 { // 330 x 1.4
		t.Errorf("crit = %d, want 462", d.Crit)
	}
	if d.HitsToKill != 31 { // 10000 / 325
		t.Errorf("hits to kill = %d", d.HitsToKill)
	}
	if len(d.Steps) == 0 {
		t.Error("no breakdown")
	}
}

func TestDamageDefense(t *testing.T) {
	// 325 x (4000+100)/(4000+1000) = 266 (int) - soft DEF (10+10)/2 = 10 → 256
	d := Damage(Build{}, baseResult(), dmgCatalog(), target(100, 10, 10), Attack{})
	if d.Avg != 256 {
		t.Errorf("avg = %d, want 256", d.Avg)
	}
	r := baseResult()
	r.Extra["bIgnoreDefRaceRate RC_Brute"] = 100
	if d := Damage(Build{}, r, dmgCatalog(), target(100, 10, 10), Attack{}); d.Avg != 325 {
		t.Errorf("ignore 100%% DEF: avg = %d, want 325", d.Avg)
	}
}

func TestDamageModifiers(t *testing.T) {
	r := baseResult()
	r.Extra["bAddRace RC_Brute"] = 10 // race +10%
	r.Extra["bAddRace RC_All"] = 10   // same category: +20% total
	r.Extra["bAddSize Size_Medium"] = 20
	// weapon 125 x 1.2 x 1.2 = 180 (integer steps: 1000→1200→1440); total 200 + 180 = 380
	if d := Damage(Build{}, r, dmgCatalog(), target(0, 0, 0), Attack{}); d.Avg != 380 {
		t.Errorf("avg = %d, want 380", d.Avg)
	}
	// Card modifiers do not touch status ATK: with no weapon only 2 x status.
	r2 := Result{StatusATK: 100, Bonuses: map[string]int{}, Extra: map[string]int{"bAddRace RC_All": 50}, HIT: 1000}
	if d := Damage(Build{}, r2, dmgCatalog(), target(0, 0, 0), Attack{}); d.Avg != 200 {
		t.Errorf("fist avg = %d, want 200", d.Avg)
	}
}

func TestDamageElementSizeSkill(t *testing.T) {
	r := baseResult()
	r.Extra["bAtkEle Ele_Fire"] = 1
	m := target(0, 0, 0)
	m.Element = "Water"
	// status neutral 200; weapon fire vs water 50%: 125 → 62; total 262
	d := Damage(Build{}, r, dmgCatalog(), m, Attack{})
	if d.Element != "Fire" || d.Avg != 262 {
		t.Errorf("fire vs water: %s %d, want Fire 262", d.Element, d.Avg)
	}
	// An endow overrides the weapon and, in LATAM, reaches status ATK too:
	// status 100 x 25% x 2 = 50, weapon 125 x 25% = 31.
	if d := Damage(Build{}, r, dmgCatalog(), m, Attack{Element: "Water"}); d.Avg != 50+31 {
		t.Errorf("water endow vs water: %d, want 81", d.Avg)
	}
	// ...except the poison endow, which leaves status ATK neutral (200).
	if d := Damage(Build{}, r, dmgCatalog(), m, Attack{Element: "Poison"}); d.Avg != 200+125 {
		t.Errorf("poison endow vs water: %d, want 325", d.Avg)
	}
	// Knuckle vs Large: 75% on the weapon part only.
	r2 := baseResult()
	r2.WeaponType = "Knuckle"
	big := target(0, 0, 0)
	big.Size = "Large"
	if d := Damage(Build{}, r2, dmgCatalog(), big, Attack{}); d.Avg != 200+93 {
		t.Errorf("knuckle vs large: %d, want 293", d.Avg)
	}
	// bNoSizeFix (Carta Drake): no size penalty.
	r2.Bonuses["bNoSizeFix"] = 1
	if d := Damage(Build{}, r2, dmgCatalog(), big, Attack{}); d.Avg != 325 {
		t.Errorf("no size fix vs large: %d, want 325", d.Avg)
	}
	// Skill: 300% x 2 hits, no criticals.
	d = Damage(Build{}, baseResult(), dmgCatalog(), target(0, 0, 0), Attack{SkillCode: "X", SkillRatio: 300, Hits: 2})
	if d.Avg != 325*3*2 || d.Crit != 0 {
		t.Errorf("skill: avg %d crit %d, want 1950 and no crit", d.Avg, d.Crit)
	}
}

func TestDamageRESAndTaken(t *testing.T) {
	m := target(0, 0, 0)
	m.RES = 400 // 400/800 x 80% = 40% cut
	if d := Damage(Build{}, baseResult(), dmgCatalog(), m, Attack{}); d.Avg != 195 {
		t.Errorf("RES 400: %d, want 195", d.Avg)
	}
	m2 := target(0, 0, 0)
	m2.DamageTaken = 10
	if d := Damage(Build{}, baseResult(), dmgCatalog(), m2, Attack{}); d.Avg != 32 {
		t.Errorf("DamageTaken 10%%: %d, want 32", d.Avg)
	}
}

func TestHitAndCritChance(t *testing.T) {
	r := baseResult()
	r.HIT = 150
	r.st[LUK] = 35              // floor(35 x 0,3) = 10
	r.Bonuses["bCritical"] = 20 // + 20 = 30 crit
	m := target(0, 0, 10)       // flee = 10 + 1 + 100 = 111
	m.Stats[LUK] = 22           // floor(22 / 5) = 4
	d := Damage(Build{}, r, dmgCatalog(), m, Attack{})
	if d.HitChance != 39 {
		t.Errorf("hit = %v, want 39", d.HitChance)
	}
	if d.CritChance != 26 {
		t.Errorf("crit chance = %v, want 26", d.CritChance)
	}
	// A skill with half the crit (Cross Impact): floor(30 x 0,5) - 4 = 11;
	// left out, the share is all of it (30 - 4 = 26).
	half := &SkillFormula{Code: "X", CanCrit: func(FormulaInput) bool { return true }, CritRate: 0.5, CritDmg: 0.5,
		Ratio: func(FormulaInput) float64 { return 100 }}
	if d := Damage(Build{}, r, dmgCatalog(), m, Attack{SkillCode: "X", Formula: half}); d.CritChance != 11 {
		t.Errorf("half-crit skill: %v, want 11", d.CritChance)
	}
	half.CritRate, half.CritDmg = 0, 0
	if d := Damage(Build{}, r, dmgCatalog(), m, Attack{SkillCode: "X", Formula: half}); d.CritChance != 26 || d.Crit == 0 {
		t.Errorf("skill without crit share: %v (crit %d), want 26", d.CritChance, d.Crit)
	}
	// Per-race crit counts for basic attacks only.
	r.Extra["bCriticalAddRace RC_Brute"] = 5
	if d := Damage(Build{}, r, dmgCatalog(), m, Attack{}); d.CritChance != 31 {
		t.Errorf("race crit: %v, want 31", d.CritChance)
	}
	half.CritRate = 1
	if d := Damage(Build{}, r, dmgCatalog(), m, Attack{SkillCode: "X", Formula: half}); d.CritChance != 26 {
		t.Errorf("race crit on a skill: %v, want 26", d.CritChance)
	}
	// Katar: the character's crit doubles (60 - 4 = 56).
	delete(r.Extra, "bCriticalAddRace RC_Brute")
	r.WeaponType = "Katar"
	if d := Damage(Build{}, r, dmgCatalog(), m, Attack{}); d.CritChance != 56 {
		t.Errorf("katar crit: %v, want 56", d.CritChance)
	}
}

// Perfect hit is rolled before the flee check; katars double CRIT; skills
// marked Hit100 never miss; Olhos de Águia adds hidden hit.
func TestHitDetails(t *testing.T) {
	m := target(0, 0, 10) // flee 111
	c := dmgCatalog()
	r := baseResult()
	r.HIT = 150 // 39%
	r.PerfectHit = 10
	if d := Damage(Build{}, r, c, m, Attack{}); d.HitChance < 45.09 || d.HitChance > 45.11 {
		t.Errorf("perfect hit 10%% + 90%% x 39%% = 45.1%%, got %v", d.HitChance)
	}
	r.PerfectHit = 0
	f := &SkillFormula{Code: "X", Hit100: true, CritRate: 1, CritDmg: 1, Ratio: func(FormulaInput) float64 { return 100 }}
	if d := Damage(Build{}, r, c, m, Attack{SkillCode: "X", Formula: f}); d.HitChance != 100 {
		t.Errorf("Hit100 skill: %v", d.HitChance)
	}
	c.Skills = map[int]*gamedata.Skill{43: {ID: 43, Code: "AC_VULTURE"}}
	if d := Damage(Build{Skills: map[int]int{43: 10}}, r, c, m, Attack{}); d.HitChance != 49 {
		t.Errorf("Olhos de Águia 10: 39%% + 10 = 49%%, got %v", d.HitChance)
	}
}

// TestSkillWeaponRequirement: a skill the equipped weapon cannot use
// (rAthena skill_db Requires: Weapon) does no damage and says why.
func TestSkillWeaponRequirement(t *testing.T) {
	c := dmgCatalog()
	c.Skills = map[int]*gamedata.Skill{2553: {ID: 2553, Code: "RL_FIREDANCE", Weapons: []string{"Revolver"}}}
	r := baseResult()
	r.WeaponType = "Shotgun"
	d := Damage(Build{}, r, c, target(0, 0, 0), Attack{SkillCode: "RL_FIREDANCE", SkillRatio: 500})
	if d.Avg != 0 || len(d.Notes) == 0 || !strings.Contains(d.Notes[0], "Revólver") {
		t.Errorf("Descarregar Tambor with a shotgun: avg %d, notes %v", d.Avg, d.Notes)
	}
	r.WeaponType = "Revolver"
	if d := Damage(Build{}, r, c, target(0, 0, 0), Attack{SkillCode: "RL_FIREDANCE", SkillRatio: 500}); d.Avg == 0 {
		t.Errorf("Descarregar Tambor with a revolver: no damage")
	}
}
