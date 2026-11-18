package clientimport

import (
	"path/filepath"

	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/grf"
	"ragnarok-of-building/internal/lua51"
	"ragnarok-of-building/internal/rathena"
)

// ImportRandomOpts joins rAthena's random option scripts with the client's
// option texts (addrandomoptionnametable, keyed by the same IDs) in each
// language.
func ImportRandomOpts(dir string, c *gamedata.Catalog, opts map[int]*rathena.RandomOpt) (int, error) {
	c.RandomOpts = map[int]*gamedata.RandomOpt{}
	for id, o := range opts {
		c.RandomOpts[id] = &gamedata.RandomOpt{ID: id, Var: o.Var, Script: o.Script, Name: gamedata.Text{}}
	}
	a, err := grf.Open(filepath.Join(dir, "data.grf"))
	if err != nil {
		return 0, err
	}
	defer a.Close()
	base, err := loadGRF(a, nil, `data\`+luaDir+`datainfo\enumvar.lub`)
	if err != nil {
		return 0, err
	}
	named := 0
	for _, l := range langDirs {
		e, err := loadGRF(a, base, `data\`+luaDir+`datainfo\addrandomoptionnametable_`+l.suffix+`.lub`)
		if err != nil {
			continue
		}
		t, ok := e["NameTable_VAR"].(*lua51.Table)
		if !ok {
			continue
		}
		for _, id := range t.IntKeys() {
			if o := c.RandomOpts[id]; o != nil {
				if s := str(t.Get(float64(id))); s != "" {
					if len(o.Name) == 0 {
						named++
					}
					o.Name[l.lang] = s
				}
			}
		}
	}
	return named, nil
}
