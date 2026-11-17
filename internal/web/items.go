package web

import (
	"bytes"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"ragnarok-of-building/internal/gamedata"
)

const searchLimit = 60

// itemView is what the item templates render.
type itemView struct {
	ID    int
	Name  string
	Slots int
	Desc  []string
	Lang  string
}

func viewItem(it *gamedata.Item, l string) itemView {
	return itemView{ID: it.ID, Name: it.Name.Get(l), Slots: it.Slots, Desc: it.Desc.Get(l), Lang: l}
}

// GET /items: the inventory-style search window plus an item info window.
func (s *Server) handleItems(w http.ResponseWriter, r *http.Request) {
	l := lang(r)
	data := map[string]any{"Title": "Itens", "Lang": l}
	if s.opt.Catalog == nil {
		data["NoCatalog"] = true
		s.render(w, r, "items", data)
		return
	}
	q := r.URL.Query().Get("q")
	data["Q"] = q
	data["Count"] = s.items.Len()
	data["Results"] = s.searchItems(q, l)
	if id, err := strconv.Atoi(r.URL.Query().Get("id")); err == nil {
		if it := s.opt.Catalog.Items[id]; it != nil {
			v := viewItem(it, l)
			data["Item"] = &v
		}
	}
	s.render(w, r, "items", data)
}

// GET /items/search?q=: result list fragment (HTMX).
func (s *Server) handleItemSearch(w http.ResponseWriter, r *http.Request) {
	if s.opt.Catalog == nil {
		http.Error(w, "catalog not loaded", http.StatusServiceUnavailable)
		return
	}
	l := lang(r)
	s.fragment(w, r, "items", "item_results", map[string]any{
		"Q": r.URL.Query().Get("q"), "Lang": l, "Results": s.searchItems(r.URL.Query().Get("q"), l),
	})
}

// GET /items/{id}: item info window fragment (HTMX).
func (s *Server) handleItemInfo(w http.ResponseWriter, r *http.Request) {
	if s.opt.Catalog == nil {
		http.Error(w, "catalog not loaded", http.StatusServiceUnavailable)
		return
	}
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	it := s.opt.Catalog.Items[id]
	if err != nil || it == nil {
		http.NotFound(w, r)
		return
	}
	v := viewItem(it, lang(r))
	s.fragment(w, r, "items", "item_info", &v)
}

func (s *Server) searchItems(q, l string) []itemView {
	var out []itemView
	for _, it := range s.items.Search(q, searchLimit) {
		out = append(out, viewItem(it, l))
	}
	return out
}

// fragment renders one named template (no layout) from a page's template set.
func (s *Server) fragment(w http.ResponseWriter, r *http.Request, page, name string, data any) {
	pages := s.pages
	if s.opt.Dev {
		p, err := s.parse()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		pages = p
	}
	t, ok := pages[page]
	if !ok {
		http.NotFound(w, r)
		return
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, name, data); err != nil {
		s.opt.Log.Error("fragment", "name", name, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}
