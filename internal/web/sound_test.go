package web

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClickSound(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "click.wav"), []byte("RIFF....WAVEfmt "), 0o644)
	os.WriteFile(filepath.Join(filepath.Dir(dir), "secret.txt"), []byte("x"), 0o644)
	get := func(o Options, path string) *httptest.ResponseRecorder {
		o.Log, o.Catalog = slog.New(slog.NewTextHandler(io.Discard, nil)), testCatalog()
		s, err := New(o)
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		s.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}
	with := Options{SoundDir: dir}
	if rec := get(with, "/sounds/click.wav"); rec.Code != 200 || rec.Header().Get("Content-Type") != "audio/wav" {
		t.Errorf("click.wav: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	for _, bad := range []string{"/sounds/..%2Fsecret.txt", "/sounds/secret.txt", "/sounds/nope.wav"} {
		if rec := get(with, bad); rec.Code != 404 {
			t.Errorf("%s: %d, want 404", bad, rec.Code)
		}
	}
	if rec := get(with, "/"); !strings.Contains(rec.Body.String(), `data-click-sound="/sounds/click.wav"`) || !strings.Contains(rec.Body.String(), "data-sound-toggle") {
		t.Error("home doesn't announce the click sound and its toggle")
	}
	if rec := get(Options{}, "/"); strings.Contains(rec.Body.String(), "data-click-sound") || strings.Contains(rec.Body.String(), "data-sound-toggle") {
		t.Error("no sound folder: the page must stay silent")
	}
}
