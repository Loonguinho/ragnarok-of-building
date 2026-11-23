package replay

import (
	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

// Client status (EFST) ids, from the LATAM client's efstids.lub (they are
// not rAthena's numbering), turned into the Buffs window's rows. A status
// does not carry the caster's level, so leveled buffs take their usual top
// level (latam-ro-calc does the same); the user can lower them.
var efstBuffs = map[int]struct {
	key string
	v   int
}{
	10: {"blessing", 10}, 12: {"incagi", 10}, 15: {"impositio", 5}, 21: {"gloria", 1},
	340: {"expiatio", 5}, 331: {"lauda_agnus", 4}, 332: {"lauda_ramus", 4}, 583: {"odin", 2},
	1198: {"argutus_vita", 5}, 1199: {"argutus_telum", 5}, 1200: {"presens_acies", 5},
	1201: {"competentia", 1}, 1227: {"religio", 1}, 1228: {"benedictum", 1},
	70: {"whistle", 10}, 71: {"assassincross", 10}, 73: {"appleidun", 10},
	74: {"humming", 10}, 76: {"fortunekiss", 10}, 77: {"serviceforyou", 10}, 80: {"drumbattle", 5},
	442: {"rushwindmill", 105}, 447: {"moonlit", 17}, 715: {"frigg", 5}, 451: {"lerads", 105},
	441: {"dancewithwug", 5}, 454: {"infinitehumming", 105},
	1261: {"interlude", 5}, 1263: {"pronmarch", 5}, 1262: {"jawaii", 5},
	30: {"loud", 1}, 426: {"gt_change", 5}, 427: {"gt_revitalize", 5},
	1057: {"soul_fairy", 1}, 1056: {"soul_shadow", 1}, 1058: {"soul_falcon", 1}, 1059: {"soul_golem", 1},
	445: {"striking", 1}, 920: {"bunchofshrimp", 1},
	1357: {"soa_warrior", 5}, 1358: {"soa_magician", 5}, 1359: {"soa_elements", 5}, 1365: {"soa_heavenearth", 10},
}

// efstStates are the class's own statuses: the attack state they switch on,
// at a fixed value or (skill != "") at the level the character learned.
var efstStates = map[int]struct {
	name  string
	v     int
	skill string
}{
	114:  {"Enchant Deadly Poison", 1, ""},
	333:  {"Cloaking Exceed", 5, ""},
	1192: {"Shadow Exceed", 10, ""},
	1194: {"Potent Venom", 10, "SHC_POTENT_VENOM"},
	334:  {"Hallucination Walk", 5, "GC_HALLUCINATIONWALK"},
	722:  {"Unlimit", 5, "RA_UNLIMIT"}, // Ilimitar, 150 s
}

const (
	efstConcentration = 3 // Concentrar, at the learned level
	// Aplicar Toxina (341) coats the weapon with a toxin but keeps its
	// element: an Executor replay with it up hit a Poison 3 Rgan for
	// 48 million.
	efstEnchantPoison = 6 // Encantar com Veneno (AS_ENCHANTPOISON): poison endow
)

// applyStatuses turns the recording's statuses into buffs and attack states
// and returns how many it took.
func applyStatuses(b *calc.Build, s *Snapshot, c *gamedata.Catalog) int {
	learned := func(code string) int {
		for id, sk := range c.Skills {
			if sk.Code == code {
				return s.Skills[id]
			}
		}
		return 0
	}
	n := 0
	for _, id := range s.Statuses {
		if bf, ok := efstBuffs[id]; ok {
			if b.Buffs == nil {
				b.Buffs = map[string]int{}
			}
			b.Buffs[bf.key] = bf.v
			n++
			continue
		}
		if st, ok := efstStates[id]; ok {
			v := st.v
			if st.skill != "" {
				if lv := learned(st.skill); lv > 0 {
					v = lv
				}
			}
			if b.Atk.States == nil {
				b.Atk.States = map[string]int{}
			}
			b.Atk.States[calc.StateKeyOf(st.name)] = v
			n++
			continue
		}
		switch id {
		case efstConcentration:
			if lv := learned("AC_CONCENTRATION"); lv > 0 {
				if b.Buffs == nil {
					b.Buffs = map[string]int{}
				}
				b.Buffs["concentration"] = lv
				n++
			}
		case efstEnchantPoison:
			b.Atk.Element = "Poison"
			n++
		}
	}
	return n
}
