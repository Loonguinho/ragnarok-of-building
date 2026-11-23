package view

import (
	"strconv"
	"strings"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

// The Buffs window, laid out like latamtools': the class's active skills
// and effects, the learned passives that give bonuses, then the buffs.
type buffsView struct {
	Items    []effectItem // triggered effects of the equipped items
	Active   []buffItem   // class buffs and formula states (data-atk-state)
	Passives []learnItem  // passives with bonuses, at the learned level
	Groups   []buffGroup  // party buffs, consumables, other
	Pet      petView
}

type buffGroup struct {
	Name   string
	Items  []buffItem
	Active int  // buffs on in this group (shown on the closed group)
	Open   bool // the group starts open: support buffs, or any buff on
}

type buffItem struct {
	Key, Label string
	SkillID    int    // icon; 0 = none
	ItemID     int    // item icon (consumables)
	Excl       string // only one of the same Excl group at a time
	State      bool   // a class buff or formula state: kept in the attack states (data-atk-state)
	Toggle     bool   // Sim/Não buttons (OnValue is the "Sim" value)
	OnValue    string
	Choices    [][2]string // selector; nil = checkbox or number
	Max        int         // > 0: number input
	Value      int
	ValueStr   string
}

// effectItem is an equipped item's triggered effect (Bota Desconhecida
// FOR: STR +175 for a while on attack), counted when turned on.
type effectItem struct {
	Key, Label, Gives string
	ItemID            int
	Value             int
}

func itemEffectRows(r calc.Result, c *gamedata.Catalog, l string) []effectItem {
	var out []effectItem
	for _, ef := range r.ItemEffects {
		it := c.Items[ef.Item]
		if it == nil {
			continue
		}
		var gives []string
		for _, k := range sortedKeys(ef.Bonus) {
			lb, v := bonusText(k, ef.Bonus[k], c, l)
			gives = append(gives, strings.TrimSpace(lb+" "+v))
		}
		for _, k := range sortedKeys(ef.Extra) {
			lb, v := bonusText(k, ef.Extra[k], c, l)
			gives = append(gives, strings.TrimSpace(lb+" "+v))
		}
		label := it.Name.Get(l)
		if ef.Event {
			label += " (" + tr(l, "durante o evento") + ")"
		}
		row := effectItem{Key: ef.Key, Label: label, Gives: strings.Join(gives, ", "), ItemID: ef.Item}
		if ef.On {
			row.Value = 1
		}
		out = append(out, row)
	}
	return out
}

// learnItem is a passive in "Aprenda para ganhar bônus": its level is the
// skill tree's.
type learnItem struct {
	SkillID, Level, Max int
	Label               string
}

func buffsWindow(b calc.Build, c *gamedata.Catalog, l string) buffsView {
	v := buffsView{Groups: buffGroups(b, c, l)}
	if c == nil {
		return v
	}
	byCode := map[string]*gamedata.Skill{}
	byEN := map[string]*gamedata.Skill{}
	for _, s := range c.Skills {
		byCode[s.Code] = s
		byEN[calc.StateKeyOf(s.Name.Get("en"))] = s
	}
	seen := map[string]bool{}
	for _, cb := range calc.ClassBuffsFor(c, b.Job) {
		seen[cb.Key] = true
		it := buffItem{Key: cb.Key, Label: cb.Label, State: true, Excl: cb.Excl, Choices: cb.Choices}
		if s := byCode[cb.Skill]; s != nil {
			it.SkillID = s.ID
			if n := s.Name.Get(l); n != "" {
				it.Label = n
			}
		}
		if cb.Toggle {
			it.Toggle, it.Choices = true, nil
			if len(cb.Choices) > 1 {
				it.OnValue = cb.Choices[1][0]
			}
		}
		v.Active = append(v.Active, stateValue(it, b))
	}
	// Formula states (auras, stances... that only change skill formulas).
	for _, o := range calc.StatesFor(c, b.Job) {
		if seen[o.Key] {
			continue
		}
		seen[o.Key] = true
		it := buffItem{Key: o.Key, Label: tr(l, o.Label), State: true, Choices: o.Choices}
		s := byEN[o.Key]
		if code, ok := stateSkillAlias[o.Key]; ok {
			s = byCode[code]
		}
		if s == nil {
			s = prefixSkill(c, b.Job, o.Key)
		}
		if s != nil {
			it.SkillID = s.ID
			if n := s.Name.Get(l); n != "" {
				it.Label = n
			}
		}
		if len(o.Choices) == 0 {
			it.Toggle, it.OnValue = true, "1"
		} else if len(o.Choices) == 2 { // "—"/"Não" and one value: Sim/Não
			it.Toggle, it.OnValue, it.Choices = true, o.Choices[1][0], nil
			if o.Choices[0][0] != "0" {
				it.OnValue = o.Choices[0][0]
			}
		}
		v.Active = append(v.Active, stateValue(it, b))
	}
	for _, code := range calc.ClassPassiveSkills(c, b.Job) {
		s := byCode[code]
		if s == nil {
			continue
		}
		v.Passives = append(v.Passives, learnItem{SkillID: s.ID, Level: b.Skills[s.ID], Max: s.MaxLv, Label: s.Name.Get(l)})
	}
	return v
}

func stateValue(it buffItem, b calc.Build) buffItem {
	it.Value = b.Atk.States[it.Key]
	it.ValueStr = strconv.Itoa(it.Value)
	return it
}

// buffGroups lists the party buffs, consumables and other rows with the
// build's values. Buffs tied to a skill take its name from the client.
func buffGroups(b calc.Build, c *gamedata.Catalog, l string) []buffGroup {
	skills := map[string]*gamedata.Skill{}
	own := map[string]bool{}
	if c != nil {
		for _, s := range c.Skills {
			skills[s.Code] = s
		}
		own = calc.OwnBuffSkills(c, b.Job)
	}
	var out []buffGroup
	for _, d := range calc.Buffs {
		if d.Skill != "" && own[d.Skill] {
			continue // the class has it as its own buff (active skills above)
		}
		if len(out) == 0 || out[len(out)-1].Name != d.Group {
			out = append(out, buffGroup{Name: d.Group, Open: d.Group == "Suporte"})
		}
		label := tr(l, d.Label)
		var sid int
		if s := skills[d.Skill]; s != nil {
			sid = s.ID
			if s.Name.Get(l) != "" {
				label = s.Name.Get(l)
			}
		}
		v := b.Buffs[d.Key]
		g := &out[len(out)-1]
		g.Items = append(g.Items, buffItem{Key: d.Key, Label: label, SkillID: sid, Excl: d.Excl, Choices: d.Choices, Max: d.Max, Value: v, ValueStr: strconv.Itoa(v)})
		if v > 0 {
			g.Active++
			g.Open = true
		}
	}
	if c != nil {
		for i := range out {
			if out[i].Name != "Consumíveis" {
				continue
			}
			g := &out[i]
			for _, it := range calc.Consumables(c) {
				key := calc.ConsumableKey(it.ID)
				v := b.Buffs[key]
				g.Items = append(g.Items, buffItem{Key: key, Label: it.Name.Get(l), ItemID: it.ID, Excl: calc.ConsumableExcl(it.ID), Value: v, ValueStr: strconv.Itoa(v)})
				if v > 0 {
					g.Active++
					g.Open = true
				}
			}
		}
	}
	return out
}

// stateSkillAlias: formula states whose latam-ro-calc name is not the
// client's English skill name.
var stateSkillAlias = map[string]string{"s_judge": "IQ_JUDGE"}

// prefixSkill finds the class-tree skill whose English name starts like the
// state's ("Third Exor Flame" -> "Third Exorcism Flame"): at least 8 letters.
func prefixSkill(c *gamedata.Catalog, job int, key string) *gamedata.Skill {
	want := strings.TrimPrefix(key, "s_")
	var best *gamedata.Skill
	bestN := 7
	for _, s := range calc.ChainSkills(c, job) {
		have := strings.TrimPrefix(calc.StateKeyOf(s.Name.Get("en")), "s_")
		n := 0
		for n < len(want) && n < len(have) && want[n] == have[n] {
			n++
		}
		if n > bestN {
			best, bestN = s, n
		}
	}
	return best
}
