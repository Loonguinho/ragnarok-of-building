package web

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
	s, err := New(Options{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Catalog: c, NoRateLimit: true})
	if err != nil {
		t.Fatal(err)
	}
	h := s.Routes()
	b := calc.Build{Job: 0, BaseLv: 3, JobLv: 1, Stats: [6]int{1, 1, 1, 1, 1, 1}, Target: 1002,
		Equip: map[string]calc.Equip{calc.Weapon: {Item: 1201}}}
	post := func(b calc.Build, l string) (int, string) {
		raw, _ := json.Marshal(map[string]any{"b": b, "enc": true})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/build/optimize?lang="+l, strings.NewReader(string(raw))))
		return rec.Code, rec.Body.String()
	}
	code, html := post(b, "pt")
	if code != 200 {
		t.Fatalf("optimize: %d %s", code, html)
	}
	// Three card slots on the dagger, Carta Forte first in each.
	if n := strings.Count(html, `data-opt-card="4003"`); n != 3 {
		t.Errorf("Carta Forte suggested %d times, want 3:\n%s", n, html)
	}
	if strings.Contains(html, `data-opt-card="4002"`) {
		t.Error("a card that adds no damage must not be suggested")
	}
	if !strings.Contains(html, `data-opt-index="3" data-opt-card="4100"`) || strings.Contains(html, `data-opt-card="4101"`) {
		t.Errorf("enchants must follow the item's enchant list: %s", html)
	}
	// The picker for that position lists only the listed enchant.
	if _, body := get(t, h, "/build/items?slot=Right_Hand&card=1&idx=3&item=1201"); !strings.Contains(body, "Encanto Forte") || strings.Contains(body, "Encanto Solto") {
		t.Errorf("enchant picker not filtered: %s", body)
	}
	if _, body := get(t, h, "/build/items?slot=Right_Hand&card=1&idx=0&item=1201"); !strings.Contains(body, "Carta Forte") {
		t.Errorf("card picker filtered by mistake: %s", body)
	}
	// Once every slot holds it, nothing is better.
	b.Equip[calc.Weapon] = calc.Equip{Item: 1201, Cards: [4]int{4003, 4003, 4003, 4100}}
	if _, html := post(b, "pt"); strings.Contains(html, "data-opt-apply") || !strings.Contains(html, "Já é a melhor opção") {
		t.Errorf("full build still gets suggestions:\n%s", html)
	}
	// No target: asks for one.
	b.Target = 0
	if _, html := post(b, "en"); !strings.Contains(html, "Choose a target") {
		t.Errorf("no target message missing:\n%s", html)
	}
	if code, _ := post(calc.Build{}, "pt"); code != 200 {
		t.Errorf("empty build: %d", code)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/build/optimize", strings.NewReader("{")))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad body: %d", rec.Code)
	}
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
	s, err := New(Options{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Catalog: c, NoRateLimit: true})
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
		t.Logf("%s: %s %s", g.Label, g.Options[0].Item.Name, g.Options[0].Gain)
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
