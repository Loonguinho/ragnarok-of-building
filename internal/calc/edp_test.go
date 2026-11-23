package calc

import "testing"

// EDP: weapon ATK +15% (fitted on replays) whatever the target's element, weapon + equip ×4
// after the card modifiers; status ATK untouched; Meteor Assault left out.
func TestDamageEDP(t *testing.T) {
	c := dmgCatalog()
	c.AttrFix[1]["Poison"] = map[string]int{"Neutral": 100, "Water": 100}
	b := Build{Atk: AttackChoice{States: map[string]int{stateKey(edpState): 1}}}
	// avg weapon 125 → 143 (×1.15), ×4 = 572; status 200 → 772.
	if d := Damage(b, baseResult(), c, target(0, 0, 0), Attack{}); d.Avg != 772 {
		t.Errorf("EDP avg = %d, want 772", d.Avg)
	}
	// Poison doing 0% to the target changes nothing (Executor replay on a
	// Poison 3 Rgan).
	c.AttrFix[1]["Poison"]["Neutral"] = 0
	if d := Damage(b, baseResult(), c, target(0, 0, 0), Attack{}); d.Avg != 772 {
		t.Errorf("EDP vs a target poison does not hurt: avg = %d, want 772", d.Avg)
	}
	if d := Damage(b, baseResult(), c, target(0, 0, 0), Attack{SkillCode: "ASC_METEORASSAULT"}); d.Avg != 325 {
		t.Errorf("Meteor Assault with EDP: avg = %d, want 325", d.Avg)
	}
}

// Profanação: +3% per stack on melee physical damage, up to 20 stacks.
func TestDamageShadowScar(t *testing.T) {
	b := Build{Atk: AttackChoice{States: map[string]int{stateKey(shadowScarState): 10}}}
	if d := Damage(b, baseResult(), dmgCatalog(), target(0, 0, 0), Attack{}); d.Avg != 422 { // 325 × 1.30
		t.Errorf("10 stacks: avg = %d, want 422", d.Avg)
	}
	r := baseResult()
	r.Ranged = true
	if d := Damage(b, r, dmgCatalog(), target(0, 0, 0), Attack{}); d.Avg == 422 {
		t.Error("Profanação raised a ranged attack")
	}
}
