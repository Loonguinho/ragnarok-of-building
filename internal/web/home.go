package web

import "net/http"

// GET / — home: the builds saved in this browser (as character slots,
// rendered by the page from localStorage) and the main actions.
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "home", map[string]any{"Title": "Ragnarok of Building", "Lang": lang(r),
		"Ready": s.opt.Catalog != nil && len(s.opt.Catalog.StatPoints) > 0})
}
