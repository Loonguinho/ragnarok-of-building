package view

import (
	"strconv"
	"strings"

	"ragnarok-of-building/internal/gamedata"
)

// bonusText is a bonus as the windows show it: a readable label and its
// value. k is a bonus name ("bStr") or a bonus2..5 key with its arguments
// ("bAddRace RC_DemiHuman", "bSPLossRate 5"), v its value.
func bonusText(k string, v int, c *gamedata.Catalog, l string) (label, value string) {
	T := func(s string) string { return tr(l, s) }
	parts := strings.Fields(k)
	name := parts[0]
	args := parts[1:]
	arg := func(i int) string {
		if i < len(args) {
			return strings.Trim(args[i], `"`)
		}
		return ""
	}
	pct := func(x float64) string { return strconv.FormatFloat(x, 'f', -1, 64) + "%" }
	plus := func(x int) string {
		if x > 0 {
			return "+" + strconv.Itoa(x)
		}
		return strconv.Itoa(x)
	}
	plusPct := func(x int) string { return plus(x) + "%" }
	in := func(m map[string]string, s string) string {
		if n, ok := m[s]; ok {
			return T(n)
		}
		return s
	}
	class := func(s string) string { return in(classNames, s) }
	race2 := func(s string) string { return strings.ReplaceAll(strings.TrimPrefix(s, "RC2_"), "_", " ") }

	// Flags: the value is just "on".
	if f, ok := flagBonuses[name]; ok && len(args) == 0 {
		return T(f), ""
	}
	if len(args) == 0 {
		switch name {
		case "bHealPower":
			return T("Cura realizada"), plusPct(v)
		case "bHealPower2":
			return T("Cura recebida"), plusPct(v)
		case "bAddItemHealRate":
			return T("Cura por itens"), plusPct(v)
		case "bHPRegenRate":
			return T("Regeneração natural de HP"), plusPct(v)
		case "bSPRegenRate":
			return T("Regeneração natural de SP"), plusPct(v)
		case "bSplashRange":
			return T("Alcance do dano em área"), plus(v)
		case "bLongAtkDef":
			return T("Resistência a dano à distância"), plusPct(v)
		case "bMagicDamageReturn":
			return T("Reflete dano mágico"), pct(float64(v))
		case "bShortWeaponDamageReturn":
			return T("Reflete dano corpo a corpo"), pct(float64(v))
		case "bReduceDamageReturn":
			return T("Dano refletido recebido"), plusPct(-v)
		case "bCriticalLong":
			return T("Crítico à distância"), plus(v)
		case "bDef2Rate":
			return T("DEF suave"), plusPct(v)
		case "bSPGainValue":
			return T("SP ao derrotar (físico)"), plus(v)
		case "bHPGainValue":
			return T("HP ao derrotar (físico)"), plus(v)
		case "bMagicSPGainValue":
			return T("SP ao derrotar (mágico)"), plus(v)
		case "bMagicHPGainValue":
			return T("HP ao derrotar (mágico)"), plus(v)
		case "bLongSPGainValue":
			return T("SP ao derrotar (à distância)"), plus(v)
		case "bSPDrainValue":
			return T("SP por golpe"), plus(v)
		case "bBreakArmorRate":
			return T("Chance de quebrar a armadura do alvo"), pct(float64(v) / 100)
		case "bBreakWeaponRate":
			return T("Chance de quebrar a arma do alvo"), pct(float64(v) / 100)
		case "bAtkEle":
			return T("Elemento do ataque"), in(eleNames, eleByIndex(v))
		case "bDefEle":
			return T("Elemento de defesa"), in(eleNames, eleByIndex(v))
		}
		if lb, ok := bonusLabels[name]; ok {
			return T(lb), signed(v, k)
		}
		return k, signed(v, k)
	}
	secs := func(ms int) string { return strconv.FormatFloat(float64(ms)/1000, 'f', -1, 64) + " s" }
	signedSecs := func(ms int) string {
		if ms > 0 {
			return "+" + secs(ms)
		}
		return secs(ms)
	}
	switch name {
	case "bAtkEle":
		return T("Elemento do ataque") + ": " + in(eleNames, arg(0)), ""
	case "bDefEle":
		return T("Elemento de defesa") + ": " + in(eleNames, arg(0)), ""
	case "bVariableCastrate":
		return T("Conjuração variável de ") + skillName(c, arg(0), l), plusPct(v)
	case "bFixedCastrate":
		return T("Conjuração fixa de ") + skillName(c, arg(0), l), plusPct(v)
	case "bSkillCooldown":
		return T("Recarga de ") + skillName(c, arg(0), l), signedSecs(v)
	case "bSkillFixedCast":
		return T("Conjuração fixa de ") + skillName(c, arg(0), l), signedSecs(v)
	case "bSkillVariableCast":
		return T("Conjuração variável de ") + skillName(c, arg(0), l), signedSecs(v)
	case "bHPRegenRate", "bSPRegenRate":
		what := "HP"
		if name == "bSPRegenRate" {
			what = "SP"
		}
		return T("Recupera") + " " + arg(0) + " " + what + " " + T("a cada"), secs(v)
	case "bRegenPercentHP", "bRegenPercentSP":
		what := "HP"
		if name == "bRegenPercentSP" {
			what = "SP"
		}
		return T("Recupera") + " " + arg(0) + "% " + what + " " + T("a cada"), secs(v)
	case "bAddItemHealRate":
		return T("Cura de") + " " + itemName(c, arg(0), l), plusPct(v)
	case "bAddItemSPHealRate":
		return T("Recuperação de SP de") + " " + itemName(c, arg(0), l), plusPct(v)
	case "bAddItemGroupHealRate":
		return T("Cura de itens do grupo") + " " + strings.TrimPrefix(arg(0), "IG_"), plusPct(v)
	case "bExpAddClass":
		return T("EXP vs") + " " + class(arg(0)), plusPct(v)
	case "bComaClass":
		return T("Chance de coma vs") + " " + class(arg(0)), pct(float64(v) / 100)
	case "bMagicSubSize":
		return T("Resistência mágica a tamanho ") + in(sizeNames, arg(0)), plusPct(v)
	case "bSPDrainValueRace":
		return T("SP por golpe vs") + " " + in(raceNames, arg(0)), plus(v)
	case "bGetZenyNum":
		return T("Zeny ao derrotar"), plus(v)
	case "bHPLossRate", "bSPLossRate":
		what := "HP"
		if name == "bSPLossRate" {
			what = "SP"
		}
		return T("Perde") + " " + arg(0) + " " + what + " " + T("a cada"), secs(v)
	case "bHPDrainRate", "bSPDrainRate":
		what := T("Absorve HP do dano")
		if name == "bSPDrainRate" {
			what = T("Absorve SP do dano")
		}
		r, _ := strconv.Atoi(arg(0))
		return what + " (" + T("chance") + " " + pct(float64(r)/10) + ")", pct(float64(v))
	case "bAutoSpell", "bAutoSpellWhenHit":
		lb := T("Chance de conjurar") + " " + skillName(c, arg(0), l) + " " + T("Nv.") + " " + arg(1)
		if name == "bAutoSpellWhenHit" {
			lb += " " + T("ao ser atingido")
		}
		return lb, pct(float64(v) / 10)
	case "bAutoSpellOnSkill":
		return T("Ao usar") + " " + skillName(c, arg(0), l) + ", " + T("chance de conjurar") + " " + skillName(c, arg(1), l) + " " + T("Nv.") + " " + arg(2), pct(float64(v) / 10)
	case "bAddMonsterDropItem":
		return T("Chance de obter") + " " + itemName(c, arg(0), l), pct(float64(v) / 100)
	case "bAddMonsterIdDropItem":
		return T("Chance de obter") + " " + itemName(c, arg(0), l) + " " + T("de") + " " + monsterName(c, arg(1), l), pct(float64(v) / 100)
	case "bExpAddRace":
		return T("EXP vs") + " " + in(raceNames, arg(0)), plusPct(v)
	case "bDropAddRace":
		return T("Drop vs") + " " + in(raceNames, arg(0)), plusPct(v)
	case "bAddClass":
		return T("Dano físico vs ") + class(arg(0)), plusPct(v)
	case "bMagicAddClass":
		return T("Dano mágico vs ") + class(arg(0)), plusPct(v)
	case "bSubClass":
		return T("Resistência a ") + class(arg(0)), plusPct(v)
	case "bAddRace2":
		return T("Dano físico vs ") + race2(arg(0)), plusPct(v)
	case "bMagicAddRace2":
		return T("Dano mágico vs ") + race2(arg(0)), plusPct(v)
	case "bSubRace2":
		return T("Resistência a ") + race2(arg(0)), plusPct(v)
	case "bAddDamageClass":
		return T("Dano físico vs ") + monsterName(c, arg(0), l), plusPct(v)
	case "bAddMagicDamageClass":
		return T("Dano mágico vs ") + monsterName(c, arg(0), l), plusPct(v)
	case "bAddDefMonster":
		return T("Resistência a ") + monsterName(c, arg(0), l), plusPct(v)
	case "bCriticalAddRace":
		return T("Crítico vs") + " " + in(raceNames, arg(0)), plus(v)
	case "bIgnoreDefRace":
		return T("Ignora DEF de ") + in(raceNames, arg(0)), ""
	case "bIgnoreDefClass":
		return T("Ignora DEF de ") + class(arg(0)), ""
	case "bIgnoreDefClassRate":
		return T("Ignora DEF de ") + class(arg(0)), pct(float64(v))
	case "bIgnoreMdefClassRate":
		return T("Ignora DEFM de ") + class(arg(0)), pct(float64(v))
	case "bIgnoreResRaceRate":
		return T("Ignora RES de") + " " + in(raceNames, arg(0)), pct(float64(v))
	case "bIgnoreMResRaceRate", "bIgnoreMresRaceRate":
		return T("Ignora MRES de") + " " + in(raceNames, arg(0)), pct(float64(v))
	case "bSPGainRace":
		return T("SP ao derrotar") + " " + in(raceNames, arg(0)), plus(v)
	case "bComaRace":
		return T("Chance de coma vs") + " " + in(raceNames, arg(0)), pct(float64(v) / 100)
	case "bSkillUseSP":
		return T("Custo de SP de") + " " + skillName(c, arg(0), l), plus(-v)
	case "bSkillUseSPrate":
		return T("Custo de SP de") + " " + skillName(c, arg(0), l), plusPct(-v)
	case "bSubSkill":
		return T("Resistência a ") + skillName(c, arg(0), l), plusPct(v)
	case "bAddSkillBlow":
		return T("Repulsão de") + " " + skillName(c, arg(0), l), plus(v)
	case "bAddEffOnSkill":
		return T("Chance de ") + in(effNames, arg(1)) + " " + T("com") + " " + skillName(c, arg(0), l), pct(float64(v) / 100)
	case "bSubDefEle":
		return T("Resistência a monstros de") + " " + in(eleNames, arg(0)), plusPct(v)
	case "bWeaponDamageRate":
		return T("Dano com") + " " + strings.TrimPrefix(arg(0), "W_"), plusPct(v)
	}
	return extraLabel(k, c, l), signed(v, k)
}

// flagBonuses are on/off bonuses.
var flagBonuses = map[string]string{
	"bUnbreakableWeapon": "Arma inquebrável", "bUnbreakableHelm": "Capacete inquebrável",
	"bUnbreakableArmor": "Armadura inquebrável", "bUnbreakableGarment": "Capa inquebrável",
	"bUnbreakableShield": "Escudo inquebrável", "bUnbreakableShoes": "Sapatos inquebráveis",
	"bNoCastCancel": "Conjuração não é interrompida", "bNoKnockback": "Imune a repulsão",
	"bNoSizeFix": "Sem penalidade de tamanho", "bNoRegen": "Sem regeneração natural",
	"bIntravision": "Vê alvos ocultos", "bNoGemStone": "Habilidades sem gemas",
	"bNoWalkDelay": "Sem atraso ao ser atingido", "bRestartFullRecover": "Volta com HP e SP cheios ao reviver",
}

var classNames = map[string]string{
	"Class_Boss": "chefes", "Class_Normal": "monstros comuns", "Class_All": "todos os monstros", "Class_Guardian": "guardiões",
}

// eleByIndex is rAthena's element constant for its number.
func eleByIndex(i int) string {
	names := []string{"Ele_Neutral", "Ele_Water", "Ele_Earth", "Ele_Fire", "Ele_Wind", "Ele_Poison", "Ele_Holy", "Ele_Dark", "Ele_Ghost", "Ele_Undead"}
	if i >= 0 && i < len(names) {
		return names[i]
	}
	return strconv.Itoa(i)
}

// skillName is a skill by code ("MG_FIREBOLT") or ID.
func skillName(c *gamedata.Catalog, s string, l string) string {
	if id, err := strconv.Atoi(s); err == nil {
		if sk := c.Skills[id]; sk != nil {
			return sk.Name.Get(l)
		}
		return s
	}
	for _, sk := range c.Skills {
		if sk.Code == s {
			return sk.Name.Get(l)
		}
	}
	return s
}

func itemName(c *gamedata.Catalog, s string, l string) string {
	if id, err := strconv.Atoi(s); err == nil {
		if it := c.Items[id]; it != nil && it.Name.Get(l) != "" {
			return it.Name.Get(l)
		}
	}
	return s
}

func monsterName(c *gamedata.Catalog, s string, l string) string {
	if id, err := strconv.Atoi(s); err == nil {
		if m := c.Monster(id); m != nil {
			return m.NameIn(l)
		}
	}
	return s
}
