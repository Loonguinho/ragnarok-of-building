package calc

import (
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

func TestBuffsChangeStats(t *testing.T) {
	c := &gamedata.Catalog{Jobs: map[int]*gamedata.Job{0: {ID: 0, MaxBaseLv: 99, MaxJobLv: 10, Parent: -1,
		BaseHP: make([]int, 99), BaseSP: make([]int, 99), JobBonus: make([][12]int, 11), BaseASPD: map[string]int{"Fist": 40}}}}
	b := Build{Job: 0, BaseLv: 50, JobLv: 1, Stats: [6]int{50, 50, 50, 50, 50, 50}}
	plain := Compute(b, c)
	b.Buffs = map[string]int{"blessing": 10, "incagi": 10, "gloria": 1, "concentration": 10, "food_str": 20, "nope": 3}
	Normalize(&b, c)
	if _, ok := b.Buffs["nope"]; ok {
		t.Error("unknown buff kept")
	}
	r := Compute(b, c)
	diff := func(i int) int { return r.FinalStats()[i] - plain.FinalStats()[i] }
	// STR: Bênção 10 + comida 20; AGI: Aumentar Agilidade 12 + Concentrar 12% of 50 = 6;
	// DEX: Bênção 10 + 6; LUK: Glória 30.
	if diff(STR) != 30 || diff(AGI) != 18 || diff(DEX) != 16 || diff(LUK) != 30 || diff(INT) != 10 {
		t.Errorf("stat changes STR %d AGI %d DEX %d INT %d LUK %d", diff(STR), diff(AGI), diff(DEX), diff(INT), diff(LUK))
	}
	if r.HIT <= plain.HIT+20 {
		t.Errorf("HIT %d -> %d: Bênção gives +20 plus DEX/LUK", plain.HIT, r.HIT)
	}
}
