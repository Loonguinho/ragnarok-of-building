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
