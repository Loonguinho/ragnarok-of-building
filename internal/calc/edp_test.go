package calc

import "testing"

// EDP: weapon ATK +25% × poison vs the target, weapon + equip ×4 after the
// card modifiers; status ATK untouched; Meteor Assault left out.
func TestDamageEDP(t *testing.T) {
	c := dmgCatalog()
	c.AttrFix[1]["Poison"] = map[string]int{"Neutral": 100, "Water": 100}
	b := Build{Atk: AttackChoice{States: map[string]int{stateKey(edpState): 1}}}
	// avg weapon 125 → 156 (×1.25), ×4 = 624; status 200 → 824.
	if d := Damage(b, baseResult(), c, target(0, 0, 0), Attack{}); d.Avg != 824 {
		t.Errorf("EDP avg = %d, want 824", d.Avg)
	}
	// Poison 50% against the target: 125 × 1.125 = 140, ×4 = 560 → 760.
	c.AttrFix[1]["Poison"]["Neutral"] = 50
	if d := Damage(b, baseResult(), c, target(0, 0, 0), Attack{}); d.Avg != 760 {
		t.Errorf("EDP vs poison 50%%: avg = %d, want 760", d.Avg)
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
