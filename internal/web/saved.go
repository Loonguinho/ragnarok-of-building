package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/store"
)

// sameOrigin rejects posts from other sites (a page elsewhere posting to
// the local app).
func sameOrigin(r *http.Request) bool {
	src := r.Header.Get("Origin")
	if src == "" {
		src = r.Header.Get("Referer")
	}
	if src == "" {
		return true // non-browser clients
	}
	u, err := url.Parse(src)
	return err == nil && u.Host == r.Host
}

type charSlot struct {
	store.Build
	JobName string
}

// GET / — home: the saved builds (as character slots) and the main actions.
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	l := lang(r)
	data := map[string]any{"Title": "Ragnarok of Building", "Lang": l, "Saving": s.opt.Store != nil,
		"Ready": s.opt.Catalog != nil && len(s.opt.Catalog.StatPoints) > 0}
	if s.opt.Store != nil {
		builds, err := s.opt.Store.Builds()
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		slots := make([]charSlot, 0, len(builds))
		for _, b := range builds {
			name := "?"
			if c := s.opt.Catalog; c != nil && c.Jobs[b.Job] != nil {
				name = c.Jobs[b.Job].Name.Get(l)
			}
			slots = append(slots, charSlot{Build: b, JobName: name})
		}
		data["Slots"] = slots
	}
	s.render(w, r, "home", data)
}

// POST /builds/save — JSON {id, name, code} -> {id}.
func (s *Server) handleSaveBuild(w http.ResponseWriter, r *http.Request) {
	if s.opt.Store == nil || !sameOrigin(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var in struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
		Code string `json:"code"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 128<<10)).Decode(&in); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	b, ok := decodeBuild(in.Code)
	if !ok {
		http.Error(w, "código de build inválido", http.StatusBadRequest)
		return
	}
	if s.opt.Catalog != nil {
		calc.Normalize(&b, s.opt.Catalog)
	}
	id, err := s.opt.Store.SaveBuild(store.Build{ID: in.ID, Name: in.Name, Code: encodeBuild(b), Job: b.Job, BaseLv: b.BaseLv, JobLv: b.JobLv})
	if errors.Is(err, store.ErrNotFound) {
		// Deleted meanwhile: save it as a new one.
		id, err = s.opt.Store.SaveBuild(store.Build{Name: in.Name, Code: encodeBuild(b), Job: b.Job, BaseLv: b.BaseLv, JobLv: b.JobLv})
	}
	switch {
	case errors.Is(err, store.ErrTooMany):
		http.Error(w, err.Error(), http.StatusConflict)
		return
	case err != nil:
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"id": id})
}

// POST /builds/{id}/delete
func (s *Server) handleDeleteBuild(w http.ResponseWriter, r *http.Request) {
	if s.opt.Store == nil || !sameOrigin(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := s.opt.Store.DeleteBuild(id); err != nil && !errors.Is(err, store.ErrNotFound) {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
