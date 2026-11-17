package clientimport

import (
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
		j.Upper = isUpper(id)
		j.Fourth = (id >= 4252 && id <= 4279) || (id >= 4302 && id <= 4330)
		j.Baby = strings.Contains(j.Code, "_B") || strings.HasPrefix(j.Code, "JT_BABY")
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
		if it.Slots == 0 {
			it.Slots = ri.Slots
		}
	}
	return jobs, items
}

// isUpper mirrors rAthena's (class & JOBL_UPPER) || pc_is_primary_fourth:
// transcendent 1st/2nd (4001-4022), transcendent 3rd incl. mounts (4060-4087)
// and primary 4th jobs incl. mounts (4252-4279) get MaxHP/SP x1.25.
func isUpper(id int) bool {
	return (id >= 4001 && id <= 4022) || (id >= 4060 && id <= 4087) || (id >= 4252 && id <= 4279)
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
