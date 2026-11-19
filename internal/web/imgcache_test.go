package web

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestImgCache(t *testing.T) {
	var hits atomic.Int32
	dp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch r.URL.Path {
		case "/images/items/item/501.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("\x89PNGfake"))
		case "/images/skill/5473.png":
			w.Header().Set("Content-Type", "text/html") // not an image
			w.Write([]byte("<html>"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer dp.Close()

	dir := t.TempDir()
	s, err := New(Options{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), NoRateLimit: true, ImageCache: dir})
	if err != nil {
		t.Fatal(err)
	}
	s.imgs.origin = dp.URL
	h := s.Routes()
	get := func(p string) (int, string) {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, p, nil))
		return w.Code, w.Body.String()
	}

	if s.dpImgBase() != "/img/dp" {
		t.Errorf("base = %q", s.dpImgBase())
	}
	if c, b := get("/img/dp/images/items/item/501.png"); c != 200 || b != "\x89PNGfake" {
		t.Fatalf("first fetch: %d %q", c, b)
	}
	if _, err := os.Stat(filepath.Join(dir, "images", "items", "item", "501.png")); err != nil {
		t.Errorf("not cached: %v", err)
	}
	dp.Close() // offline from here on
	if c, b := get("/img/dp/images/items/item/501.png"); c != 200 || b != "\x89PNGfake" {
		t.Errorf("offline, cached: %d %q", c, b)
	}
	if c, _ := get("/img/dp/images/items/item/502.png"); c != 404 {
		t.Errorf("offline, not cached: %d", c)
	}
	for _, p := range []string{"/img/dp/images/../../etc/passwd", "/img/dp/other/x.png", "/img/dp/images/skill/a.svg"} {
		if c, _ := get(p); c != 404 {
			t.Errorf("%s served: %d", p, c)
		}
	}
	n := hits.Load()
	if c, _ := get("/img/dp/images/items/item/501.png"); c != 200 || hits.Load() != n {
		t.Errorf("cached icon fetched again")
	}

	// Without the option the page links Divine Pride and there is no route.
	s2, _ := New(Options{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), NoRateLimit: true})
	if s2.dpImgBase() != dpStatic {
		t.Errorf("hosted base = %q", s2.dpImgBase())
	}
	w := httptest.NewRecorder()
	s2.Routes().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/img/dp/images/items/item/501.png", nil))
	if w.Code == 200 {
		t.Error("hosted server serves the cache route")
	}
}

func TestImgCacheRejectsNonImages(t *testing.T) {
	dp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html>"))
	}))
	defer dp.Close()
	s, _ := New(Options{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), NoRateLimit: true, ImageCache: t.TempDir()})
	s.imgs.origin = dp.URL
	w := httptest.NewRecorder()
	s.Routes().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/img/dp/images/skill/5473.png", nil))
	if w.Code != 404 {
		t.Errorf("html served as an icon: %d", w.Code)
	}
}
