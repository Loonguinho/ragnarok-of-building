package calc

import (
	"fmt"
	"math"
	"strings"

	"ragnarok-of-building/internal/gamedata"
)

// Attack describes what is used against the target.
type Attack struct {
	SkillRatio int    // % of ATK (100 = basic attack)
	Hits       int    // hits per use (skills); total damage = per-hit x hits
	Element    string // "" = weapon element; otherwise Neutral, Fire, ...
}

// Step is one line of the damage breakdown.
type Step struct {
	Label string
	Value string
}

// DamageResult is the damage of one attack against one monster.
type DamageResult struct {
	Min, Avg, Max int64 // per use, normal (non-critical) hits
	Crit          int64 // critical hit (basic attacks only), 0 if impossible
	HitChance     int   // % (5-100)
	CritChance    float64
	Expected      float64 // average per use counting misses and crits
	HitsToKill    int     // uses to kill at the average normal damage
	Element       string
	Steps         []Step
	Notes         []string
}

// Damage computes a physical attack against a monster, porting rAthena's
// renewal battle_calc_weapon_attack for players: status ATK x2 (neutral),
// weapon ATK with variance/STR bonus/refine/size/element, equipment ATK,
// ATK rate, card modifiers by category (race, element, size, class,
// monster), P.ATK, crit/short/long rates, skill ratio, RES, DEF, critical
// x(1.4 + C.RATE%) and the monster's DamageTaken.
func Damage(b Build, r Result, c *gamedata.Catalog, m *gamedata.Monster, a Attack) DamageResult {
	var d DamageResult
	if m == nil {
		return d
	}
	if a.SkillRatio <= 0 {
		a.SkillRatio = 100
	}
	if a.Hits <= 0 {
		a.Hits = 1
	}
	isSkill := a.SkillRatio != 100 || a.Hits != 1

	// Attack element: forced, else the weapon's (bAtkEle), else Neutral.
	ele := a.Element
	if ele == "" {
		ele = "Neutral"
		for k, v := range r.Extra {
			if v != 0 && strings.HasPrefix(k, "bAtkEle Ele_") {
				ele = strings.TrimPrefix(k, "bAtkEle Ele_")
			}
		}
	}
	d.Element = ele

	attr := func(atkEle string) int {
		if t, ok := c.AttrFix[m.ElementLv][atkEle][m.Element]; ok {
			return t
		}
		return 100
	}
	// Negative values (healing) are treated as no damage.
	eleNeutral, eleAtk := max(0, attr("Neutral")), max(0, attr(ele))
	sizePct := 100
	if row, ok := c.SizeFix[r.WeaponType]; ok {
		if v, ok := row[m.Size]; ok {
			sizePct = v
		}
	}

	// Card modifiers (battle_calc_cardfix, attacker side): each category
	// multiplies (100 + sum)%; the sums include the "All" variants.
	sum := func(name string, args ...string) int {
		n := 0
		for k, v := range r.Extra {
			parts := strings.Fields(k)
			if len(parts) < 2 || parts[0] != name {
				continue
			}
			for _, want := range args {
				if strings.EqualFold(parts[1], want) {
					n += v
					break
				}
			}
		}
		return n
	}
	race := sum("bAddRace", "RC_"+m.Race, "RC_All")
	eleMod := sum("bAddEle", "Ele_"+m.Element, "Ele_All") // vs the target's element
	size := sum("bAddSize", "Size_"+m.Size, "Size_All")
	class := sum("bAddClass", "Class_"+m.Class, "Class_All")
	mob := sum("bAddDamageClass", fmt.Sprint(m.ID))
	cardfix := int64(1000)
	for _, v := range []int{race, eleMod, size, class, mob} {
		cardfix = cardfix * int64(100+v) / 100
	}

	// Defense, with ignore-DEF and ignore-RES bonuses.
	ignoreDef := min(100, sum("bIgnoreDefRaceRate", "RC_"+m.Race, "RC_All")+sum("bIgnoreDefClassRate", "Class_"+m.Class, "Class_All"))
	def1 := int64(m.DEF) - int64(m.DEF)*int64(ignoreDef)/100
	def2 := int64(m.SoftDEF()) - int64(m.SoftDEF())*int64(ignoreDef)/100
	if def1 == -400 {
		def1 = -399
	}
	res := float64(m.RES)
	if ign := min(100, sum("bIgnoreResRaceRate", "RC_"+m.Race, "RC_All")); ign > 0 {
		res -= float64(ign) * res / 100
	}

	// Weapon ATK range (battle_calc_base_weapon_attack).
	watk := float64(r.WeaponATK + r.WeaponRefineATK)
	wmin, wmax := watk, watk
	if r.WeaponATK > 0 {
		baseStat := r.st[STR]
		if r.Ranged {
			baseStat = r.st[DEX]
		}
		variance := 5.0 * float64(r.WeaponATK) * float64(r.WeaponLv) / 100
		statBonus := float64(r.WeaponATK) * float64(baseStat) / 200
		wmin = math.Max(0, math.Trunc(watk-variance+statBonus))
		wmax = math.Trunc(watk + variance + statBonus)
	}

	b2 := r.Bonuses
	// one runs the pipeline for a given base weapon damage; steps are recorded
	// when rec is true.
	one := func(weapon int64, crit, rec bool) int64 {
		step := func(label, format string, args ...any) {
			if rec {
				d.Steps = append(d.Steps, Step{label, fmt.Sprintf(format, args...)})
			}
		}
		status := int64(r.StatusATK) * int64(eleNeutral) / 100 * 2
		step("ATQ de status", "%d × %d%% (neutro) × 2 = %d", r.StatusATK, eleNeutral, status)

		w := weapon * int64(sizePct) / 100
		w = w * int64(eleAtk) / 100
		step("ATQ da arma", "%d × tamanho %d%% × elemento %d%% = %d", weapon, sizePct, eleAtk, w)

		eq := int64(r.OtherEquipATK) * int64(eleAtk) / 100
		step("ATQ de equipamento", "%d × elemento %d%% = %d", r.OtherEquipATK, eleAtk, eq)

		pct := (w + eq) * int64(b2["bAtkRate"]) / 100
		if pct != 0 {
			step("ATQ %", "(arma + equip) × %d%% = %d", b2["bAtkRate"], pct)
		}

		w2, eq2 := w*cardfix/1000, eq*cardfix/1000
		if cardfix != 1000 {
			step("Modificadores de carta", "raça %+d%% · elemento %+d%% · tamanho %+d%% · classe %+d%% → ×%.3f",
				race, eleMod, size, class, float64(cardfix)/1000)
		}

		dmg := status + w2 + eq2 + pct
		step("Soma", "%d + %d + %d + %d = %d", status, w2, eq2, pct, dmg)

		if r.PATK != 0 {
			dmg = int64(math.Floor(float64(dmg) * float64(100+r.PATK) / 100))
			step("P.ATQ", "× %d%% = %d", 100+r.PATK, dmg)
		}

		if crit {
			div := int64(100)
			if isSkill {
				div = 200
			}
			if cr := int64(b2["bCritAtkRate"]); cr != 0 {
				dmg += int64(math.Floor(float64(dmg * cr / div)))
				step("Dano crítico (bônus)", "%+d%% = %d", cr*100/div, dmg)
			}
		} else if nc := int64(b2["bNonCritAtkRate"]); nc != 0 {
			dmg += dmg * nc / 100
			step("Dano não crítico (bônus)", "%+d%% = %d", nc, dmg)
		}
		rangeRate, rangeLabel := int64(b2["bShortAtkRate"]), "Dano corpo a corpo"
		if r.Ranged {
			rangeRate, rangeLabel = int64(b2["bLongAtkRate"]), "Dano à distância"
		}
		if rangeRate != 0 {
			dmg += dmg * rangeRate / 100
			step(rangeLabel, "%+d%% = %d", rangeRate, dmg)
		}

		if a.SkillRatio != 100 {
			dmg = dmg * int64(a.SkillRatio) / 100
			step("Habilidade", "× %d%% = %d", a.SkillRatio, dmg)
		}

		if res > 0 && dmg > 0 {
			cut := int64(float32(res/(res+400)) * 80.0 / 100.0 * float32(dmg))
			dmg -= cut
			step("RES do alvo", "%.0f → −%d = %d", res, cut, dmg)
		}

		before := dmg
		dmg = dmg*(4000+def1)/(4000+10*def1) - def2
		if dmg < 1 {
			dmg = 1
		}
		ign := ""
		if ignoreDef > 0 {
			ign = fmt.Sprintf(" (ignora %d%%)", ignoreDef)
		}
		step("DEF do alvo", "%d × (4000+%d)/(4000+%d) − %d%s = %d", before, def1, 10*def1, def2, ign, dmg)

		if crit {
			dmg = int64(math.Floor(float64(float32(dmg) * (1.4 + 0.01*float32(r.CRATE)))))
			step("Crítico", "× %.2f = %d", 1.4+0.01*float64(r.CRATE), dmg)
		}

		if m.DamageTaken != 100 {
			dmg = max(1, dmg*int64(m.DamageTaken)/100)
			step("Dano recebido pelo alvo", "× %d%% = %d", m.DamageTaken, dmg)
		}
		if a.Hits > 1 {
			step("Golpes", "%d × %d = %d", dmg, a.Hits, dmg*int64(a.Hits))
		}
		return dmg * int64(a.Hits)
	}

	d.Min = one(int64(wmin), false, false)
	d.Max = one(int64(wmax), false, false)
	d.Avg = one(int64((wmin+wmax)/2), false, true)

	// Hit and critical chances (is_attack_hitting / is_attack_critical, renewal).
	mobFlee := m.Level + m.Stats[AGI] + 100
	d.HitChance = clamp(r.HIT-mobFlee, 5, 100)
	if !isSkill {
		d.Crit = one(int64(wmax), true, false) // criticals use max weapon variance
		d.CritChance = math.Max(0, float64(r.CritTenths-m.Stats[LUK]*2)/10)
		d.CritChance = math.Min(100, d.CritChance)
	}
	cc := d.CritChance / 100
	d.Expected = cc*float64(d.Crit) + (1-cc)*float64(d.HitChance)/100*float64(d.Avg)
	if d.Avg > 0 {
		d.HitsToKill = int((int64(m.HP) + d.Avg - 1) / d.Avg)
	}

	if eleAtk <= 0 {
		d.Notes = append(d.Notes, fmt.Sprintf("O elemento %s não causa dano em %s %d.", ele, m.Element, m.ElementLv))
	}
	if r.SkippedScriptEffects > 0 {
		d.Notes = append(d.Notes, "Efeitos de equipamento não simulados (procs, combos) não entram no dano.")
	}
	return d
}
