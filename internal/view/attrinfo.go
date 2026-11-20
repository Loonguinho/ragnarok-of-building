package view

import (
	"fmt"
	"strings"

	"ragnarok-of-building/internal/calc"
)

// AttrLine is one effect of an attribute: what it raises and by how much.
type AttrLine struct{ Label, Value string }

func plus(n int) string { return fmt.Sprintf("+%d", n) }

func pctS(v float64) string {
	return strings.Replace(strings.TrimSuffix(strings.TrimSuffix(fmt.Sprintf("+%.1f", v), "0"), "."), ".", ",", 1) + "%"
}

// AttrEffects is what the final value of attribute i (STR..LUK, or POW..CRT
// with trait) gives the build, with the same formulas as the status window
// (bROWiki Atributos; rAthena renewal).
func AttrEffects(r calc.Result, i int, trait bool, l string) []AttrLine {
	var out []AttrLine
	add := func(label, value string) { out = append(out, AttrLine{tr(l, label), value}) }
	tol := func(names ...string) {
		for _, t := range r.Tolerances() {
			for _, n := range names {
				if t.Eff == n {
					add("Tolerância a "+strings.ToLower(t.Label), pctS(float64(t.Base)))
				}
			}
		}
	}
	if trait {
		t := r.FinalTraits()[i]
		switch i {
		case calc.POW:
			add("ATQ de status", plus(5*t))
			add("P.ATQ", plus(t/3))
		case calc.STA:
			add("RES", plus(t+t/3*5))
		case calc.WIS:
			add("MRES", plus(t+t/3*5))
		case calc.SPL:
			add("ATQM de status", plus(5*t))
			add("S.ATQM", plus(t/3))
		case calc.CON:
			add("Precisão", plus(2*t))
			add("Esquiva", plus(2*t))
			add("P.ATQ", plus(t/5))
			add("S.ATQM", plus(t/5))
		case calc.CRT:
			add("H.PLUS", plus(t))
			add("C.RATE", plus(t/3))
		}
		return out
	}
	s := r.FinalStats()[i]
	switch i {
	case calc.STR:
		if r.Ranged {
			add("ATQ de status", plus(s/5))
		} else {
			add("ATQ de status", plus(s))
			add("ATQ da arma", pctS(float64(s)/2))
		}
		add("Peso máximo", plus(30*s))
	case calc.AGI:
		add("Esquiva", plus(s))
		add("DEF leve", plus(s/5))
		add("Velocidade de ataque", tr(l, "aumenta"))
		tol("Eff_Sleep", "Eff_Bleeding")
	case calc.VIT:
		add("HP máx.", pctS(float64(s)))
		add("DEF leve", plus(s/2))
		add("DEFM leve", plus(s/5))
		add("Cura de itens de HP", pctS(float64(2*s)))
		tol("Eff_Stun", "Eff_Poison", "Eff_DPoison")
	case calc.INT:
		add("ATQM de status", plus(s+s/2))
		add("SP máx.", pctS(float64(s)))
		add("DEFM leve", plus(s))
		add("Cura de itens de SP", pctS(float64(s)))
		add("Conjuração variável", tr(l, "INT soma 1× em DES×2 + INT"))
		tol("Eff_Silence", "Eff_Blind")
	case calc.DEX:
		if r.Ranged {
			add("ATQ de status", plus(s))
			add("ATQ da arma", pctS(float64(s)/2))
		} else {
			add("ATQ de status", plus(s/5))
		}
		add("Precisão", plus(s))
		add("ATQM de status", plus(s/5))
		add("DEFM leve", plus(s/5))
		add("Velocidade de ataque", tr(l, "aumenta"))
		add("Conjuração variável", tr(l, "DES soma 2× em DES×2 + INT"))
	case calc.LUK:
		add("ATQ de status", plus(s/3))
		add("ATQM de status", plus(s/3))
		add("Precisão", plus(s/3))
		add("Esquiva", plus(s/5))
		add("Crítico", strings.Replace(fmt.Sprintf("+%.1f", float64(s)*0.3), ".", ",", 1))
		add("Esquiva perfeita", plus(s/10))
		add("Precisão perfeita", pctS(float64(s/10)))
		add("Tolerância a críticos", pctS(float64(r.CritShield())))
		tol("Eff_Confusion", "Eff_Curse")
	}
	return out
}

// CastLines explains the casting step by step: the cut from DEX×2 + INT,
// the cut from items and buffs (the two multiply), the total cut (what the
// status window shows) and what a 2 s variable cast becomes.
func CastLines(r calc.Result, l string) []AttrLine {
	ci := r.Cast()
	comma := func(s string) string { return strings.Replace(s, ".", ",", 1) }
	cut := func(p float64) string {
		if p <= 0.05 {
			return "0%"
		}
		return comma(fmt.Sprintf("−%.1f%%", p))
	}
	items := tr(l, "nenhuma")
	if ci.ItemPct != 0 {
		items = fmt.Sprintf("−%d%%", ci.ItemPct)
	}
	total := 100 - ci.VarLeft
	example := tr(l, "instantânea")
	if ci.VarLeft > 0.05 {
		example = comma(fmt.Sprintf("%.2f s", 2*ci.VarLeft/100))
	}
	out := []AttrLine{
		{fmt.Sprintf("%s (%d/530)", tr(l, "Pelos atributos"), ci.StatSum), cut(ci.StatPct)},
		{tr(l, "Por itens e buffs"), items},
		{tr(l, "Redução total"), cut(total)},
		{tr(l, "Ex.: 2 s de conjuração"), "→ " + example},
	}
	if ci.FixedRate != 0 {
		out = append(out, AttrLine{tr(l, "Conjuração fixa"), fmt.Sprintf("%+d%%", ci.FixedRate)})
	}
	if ci.FixedMs != 0 {
		out = append(out, AttrLine{tr(l, "Conjuração fixa"), comma(fmt.Sprintf("%+.1f s", float64(ci.FixedMs)/1000))})
	}
	return out
}

// ToleranceLines lists the status tolerances (attribute + items).
func ToleranceLines(r calc.Result, l string) []AttrLine {
	var out []AttrLine
	for _, t := range r.Tolerances() {
		v := strings.Replace(fmt.Sprintf("%.0f%%", t.Pct), ".", ",", 1)
		if t.Item != 0 {
			v += " " + strings.Replace(fmt.Sprintf("(%+.0f%% %s)", t.Item, tr(l, "itens")), ".", ",", 1)
		}
		out = append(out, AttrLine{tr(l, t.Label), v})
	}
	out = append(out, AttrLine{tr(l, "Críticos"), fmt.Sprintf("%d%%", r.CritShield())})
	return out
}
