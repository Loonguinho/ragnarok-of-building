package view

import (
	"context"
	"os"
	"testing"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

// TestBestCardCombo: Melhor carta also ranks a card with its set — Carta
// Cavaleiro Branco on the sword with Carta Cavaleira Khalitzburg placed on
// the shield — above the card alone.
func TestBestCardCombo(t *testing.T) {
	const path = "../../data/gen/catalog.json.gz"
	if _, err := os.Stat(path); err != nil {
		t.Skip("no generated catalog")
	}
	c, err := gamedata.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	const sword, shield, whiteKnight, khalitzburg = 1102, 2104, 4608, 4609
	if c.Items[sword] == nil || c.Items[shield] == nil || c.Items[whiteKnight] == nil || c.Items[khalitzburg] == nil {
		t.Skip("catalog without the test items")
	}
	v, err := New(Options{Catalog: c})
	if err != nil {
		t.Fatal(err)
	}
	b := calc.Build{Job: 4252, BaseLv: 250, JobLv: 50, Stats: [6]int{120, 100, 80, 1, 100, 1}, Target: 21067,
		Equip: map[string]calc.Equip{calc.Weapon: {Item: sword}, calc.Shield: {Item: shield}}}
	res := v.BestCards(context.Background(), b, calc.Weapon, 0, GoalDamage, "pt")
	var alone, set *BestItem
	for i := range res.Items {
		it := &res.Items[i]
		if it.Item.ID != whiteKnight {
			continue
		}
		if len(it.Combo) == 0 {
			alone = it
		} else {
			set = it
		}
	}
	if set == nil {
		t.Fatalf("no set row for Carta Cavaleiro Branco: %+v", res.Items)
	}
	if p := set.Combo; len(p) != 1 || p[0].Card != khalitzburg || p[0].Slot != calc.Shield {
		t.Errorf("set parts %+v, want Khalitzburg on the shield", p)
	}
	if alone != nil && set.Ratio <= alone.Ratio {
		t.Errorf("set %v not above the card alone %v", set.Ratio, alone.Ratio)
	}
	nb := v.ApplyCombo(b, calc.Weapon, 0, whiteKnight, set.Combo)
	if nb.Equip[calc.Weapon].Cards[0] != whiteKnight || nb.Equip[calc.Shield].Cards[0] != khalitzburg {
		t.Errorf("applied: %+v", nb.Equip)
	}
}
