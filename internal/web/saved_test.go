package web

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/store"
)

func TestSavedBuildsFlow(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s, err := New(Options{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Catalog: testCatalog(), Store: st})
	if err != nil {
		t.Fatal(err)
	}
	h := s.Routes()
	do := func(method, path, body, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	code := encodeBuild(calc.Build{Job: 0, BaseLv: 3, JobLv: 1, Stats: [6]int{1, 1, 1, 1, 1, 1}})
	if rec := do("POST", "/builds/save", `{"name":"Meu Aprendiz","code":"`+code+`"}`, "https://evil.example"); rec.Code != http.StatusForbidden {
		t.Errorf("cross-site save: %d", rec.Code)
	}
	rec := do("POST", "/builds/save", `{"name":"Meu Aprendiz","code":"`+code+`"}`, "")
	var out struct{ ID int64 }
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil || out.ID == 0 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do("POST", "/builds/save", `{"name":"x","code":"!!"}`, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("bad code accepted: %d", rec.Code)
	}
	if rec := do("GET", "/", "", ""); !strings.Contains(rec.Body.String(), "Meu Aprendiz") {
		t.Fatal("home doesn't list the saved build")
	}
	id := strconv.FormatInt(out.ID, 10)
	if rec := do("GET", "/build?s="+id, "", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Meu Aprendiz") {
		t.Errorf("open saved build: %d", rec.Code)
	}
	if rec := do("POST", "/builds/"+id+"/delete", "", "https://evil.example"); rec.Code != http.StatusForbidden {
		t.Errorf("cross-site delete: %d", rec.Code)
	}
	do("POST", "/builds/"+id+"/delete", "", "")
	if rec := do("GET", "/", "", ""); strings.Contains(rec.Body.String(), "Meu Aprendiz") {
		t.Error("build not deleted")
	}
}
