package rathena

import (
	"os"
	"slices"

	"gopkg.in/yaml.v3"
)

// loadSkillWeapons reads, from skill_db.yml (optional: older downloads lack
// it), the weapon types each skill requires (Requires: Weapon), by skill
// code (a skill without the field takes any weapon), and which skills are
// passive (no TargetType).
func (db *DB) loadSkillWeapons(path string) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	defer f.Close()
	var doc struct {
		Body []struct {
			Name       string `yaml:"Name"`
			TargetType string `yaml:"TargetType"`
			Requires   struct {
				Weapon map[string]bool `yaml:"Weapon"`
			} `yaml:"Requires"`
		} `yaml:"Body"`
	}
	if err := yaml.NewDecoder(f).Decode(&doc); err != nil {
		return err
	}
	db.SkillWeapons = map[string][]string{}
	db.SkillPassive = map[string]bool{}
	for _, s := range doc.Body {
		if s.TargetType == "" || s.TargetType == "Passive" { // passives have none
			db.SkillPassive[s.Name] = true
		}
		var ws []string
		for w, on := range s.Requires.Weapon {
			if on {
				ws = append(ws, w)
			}
		}
		if len(ws) > 0 {
			slices.Sort(ws)
			db.SkillWeapons[s.Name] = ws
		}
	}
	return nil
}
