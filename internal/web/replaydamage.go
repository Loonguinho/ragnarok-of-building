package web

import (
	"fmt"
	"strings"

	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/replay"
)

// The replay damage window: what each character in the recording dealt to
// monsters, shown after a replay import when the recording has damage. The
// recording character comes first; a selector at the top switches to the
// others.

type rdmgView struct {
	Recorded, Duration string
	Chars              []rdmgChar
}

type rdmgChar struct {
	Name, Job   string
	Self        bool
	Total, DPS  string
	Hits, Kills int
	Skills      []rdmgSkill
	Monsters    []rdmgMob
}

type rdmgSkill struct {
	ID              int // 0 = basic attack
	Name            string
	Total, Avg, Max string
	Hits, Monsters  int
	Pct             float64 // share of the character's total or of the monster's damage
	PctS            string
}

type rdmgMob struct {
	ID                 int
	Name               string
	Total              string
	Hits, Kills, Count int
	Pct                float64
	PctS               string
	Skills             []rdmgSkill
}

func replayDamageView(s *replay.Snapshot, c *gamedata.Catalog, l string) *rdmgView {
	if len(s.Damage) == 0 {
		return nil
	}
	v := &rdmgView{}
	if !s.RecordedAt.IsZero() {
		v.Recorded = s.RecordedAt.Format("02/01/2006 15:04")
	}
	durMs := s.Damage[0].DurationMs
	if durMs > 0 {
		sec := durMs / 1000
		v.Duration = fmt.Sprintf("%d:%02d", sec/60, sec%60)
	}
	skill := func(sd replay.SkillDamage, of int64) rdmgSkill {
		r := rdmgSkill{ID: sd.Skill, Total: fmtInt(sd.Total), Max: fmtInt(sd.Max), Hits: sd.Hits, Monsters: sd.Monsters,
			Pct: float64(sd.Total) * 100 / float64(of)}
		r.PctS = pct(r.Pct, l)
		if sd.Hits > 0 {
			r.Avg = fmtInt(sd.Total / int64(sd.Hits))
		}
		r.Name = tr(l, "Ataque básico")
		if sd.Skill != 0 {
			r.Name = fmt.Sprintf("#%d", sd.Skill)
			if sk := c.Skills[sd.Skill]; sk != nil && sk.Name.Get(l) != "" {
				r.Name = sk.Name.Get(l)
			}
		}
		return r
	}
	for _, d := range s.Damage {
		if d.Total <= 0 {
			continue
		}
		ch := rdmgChar{Name: d.Name, Self: d.Self, Total: fmtInt(d.Total), Hits: d.Hits, Kills: d.Kills}
		if ch.Name == "" {
			ch.Name = fmt.Sprintf("#%d", d.AID)
		}
		if j := c.Jobs[d.Job]; j != nil {
			ch.Job = j.Name.Get(l)
		}
		if durMs > 0 {
			ch.DPS = fmtInt(d.Total * 1000 / int64(durMs))
		}
		for _, sd := range d.Skills {
			ch.Skills = append(ch.Skills, skill(sd, d.Total))
		}
		for _, m := range d.Monsters {
			row := rdmgMob{ID: m.Mob, Name: fmt.Sprintf("#%d", m.Mob), Total: fmtInt(m.Total), Hits: m.Hits, Kills: m.Kills, Count: m.Count,
				Pct: float64(m.Total) * 100 / float64(d.Total)}
			row.PctS = pct(row.Pct, l)
			if mon := c.Monster(m.Mob); mon != nil {
				row.Name = mon.NameIn(l)
			}
			for _, sd := range m.Skill {
				row.Skills = append(row.Skills, skill(sd, m.Total))
			}
			ch.Monsters = append(ch.Monsters, row)
		}
		v.Chars = append(v.Chars, ch)
	}
	if len(v.Chars) == 0 {
		return nil
	}
	return v
}

// pct formats a share with the language's decimal mark ("86,7%").
func pct(v float64, l string) string {
	s := fmt.Sprintf("%.1f%%", v)
	if l != "en" {
		s = strings.Replace(s, ".", ",", 1)
	}
	return s
}
