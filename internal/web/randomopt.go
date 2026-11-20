package web

import (
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"ragnarok-of-building/internal/gamedata"
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
	list := randomOptList(c, atoi(r.URL.Query().Get("item")), atoi(r.URL.Query().Get("pos")), lang(r))
	var b strings.Builder
	for _, o := range list {
		b.WriteString(`<option value="` + strconv.Itoa(o.ID) + `"`)
		label := o.Label
		if o.Max > 0 {
			b.WriteString(` data-min="` + strconv.Itoa(o.Min) + `" data-max="` + strconv.Itoa(o.Max) + `"`)
			label += " (" + strconv.Itoa(o.Min) + "–" + strconv.Itoa(o.Max) + ")"
		}
		b.WriteString(">" + template.HTMLEscapeString(label) + "</option>")
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write([]byte(b.String()))
}

// randomOptList is the random options an item position can roll, sorted by
// name, with their value range when the item's options are known; every
// option otherwise.
func randomOptList(c *gamedata.Catalog, item, pos int, l string) []optLine {
	var list []optLine
	if allowed := c.ItemOpts[item].At(pos); allowed != nil {
		for _, a := range allowed {
			if o := c.RandomOpts[a.ID]; o != nil {
				list = append(list, optLine{ID: a.ID, Min: a.Min, Max: a.Max, Label: o.Label(l, 0)})
			}
		}
	} else {
		for id, o := range c.RandomOpts {
			list = append(list, optLine{ID: id, Label: o.Label(l, 0)})
		}
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Label != list[j].Label {
			return list[i].Label < list[j].Label
		}
		return list[i].ID < list[j].ID
	})
	return list
}
