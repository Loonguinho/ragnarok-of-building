package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/replay"
)

const maxReplayUpload = 16 << 20

type classFamily struct {
	Name string
	Jobs []jobOption
}

// GET /new: start a build from a replay or from scratch.
func (s *Server) handleNew(w http.ResponseWriter, r *http.Request) {
	l := lang(r)
	c := s.opt.Catalog
	data := map[string]any{"Title": "Novo Build", "Lang": l}
	if c == nil || len(c.StatPoints) == 0 {
		data["NoCatalog"] = true
		s.render(w, r, "new", data)
		return
	}
	data["Families"] = s.classFamilies(l, 0)
	s.render(w, r, "new", data)
}

// classFamilies groups the playable classes by their first class
// (Espadachim, Mago, ...), families by their lowest job ID.
func (s *Server) classFamilies(l string, current int) []classFamily {
	c := s.opt.Catalog
	hidden := s.hiddenJobs(l, current)
	families := map[string]*classFamily{}
	var order []string
	for _, j := range c.Jobs {
		if len(j.Tree) == 0 || !j.HasStats() || j.Name.Get(l) == "" || hidden[j.ID] {
			continue
		}
		chain := c.Chain(j.ID)
		key := "Outras"
		if len(chain) > 1 {
			key = c.Jobs[chain[1]].Name.Get(l)
		}
		f := families[key]
		if f == nil {
			f = &classFamily{Name: key}
			families[key] = f
			order = append(order, key)
		}
		f.Jobs = append(f.Jobs, jobOption{ID: j.ID, Name: j.Name.Get(l)})
	}
	var out []classFamily
	for _, k := range order {
		f := families[k]
		sort.Slice(f.Jobs, func(i, k int) bool { return f.Jobs[i].ID < f.Jobs[k].ID })
		out = append(out, *f)
	}
	// Families are listed by their lowest job ID (1st classes first).
	sort.SliceStable(out, func(i, k int) bool {
		if (out[i].Name == "Outras") != (out[k].Name == "Outras") {
			return out[k].Name == "Outras"
		}
		return out[i].Jobs[0].ID < out[k].Jobs[0].ID
	})
	return out
}

// POST /build/import (multipart "file"): .rrf replay -> build share code.
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	fail := func(code int, msg string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(map[string]string{"error": msg})
	}
	if s.opt.Catalog == nil {
		fail(http.StatusServiceUnavailable, tr(lang(r), "Os dados do jogo não foram importados no servidor."))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxReplayUpload+1<<20)
	f, _, err := r.FormFile("file")
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			fail(http.StatusRequestEntityTooLarge, tr(lang(r), "Arquivo grande demais (máx. 16 MB)."))
			return
		}
		fail(http.StatusBadRequest, tr(lang(r), "Envie um arquivo .rrf."))
		return
	}
	defer f.Close()
	buf, err := io.ReadAll(io.LimitReader(f, maxReplayUpload+1))
	if err != nil || len(buf) > maxReplayUpload {
		fail(http.StatusRequestEntityTooLarge, tr(lang(r), "Arquivo grande demais (máx. 16 MB)."))
		return
	}
	b, notes, dv, err := s.importReplay(buf, lang(r))
	if err != nil {
		s.opt.Log.Info("replay import rejected", "err", err)
		fail(http.StatusUnprocessableEntity, tr(lang(r), "Não consegui ler este replay. Ele é um .rrf gravado pelo cliente do RO LATAM?"))
		return
	}
	jobName := ""
	if j := s.opt.Catalog.Jobs[b.Job]; j != nil {
		jobName = j.Name.Get(lang(r))
	}
	out := map[string]any{
		"code":  encodeBuild(b),
		"notes": notes,
		"summary": map[string]any{
			"job": jobName, "base": b.BaseLv, "joblv": b.JobLv,
			"items": len(b.Equip), "skills": len(b.Skills),
		},
	}
	// The damage window, only when the recording has damage dealt to monsters.
	if dv != nil {
		if html, err := s.fragmentString(lang(r), "build", "replay_damage", dv); err == nil {
			out["damage"] = html
		} else {
			s.opt.Log.Warn("replay damage window", "err", err)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// importReplay reads a .rrf: the build it shows (normalized), notes for the
// player (translated) and the damage window (nil without damage data).
func (s *Server) importReplay(buf []byte, l string) (calc.Build, []string, *rdmgView, error) {
	snap, err := replay.Decode(buf)
	if err != nil {
		return calc.Build{}, nil, nil, err
	}
	b, notes := replay.ToBuild(snap, s.opt.Catalog)
	notes = append(notes, calc.Normalize(&b, s.opt.Catalog)...)
	for i := range notes {
		notes[i] = tr(l, notes[i])
	}
	return b, notes, replayDamageView(snap, s.opt.Catalog, l), nil
}

// listedTiers are the class tiers offered in class lists: 3rd and 4th
// classes (endgame builds); the others only show when the build is one.
var listedTiers = map[string]bool{"Third": true, "Third_Upper": true, "Fourth": true}

// hiddenJobs are the classes left out of class lists, unless one is the
// build's own class (current):
//   - classes below the 3rd (first, second, transcendent, baby 3rd ...);
//   - when two classes share a name (a 3rd job and its transcendent
//     version, e.g. JT_RUNE_KNIGHT and JT_RUNE_KNIGHT_H), the
//     non-transcendent one.
func (s *Server) hiddenJobs(l string, current int) map[int]bool {
	byName := map[string][]*gamedata.Job{}
	for _, j := range s.opt.Catalog.Jobs {
		if len(j.Tree) == 0 || !j.HasStats() || j.Name.Get(l) == "" {
			continue
		}
		byName[j.Name.Get(l)] = append(byName[j.Name.Get(l)], j)
	}
	hidden := map[int]bool{}
	for _, j := range s.opt.Catalog.Jobs {
		if j.EquipTier != "" && !listedTiers[j.EquipTier] && !j.Fourth && j.ID != current { // Fourth: 4th classes rAthena files as Normal (Spirit Handler)
			hidden[j.ID] = true
		}
	}
	for _, js := range byName {
		if len(js) < 2 {
			continue
		}
		hasUpper := false
		for _, j := range js {
			hasUpper = hasUpper || j.Upper
		}
		for _, j := range js {
			if hasUpper && !j.Upper && j.ID != current {
				hidden[j.ID] = true
			}
		}
	}
	return hidden
}
