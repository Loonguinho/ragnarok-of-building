package web

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/go-chi/chi/v5"
)

var soundName = regexp.MustCompile(`^[a-z0-9_]+\.wav$`)

// hasSound reports whether an interface sound was extracted from the client.
func (s *Server) hasSound(name string) bool {
	if s.opt.SoundDir == "" || !soundName.MatchString(name) {
		return false
	}
	st, err := os.Stat(filepath.Join(s.opt.SoundDir, name))
	return err == nil && !st.IsDir()
}

// GET /sounds/{name}: interface sounds (only plain names, only that folder).
func (s *Server) handleSound(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !s.hasSound(name) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, filepath.Join(s.opt.SoundDir, name))
}
