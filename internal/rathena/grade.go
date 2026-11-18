package rathena

import "path/filepath"

var gradeIndex = map[string]int{"None": 0, "D": 1, "C": 2, "B": 3, "A": 4}

// loadGrades reads enchantgrade.yml. rAthena applies the Bonus of the grade
// below the item's (EnchantgradeDatabase::findCurrentLevelInfo uses
// enchantgrade - 1), so the entry for "None" is what grade D gets.
func (db *DB) loadGrades(re string) error {
	var doc struct {
		Body []struct {
			Type   string `yaml:"Type"`
			Levels []struct {
				Level  int `yaml:"Level"`
				Grades []struct {
					Grade string `yaml:"Grade"`
					Bonus int    `yaml:"Bonus"`
				} `yaml:"Grades"`
			} `yaml:"Levels"`
		} `yaml:"Body"`
	}
	if err := readYAML(filepath.Join(re, "enchantgrade.yml"), &doc); err != nil {
		return err
	}
	db.GradeBonus = map[string]map[int][5]int{}
	for _, t := range doc.Body {
		if db.GradeBonus[t.Type] == nil {
			db.GradeBonus[t.Type] = map[int][5]int{}
		}
		for _, l := range t.Levels {
			var b [5]int
			for _, g := range l.Grades {
				if i, ok := gradeIndex[g.Grade]; ok && i < 4 {
					b[i+1] = g.Bonus
				}
			}
			db.GradeBonus[t.Type][l.Level] = b
		}
	}
	return nil
}
