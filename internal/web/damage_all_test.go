package web

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

// TestDamageWindowEverySkill renders the damage window for every skill with
// a LATAM formula, in every job that can learn it, and for each variant
// (needs the generated catalog; skipped without it).
func TestDamageWindowEverySkill(t *testing.T) {
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
	h := s.Routes()
	target := 1039 // Baphomet
	rendered, formulas := 0, 0
	for jobID, j := range c.Jobs {
		if len(j.Tree) == 0 {
			continue
		}
		seen := map[int]bool{}
		for _, id := range c.Chain(jobID) {
			for _, skID := range c.Jobs[id].Tree {
				sk := c.Skills[skID]
				if sk == nil || seen[skID] {
					continue
				}
				seen[skID] = true
				f := calc.FormulaFor(c, jobID, sk.Code)
				if f == nil {
					continue
				}
				formulas++
				for v := range f.Variants() {
					b := map[string]any{"j": jobID, "b": 250, "jl": 60, "s": []int{50, 50, 50, 50, 50, 50},
						"k": map[string]int{strconv.Itoa(skID): sk.MaxLv}, "m": target,
						"a": map[string]any{"s": skID, "l": sk.MaxLv, "v": v, "sk": 3}}
					body, _ := json.Marshal(b)
					rec := httptest.NewRecorder()
					h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/build/calc", strings.NewReader(string(body))))
					if rec.Code != http.StatusOK {
						t.Fatalf("job %s skill %s variant %d: HTTP %d %s", j.Code, sk.Code, v, rec.Code, rec.Body.String())
					}
					rendered++
				}
			}
		}
	}
	if formulas < 300 {
		t.Errorf("only %d job/skill pairs with formulas", formulas)
	}
	t.Logf("%d damage windows rendered (%d job/skill pairs)", rendered, formulas)
}
