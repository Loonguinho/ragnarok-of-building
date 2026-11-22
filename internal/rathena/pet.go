package rathena

import (
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Pet is one pet_db.yml entry: the monster, its egg and the bonus script
// (tiers by intimacy, getpetinfo(PETINFO_INTIMATE)).
type Pet struct {
	Mob    int
	Egg    int
	Script string
}

type petEntry struct {
	Mob     string `yaml:"Mob"`
	EggItem string `yaml:"EggItem"`
	Script  string `yaml:"Script"`
}

// aegisNumber is how pet_db names an item without an AegisName.
var aegisNumber = regexp.MustCompile(`^aegis_(\d+)$`)

// loadPets reads pet_db.yml (optional: older downloads lack it). The pets
// whose monster rAthena has no stats for (most evolved pets, which LATAM
// has) are commented out there, scripts included: those blocks are read
// too, and an active entry wins over a commented one.
func (db *DB) loadPets(path string, items map[string]int) error {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	var doc struct {
		Body []petEntry `yaml:"Body"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return err
	}
	var commented []petEntry
	for _, block := range commentedPetBlocks(string(raw)) {
		var e []petEntry
		if yaml.Unmarshal([]byte(block), &e) == nil {
			commented = append(commented, e...)
		}
	}
	mobs := map[string]int{}
	for id, aegis := range db.MobStubs {
		mobs[aegis] = id
	}
	for id, m := range db.Mobs {
		mobs[m.Aegis] = id
	}
	item := func(name string) int {
		if m := aegisNumber.FindStringSubmatch(name); m != nil {
			id, _ := strconv.Atoi(m[1])
			return id
		}
		return items[name]
	}
	db.Pets = map[int]*Pet{}
	add := func(e petEntry, replace bool) {
		mob, egg := mobs[e.Mob], item(e.EggItem)
		if mob == 0 || egg == 0 || strings.TrimSpace(e.Script) == "" {
			return
		}
		if _, ok := db.Pets[egg]; ok && !replace {
			return
		}
		db.Pets[egg] = &Pet{Mob: mob, Egg: egg, Script: e.Script}
	}
	for _, e := range commented {
		add(e, false)
	}
	for _, e := range doc.Body {
		add(e, true)
	}
	return nil
}

// commentedPetBlocks returns each commented-out "#  - Mob:" entry of
// pet_db.yml, uncommented, as a one-entry YAML list.
func commentedPetBlocks(raw string) []string {
	var out []string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			out = append(out, strings.Join(cur, "\n"))
			cur = nil
		}
	}
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "#  - Mob:"):
			flush()
			cur = append(cur, line[1:])
		case cur != nil && strings.HasPrefix(line, "#"):
			cur = append(cur, line[1:])
		default:
			flush()
		}
	}
	flush()
	return out
}
