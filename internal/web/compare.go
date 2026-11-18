package web

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

// maxCompare is how many builds fit side by side.
const maxCompare = 4

type compareCol struct {
	Name, Job, Levels string
	Skill             string // attack used for the damage rows
	Valid             bool
}

type compareCell struct {
	Text string
	Best bool
}

type compareRow struct {
	Label string
	Group string // first row of a group carries its title
	Cells []compareCell
}

type compareTable struct {
	Cols   []compareCol
	Rows   []compareRow
	Target *gamedata.Monster
}

// row adds a line of numbers; the best value (highest, or lowest when
// lowerBetter) is highlighted when the builds differ.
func (t *compareTable) row(group, label string, vals []float64, text func(i int) string, lowerBetter bool) {
	r := compareRow{Label: label, Group: group}
	best, differ := math.NaN(), false
	for i, v := range vals {
		if !t.Cols[i].Valid || math.IsNaN(v) {
			continue
		}
		if !math.IsNaN(best) && v != best {
			differ = true
		}
		if math.IsNaN(best) || (lowerBetter && v < best) || (!lowerBetter && v > best) {
			best = v
		}
	}
	for i, v := range vals {
		c := compareCell{Text: "—"}
		if t.Cols[i].Valid && !math.IsNaN(v) {
			c.Text = text(i)
			c.Best = differ && len(vals) > 1 && v == best
		}
		r.Cells = append(r.Cells, c)
	}
	t.Rows = append(t.Rows, r)
}

// compare computes the builds against one target (0 = the first build's).
func (s *Server) compare(codes, names []string, target int, l string) compareTable {
	c := s.opt.Catalog
	n := len(codes)
	views := make([]buildView, n)
	var t compareTable
	if target == 0 {
		for _, code := range codes {
			if b, ok := decodeBuild(code); ok && b.Target != 0 {
				target = b.Target
				break
			}
		}
	}
	s.loadTarget(target, l)
	t.Target = c.Monster(target)
	for i, code := range codes {
		col := compareCol{Name: names[i]}
		if b, ok := decodeBuild(code); ok {
			b.Target = target
			views[i] = s.buildView(b, l)
			v := views[i]
			col.Valid = true
			col.Job = v.JobName
			col.Levels = fmt.Sprintf("%d / %d", v.Build.BaseLv, v.Build.JobLv)
			col.Skill = tr(l, "Ataque básico")
			if sk := c.Skills[v.Build.Atk.Skill]; sk != nil {
				col.Skill = fmt.Sprintf("%s %s %d", sk.Name.Get(l), tr(l, "Nv."), v.Build.Atk.Level)
			}
			if col.Name == "" {
				col.Name = col.Job
			}
		}
		t.Cols = append(t.Cols, col)
	}
	num := func(f func(v buildView) float64) []float64 {
		out := make([]float64, n)
		for i := range views {
			if t.Cols[i].Valid {
				out[i] = f(views[i])
			}
		}
		return out
	}
	itoa := func(f func(v buildView) int) func(int) string {
		return func(i int) string { return fmtInt(int64(f(views[i]))) }
	}
	// Stats and traits (final values).
	for k, name := range statNames {
		k := k
		get := func(v buildView) int { return v.R.FinalStats()[k] }
		t.row(groupIf(k == 0, "Atributos"), name, num(func(v buildView) float64 { return float64(get(v)) }), itoa(get), false)
	}
	anyTraits := false
	for _, v := range views {
		if v.Job != nil && v.Job.Fourth {
			anyTraits = true
		}
	}
	if anyTraits {
		for k, name := range traitNames {
			k := k
			get := func(v buildView) int { return v.R.FinalTraits()[k] }
			t.row("", name, num(func(v buildView) float64 { return float64(get(v)) }), itoa(get), false)
		}
	}
	// Status.
	type stat struct {
		label string
		get   func(v buildView) float64
		text  func(v buildView) string
		lower bool
	}
	plus := func(a, b int) string { return fmtInt(int64(a)) + " + " + fmtInt(int64(b)) }
	stats := []stat{
		{"HP", func(v buildView) float64 { return float64(v.R.HP) }, nil, false},
		{"SP", func(v buildView) float64 { return float64(v.R.SP) }, nil, false},
		{"ATQ", func(v buildView) float64 { return float64(v.R.StatusATK + v.R.EquipATK) }, func(v buildView) string { return plus(v.R.StatusATK, v.R.EquipATK) }, false},
		{"ATQM", func(v buildView) float64 { return float64(v.R.MATKMin + v.R.EquipMATK) }, func(v buildView) string { return plus(v.R.MATKMin, v.R.EquipMATK) }, false},
		{"Precisão", func(v buildView) float64 { return float64(v.R.HIT) }, nil, false},
		{"Crítico", func(v buildView) float64 { return v.R.CRIT }, func(v buildView) string { return strconv.FormatFloat(v.R.CRIT, 'f', 1, 64) }, false},
		{"Esquiva", func(v buildView) float64 { return float64(v.R.FLEE) }, func(v buildView) string { return plus(v.R.FLEE, v.R.FLEE2) }, false},
		{"ASPD", func(v buildView) float64 { return float64(v.R.ASPD) }, nil, false},
		{"DEF", func(v buildView) float64 { return float64(v.R.HardDEF + v.R.SoftDEF) }, func(v buildView) string { return plus(v.R.HardDEF, v.R.SoftDEF) }, false},
		{"DEFM", func(v buildView) float64 { return float64(v.R.HardMDEF + v.R.SoftMDEF) }, func(v buildView) string { return plus(v.R.HardMDEF, v.R.SoftMDEF) }, false},
	}
	if anyTraits {
		stats = append(stats,
			stat{"P.ATQ", func(v buildView) float64 { return float64(v.R.PATK) }, nil, false},
			stat{"S.ATQM", func(v buildView) float64 { return float64(v.R.SMATK) }, nil, false},
			stat{"RES", func(v buildView) float64 { return float64(v.R.RES) }, nil, false},
			stat{"MRES", func(v buildView) float64 { return float64(v.R.MRES) }, nil, false},
		)
	}
	for k, st := range stats {
		st := st
		text := func(i int) string { return fmtInt(int64(st.get(views[i]))) }
		if st.text != nil {
			text = func(i int) string { return st.text(views[i]) }
		}
		t.row(groupIf(k == 0, "Status"), st.label, num(st.get), text, st.lower)
	}
	// Damage against the target, each build with its own attack.
	if t.Target != nil {
		dm := func(f func(d calc.DamageResult) float64) []float64 {
			return num(func(v buildView) float64 { return f(v.D.Dmg) })
		}
		skillRow := compareRow{Label: "Ataque", Group: "Dano contra o alvo"}
		for _, col := range t.Cols {
			skillRow.Cells = append(skillRow.Cells, compareCell{Text: col.Skill})
		}
		t.Rows = append(t.Rows, skillRow)
		t.row("", "Habilidade", num(func(v buildView) float64 { return float64(v.D.Ratio) }), func(i int) string {
			return fmtInt(int64(views[i].D.Ratio)) + "% × " + strconv.Itoa(max(1, views[i].D.Hits))
		}, false)
		t.row("", "Dano normal", dm(func(d calc.DamageResult) float64 { return float64(d.Avg) }), func(i int) string {
			d := views[i].D.Dmg
			return fmtInt(d.Min) + " ~ " + fmtInt(d.Max)
		}, false)
		crit := dm(func(d calc.DamageResult) float64 {
			if d.Crit == 0 {
				return math.NaN()
			}
			return float64(d.Crit)
		})
		t.row("", "Crítico", crit, func(i int) string { return fmtInt(views[i].D.Dmg.Crit) }, false)
		t.row("", "Chance de acerto", dm(func(d calc.DamageResult) float64 { return float64(d.HitChance) }), func(i int) string {
			return strconv.Itoa(views[i].D.Dmg.HitChance) + "%"
		}, false)
		t.row("", "Dano esperado", dm(func(d calc.DamageResult) float64 { return d.Expected }), func(i int) string {
			return fmtInt(int64(views[i].D.Dmg.Expected + 0.5))
		}, false)
		t.row("", "Usos para matar", dm(func(d calc.DamageResult) float64 {
			if d.HitsToKill <= 0 {
				return math.NaN()
			}
			return float64(d.HitsToKill)
		}), func(i int) string { return strconv.Itoa(views[i].D.Dmg.HitsToKill) }, true)
	}
	return t
}

func groupIf(first bool, g string) string {
	if first {
		return g
	}
	return ""
}

// GET /compare?b=CODE&b=CODE&m=MONSTER — the comparison window.
func (s *Server) handleCompare(w http.ResponseWriter, r *http.Request) {
	l := lang(r)
	data := map[string]any{"Title": "Comparar Builds", "Lang": l, "Max": maxCompare,
		"Ready": s.opt.Catalog != nil && len(s.opt.Catalog.StatPoints) > 0}
	s.render(w, r, "compare", data)
}

// POST /compare/calc — JSON {builds: [{code, name}], m} -> {html, target}.
func (s *Server) handleCompareCalc(w http.ResponseWriter, r *http.Request) {
	if s.opt.Catalog == nil {
		http.Error(w, "catalog not loaded", http.StatusServiceUnavailable)
		return
	}
	var in struct {
		Builds []struct {
			Code string `json:"code"`
			Name string `json:"name"`
		} `json:"builds"`
		M int `json:"m"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxCompare*maxBuildBody+1024)).Decode(&in); err != nil || len(in.Builds) == 0 || len(in.Builds) > maxCompare {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	codes, names := make([]string, len(in.Builds)), make([]string, len(in.Builds))
	for i, b := range in.Builds {
		codes[i], names[i] = b.Code, b.Name
		if len([]rune(names[i])) > 40 {
			names[i] = string([]rune(names[i])[:40])
		}
	}
	l := lang(r)
	t := s.compare(codes, names, in.M, l)
	html, err := s.fragmentString(l, "compare", "compare_table", t)
	if err != nil {
		s.opt.Log.Error("compare fragment", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	out := map[string]any{"html": html}
	if t.Target != nil {
		out["target"] = map[string]any{"id": t.Target.ID, "name": t.Target.NameIn(l)}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}
