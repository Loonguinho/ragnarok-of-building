package calc

import (
	"fmt"
	"math"
	"strings"

	"ragnarok-of-building/internal/gamedata"
)

// magicDamage computes a magic skill against a monster, following the LATAM
// magic pipeline of latam-ro-calc (MIT): (status MATK + weapon MATK with
// variance) + equipment MATK, then skill element, S.MATK, magic card
// modifiers by category (race, size, element, class, monster), MATK %, skill
// ratio, own-element bonus, MRES, hard MDEF (1000+MDEF)/(1000+10 MDEF), soft
// MDEF, skill bonus (bSkillAtk), the monster's damage taken and hits. Every
// stage is floored. Magic never misses and does not crit.
func magicDamage(b Build, r Result, c *gamedata.Catalog, m *gamedata.Monster, a Attack, f *SkillFormula, ratio float64, hits int) DamageResult {
	d := DamageResult{Ratio: ratio, Hits: hits, HitChance: 100}

	ele := f.ElementOf(a.Input)
	if ele == "" {
		ele = a.Element
	}
	if ele == "" && arrowMagic[a.SkillCode] && r.AmmoFires {
		ele = r.AmmoEle
	}
	if ele == "" {
		ele = "Neutral"
		for k, v := range r.Extra {
			if v != 0 && strings.HasPrefix(k, "bAtkEle Ele_") {
				ele = strings.TrimPrefix(k, "bAtkEle Ele_")
			}
		}
	}
	d.Element = ele
	elePct := 100
	if t, ok := c.AttrFix[m.ElementLv][ele][m.Element]; ok {
		elePct = max(0, t)
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
	race := sum("bMagicAddRace", "RC_"+m.Race, "RC_All")
	size := sum("bMagicAddSize", "Size_"+m.Size, "Size_All")
	eleMod := sum("bMagicAddEle", "Ele_"+m.Element, "Ele_All")
	class := sum("bMagicAddClass", "Class_"+m.Class, "Class_All")
	ownEle := sum("bMagicAtkEle", "Ele_"+ele, "Ele_All")
	skillAtk := 0
	if a.SkillCode != "" {
		skillAtk = sum("bSkillAtk", a.SkillCode)
	}

	mdef := float64(m.MDEF)
	ignore := min(100, sum("bIgnoreMdefRaceRate", "RC_"+m.Race, "RC_All")+sum("bIgnoreMdefClassRate", "Class_"+m.Class, "Class_All"))
	mdef -= mdef * float64(ignore) / 100
	if f.IgnoreDef {
		mdef = 0
	}
	hardMDEF := math.Round((1000+mdef)/(1000+mdef*10)*10000) / 10000
	softMDEF := int64(m.SoftMDEF())
	if f.IgnoreSoftDef {
		softMDEF = 0
	}
	mres := stripShadow(b, m.MRES)
	if ign := min(100, sum("bIgnoreMresRaceRate", "RC_"+m.Race, "RC_All")); ign > 0 {
		mres = math.Floor(mres - mres*float64(ign)/100)
	}
	mresCut := 1 - math.Min(0.8, 0.8*mres/(mres+400))

	// Weapon MATK: base + refine, variance 10% x weapon level.
	raw := float64(r.WeaponMATK + r.WeaponRefineMATK)
	variance := math.Round(0.1*float64(r.WeaponLv)*raw*100) / 100

	bn := r.Bonuses
	one := func(weaponMatk float64, rec bool) int64 {
		step := func(label, format string, args ...any) {
			if rec {
				d.Steps = append(d.Steps, Step{label, fmt.Sprintf(format, args...)})
			}
		}
		total := int64(math.Floor(float64(r.MATKMin)+weaponMatk)) + int64(r.OtherEquipMATK)
		step("ATQM", "status %d + arma %.0f + equip %d = %d", r.MATKMin, weaponMatk, r.OtherEquipMATK, total)
		mul := func(label string, pct float64) {
			if pct == 100 {
				return
			}
			total = int64(math.Floor(float64(total) * pct / 100))
			step(label, "× %.4g%% = %d", pct, total)
		}
		mul("Elemento ("+strings.ToLower(ele)+")", float64(elePct))
		mul("S.ATQM", float64(100+r.SMATK))
		mul("Raça (mágico)", float64(100+race))
		mul("Tamanho (mágico)", float64(100+size))
		mul("Elemento do alvo (mágico)", float64(100+eleMod))
		mul("Classe do alvo (mágico)", float64(100+class))
		mul("ATQM %", float64(100+bn["bMatkRate"]))
		mul("Habilidade", ratio)
		mul("Bônus do elemento próprio", float64(100+ownEle))
		if mres > 0 {
			total = int64(math.Floor(float64(total) * mresCut))
			step("MRES do alvo", "%.0f → × %.4f = %d", mres, mresCut, total)
		}
		total = int64(math.Floor(float64(total) * hardMDEF))
		total -= softMDEF
		if total < 1 {
			total = 1
		}
		step("DEFM do alvo", "× (1000+%.0f)/(1000+%.0f) − %d = %d", mdef, mdef*10, softMDEF, total)
		mul("Bônus na habilidade (equip)", float64(100+skillAtk))
		if f.Flat != nil {
			total += f.Flat(a.Input)
			step("Dano fixo da habilidade", "%+d = %d", f.Flat(a.Input), total)
		}
		if m.DamageTaken != 100 {
			total = max(1, total*int64(m.DamageTaken)/100)
			step("Dano recebido pelo alvo", "× %d%% = %d", m.DamageTaken, total)
		}
		if hits > 1 {
			step("Golpes", "%d × %d = %d", total, hits, total*int64(hits))
		}
		return total * int64(hits)
	}

	d.Min = one(raw-variance, false)
	d.Max = one(raw+variance, false)
	d.Avg = one(raw, true)
	d.Expected = float64(d.Avg)
	if d.Avg > 0 {
		d.HitsToKill = int((int64(m.HP) + d.Avg - 1) / d.Avg)
	}
	if elePct <= 0 {
		d.Notes = append(d.Notes, fmt.Sprintf("O elemento %s não causa dano em %s %d.", ele, m.Element, m.ElementLv))
	}
	return d
}

// arrowMagic: the spells that take the arrow's element.
var arrowMagic = map[string]bool{"TR_METALIC_FURY": true, "TR_SOUNDBLEND": true, "TR_RHYTHMICAL_WAVE": true}
