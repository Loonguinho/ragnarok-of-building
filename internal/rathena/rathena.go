// Package rathena reads the rAthena (GPL-3.0) renewal database files that
// cmd/fetchrathena downloads into data/rathena. Only the tables the simulator
// needs are parsed: job stats/HP/SP/ASPD/levels, status points and equipment.
package rathena

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Job holds every per-job table, keyed by the job's numeric ID.
type Job struct {
	Names      []string // rAthena names mapped to this ID (e.g. Knight, Knight2)
	MaxWeight  int
	HPFactor   int // formula fallback when BaseHP has no entry (rAthena calc_basehp)
	HPIncrease int
	SPFactor   int
	SPIncrease int
	MaxBaseLv  int
	MaxJobLv   int
	BaseHP     []int // index: base level - 1
	BaseSP     []int
	BaseAP     []int
	BonusStats map[int][12]int // job level -> STR AGI VIT INT DEX LUK POW STA WIS SPL CON CRT
	BaseASPD   map[string]int  // weapon type (Fist, Dagger, 1hSword, ..., Shield) -> base delay
}

// BaseHPAt returns base HP at a level: the table value, or rAthena's
// calc_basehp formula when the table has no entry (e.g. 4th jobs).
func (j *Job) BaseHPAt(lv int) int {
	if lv >= 1 && lv <= len(j.BaseHP) && j.BaseHP[lv-1] > 0 {
		return j.BaseHP[lv-1]
	}
	hp := 35.0 + math.Floor(float64(lv)*float64(j.HPIncrease)/100)
	for i := 2; i <= lv; i++ {
		hp += math.Floor(float64(j.HPFactor)/100*float64(i) + 0.5)
	}
	return int(hp)
}

// BaseSPAt is BaseHPAt for SP (rAthena calc_basesp).
func (j *Job) BaseSPAt(lv int) int {
	if lv >= 1 && lv <= len(j.BaseSP) && j.BaseSP[lv-1] > 0 {
		return j.BaseSP[lv-1]
	}
	sp := 10.0 + math.Floor(float64(lv)*float64(j.SPIncrease)/100)
	for i := 2; i <= lv; i++ {
		sp += math.Floor(float64(j.SPFactor)/100*float64(i) + 0.5)
	}
	return int(sp)
}

// Item is one equipment or card entry.
type Item struct {
	ID          int
	Name        string
	Type        string // Weapon, Armor, Card, ...
	SubType     string // Dagger, 1hSword, ... for weapons
	Locations   []string
	Slots       int
	Attack      int
	MagicAttack int
	Defense     int
	Range       int
	WeaponLevel int
	ArmorLevel  int
	EquipLvMin  int
	EquipLvMax  int
	Refineable  bool
	Jobs        []string
	Classes     []string
	Gender      string
	Script      string
	Weight      int // tenths, as in rAthena
}

// DB is everything loaded from one rAthena checkout.
type DB struct {
	Commit      string
	Jobs        map[int]*Job
	StatPoints  []int // index: base level - 1, total points at that level
	TraitPoints []int // index: base level - 1, total trait points (4th jobs)
	Items       map[int]*Item
	// Refine[group][equipLevel][refine] = cumulative bonus in 1/100
	// (group: Armor, Weapon, Shadow_Armor, Shadow_Weapon).
	Refine map[string]map[int][]int
	Mobs   map[int]*Mob
	// AttrFix[defElementLevel][atkElement][defElement] = damage %.
	AttrFix map[int]map[string]map[string]int
	// SizeFix[weaponType][Small|Medium|Large] = damage %.
	SizeFix map[string]map[string]int
}

// Load reads the files under dir (data/rathena).
func Load(dir string) (*DB, error) {
	ids, err := parseJobEnum(filepath.Join(dir, "src", "common", "mmo.hpp"))
	if err != nil {
		return nil, err
	}
	db := &DB{Jobs: map[int]*Job{}, Items: map[int]*Item{}}
	if b, err := os.ReadFile(filepath.Join(dir, "COMMIT")); err == nil {
		db.Commit = strings.TrimSpace(string(b))
	}
	re := filepath.Join(dir, "db", "re")

	job := func(name string) *Job {
		id, ok := ids[strings.ToUpper(name)]
		if !ok {
			return nil
		}
		j := db.Jobs[id]
		if j == nil {
			// Defaults from rAthena's JobDatabase when a field is absent.
			j = &Job{BonusStats: map[int][12]int{}, BaseASPD: map[string]int{}, HPIncrease: 500, SPIncrease: 100}
			db.Jobs[id] = j
		}
		for _, n := range j.Names {
			if n == name {
				return j
			}
		}
		j.Names = append(j.Names, name)
		return j
	}

	// Job tables share one shape: Body: [{Jobs: {Name: true}, ...fields}].
	for _, f := range []string{"job_stats.yml", "job_basepoints.yml", "job_aspd.yml", "job_exp.yml"} {
		var doc struct {
			Body []jobEntry `yaml:"Body"`
		}
		if err := readYAML(filepath.Join(re, f), &doc); err != nil {
			return nil, err
		}
		for _, e := range doc.Body {
			for name, on := range e.Jobs {
				if !on {
					continue
				}
				if j := job(name); j != nil {
					e.apply(j)
				}
			}
		}
	}

	var sp struct {
		Body []struct {
			Level       int `yaml:"Level"`
			Points      int `yaml:"Points"`
			TraitPoints int `yaml:"TraitPoints"`
		} `yaml:"Body"`
	}
	if err := readYAML(filepath.Join(re, "statpoint.yml"), &sp); err != nil {
		return nil, err
	}
	for _, e := range sp.Body {
		if e.Level < 1 || e.Level > 1000 {
			continue
		}
		for len(db.StatPoints) < e.Level {
			db.StatPoints = append(db.StatPoints, 0)
			db.TraitPoints = append(db.TraitPoints, 0)
		}
		db.StatPoints[e.Level-1] = e.Points
		db.TraitPoints[e.Level-1] = e.TraitPoints
	}

	var rf struct {
		Body []struct {
			Group  string `yaml:"Group"`
			Levels []struct {
				Level        int `yaml:"Level"`
				RefineLevels []struct {
					Level int `yaml:"Level"`
					Bonus int `yaml:"Bonus"`
				} `yaml:"RefineLevels"`
			} `yaml:"Levels"`
		} `yaml:"Body"`
	}
	if err := readYAML(filepath.Join(re, "refine.yml"), &rf); err != nil {
		return nil, err
	}
	db.Refine = map[string]map[int][]int{}
	for _, g := range rf.Body {
		if db.Refine[g.Group] == nil {
			db.Refine[g.Group] = map[int][]int{}
		}
		for _, l := range g.Levels {
			tbl := make([]int, 21)
			for _, r := range l.RefineLevels {
				if r.Level >= 1 && r.Level <= 20 {
					tbl[r.Level] = r.Bonus
				}
			}
			for i := 1; i <= 20; i++ { // carry forward if a level is missing
				if tbl[i] == 0 {
					tbl[i] = tbl[i-1]
				}
			}
			db.Refine[g.Group][l.Level] = tbl
		}
	}

	if err := db.loadMobs(re); err != nil {
		return nil, err
	}

	for _, f := range []string{"item_db_equip.yml", "item_db_etc.yml"} {
		var doc struct {
			Body []itemEntry `yaml:"Body"`
		}
		if err := readYAML(filepath.Join(re, f), &doc); err != nil {
			return nil, err
		}
		for _, e := range doc.Body {
			if f == "item_db_etc.yml" && e.Type != "Card" {
				continue // only cards matter from the etc table
			}
			db.Items[e.ID] = e.item()
		}
	}
	return db, nil
}

func readYAML(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("rathena: %w (run `go run ./cmd/fetchrathena`)", err)
	}
	defer f.Close()
	if err := yaml.NewDecoder(bufio.NewReader(f)).Decode(v); err != nil {
		return fmt.Errorf("rathena: %s: %w", filepath.Base(path), err)
	}
	return nil
}

type jobEntry struct {
	Jobs         map[string]bool `yaml:"Jobs"`
	MaxWeight    int             `yaml:"MaxWeight"`
	HpFactor     *int            `yaml:"HpFactor"`
	HpIncrease   *int            `yaml:"HpIncrease"`
	SpFactor     *int            `yaml:"SpFactor"`
	SpIncrease   *int            `yaml:"SpIncrease"`
	MaxBaseLevel int             `yaml:"MaxBaseLevel"`
	MaxJobLevel  int             `yaml:"MaxJobLevel"`
	BaseHp       []struct {
		Level int `yaml:"Level"`
		Hp    int `yaml:"Hp"`
	} `yaml:"BaseHp"`
	BaseSp []struct {
		Level int `yaml:"Level"`
		Sp    int `yaml:"Sp"`
	} `yaml:"BaseSp"`
	BaseAp []struct {
		Level int `yaml:"Level"`
		Ap    int `yaml:"Ap"`
	} `yaml:"BaseAp"`
	BonusStats []map[string]int `yaml:"BonusStats"`
	BaseASPD   map[string]int   `yaml:"BaseASPD"`
}

var statKeys = []string{"Str", "Agi", "Vit", "Int", "Dex", "Luk", "Pow", "Sta", "Wis", "Spl", "Con", "Crt"}

func (e *jobEntry) apply(j *Job) {
	if e.MaxWeight > 0 {
		j.MaxWeight = e.MaxWeight
	}
	for _, f := range []struct {
		src *int
		dst *int
	}{{e.HpFactor, &j.HPFactor}, {e.HpIncrease, &j.HPIncrease}, {e.SpFactor, &j.SPFactor}, {e.SpIncrease, &j.SPIncrease}} {
		if f.src != nil {
			*f.dst = *f.src
		}
	}
	if e.MaxBaseLevel > 0 {
		j.MaxBaseLv = e.MaxBaseLevel
	}
	if e.MaxJobLevel > 0 {
		j.MaxJobLv = e.MaxJobLevel
	}
	set := func(tbl *[]int, lv, v int) {
		if lv < 1 || lv > 1000 {
			return
		}
		for len(*tbl) < lv {
			*tbl = append(*tbl, 0)
		}
		(*tbl)[lv-1] = v
	}
	for _, p := range e.BaseHp {
		set(&j.BaseHP, p.Level, p.Hp)
	}
	for _, p := range e.BaseSp {
		set(&j.BaseSP, p.Level, p.Sp)
	}
	for _, p := range e.BaseAp {
		set(&j.BaseAP, p.Level, p.Ap)
	}
	for _, b := range e.BonusStats {
		lv := b["Level"]
		var s [12]int
		for i, k := range statKeys {
			s[i] = b[k]
		}
		j.BonusStats[lv] = s
	}
	for w, v := range e.BaseASPD {
		j.BaseASPD[w] = v
	}
}

type itemEntry struct {
	ID            int             `yaml:"Id"`
	Name          string          `yaml:"Name"`
	Type          string          `yaml:"Type"`
	SubType       string          `yaml:"SubType"`
	Locations     map[string]bool `yaml:"Locations"`
	Slots         int             `yaml:"Slots"`
	Attack        int             `yaml:"Attack"`
	MagicAttack   int             `yaml:"MagicAttack"`
	Defense       int             `yaml:"Defense"`
	Range         int             `yaml:"Range"`
	WeaponLevel   int             `yaml:"WeaponLevel"`
	ArmorLevel    int             `yaml:"ArmorLevel"`
	EquipLevelMin int             `yaml:"EquipLevelMin"`
	EquipLevelMax int             `yaml:"EquipLevelMax"`
	Refineable    bool            `yaml:"Refineable"`
	Jobs          map[string]bool `yaml:"Jobs"`
	Classes       map[string]bool `yaml:"Classes"`
	Gender        string          `yaml:"Gender"`
	Script        string          `yaml:"Script"`
	Weight        int             `yaml:"Weight"`
}

func (e itemEntry) item() *Item {
	keys := func(m map[string]bool) []string {
		var out []string
		for k, v := range m {
			if v {
				out = append(out, k)
			}
		}
		return out
	}
	return &Item{
		ID: e.ID, Name: e.Name, Type: e.Type, SubType: e.SubType,
		Locations: keys(e.Locations), Slots: e.Slots, Attack: e.Attack,
		MagicAttack: e.MagicAttack, Defense: e.Defense, Range: e.Range,
		WeaponLevel: e.WeaponLevel, ArmorLevel: e.ArmorLevel,
		EquipLvMin: e.EquipLevelMin, EquipLvMax: e.EquipLevelMax,
		Refineable: e.Refineable, Jobs: keys(e.Jobs), Classes: keys(e.Classes),
		Gender: e.Gender, Script: e.Script, Weight: e.Weight,
	}
}

var enumLine = regexp.MustCompile(`^\s*JOB_([A-Z0-9_]+)\s*(?:=\s*(\d+))?\s*,`)

// parseJobEnum reads `enum e_job { JOB_NOVICE, ..., JOB_NOVICE_HIGH = 4001, ... }`
// and returns NAME (without JOB_) -> numeric ID.
func parseJobEnum(path string) (map[string]int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("rathena: %w (run `go run ./cmd/fetchrathena`)", err)
	}
	defer f.Close()
	ids := map[string]int{}
	in := false
	next := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "enum e_job ") || strings.HasPrefix(line, "enum e_job{") {
			in = true
			continue
		}
		if !in {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "}") {
			break
		}
		m := enumLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if m[2] != "" {
			next, _ = strconv.Atoi(m[2])
		}
		ids[m[1]] = next
		next++
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("rathena: e_job enum not found in %s", path)
	}
	return ids, sc.Err()
}
