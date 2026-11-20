package calc

import "math"

// Casting and status tolerances, from the final attributes (bROWiki
// Atributos; rAthena renewal skill_vfcastfix and status_get_sc_def).

// vctScale: DEX×2 + INT at which the stats alone remove variable casting.
const vctScale = 530

// CastInfo is how the build's casting compares to a skill's base times.
type CastInfo struct {
	StatPct   float64 // variable cast removed by DEX×2 + INT (0-100)
	ItemPct   int     // variable cast removed by items and buffs (bVariableCastrate, bCastrate)
	VarLeft   float64 // % of a skill's variable cast left (0 = instant)
	FixedRate int     // fixed cast change in % (negative = shorter)
	FixedMs   int     // fixed cast change in ms (negative = shorter)
	StatSum   int     // DEX×2 + INT
}

// Cast returns the build's casting.
func (r Result) Cast() CastInfo {
	st := r.st
	ci := CastInfo{StatSum: st[DEX]*2 + st[INT]}
	stat := math.Min(1, math.Sqrt(float64(max(0, ci.StatSum))/vctScale))
	ci.StatPct = stat * 100
	rate := r.Bonuses["bVariableCastrate"] + r.Bonuses["bCastrate"]
	ci.ItemPct = -rate
	left := (1 - stat) * (1 + float64(rate)/100)
	ci.VarLeft = math.Max(0, math.Min(1, left)) * 100
	ci.FixedRate = r.Bonuses["bFixedCastrate"]
	ci.FixedMs = r.Bonuses["bFixedCast"]
	return ci
}

// Tolerance is the build's resistance to one status (0-100%).
type Tolerance struct {
	Eff   string // rAthena effect (Eff_Stun...)
	Label string // Portuguese
	Stat  int    // attribute that gives it (STR..LUK), -1 = hard MDEF
	Base  int    // % from the attribute (or MDEF)
	Item  float64
	Pct   float64 // total, capped at 100
}

var toleranceDefs = []struct {
	eff, label string
	stat       int
}{
	{"Eff_Stun", "Atordoamento", VIT}, {"Eff_Poison", "Envenenamento", VIT}, {"Eff_DPoison", "Envenenamento mortal", VIT},
	{"Eff_Sleep", "Sono", AGI}, {"Eff_Bleeding", "Sangramento", AGI},
	{"Eff_Silence", "Silêncio", INT}, {"Eff_Blind", "Cegueira", INT},
	{"Eff_Confusion", "Caos", LUK}, {"Eff_Curse", "Maldição", LUK},
	{"Eff_Freeze", "Congelamento", -1}, {"Eff_Stone", "Petrificação", -1},
}

// Tolerances lists the build's status resistances: 1% per point of the
// attribute (VIT stun and poisons, AGI sleep and bleeding, INT silence and
// blindness, LUK chaos and curse; hard MDEF freezing and stone), plus the
// items' bResEff (in 1/100 %).
func (r Result) Tolerances() []Tolerance {
	out := make([]Tolerance, 0, len(toleranceDefs))
	for _, d := range toleranceDefs {
		t := Tolerance{Eff: d.eff, Label: d.label, Stat: d.stat}
		if d.stat >= 0 {
			t.Base = r.st[d.stat]
		} else {
			t.Base = r.HardMDEF
		}
		t.Item = float64(r.Extra["bResEff "+d.eff]) / 100
		t.Pct = math.Max(0, math.Min(100, float64(t.Base)+t.Item))
		out = append(out, t)
	}
	return out
}

// CritShield is the % of critical hits the build shrugs off (SOR ÷ 5).
func (r Result) CritShield() int { return r.st[LUK] / 5 }
