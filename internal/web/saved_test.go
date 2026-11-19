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

func TestSavedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "builds.json")
	s, err := New(Options{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), NoRateLimit: true, SavedFile: path})
	if err != nil {
		t.Fatal(err)
	}
	h := s.Routes()
	put := func(body string) int {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/saved", strings.NewReader(body)))
		return w.Code
	}
	// Entries without id or code are dropped; a name with </script> stays inert.
	if c := put(`[{"id":"a","code":"zabc","name":"</script>x"},{"id":"","code":"z"},{"name":"no code","id":"b"}]`); c != http.StatusNoContent {
		t.Fatalf("PUT = %d", c)
	}
	b, _ := os.ReadFile(path)
	if got := string(b); !strings.Contains(got, `"id":"a"`) || strings.Contains(got, `"id":"b"`) || strings.Contains(got, "</script>") {
		t.Errorf("file = %s", got)
	}
	if string(s.savedJSON()) != string(b) {
		t.Errorf("page data %q differs from the file %q", s.savedJSON(), b)
	}
	if c := put(`{"not":"a list"}`); c != http.StatusBadRequest {
		t.Errorf("bad body = %d", c)
	}
	// Read back by a new server (the next start of the app).
	s2, _ := New(Options{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), SavedFile: path})
	if !strings.Contains(string(s2.savedJSON()), `"id":"a"`) {
		t.Errorf("reloaded = %s", s2.savedJSON())
	}
	// Without the option there is no /saved and no page data.
	s3, _ := New(Options{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), NoRateLimit: true})
	w := httptest.NewRecorder()
	s3.Routes().ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/saved", strings.NewReader("[]")))
	if w.Code == http.StatusNoContent || s3.savedJSON() != "" {
		t.Errorf("browser mode: PUT /saved = %d, data %q", w.Code, s3.savedJSON())
	}
}
