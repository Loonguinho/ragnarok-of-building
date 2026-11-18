package web

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
}
