package web

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/divinepride"
	"ragnarok-of-building/internal/gamedata"
)

// A stub (ID only in rAthena) is found by name and, once chosen, gets its
// stats from Divine Pride — here from the disk cache, so no network.
func TestStubMonsterFromDivinePride(t *testing.T) {
	cache := t.TempDir()
	dir := filepath.Join(cache, "LATAM", "pt", "Monster")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	const body = `{"id":20994,"name":"x","level":250,"health":2000000000,"race":"Dragon","element":"Neutral 2",
		"elementLevel":2,"size":"Large","type":"MVP","def":346,"mDef":102,"res":500,"mRes":500,
		"str":211,"agi":211,"vit":132,"int":152,"dex":279,"luk":108,"attributes":["Taking only 1% of the damage dealt to it"]}`
	if err := os.WriteFile(filepath.Join(dir, "20994.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c := testCatalog()
	c.Monsters = map[int]*gamedata.Monster{}
	c.MonsterStubs = map[int]string{20994: "MD_BETELGEUSE", 20995: "MD_G_DEADSOUL"}
	s, err := New(Options{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Catalog: c, NoRateLimit: true,
		DP: divinepride.New("", "LATAM", cache)})
	if err != nil {
		t.Fatal(err)
	}
	h := s.Routes()
	if _, html := get(t, h, "/build/monsters?q=betelgeuse"); !strings.Contains(html, `data-monster="20994"`) || !strings.Contains(html, "Betelgeuse (MD)") {
		t.Fatalf("stub not found by name: %s", html)
	}
	if note := s.loadTarget(20994, "pt"); note != "" {
		t.Fatal(note)
	}
	m := c.Monster(20994)
	if m == nil || m.HP != 2000000000 || !m.MVP || m.Class != "Boss" || m.DamageTaken != 1 || m.Element != "Neutral" || m.ElementLv != 2 || m.Race != "Dragon" {
		t.Fatalf("parsed monster: %+v", m)
	}
	b := calc.Build{Target: 20994}
	calc.Normalize(&b, c)
	if b.Target != 20994 {
		t.Error("loaded stub dropped as target")
	}
	// Without a Divine Pride key: a note, and it stays unknown (no network).
	s.opt.DP = nil
	if note := s.loadTarget(20995, "pt"); !strings.Contains(note, "DP_API_KEY") || c.Monster(20995) != nil {
		t.Errorf("failed fetch: %q", note)
	}
}

func TestStubName(t *testing.T) {
	for in, want := range map[string]string{"MD_BETELGEUSE": "Betelgeuse (MD)", "VH_BETELGEUSE": "Betelgeuse (VH)",
		"BETELGEUSE": "Betelgeuse", "E_PORING_20": "Poring 20 (E)"} {
		if got := gamedata.StubName(in); got != want {
			t.Errorf("StubName(%q) = %q, want %q", in, got, want)
		}
	}
}

// Random options: only items that can get them, only the options each
// position rolls, with their value range.
func TestItemRandomOptions(t *testing.T) {
	c := testCatalog()
	c.RandomOpts = map[int]*gamedata.RandomOpt{
		1:  {ID: 1, Name: gamedata.Text{"pt": "HP máx. +%d"}, Script: "bonus bMaxHP,getrandomoptinfo(ROA_VALUE);"},
		17: {ID: 17, Name: gamedata.Text{"pt": "ATQ +%d"}, Script: "bonus bAtk,getrandomoptinfo(ROA_VALUE);"},
	}
	c.Items[2301] = &gamedata.Item{ID: 2301, Name: gamedata.Text{"pt": "Túnica"}, Type: "Armor", Locations: []string{"Armor"}, Defense: 10}
	c.ItemOpts = map[int]*gamedata.ItemOpts{2301: {Positions: [][]gamedata.OptRange{{{ID: 1, Min: 100, Max: 1000}}}}}
	s, err := New(Options{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Catalog: c, NoRateLimit: true})
	if err != nil {
		t.Fatal(err)
	}
	h := s.Routes()
	_, body := get(t, h, "/build/randomopts?lang=pt&item=2301&pos=0")
	if !strings.Contains(body, `data-min="100" data-max="1000"`) || !strings.Contains(body, "HP máx. +N (100–1000)") || strings.Contains(body, "ATQ") {
		t.Errorf("position 0 of 2301: %s", body)
	}
	if _, body := get(t, h, "/build/randomopts?lang=pt&item=1201&pos=0"); !strings.Contains(body, "ATQ +N") {
		t.Errorf("unknown item lists every option: %s", body)
	}
	v := s.buildView(calc.Build{Job: 0, BaseLv: 3, JobLv: 1, Stats: [6]int{1, 1, 1, 1, 1, 1},
		Equip: map[string]calc.Equip{calc.Armor: {Item: 2301}, calc.Weapon: {Item: 1201}}}, "pt")
	for _, sv := range v.Slots {
		switch sv.Key {
		case calc.Armor:
			if sv.MaxOpts != 1 {
				t.Errorf("armor takes 1 option, got %d", sv.MaxOpts)
			}
		case calc.Weapon:
			if sv.MaxOpts != 0 {
				t.Errorf("weapon takes none, got %d", sv.MaxOpts)
			}
		}
	}
}
