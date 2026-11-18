package web

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"ragnarok-of-building/internal/calc"
)

func TestCompare(t *testing.T) {
	s, err := New(Options{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Catalog: testCatalog(), NoRateLimit: true})
	if err != nil {
		t.Fatal(err)
	}
	h := s.Routes()
	weak := encodeBuild(calc.Build{Job: 0, BaseLv: 3, JobLv: 1, Stats: [6]int{1, 1, 1, 1, 1, 1}})
	strong := encodeBuild(calc.Build{Job: 0, BaseLv: 3, JobLv: 1, Stats: [6]int{5, 1, 1, 1, 1, 1}})
	post := func(body string) (int, string) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/compare/calc?lang=pt", strings.NewReader(body)))
		var out struct{ HTML string }
		json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out.HTML
	}
	code, html := post(`{"builds":[{"code":"` + weak + `","name":"Fraco"},{"code":"` + strong + `","name":"Forte"}]}`)
	if code != http.StatusOK {
		t.Fatalf("compare: %d", code)
	}
	for _, want := range []string{"Fraco", "Forte", "Atributos", "Status"} {
		if !strings.Contains(html, want) {
			t.Errorf("table lacks %q", want)
		}
	}
	// FOR row: the second build is best.
	row := regexp.MustCompile(`<tr><th>FOR</th><td>([^<]*)</td><td class="is-best">([^<]*)</td></tr>`).FindStringSubmatch(html)
	if row == nil || row[1] != "1" || row[2] != "5" {
		t.Errorf("FOR row not highlighted on the stronger build: %v", row)
	}
	// Equal values aren't highlighted (AGI is 1 for both).
	if !strings.Contains(html, `<tr><th>AGI</th><td>1</td><td>1</td></tr>`) {
		t.Error("equal values must not be highlighted")
	}
	// An invalid code becomes a column marked invalid, not an error.
	if code, html := post(`{"builds":[{"code":"` + weak + `","name":"A"},{"code":"!!","name":"B"}]}`); code != 200 || !strings.Contains(html, "código de build inválido") {
		t.Errorf("invalid column: %d", code)
	}
	if code, _ := post(`{"builds":[]}`); code != http.StatusBadRequest {
		t.Errorf("no builds: %d", code)
	}
	five := strings.Repeat(`{"code":"`+weak+`"},`, 5)
	if code, _ := post(`{"builds":[` + strings.TrimSuffix(five, ",") + `]}`); code != http.StatusBadRequest {
		t.Errorf("more than 4 builds: %d", code)
	}
	// English labels.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/compare/calc?lang=en", strings.NewReader(`{"builds":[{"code":"`+weak+`","name":"X"}]}`)))
	if !strings.Contains(rec.Body.String(), "STR") || !strings.Contains(rec.Body.String(), "Stats") {
		t.Error("compare table not translated")
	}
	if code, body := get(t, h, "/compare"); code != 200 || !strings.Contains(body, "Comparar Builds") {
		t.Errorf("/compare page: %d", code)
	}
}
