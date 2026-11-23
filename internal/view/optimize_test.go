package view

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

func TestOptimizeCards(t *testing.T) {
	c := testCatalog()
	c.Items[4003] = &gamedata.Item{ID: 4003, Name: gamedata.Text{"pt": "Carta Forte"}, Type: "Card",
		Locations: []string{"Right_Hand"}, Script: "bonus bBaseAtk,200;"}
	c.Monsters = map[int]*gamedata.Monster{1002: {ID: 1002, Name: "Poring", Level: 1, HP: 5000, Size: "Medium",
		Race: "Plant", Element: "Water", ElementLv: 1, Class: "Normal", DamageTaken: 100}}
	// Enchants: the dagger takes 4100 in its 4th position; 4101 fits nothing.
	c.Items[4100] = &gamedata.Item{ID: 4100, Name: gamedata.Text{"pt": "Encanto Forte"}, Type: "Card", SubType: "Enchant", Script: "bonus bBaseAtk,50;"}
	c.Items[4101] = &gamedata.Item{ID: 4101, Name: gamedata.Text{"pt": "Encanto Solto"}, Type: "Card", SubType: "Enchant", Script: "bonus bBaseAtk,500;"}
	c.Enchants = map[int]map[int][]int{1201: {3: {4100}}}
	s, err := newTestServer(Options{Catalog: c})
	if err != nil {
		t.Fatal(err)
	}
	b := calc.Build{Job: 0, BaseLv: 3, JobLv: 1, Stats: [6]int{1, 1, 1, 1, 1, 1}, Target: 1002,
		Equip: map[string]calc.Equip{calc.Weapon: {Item: 1201}}}
	run := func(b calc.Build) optResult {
		calc.Normalize(&b, c)
		return s.optimize(context.Background(), b, true, "pt")
	}
	suggested := func(r optResult, card int) (n int, at []int) {
		for _, g := range r.Groups {
			for _, o := range g.Options {
				if o.Item.ID == card {
					n++
					at = append(at, o.Index)
				}
			}
		}
		return n, at
	}
	res := run(b)
	// Three card slots on the dagger, Carta Forte first in each.
	if n, _ := suggested(res, 4003); n != 3 {
		t.Errorf("Carta Forte suggested %d times, want 3: %+v", n, res.Groups)
	}
	if n, _ := suggested(res, 4002); n != 0 {
		t.Error("a card that adds no damage must not be suggested")
	}
	if n, at := suggested(res, 4100); n != 1 || at[0] != 3 {
		t.Errorf("enchants must follow the item's enchant list: %d at %v", n, at)
	}
	if n, _ := suggested(res, 4101); n != 0 {
		t.Error("an enchant the item's list lacks was suggested")
	}
	// The picker for that position lists only the listed enchant.
	if got := names(s.pickItems(calc.Weapon, true, 3, 1201, 0, "", "pt")); !strings.Contains(got, "Encanto Forte") || strings.Contains(got, "Encanto Solto") {
		t.Errorf("enchant picker not filtered: %s", got)
	}
	if got := names(s.pickItems(calc.Weapon, true, 0, 1201, 0, "", "pt")); !strings.Contains(got, "Carta Forte") {
		t.Errorf("card picker filtered by mistake: %s", got)
	}
	// Once every slot holds it, nothing is better.
	b.Equip[calc.Weapon] = calc.Equip{Item: 1201, Cards: [4]int{4003, 4003, 4003, 4100}}
	for _, g := range run(b).Groups {
		if len(g.Options) > 0 || !g.Best {
			t.Errorf("full build still gets suggestions: %+v", g)
		}
	}
	// No target: asks for one.
	b.Target = 0
	if !run(b).NoTarget {
		t.Error("no target not reported")
	}
	run(calc.Build{}) // must not panic
}

// TestOptimizeRealCatalog times a full run on a geared 4th class (needs the
// generated catalog).
func TestOptimizeRealCatalog(t *testing.T) {
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
	// Any slotted item for each of a few slots.
	eq := map[string]calc.Equip{}
	for _, slot := range []string{calc.Weapon, calc.Armor, calc.Garment, calc.Shoes, calc.AccRight, calc.AccLeft, calc.HeadTop, calc.Shield} {
		for _, it := range c.Items {
			if it.Slots >= 1 && calc.Fits(it, slot) && len(it.Locations) == 1 && (slot != calc.Weapon || it.SubType == "2hSword") {
				eq[slot] = calc.Equip{Item: it.ID}
				break
			}
		}
	}
	b := calc.Build{Job: 4252, BaseLv: 250, JobLv: 50, Stats: [6]int{120, 100, 80, 1, 100, 1}, Target: 1039, Equip: eq}
	start := time.Now()
	res := s.optimize(context.Background(), b, true, "pt")
	t.Logf("%d combinations in %v, %d groups, partial=%v", res.Tested, time.Since(start), len(res.Groups), res.Partial)
	if res.Partial || res.Tested == 0 || len(res.Groups) == 0 {
		t.Errorf("unexpected result: %+v", res)
	}
	for _, g := range res.Groups {
		if len(g.Options) > 0 {
			t.Logf("%s: %s %s", g.Label, g.Options[0].Item.Name, g.Options[0].Gain)
		}
	}
}

// Every enchant position stays listed: with the best enchant in it, as
// "already the best"; with only skill enchants that don't fit the attack,
// with that explanation. (A Vil Invernal weapon takes 3 enchants.)
func TestOptimizeKeepsEnchantPositions(t *testing.T) {
	const path = "../../data/gen/catalog.json.gz"
	if _, err := os.Stat(path); err != nil {
		t.Skip("no generated catalog")
	}
	c, err := gamedata.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	const sword, stormSlash, slashEnchant = 600030, 5213, 311275 // Espada de Duas Mãos Vil Invernal, Dilacerar (2H sword or axe), Cristal Invernal (Dilacerar)
	if c.Items[sword] == nil || len(c.Enchants[sword]) < 3 || c.Skills[stormSlash] == nil {
		t.Skip("catalog without the Vil Invernal data")
	}
	s, err := newTestServer(Options{Catalog: c})
	if err != nil {
		t.Fatal(err)
	}
	groups := func(b calc.Build) map[string]optGroup {
		out := map[string]optGroup{}
		for _, g := range s.optimize(context.Background(), b, true, "pt").Groups {
			if g.Enchant {
				out[g.Label] = g
			}
		}
		return out
	}
	b := calc.Build{Job: 4252, BaseLv: 250, JobLv: 50, Stats: [6]int{120, 100, 80, 1, 100, 1}, Target: 1039,
		Equip: map[string]calc.Equip{calc.Weapon: {Item: sword}}}
	// Basic attack: the skill enchants (positions 3 and 4) are listed, explained.
	g := groups(b)
	if len(g) != 3 || !g["Arma · Encantamento (slot 4)"].SkillOnly {
		t.Fatalf("basic attack: %+v", g)
	}
	// Basic attack with Storm Slash learned: positions 3 and 4 suggest its
	// enchant, measured on Storm Slash.
	b.Skills = map[int]int{stormSlash: 5}
	g = groups(b)
	for _, label := range []string{"Arma · Encantamento (slot 3)", "Arma · Encantamento (slot 4)"} {
		if o := g[label].Options; len(o) == 0 || o[0].Item.ID != slashEnchant || o[0].Skill == "" {
			t.Errorf("%s with Storm Slash learned: %+v", label, g[label])
		}
	}
	// Its enchant in position 4 under a basic attack: no swap to another
	// skill's enchant is suggested there.
	b.Equip[calc.Weapon] = calc.Equip{Item: sword, Cards: [4]int{0, 0, 0, slashEnchant}}
	if p4 := groups(b)["Arma · Encantamento (slot 4)"]; len(p4.Options) != 0 || p4.SkillHeld == "" {
		t.Errorf("held skill enchant: %+v", p4)
	}
	// Storm Slash with its enchant in position 4: still listed, as the best.
	b.Atk = calc.AttackChoice{Skill: stormSlash, Level: 5}
	b.Equip[calc.Weapon] = calc.Equip{Item: sword, Cards: [4]int{0, 0, 0, slashEnchant}}
	g = groups(b)
	if p4 := g["Arma · Encantamento (slot 4)"]; len(g) != 3 || !p4.Best || p4.SkillHeld != "" || len(p4.Options) != 0 {
		t.Fatalf("enchanted position: %+v", g)
	}
	if p3 := g["Arma · Encantamento (slot 3)"]; len(p3.Options) == 0 || p3.Options[0].Item.ID != slashEnchant {
		t.Errorf("position 3 should suggest the Storm Slash enchant: %+v", p3)
	}
}

func TestOptFamily(t *testing.T) {
	for in, want := range map[string]string{
		"Insígnia do Talento 10": "insígnia do talento", "Carta Poring": "carta poring",
		"Fúria II": "fúria", "Orbe Lupino - DEFM 1": "orbe lupino - defm", "Carta Ifrit": "carta ifrit",
	} {
		if got := optFamily(in); got != want {
			t.Errorf("optFamily(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMonsterLocalNames(t *testing.T) {
	c := &gamedata.Catalog{Monsters: map[int]*gamedata.Monster{
		1061: {ID: 1061, Name: "Nightmare", Names: gamedata.Text{"pt": "Pesadelo", "es": "Pesadilla"}},
		1002: {ID: 1002, Name: "Poring"},
	}}
	m := c.Monsters[1061]
	if m.NameIn("pt") != "Pesadelo" || m.NameIn("en") != "Nightmare" || m.NameIn("es") != "Pesadilla" || m.OtherName("pt") != "Nightmare" {
		t.Errorf("names: %q %q %q", m.NameIn("pt"), m.NameIn("en"), m.NameIn("es"))
	}
	for _, q := range []string{"pesadelo", "nightmare", "pesadilla"} {
		if r := c.SearchMonsters(q, 5); len(r) != 1 || r[0].ID != 1061 {
			t.Errorf("search %q: %v", q, r)
		}
	}
	if c.Monsters[1002].NameIn("pt") != "Poring" || c.Monsters[1002].OtherName("pt") != "" {
		t.Error("monster without a local name")
	}
}

func TestEffectiveHP(t *testing.T) {
	c := testCatalog()
	c.Items[2301] = &gamedata.Item{ID: 2301, Name: gamedata.Text{"pt": "Couraça"}, Type: "Armor", Locations: []string{"Armor"}, Defense: 500}
	c.Items[2302] = &gamedata.Item{ID: 2302, Name: gamedata.Text{"pt": "Manto Anti-Planta"}, Type: "Armor", Locations: []string{"Armor"},
		Script: "bonus2 bSubRace,RC_Plant,20;"}
	c.Monsters = map[int]*gamedata.Monster{1002: {ID: 1002, Name: "Poring", Level: 1, HP: 5000, Size: "Medium",
		Race: "Plant", Element: "Water", ElementLv: 1, Class: "Normal", DamageTaken: 100}}
	s, err := newTestServer(Options{Catalog: c})
	if err != nil {
		t.Fatal(err)
	}
	b := calc.Build{Job: 0, BaseLv: 3, JobLv: 1, Stats: [6]int{1, 1, 1, 1, 1, 1}, Target: 1002}
	calc.Normalize(&b, c)
	bare := s.effectiveHP(b, true)
	hp := float64(calc.Compute(b, c).HP)
	if bare <= 0 {
		t.Fatal("no effective HP")
	}
	with := func(id int) float64 {
		nb := cloneBuild(b)
		nb.Equip[calc.Armor] = calc.Equip{Item: id}
		calc.Normalize(&nb, c)
		return s.effectiveHP(nb, true)
	}
	// bROWiki: 500 hard DEF halves the damage.
	if got := with(2301) / float64(calc.Compute(func() calc.Build { nb := cloneBuild(b); nb.Equip[calc.Armor] = calc.Equip{Item: 2301}; return nb }(), c).HP); got < 1.99 || got > 2.01 {
		t.Errorf("500 DEF: effective HP / HP = %.3f, want 2 (bare %.0f, HP %.0f)", got, bare, hp)
	}
	if got := with(2302) / bare; got < 1.249 || got > 1.251 {
		t.Errorf("20%% vs the target's race: ×%.3f, want ×1.25", got)
	}

	// The optimizer with items: defense picks the 500 DEF armor; damage
	// suggests the better weapon at the chosen refine.
	c.Items[1202] = &gamedata.Item{ID: 1202, Name: gamedata.Text{"pt": "Faca Boa"}, Type: "Weapon", SubType: "Dagger",
		Locations: []string{"Right_Hand"}, Attack: 60, WeaponLv: 1, Refineable: true}
	b.Equip = map[string]calc.Equip{calc.Weapon: {Item: 1201}}
	best := func(r optResult, slot string) *optOption {
		for _, g := range r.Groups {
			if g.Kind == "item" && g.Slot == slot && len(g.Options) > 0 {
				return &g.Options[0]
			}
		}
		return nil
	}
	r := s.optimizeWith(context.Background(), b, OptOptions{Goal: GoalPhysDef, Items: true}, "pt")
	if o := best(r, calc.Armor); o == nil || o.Item.ID != 2301 {
		t.Errorf("defense: best armor %+v", o)
	}
	r = s.optimizeWith(context.Background(), b, OptOptions{Goal: GoalDamage, Items: true, Refine: 10}, "pt")
	if o := best(r, calc.Weapon); o == nil || o.Item.ID != 1202 || o.Refine != 10 || o.Kind != "item" {
		t.Errorf("damage: best weapon %+v", o)
	}
	if b.Equip[calc.Weapon].Refine != 0 || len(b.Equip) != 1 {
		t.Errorf("the caller's build changed: %+v", b.Equip)
	}
}
