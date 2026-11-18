package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"

	"ragnarok-of-building/internal/calc"
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
	// Group playable classes by their first class (Espadachim, Mago, ...).
	families := map[string]*classFamily{}
	var order []string
	for _, j := range c.Jobs {
		if len(j.Tree) == 0 || !j.HasStats() || j.Name.Get(l) == "" {
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
	data["Families"] = out
	s.render(w, r, "new", data)
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
	snap, err := replay.Decode(buf)
	if err != nil {
		s.opt.Log.Info("replay import rejected", "err", err)
		fail(http.StatusUnprocessableEntity, tr(lang(r), "Não consegui ler este replay. Ele é um .rrf gravado pelo cliente do RO LATAM?"))
		return
	}
	b, notes := replay.ToBuild(snap, s.opt.Catalog)
	notes = append(notes, calc.Normalize(&b, s.opt.Catalog)...)
	for i := range notes {
		notes[i] = tr(lang(r), notes[i])
	}
	jobName := ""
	if j := s.opt.Catalog.Jobs[b.Job]; j != nil {
		jobName = j.Name.Get(lang(r))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"code":  encodeBuild(b),
		"notes": notes,
		"summary": map[string]any{
			"job": jobName, "base": b.BaseLv, "joblv": b.JobLv,
			"items": len(b.Equip), "skills": len(b.Skills),
		},
	})
}
