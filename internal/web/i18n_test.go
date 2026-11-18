package web

import (
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"ragnarok-of-building/internal/gamedata"
	assets "ragnarok-of-building/web"
)

// Every text marked for translation in the templates and scripts must be in
// the dictionary with both an English and a Spanish version.
func TestTemplatesTranslated(t *testing.T) {
	tmplKey := regexp.MustCompile("\\{\\{\\s*th?\\s+(?:\"((?:[^\"\\\\]|\\\\.)*)\"|`([^`]*)`)")
	jsKey := regexp.MustCompile(`\bT\(\s*(?:"((?:[^"\\]|\\.)*)"|'((?:[^'\\]|\\.)*)')`)
	unq := strings.NewReplacer(`\"`, `"`, `\\`, `\`)
	missing := map[string]string{}
	checked := 0
	fs.WalkDir(assets.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.Contains(path, "htmx") || !(strings.HasSuffix(path, ".html") || strings.HasSuffix(path, ".js")) {
			return err
		}
		b, _ := fs.ReadFile(assets.FS, path)
		for _, re := range []*regexp.Regexp{tmplKey, jsKey} {
			for _, m := range re.FindAllStringSubmatch(string(b), -1) {
				k := unq.Replace(m[1] + m[2])
				checked++
				if v, ok := dict[k]; !ok || v[0] == "" || v[1] == "" {
					missing[k] = path
				}
			}
		}
		return nil
	})
	for k, p := range missing {
		t.Errorf("%s: no EN/ES translation for %q", p, k)
	}
	if checked < 150 {
		t.Errorf("only %d marked texts found", checked)
	}
}

func TestTrFormats(t *testing.T) {
	cases := []struct{ l, in, want string }{
		{"en", "Salvar", "Save"},
		{"es", "Salvar", "Guardar"},
		{"pt", "Salvar", "Salvar"},
		{"en", "texto que não está no dicionário", "texto que não está no dicionário"},
		{"en", "100 × tamanho 75% × elemento 150% = 112", "100 × size 75% × element 150% = 112"},
		{"en", "50 × elemento 150% = 75", "50 × element 150% = 75"},
		{"es", "Nv. 10", "Nv. 10"},
		{"en", "Nv. 10", "Lv. 10"},
		{"en", "O elemento Ghost não causa dano em Ghost 4.", "Element Ghost does no damage to Ghost 4."},
		{"en", "Build — Magus", "Build — Magus"},
	}
	for _, c := range cases {
		if got := tr(c.l, c.in); got != c.want {
			t.Errorf("tr(%s, %q) = %q, want %q", c.l, c.in, got, c.want)
		}
	}
	for h, want := range map[string]string{"es-AR,es;q=0.9": "es", "en-US,en;q=0.8": "en", "pt-BR": "pt", "fr-FR,de": "", "": ""} {
		if got := acceptLang(h); got != want {
			t.Errorf("acceptLang(%q) = %q, want %q", h, got, want)
		}
	}
}

func TestHomeInEnglish(t *testing.T) {
	s, err := New(Options{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Catalog: testCatalog()})
	if err != nil {
		t.Fatal(err)
	}
	h := s.Routes()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/?lang=en", nil))
	body := rec.Body.String()
	for _, want := range []string{`<html lang="en">`, "Characters", "New build", "Item Database", `aria-current="true">English`} {
		if !strings.Contains(body, want) {
			t.Errorf("English home lacks %q", want)
		}
	}
	var c *http.Cookie
	for _, k := range rec.Result().Cookies() {
		if k.Name == "lang" {
			c = k
		}
	}
	if c == nil || c.Value != "en" {
		t.Fatal("?lang=en not remembered in a cookie")
	}
	// The cookie keeps the language on the next page, which beats the browser's.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(c)
	req.Header.Set("Accept-Language", "es-AR")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "Characters") {
		t.Error("language cookie ignored")
	}
	// No choice yet: the browser's language.
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "es-AR,es;q=0.9")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "Personajes") {
		t.Error("Accept-Language not used on first visit")
	}
}

func TestMonsterSearchTranslated(t *testing.T) {
	c := testCatalog()
	c.Monsters = map[int]*gamedata.Monster{1002: {ID: 1002, Name: "Poring", Level: 1, Race: "Plant", Element: "Water", ElementLv: 1}}
	s, err := New(Options{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Catalog: c, NoRateLimit: true})
	if err != nil {
		t.Fatal(err)
	}
	for l, want := range map[string]string{"pt": "Planta · Água", "en": "Plant · Water", "es": "Planta · Agua"} {
		rec := httptest.NewRecorder()
		s.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/build/monsters?q=poring&lang="+l, nil))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s: %d, want %q in %s", l, rec.Code, want, rec.Body.String())
		}
	}
}
