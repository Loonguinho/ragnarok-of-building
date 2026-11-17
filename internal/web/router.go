// Package web wires HTTP routes, templates and static assets.
package web

import (
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	assets "ragnarok-of-building/web"
)

// Server holds parsed templates and dependencies for handlers.
type Server struct {
	pages map[string]*template.Template
	log   *slog.Logger
}

var funcs = template.FuncMap{
	"list": func(v ...any) []any { return v },
}

// New parses every page template together with the shared layout and partials.
func New(log *slog.Logger) (*Server, error) {
	pageFiles, err := fs.Glob(assets.FS, "templates/pages/*.html")
	if err != nil {
		return nil, err
	}
	partials, err := fs.Glob(assets.FS, "templates/partials/*.html")
	if err != nil {
		return nil, err
	}
	s := &Server{pages: map[string]*template.Template{}, log: log}
	for _, p := range pageFiles {
		files := append([]string{"templates/layout.html", p}, partials...)
		t, err := template.New("layout.html").Funcs(funcs).ParseFS(assets.FS, files...)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", p, err)
		}
		name := p[len("templates/pages/") : len(p)-len(".html")]
		s.pages[name] = t
	}
	return s, nil
}

// Routes returns the HTTP handler.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer, middleware.Compress(5))

	static, _ := fs.Sub(assets.FS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(static))))

	r.Get("/", s.page("home", map[string]any{"Title": "Ragnarok of Building"}))
	r.Get("/uikit", s.page("uikit", map[string]any{"Title": "RO UI Kit"}))
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	return r
}

func (s *Server) page(name string, data any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t, ok := s.pages[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := t.ExecuteTemplate(w, "layout.html", data); err != nil {
			s.log.Error("render", "page", name, "err", err)
		}
	}
}
