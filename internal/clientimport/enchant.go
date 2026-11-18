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
	for _, slots := range c.Enchants {
		for k := range slots {
			slices.Sort(slots[k])
		}
	}
	return len(c.Enchants), nil
}
