// Package gamedata holds the read-only game catalog (items, skills, jobs)
// loaded into memory at startup.
package gamedata

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// Item is one item from the client's iteminfo.
type Item struct {
	ID       int    `json:"id"`
	Name     Text   `json:"name"`
	Desc     Lines  `json:"desc"`
	Slots    int    `json:"slots"`
	ClassNum int    `json:"classNum"`
	Resource string `json:"resource"`
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
}

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
}

// TreeColumns is the width of the client's skill window grid.
const TreeColumns = 7

// Catalog is all game data.
type Catalog struct {
	Generated time.Time      `json:"generated"`
	Source    string         `json:"source"`
	Items     map[int]*Item  `json:"items"`
	Skills    map[int]*Skill `json:"skills"`
	Jobs      map[int]*Job   `json:"jobs"`
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
