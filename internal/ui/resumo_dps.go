package ui

import (
	"math"
	"strconv"
	"strings"

	"gioui.org/layout"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/view"
)

// seconds is "0,25s" (one decimal above 10 s).
func (a *App) seconds(ms int) string {
	digits := 2
	if ms >= 10000 {
		digits = 1
	}
	s := strconv.FormatFloat(float64(ms)/1000, 'f', digits, 64)
	if a.Lang != "en" {
		s = strings.Replace(s, ".", ",", 1)
	}
	return s + "s"
}

// decimal is a number with one decimal, in the language's style.
func (a *App) decimal(f float64) string {
	s := strconv.FormatFloat(f, 'f', 1, 64)
	if a.Lang != "en" {
		s = strings.Replace(s, ".", ",", 1)
	}
	return s
}

// limitLabel says what sets the attack's pace.
func (a *App) limitLabel(t calc.Timing) string {
	switch t.Limit {
	case "cast":
		return a.T("conjuração")
	case "after":
		return a.T("pós-conjuração")
	case "cooldown":
		return a.T("recarga")
	}
	return a.T("Vel.Atq")
}

// dpsRows are the Resumo's damage per second and the times behind it:
// "DPS 187.321.227 · 2,6 usos/s (limite: Vel.Atq)", then cast, after-cast
// delay, cooldown, the ASPD's wait and SP. With the equipment window's
// comparison, the DPS shows "now → compared".
func (a *App) dpsRows(d view.DamageView, expected int64, cmp bool, cd view.DamageView, cexp int64) []layout.Widget {
	th := a.Th
	t := d.Timing
	if t.CycleMs <= 0 || expected <= 0 {
		return nil
	}
	dps := int64(math.Round(float64(expected) * t.PerSec))
	head := []layout.Widget{T(th, a.T("DPS")).C(ColLabel).W(), B(th, view.FmtInt(dps)).W()}
	if cmp && cd.Timing.CycleMs > 0 {
		cdps := int64(math.Round(float64(cexp) * cd.Timing.PerSec))
		if cdps != dps {
			c := betterColor(float64(dps), float64(cdps), false)
			head = append(head, T(th, "→").C(ColMuted).W(), B(th, view.FmtInt(cdps)).C(c).W(),
				T(th, gainText(float64(dps), float64(cdps), a.Lang)).C(c).W())
		}
	}
	head = append(head, T(th, "· "+a.decimal(t.PerSec)+" "+a.T("usos/s")+" ("+a.T("limite")+": "+a.limitLabel(t)+")").C(ColMuted).W())
	var parts []string
	if d.Atk.Skill != 0 {
		parts = append(parts,
			a.T("Conj.")+" "+a.seconds(t.CastMs+t.FixedMs),
			a.T("Pós")+" "+a.seconds(t.AfterMs),
			a.T("Recarga")+" "+a.seconds(t.CooldownMs))
	}
	parts = append(parts, a.T("Vel.Atq")+" "+a.seconds(t.MotionMs))
	if t.SP > 0 {
		parts = append(parts, "SP "+strconv.Itoa(t.SP)+" ("+a.decimal(float64(t.SP)*t.PerSec)+"/s)")
	}
	return []layout.Widget{
		Row(4, head...),
		T(th, strings.Join(parts, " · ")).Sz(SmallText).C(ColMuted).Wrapping().W(),
	}
}
