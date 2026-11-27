package rathena

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"ragnarok-of-building/internal/gamedata"
)

// levelMs is a skill_db.yml time: one number for every level, or a list of
// {Level, Time}.
type levelMs map[int]int // level -> ms; key 0 = every level

func (t *levelMs) UnmarshalYAML(n *yaml.Node) error {
	*t = levelMs{}
	switch n.Kind {
	case yaml.ScalarNode:
		var v int
		if err := n.Decode(&v); err != nil {
			return err
		}
		(*t)[0] = v
	case yaml.SequenceNode:
		var list []struct {
			Level int `yaml:"Level"`
			Time  int `yaml:"Time"`
		}
		if err := n.Decode(&list); err != nil {
			return err
		}
		for _, e := range list {
			(*t)[e.Level] = e.Time
		}
	default:
		return fmt.Errorf("skill time: unexpected %v", n.Kind)
	}
	return nil
}

// at is the time at level lv: a listed level, or the last one listed
// below it (rAthena fills the levels a list leaves out with the previous).
func (t levelMs) at(lv int) int {
	if v, ok := t[0]; ok {
		return v
	}
	best, bestLv := 0, 0
	for l, v := range t {
		if l <= lv && l > bestLv {
			best, bestLv = v, l
		}
	}
	return best
}

// LoadSkillTimes reads each skill's cast, fixed cast, after-cast delay and
// cooldown per level from skill_db.yml, by skill code. Skills with none of
// them are left out.
func LoadSkillTimes(path string) (map[string]*gamedata.SkillTimes, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var doc struct {
		Body []struct {
			Name     string  `yaml:"Name"`
			MaxLevel int     `yaml:"MaxLevel"`
			Cast     levelMs `yaml:"CastTime"`
			Fixed    levelMs `yaml:"FixedCastTime"`
			After    levelMs `yaml:"AfterCastActDelay"`
			Cooldown levelMs `yaml:"Cooldown"`
		} `yaml:"Body"`
	}
	if err := yaml.NewDecoder(f).Decode(&doc); err != nil {
		return nil, err
	}
	out := map[string]*gamedata.SkillTimes{}
	for _, s := range doc.Body {
		if len(s.Cast)+len(s.Fixed)+len(s.After)+len(s.Cooldown) == 0 || s.MaxLevel <= 0 {
			continue
		}
		st := &gamedata.SkillTimes{}
		fill := func(t levelMs) []int {
			if len(t) == 0 {
				return nil
			}
			v := make([]int, s.MaxLevel)
			for lv := 1; lv <= s.MaxLevel; lv++ {
				v[lv-1] = t.at(lv)
			}
			return v
		}
		st.Cast, st.Fixed, st.After, st.Cooldown = fill(s.Cast), fill(s.Fixed), fill(s.After), fill(s.Cooldown)
		out[s.Name] = st
	}
	return out, nil
}
