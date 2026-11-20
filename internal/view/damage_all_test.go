package view

import (
	"os"
	"testing"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

// TestDamageWindowEverySkill computes the damage window for every skill with
// a LATAM formula, in every job that can learn it, and for each variant
// (needs the generated catalog; skipped without it).
func TestDamageWindowEverySkill(t *testing.T) {
	const path = "../../data/gen/catalog.json.gz"
	if _, err := os.Stat(path); err != nil {
		t.Skip("no generated catalog")
	}
	c, err := gamedata.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	v := testViews(t, c)
	target := 1039 // Baphomet
	rendered, formulas := 0, 0
	for jobID, j := range c.Jobs {
		if len(j.Tree) == 0 {
			continue
		}
		seen := map[int]bool{}
		for _, id := range c.Chain(jobID) {
			for _, skID := range c.Jobs[id].Tree {
				sk := c.Skills[skID]
				if sk == nil || seen[skID] {
					continue
				}
				seen[skID] = true
				f := calc.FormulaFor(c, jobID, sk.Code)
				if f == nil {
					continue
				}
				formulas++
				for vi := range f.Variants() {
					b := calc.Build{Job: jobID, BaseLv: 250, JobLv: 60, Stats: [6]int{50, 50, 50, 50, 50, 50},
						Skills: map[int]int{skID: sk.MaxLv}, Target: target,
						Atk: calc.AttackChoice{Skill: skID, Level: sk.MaxLv, Variant: vi, Stack: 3}}
					bv := v.Build(b, "pt")
					if bv.D.Target == nil {
						t.Fatalf("job %s skill %s variant %d: no target", j.Code, sk.Code, vi)
					}
					rendered++
				}
			}
		}
	}
	if formulas < 300 {
		t.Errorf("only %d job/skill pairs with formulas", formulas)
	}
	t.Logf("%d damage windows computed (%d job/skill pairs)", rendered, formulas)
}
