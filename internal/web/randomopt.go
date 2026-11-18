package web

import (
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// GET /build/randomopts?lang=: the random options as <option> elements,
// sorted by name (the slot editor fills its picker with them once).
func (s *Server) handleRandomOpts(w http.ResponseWriter, r *http.Request) {
	c := s.opt.Catalog
	if c == nil {
		http.Error(w, "catalog not loaded", http.StatusServiceUnavailable)
		return
	}
	l := lang(r)
	type opt struct {
		id    int
		label string
	}
	var list []opt
	for id, o := range c.RandomOpts {
		list = append(list, opt{id, o.Label(l, 0)})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].label != list[j].label {
			return list[i].label < list[j].label
		}
		return list[i].id < list[j].id
	})
	var b strings.Builder
	for _, o := range list {
		b.WriteString(`<option value="` + strconv.Itoa(o.id) + `">` + template.HTMLEscapeString(o.label) + "</option>")
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write([]byte(b.String()))
}
