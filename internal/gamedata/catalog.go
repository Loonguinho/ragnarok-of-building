// Package gamedata holds the read-only game catalog (items, skills, jobs)
// loaded into memory at startup.
package gamedata

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// Langs supported by the UI and catalog text.
var Langs = []string{"pt", "en", "es"}

// Text is localized text keyed by language ("pt", "en", "es").
type Text map[string]string

// Get returns the text in lang, falling back to pt, then any language.
func (t Text) Get(lang string) string {
	if s := t[lang]; s != "" {
		return s
	}
	if s := t["pt"]; s != "" {
		return s
	}
	for _, s := range t {
		return s
	}
	return ""
}

// Lines is localized multi-line text (client descriptions with ^RRGGBB codes).
type Lines map[string][]string

// Get returns the lines in lang with the same fallback as Text.
func (l Lines) Get(lang string) []string {
	if s := l[lang]; len(s) > 0 {
		return s
	}
	if s := l["pt"]; len(s) > 0 {
		return s
	}
	for _, s := range l {
		return s
	}
	return nil
}

// Item is one item: names/descriptions from the client, mechanics from rAthena.
type Item struct {
	ID       int    `json:"id"`
	Name     Text   `json:"name"`
	Desc     Lines  `json:"desc"`
	Slots    int    `json:"slots"`
	ClassNum int    `json:"classNum"`
	Resource string `json:"resource"`

	// From rAthena (empty when the item is not equipment or a card).
	Type        string   `json:"type,omitempty"`    // Weapon, Armor, Card, ShadowGear, ...
	SubType     string   `json:"subType,omitempty"` // weapon type: Dagger, 1hSword, ...
	Locations   []string `json:"loc,omitempty"`     // Head_Top, Armor, Right_Hand, ...
	Attack      int      `json:"atk,omitempty"`
	MagicAttack int      `json:"matk,omitempty"`
	Defense     int      `json:"def,omitempty"`
	WeaponLv    int      `json:"wlv,omitempty"`
	ArmorLv     int      `json:"alv,omitempty"`
	EquipLvMin  int      `json:"elv,omitempty"`
	EquipLvMax  int      `json:"elvMax,omitempty"`
	Refineable  bool     `json:"refine,omitempty"`
	Script      string   `json:"script,omitempty"`
	// Who can equip it (rAthena Jobs/Classes); empty means everyone.
	EquipJobs    []string `json:"jobs,omitempty"`
	EquipClasses []string `json:"classes,omitempty"`
}

// HasLocation reports whether the item can go in a slot location.
func (it *Item) HasLocation(loc string) bool {
	if it == nil {
		return false
	}
	for _, l := range it.Locations {
		if l == loc {
			return true
		}
	}
	return false
}

// Req is a skill prerequisite.
type Req struct {
	Skill int `json:"skill"`
	Level int `json:"level"`
}

// Skill is one skill from the client's skillinfolist.
type Skill struct {
	ID    int    `json:"id"`
	Code  string `json:"code"` // e.g. KN_BOWLINGBASH
	Name  Text   `json:"name"`
	Desc  Lines  `json:"desc"`
	MaxLv int    `json:"maxLv"`
	SP    []int  `json:"sp,omitempty"`
	Range []int  `json:"range,omitempty"`
	// Needs are the default prerequisites; NeedsByJob overrides them for
	// specific jobs (the client's NeedSkillList).
	Needs      []Req         `json:"needs,omitempty"`
	NeedsByJob map[int][]Req `json:"needsByJob,omitempty"`
	// Type from the client: "Quest" (learned by quest) or "Soul" (granted by
	// Soul Link). Such skills cost no skill points.
	Type string `json:"type,omitempty"`
}

// Free reports whether the skill costs no skill points.
func (s *Skill) Free() bool { return s.Type == "Quest" || s.Type == "Soul" }

// NeedsFor returns the prerequisites that apply to a job chain (closest job first).
func (s *Skill) NeedsFor(chain []int) []Req {
	for i := len(chain) - 1; i >= 0; i-- {
		if r, ok := s.NeedsByJob[chain[i]]; ok {
			return r
		}
	}
	return s.Needs
}

// Job is a playable class with its own skill tab.
type Job struct {
	ID     int    `json:"id"`
	Code   string `json:"code"` // e.g. JT_KNIGHT
	Name   Text   `json:"name"`
	Parent int    `json:"parent"` // -1 for none
	// Tree maps the client's skill-window grid position (row*7+col) to a skill ID.
	Tree map[int]int `json:"tree"`

	// From rAthena.
	MaxBaseLv int            `json:"maxBaseLv,omitempty"`
	MaxJobLv  int            `json:"maxJobLv,omitempty"`
	MaxWeight int            `json:"maxWeight,omitempty"`
	BaseHP    []int          `json:"baseHp,omitempty"` // index: base level - 1
	BaseSP    []int          `json:"baseSp,omitempty"`
	BaseAP    []int          `json:"baseAp,omitempty"`
	JobBonus  [][12]int      `json:"jobBonus,omitempty"` // index: job level; cumulative STR..LUK, POW..CRT
	BaseASPD  map[string]int `json:"baseAspd,omitempty"` // weapon type -> base delay
	Upper     bool           `json:"upper,omitempty"`    // MaxHP/SP x1.25 (transcendent, primary 4th)
	Fourth    bool           `json:"fourth,omitempty"`   // has trait stats
	Baby      bool           `json:"baby,omitempty"`
	// Equipment restrictions: rAthena job group (Knight, BardDancer, ...) and
	// class tier (Normal, Upper, Baby, Third, Third_Upper, Third_Baby, Fourth).
	EquipGroup string `json:"equipGroup,omitempty"`
	EquipTier  string `json:"equipTier,omitempty"`
}

// tierSets expands rAthena's grouped class flags (itemdb.hpp e_item_job).
var tierSets = map[string][]string{
	"All_Upper": {"Upper", "Third_Upper", "Fourth"},
	"All_Baby":  {"Baby", "Third_Baby"},
	"All_Third": {"Third", "Third_Upper", "Third_Baby"},
}

// CanEquip mirrors rAthena's renewal pc_isItemClass + job check. Jobs without
// imported restrictions (no EquipGroup) can equip anything.
func (j *Job) CanEquip(it *Item) bool {
	if j == nil || it == nil || j.EquipGroup == "" {
		return true
	}
	if len(it.EquipJobs) > 0 && !slices.Contains(it.EquipJobs, "All") && !slices.Contains(it.EquipJobs, j.EquipGroup) {
		return false
	}
	if len(it.EquipClasses) == 0 || slices.Contains(it.EquipClasses, "All") {
		return true
	}
	for _, c := range it.EquipClasses {
		if c == j.EquipTier || slices.Contains(tierSets[c], j.EquipTier) {
			return true
		}
	}
	return false
}

// HasStats reports whether rAthena job tables were imported for this job.
func (j *Job) HasStats() bool { return j.MaxBaseLv > 0 && len(j.BaseHP) > 0 }

// TreeColumns is the width of the client's skill window grid.
const TreeColumns = 7

// Catalog is all game data.
type Catalog struct {
	Generated time.Time     `json:"generated"`
	Source    string        `json:"source"`
	Items     map[int]*Item `json:"items"`
	// Total status/trait points at each base level (index: level - 1), from rAthena.
	StatPoints  []int `json:"statPoints,omitempty"`
	TraitPoints []int `json:"traitPoints,omitempty"`
	// Refine[group][equipLevel][refine] = cumulative bonus in 1/100 (rAthena refine.yml).
	Refine map[string]map[int][]int `json:"refine,omitempty"`
	Skills map[int]*Skill           `json:"skills"`
	Jobs   map[int]*Job             `json:"jobs"`
}

// SkillGroup is one class step's skill tabs and its skill point budget.
type SkillGroup struct {
	Jobs   []int `json:"jobs"`
	Budget int   `json:"budget"` // -1 = unknown (no job tables imported)
}

// SkillBudgets splits a job's chain into class steps with their skill point
// budgets (renewal): max job level - 1 for completed steps, current job
// level - 1 for the last one (jobLv <= 0 means max). A class and its
// transcendent version (KNIGHT, KNIGHT_H) share one step. Leftover points
// carry forward, so callers check cumulative sums per step.
func (c *Catalog) SkillBudgets(jobID, jobLv int) []SkillGroup {
	chain := c.Chain(jobID)
	var groups []SkillGroup
	for i, id := range chain {
		j := c.Jobs[id]
		if i > 0 && len(groups) > 0 {
			prev := c.Jobs[chain[i-1]]
			if j.Code == prev.Code+"_H" { // transcendent version: same step
				groups[len(groups)-1].Jobs = append(groups[len(groups)-1].Jobs, id)
				continue
			}
		}
		groups = append(groups, SkillGroup{Jobs: []int{id}})
	}
	for gi := range groups {
		last := c.Jobs[groups[gi].Jobs[len(groups[gi].Jobs)-1]]
		switch {
		case last.MaxJobLv == 0:
			groups[gi].Budget = -1
		case gi == len(groups)-1 && jobLv > 0:
			groups[gi].Budget = max(0, min(jobLv, last.MaxJobLv)-1)
		default:
			groups[gi].Budget = last.MaxJobLv - 1
		}
	}
	return groups
}

// Chain returns the job's ancestry from the root (Novice) to the job itself.
func (c *Catalog) Chain(jobID int) []int {
	var chain []int
	seen := map[int]bool{}
	for id := jobID; id >= 0 && !seen[id]; {
		j, ok := c.Jobs[id]
		if !ok {
			break
		}
		seen[id] = true
		chain = append([]int{id}, chain...)
		id = j.Parent
	}
	return chain
}

// Save writes the catalog as gzipped JSON.
func (c *Catalog) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	zw := gzip.NewWriter(f)
	if err := json.NewEncoder(zw).Encode(c); err != nil {
		f.Close()
		return err
	}
	if err := zw.Close(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// Load reads a catalog written by Save.
func Load(path string) (*Catalog, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("gamedata: %w", err)
	}
	var c Catalog
	if err := json.NewDecoder(zr).Decode(&c); err != nil {
		return nil, fmt.Errorf("gamedata: %w", err)
	}
	return &c, nil
}
