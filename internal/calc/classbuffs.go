package calc

import (
	"strconv"

	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/script"
)

// ClassBuff is one of a class's own buffs (Encantar Lâmina, Postura de
// Ataque...), generated from latam-ro-calc by tools/latamgen/buffgen.py.
// Its key is the formula state's (stateKey(Name)): the same selector turns
// on the bonuses and the formulas that check the buff.
type ClassBuff struct {
	Key     string
	Name    string // latam-ro-calc's skill name
	Skill   string // client skill code, "" when unknown
	Label   string // latam-ro-calc's label (the client name wins when Skill is set)
	Toggle  bool   // on/off: the one option is the "on" value
	Choices [][2]string
	Scripts map[int]string // value -> rAthena script
}

var latamClassBuffs = map[string][]ClassBuff{}

func regClassBuffs(job string, bs []ClassBuff) {
	for i := range bs {
		bs[i].Key = stateKey(bs[i].Name)
	}
	latamClassBuffs[job] = append(latamClassBuffs[job], bs...)
}

// ClassBuffsFor lists the own buffs of a job and its ancestors (the job's
// own first wins when two classes share a buff).
func ClassBuffsFor(c *gamedata.Catalog, jobID int) []ClassBuff {
	var out []ClassBuff
	seen := map[string]bool{}
	chain := c.Chain(jobID)
	for i := len(chain) - 1; i >= 0; i-- {
		j := c.Jobs[chain[i]]
		if j == nil {
			continue
		}
		for _, b := range latamClassBuffs[j.Code] {
			if !seen[b.Key] {
				seen[b.Key] = true
				out = append(out, b)
			}
		}
	}
	return out
}

// script returns the bonuses of value v (a toggle's checkbox sends 1).
func (b *ClassBuff) script(v int) string {
	if v == 0 {
		return ""
	}
	if s, ok := b.Scripts[v]; ok {
		return s
	}
	if b.Toggle {
		for _, s := range b.Scripts {
			return s
		}
	}
	return ""
}

// validValue reports whether v is one of the buff's options (1 = on for a toggle).
func (b *ClassBuff) validValue(v int) bool {
	if b.Toggle && v == 1 {
		return true
	}
	for _, c := range b.Choices {
		if c[0] == strconv.Itoa(v) {
			return true
		}
	}
	return false
}

// applyClassBuffs adds the bonuses of the class buffs turned on (kept in
// the attack states).
func applyClassBuffs(b *Build, c *gamedata.Catalog, bonus, extra map[string]int) {
	if len(b.Atk.States) == 0 {
		return
	}
	for _, cb := range ClassBuffsFor(c, b.Job) {
		src := cb.script(b.Atk.States[cb.Key])
		if src == "" {
			continue
		}
		res := script.Eval(src, script.Context{BaseLv: b.BaseLv, JobLv: b.JobLv, Class: b.Job})
		addAll(bonus, res.Bonus)
		addAll(extra, res.Extra)
	}
}

// ClassPassive is a class passive (Perícia com Espada, Treino com Espadas...)
// generated from latam-ro-calc: it counts at the level the build learned
// the skill. Each effect is {weapon type ("" = any), rAthena script}.
type ClassPassive struct {
	Name   string
	Skill  string // client skill code
	Levels map[int][][2]string
}

var latamClassPassives = map[string][]ClassPassive{}

func regClassPassives(job string, ps []ClassPassive) {
	latamClassPassives[job] = append(latamClassPassives[job], ps...)
}

// classPassivesFor lists a job's passives and its ancestors' (the job's own
// definition of a skill wins).
func classPassivesFor(c *gamedata.Catalog, jobID int) []ClassPassive {
	var out []ClassPassive
	seen := map[string]bool{}
	chain := c.Chain(jobID)
	for i := len(chain) - 1; i >= 0; i-- {
		j := c.Jobs[chain[i]]
		if j == nil {
			continue
		}
		for _, p := range latamClassPassives[j.Code] {
			if !seen[p.Skill] {
				seen[p.Skill] = true
				out = append(out, p)
			}
		}
	}
	return out
}

// applyClassPassives adds the learned passives' bonuses for the weapon in
// hand. A level the table lacks uses the highest one below it.
func applyClassPassives(b *Build, c *gamedata.Catalog, weapon string, bonus, extra map[string]int) {
	for _, p := range classPassivesFor(c, b.Job) {
		lv := learnedLv(*b, c, p.Skill)
		if lv <= 0 {
			continue
		}
		effs, best := p.Levels[lv], 0
		if effs == nil {
			for l := range p.Levels {
				if l <= lv && l > best {
					best = l
				}
			}
			effs = p.Levels[best]
		}
		for _, e := range effs {
			if e[0] != "" && e[0] != weapon {
				continue
			}
			res := script.Eval(e[1], script.Context{BaseLv: b.BaseLv, JobLv: b.JobLv, Class: b.Job})
			addAll(bonus, res.Bonus)
			addAll(extra, res.Extra)
		}
	}
}
