package calc

import (
	"math"
	"testing"
)

// Variable cast: 1 − √((DEX×2 + INT) ÷ 530), then the item reductions.
func TestCast(t *testing.T) {
	var r Result
	r.Bonuses = map[string]int{}
	r.st[DEX], r.st[INT] = 100, 120 // 320 / 530
	ci := r.Cast()
	want := (1 - math.Sqrt(320.0/530)) * 100
	if ci.StatSum != 320 || math.Abs(ci.VarLeft-want) > 1e-9 {
		t.Errorf("stat only: sum %d, left %.3f, want 320 / %.3f", ci.StatSum, ci.VarLeft, want)
	}
	r.Bonuses["bVariableCastrate"] = -20
	if ci := r.Cast(); math.Abs(ci.VarLeft-want*0.8) > 1e-9 || ci.ItemPct != 20 {
		t.Errorf("with −20%%: left %.3f, want %.3f", ci.VarLeft, want*0.8)
	}
	r.st[DEX], r.st[INT] = 200, 130 // 530: instant
	if ci := r.Cast(); ci.VarLeft != 0 || ci.StatPct != 100 {
		t.Errorf("530: left %.3f, stat %.1f", ci.VarLeft, ci.StatPct)
	}
}

func TestTolerances(t *testing.T) {
	var r Result
	r.Extra = map[string]int{"bResEff Eff_Stun": 5000}
	r.st[VIT], r.st[LUK] = 80, 25
	r.HardMDEF = 150
	got := map[string]float64{}
	for _, x := range r.Tolerances() {
		got[x.Eff] = x.Pct
	}
	if got["Eff_Stun"] != 100 || got["Eff_Poison"] != 80 || got["Eff_Curse"] != 25 || got["Eff_Freeze"] != 100 {
		t.Errorf("tolerances = %v", got)
	}
	if r.CritShield() != 5 {
		t.Errorf("crit shield = %d, want 5", r.CritShield())
	}
}
