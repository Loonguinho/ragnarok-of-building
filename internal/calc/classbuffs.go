package calc

import (
	"slices"
	"strconv"
	"strings"

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
	Excl    string // buffs that cannot be on together share it
	Choices [][2]string
	Scripts map[int]string // value -> rAthena script
	// AutoSkill: a support buff the character casts on itself, on by
	// itself at the level learned in the skill tree (this skill's ID); the
	// value autoOff turns it off ("—").
	AutoSkill int
}

// autoOff is an automatic buff turned off by the player.
const autoOff = -1

var latamClassBuffs = map[string][]ClassBuff{}

func regClassBuffs(job string, bs []ClassBuff) {
	for i := range bs {
		bs[i].Key = stateKey(bs[i].Name)
	}
	latamClassBuffs[job] = append(latamClassBuffs[job], bs...)
}

// ClassBuffsFor lists the own buffs of a job and its ancestors (the job's
// own first wins when two classes share a buff). Buffs whose client
// description has a level table (Skill.Effects) use those LATAM values; the
// latam-ro-calc ones fill in the rest. Party buffs of the Buffs window are
// left out (they are there already).
func ClassBuffsFor(c *gamedata.Catalog, jobID int) []ClassBuff {
	var out []ClassBuff
	seen := map[string]bool{}
	index := map[string]int{} // skill code -> position in out
	chain := c.Chain(jobID)
	for i := len(chain) - 1; i >= 0; i-- {
		j := c.Jobs[chain[i]]
		if j == nil {
			continue
		}
		for _, b := range latamClassBuffs[j.Code] {
			if !seen[b.Key] {
				seen[b.Key] = true
				if b.Skill != "" {
					index[b.Skill] = len(out)
				}
				out = append(out, b)
			}
		}
	}
	for _, s := range chainSkills(c, chain) {
		e := s.Effects
		k, own := index[s.Code]
		// A party buff of the Buffs window stays there, unless the class has
		// it as its own buff (the Shura's Chakras): then this row replaces it.
		if e == nil || e.Kind != "buff" || (partyBuffSkills[s.Code] && !own) {
			continue
		}
		cb := ClassBuff{Name: s.Name.Get("en"), Skill: s.Code, Label: s.Name.Get("pt"), Excl: e.Group,
			Choices: [][2]string{{"0", "—"}}, Scripts: e.Levels}
		if g, ok := buffExcl[s.Code]; ok {
			cb.Excl = g
		}
		if extra := buffExtra[s.Code]; extra != "" {
			scripts := map[int]string{}
			for lv, sc := range cb.Scripts {
				scripts[lv] = sc + " " + extra
			}
			cb.Scripts = scripts
		}
		if autoBuffs[s.Code] {
			cb.AutoSkill = s.ID
			cb.Choices[0] = [2]string{strconv.Itoa(autoOff), "—"}
		}
		lvs := make([]int, 0, len(e.Levels))
		for lv := range e.Levels {
			lvs = append(lvs, lv)
		}
		slices.Sort(lvs)
		for _, lv := range lvs {
			cb.Choices = append(cb.Choices, [2]string{strconv.Itoa(lv), "Nv. " + strconv.Itoa(lv)})
		}
		cb.Toggle = len(lvs) == 1
		if own { // the client's values replace latam-ro-calc's
			cb.Key = out[k].Key // same selector as the formula state
			// Keep latam-ro-calc's skill damage bonuses, which the client
			// writes in the text ("O dano de Pancada Corporal aumenta em 30%").
			scripts := map[int]string{}
			for lv, sc := range cb.Scripts {
				scripts[lv] = sc
				if extra := skillAtkParts(out[k].script(lv)); extra != "" {
					scripts[lv] += " " + extra
				}
			}
			cb.Scripts = scripts
			out[k] = cb
			continue
		}
		cb.Key = stateKey(cb.Name)
		if seen[cb.Key] {
			continue
		}
		seen[cb.Key] = true
		out = append(out, cb)
	}
	return out
}

// chainSkills are the skills of the job chain's skill trees.
func chainSkills(c *gamedata.Catalog, chain []int) []*gamedata.Skill {
	var out []*gamedata.Skill
	seen := map[int]bool{}
	for i := len(chain) - 1; i >= 0; i-- {
		j := c.Jobs[chain[i]]
		if j == nil {
			continue
		}
		pos := make([]int, 0, len(j.Tree))
		for p := range j.Tree {
			pos = append(pos, p)
		}
		slices.Sort(pos)
		for _, p := range pos {
			id := j.Tree[p]
			if s := c.Skills[id]; s != nil && !seen[id] {
				seen[id] = true
				out = append(out, s)
			}
		}
	}
	return out
}

// partyBuffSkills are the skills of the Buffs window's own rows.
var partyBuffSkills = func() map[string]bool {
	m := map[string]bool{}
	for _, d := range Buffs {
		if d.Skill != "" {
			m[d.Skill] = true
		}
	}
	return m
}()

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
// definition of a skill wins). A passive whose client description has a
// level table uses it (LATAM values) instead of latam-ro-calc's.
func classPassivesFor(c *gamedata.Catalog, jobID int) []ClassPassive {
	var out []ClassPassive
	index := map[string]int{}
	chain := c.Chain(jobID)
	for i := len(chain) - 1; i >= 0; i-- {
		j := c.Jobs[chain[i]]
		if j == nil {
			continue
		}
		for _, p := range latamClassPassives[j.Code] {
			if _, ok := index[p.Skill]; !ok {
				index[p.Skill] = len(out)
				out = append(out, p)
			}
		}
	}
	for _, s := range chainSkills(c, chain) {
		e := s.Effects
		if e == nil || e.Kind != "passive" {
			continue
		}
		p := ClassPassive{Name: s.Name.Get("en"), Skill: s.Code, Levels: map[int][][2]string{}}
		for lv, sc := range e.Levels {
			if len(e.Weapons) == 0 {
				p.Levels[lv] = [][2]string{{"", sc}}
				continue
			}
			for _, w := range e.Weapons {
				p.Levels[lv] = append(p.Levels[lv], [2]string{w, sc})
			}
		}
		if k, ok := index[s.Code]; ok {
			out[k] = p
		} else {
			index[s.Code] = len(out)
			out = append(out, p)
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
			if ws := passiveWeapons[p.Skill]; e[0] == "" && ws != nil && !slices.Contains(ws, weapon) {
				continue
			}
			res := script.Eval(e[1], script.Context{BaseLv: b.BaseLv, JobLv: b.JobLv, Class: b.Job})
			addAll(bonus, res.Bonus)
			addAll(extra, res.Extra)
		}
	}
}

// skillAtkParts keeps the per-skill damage bonuses of a script.
func skillAtkParts(src string) string {
	var out []string
	for _, st := range strings.Split(src, ";") {
		if st = strings.TrimSpace(st); strings.HasPrefix(st, "bonus2 bSkillAtk,") {
			out = append(out, st+";")
		}
	}
	return strings.Join(out, " ")
}

// OwnBuffSkills are the skills a job has among its own buffs: the Buffs
// window hides their party rows (one control per buff).
func OwnBuffSkills(c *gamedata.Catalog, jobID int) map[string]bool {
	m := map[string]bool{}
	for _, cb := range ClassBuffsFor(c, jobID) {
		if cb.Skill != "" {
			m[cb.Skill] = true
		}
	}
	return m
}

// StateKeyOf is the state key of a buff or skill name (as ClassBuff.Key).
func StateKeyOf(name string) string { return stateKey(name) }

// ClassPassiveSkills are the codes of the job's passives that give bonuses
// (the Buffs window's "Aprenda para ganhar bônus"), in tree order.
func ClassPassiveSkills(c *gamedata.Catalog, jobID int) []string {
	have := map[string]bool{}
	for _, p := range classPassivesFor(c, jobID) {
		have[p.Skill] = true
	}
	var out []string
	for _, s := range chainSkills(c, c.Chain(jobID)) {
		if have[s.Code] {
			out = append(out, s.Code)
			delete(have, s.Code)
		}
	}
	return out
}

// ChainSkills are the skills of a job's and its ancestors' trees.
func ChainSkills(c *gamedata.Catalog, jobID int) []*gamedata.Skill {
	return chainSkills(c, c.Chain(jobID))
}
