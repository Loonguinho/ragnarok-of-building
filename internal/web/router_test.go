package web

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

func testCatalog() *gamedata.Catalog {
	return &gamedata.Catalog{
		Items: map[int]*gamedata.Item{
			4001: {ID: 4001, Name: gamedata.Text{"pt": "Carta Poring", "en": "Poring Card"},
				Desc: gamedata.Lines{"pt": {"^0000ffSOR +2^000000"}}},
			1201: {ID: 1201, Name: gamedata.Text{"pt": "Faca"}, Slots: 3},
		},
		Skills: map[int]*gamedata.Skill{
			1: {ID: 1, Name: gamedata.Text{"pt": "Habilidades Básicas"}, MaxLv: 9},
			5: {ID: 5, Name: gamedata.Text{"pt": "Golpe Fulminante"}, MaxLv: 10,
				Desc: gamedata.Lines{"pt": {"^777777Ataque^000000 <forte>"}}},
			62: {ID: 62, Name: gamedata.Text{"pt": "Impacto de Tyr"}, MaxLv: 10,
				Needs: []gamedata.Req{{Skill: 5, Level: 10}}},
		},
		Jobs: map[int]*gamedata.Job{
			0: {ID: 0, Name: gamedata.Text{"pt": "Aprendiz"}, Parent: -1, Tree: map[int]int{0: 1}},
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
		{"/", "Selecionar Servidor"},
		{"/uikit", "Atributos"},
		{"/healthz", "ok"},
		{"/static/css/ro-ui.css", "ro-window"},
		{"/static/js/rowindow.js", "ro.windows"},
		{"/static/js/skilltree.js", "SKILL_TREE"},
		{"/skills?job=7", "Impacto de Tyr"},
		{"/skills?job=7", `"needs":[[5,10]]`},
		{"/skills?job=7", `<span style="color:#777777">Ataque</span> &lt;forte&gt;`},
		{"/items", "2 itens no banco"},
		{"/items?q=faca", "Faca [3]"},
		{"/items/search?q=poring+card", "Carta Poring"},
		{"/items/search?q=poring&lang=en", "Poring Card"},
		{"/items/search?q=zzz", "Nenhum item encontrado"},
		{"/items/4001", `<span style="color:#0000ff">SOR +2</span>`},
		{"/items?id=4001", "win-iteminfo"},
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
