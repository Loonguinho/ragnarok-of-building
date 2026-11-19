package calc

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"ragnarok-of-building/internal/gamedata"
)

// Attack describes what is used against the target.
type Attack struct {
	SkillCode  string  // client skill code ("" = basic attack); used for bSkillAtk
	SkillRatio float64 // % of ATK for one use (100 = basic attack)
	Hits       int     // real damage multiplier (skills that hit N full times)
	Element    string  // "" = weapon element; otherwise an endow/forced element
	Formula    *SkillFormula
	Input      FormulaInput // used when Formula is set
}

// Step is one line of the damage breakdown.
type Step struct {
	Label string
	Value string
}

// DamageResult is the damage of one attack against one monster.
type DamageResult struct {
	Min, Avg, Max int64   // per use, normal (non-critical) hits
	Crit          int64   // critical hit, 0 if the attack cannot crit
	HitChance     float64 // % of non-critical attacks that land (perfect hit included)
	CritChance    float64
	Expected      float64 // average per use counting misses and crits
	HitsToKill    int     // uses to kill at the average normal damage
	Element       string
	Ratio         float64 // skill % actually used
	Hits          int
	Steps         []Step
	Notes         []string
}

// Damage computes a physical attack against a monster. The base pipeline is
// rAthena's renewal battle_calc_weapon_attack for players, with the LATAM
// adjustments measured by latam-ro-calc (MIT): card modifier stages floored
// one by one, status ATK taking the endow element (except poison), skill
// criticals with a fraction of the crit damage bonus, and bSkillAtk after DEF.
func Damage(b Build, r Result, c *gamedata.Catalog, m *gamedata.Monster, a Attack) DamageResult {
	var d DamageResult
	if m == nil {
		return d
	}
	f := a.Formula
	isSkill := a.SkillCode != ""
	ratio := a.SkillRatio
	hits := max(1, a.Hits)
	if f != nil {
		ratio = f.Ratio(a.Input)
		if f.Hits != nil {
			hits = max(1, f.Hits(a.Input))
		}
		if f.Partial != "" {
			d.Notes = append(d.Notes, "Parte desta habilidade ainda não é simulada ("+f.Partial+"): o dano real pode ser maior.")
		}
		if f.RequiresState != "" && !a.Input.Active(f.RequiresState) {
			d.Notes = append(d.Notes, "Esta habilidade exige o estado \""+f.RequiresState+"\" ativo.")
		}
		if len(f.Weapons) > 0 && !slices.Contains(f.Weapons, r.WeaponType) {
			d.Notes = append(d.Notes, "Esta habilidade exige arma do tipo "+strings.Join(f.Weapons, " ou ")+".")
		}
	}
	if ratio <= 0 {
		ratio = 100
	}
	d.Ratio, d.Hits = ratio, hits
	if f != nil && f.Magic {
		md := magicDamage(r, c, m, a, f, ratio, hits)
		md.Notes = append(d.Notes, md.Notes...)
		return md
	}

	// Crit rules: basic attacks crit; skills only when the formula allows it.
	canCrit, critRate, critDmgPart := !isSkill, 1.0, 1.0
	if f != nil {
		canCrit = f.ForceCrit || (f.CanCrit != nil && f.CanCrit(a.Input))
		critRate, critDmgPart = f.CritRate, f.CritDmg
	} else if isSkill {
		critDmgPart = 0.5
	}
	isMelee := !r.Ranged
	if f != nil && f.Melee != nil {
		isMelee = f.Melee(a.Input)
	}

	// Attack element: forced (endow), else the weapon's (bAtkEle), else Neutral.
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
	// Status ATK is neutral, except with an endow (a forced element), which
	// it takes too, unless the endow is poison (LATAM recordings).
	statusEle := "Neutral"
	if a.Element != "" && a.Element != "Poison" {
		statusEle = a.Element
	}
	// Negative values (healing) are treated as no damage.
	eleStatus, eleAtk := max(0, attr(statusEle)), max(0, attr(ele))
	sizePct := 100
	if row, ok := c.SizeFix[r.WeaponType]; ok {
		if v, ok := row[m.Size]; ok {
			sizePct = v
		}
	}

	sum := func(name string, args ...string) int {
		n := 0
		for k, v := range r.Extra {
			parts := strings.Fields(k)
			if len(parts) < 2 || parts[0] != name {
				continue
			}
			for _, want := range args {
				if strings.EqualFold(strings.Trim(parts[1], `"`), want) {
					n += v
					break
				}
			}
		}
		return n
	}
	race := sum("bAddRace", "RC_"+m.Race, "RC_All")
	eleMod := sum("bAddEle", "Ele_"+m.Element, "Ele_All")
	size := sum("bAddSize", "Size_"+m.Size, "Size_All")
	class := sum("bAddClass", "Class_"+m.Class, "Class_All")
	mob := sum("bAddDamageClass", fmt.Sprint(m.ID))
	skillAtk := 0
	if a.SkillCode != "" {
		skillAtk = sum("bSkillAtk", a.SkillCode)
	}
	// Card modifiers: each category multiplies and is floored on its own.
	cardfix := func(v int64) int64 {
		for _, p := range []int{race, size, eleMod, class, mob} {
			v = v * int64(100+p) / 100
		}
		return v
	}

	ignoreDef := min(100, sum("bIgnoreDefRaceRate", "RC_"+m.Race, "RC_All")+sum("bIgnoreDefClassRate", "Class_"+m.Class, "Class_All"))
	def1 := int64(m.DEF) - int64(m.DEF)*int64(ignoreDef)/100
	def2 := int64(m.SoftDEF()) - int64(m.SoftDEF())*int64(ignoreDef)/100
	if f != nil && f.IgnoreDef {
		def1 = 0
	}
	if f != nil && f.IgnoreSoftDef {
		def2 = 0
	}
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

	bn := r.Bonuses
	mastery := int64(bn["bMasteryAtk"] + sum("bMasteryAtkRace", "RC_"+m.Race, "RC_All") + sum("bMasteryAtkEle", "Ele_"+m.Element, "Ele_All"))
	one := func(weapon int64, crit, rec bool) int64 {
		step := func(label, format string, args ...any) {
			if rec {
				d.Steps = append(d.Steps, Step{label, fmt.Sprintf(format, args...)})
			}
		}
		status := int64(r.StatusATK) * int64(eleStatus) / 100 * 2
		step("ATQ de status", "%d × %d%% (%s) × 2 = %d", r.StatusATK, eleStatus, strings.ToLower(statusEle), status)

		w := weapon * int64(sizePct) / 100
		w = w * int64(eleAtk) / 100
		step("ATQ da arma", "%d × tamanho %d%% × elemento %d%% = %d", weapon, sizePct, eleAtk, w)

		eq := int64(r.OtherEquipATK) * int64(eleAtk) / 100
		step("ATQ de equipamento", "%d × elemento %d%% = %d", r.OtherEquipATK, eleAtk, eq)

		pct := (w + eq) * int64(bn["bAtkRate"]) / 100
		if pct != 0 {
			step("ATQ %", "(arma + equip) × %d%% = %d", bn["bAtkRate"], pct)
		}

		w2, eq2 := cardfix(w), cardfix(eq)
		if w2 != w || eq2 != eq {
			step("Modificadores de carta", "raça %+d%% · tamanho %+d%% · elemento %+d%% · classe %+d%%", race, size, eleMod, class)
		}

		dmg := status + w2 + eq2 + pct
		step("Soma", "%d + %d + %d + %d = %d", status, w2, eq2, pct, dmg)

		if r.PATK != 0 {
			dmg = int64(math.Floor(float64(dmg) * float64(100+r.PATK) / 100))
			step("P.ATQ", "× %d%% = %d", 100+r.PATK, dmg)
		}

		// Mastery ATK (class passives): added after the ATK multipliers, untouched
		// by size, element and card modifiers (latam-ro-calc's calcTotalAtk).
		if mastery != 0 {
			dmg += mastery
			step("ATQ de maestria", "%+d = %d", mastery, dmg)
		}

		if crit {
			if cr := float64(bn["bCritAtkRate"]) * critDmgPart; cr != 0 {
				dmg = int64(math.Floor(float64(dmg) * (100 + cr) / 100))
				step("Dano crítico (equip)", "%+.1f%% = %d", cr, dmg)
			}
		} else if nc := int64(bn["bNonCritAtkRate"]); nc != 0 {
			dmg += dmg * nc / 100
			step("Dano não crítico (equip)", "%+d%% = %d", nc, dmg)
		}
		rangeRate, rangeLabel := int64(bn["bShortAtkRate"]), "Dano corpo a corpo"
		if !isMelee {
			rangeRate, rangeLabel = int64(bn["bLongAtkRate"]), "Dano à distância"
		}
		if rangeRate != 0 {
			dmg = dmg * (100 + rangeRate) / 100
			step(rangeLabel, "%+d%% = %d", rangeRate, dmg)
		}

		if ratio != 100 {
			dmg = ratioFloor(dmg, ratio)
			step("Habilidade", "× %.0f%% = %d", ratio, dmg)
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
		if f != nil && (f.IgnoreDef || f.IgnoreSoftDef) {
			ign = " (habilidade ignora DEF)"
		}
		step("DEF do alvo", "%d × (4000+%d)/(4000+%d) − %d%s = %d", before, def1, 10*def1, def2, ign, dmg)

		if skillAtk != 0 {
			dmg = dmg * int64(100+skillAtk) / 100
			step("Bônus na habilidade (equip)", "%+d%% = %d", skillAtk, dmg)
		}

		if crit {
			dmg = int64(math.Floor(float64(float32(dmg) * (1.4 + 0.01*float32(r.CRATE)))))
			step("Crítico", "× %.2f = %d", 1.4+0.01*float64(r.CRATE), dmg)
		}

		if m.DamageTaken != 100 {
			dmg = max(1, dmg*int64(m.DamageTaken)/100)
			step("Dano recebido pelo alvo", "× %d%% = %d", m.DamageTaken, dmg)
		}
		if hits > 1 {
			step("Golpes", "%d × %d = %d", dmg, hits, dmg*int64(hits))
		}
		return dmg * int64(hits)
	}

	d.Min = one(int64(wmin), false, false)
	d.Max = one(int64(wmax), false, false)
	d.Avg = one(int64((wmin+wmax)/2), false, true)

	// Hit and critical chances (is_attack_hitting / is_attack_critical, renewal).
	d.HitChance = hitChance(b, r, c, m, f)
	if canCrit {
		d.Crit = one(int64(wmax), true, false) // criticals use max weapon variance
		// bCriticalAddRace adds whole CRIT points against a race.
		cri := r.CritTenths + 10*sum("bCriticalAddRace", "RC_"+m.Race, "RC_All") - m.Stats[LUK]*2
		d.CritChance = math.Max(0, float64(cri)/10) * critRate
		if f != nil && f.ForceCrit {
			d.CritChance = 100
		}
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
		d.Notes = append(d.Notes, "Efeitos de equipamento não simulados (procs, efeitos condicionais) não entram no dano.")
	}
	return d
}

// hitChance is the % of non-critical attacks that land (rAthena
// is_attack_hitting, renewal): HIT − the target's flee, plus the hidden
// hit of Olhos de Águia and Pesquisa de Armas, capped to 5..100; perfect
// hit is rolled first and skips the flee check. (Treino com Espadas' +3 per
// level comes as HIT from the class passives.)
func hitChance(b Build, r Result, c *gamedata.Catalog, m *gamedata.Monster, f *SkillFormula) float64 {
	if f != nil && f.Hit100 {
		return 100
	}
	mobFlee := m.Level + m.Stats[AGI] + 100
	rate := r.HIT - mobFlee + learnedLv(b, c, "AC_VULTURE")
	if lv := learnedLv(b, c, "BS_WEAPONRESEARCH"); lv > 0 {
		rate += rate * 2 * lv / 100
	}
	rolled := float64(clamp(rate, 5, 100))
	perfect := float64(clamp(r.PerfectHit, 0, 100))
	return perfect + (100-perfect)*rolled/100
}

// learnedLv is the level the build has in a skill (by client code).
func learnedLv(b Build, c *gamedata.Catalog, code string) int {
	for id, lv := range b.Skills {
		if sk := c.Skills[id]; sk != nil && sk.Code == code {
			return lv
		}
	}
	return 0
}
