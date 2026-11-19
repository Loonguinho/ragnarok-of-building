package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

func TestDownloadCatalog(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.json.gz")
	c := &gamedata.Catalog{Items: map[int]*gamedata.Item{}, Skills: map[int]*gamedata.Skill{}, Jobs: map[int]*gamedata.Job{}}
	if err := c.Save(src); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			http.ServeFile(w, r, src)
		case "/junk":
			w.Write([]byte("not a catalog"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dst := filepath.Join(dir, "catalog.json.gz")
	if err := downloadCatalog(srv.URL+"/ok", dst); err != nil {
		t.Fatal(err)
	}
	if _, err := gamedata.Load(dst); err != nil {
		t.Fatalf("downloaded catalog does not load: %v", err)
	}
	// A missing asset or a broken file leaves nothing behind.
	for _, path := range []string{"/missing", "/junk"} {
		bad := filepath.Join(dir, strings.Trim(path, "/")+".json.gz")
		if err := downloadCatalog(srv.URL+path, bad); err == nil {
			t.Errorf("%s: no error", path)
		}
		if _, err := os.Stat(bad); err == nil {
			t.Errorf("%s: file kept", path)
		}
		if _, err := os.Stat(bad + ".part"); err == nil {
			t.Errorf("%s: .part kept", path)
		}
	}
}

func TestCatalogURL(t *testing.T) {
	version, repo = "v0.1.0-beta.2", "someone/rob"
	defer func() { version, repo = "", "Loonguinho/ragnarok-of-building" }()
	if got := catalogURL(); got != "https://github.com/someone/rob/releases/download/v0.1.0-beta.2/catalog.json.gz" {
		t.Error(got)
	}
	version = ""
	if got := catalogURL(); got != "https://github.com/someone/rob/releases/latest/download/catalog.json.gz" {
		t.Error(got)
	}
}
