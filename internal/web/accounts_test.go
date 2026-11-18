package web

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/store"
)

func TestAccountsFlow(t *testing.T) {
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
	var cookie *http.Cookie
	do := func(method, path, body, ctype, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if ctype != "" {
			req.Header.Set("Content-Type", ctype)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		for _, c := range rec.Result().Cookies() {
			if c.Name == sessionCookie {
				cookie = c
			}
		}
		return rec
	}
	form := func(v url.Values) string { return v.Encode() }
	const fctype = "application/x-www-form-urlencoded"

	if rec := do("GET", "/chars", "", "", ""); rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), "/login") {
		t.Fatalf("/chars logged out: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	// Cross-site register is refused.
	if rec := do("POST", "/register", form(url.Values{"id": {"jogador1"}, "senha": {"segredo1"}, "senha2": {"segredo1"}}), fctype, "https://evil.example"); rec.Code != http.StatusForbidden {
		t.Errorf("cross-site register: %d", rec.Code)
	}
	if rec := do("POST", "/register", form(url.Values{"id": {"jogador1"}, "senha": {"segredo1"}, "senha2": {"outra123"}}), fctype, ""); !strings.Contains(rec.Body.String(), "não conferem") {
		t.Errorf("password mismatch not reported")
	}
	rec := do("POST", "/register", form(url.Values{"id": {"jogador1"}, "senha": {"segredo1"}, "senha2": {"segredo1"}, "next": {"//evil.example"}}), fctype, "")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/chars" || cookie == nil {
		t.Fatalf("register: %d %s (cookie %v)", rec.Code, rec.Header().Get("Location"), cookie)
	}
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("session cookie flags: %+v", cookie)
	}

	// Save a build, then update it.
	code := encodeBuild(calc.Build{Job: 0, BaseLv: 3, JobLv: 1, Stats: [6]int{1, 1, 1, 1, 1, 1}})
	rec = do("POST", "/builds/save", `{"name":"Meu Aprendiz","code":"`+code+`"}`, "application/json", "")
	var out struct{ ID int64 }
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil || out.ID == 0 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do("POST", "/builds/save", `{"name":"x","code":"!!"}`, "application/json", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("bad code accepted: %d", rec.Code)
	}
	rec = do("GET", "/chars", "", "", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Meu Aprendiz") || !strings.Contains(rec.Body.String(), "Aprendiz") {
		t.Fatalf("/chars: %d", rec.Code)
	}
	rec = do("GET", "/build?s="+itoa(int(out.ID)), "", "", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `name: "Meu Aprendiz"`) && !strings.Contains(rec.Body.String(), `"name":"Meu Aprendiz"`) && !strings.Contains(rec.Body.String(), `Meu Aprendiz`) {
		t.Errorf("open saved build: %d", rec.Code)
	}

	// Another account can't see or delete it.
	mine := cookie
	cookie = nil
	do("POST", "/register", form(url.Values{"id": {"jogador2"}, "senha": {"segredo1"}, "senha2": {"segredo1"}}), fctype, "")
	if rec := do("GET", "/chars", "", "", ""); strings.Contains(rec.Body.String(), "Meu Aprendiz") {
		t.Error("other account lists the build")
	}
	do("POST", "/builds/"+itoa(int(out.ID))+"/delete", "", "", "")
	cookie = mine
	if rec := do("GET", "/chars", "", "", ""); !strings.Contains(rec.Body.String(), "Meu Aprendiz") {
		t.Error("other account deleted the build")
	}

	// Delete, log out, log back in.
	do("POST", "/builds/"+itoa(int(out.ID))+"/delete", "", "", "")
	if rec := do("GET", "/chars", "", "", ""); strings.Contains(rec.Body.String(), "Meu Aprendiz") {
		t.Error("build not deleted")
	}
	do("POST", "/logout", "", "", "")
	if rec := do("GET", "/chars", "", "", ""); rec.Code != http.StatusSeeOther {
		t.Errorf("/chars after logout: %d", rec.Code)
	}
	if rec := do("POST", "/login", form(url.Values{"id": {"jogador1"}, "senha": {"errada"}}), fctype, ""); !strings.Contains(rec.Body.String(), "incorretos") {
		t.Error("wrong password not reported")
	}
	if rec := do("POST", "/login", form(url.Values{"id": {"jogador1"}, "senha": {"segredo1"}, "next": {"/build"}}), fctype, ""); rec.Header().Get("Location") != "/build" {
		t.Errorf("login redirect: %s", rec.Header().Get("Location"))
	}
}
