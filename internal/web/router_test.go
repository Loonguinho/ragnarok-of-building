package web

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

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

func get(t *testing.T, h http.Handler, path string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec.Code, rec.Body.String()
}

func TestRoutes(t *testing.T) {
	s, err := New(Options{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Catalog: testCatalog()})
	if err != nil {
		t.Fatal(err)
	}
	h := s.Routes()
	cases := []struct{ path, want string }{
		{"/", "Personagens"},
		{"/uikit", "Atributos"},
		{"/healthz", "ok"},
		{"/static/css/ro-ui.css", "ro-window"},
		{"/static/js/rowindow.js", "ro.windows"},
		{"/static/js/skilltree.js", "SKILL_TREE"},
		{"/skills?job=7", "Impacto de Tyr"},
		{"/skills?job=7", `"needs":[[5,10]]`},
		{"/skills?job=7", `<span style="color:#777777">Ataque</span> &lt;forte&gt;`},
		{"/items", "3 itens no banco"},
		{"/items?q=faca", "Faca [3]"},
		{"/items/search?q=poring+card", "Carta Poring"},
		{"/items/search?q=poring&lang=en", "Poring Card"},
		{"/items/search?q=zzz", "Nenhum item encontrado"},
		{"/items/4001", `<span style="color:#0000ff">SOR +2</span>`},
		{"/items/4001/tip", `<span style="color:#0000ff">SOR +2</span>`},
		{"/items?id=4001", "win-iteminfo"},
		{"/items/search?type=weapon", "Faca [3]"},
		{"/items/search?type=card&q=carta", "Carta Fabre"},
	}
	for _, c := range cases {
		code, body := get(t, h, c.path)
		if code != http.StatusOK {
			t.Errorf("%s: status %d", c.path, code)
			continue
		}
		if !strings.Contains(body, c.want) {
			t.Errorf("%s: body missing %q", c.path, c.want)
		}
	}
}

func TestBuild(t *testing.T) {
	s, err := New(Options{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Catalog: testCatalog()})
	if err != nil {
		t.Fatal(err)
	}
	h := s.Routes()

	code, body := get(t, h, "/build")
	if code != http.StatusOK || !strings.Contains(body, "win-status") || !strings.Contains(body, "Aprendiz") {
		t.Fatalf("GET /build: %d", code)
	}

	post := func(body string) (int, map[string]any) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/build/calc", strings.NewReader(body)))
		var out map[string]any
		json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	code, out := post(`{"j":0,"b":3,"jl":1,"s":[1,1,1,1,1,1],"e":{"Right_Hand":{"i":1201,"r":7,"c":[4002,0,0,0]}}}`)
	if code != http.StatusOK {
		t.Fatalf("calc: %d", code)
	}
	html := out["html"].(map[string]any)
	if st := html["build_status"].(string); !strings.Contains(st, "2 + 31") { // status ATK at Lv3 + (17 + 7*2)
		t.Errorf("status window lacks ATK 2 + 31:\n%s", st)
	}
	if eq := html["build_equip"].(string); !strings.Contains(eq, "+7 Faca [3]") || !strings.Contains(eq, "Carta Fabre") {
		t.Error("equipment window lacks the refined weapon and its card")
	}
	if bn := html["build_bonus"].(string); !strings.Contains(bn, "HP Máx.") {
		t.Error("bonus window lacks the card's HP bonus")
	}

	// The share code restores the same build.
	share := out["code"].(string)
	if _, body := get(t, h, "/build?b="+share); !strings.Contains(body, "+7 Faca [3]") {
		t.Error("share link did not restore the build")
	}

	// Picker filters by slot: a weapon slot offers the knife, cards only with card=1.
	if _, body := get(t, h, "/build/items?slot=Right_Hand&q=faca"); !strings.Contains(body, "Faca") {
		t.Error("weapon picker lacks the knife")
	}
	if _, body := get(t, h, "/build/items?slot=Armor&q=faca"); strings.Contains(body, "Faca") {
		t.Error("armor picker offers a weapon")
	}
	if _, body := get(t, h, "/build/items?slot=Right_Hand&card=1&q=carta"); !strings.Contains(body, "Carta Fabre") || strings.Contains(body, "Carta Poring") {
		t.Error("card picker filter wrong")
	}

	if code, _ := post(`not json`); code != http.StatusBadRequest {
		t.Errorf("bad JSON: %d", code)
	}
	if code, _ := get(t, h, "/build?b=%%%"); code != http.StatusOK {
		t.Errorf("broken share code should fall back to a default build, got %d", code)
	}
}

func TestItemTypeFilterExcludes(t *testing.T) {
	s, _ := New(Options{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Catalog: testCatalog()})
	_, body := get(t, s.Routes(), "/items/search?type=card&q=faca")
	if strings.Contains(body, "Faca") {
		t.Error("card filter returned a weapon")
	}
	if _, body := get(t, s.Routes(), "/items/search"); !strings.Contains(body, "escolha um tipo") {
		t.Error("empty search should show the hint")
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
	s, _ := New(Options{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Catalog: c})
	h := s.Routes()

	if code, body := get(t, h, "/new"); code != 200 || !strings.Contains(body, "Importar Replay") || !strings.Contains(body, "job=4252") {
		t.Errorf("/new: %d", code)
	}
	// Without ?job the default build is kept (regression: job "" parsed as Novice).
	if _, body := get(t, h, "/build"); strings.Contains(body, "Base Lv. 3 ") {
		t.Error("/build without ?job became a Novice")
	}
	// Blank build for a chosen class starts at its max levels.
	if _, body := get(t, h, "/build?new=1&job=4252"); !strings.Contains(body, "Base Lv. 275") {
		t.Error("blank build did not start at max base level")
	}

	upload := func(name string, content []byte) (int, map[string]any) {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, _ := mw.CreateFormFile("file", name)
		fw.Write(content)
		mw.Close()
		req := httptest.NewRequest(http.MethodPost, "/build/import", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var out map[string]any
		json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	rrf, err := os.ReadFile("../replay/testdata/synthetic.rrf")
	if err != nil {
		t.Fatal(err)
	}
	code, out := upload("teste.rrf", rrf)
	if code != 200 || out["code"] == nil {
		t.Fatalf("import: %d %v", code, out)
	}
	if _, body := get(t, h, "/build?b="+out["code"].(string)); !strings.Contains(body, "+9 Faca [3]") || !strings.Contains(body, "Base Lv. 250") {
		t.Error("imported build lacks the +9 weapon or the level")
	}
	if code, out := upload("x.rrf", []byte("not a replay")); code != http.StatusUnprocessableEntity || out["error"] == nil {
		t.Errorf("garbage upload: %d %v", code, out)
	}
}

func TestItemNotFound(t *testing.T) {
	s, _ := New(Options{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Catalog: testCatalog()})
	if code, _ := get(t, s.Routes(), "/items/999999"); code != http.StatusNotFound {
		t.Errorf("status %d, want 404", code)
	}
}

func TestSkillsWithoutCatalog(t *testing.T) {
	s, err := New(Options{Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	code, body := get(t, s.Routes(), "/skills")
	if code != http.StatusOK || !strings.Contains(body, "cmd/import") {
		t.Errorf("status %d, body lacks import hint", code)
	}
}

func TestRoText(t *testing.T) {
	got := string(roText([]string{"^ff0000Fogo^000000 normal", "<b>"}))
	want := `<span style="color:#ff0000">Fogo</span> normal<br>&lt;b&gt;`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
