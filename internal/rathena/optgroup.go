package rathena

import (
	"path/filepath"
	"strings"
)

// OptRange is a random option an item position can roll, with its value range.
type OptRange struct{ ID, Min, Max int }

// OptGroup is an item_randomopt_group.yml group: the options each guaranteed
// position rolls, plus up to MaxRandom more from Random.
type OptGroup struct {
	Slots     [][]OptRange
	Random    []OptRange
	MaxRandom int
}

type optEntry struct {
	Option   string `yaml:"Option"`
	MinValue int    `yaml:"MinValue"`
	MaxValue int    `yaml:"MaxValue"`
}

// loadOptGroups reads the random option groups and which items get them:
// monster drops (RandomOptionGroup), item reforms (the result item) and
// Laphine upgrades (the target items). Needs RandomOpts and the item AegisName
// map. Groups are keyed by name; items by ID.
func (db *DB) loadOptGroups(re string, aegis map[string]int, drops [][2]string) error {
	byVar := map[string]int{}
	for id, o := range db.RandomOpts {
		byVar[o.Var] = id
	}
	ranges := func(es []optEntry) []OptRange {
		var out []OptRange
		for _, e := range es {
			if id, ok := byVar[e.Option]; ok {
				out = append(out, OptRange{ID: id, Min: e.MinValue, Max: max(e.MaxValue, e.MinValue)})
			}
		}
		return out
	}
	var groups struct {
		Body []struct {
			Group string `yaml:"Group"`
			Slots []struct {
				Slot    int        `yaml:"Slot"`
				Options []optEntry `yaml:"Options"`
			} `yaml:"Slots"`
			MaxRandom int        `yaml:"MaxRandom"`
			Random    []optEntry `yaml:"Random"`
		} `yaml:"Body"`
	}
	if err := readYAML(filepath.Join(re, "item_randomopt_group.yml"), &groups); err != nil {
		return err
	}
	db.OptGroups = map[string]*OptGroup{}
	for _, g := range groups.Body {
		og := &OptGroup{MaxRandom: g.MaxRandom, Random: ranges(g.Random)}
		for _, s := range g.Slots {
			if r := ranges(s.Options); len(r) > 0 {
				og.Slots = append(og.Slots, r)
			}
		}
		if len(og.Slots) > 0 || (og.MaxRandom > 0 && len(og.Random) > 0) {
			db.OptGroups[strings.TrimSpace(g.Group)] = og
		}
	}

	db.ItemOptGroups = map[int][]string{}
	link := func(item, group string) {
		id, ok := aegis[item]
		group = strings.TrimSpace(group)
		if !ok || db.OptGroups[group] == nil {
			return
		}
		for _, g := range db.ItemOptGroups[id] {
			if g == group {
				return
			}
		}
		db.ItemOptGroups[id] = append(db.ItemOptGroups[id], group)
	}
	for _, d := range drops {
		link(d[0], d[1])
	}
	// Groups named after the item they are for (Bloody_Knight_Shield).
	for name := range db.OptGroups {
		link(name, name)
	}
	var reform struct {
		Body []struct {
			BaseItems []struct {
				BaseItem          string `yaml:"BaseItem"`
				ResultItem        string `yaml:"ResultItem"`
				RandomOptionGroup string `yaml:"RandomOptionGroup"`
			} `yaml:"BaseItems"`
		} `yaml:"Body"`
	}
	if err := readYAML(filepath.Join(re, "item_reform.yml"), &reform); err != nil {
		return err
	}
	for _, r := range reform.Body {
		for _, b := range r.BaseItems {
			if b.RandomOptionGroup == "" {
				continue
			}
			target := b.ResultItem
			if target == "" {
				target = b.BaseItem
			}
			link(target, b.RandomOptionGroup)
		}
	}
	var laphine struct {
		Body []struct {
			RandomOptionGroup string `yaml:"RandomOptionGroup"`
			TargetItems       []struct {
				Item string `yaml:"Item"`
			} `yaml:"TargetItems"`
		} `yaml:"Body"`
	}
	if err := readYAML(filepath.Join(re, "laphine_upgrade.yml"), &laphine); err != nil {
		return err
	}
	for _, l := range laphine.Body {
		if l.RandomOptionGroup == "" {
			continue
		}
		for _, t := range l.TargetItems {
			link(t.Item, l.RandomOptionGroup)
		}
	}
	return nil
}
