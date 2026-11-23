package view

import (
	"context"
	"os"
	"testing"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

// TestCompareTraits: on a Túnica Nebulosa, CRT brings Constelação da
// Criatividade and CON brings Concentração; the points are kept, the best
// found is at least as good as every pair, and applying a column gives its
// traits and enchants.
func TestCompareTraits(t *testing.T) {
	const path = "../../data/gen/catalog.json.gz"
	if _, err := os.Stat(path); err != nil {
		t.Skip("no generated catalog")
	}
	c, err := gamedata.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	const tunic, creativity, bow = 450172, 310683, 700034
	if c.Items[tunic] == nil || len(c.Enchants[tunic][1]) == 0 || c.Items[bow] == nil {
		t.Skip("catalog without the Nébula gear")
	}
	v, err := New(Options{Catalog: c})
	if err != nil {
		t.Fatal(err)
	}
	job := 0
	for id, j := range c.Jobs {
		if j.Code == "JT_WINDHAWK" {
			job = id
		}
	}
	b := calc.Build{Job: job, BaseLv: 250, JobLv: 50, Stats: [6]int{1, 100, 1, 1, 120, 100}, Traits: [6]int{100, 0, 0, 0, 0, 97}, Target: 21067,
		Equip: map[string]calc.Equip{calc.Weapon: {Item: bow, Refine: 12}, calc.Armor: {Item: tunic, Refine: 12, Grade: 4, Cards: [4]int{0, creativity}}}}
	r := v.CompareTraits(context.Background(), b, TraitOptions{}, "pt")
	if len(r.Cols) < 3 || r.Budget != 197 {
		t.Fatalf("columns %d, budget %d", len(r.Cols), r.Budget)
	}
	best := r.Cols[len(r.Cols)-1]
	names := map[string]bool{}
	for i, col := range r.Cols {
		sum := 0
		for _, x := range col.Traits {
			sum += x
		}
		if sum != 197 {
			t.Errorf("%s spends %d", col.Title, sum)
		}
		if col.Ratio > best.Ratio+1e-9 {
			t.Errorf("%s (%v) beats the best found (%v)", col.Title, col.Ratio, best.Ratio)
		}
		if i == 0 && col.Ratio != 1 {
			t.Errorf("current ratio %v", col.Ratio)
		}
		for _, e := range col.Enchants {
			names[e.Name] = true
			if enchLevel(c.Items[e.Card]) != 1 {
				t.Errorf("%s: %s is another level", col.Title, e.Name)
			}
		}
	}
	if len(names) < 2 {
		t.Errorf("the enchant never changes with the traits: %v", names)
	}
	nb := v.ApplyTraits(b, best.Traits, best.Enchants)
	if nb.Traits != best.Traits || nb.Equip[calc.Armor].Cards[1] != best.Enchants[0].Card {
		t.Errorf("applied %v %v, want %v %v", nb.Traits, nb.Equip[calc.Armor].Cards, best.Traits, best.Enchants)
	}
	if b.Equip[calc.Armor].Cards[1] != creativity {
		t.Error("the comparison changed the caller's build")
	}
}

// TestBestCardTraits: Melhor carta ranks a trait enchant with its traits too.
func TestBestCardTraits(t *testing.T) {
	const path = "../../data/gen/catalog.json.gz"
	if _, err := os.Stat(path); err != nil {
		t.Skip("no generated catalog")
	}
	c, err := gamedata.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	v, err := New(Options{Catalog: c})
	if err != nil {
		t.Fatal(err)
	}
	b := calc.Build{Job: 4252, BaseLv: 250, JobLv: 50, Stats: [6]int{120, 100, 1, 1, 100, 1}, Traits: [6]int{0, 100, 97, 0, 0, 0}, Target: 21067,
		Equip: map[string]calc.Equip{calc.Weapon: {Item: 1102}, calc.Armor: {Item: 450172, Refine: 12, Grade: 4}}}
	plain := v.BestCards(context.Background(), b, calc.Armor, 1, GoalDamage, "pt")
	for _, bi := range plain.Items {
		if bi.Traits != nil {
			t.Fatal("traits rows without asking")
		}
	}
	got := v.BestCardsTraits(context.Background(), b, calc.Armor, 1, GoalDamage, true, "pt")
	found := false
	for i, bi := range got.Items {
		found = found || bi.Traits != nil
		if i > 0 && bi.Ratio > got.Items[i-1].Ratio+1e-9 {
			t.Errorf("row %d out of order", i)
		}
	}
	if !found {
		t.Error("no row with the traits changed")
	}
}

// TestCompareTraitsOwnPair: the build's own traits are a column when better
// enchants of any level exist for them, so every column is compared at its
// best.
func TestCompareTraitsOwnPair(t *testing.T) {
	const path = "../../data/gen/catalog.json.gz"
	if _, err := os.Stat(path); err != nil {
		t.Skip("no generated catalog")
	}
	c, err := gamedata.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	v, err := New(Options{Catalog: c})
	if err != nil {
		t.Fatal(err)
	}
	b := calc.Build{Job: 4252, BaseLv: 250, JobLv: 50, Stats: [6]int{120, 100, 1, 1, 100, 1}, Traits: [6]int{100, 0, 0, 0, 97, 0}, Target: 21067,
		Equip: map[string]calc.Equip{calc.Weapon: {Item: 1102}, calc.Armor: {Item: 450172, Refine: 12, Grade: 4, Cards: [4]int{0, 310674}}}}
	r := v.CompareTraits(context.Background(), b, TraitOptions{AnyLevel: true}, "pt")
	for _, col := range r.Cols[1:] {
		if col.Traits == b.Traits && len(col.Enchants) > 0 && col.Enchants[0].Changed && col.Ratio > 1 {
			return
		}
	}
	t.Errorf("no column with the build's own traits and better enchants: %+v", r.Cols)
}

// TestTraitBudget: the points are the level's (recorded characters: 250 ->
// 197, 249 -> 190, 247 -> 184, 241 -> 162, 240 -> 159), and an overspent
// build does not give the columns more; Decorar Carrinho costs no point.
func TestTraitBudget(t *testing.T) {
	const path = "../../data/gen/catalog.json.gz"
	if _, err := os.Stat(path); err != nil {
		t.Skip("no generated catalog")
	}
	c, err := gamedata.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for lv, want := range map[int]int{250: 197, 249: 190, 247: 184, 241: 162, 240: 159} {
		b := calc.Build{Job: 4252, BaseLv: lv, JobLv: 50, Stats: [6]int{1, 1, 1, 1, 1, 1}, Traits: [6]int{100, 0, 0, 0, 0, 97}}
		if got := traitBudget(b, calc.Compute(b, c)); got != want {
			t.Errorf("level %d: %d points, want %d", lv, got, want)
		}
	}
	if sk := c.Skills[2544]; sk != nil && !sk.Free() {
		t.Errorf("%s costs a skill point", sk.Name.Get("pt"))
	}
}
