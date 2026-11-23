package calc

import (
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

func ammoItems(c *gamedata.Catalog) {
	c.Items = map[int]*gamedata.Item{
		1:   {ID: 1, Type: "Weapon", SubType: "Bow", Attack: 100, WeaponLv: 1, Locations: []string{"Both_Hand"}},
		2:   {ID: 2, Type: "Weapon", SubType: "Dagger", Attack: 100, WeaponLv: 1, Locations: []string{"Right_Hand"}},
		10:  {ID: 10, Type: "Ammo", SubType: "Arrow", Attack: 30, Locations: []string{"Ammo"}, Script: "bonus bAtkEle,Ele_Fire; bonus2 bAddRace,RC_Brute,10;"},
		300: {ID: 300, Type: "Card", Script: "bonus bStr,1;", EventScript: "bonus bShortAtkRate,10;"},
	}
}

// TestAmmo: arrows give a bow's attacks their ATK, element and bonuses, and
// nothing to a dagger.
func TestAmmo(t *testing.T) {
	c := dmgCatalog()
	ammoItems(c)
	c.Jobs = map[int]*gamedata.Job{4252: {ID: 4252, Parent: -1}}
	build := func(weapon int) Build {
		return Build{Job: 4252, BaseLv: 100, Stats: [6]int{1, 1, 1, 1, 1, 1},
			Equip: map[string]Equip{Weapon: {Item: weapon}, Ammo: {Item: 10}}}
	}
	bow := Compute(build(1), c)
	if !bow.AmmoFires || bow.AmmoATK != 30 || bow.AmmoEle != "Fire" || bow.Extra["bAddRace RC_Brute"] != 10 {
		t.Errorf("bow: fires %v atk %d ele %q extra %v", bow.AmmoFires, bow.AmmoATK, bow.AmmoEle, bow.Extra)
	}
	if bow.Extra["bAtkEle Ele_Fire"] != 0 {
		t.Error("the arrow's element leaked into the general bonuses")
	}
	dagger := Compute(build(2), c)
	if dagger.AmmoFires || dagger.Extra["bAddRace RC_Brute"] != 0 || usesAmmo(dagger, "") {
		t.Errorf("dagger takes the arrow: %+v", dagger.Extra)
	}
	m := target(0, 0, 0)
	d := Damage(Build{}, bow, c, m, Attack{})
	none := bow
	none.Ammo, none.AmmoFires, none.AmmoEle = nil, false, ""
	if d.Element != "Fire" || d.Avg <= Damage(Build{}, none, c, m, Attack{}).Avg {
		t.Errorf("bow damage: element %s, %d", d.Element, d.Avg)
	}
	if d := Damage(Build{}, bow, c, m, Attack{Element: "Neutral"}); d.Element != "Neutral" {
		t.Errorf("a forced element should win: %s", d.Element)
	}
	if !Fits(c.Items[10], Ammo) || Fits(c.Items[10], Weapon) || Fits(c.Items[1], Ammo) {
		t.Error("ammo slot rules")
	}
}

// TestEventEffect: an item's event bonus counts only when turned on.
func TestEventEffect(t *testing.T) {
	c := dmgCatalog()
	ammoItems(c)
	c.Jobs = map[int]*gamedata.Job{4252: {ID: 4252, Parent: -1}}
	b := Build{Job: 4252, BaseLv: 100, Equip: map[string]Equip{Weapon: {Item: 2, Cards: [4]int{300}}}}
	r := Compute(b, c)
	if len(r.ItemEffects) != 1 || !r.ItemEffects[0].Event || r.Bonuses["bShortAtkRate"] != 0 {
		t.Fatalf("off: %+v %v", r.ItemEffects, r.Bonuses)
	}
	b.Buffs = map[string]int{EventEffectKey(300): 1}
	if r := Compute(b, c); r.Bonuses["bShortAtkRate"] != 10 {
		t.Errorf("on: %v", r.Bonuses)
	}
}

// TestCannonball: a cannonball counts only for the skills that shoot it, as
// mastery ATK and with its element.
func TestCannonball(t *testing.T) {
	r := baseResult()
	r.Ammo = &gamedata.Item{Type: "Ammo", SubType: "Cannonball", Attack: 120}
	r.AmmoATK, r.AmmoEle = 120, "Fire"
	f := &SkillFormula{Melee: meleeNever, Ratio: func(FormulaInput) float64 { return 100 }}
	c, m := dmgCatalog(), target(0, 0, 0)
	with := Damage(Build{}, r, c, m, Attack{SkillCode: "GN_CARTCANNON", Formula: f})
	other := Damage(Build{}, r, c, m, Attack{SkillCode: "GN_CART_TORNADO", Formula: f})
	if with.Element != "Fire" || with.Avg != other.Avg+120 || other.Element != "Neutral" {
		t.Errorf("cannon %s %d, other skill %s %d", with.Element, with.Avg, other.Element, other.Avg)
	}
}

// TestCannonReplay: the gearless control of latam-ro-calc's
// gn-cart-cannon-gear-states.rrf (Bioquímico 158, INT 131): status ATK 167,
// Propulsão 5 (ATK 50) and a 120 ATK cannonball make 504; the ratio 3.178,96
// is floored; soft DEF 50 -> 15.967, fifteen times.
func TestCannonReplay(t *testing.T) {
	c := dmgCatalog()
	c.Skills = map[int]*gamedata.Skill{2475: {ID: 2475, Code: "GN_REMODELING_CART"}}
	r := Result{StatusATK: 167, OtherEquipATK: 50, WeaponType: "Fist", AmmoATK: 120,
		Ammo: &gamedata.Item{Type: "Ammo", SubType: "Cannonball"}}
	m := target(0, 0, 0)
	m.Level, m.Stats = 100, [6]int{}
	f := latamByJob["JT_GENETIC"]["GN_CARTCANNON"]
	in := FormulaInput{Lv: 5, BaseLv: 158, Learned: map[int]int{2475: 5}, Catalog: c, Monster: m}
	in.Stats[INT] = 131
	if got := f.Ratio(in); got < 3178.9 || got > 3179 {
		t.Fatalf("ratio %v, want 3178.96", got)
	}
	want := int64(504*3178/100) - int64(m.SoftDEF())
	if d := Damage(Build{}, r, c, m, Attack{SkillCode: "GN_CARTCANNON", Formula: f, Input: in}); d.Avg != want {
		t.Errorf("%d, want %d", d.Avg, want)
	}
}

// TestAttackPropertyStatus: an attack-property endow leaves the status ATK
// neutral; an ordinary one does not.
func TestAttackPropertyStatus(t *testing.T) {
	c, m := dmgCatalog(), target(0, 0, 0)
	m.Element = "Water" // Fire 50%, Neutral 100%
	c.AttrFix[1]["Neutral"]["Water"] = 100
	r := baseResult()
	r.StatusATK = 200
	plain := Damage(Build{}, r, c, m, Attack{Element: "Fire"}).Avg
	var b Build
	SetAttackProperty(&b, "Fire")
	if d := Damage(b, r, c, m, Attack{Element: "Fire"}); d.Avg != plain+200 {
		t.Errorf("attack property %d, endow %d", d.Avg, plain)
	}
}

// TestAttackPropertyNormalize: the endow's kind survives with its element
// and goes when another element is chosen.
func TestAttackPropertyNormalize(t *testing.T) {
	c := dmgCatalog()
	c.Jobs = map[int]*gamedata.Job{4252: {ID: 4252, Parent: -1}}
	b := Build{Job: 4252, BaseLv: 100}
	SetAttackProperty(&b, "Water")
	Normalize(&b, c)
	if !AttackPropertyOn(b, "Water") {
		t.Errorf("dropped: %v", b.Atk.States)
	}
	b.Atk.Element = "Fire"
	Normalize(&b, c)
	if len(b.Atk.States) != 0 {
		t.Errorf("kept with another element: %v", b.Atk.States)
	}
}
