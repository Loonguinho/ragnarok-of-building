package rathena

import "path/filepath"

// RandomOpt is one random option ("bônus aleatório") from item_randomopt_db.yml:
// its client variable (VAR_MAXHPAMOUNT...) and the script that applies it,
// reading the rolled value with getrandomoptinfo(ROA_VALUE).
type RandomOpt struct {
	ID     int
	Var    string
	Script string
}

func (db *DB) loadRandomOpts(re string) error {
	var doc struct {
		Body []struct {
			ID     int    `yaml:"Id"`
			Option string `yaml:"Option"`
			Script string `yaml:"Script"`
		} `yaml:"Body"`
	}
	if err := readYAML(filepath.Join(re, "item_randomopt_db.yml"), &doc); err != nil {
		return err
	}
	db.RandomOpts = map[int]*RandomOpt{}
	for _, e := range doc.Body {
		if e.ID > 0 && e.Script != "" {
			db.RandomOpts[e.ID] = &RandomOpt{ID: e.ID, Var: e.Option, Script: e.Script}
		}
	}
	return nil
}
