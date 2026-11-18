package clientimport

import (
	"slices"
	"strings"

	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/rathena"
)

// MergeRathena adds rAthena mechanics (job tables, status points, equipment
// stats) to a catalog built from the client. Client text always wins.
func MergeRathena(c *gamedata.Catalog, db *rathena.DB) (jobs, items int) {
	c.StatPoints = db.StatPoints
	c.TraitPoints = db.TraitPoints
	c.Refine = db.Refine
	c.AttrFix, c.SizeFix = db.AttrFix, db.SizeFix
	c.Monsters = map[int]*gamedata.Monster{}
	for id, m := range db.Mobs {
		if m.HP <= 0 || m.Name == "" {
			continue
		}
		c.Monsters[id] = &gamedata.Monster{
			ID: id, Name: m.Name, Level: m.Level, HP: m.HP, DEF: m.Defense, MDEF: m.MDefense,
			RES: m.Res, MRES: m.MRes, Stats: m.Stats, Size: m.Size, Race: m.Race,
			Element: m.Element, ElementLv: m.ElementLv, Class: m.Class, MVP: m.MVP,
			DamageTaken: m.DamageTaken, Modes: m.Modes,
		}
		c.Monsters[id].Names = monsterNames(c, m.Cards)
	}
	// Monsters rAthena lists without stats: searchable, stats on demand.
	c.MonsterStubs = map[int]string{}
	for id, aegis := range db.MobStubs {
		c.MonsterStubs[id] = aegis
	}
	for id, m := range db.Mobs {
		if c.Monsters[id] == nil && m.Aegis != "" {
			c.MonsterStubs[id] = m.Aegis
		}
	}
	if db.Commit != "" {
		c.Source += " + rAthena " + db.Commit[:min(8, len(db.Commit))]
	}

	for id, j := range c.Jobs {
		rj := db.Jobs[id]
		if rj == nil {
			continue
		}
		jobs++
		j.MaxBaseLv, j.MaxJobLv, j.MaxWeight = rj.MaxBaseLv, rj.MaxJobLv, rj.MaxWeight
		j.BaseHP = make([]int, rj.MaxBaseLv)
		j.BaseSP = make([]int, rj.MaxBaseLv)
		j.BaseAP = make([]int, rj.MaxBaseLv)
		for lv := 1; lv <= rj.MaxBaseLv; lv++ {
			j.BaseHP[lv-1] = rj.BaseHPAt(lv)
			j.BaseSP[lv-1] = rj.BaseSPAt(lv)
			if lv <= len(rj.BaseAP) {
				j.BaseAP[lv-1] = rj.BaseAP[lv-1]
			}
		}
		j.JobBonus = make([][12]int, rj.MaxJobLv+1)
		for lv := 1; lv <= rj.MaxJobLv; lv++ {
			j.JobBonus[lv] = j.JobBonus[lv-1]
			for i, v := range rj.BonusStats[lv] {
				j.JobBonus[lv][i] += v
			}
		}
		j.BaseASPD = rj.BaseASPD
		j.Upper = isUpper(j)
		j.Fourth = (id >= 4252 && id <= 4279) || (id >= 4302 && id <= 4330)
		j.Baby = strings.Contains(j.Code, "_B") || strings.HasPrefix(j.Code, "JT_BABY")
	}

	for _, j := range c.Jobs {
		j.EquipGroup, j.EquipTier = equipClass(c, j)
	}

	for id, ri := range db.Items {
		it := c.Items[id]
		if it == nil {
			continue // not in the LATAM client
		}
		items++
		it.Type, it.SubType, it.Locations = canonicalType(ri.Type), ri.SubType, ri.Locations
		it.Attack, it.MagicAttack, it.Defense = ri.Attack, ri.MagicAttack, ri.Defense
		it.WeaponLv, it.ArmorLv = ri.WeaponLevel, ri.ArmorLevel
		it.EquipLvMin, it.EquipLvMax = ri.EquipLvMin, ri.EquipLvMax
		it.Refineable, it.Script = ri.Refineable, ri.Script
		it.EquipJobs, it.EquipClasses = ri.Jobs, ri.Classes
		it.Weight = ri.Weight
		if it.Slots == 0 {
			it.Slots = ri.Slots
		}
	}
	for _, cb := range db.Combos {
		ok := true
		for _, id := range cb.Items {
			if c.Items[id] == nil {
				ok = false // an item the LATAM client doesn't have
				break
			}
		}
		if ok {
			c.Combos = append(c.Combos, gamedata.Combo{Items: cb.Items, Script: cb.Script})
		}
	}
	return jobs, items
}

// cardPrefix is how each client language names a monster's card.
var cardPrefix = map[string]string{"pt": "Carta ", "es": "Carta "}

// monsterNames takes a monster's local names from its card ("Carta Pesar
// Noturno" -> "Pesar Noturno"): the client has no monster name table.
func monsterNames(c *gamedata.Catalog, cards []int) gamedata.Text {
	for _, id := range cards {
		it := c.Items[id]
		if it == nil {
			continue
		}
		names := gamedata.Text{}
		for l, p := range cardPrefix {
			if n := it.Name.Get(l); strings.HasPrefix(n, p) && len(n) > len(p) {
				names[l] = strings.TrimSpace(strings.TrimPrefix(n, p))
			}
		}
		if len(names) > 0 {
			return names
		}
	}
	return nil
}

// isUpper mirrors rAthena's (class & JOBL_UPPER) || pc_is_primary_fourth:
// transcendent classes (client codes ending in _H, including transcendent
// 3rd jobs) and the 13 primary 4th jobs (4252-4264) get MaxHP/SP x1.25.
func isUpper(j *gamedata.Job) bool {
	return strings.HasSuffix(j.Code, "_H") || (j.ID >= 4252 && j.ID <= 4264)
}

// equipGroups maps client job codes to rAthena item job groups.
var equipGroups = map[string]string{
	"JT_NOVICE": "Novice", "JT_SWORDMAN": "Swordman", "JT_MAGICIAN": "Mage", "JT_ARCHER": "Archer",
	"JT_ACOLYTE": "Acolyte", "JT_MERCHANT": "Merchant", "JT_THIEF": "Thief", "JT_KNIGHT": "Knight",
	"JT_PRIEST": "Priest", "JT_WIZARD": "Wizard", "JT_BLACKSMITH": "Blacksmith", "JT_HUNTER": "Hunter",
	"JT_ASSASSIN": "Assassin", "JT_CRUSADER": "Crusader", "JT_MONK": "Monk", "JT_SAGE": "Sage",
	"JT_ROGUE": "Rogue", "JT_ALCHEMIST": "Alchemist", "JT_BARD": "BardDancer", "JT_DANCER": "BardDancer",
	"JT_SUPERNOVICE": "SuperNovice", "JT_GUNSLINGER": "Gunslinger", "JT_NINJA": "Ninja",
	"JT_TAEKWON": "Taekwon", "JT_STAR": "StarGladiator", "JT_LINKER": "SoulLinker",
	"JT_KAGEROU": "KagerouOboro", "JT_OBORO": "KagerouOboro", "JT_REBELLION": "Rebellion", "JT_DO_SUMMONER": "Summoner",
}

var thirdJobs = []string{"RUNE_KNIGHT", "WARLOCK", "RANGER", "ARCHBISHOP", "MECHANIC", "GUILLOTINE_CROSS",
	"ROYAL_GUARD", "SORCERER", "MINSTREL", "WANDERER", "SURA", "GENETIC", "SHADOW_CHASER"}

// equipClass returns the rAthena item group (deepest base class in the
// chain) and class tier, following map.hpp's MAPID flags.
func equipClass(c *gamedata.Catalog, j *gamedata.Job) (group, tier string) {
	for _, id := range c.Chain(j.ID) {
		if g, ok := equipGroups[strings.TrimSuffix(c.Jobs[id].Code, "_H")]; ok {
			group = g
		}
	}
	base := strings.TrimSuffix(strings.TrimPrefix(j.Code, "JT_"), "_H")
	switch {
	case (j.ID >= 4252 && j.ID <= 4264) || j.ID == 4302 || j.ID == 4303 || j.ID == 4307:
		tier = "Fourth" // primary 4th, Sky Emperor, Soul Ascetic, Hyper Novice
	case slices.Contains(thirdJobs, base):
		tier = "Third"
		if strings.HasSuffix(j.Code, "_H") {
			tier = "Third_Upper"
		}
	case slices.Contains([]string{"SUPERNOVICE2", "STAR_EMPEROR", "SOUL_REAPER", "NIGHT_WATCH", "SHINKIRO", "SHIRANUI"}, base):
		tier = "Third" // expanded jobs in rAthena's JOBL_THIRD block
	case strings.HasSuffix(j.Code, "_H"):
		tier = "Upper"
	case j.Baby:
		tier = "Baby"
	default:
		tier = "Normal" // includes Kagerou/Oboro, Rebellion, Summoner, Spirit Handler
	}
	return group, tier
}

// canonicalType fixes inconsistent casing in rAthena (Shadowgear/ShadowGear).
func canonicalType(t string) string {
	switch strings.ToLower(t) {
	case "shadowgear":
		return "ShadowGear"
	case "petegg":
		return "PetEgg"
	case "petarmor":
		return "PetArmor"
	}
	return t
}
