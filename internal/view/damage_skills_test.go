package view

import (
	"os"
	"testing"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

// TestDamageSkillsActiveOnly: the main skill list leaves the passives out.
func TestDamageSkillsActiveOnly(t *testing.T) {
	const path = "../../data/gen/catalog.json.gz"
	if _, err := os.Stat(path); err != nil {
		t.Skip("no generated catalog")
	}
	c, err := gamedata.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	s, err := newTestServer(Options{Catalog: c})
	if err != nil {
		t.Fatal(err)
	}
	b := calc.Build{Job: 4252, BaseLv: 250, JobLv: 50, Skills: map[int]int{}}
	passives := 0
	for _, id := range c.Jobs[4252].Tree {
		if sk := c.Skills[id]; sk != nil {
			b.Skills[id] = sk.MaxLv
			if sk.Passive {
				passives++
			}
		}
	}
	if passives == 0 {
		t.Skip("catalog without passive flags")
	}
	r := calc.Compute(b, c)
	for _, sk := range s.damageView(b, r, "pt").Skills {
		if c.Skills[sk.ID].Passive {
			t.Errorf("passive %s offered", sk.Name)
		}
	}
}
