package web

import (
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// GET /build/randomopts?lang=&item=&pos=: the random options as <option>
// elements sorted by name. With an item whose options are known, only the
// ones its position pos can roll, each with its value range (data-min,
// data-max).
func (s *Server) handleRandomOpts(w http.ResponseWriter, r *http.Request) {
	c := s.opt.Catalog
	if c == nil {
		http.Error(w, "catalog not loaded", http.StatusServiceUnavailable)
		return
	}
	l := lang(r)
	type opt struct {
		id, min, max int
		label        string
	}
	var list []opt
	allowed := c.ItemOpts[atoi(r.URL.Query().Get("item"))].At(atoi(r.URL.Query().Get("pos")))
	if allowed != nil {
		for _, a := range allowed {
			if o := c.RandomOpts[a.ID]; o != nil {
				list = append(list, opt{a.ID, a.Min, a.Max, o.Label(l, 0)})
			}
		}
	} else {
		for id, o := range c.RandomOpts {
			list = append(list, opt{id, 0, 0, o.Label(l, 0)})
		}
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].label != list[j].label {
			return list[i].label < list[j].label
		}
		return list[i].id < list[j].id
	})
	var b strings.Builder
	for _, o := range list {
		b.WriteString(`<option value="` + strconv.Itoa(o.id) + `"`)
		label := o.label
		if o.max > 0 {
			b.WriteString(` data-min="` + strconv.Itoa(o.min) + `" data-max="` + strconv.Itoa(o.max) + `"`)
			label += " (" + strconv.Itoa(o.min) + "–" + strconv.Itoa(o.max) + ")"
		}
		b.WriteString(">" + template.HTMLEscapeString(label) + "</option>")
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write([]byte(b.String()))
}
