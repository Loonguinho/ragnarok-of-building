package calc

import (
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
	r.HIT, r.CritTenths = 150, 300 // 30% crit
	m := target(0, 0, 10)          // flee = 10 + 1 + 100 = 111
	m.Stats[LUK] = 20              // -4% crit
	d := Damage(Build{}, r, dmgCatalog(), m, Attack{})
	if d.HitChance != 39 {
		t.Errorf("hit = %d, want 39", d.HitChance)
	}
	if d.CritChance < 25.9 || d.CritChance > 26.1 {
		t.Errorf("crit chance = %v, want 26", d.CritChance)
	}
}
