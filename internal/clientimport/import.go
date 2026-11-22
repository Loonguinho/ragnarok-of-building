// Package clientimport builds the game catalog from an installed RO LATAM client.
package clientimport

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"

	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/grf"
	"ragnarok-of-building/internal/lua51"
)

type env = map[string]lua51.Value

// langDirs maps catalog languages to the client's per-language folders/suffixes.
var langDirs = []struct {
	lang, sysDir, grfDir, suffix string
}{
	{"pt", "", "", "ptbr"},
	{"en", "english", `english\`, "enus"},
	{"es", "spanish", `spanish\`, "eses"},
}

const luaDir = `luafiles514\lua files\`

// Import reads the client at dir (e.g. C:\Gravity\Ragnarok).
func Import(dir string) (*gamedata.Catalog, error) {
	a, err := grf.Open(filepath.Join(dir, "data.grf"))
	if err != nil {
		return nil, err
	}
	defer a.Close()

	c := &gamedata.Catalog{
		Generated: time.Now().UTC(),
		Source:    "RO LATAM client",
		Items:     map[int]*gamedata.Item{},
		Skills:    map[int]*gamedata.Skill{},
		Jobs:      map[int]*gamedata.Job{},
	}
	if err := importItems(dir, c); err != nil {
		return nil, err
	}
	base, err := loadGRF(a, nil, `data\`+luaDir+`datainfo\jobidentity.lub`,
		`data\`+luaDir+`skillinfoz\skillid.lub`,
		`data\`+luaDir+`skillinfoz\jobinheritlist.lub`)
	if err != nil {
		return nil, err
	}
	if err := importSkills(a, base, c); err != nil {
		return nil, err
	}
	if err := importJobs(a, base, c); err != nil {
		return nil, err
	}
	return c, nil
}

// loadGRF runs .lub files from the archive in order on a copy of start.
// Partial runs (code after the data section) are accepted.
func loadGRF(a *grf.Archive, start env, paths ...string) (env, error) {
	e := env{}
	for k, v := range start {
		e[k] = v
	}
	for _, p := range paths {
		b, err := a.Read(p)
		if err != nil {
			return nil, err
		}
		if _, err := lua51.LoadEnv(b, e); err != nil {
			var pe *lua51.PartialError
			if !errors.As(err, &pe) {
				return nil, fmt.Errorf("%s: %w", p, err)
			}
		}
	}
	return e, nil
}

func importItems(dir string, c *gamedata.Catalog) error {
	for _, l := range langDirs {
		p := filepath.Join(dir, "System", l.sysDir, "iteminfo_new.lub")
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		g, err := lua51.Load(b)
		if err != nil {
			var pe *lua51.PartialError
			if !errors.As(err, &pe) {
				return fmt.Errorf("%s: %w", p, err)
			}
		}
		tbl, ok := g["tbl"].(*lua51.Table)
		if !ok {
			return fmt.Errorf("%s: no item table", p)
		}
		for _, id := range tbl.IntKeys() {
			row, ok := tbl.Get(float64(id)).(*lua51.Table)
			if !ok {
				continue
			}
			it := c.Items[id]
			if it == nil {
				it = &gamedata.Item{ID: id, Name: gamedata.Text{}, Desc: gamedata.Lines{}}
				c.Items[id] = it
			}
			it.Name[l.lang] = str(row.Get("identifiedDisplayName"))
			it.Desc[l.lang] = lines(row.Get("identifiedDescriptionName"))
			if l.lang == "pt" {
				it.Slots = num(row.Get("slotCount"))
				it.ClassNum = num(row.Get("ClassNum"))
				it.Resource = str(row.Get("identifiedResourceName"))
			}
		}
	}
	return nil
}

func importSkills(a *grf.Archive, base env, c *gamedata.Catalog) error {
	g, err := loadGRF(a, base, `data\`+luaDir+`skillinfoz\skillinfolist.lub`)
	if err != nil {
		return err
	}
	info, ok := g["SKILL_INFO_LIST"].(*lua51.Table)
	if !ok {
		return errors.New("SKILL_INFO_LIST missing")
	}
	for _, id := range info.IntKeys() {
		row, ok := info.Get(float64(id)).(*lua51.Table)
		if !ok {
			continue
		}
		s := &gamedata.Skill{
			ID:    id,
			Code:  str(row.Get(1.0)),
			Name:  gamedata.Text{"pt": str(row.Get("SkillName"))},
			Desc:  gamedata.Lines{},
			MaxLv: num(row.Get("MaxLv")),
			SP:    ints(row.Get("SpAmount")),
			Range: ints(row.Get("AttackRange")),
			Needs: reqs(row.Get("_NeedSkillList")),
			Type:  str(row.Get("Type")),
		}
		if byJob, ok := row.Get("NeedSkillList").(*lua51.Table); ok {
			s.NeedsByJob = map[int][]gamedata.Req{}
			for _, job := range byJob.IntKeys() {
				s.NeedsByJob[job] = reqs(byJob.Get(float64(job)))
			}
		}
		c.Skills[id] = s
	}
	// skillinfolist_data.lub (SkillInfoList_data) is the current table since
	// the 2026 rebalance: new skills (Espinhos Caóticos, Baforada
	// Draconiana...) are only there, and it has the new SP costs. Its rows
	// have no name (the localized lists give it).
	if g, err := loadGRF(a, base, `data\`+luaDir+`skillinfoz\skillinfolist_data.lub`); err == nil {
		if data, ok := g["SkillInfoList_data"].(*lua51.Table); ok {
			for _, id := range data.IntKeys() {
				row, ok := data.Get(float64(id)).(*lua51.Table)
				if !ok {
					continue
				}
				s := c.Skills[id]
				if s == nil {
					s = &gamedata.Skill{ID: id, Code: str(row.Get(1.0)), Name: gamedata.Text{}, Desc: gamedata.Lines{}}
					c.Skills[id] = s
				}
				if v := num(row.Get("MaxLv")); v > 0 {
					s.MaxLv = v
				}
				if v := ints(row.Get("SpAmount")); len(v) > 0 {
					s.SP = v
				}
				if v := ints(row.Get("AttackRange")); len(v) > 0 {
					s.Range = v
				}
				if row.Get("_NeedSkillList") != nil {
					s.Needs = reqs(row.Get("_NeedSkillList"))
				}
				if byJob, ok := row.Get("NeedSkillList").(*lua51.Table); ok {
					s.NeedsByJob = map[int][]gamedata.Req{}
					for _, job := range byJob.IntKeys() {
						s.NeedsByJob[job] = reqs(byJob.Get(float64(job)))
					}
				}
				if t := str(row.Get("Type")); t != "" {
					s.Type = t
				}
			}
		}
	}

	for _, l := range langDirs {
		// Localized names.
		if g, err := loadGRF(a, base, `data\`+luaDir+`skillinfoz\skillinfolist_`+l.suffix+`.lub`); err == nil {
			if t, ok := g["SkillInfoList_string"].(*lua51.Table); ok {
				for _, id := range t.IntKeys() {
					if s, row := c.Skills[id], tab(t.Get(float64(id))); s != nil && row != nil {
						if n := str(row.Get("SkillName")); n != "" {
							s.Name[l.lang] = n
						}
					}
				}
			}
		}
		// Descriptions.
		g, err := loadGRF(a, base, `data\`+l.grfDir+luaDir+`skillinfoz\skilldescript.lub`)
		if err != nil {
			continue
		}
		if t, ok := g["SKILL_DESCRIPT"].(*lua51.Table); ok {
			for _, id := range t.IntKeys() {
				if s := c.Skills[id]; s != nil {
					s.Desc[l.lang] = lines(t.Get(float64(id)))
				}
			}
		}
	}
	return nil
}

func importJobs(a *grf.Archive, base env, c *gamedata.Catalog) error {
	g, err := loadGRF(a, base, `data\`+luaDir+`skillinfoz\skilltreeview.lub`)
	if err != nil {
		return err
	}
	trees, ok := g["SKILL_TREEVIEW_FOR_JOB"].(*lua51.Table)
	if !ok {
		return errors.New("SKILL_TREEVIEW_FOR_JOB missing")
	}
	inherit, _ := base["JOB_INHERIT_LIST"].(*lua51.Table)
	codes := map[int]string{}
	if jobid, ok := base["JOBID"].(*lua51.Table); ok {
		for k, v := range jobid.Hash {
			if name, ok := k.(string); ok {
				if _, dup := codes[num(v)]; !dup || len(name) < len(codes[num(v)]) {
					codes[num(v)] = name
				}
			}
		}
	}

	ensure := func(id int) *gamedata.Job {
		j := c.Jobs[id]
		if j == nil {
			j = &gamedata.Job{ID: id, Code: codes[id], Name: gamedata.Text{}, Parent: -1, Tree: map[int]int{}}
			if inherit != nil {
				if p, ok := inherit.Get(float64(id)).(float64); ok && int(p) != id {
					j.Parent = int(p)
				}
			}
			c.Jobs[id] = j
		}
		return j
	}
	for _, id := range trees.IntKeys() {
		j := ensure(id)
		row := tab(trees.Get(float64(id)))
		if row == nil {
			continue
		}
		for _, pos := range row.IntKeys() {
			j.Tree[pos] = num(row.Get(float64(pos)))
		}
	}
	// Make sure every ancestor exists so chains are complete.
	for _, j := range c.Jobs {
		for p := j.Parent; p >= 0 && c.Jobs[p] == nil; p = ensure(p).Parent {
		}
	}
	// Transcendent (_H) and baby (_B) variants share the base job's tab; the
	// client assigns them in code (JobSkillTab) that the data VM does not run.
	byCode := map[string]*gamedata.Job{}
	for _, j := range c.Jobs {
		byCode[j.Code] = j
	}
	for _, j := range c.Jobs {
		if len(j.Tree) > 0 || len(j.Code) < 3 {
			continue
		}
		if suf := j.Code[len(j.Code)-2:]; suf == "_H" || suf == "_B" {
			if base := byCode[j.Code[:len(j.Code)-2]]; base != nil {
				maps.Copy(j.Tree, base.Tree)
			}
		}
	}

	for _, l := range langDirs {
		g, err := loadGRF(a, base, `data\`+luaDir+`datainfo\pcjobnamegender_`+l.suffix+`.lub`)
		if err != nil {
			continue
		}
		names, ok := g["PCJobNameTableMan"].(*lua51.Table)
		if !ok {
			continue
		}
		for id, j := range c.Jobs {
			if n := str(names.Get(float64(id))); n != "" {
				j.Name[l.lang] = n
			}
		}
	}
	return nil
}

// ---- value helpers ----

var cp1252 = charmap.Windows1252.NewDecoder()

// str returns a Lua string as UTF-8. Some client files are CP1252, others UTF-8.
func str(v lua51.Value) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	if utf8.ValidString(s) {
		return s
	}
	if out, err := cp1252.String(s); err == nil {
		return out
	}
	return s
}

func num(v lua51.Value) int {
	f, ok := v.(float64)
	if !ok || math.IsNaN(f) {
		return 0
	}
	return int(f)
}

func tab(v lua51.Value) *lua51.Table {
	t, _ := v.(*lua51.Table)
	return t
}

func lines(v lua51.Value) []string {
	t := tab(v)
	if t == nil {
		return nil
	}
	arr := t.Array()
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		out = append(out, str(e))
	}
	return out
}

func ints(v lua51.Value) []int {
	t := tab(v)
	if t == nil {
		return nil
	}
	arr := t.Array()
	out := make([]int, len(arr))
	for i, e := range arr {
		out[i] = num(e)
	}
	return out
}

func reqs(v lua51.Value) []gamedata.Req {
	t := tab(v)
	if t == nil {
		return nil
	}
	var out []gamedata.Req
	for _, e := range t.Array() {
		if pair := ints(e); len(pair) >= 2 {
			out = append(out, gamedata.Req{Skill: pair[0], Level: pair[1]})
		}
	}
	return out
}
