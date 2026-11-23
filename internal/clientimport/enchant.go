package clientimport

import (
	"errors"
	"path/filepath"
	"slices"

	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/grf"
	"ragnarok-of-building/internal/lua51"
)

// enchantInfo is one CreateEnchantInfo() block of the client's enchant list:
// the items it applies to and the enchants each card position accepts.
type enchantInfo struct {
	targets []string         // AegisName
	slots   map[int][]string // card index (0-based) -> enchant AegisNames
}

// parseEnchantList runs enchantlist.lub with native stand-ins for the
// client's EnchantInfo object, collecting targets and enchants. Methods that
// only set rates, costs or texts are accepted and ignored.
func parseEnchantList(b []byte) ([]*enchantInfo, error) {
	var infos []*enchantInfo
	noop := lua51.GoFunc(func([]lua51.Value) []lua51.Value { return nil })
	method := func(t *lua51.Table, name string, f func(args []lua51.Value)) {
		t.Set(name, lua51.GoFunc(func(args []lua51.Value) []lua51.Value {
			f(args)
			return nil
		}))
	}
	ignored := []string{"SetSlotOrder", "SetCondition", "ApproveRandomOption", "SetReset", "SetCaution",
		"SetRequire", "SetSuccessRate", "SetGradeBonus"}
	create := lua51.GoFunc(func([]lua51.Value) []lua51.Value {
		info := &enchantInfo{slots: map[int][]string{}}
		infos = append(infos, info)
		obj := lua51.NewTable()
		for _, n := range ignored {
			obj.Set(n, noop)
		}
		method(obj, "AddTargetItem", func(args []lua51.Value) {
			if len(args) > 1 {
				info.targets = append(info.targets, str(args[1]))
			}
		})
		slotsT := lua51.NewTable()
		for k := 0; k <= 4; k++ {
			k := k
			s := lua51.NewTable()
			for _, n := range ignored {
				s.Set(n, noop)
			}
			add := func(a string) {
				if a != "" && !slices.Contains(info.slots[k], a) {
					info.slots[k] = append(info.slots[k], a)
				}
			}
			// SetEnchant(grade, enchant, rate); AddPerfectEnchant(enchant, ...).
			method(s, "SetEnchant", func(args []lua51.Value) {
				if len(args) > 2 {
					add(str(args[2]))
				}
			})
			method(s, "AddPerfectEnchant", func(args []lua51.Value) {
				if len(args) > 1 {
					add(str(args[1]))
				}
			})
			// AddUpgradeEnchant(from, to, ...): an enchant upgraded in place.
			method(s, "AddUpgradeEnchant", func(args []lua51.Value) {
				for _, a := range args[1:min(len(args), 3)] {
					add(str(a))
				}
			})
			s.Missing = func(lua51.Value) lua51.Value { return noop }
			slotsT.Set(float64(k), s)
		}
		obj.Set("Slot", slotsT)
		obj.Missing = func(lua51.Value) lua51.Value { return noop }
		return []lua51.Value{obj}
	})
	_, err := lua51.LoadEnv(b, map[string]lua51.Value{"Table": lua51.NewTable(), "CreateEnchantInfo": create})
	var pe *lua51.PartialError
	if errors.As(err, &pe) && len(infos) > 0 {
		err = nil // stopped after the data (an unknown method): keep what was read
	}
	return infos, err
}

// ImportEnchants reads which enchants each item accepts (the client's
// enchant list) into c.Enchants. aegis maps AegisName to item ID (rAthena).
func ImportEnchants(dir string, c *gamedata.Catalog, aegis map[string]int) (int, error) {
	a, err := grf.Open(filepath.Join(dir, "data.grf"))
	if err != nil {
		return 0, err
	}
	defer a.Close()
	b, err := a.Read(`data\` + luaDir + `enchant\enchantlist.lub`)
	if err != nil {
		return 0, err
	}
	infos, err := parseEnchantList(b)
	if err != nil {
		return 0, err
	}
	c.Enchants = map[int]map[int][]int{}
	for _, info := range infos {
		for _, t := range info.targets {
			id, ok := aegis[t]
			if !ok || c.Items[id] == nil {
				continue
			}
			for k, list := range info.slots {
				for _, e := range list {
					eid, ok := aegis[e]
					if !ok || c.Items[eid] == nil {
						continue
					}
					if c.Enchants[id] == nil {
						c.Enchants[id] = map[int][]int{}
					}
					if !slices.Contains(c.Enchants[id][k], eid) {
						c.Enchants[id][k] = append(c.Enchants[id][k], eid)
					}
				}
			}
		}
	}
	addNPCEnchants(c)
	for _, slots := range c.Enchants {
		for k := range slots {
			slices.Sort(slots[k])
		}
	}
	return len(c.Enchants), nil
}

// addNPCEnchants adds enchants an NPC gives that the client's enchant list
// does not have.
//
// The Memoráveis (Old headgears of the Nightmare Biolabs, 18971-18984) are
// enchanted by Wandering Grudge (lhz_dun_n): the 3rd and 4th positions take
// a stat +1 to +5, the 2nd a stat +1 to +5 or a special enchant Lv 1-10
// (Mettle, Magic Essence, Acute, Master Archer, Adamantine, Affection,
// Tenacity). https://irowiki.org/wiki/Bio5
func addNPCEnchants(c *gamedata.Catalog) {
	var stats, special []int
	for base := 4700; base <= 4750; base += 10 { // STR, INT, DEX, AGI, VIT, LUK
		for lv := 0; lv < 5; lv++ {
			stats = append(stats, base+lv)
		}
	}
	for _, base := range []int{29061, 29071, 29081, 29091, 29101, 29111, 29706} {
		for lv := 0; lv < 10; lv++ {
			special = append(special, base+lv)
		}
	}
	known := func(ids []int) []int {
		var out []int
		for _, id := range ids {
			if c.Items[id] != nil {
				out = append(out, id)
			}
		}
		return out
	}
	stats, special = known(stats), known(special)
	addIllusionEnchants(c, known)
	addAutomatronEnchants(c, known)
	for id := 18971; id <= 18984; id++ {
		if c.Items[id] == nil {
			continue
		}
		if c.Enchants[id] == nil {
			c.Enchants[id] = map[int][]int{}
		}
		add := func(k int, ids []int) {
			for _, e := range ids {
				if !slices.Contains(c.Enchants[id][k], e) {
					c.Enchants[id][k] = append(c.Enchants[id][k], e)
				}
			}
		}
		add(1, stats)
		add(1, special)
		add(2, stats)
		add(3, stats)
	}
}

// addIllusionEnchants adds the Illusion equipment's NPC enchants (rAthena
// npc/re/merchants):
//
//   - enchan_illusion_dungeons.txt, the Illusion Enchanter (Prontera): the
//     Ilusional gear of every Illusion dungeon it lists takes two
//     enchants, in the 4th position then the 3rd; weapons (level 1-3) the
//     melee, ranged and magic lists, armor/shoes/garment/shield/upper hat
//     one list, accessories another;
//   - enchan_illusion_17_1.txt, RS26: the Ilusión A/B set takes modules in
//     the 2nd-4th positions, a list per part, plus one Legendary module.
func addIllusionEnchants(c *gamedata.Catalog, known func([]int) []int) {
	add := func(id int, positions []int, ids []int) {
		if c.Items[id] == nil {
			return
		}
		if c.Enchants[id] == nil {
			c.Enchants[id] = map[int][]int{}
		}
		for _, k := range positions {
			for _, e := range ids {
				if !slices.Contains(c.Enchants[id][k], e) {
					c.Enchants[id][k] = append(c.Enchants[id][k], e)
				}
			}
		}
	}
	seq := func(from, to int) []int {
		var out []int
		for i := from; i <= to; i++ {
			out = append(out, i)
		}
		return out
	}
	cat := func(lists ...[]int) []int {
		var out []int
		for _, l := range lists {
			out = append(out, l...)
		}
		return known(out)
	}
	// Illusion Enchanter.
	weapon := cat(seq(4700, 4703), seq(4720, 4723), seq(4750, 4753), seq(4808, 4811), []int{4820}, seq(29081, 29085), seq(29061, 29065), []int{4807, 4842}, // melee
		seq(4730, 4733), seq(4832, 4836), seq(29091, 29095), seq(29071, 29075), // ranged
		seq(4710, 4713), seq(4812, 4815), []int{4826, 4805, 4850}) // magic
	armor := cat(seq(4700, 4703), seq(4740, 4743), seq(4710, 4713), seq(4750, 4753), []int{4994, 4995, 4997, 4998, 29009, 29010, 29000, 29001, 29003, 29004, 29006, 29007},
		[]int{4861, 4862, 4867, 4868, 4992, 4993, 29208, 29210})
	acc := cat(seq(4700, 4703), seq(4730, 4733), seq(4740, 4743), seq(4710, 4713), seq(4720, 4723), seq(4750, 4753),
		[]int{4928, 4870, 4800, 4871, 4801, 4795, 4796, 4797, 4798, 4799})
	for _, id := range []int{
		15195, 16063, 19209, 20838, 19210, 22133, 26007, 26109, 28725, // Moonlight
		28022, 28023, 2039, 18149, 28612, 20840, 28508, 28509, // Vampire
		1846, 13337, 19223, 28922, 20847, // Frozen
		28745, 28244, 2051, 22190, 19344, // Teddy bear
		13469, 1326, 32005, 13338, 16065, 19247, // Turtle
		450182, 470066, 490120, 490121, 550030, 550031, 530015, 500030, 620010, 510034, 460017, 460018, // Twins
		28626, 18174, 19366, 15348, 20923, 22192, // Luanda
		19428, 20948, 32238, 32239, 28254, 21050, 32301, 28762, // Labyrinth
		570008, 580008, 630006, 610012, 600011, 630007, 450144, 450145, 450146, 400053, 480054, 490069, 490070, // underwater
	} {
		it := c.Items[id]
		switch {
		case it == nil:
		case it.Type == "Weapon": // rAthena stops at weapon level 3; LATAM's are level 4 and take them
			add(id, []int{2, 3}, weapon)
		case slices.Contains(it.Locations, "Right_Accessory") || slices.Contains(it.Locations, "Left_Accessory") || slices.Contains(it.Locations, "Both_Accessory"):
			add(id, []int{2, 3}, acc)
		default:
			add(id, []int{2, 3}, armor)
		}
	}
	// RS26 modules.
	legendary := known(seq(29547, 29552))
	modules := map[int][]int{
		0: cat([]int{29527, 29528, 29534, 29535, 29536, 29540}),                                                             // armor
		1: cat([]int{29527, 29528, 29537, 29538, 29539, 29542}),                                                             // engine wing
		2: cat([]int{4742, 4752, 4702, 4732, 4712, 4722, 29529, 29532, 4826, 4881, 4866, 4836, 29543, 29544, 29545, 29546}), // accessories
		3: cat([]int{29527, 29528, 29531, 29530, 29533, 29541}),                                                             // leg
	}
	// The [Apoio] (support) pieces take their set piece's modules; the
	// Escudo Ilusión, absent from the script, every module of the set (a
	// recording shows it with armor and engine wing modules).
	for table, ids := range map[int][]int{
		0: {15376, 15377, 450147, 450148}, 1: {20933, 20934, 480062, 480063}, 3: {22196, 22197, 470054, 470055},
		2: {32207, 32208, 32209, 32210, 490072, 490073, 490074, 490075},
	} {
		for _, id := range ids {
			add(id, []int{1, 2, 3}, append(append([]int{}, modules[table]...), legendary...))
		}
	}
	all := append(append(append(append([]int{}, modules[0]...), modules[1]...), modules[3]...), legendary...)
	for _, id := range []int{460004, 460014} {
		add(id, []int{1, 2, 3}, all)
	}
}

// addAutomatronEnchants adds the Automatron equipment's modules (rAthena
// npc/re/merchants/enchan_sage_legacy_17_2.txt, Lisa): each module item
// becomes an enchant (310082 onwards, in module order, 1000150-1000151
// skipped; 1000207 and 1000208 are 310178 and 310179), in the 2nd-4th
// positions, from a list per part.
func addAutomatronEnchants(c *gamedata.Catalog, known func([]int) []int) {
	enchantOf := map[int]int{1000207: 310178, 1000208: 310179}
	next := 310082
	for m := 1000105; m < 1000203; m++ {
		if m == 1000150 || m == 1000151 {
			continue
		}
		enchantOf[m] = next
		next++
	}
	mods := func(ids ...int) []int {
		var out []int
		for _, m := range ids {
			out = append(out, enchantOf[m])
		}
		return known(out)
	}
	rng := func(from, to int) []int {
		var out []int
		for m := from; m <= to; m++ {
			out = append(out, m)
		}
		return out
	}
	garment := mods(1000105, 1000106, 1000125, 1000126, 1000127, 1000135, 1000142, 1000208)
	armor := mods(append([]int{1000105, 1000106, 1000122, 1000123, 1000124, 1000128, 1000129, 1000130, 1000131, 1000132, 1000133, 1000207, 1000140}, rng(1000152, 1000202)...)...)
	shoes := mods(append([]int{1000105, 1000106, 1000119, 1000120, 1000121, 1000134, 1000141}, rng(1000144, 1000149)...)...)
	acc := mods(append(rng(1000107, 1000118), 1000136, 1000137, 1000138, 1000139, 1000143)...)
	// The shields are not in the script: every module of the garment, armor
	// and leg lists, like the Escudo Ilusión.
	shield := append(append(append([]int{}, garment...), armor...), shoes...)
	for ids, list := range map[[4]int][]int{
		{480020, 480021}: garment, {450127, 450128}: armor, {470022, 470023}: shoes, {490024, 490025, 490026, 490027}: acc,
		{460157, 460158}: shield,
	} {
		for _, id := range ids {
			if id == 0 || c.Items[id] == nil {
				continue
			}
			if c.Enchants[id] == nil {
				c.Enchants[id] = map[int][]int{}
			}
			for _, k := range []int{1, 2, 3} {
				for _, e := range list {
					if !slices.Contains(c.Enchants[id][k], e) {
						c.Enchants[id][k] = append(c.Enchants[id][k], e)
					}
				}
			}
		}
	}
}
