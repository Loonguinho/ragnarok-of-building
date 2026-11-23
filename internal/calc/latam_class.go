package calc

import (
	"math"
	"slices"
	"strconv"

	"ragnarok-of-building/internal/gamedata"
)

// Hand-written class rules of latam-ro-calc (MIT) that are code, not data,
// so the generators do not carry them over.

func init() {
	// Class toggles whose bonuses latam-ro-calc computes in code
	// (setAdditionalBonus); the client descriptions agree with the values.
	regClassBuffs("JT_KAGEROU", []ClassBuff{distortedCrescent})
	regClassBuffs("JT_OBORO", []ClassBuff{distortedCrescent})
	for _, job := range []string{"JT_ROYAL_GUARD", "JT_ROYAL_GUARD_H"} {
		regClassBuffs(job, []ClassBuff{
			{Name: "Prestige", Skill: "LG_PRESTIGE", Label: "Prestígio", Choices: levelChoices(5),
				Scripts: perLevelScript(5, func(lv int) string { return "bonus bDef," + strconv.Itoa(lv*15) + "*BaseLevel/100;" })},
			{Name: "Ride Peco", Label: "Montaria (Grifo)", Toggle: true, Choices: [][2]string{{"0", "—"}, {"1", "Sim"}}},
		})
	}
	for _, job := range []string{"JT_RUNE_KNIGHT", "JT_RUNE_KNIGHT_H"} {
		regClassBuffs(job, []ClassBuff{
			{Name: "Ride Dragon", Label: "Montaria (Dragão)", Toggle: true, Choices: [][2]string{{"0", "—"}, {"1", "Sim"}}},
		})
	}
	regClassBuffs("JT_SHADOW_CROSS", []ClassBuff{
		{Name: "Potent Venom", Skill: "SHC_POTENT_VENOM", Label: "Veneno Potente", Choices: levelChoices(10),
			Scripts: perLevelScript(10, func(lv int) string { return "bonus2 bIgnoreResRaceRate,RC_All," + strconv.Itoa(lv*2) + ";" })},
	})

	// Star Gladiator's Wrath (Fúria Solar/Lunar/Estelar), by level: the
	// target was aligned by Oposição Solar, Lunar e Estelar. Milagre Solar,
	// Lunar e Estelar (a Proteção proc) makes every target a Star one. They
	// only pay through the Star Emperor ATK adjustment below.
	wrathLv := [][2]string{{"0", "-"}, {"1", "Nv 1"}, {"2", "Nv 2"}, {"3", "Nv 3"}}
	regStates("JT_STAR_EMPEROR", []StateOption{
		{Key: stateKey("Wrath of Sun"), Label: "Fúria Solar", Choices: wrathLv},
		{Key: stateKey("Wrath of Moon"), Label: "Fúria Lunar", Choices: wrathLv},
		{Key: stateKey("Wrath of"), Label: "Fúria Estelar", Choices: wrathLv},
		{Key: stateKey("Miracle"), Label: "Milagre Solar, Lunar e Estelar", Choices: [][2]string{{"0", "-"}, {"1", "Sim"}}},
	})
}

// wrathByAlignment: Oposição Solar, Lunar e Estelar aligns a monster by
// size, so only one Wrath reaches it (StarEmperor.WRATH_BY_ALIGNMENT).
var wrathByAlignment = []struct {
	state, size, label string
	minHP              int
	withStr            bool
}{
	{"Wrath of Sun", "Small", "Fúria Solar", 0, false},
	{"Wrath of Moon", "Medium", "Fúria Lunar", 6000, false},
	{"Wrath of", "Large", "Fúria Estelar", 20000, true},
}

// inLine: the job is the class (either version, plain or _H) or descends from it.
func inLine(c *gamedata.Catalog, jobID int, code string) bool {
	return inChain(c, jobID, code) || inChain(c, jobID, code+"_H")
}

func inChain(c *gamedata.Catalog, jobID int, code string) bool {
	for _, id := range c.Chain(jobID) {
		if j := c.Jobs[id]; j != nil && j.Code == code {
			return true
		}
	}
	return false
}

// classFinalAtk is latam-ro-calc's modifyFinalAtk: the class's last ATK
// multipliers, applied to the ATK after P.ATK and mastery, before the
// damage modifiers. Only the Star Emperor line has one: Kihop (learned
// level, +15%/lv +10%) and the Wrath that matches the target's size,
// +(base level + LUK + DEX [+ STR for Star]) / (12 - 3 x Nv) % (bROWiki:
// ÷9, ÷6, ÷3). Under Milagre, Fúria Estelar reaches every target.
func classFinalAtk(b Build, r Result, c *gamedata.Catalog, m *gamedata.Monster, atk int64, step func(label, format string, args ...any)) int64 {
	if !inChain(c, b.Job, "JT_STAR_EMPEROR") {
		return atk
	}
	if lv := learnedLv(b, c, "TK_POWER"); lv > 0 {
		pct := int64(lv*15 + 10)
		atk += atk * pct / 100
		step("Kihop Nv "+strconv.Itoa(lv), "%+d%% = %d", pct, atk)
	}
	miracle := b.Atk.States[stateKey("Miracle")] > 0
	// Star first: under Milagre it wins over the size's own Wrath.
	for i := len(wrathByAlignment) - 1; i >= 0; i-- {
		w := wrathByAlignment[i]
		lv := min(b.Atk.States[stateKey(w.state)], 3)
		aligned := m.Size == w.size && m.HP >= w.minHP || miracle && w.withStr
		if lv <= 0 || !aligned {
			continue
		}
		sum := b.BaseLv + r.st[LUK] + r.st[DEX]
		if w.withStr {
			sum += r.st[STR]
		}
		bonus := sum / (12 - 3*lv)
		atk = int64(math.Floor(float64(atk) * float64(100+bonus) / 100))
		step(w.label+" Nv "+strconv.Itoa(lv), "%+d%% = %d", bonus, atk)
		break
	}
	return atk
}

// distortedCrescent (Lua Crescente Distorcida, Kagerou/Oboro): ATK and
// MATK +(base level / 3 + 100).
var distortedCrescent = ClassBuff{Name: "Distorted Crescent", Skill: "OB_ZANGETSU", Label: "Lua Crescente Distorcida",
	Toggle: true, Choices: [][2]string{{"0", "—"}, {"5", "Sim"}},
	Scripts: map[int]string{5: "bonus bBaseAtk,BaseLevel/3+100; bonus bMatk,BaseLevel/3+100;"}}

func perLevelScript(n int, f func(int) string) map[int]string {
	m := map[int]string{}
	for lv := 1; lv <= n; lv++ {
		m[lv] = f(lv)
	}
	return m
}

// passiveWeapons: passives read from client descriptions that only count
// with these weapons (the table does not say it, the text does).
var passiveWeapons = map[string][]string{
	"AG_TWOHANDSTAFF": {"2hStaff"}, // "Cajados de Duas Mãos darão S.ATQM"
}

// LatamPassives applies the class rules latam-ro-calc writes in code
// (setAdditionalBonus / getMasteryAtk; MIT), checked against the client
// descriptions. A passive whose description has a level table is applied
// from it (applyClassPassives), so the rules here skip it.
func LatamPassives(b Build, c *gamedata.Catalog, weapon string, bonus, extra map[string]int) {
	learn := func(code string) int {
		for id, lv := range b.Skills {
			if sk := c.Skills[id]; sk != nil && sk.Code == code {
				if sk.Effects != nil && sk.Effects.Kind == "passive" {
					return 0
				}
				return lv
			}
		}
		return 0
	}
	with := func(ws ...string) bool { return slices.Contains(ws, weapon) }
	on := func(name string) int { return b.Atk.States[stateKey(name)] }
	sizes := func(prefix string, s, m, l int) {
		extra[prefix+" Size_Small"] += s
		extra[prefix+" Size_Medium"] += m
		extra[prefix+" Size_Large"] += l
	}
	// War Book Mastery (Sky Emperor): with a book, P.ATK +(lv+2) and HIT +3/lv.
	if lv := learn("SKE_WAR_BOOK_MASTERY"); lv > 0 && weapon == "Book" {
		bonus["bPAtk"] += lv + 2
		bonus["bHit"] += lv * 3
	}
	// Versatilidade (Abyss Chaser): daggers and bows, damage by size.
	if lv := learn("ABC_DAGGER_AND_BOW_M"); lv > 0 && with("Dagger", "Bow") {
		sizes("bAddSize", lv, []int{0, 2, 3, 5, 6, 8, 9, 11, 12, 14, 15}[min(lv, 10)], lv*2)
	}
	// Magic Sword Mastery (Abyss Chaser): daggers and swords, magic damage on every size.
	if lv := learn("ABC_MAGIC_SWORD_M"); lv > 0 && with("Dagger", "1hSword") {
		extra["bMagicAddSize Size_All"] += int(math.Ceil(float64(lv) * 1.5))
	}
	// Mace & Book Mastery (Cardinal): damage by size.
	if lv := learn("CD_MACE_BOOK_M"); lv > 0 && with("Mace", "2hMace", "Book") {
		lv = min(lv, 10)
		sizes("bAddSize", lv, []int{0, 2, 3, 5, 6, 8, 9, 11, 12, 14, 15}[lv], []int{0, 3, 5, 7, 9, 10, 12, 13, 15, 16, 18}[lv])
	}
	// Magic Book Mastery (Elemental Master): water, wind, earth, fire and poison magic.
	if lv := learn("EM_MAGIC_BOOK_M"); lv > 0 && weapon == "Book" {
		for _, e := range []string{"Water", "Wind", "Earth", "Fire", "Poison"} {
			extra["bMagicAtkEle Ele_"+e] += lv
		}
	}
	// Senso das Sombras (Shadow Cross): FLEE +10/lv; CRIT with daggers (10+4/lv) and katars (5+2/lv).
	if lv := learn("SHC_SHADOW_SENSE"); lv > 0 {
		bonus["bFlee"] += lv * 10
		switch weapon {
		case "Dagger":
			bonus["bCritical"] += 10 + lv*4
		case "Katar":
			bonus["bCritical"] += 5 + lv*2
		}
	}
	// Presença de Palco (Troubadour/Trouvère): bows, whips and instruments.
	if lv := learn("TR_STAGE_MANNER"); lv > 0 && with("Bow", "Whip", "Musical") {
		bonus["bPAtk"] += lv * 3
		bonus["bSMatk"] += lv * 3
	}
	// Corrida (Taekwon line, latam-ro-calc's Taekwondo.getMasteryAtk): kick
	// damage +10/lv bare-handed, as mastery ATK.
	if lv := learnedLv(b, c, "TK_RUN"); lv > 0 && weapon == "Fist" && inLine(c, b.Job, "JT_TAEKWON") {
		bonus["bMasteryAtk"] += lv * 10
	}
	// Spear Mastery on a dragon (Rune Knight) or gryphon (Royal Guard): +1/lv
	// mounted; Dragon Training learned: +10/lv. A dragon also lifts the size
	// penalty of spears.
	if lv := learnedLv(b, c, "KN_SPEARMASTERY"); lv > 0 && with("1hSpear", "2hSpear") {
		if inLine(c, b.Job, "JT_RUNE_KNIGHT") {
			if learnedLv(b, c, "RK_DRAGONTRAINING") > 0 {
				bonus["bMasteryAtk"] += lv * 10
			}
			if on("Ride Dragon") > 0 {
				bonus["bMasteryAtk"] += lv
				bonus["bNoSizeFix"] = 1
			}
		}
		if inLine(c, b.Job, "JT_ROYAL_GUARD") && on("Ride Peco") > 0 {
			bonus["bMasteryAtk"] += lv
		}
	}
	// Potencializar Magia (Arch Mage, LATAM values): skill damage by level.
	if inLine(c, b.Job, "JT_ARCH_MAGE") {
		for _, sb := range map[int][]struct {
			code string
			pct  int
		}{
			1: {{"AG_VIOLENT_QUAKE", -50}},
			3: {{"AG_ALL_BLOOM", 300}, {"AG_VIOLENT_QUAKE", 200}, {"AG_DESTRUCTIVE_HURRICANE", 200}},
			5: {{"AG_DESTRUCTIVE_HURRICANE", 50}},
		}[on("Climax")] {
			extra["bSkillAtk "+sb.code] += sb.pct
		}
	}
	// Sorcerer's level 2 elemental spirits (when no Elemental Master spirit is out).
	if inLine(c, b.Job, "JT_SORCERER") && on("_ElementalMaster_spirit") == 0 {
		switch on("_Sorcerer_Elemental_Spirit") {
		case 12:
			bonus["bBaseAtk"] += 120
		case 22:
			bonus["bMatk"] += 80
		case 32:
			bonus["bFixedCast"] -= 1000
		case 42:
			bonus["bMaxHPrate"] += 10
		}
	}
}
