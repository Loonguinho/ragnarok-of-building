package web

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"ragnarok-of-building/internal/calc"
)

// fakePaste is a paste.rs stand-in: POST / stores, GET /ID returns it.
func fakePaste(t *testing.T) *httptest.Server {
	var mu sync.Mutex
	store := map[string]string{}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodPost {
			b, _ := io.ReadAll(r.Body)
			store["Ab12"] = string(b)
			w.WriteHeader(http.StatusCreated)
			io.WriteString(w, srv.URL+"/Ab12")
			return
		}
		body, ok := store[strings.TrimPrefix(r.URL.Path, "/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestPasteRoundTrip(t *testing.T) {
	ps := fakePaste(t)
	s, err := New(Options{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Catalog: testCatalog(), NoRateLimit: true, PasteURL: ps.URL})
	if err != nil {
		t.Fatal(err)
	}
	h := s.Routes()
	code := encodeBuild(calc.Build{Job: 4252, BaseLv: 250, JobLv: 50, Stats: [6]int{120, 1, 1, 1, 1, 1}})

	form := url.Values{"b": {code}, "text": {"**Build** · Nv. 250/50"}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/build/paste", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rec, req)
	var made map[string]string
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &made) != nil || made["url"] != ps.URL+"/Ab12" {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/build/paste?u="+url.QueryEscape(made["url"]), nil))
	var read map[string]string
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &read) != nil || read["code"] != code {
		t.Fatalf("read: %d %s", rec.Code, rec.Body)
	}

	// Bad codes are not pasted; other hosts are never fetched.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/build/paste", strings.NewReader("b=nope"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad code: %d", rec.Code)
	}
	for _, u := range []string{"https://evil.example/Ab12", ps.URL + "/../x", ps.URL + "/a/b", "", "file:///etc/passwd"} {
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/build/paste?u="+url.QueryEscape(u), nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%q: %d", u, rec.Code)
		}
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/build/paste?u="+url.QueryEscape(ps.URL+"/Zz99"), nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("missing paste: %d", rec.Code)
	}
}

func TestFindBuildCode(t *testing.T) {
	code := encodeBuild(calc.Build{Job: 4252, BaseLv: 200})
	for _, text := range []string{
		"http://localhost:8080/build?b=" + code,
		"blah\n\n" + code + "\n",
		"zNotAValidCodeAtAll1234 then " + code,
	} {
		if got := findBuildCode([]byte(text)); got != code {
			t.Errorf("%q: got %q", text, got)
		}
	}
	if got := findBuildCode([]byte("nothing here")); got != "" {
		t.Errorf("found %q", got)
	}
}
