package view

import (
	"os"
	"strings"
	"testing"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

func testCatalog() *gamedata.Catalog {
	return &gamedata.Catalog{
		Items: map[int]*gamedata.Item{
			4001: {ID: 4001, Name: gamedata.Text{"pt": "Carta Poring", "en": "Poring Card"},
				Desc: gamedata.Lines{"pt": {"^0000ffSOR +2^000000"}}},
			1201: {ID: 1201, Name: gamedata.Text{"pt": "Faca"}, Slots: 3, Type: "Weapon", SubType: "Dagger",
				Locations: []string{"Right_Hand"}, Attack: 17, WeaponLv: 1, Refineable: true},
			4002: {ID: 4002, Name: gamedata.Text{"pt": "Carta Fabre"}, Type: "Card", Locations: []string{"Right_Hand"},
				Script: "bonus bVit,1;\nbonus bMaxHP,100;"},
		},
		Skills: map[int]*gamedata.Skill{
			1: {ID: 1, Name: gamedata.Text{"pt": "Habilidades Básicas"}, MaxLv: 9},
			5: {ID: 5, Name: gamedata.Text{"pt": "Golpe Fulminante"}, MaxLv: 10,
				Desc: gamedata.Lines{"pt": {"^777777Ataque^000000 <forte>"}}},
			62: {ID: 62, Name: gamedata.Text{"pt": "Impacto de Tyr"}, MaxLv: 10,
				Needs: []gamedata.Req{{Skill: 5, Level: 10}}},
		},
		StatPoints: []int{48, 51, 54},
		Refine:     map[string]map[int][]int{"Weapon": {1: {0, 200, 400, 600, 800, 1000, 1200, 1400}}},
		Jobs: map[int]*gamedata.Job{
			0: {ID: 0, Name: gamedata.Text{"pt": "Aprendiz"}, Parent: -1, Tree: map[int]int{0: 1},
				MaxBaseLv: 3, MaxJobLv: 10, BaseHP: []int{40, 46, 53}, BaseSP: []int{11, 12, 13},
				JobBonus: make([][12]int, 11), BaseASPD: map[string]int{"Fist": 40, "Dagger": 55}},
			1: {ID: 1, Name: gamedata.Text{"pt": "Espadachim"}, Parent: 0, Tree: map[int]int{0: 5}},
			7: {ID: 7, Name: gamedata.Text{"pt": "Cavaleiro"}, Parent: 1, Tree: map[int]int{8: 62}},
		},
	}
}

// newTestServer is the views' internals, for tests of unexported parts.
func newTestServer(o Options) (*Server, error) {
	v, err := New(o)
	if err != nil {
		return nil, err
	}
	return v.s, nil
}

func testViews(t *testing.T, c *gamedata.Catalog) *Views {
	t.Helper()
	v, err := New(Options{Catalog: c})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func names(items []ItemView) string {
	var s []string
	for _, it := range items {
		s = append(s, it.Name)
	}
	return strings.Join(s, ", ")
}

func TestSkillTree(t *testing.T) {
	v := testViews(t, testCatalog())
	tree, ok := v.SkillTree(7, 0, "pt")
	if !ok {
		t.Fatal("no tree for the Knight")
	}
	var found bool
	for _, tab := range tree.Tabs {
		for _, row := range tab.Rows {
			for _, c := range row {
				if c.Name == "Impacto de Tyr" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Error("the Knight's tab lacks Impacto de Tyr")
	}
	if n := tree.Skills[62].Needs; len(n) != 1 || n[0] != [2]int{5, 10} {
		t.Errorf("prerequisites: %v", n)
	}
	if _, ok := v.SkillTree(999, 0, "pt"); ok {
		t.Error("unknown job")
	}
}

func TestItemSearch(t *testing.T) {
	v := testViews(t, testCatalog())
	if got := names(v.SearchItems("poring card", "", "pt")); !strings.Contains(got, "Carta Poring") {
		t.Errorf("search: %s", got)
	}
	if got := names(v.SearchItems("poring", "", "en")); !strings.Contains(got, "Poring Card") {
		t.Errorf("English: %s", got)
	}
	if got := v.SearchItems("zzz", "", "pt"); len(got) != 0 {
		t.Errorf("nothing should match: %v", got)
	}
	if got := names(v.SearchItems("", "weapon", "pt")); got != "Faca" {
		t.Errorf("weapons: %s", got)
	}
	if got := names(v.SearchItems("carta", "card", "pt")); !strings.Contains(got, "Carta Fabre") {
		t.Errorf("cards: %s", got)
	}
	if got := names(v.SearchItems("faca", "card", "pt")); got != "" {
		t.Errorf("card filter returned %s", got)
	}
	if v.Item(999999, "pt") != nil || v.Item(4001, "pt").Desc[0] != "^0000ffSOR +2^000000" {
		t.Error("item lookup")
	}
}

func TestBuild(t *testing.T) {
	v := testViews(t, testCatalog())
	b := calc.Build{Job: 0, BaseLv: 3, JobLv: 1, Stats: [6]int{1, 1, 1, 1, 1, 1},
		Equip: map[string]calc.Equip{calc.Weapon: {Item: 1201, Refine: 7, Cards: [4]int{4002}}}}
	bv := v.Build(b, "pt")
	if bv.R.StatusATK != 2 || bv.R.EquipATK != 31 { // status ATK at Lv3 + (17 + 7*2)
		t.Errorf("ATK %d + %d, want 2 + 31", bv.R.StatusATK, bv.R.EquipATK)
	}
	var weapon SlotView
	for _, s := range bv.Slots {
		if s.Key == calc.Weapon {
			weapon = s
		}
	}
	if weapon.Item == nil || weapon.Item.Name != "Faca" || weapon.Refine != 7 || weapon.Cards[0] == nil || weapon.Cards[0].Name != "Carta Fabre" {
		t.Errorf("weapon slot: %+v", weapon)
	}
	var hp bool
	for _, l := range bv.Bonuses {
		hp = hp || l.Label == "HP Máx."
	}
	if !hp {
		t.Error("bonus window lacks the card's HP bonus")
	}
	// The share code restores the same build.
	d, ok := DecodeBuild(bv.Code)
	if !ok || d.Equip[calc.Weapon].Refine != 7 {
		t.Error("share code did not restore the build")
	}
	if _, ok := DecodeBuild("%%%"); ok {
		t.Error("broken share code accepted")
	}

	// The picker filters by slot: a weapon slot offers the knife, cards only for cards.
	if got, _ := v.PickItems(b, calc.Weapon, false, -1, "faca", "pt"); !strings.Contains(names(got), "Faca") {
		t.Error("weapon picker lacks the knife")
	}
	if got, _ := v.PickItems(b, calc.Armor, false, -1, "faca", "pt"); strings.Contains(names(got), "Faca") {
		t.Error("armor picker offers a weapon")
	}
	if got, _ := v.PickItems(b, calc.Weapon, true, 0, "carta", "pt"); !strings.Contains(names(got), "Carta Fabre") || strings.Contains(names(got), "Carta Poring") {
		t.Errorf("card picker: %s", names(got))
	}
}

func TestNewAndImport(t *testing.T) {
	c := testCatalog()
	c.Jobs[4252] = &gamedata.Job{ID: 4252, Name: gamedata.Text{"pt": "Cavaleiro Draconiano"}, Parent: 7,
		Tree: map[int]int{0: 62}, MaxBaseLv: 275, MaxJobLv: 60, BaseHP: make([]int, 275), BaseSP: make([]int, 275),
		JobBonus: make([][12]int, 61), BaseASPD: map[string]int{"Fist": 40}}
	c.Jobs[7].MaxBaseLv, c.Jobs[7].MaxJobLv = 99, 50
	c.Jobs[7].BaseHP, c.Jobs[7].BaseSP = make([]int, 99), make([]int, 99)
	c.Jobs[7].JobBonus = make([][12]int, 51)
	v := testViews(t, c)

	var listed bool
	for _, f := range v.ClassFamilies("pt", 0) {
		for _, j := range f.Jobs {
			listed = listed || j.ID == 4252
		}
	}
	if !listed {
		t.Error("the class list lacks the 4th class")
	}
	// A blank build for a chosen class starts at its max levels.
	if b := v.NewBuild(4252); b.BaseLv != 275 || b.JobLv != 60 {
		t.Errorf("blank build at %d/%d", b.BaseLv, b.JobLv)
	}

	rrf, err := os.ReadFile("../replay/testdata/synthetic.rrf")
	if err != nil {
		t.Fatal(err)
	}
	b, _, _, err := v.ImportReplay(rrf, "pt")
	if err != nil {
		t.Fatal(err)
	}
	if b.BaseLv != 250 || b.Equip[calc.Weapon].Refine != 9 || b.Equip[calc.Weapon].Item != 1201 {
		t.Errorf("imported build: base %d, weapon %+v", b.BaseLv, b.Equip[calc.Weapon])
	}
	if _, _, _, err := v.ImportReplay([]byte("not a replay"), "pt"); err == nil {
		t.Error("garbage accepted")
	}
}
