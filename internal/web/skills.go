package web

import (
	"net/http"
	"sort"
	"strconv"
)

type skillCell struct {
	Empty bool
	ID    int
	Name  string
	MaxLv int
	Desc  []string
}

type skillTab struct {
	JobID int
	Name  string
	Rows  [][]skillCell
}

type jobOption struct {
	ID       int
	Name     string
	Selected bool
}

// skillJS is the per-skill data the client-side tree logic needs.
type skillJS struct {
	Max   int      `json:"max"`
	Needs [][2]int `json:"needs,omitempty"`
}

func (s *Server) handleSkills(w http.ResponseWriter, r *http.Request) {
	l := lang(r)
	c := s.opt.Catalog
	data := map[string]any{"Title": "Habilidades", "Lang": l}
	if c == nil {
		data["NoCatalog"] = true
		s.render(w, r, "skills", data)
		return
	}

	jobID := 4252 // Dragon Knight by default
	if v, err := strconv.Atoi(r.URL.Query().Get("job")); err == nil {
		if _, ok := c.Jobs[v]; ok {
			jobID = v
		}
	}

	var opts []jobOption
	for _, j := range c.Jobs {
		if len(j.Tree) == 0 || j.Name.Get(l) == "" {
			continue
		}
		opts = append(opts, jobOption{ID: j.ID, Name: j.Name.Get(l), Selected: j.ID == jobID})
	}
	sort.Slice(opts, func(i, k int) bool { return opts[i].ID < opts[k].ID })

	d := s.skillTreeData(jobID, l, nil)
	data["Jobs"] = opts
	data["JobID"] = jobID
	data["JobName"] = c.Jobs[jobID].Name.Get(l)
	data["Tabs"] = d["Tabs"]
	// html/template JSON-encodes this safely inside <script>.
	data["Tree"] = d["Tree"]
	s.render(w, r, "skills", data)
}
