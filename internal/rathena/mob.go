package rathena

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

// mobStub matches a commented-out mob_db entry: "#  - Id: 20994" followed by
// "#    AegisName: MD_BETELGEUSE".
var mobStub = regexp.MustCompile(`(?m)^#\s*- Id:\s*(\d+)\s*\n#\s*AegisName:\s*(\S+)`)

// Mob is one monster from mob_db.yml (renewal).
type Mob struct {
	ID          int
	Aegis       string
	Name        string
	Level       int
	HP          int
	Attack      int
	Attack2     int
	Defense     int
	MDefense    int
	Res         int
	MRes        int
	Stats       [6]int // STR AGI VIT INT DEX LUK
	Size        string // Small, Medium, Large
	Race        string // Demihuman, Brute, ...
	Element     string // Neutral, Water, ...
	ElementLv   int
	Class       string // Normal, Boss, Guardian, Battlefield, Event
	MVP         bool
	DamageTaken int // % of damage taken from players (default 100)
	Modes       []string
	DropAegis   []string // dropped items (AegisName), resolved into Cards
	Cards       []int    // dropped cards (item IDs)
}

type mobEntry struct {
	ID              int             `yaml:"Id"`
	AegisName       string          `yaml:"AegisName"`
	Name            string          `yaml:"Name"`
	Level           int             `yaml:"Level"`
	Hp              int             `yaml:"Hp"`
	Attack          int             `yaml:"Attack"`
	Attack2         int             `yaml:"Attack2"`
	Defense         int             `yaml:"Defense"`
	MagicDefense    int             `yaml:"MagicDefense"`
	Resistance      int             `yaml:"Resistance"`
	MagicResistance int             `yaml:"MagicResistance"`
	Str             *int            `yaml:"Str"`
	Agi             *int            `yaml:"Agi"`
	Vit             *int            `yaml:"Vit"`
	Int             *int            `yaml:"Int"`
	Dex             *int            `yaml:"Dex"`
	Luk             *int            `yaml:"Luk"`
	Size            string          `yaml:"Size"`
	Race            string          `yaml:"Race"`
	Element         string          `yaml:"Element"`
	ElementLevel    int             `yaml:"ElementLevel"`
	Class           string          `yaml:"Class"`
	DamageTaken     *int            `yaml:"DamageTaken"`
	Modes           map[string]bool `yaml:"Modes"`
	Drops           []struct {
		Item              string `yaml:"Item"`
		RandomOptionGroup string `yaml:"RandomOptionGroup"`
	} `yaml:"Drops"`
}

// loadMobs reads mob_db.yml, attr_fix.yml and size_fix.yml.
func (db *DB) loadMobs(re string) error {
	var mobs struct {
		Body []mobEntry `yaml:"Body"`
	}
	if err := readYAML(filepath.Join(re, "mob_db.yml"), &mobs); err != nil {
		return err
	}
	db.Mobs = map[int]*Mob{}
	for _, e := range mobs.Body {
		stat := func(p *int) int { // rAthena default for an omitted stat is 1
			if p == nil {
				return 1
			}
			return *p
		}
		m := &Mob{
			ID: e.ID, Aegis: e.AegisName, Name: e.Name, Level: e.Level, HP: e.Hp,
			Attack: e.Attack, Attack2: e.Attack2, Defense: e.Defense, MDefense: e.MagicDefense,
			Res: e.Resistance, MRes: e.MagicResistance,
			Stats: [6]int{stat(e.Str), stat(e.Agi), stat(e.Vit), stat(e.Int), stat(e.Dex), stat(e.Luk)},
			Size:  def(e.Size, "Small"), Race: def(e.Race, "Formless"), Element: def(e.Element, "Neutral"),
			ElementLv: max(1, e.ElementLevel), Class: def(e.Class, "Normal"), DamageTaken: 100,
			Modes: keys(e.Modes), // RaceGroups is not read: the file has duplicate keys in it
		}
		if e.DamageTaken != nil {
			m.DamageTaken = *e.DamageTaken
		}
		m.MVP = e.Modes["Mvp"]
		for _, d := range e.Drops {
			m.DropAegis = append(m.DropAegis, d.Item)
			if d.RandomOptionGroup != "" {
				db.optDrops = append(db.optDrops, [2]string{d.Item, d.RandomOptionGroup})
			}
		}
		db.Mobs[m.ID] = m
	}

	// Commented entries: monsters rAthena knows only by ID.
	raw, err := os.ReadFile(filepath.Join(re, "mob_db.yml"))
	if err != nil {
		return err
	}
	db.MobStubs = map[int]string{}
	for _, m := range mobStub.FindAllSubmatch(raw, -1) {
		if id, _ := strconv.Atoi(string(m[1])); id > 0 && db.Mobs[id] == nil {
			db.MobStubs[id] = string(m[2])
		}
	}

	// attr_fix.yml: Body: [{Level: n, <AtkEle>: {<DefEle>: pct}}]
	var attr struct {
		Body []map[string]any `yaml:"Body"`
	}
	if err := readYAML(filepath.Join(re, "attr_fix.yml"), &attr); err != nil {
		return err
	}
	db.AttrFix = map[int]map[string]map[string]int{}
	for _, e := range attr.Body {
		lv, _ := e["Level"].(int)
		tbl := map[string]map[string]int{}
		for atk, v := range e {
			row, ok := v.(map[string]any)
			if atk == "Level" || !ok {
				continue
			}
			tbl[atk] = map[string]int{}
			for d, pct := range row {
				if n, ok := pct.(int); ok {
					tbl[atk][d] = n
				}
			}
		}
		db.AttrFix[lv] = tbl
	}

	// size_fix.yml: Body: [{Weapon: X, Small/Medium/Large: pct}] (omitted = 100)
	var size struct {
		Body []struct {
			Weapon string `yaml:"Weapon"`
			Small  *int   `yaml:"Small"`
			Medium *int   `yaml:"Medium"`
			Large  *int   `yaml:"Large"`
		} `yaml:"Body"`
	}
	if err := readYAML(filepath.Join(re, "size_fix.yml"), &size); err != nil {
		return err
	}
	db.SizeFix = map[string]map[string]int{}
	for _, e := range size.Body {
		pct := func(p *int) int {
			if p == nil {
				return 100
			}
			return *p
		}
		db.SizeFix[e.Weapon] = map[string]int{"Small": pct(e.Small), "Medium": pct(e.Medium), "Large": pct(e.Large)}
	}
	return nil
}

func def(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func keys(m map[string]bool) []string {
	var out []string
	for k, v := range m {
		if v {
			out = append(out, k)
		}
	}
	return out
}
