package view

import (
	"context"
	"strings"
	"testing"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

func extraCatalog(t *testing.T) (*Server, *gamedata.Catalog, calc.Build) {
	t.Helper()
	c := testCatalog()
	c.Monsters = map[int]*gamedata.Monster{1002: {ID: 1002, Name: "Poring", Level: 1, HP: 5000, Size: "Medium",
		Race: "Plant", Element: "Water", ElementLv: 1, Class: "Normal", DamageTaken: 100}}
	ench := func(id int, name, script string, locs ...string) {
		c.Items[id] = &gamedata.Item{ID: id, Name: gamedata.Text{"pt": name}, Type: "Card", SubType: "Enchant", Script: script, Locations: locs}
	}
	for i := 1; i <= 3; i++ {
		ench(4100+i, "Cristal (Grau "+string(rune('0'+i))+")", "bonus bBaseAtk,"+string(rune('0'+i))+"0; if (getrefine()>=9) bonus bBaseAtk,100;")
	}
	ench(4110, "Cristal (Tardio)", "if (getrefine()>=9) bonus bBaseAtk,300;")
	c.Enchants = map[int]map[int][]int{1201: {3: {4101, 4102, 4103, 4110}}}
	// Random options: ATQ, in both positions.
	c.RandomOpts = map[int]*gamedata.RandomOpt{17: {ID: 17, Name: gamedata.Text{"pt": "ATQ +%d"}, Script: "bonus bBaseAtk,getrandomoptinfo(ROA_VALUE);"},
		18: {ID: 18, Name: gamedata.Text{"pt": "Precisão +%d"}, Script: "bonus bHit,getrandomoptinfo(ROA_VALUE);"}}
	roll := []gamedata.OptRange{{ID: 17, Min: 5, Max: 25}, {ID: 18, Min: 1, Max: 9}}
	c.ItemOpts = map[int]*gamedata.ItemOpts{1201: {Positions: [][]gamedata.OptRange{roll, roll}}}
	// Costumes: a top and a bottom one, a stone for each, and their set.
	for id, loc := range map[int]string{19500: calc.CosTop, 19502: calc.CosLow} {
		c.Items[id] = &gamedata.Item{ID: id, Name: gamedata.Text{"pt": "Visual"}, Type: "Armor", Locations: []string{loc}}
	}
	ench(29001, "Pedra (Topo)", "bonus bBaseAtk,5;", calc.CosTop)
	ench(29002, "Pedra (Baixo)", "bonus bBaseAtk,5;", calc.CosLow)
	c.Combos = append(c.Combos, gamedata.Combo{Items: []int{29001, 29002}, Script: "bonus bBaseAtk,200;"})
	s, err := newTestServer(Options{Catalog: c})
	if err != nil {
		t.Fatal(err)
	}
	b := calc.Build{Job: 0, BaseLv: 3, JobLv: 1, Stats: [6]int{1, 1, 1, 1, 1, 1}, Target: 1002,
		Equip: map[string]calc.Equip{calc.Weapon: {Item: 1201, Refine: 7}, calc.CosTop: {Item: 19500}, calc.CosLow: {Item: 19502}}}
	calc.Normalize(&b, c)
	return s, c, b
}

func TestOptimizerTiersAndSteps(t *testing.T) {
	s, _, b := extraCatalog(t)
	r := s.optimizeWith(context.Background(), b, OptOptions{Enchants: true}, "pt")
	if len(r.Groups) != 1 {
		t.Fatalf("groups: %+v", r.Groups)
	}
	opts := r.Groups[0].Options
	if len(opts) != 2 || opts[0].Item.ID != 4103 || opts[0].Later {
		t.Fatalf("options: %+v", opts)
	}
	if ts := opts[0].Tiers; len(ts) != 3 || ts[0].Item.ID != 4101 || ts[2].Item.ID != 4103 || gainPct(ts[0].Gain) >= gainPct(ts[2].Gain) {
		t.Errorf("tiers: %+v", ts)
	}
	if st := opts[0].Steps; len(st) == 0 || st[0].Refine != 9 || gainPct(st[0].Gain) <= gainPct(opts[0].Gain) {
		t.Errorf("steps: %+v (now %s)", st, opts[0].Gain)
	}
	// One that gives nothing yet, but does at +9.
	if o := opts[1]; o.Item.ID != 4110 || !o.Later || len(o.Steps) == 0 || o.Steps[0].Refine != 9 {
		t.Errorf("later option: %+v", o)
	}
	// Already at +9 there is no step left and it counts right away.
	b.Equip[calc.Weapon] = calc.Equip{Item: 1201, Refine: 9}
	r = s.optimizeWith(context.Background(), b, OptOptions{Enchants: true}, "pt")
	if o := r.Groups[0].Options[0]; o.Item.ID != 4110 || o.Later || len(o.Steps) != 0 {
		t.Errorf("at +9: %+v", o)
	}
}

func TestOptimizerRandomOptions(t *testing.T) {
	s, c, b := extraCatalog(t)
	v := &Views{s: s}
	group := func(b calc.Build, i int) *optGroup {
		r := s.optimizeWith(context.Background(), b, OptOptions{Opts: true}, "pt")
		for k := range r.Groups {
			if r.Groups[k].Kind == "opt" && r.Groups[k].Index == i {
				return &r.Groups[k]
			}
		}
		return nil
	}
	g := group(b, 0)
	if g == nil || len(g.Options) == 0 || g.Options[0].Opt != [2]int{17, 25} || g.Options[0].Name != "ATQ +25" {
		t.Fatalf("option 1: %+v", g)
	}
	if group(b, 1) != nil {
		t.Error("the second position is offered before the first is filled")
	}
	nb := v.SetOpt(b, calc.Weapon, 0, g.Options[0].Opt)
	if got := nb.Equip[calc.Weapon].Opts; len(got) != 1 || got[0] != [2]int{17, 25} {
		t.Errorf("applied: %v", got)
	}
	if len(b.Equip[calc.Weapon].Opts) != 0 {
		t.Error("the original build changed")
	}
	if group(nb, 1) == nil {
		t.Error("the second position is not offered")
	}
	// Melhor item: the best rolls count.
	c.Items[1202] = &gamedata.Item{ID: 1202, Name: gamedata.Text{"pt": "Faca Boa"}, Type: "Weapon", SubType: "Dagger", Locations: []string{"Right_Hand"}, Attack: 17, WeaponLv: 1}
	c.ItemOpts[1202] = c.ItemOpts[1201]
	gain := func(opts bool) (float64, string) {
		r := v.BestItemsOpts(context.Background(), b, calc.Weapon, 7, 0, opts, "pt")
		for _, it := range r.Items {
			if it.Item.ID == 1202 {
				return it.Ratio, it.OptsText
			}
		}
		return 0, ""
	}
	plain, t0 := gain(false)
	rolled, t1 := gain(true)
	if rolled <= plain || t0 != "" || t1 != "ATQ +25 · ATQ +25" {
		t.Errorf("Melhor item: %.3f %q, with rolls %.3f %q", plain, t0, rolled, t1)
	}
}

func TestOptimizerCostumes(t *testing.T) {
	s, _, b := extraCatalog(t)
	v := &Views{s: s}
	r := s.optimizeWith(context.Background(), b, OptOptions{Costumes: true}, "pt")
	var set *optOption
	for _, g := range r.Groups {
		if g.Kind == "set" {
			set = &g.Options[0]
		}
	}
	if set == nil || len(set.Parts) != 2 {
		t.Fatalf("no set: %+v", r.Groups)
	}
	if strings.Join(r.NoCostume, ",") != "Visual Meio,Visual Capa" {
		t.Errorf("empty costume slots: %v", r.NoCostume)
	}
	nb := v.ApplyParts(b, set.Parts)
	if nb.Equip[calc.CosTop].Cards[0] != 29001 || nb.Equip[calc.CosLow].Cards[0] != 29002 {
		t.Errorf("applied: %v", nb.Equip)
	}
	// Melhor carta completes the set too.
	best := v.BestCards(context.Background(), b, calc.CosTop, 0, GoalDamage, "pt")
	found := false
	for _, it := range best.Items {
		found = found || it.Item.ID == 29001 && len(it.Combo) == 1 && it.Combo[0].Card == 29002
	}
	if !found {
		t.Errorf("Melhor carta misses the stone set: %+v", best.Items)
	}
}

func TestOptFamilyTiers(t *testing.T) {
	for a, b := range map[string]string{
		"Cristal Invernal (Grau Físico 1)": "Cristal Invernal (Grau Físico 5)", "Orbe Lupino - Fatal 1": "Orbe Lupino - Fatal 4"} {
		if optFamily(a) != optFamily(b) {
			t.Errorf("%q and %q are one family", a, b)
		}
	}
	if optFamily("Cristal Invernal (Grau Físico 1)") == optFamily("Cristal Invernal (Grau Mágico 1)") {
		t.Error("physical and magic grades are two families")
	}
}

// TestOptimizerSkillCards: a card for the attack's skill is listed although
// five general cards beat it, and the sets of cards are tried whole.
func TestOptimizerSkillCards(t *testing.T) {
	s, c, b := extraCatalog(t)
	card := func(id int, name, script string) {
		c.Items[id] = &gamedata.Item{ID: id, Name: gamedata.Text{"pt": name}, Type: "Card", Locations: []string{"Right_Hand"}, Script: script}
	}
	for i := range 6 {
		card(4500+i, "Carta Geral "+string(rune('A'+i)), "bonus bBaseAtk,"+string(rune('3'+i))+"00;")
	}
	var code string
	for id, lv := range b.Skills {
		if lv > 0 {
			code, b.Atk.Skill, b.Atk.Level = c.Skills[id].Code, id, lv
			break
		}
	}
	if code == "" {
		c.Skills[5] = &gamedata.Skill{ID: 5, Code: "SM_BASH", Name: gamedata.Text{"pt": "Golpe Fulminante"}}
		code = "SM_BASH"
	}
	card(4510, "Carta da Skill", "bonus2 bSkillAtk,\""+code+"\",20;")
	card(4520, "Carta Par 1", "bonus bBaseAtk,1;")
	card(4521, "Carta Par 2", "bonus bBaseAtk,1;")
	c.Combos = append(c.Combos, gamedata.Combo{Items: []int{4520, 4521}, Script: "bonus bBaseAtk,5000;"})
	if !boostsSkill(c.Items[4510], code) || boostsSkill(c.Items[4500], code) {
		t.Fatal("boostsSkill")
	}
	// The set goes in two of the weapon's three slots.
	opts, _, _ := s.cardSets(context.Background(), b, OptOptions{Goal: GoalDamage, Cards: true}, "", "pt")
	if len(opts) == 0 || len(opts[0].Parts) != 2 || opts[0].Parts[0].Idx == opts[0].Parts[1].Idx {
		t.Fatalf("card set: %+v", opts)
	}
	nb := (&Views{s: s}).ApplyParts(b, opts[0].Parts)
	got := nb.Equip[calc.Weapon].Cards
	if !(got[0] == 4520 || got[1] == 4520 || got[2] == 4520) || !(got[0] == 4521 || got[1] == 4521 || got[2] == 4521) {
		t.Errorf("applied: %v", got)
	}
	// One that needs a slot the items do not have is named.
	c.Items[4530] = &gamedata.Item{ID: 4530, Name: gamedata.Text{"pt": "Carta de Capa"}, Type: "Card", Locations: []string{"Garment"}}
	c.Combos = append(c.Combos, gamedata.Combo{Items: []int{4510, 4530}, Script: "bonus bBaseAtk,1;"})
	c.Skills[99] = &gamedata.Skill{ID: 99, Code: code, Name: gamedata.Text{"pt": "X"}}
	sb := b
	sb.Atk.Skill = 99
	if _, _, missing := s.cardSets(context.Background(), sb, OptOptions{Goal: GoalDamage, Cards: true}, "conjunto de X", "pt"); len(missing) != 1 || !strings.Contains(missing[0], "Carta de Capa") {
		t.Errorf("missing slots: %v", missing)
	}
	// The skill's card, listed besides the best five.
	p := optPosition{slots: []string{calc.Weapon}, pool: []*gamedata.Item{c.Items[4500], c.Items[4510]}}
	out := s.skillCards(p, []float64{200, 110}, []float64{0, 0}, 100, nil, code, func(float64) string { return "+10%" }, "carta de X", "pt")
	if len(out) != 1 || out[0].Item.ID != 4510 || out[0].Tag != "carta de X" || out[0].Later {
		t.Errorf("skill cards: %+v", out)
	}
	if out := s.skillCards(p, []float64{200, 110}, []float64{0, 0}, 100, nil, "", func(float64) string { return "" }, "", "pt"); out != nil {
		t.Error("the basic attack has no skill cards")
	}
	// No gain now, but at a higher refine.
	if out := s.skillCards(p, []float64{200, 100}, []float64{0, 1.2}, 100, nil, code, func(float64) string { return "+0%" }, "x", "pt"); len(out) != 1 || !out[0].Later {
		t.Errorf("later skill card: %+v", out)
	}
}

// TestOptimizerMultiCards: the cards of an item with several slots are tried
// together, one card in all of them included.
func TestOptimizerMultiCards(t *testing.T) {
	s, c, b := extraCatalog(t)
	card := func(id int, name, script string) {
		c.Items[id] = &gamedata.Item{ID: id, Name: gamedata.Text{"pt": name}, Type: "Card", Locations: []string{"Right_Hand"}, Script: script}
	}
	card(4500, "Carta Forte", "bonus bBaseAtk,100;")
	card(4501, "Carta Média", "bonus bBaseAtk,50; if (getrefine()>=9) bonus bBaseAtk,300;")
	r := s.optimizeWith(context.Background(), b, OptOptions{Cards: true}, "pt")
	var g *optGroup
	for i := range r.Groups {
		if r.Groups[i].Kind == "set" && r.Groups[i].Slot == calc.Weapon {
			g = &r.Groups[i]
		}
	}
	if g == nil || g.Options[0].Name != "3× Carta Forte" || len(g.Options[0].Parts) != 3 {
		t.Fatalf("cards together: %+v", g)
	}
	nb := (&Views{s: s}).ApplyParts(b, g.Options[0].Parts)
	if nb.Equip[calc.Weapon].Cards != [4]int{4500, 4500, 4500, 0} {
		t.Errorf("applied: %v", nb.Equip[calc.Weapon].Cards)
	}
	found := false
	for _, o := range g.Options {
		if o.Name == "3× Carta Média" {
			found = len(o.Steps) > 0 && o.Steps[0].Refine == 9
		}
	}
	if !found {
		t.Errorf("three of the card that grows at +9, with its step: %+v", g.Options)
	}
}
