package calc

import (
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

// Rules an Executor replay settled (Executor_JorDun01.rrf, 2026-11-22):
// with them, Impacto Brutal lands within 0,02% of every packet without EDP.

// Perícia com Katar Avançada multiplies the finished damage, with a katar
// only (it is not ATK %).
func TestKatarDamageRate(t *testing.T) {
	r := baseResult()
	r.Bonuses["bKatarDmgRate"] = 20
	if d := Damage(Build{}, r, dmgCatalog(), target(0, 0, 0), Attack{}); d.Avg != 325 {
		t.Errorf("dagger: avg = %d, want 325 (no change)", d.Avg)
	}
	r.WeaponType = "Katar"
	if d := Damage(Build{}, r, dmgCatalog(), target(0, 0, 0), Attack{}); d.Avg != 390 { // 325 × 1,2
		t.Errorf("katar: avg = %d, want 390", d.Avg)
	}
}

// readparam(bStr) is the allocated points, without the class bonus.
func TestReadparamAllocatedOnly(t *testing.T) {
	c := novice()
	c.Jobs[0].JobBonus[1] = [12]int{STR: 9}
	c.Items[2302] = &gamedata.Item{ID: 2302, Type: "Armor", Locations: []string{"Armor"}, Script: "bonus bAtk,readparam(bStr);"}
	b := Build{Job: 0, BaseLv: 1, JobLv: 1, Stats: [6]int{30, 1, 1, 1, 1, 1}, Equip: map[string]Equip{Armor: {Item: 2302}}}
	Normalize(&b, c)
	if got := Compute(b, c).Bonuses["bAtk"]; got != 30 {
		t.Errorf("readparam(bStr) = %d, want the 30 allocated", got)
	}
}
