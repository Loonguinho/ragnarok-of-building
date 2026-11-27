package view

import (
	"os"
	"strings"
	"testing"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

// TestCardSets: Comparar conjuntos lists a piece with the card of its set,
// the card going in a free position of the worn item that takes it.
func TestCardSets(t *testing.T) {
	const path = "../../data/gen/catalog.json.gz"
	if _, err := os.Stat(path); err != nil {
		t.Skip("no generated catalog")
	}
	c, err := gamedata.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	v := testViews(t, c)
	b := v.DefaultBuild()
	b.Job, b.BaseLv = 4252, 250
	b.Equip = map[string]calc.Equip{calc.Shoes: {Item: 470071, Refine: 11}} // Bota Desconhecida FOR [1]
	var found *GearSet
	n := 0
	for _, s := range v.GearSets(b, "pt") {
		if s.Cards {
			n++
		}
		if strings.Contains(s.Name, "Bebê Lobo") && strings.Contains(s.Name, "Carta Fofinho") {
			s := s
			found = &s
		}
	}
	t.Logf("%d sets with cards", n)
	if found == nil {
		t.Fatal("no Bebê Lobo + Carta Fofinho set")
	}
	var shoes *SetPiece
	for i := range found.Pieces {
		if found.Pieces[i].Slot == calc.Shoes {
			shoes = &found.Pieces[i]
		}
	}
	if shoes == nil || shoes.Item != 470071 || shoes.Cards == nil || shoes.Cards[0] != 27152 {
		t.Fatalf("shoes piece %+v, want the worn boots with Carta Fofinho", shoes)
	}
	nb := v.SetBuild(b, found.Pieces)
	if nb.Equip[calc.HeadLow].Item != 410706 || nb.Equip[calc.Shoes].Cards[0] != 27152 || nb.Equip[calc.Shoes].Refine != 11 {
		t.Fatalf("set build: lower %d, shoes %+v", nb.Equip[calc.HeadLow].Item, nb.Equip[calc.Shoes])
	}
}
