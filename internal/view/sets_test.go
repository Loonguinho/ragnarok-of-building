package view

import (
	"context"
	"math"
	"slices"
	"strings"
	"testing"

	"ragnarok-of-building/internal/calc"
)

func ambivalente(t *testing.T, v *Views, b calc.Build) []GearSet {
	t.Helper()
	var out []GearSet
	for _, g := range v.GearSets(b, "pt") {
		if strings.Contains(g.Name, "Coroa Ambivalente") {
			out = append(out, g)
		}
	}
	return out
}

// TestCompareSets: the Ambivalente sets of the Dragon Knight bring crown,
// weapon and boots; the whole set gives more than its pieces one by one; a
// column equipped gives the damage it shows.
func TestCompareSets(t *testing.T) {
	c, v := stressCatalog(t)
	b := calc.Build{Job: 4252, BaseLv: 250, JobLv: 50, Stats: [6]int{130, 100, 80, 1, 100, 100}, Traits: [6]int{100, 0, 0, 0, 97, 0},
		Target: 21542, Skills: map[int]int{}, Equip: map[string]calc.Equip{}}
	for _, id := range c.Chain(b.Job) {
		for _, sk := range c.Jobs[id].Tree {
			b.Skills[sk] = c.Skills[sk].MaxLv
		}
	}
	for _, sk := range c.Skills {
		if sk.Code == "DK_HACKANDSLASHER" {
			b.Atk = calc.AttackChoice{Skill: sk.ID, Level: sk.MaxLv}
		}
	}
	calc.Normalize(&b, c)
	sets := ambivalente(t, v, b)
	if len(sets) != 2 {
		for _, s := range sets {
			t.Log(s.Name)
		}
		t.Fatalf("%d Ambivalente sets, want 2", len(sets))
	}
	for _, g := range sets {
		var slots []string
		for _, p := range g.Pieces {
			slots = append(slots, p.Slot)
		}
		slices.Sort(slots)
		if !slices.Equal(slots, []string{calc.HeadTop, calc.Shoes, calc.Weapon}) && len(slots) != 3 {
			t.Errorf("%s: %v", g.Name, slots)
		}
	}
	// The one with the two-handed sword, at +9 [B].
	i := slices.IndexFunc(sets, func(g GearSet) bool {
		return slices.ContainsFunc(g.Pieces, func(p SetPiece) bool { return p.Item == 600041 })
	})
	if i < 0 {
		t.Fatal("no set with Execution Great Sword")
	}
	pieces := slices.Clone(sets[i].Pieces)
	for k := range pieces {
		pieces[k].Refine, pieces[k].Grade = 9, 3
	}
	before := jsonOf(b)
	best := v.BestSetEnchants(context.Background(), b, pieces)
	if jsonOf(b) != before {
		t.Fatal("the build changed")
	}
	score := func(p []SetPiece) float64 { return v.s.optExpected(v.SetBuild(b, p)) }
	if score(best) <= score(pieces) {
		t.Errorf("best enchants %v, without %v", score(best), score(pieces))
	}
	cards := map[string][4]int{}
	for _, p := range best {
		cards[p.Slot] = *p.Cards
	}
	if cards[calc.HeadTop][2] != 312021 || cards[calc.Shoes][2] != 312450 {
		t.Errorf("want Dom Infernal and its Essência: %v", cards)
	}
	// Every pair of Dom and Essência, with the rest as found.
	for _, dom := range []int{0, 312014, 312021} {
		for _, ess := range []int{0, 312449, 312450} {
			x := slices.Clone(best)
			for k := range x {
				cd := *x[k].Cards
				switch x[k].Slot {
				case calc.HeadTop:
					cd[2] = dom
				case calc.Shoes:
					cd[2] = ess
				}
				x[k].Cards = &cd
			}
			if score(x) > score(best)*1.000001 {
				t.Errorf("Dom %d + Essência %d beats the best found", dom, ess)
			}
		}
	}
	res := v.CompareSets(b, [][]SetPiece{nil, best}, "pt")
	if len(res.Cols) != 2 || len(res.Rows) == 0 || !res.Rows[0].Main {
		t.Fatalf("%+v", res)
	}
	nb := v.SetBuild(b, best)
	if want := fmtInt(int64(v.s.optExpected(nb) + 0.5)); res.Rows[0].Values[1] != want {
		t.Errorf("shows %s, equipped %s", res.Rows[0].Values[1], want)
	}
	if want := fmtInt(int64(v.s.optExpected(b) + 0.5)); res.Rows[0].Values[0] != want {
		t.Errorf("the current column shows %s, the build gives %s", res.Rows[0].Values[0], want)
	}
	// The order of the pieces does not matter.
	rev := slices.Clone(best)
	slices.Reverse(rev)
	if math.Abs(score(rev)-score(best)) > 1e-6 {
		t.Error("the order of the pieces changes the damage")
	}
	// A shield has no room beside the two-handed sword.
	sb := cloneBuild(b)
	for id, it := range c.Items {
		if calc.Fits(it, calc.Shield) && it.Type == "Armor" && c.Jobs[b.Job].CanEquip(it) {
			sb.Equip[calc.Shield] = calc.Equip{Item: id}
			break
		}
	}
	if r := v.CompareSets(sb, [][]SetPiece{nil, best}, "pt"); len(r.Cols[1].Left) != 1 {
		t.Errorf("the shield the sword takes off is not told: %v", r.Cols[1].Left)
	}
}

// TestGearSets: every 4th class has its Ambivalente sets, and every set
// listed goes on whole.
func TestGearSets(t *testing.T) {
	c, v := stressCatalog(t)
	total := 0
	for id, j := range c.Jobs {
		if !j.Fourth || len(j.Tree) == 0 {
			continue
		}
		b := calc.Build{Job: id, BaseLv: 250, JobLv: 50, Stats: [6]int{1, 1, 1, 1, 1, 1}, Equip: map[string]calc.Equip{}}
		calc.Normalize(&b, c)
		amb := ambivalente(t, v, b)
		want := 2
		if len(amb) < want {
			t.Errorf("%s: %d Ambivalente sets", j.Code, len(amb))
		}
		for _, g := range amb {
			if len(g.Pieces) != 3 {
				t.Errorf("%s: %s has %d pieces", j.Code, g.Name, len(g.Pieces))
			}
		}
		for _, g := range v.GearSets(b, "pt") {
			total++
			nb := v.SetBuild(b, g.Pieces)
			for _, p := range g.Pieces {
				if nb.Equip[p.Slot].Item != p.Item {
					t.Errorf("%s: %s: %s did not go on", j.Code, g.Name, p.Slot)
				}
			}
		}
	}
	t.Logf("%d sets", total)
}
