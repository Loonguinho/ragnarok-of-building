package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/store"
)

const sessionCookie = "rob_session"

// user returns the logged-in user (ok=false when logged out or accounts
// are disabled).
func (s *Server) user(r *http.Request) (store.User, bool) {
	if s.opt.Store == nil {
		return store.User{}, false
	}
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return store.User{}, false
	}
	return s.opt.Store.UserBySession(c.Value)
}

// sameOrigin rejects cross-site form/JSON posts (on top of SameSite=Lax).
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

func (s *Server) setSession(w http.ResponseWriter, r *http.Request, u store.User) error {
	token, err := s.opt.Store.NewSession(u.ID)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil, MaxAge: int(store.SessionTTL.Seconds())})
	return nil
}

// safeNext keeps redirects on this site.
func safeNext(next string) string {
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.HasPrefix(next, "/\\") {
		return "/chars"
	}
	return next
}

// GET /login — the client's login window.
func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.user(r); ok {
		http.Redirect(w, r, "/chars", http.StatusSeeOther)
		return
	}
	s.render(w, r, "login", map[string]any{"Title": "Login", "Lang": lang(r), "Next": r.URL.Query().Get("next"),
		"Register": r.URL.Query().Has("novo"), "NoStore": s.opt.Store == nil})
}

// POST /login and /register.
func (s *Server) handleLogin(register bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.opt.Store == nil || !sameOrigin(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
		name, pass, next := r.PostFormValue("id"), r.PostFormValue("senha"), r.PostFormValue("next")
		var u store.User
		var err error
		if register {
			if pass != r.PostFormValue("senha2") {
				err = errors.New("as senhas não conferem")
			} else {
				u, err = s.opt.Store.Register(name, pass)
			}
		} else {
			u, err = s.opt.Store.Login(name, pass)
		}
		if err == nil {
			err = s.setSession(w, r, u)
		}
		if err != nil {
			s.render(w, r, "login", map[string]any{"Title": "Login", "Lang": lang(r), "Next": next, "Register": register,
				"Error": err.Error(), "Name": name})
			return
		}
		http.Redirect(w, r, safeNext(next), http.StatusSeeOther)
	}
}

// POST /logout
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if c, err := r.Cookie(sessionCookie); err == nil && s.opt.Store != nil {
		s.opt.Store.EndSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

type charSlot struct {
	store.Build
	JobName string
}

// GET /chars — character select: the account's saved builds.
func (s *Server) handleChars(w http.ResponseWriter, r *http.Request) {
	u, ok := s.user(r)
	if !ok {
		http.Redirect(w, r, "/login?next=/chars", http.StatusSeeOther)
		return
	}
	builds, err := s.opt.Store.Builds(u.ID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	l := lang(r)
	slots := make([]charSlot, 0, len(builds))
	for _, b := range builds {
		name := "?"
		if c := s.opt.Catalog; c != nil && c.Jobs[b.Job] != nil {
			name = c.Jobs[b.Job].Name.Get(l)
		}
		slots = append(slots, charSlot{Build: b, JobName: name})
	}
	s.render(w, r, "chars", map[string]any{"Title": "Seleção de Personagem", "Lang": l, "Slots": slots, "Max": store.MaxBuilds})
}

// POST /builds/save — JSON {id, name, code} -> {id}.
func (s *Server) handleSaveBuild(w http.ResponseWriter, r *http.Request) {
	u, ok := s.user(r)
	if !ok || !sameOrigin(r) {
		http.Error(w, "faça login para salvar", http.StatusUnauthorized)
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
	id, err := s.opt.Store.SaveBuild(u.ID, store.Build{ID: in.ID, Name: in.Name, Code: encodeBuild(b), Job: b.Job, BaseLv: b.BaseLv, JobLv: b.JobLv})
	switch {
	case errors.Is(err, store.ErrNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
		return
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
	u, ok := s.user(r)
	if !ok || !sameOrigin(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := s.opt.Store.DeleteBuild(u.ID, id); err != nil && !errors.Is(err, store.ErrNotFound) {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/chars", http.StatusSeeOther)
}
