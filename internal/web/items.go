package web

import (
	"bytes"
	"net/http"
	"slices"
	"strconv"
	"strings"

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
	Loc   string // comma-separated equip locations (for client-side slot rules)
	Type  string
}

func viewItem(it *gamedata.Item, l string) itemView {
	return itemView{ID: it.ID, Name: it.Name.Get(l), Slots: it.Slots, Desc: it.Desc.Get(l), Lang: l,
		Loc: strings.Join(it.Locations, ","), Type: it.Type}
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
	q, typ := r.URL.Query().Get("q"), r.URL.Query().Get("type")
	data["Q"], data["Type"], data["Types"] = q, typ, itemTypes
	data["Count"] = s.items.Len()
	data["Results"] = s.searchItems(q, typ, l)
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
	q, typ := r.URL.Query().Get("q"), r.URL.Query().Get("type")
	s.fragment(w, r, "items", "item_results", map[string]any{
		"Q": q, "Type": typ, "Lang": l, "Results": s.searchItems(q, typ, l),
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

// itemTypes are the item database filters (value, label).
var itemTypes = [][2]string{
	{"", "Todos"}, {"weapon", "Armas"}, {"armor", "Armaduras e acessórios"}, {"head", "Chapéus"},
	{"card", "Cartas"}, {"shadow", "Equip. sombrios"}, {"other", "Outros"},
}

func matchType(it *gamedata.Item, typ string) bool {
	isHead := it.Type == "Armor" && (it.HasLocation("Head_Top") || it.HasLocation("Head_Mid") || it.HasLocation("Head_Low"))
	switch typ {
	case "":
		return true
	case "weapon":
		return it.Type == "Weapon"
	case "armor":
		return it.Type == "Armor" && !isHead
	case "head":
		return isHead
	case "card":
		return it.Type == "Card"
	case "shadow":
		return it.Type == "ShadowGear"
	case "other":
		return it.Type == ""
	}
	return false
}

// searchItems searches by name/ID within a type. With no query, a type
// filter lists that type by ID; with neither, nothing is listed.
func (s *Server) searchItems(q, typ, l string) []itemView {
	var pool []*gamedata.Item
	switch {
	case q != "":
		pool = s.items.Search(q, len(s.opt.Catalog.Items))
	case typ != "":
		ids := make([]int, 0, len(s.opt.Catalog.Items))
		for id, it := range s.opt.Catalog.Items {
			if matchType(it, typ) && it.Name.Get(l) != "" {
				ids = append(ids, id)
			}
		}
		slices.Sort(ids)
		for _, id := range ids[:min(searchLimit, len(ids))] {
			pool = append(pool, s.opt.Catalog.Items[id])
		}
	}
	var out []itemView
	for _, it := range pool {
		if matchType(it, typ) {
			out = append(out, viewItem(it, l))
			if len(out) >= searchLimit {
				break
			}
		}
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
