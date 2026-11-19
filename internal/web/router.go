// Package web wires HTTP routes, templates and static assets.
package web

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"ragnarok-of-building/internal/divinepride"
	"ragnarok-of-building/internal/gamedata"
	assets "ragnarok-of-building/web"
)

// Options configure the server.
type Options struct {
	Log     *slog.Logger
	Catalog *gamedata.Catalog // nil until `go run ./cmd/import` has been run
	// Dev reads templates and static files from ./web on every request
	// (edit + refresh, no rebuild).
	Dev bool
	// NoRateLimit turns off the per-IP request limits (tests).
	NoRateLimit bool
	// SoundDir holds the interface sounds extracted from the client
	// (data/gen/sounds); empty or missing means a silent interface.
	SoundDir string
	// DP fetches, one ID at a time, monsters rAthena has no stats for
	// (nil: no Divine Pride key).
	DP *divinepride.Client
	// PasteURL is the paste service for shared builds (empty: paste.rs).
	PasteURL string
	// SavedFile keeps the saved builds in this JSON file (the desktop app);
	// empty: they stay in each browser's localStorage.
	SavedFile string
	// ImageCache keeps the Divine Pride icons in this folder (the desktop
	// app: shown offline once seen); empty: the page links Divine Pride.
	ImageCache string
}

// Server holds parsed templates and dependencies for handlers.
type Server struct {
	opt   Options
	saved *savedStore // nil: saved builds live in the browser
	imgs  *imgCache   // nil: icons come straight from Divine Pride
	fsys  fs.FS
	pages map[string]*template.Template
	items *gamedata.ItemIndex

	// Optimizer card pools, built on first use.
	optOnce     sync.Once
	optCardList []*gamedata.Item

	// Monster stats being fetched from Divine Pride, and IDs that failed.
	mobMu     sync.Mutex
	mobFailed map[int]bool
}

// New parses templates (once, unless Dev).
func New(opt Options) (*Server, error) {
	s := &Server{opt: opt, fsys: assets.FS}
	if opt.SavedFile != "" {
		s.saved = newSavedStore(opt.SavedFile)
	}
	if opt.ImageCache != "" {
		s.imgs = newImgCache(opt.ImageCache)
	}
	if opt.Dev {
		s.fsys = os.DirFS("web")
	}
	pages, err := s.parse()
	if err != nil {
		return nil, err
	}
	s.pages = pages
	if opt.Catalog != nil {
		s.items = gamedata.NewItemIndex(opt.Catalog)
	}
	return s, nil
}

func (s *Server) parse() (map[string]*template.Template, error) {
	pageFiles, err := fs.Glob(s.fsys, "templates/pages/*.html")
	if err != nil {
		return nil, err
	}
	partials, err := fs.Glob(s.fsys, "templates/partials/*.html")
	if err != nil {
		return nil, err
	}
	// One template set per interface language ("en/build"), each with its
	// own t/th functions.
	pages := map[string]*template.Template{}
	for _, ul := range uiLangs {
		for _, p := range pageFiles {
			files := append([]string{"templates/layout.html", p}, partials...)
			t, err := template.New("layout.html").Funcs(funcs).Funcs(langFuncs(ul.Code)).Funcs(template.FuncMap{"asset": s.assetURL, "savedFile": s.savedJSON, "dpimg": s.dpImgBase}).ParseFS(s.fsys, files...)
			if err != nil {
				return nil, fmt.Errorf("parse %s: %w", p, err)
			}
			pages[ul.Code+"/"+p[len("templates/pages/"):len(p)-len(".html")]] = t
		}
	}
	return pages, nil
}

// Routes returns the HTTP handler.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer, middleware.Compress(5), securityHeaders, rememberLang)
	// Per-IP limits: the damage window recalculates on every change, replay
	// uploads are the heaviest request.
	calcLimit := newRateLimiter(10, 40)
	searchLimit := newRateLimiter(10, 40)
	importLimit := newRateLimiter(0.2, 5) // 5 in a row, then one every 5 s
	pasteLimit := newRateLimiter(0.1, 5)  // builds pasted/read on paste.rs: one every 10 s
	if s.opt.NoRateLimit {
		calcLimit, searchLimit, importLimit, pasteLimit = nil, nil, nil, nil
	}

	static, _ := fs.Sub(s.fsys, "static")
	fileServer := http.StripPrefix("/static/", http.FileServer(http.FS(static)))
	if s.opt.Dev {
		r.Handle("/static/*", noCache(fileServer))
	} else {
		r.Handle("/static/*", staticCache(fileServer))
	}

	// No home screen: everything is in the build editor.
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		to := "/build"
		if r.URL.RawQuery != "" {
			to += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, to, http.StatusFound)
	})
	r.Get("/uikit", s.simple("uikit", "RO UI Kit"))
	r.Get("/skills", s.handleSkills)
	r.Get("/items", s.handleItems)
	r.With(searchLimit.limit).Get("/items/search", s.handleItemSearch)
	r.Get("/items/{id}", s.handleItemInfo)
	r.Get("/items/{id}/tip", s.handleItemTip)
	r.Get("/new", s.handleNew)
	r.With(importLimit.limit).Post("/build/import", oneAtATime(2, http.HandlerFunc(s.handleImport)).ServeHTTP)
	r.Get("/build", s.handleBuild)
	if s.saved != nil {
		r.Put("/saved", s.handleSavedPut)
	}
	if s.imgs != nil {
		r.Get("/img/dp/*", s.handleImgCache)
	}
	r.With(calcLimit.limit).Post("/build/calc", s.handleBuildCalc)
	r.With(calcLimit.limit).Post("/build/optimize", oneAtATime(2, http.HandlerFunc(s.handleOptimize)).ServeHTTP)
	r.Get("/build/skills", s.handleBuildSkills)
	r.With(searchLimit.limit).Get("/build/items", s.handleBuildItems)
	r.With(calcLimit.limit).Get("/build/swap", s.handleSwap)
	r.Get("/build/randomopts", s.handleRandomOpts)
	r.With(pasteLimit.limit).Post("/build/paste", s.handlePasteCreate)
	r.With(pasteLimit.limit).Get("/build/paste", s.handlePasteRead)
	r.With(searchLimit.limit).Get("/build/monsters", s.handleBuildMonsters)
	r.Get("/sounds/{name}", s.handleSound)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	return r
}

func (s *Server) simple(page, title string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.render(w, r, page, map[string]any{"Title": title, "Lang": lang(r)})
	}
}

// render executes a page into a buffer first, so template errors produce a
// clean 500 instead of a half-written page.
func (s *Server) render(w http.ResponseWriter, r *http.Request, page string, data any) {
	pages := s.pages
	if s.opt.Dev {
		p, err := s.parse()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		pages = p
	}
	t, ok := pages[lang(r)+"/"+page]
	if !ok {
		http.NotFound(w, r)
		return
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout.html", data); err != nil {
		s.opt.Log.Error("render", "page", page, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if s.opt.Dev {
		w.Header().Set("Cache-Control", "no-store")
	}
	buf.WriteTo(w)
}

func noCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		h.ServeHTTP(w, r)
	})
}

// lang picks the UI language from ?lang=, then a cookie, defaulting to pt.
func lang(r *http.Request) string {
	l := r.URL.Query().Get("lang")
	if l == "" {
		if c, err := r.Cookie("lang"); err == nil {
			l = c.Value
		}
	}
	if l == "" {
		l = acceptLang(r.Header.Get("Accept-Language"))
	}
	for _, ok := range gamedata.Langs {
		if l == ok {
			return l
		}
	}
	return "pt"
}
