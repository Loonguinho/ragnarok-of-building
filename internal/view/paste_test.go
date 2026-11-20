package view

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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
	s, err := newTestServer(Options{Catalog: testCatalog(), PasteURL: ps.URL})
	if err != nil {
		t.Fatal(err)
	}
	code := encodeBuild(calc.Build{Job: 4252, BaseLv: 250, JobLv: 50, Stats: [6]int{120, 1, 1, 1, 1, 1}})
	ctx := context.Background()
	link, err := s.createPaste(ctx, code, "**Build** · Nv. 250/50")
	if err != nil || link != ps.URL+"/Ab12" {
		t.Fatalf("create: %q %v", link, err)
	}
	if got, _, err := s.readPaste(ctx, link); err != nil || got != code {
		t.Fatalf("read: %q %v", got, err)
	}
	// Other hosts are never fetched.
	for _, u := range []string{"https://evil.example/Ab12", ps.URL + "/../x", ps.URL + "/a/b", "", "file:///etc/passwd"} {
		if _, status, err := s.readPaste(ctx, u); err == nil || status != http.StatusBadRequest {
			t.Errorf("%q: %d %v", u, status, err)
		}
	}
	if _, status, _ := s.readPaste(ctx, ps.URL+"/Zz99"); status != http.StatusNotFound {
		t.Errorf("missing paste: %d", status)
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
