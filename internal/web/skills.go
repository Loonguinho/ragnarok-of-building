package web

import (
	"net/http"
	"sort"
	"strconv"

	"ragnarok-of-building/internal/gamedata"
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

	chain := c.Chain(jobID)
	js := map[int]skillJS{}
	var tabs []skillTab
	for _, id := range chain {
		j := c.Jobs[id]
		if len(j.Tree) == 0 {
			continue
		}
		maxPos := 0
		for pos := range j.Tree {
			maxPos = max(maxPos, pos)
		}
		rows := make([][]skillCell, maxPos/gamedata.TreeColumns+1)
		for i := range rows {
			rows[i] = make([]skillCell, gamedata.TreeColumns)
			for k := range rows[i] {
				rows[i][k].Empty = true
			}
		}
		for pos, sid := range j.Tree {
			sk := c.Skills[sid]
			if sk == nil {
				continue
			}
			rows[pos/gamedata.TreeColumns][pos%gamedata.TreeColumns] = skillCell{
				ID: sid, Name: sk.Name.Get(l), MaxLv: sk.MaxLv, Desc: sk.Desc.Get(l),
			}
			var needs [][2]int
			for _, n := range sk.NeedsFor(chain) {
				needs = append(needs, [2]int{n.Skill, n.Level})
			}
			js[sid] = skillJS{Max: sk.MaxLv, Needs: needs}
		}
		tabs = append(tabs, skillTab{JobID: id, Name: j.Name.Get(l), Rows: rows})
	}

	data["Jobs"] = opts
	data["JobID"] = jobID
	data["JobName"] = c.Jobs[jobID].Name.Get(l)
	data["Tabs"] = tabs
	// html/template JSON-encodes this safely inside <script>.
	data["Tree"] = map[string]any{"job": jobID, "skills": js}
	s.render(w, r, "skills", data)
}
