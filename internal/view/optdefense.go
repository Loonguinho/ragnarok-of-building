package view

import (
	"math"

	"ragnarok-of-building/internal/calc"
)

// Defense for the optimizer: the monster's ATK is not known, so the
// measure is effective HP against the damage window's target, physical or
// magic: Max HP over the share of a hit that gets through, with the same
// reductions the damage engine applies the other way (renewal; bROWiki:
// 500 hard DEF or 125 hard MDEF halve the damage):
//
//	physical: (4000+DEF)/(4000+10 DEF) × (1 − RES cut)
//	magic:    (1000+MDEF)/(1000+10 MDEF) × (1 − MRES cut)
//	RES/MRES cut = min(80%, 80% × R/(R+400))
//
// and the resistances against the target's race, size, element (its
// attacks take its element) and class, each kind summed and capped at
// 100%. Soft DEF/MDEF subtract a flat amount, which needs the monster's
// damage to weigh, so they are left out.

const (
	GoalDamage  = "dmg"
	GoalPhysDef = "pdef"
	GoalMagDef  = "mdef"
	GoalMaxHP   = "hp"    // Max HP
	GoalCRate   = "crate" // T.CRÍT (C.RATE)
)

// crateBase is added to C.RATE when it is scored: it is small, and 0 in many
// builds, and the rankings compare scores by their ratio.
const crateBase = 1000

// statGoal reports a goal that is a number of the status window: it needs no
// target, and gains read as that number.
func statGoal(goal string) bool { return goal == GoalMaxHP || goal == GoalCRate }

// goalValue is the number to show for a score.
func goalValue(goal string, score float64) float64 {
	if goal == GoalCRate {
		return score - crateBase
	}
	return score
}

// goalGain is a change in the score as text: "+12,5%", or the number itself
// for the status window's ("+3.200", "+4").
func goalGain(goal string, base, score float64, l string) string {
	if !statGoal(goal) {
		if base <= 0 {
			return ""
		}
		return pctChange(score/base, l)
	}
	d := int64(math.Round(score - base))
	if d < 0 {
		return fmtInt(d)
	}
	return "+" + fmtInt(d)
}

// effectiveHP is the build's effective HP against its target (physical or
// magic); 0 without a target.
func (s *Server) effectiveHP(b calc.Build, physical bool) float64 {
	c := s.opt.Catalog
	m := c.Monster(b.Target)
	if m == nil {
		return 0
	}
	r := calc.Compute(b, c)
	cut := func(x float64) float64 { return 1 - math.Min(0.8, 0.8*x/(x+400)) }
	pass := 1.0
	if physical {
		def := float64(r.HardDEF)
		pass *= (4000 + def) / (4000 + 10*def) * cut(float64(r.RES))
	} else {
		mdef := float64(r.HardMDEF)
		pass *= (1000 + mdef) / (1000 + 10*mdef) * cut(float64(r.MRES))
	}
	sum := func(name string, keys ...string) float64 {
		t := 0
		for _, k := range keys {
			t += r.Extra[name+" "+k]
		}
		return math.Min(100, float64(t))
	}
	pass *= 1 - sum("bSubRace", "RC_"+m.Race, "RC_All")/100
	pass *= 1 - sum("bSubSize", "Size_"+m.Size, "Size_All")/100
	pass *= 1 - sum("bSubEle", "Ele_"+m.Element, "Ele_All")/100
	pass *= 1 - sum("bSubClass", "Class_"+m.Class, "Class_All")/100
	pass *= 1 - sum("bSubDefEle", "Ele_"+m.Element, "Ele_All")/100
	if pass <= 0 {
		pass = 0.0001 // full immunity: still comparable
	}
	return float64(r.HP) / pass
}

// optScore is what the optimizer raises: expected damage, effective HP, Max
// HP or C.RATE.
func (s *Server) optScore(b calc.Build, goal string) float64 {
	switch goal {
	case GoalPhysDef:
		return s.effectiveHP(b, true)
	case GoalMagDef:
		return s.effectiveHP(b, false)
	case GoalMaxHP:
		return float64(calc.Compute(b, s.opt.Catalog).HP)
	case GoalCRate:
		return float64(crateBase + calc.Compute(b, s.opt.Catalog).CRATE)
	}
	return s.optExpected(b)
}
