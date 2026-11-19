package web

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Icons (items, skills, monsters, classes) come from Divine Pride's static
// site. With Options.ImageCache (the desktop app) the page asks this server
// instead: each icon is fetched once, the first time it is shown, kept in
// the cache folder and served from there afterwards, also offline. Without
// it (a hosted server) the page links Divine Pride directly.

const (
	dpStatic    = "https://static.divine-pride.net"
	maxImgBytes = 2 << 20
	missTTL     = 24 * time.Hour // an icon Divine Pride does not have is asked again after this
)

// dpImgPath: the only paths the cache serves (one image each, no listing).
var dpImgPath = regexp.MustCompile(`^images/(items/item|skill|mobs/png|jobs)/[A-Za-z0-9_]{1,64}\.png$`)

type imgCache struct {
	dir    string
	origin string // dpStatic; tests point it elsewhere
	client *http.Client
	locks  sync.Map // path -> *sync.Mutex: one download per icon at a time
}

func newImgCache(dir string) *imgCache {
	return &imgCache{dir: dir, origin: dpStatic, client: &http.Client{Timeout: 15 * time.Second}}
}

// dpImgBase is the template function the icons' src start with.
func (s *Server) dpImgBase() string {
	if s.imgs != nil {
		return "/img/dp"
	}
	return dpStatic
}

// GET /img/dp/images/...: the icon from the cache, fetched once when missing.
func (s *Server) handleImgCache(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/img/dp/")
	if !dpImgPath.MatchString(rel) {
		http.NotFound(w, r)
		return
	}
	c := s.imgs
	file := filepath.Join(c.dir, filepath.FromSlash(rel))
	serve := func() bool {
		f, err := os.Open(file)
		if err != nil {
			return false
		}
		defer f.Close()
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=604800")
		io.Copy(w, f)
		return true
	}
	if serve() {
		return
	}
	mu, _ := c.locks.LoadOrStore(rel, &sync.Mutex{})
	mu.(*sync.Mutex).Lock()
	defer mu.(*sync.Mutex).Unlock()
	if serve() { // fetched by another request meanwhile
		return
	}
	if st, err := os.Stat(file + ".miss"); err == nil && time.Since(st.ModTime()) < missTTL {
		http.NotFound(w, r)
		return
	}
	if err := c.fetch(r, rel, file); err != nil {
		w.Header().Set("Cache-Control", "no-store")
		http.NotFound(w, r)
		return
	}
	serve()
}

// fetch downloads one icon into file (only PNG/image responses, size-capped).
func (c *imgCache) fetch(r *http.Request, rel, file string) error {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, c.origin+"/"+rel, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "ragnarok-of-building")
	res, err := c.client.Do(req)
	if err != nil {
		return err // offline: try again next time
	}
	defer res.Body.Close()
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	if res.StatusCode == http.StatusNotFound {
		os.WriteFile(file+".miss", nil, 0o644)
		return os.ErrNotExist
	}
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), "image/") {
		return os.ErrNotExist
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, maxImgBytes+1))
	if err != nil || len(b) > maxImgBytes || len(b) == 0 {
		return os.ErrNotExist
	}
	tmp := file + ".part"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}
