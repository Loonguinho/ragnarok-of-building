package web

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoutes(t *testing.T) {
	s, err := New(slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	h := s.Routes()
	cases := []struct{ path, want string }{
		{"/", "Selecionar Servidor"},
		{"/uikit", "Atributos"},
		{"/healthz", "ok"},
		{"/static/css/ro-ui.css", "ro-window"},
		{"/static/js/rowindow.js", "ro.windows"},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", c.path, rec.Code)
			continue
		}
		if !strings.Contains(rec.Body.String(), c.want) {
			t.Errorf("%s: body missing %q", c.path, c.want)
		}
	}
}
