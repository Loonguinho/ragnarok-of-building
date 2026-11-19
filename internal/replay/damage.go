package replay

import (
	"encoding/binary"
	"sort"
	"strings"
	"unicode/utf8"
)

// DamageSummary is the damage one character dealt to monsters during the
// recording: per monster type (with kills) and per skill. Skill 0 is the
// basic attack.
type DamageSummary struct {
	AID        uint32
	Name       string
	Job        int  // class ID (0 when unknown)
	Self       bool // the recording character
	Total      int64
	Hits       int
	Kills      int
	DurationMs int             // length of the recording, 0 when unknown
	Monsters   []MonsterDamage // most damaged first
	Skills     []SkillDamage   // most damage first
}

// MonsterDamage is the damage dealt to one monster type.
type MonsterDamage struct {
	Mob   int // monster ID
	Total int64
	Hits  int
	Kills int           // monsters of this type hit by the character that died
	Count int           // monsters of this type hit
	Skill []SkillDamage // most damage first
}

// SkillDamage is the damage of one skill (0 = basic attack).
type SkillDamage struct {
	Skill    int
	Total    int64
	Hits     int // damage packets (one per hit, meteor or swing)
	Max      int64
	Min      int64
	Monsters int // monster types it hit (summary only)
}

// Packets read (layouts: rrfparser by Adson Leal, MIT):
//
//	0x01de skill damage: skill u16, src u32, dst u32, ..., damage i32 @24
//	0x08c8 / 0x02e1 basic attack: src u32 @2, dst u32 @6, damage i32 @22
//	0x09fd/0x0915 (walking), 0x09fe, 0x09ff an object in view: type u8 @4,
//	  ID u32 @5, class u16 @23, name from @90 / @83 / @84 to the end
//	0x0857 an object in view when the recording started (container 15):
//	  type u8 @4, ID u32 @5, class u16 @19, name from @69
//	0x0080 an object leaves: ID u32 @2, type u8 @6 (1 = died)
const (
	objPC      = 0
	objMonster = 5
)

type entity struct {
	kind  byte
	class int
	name  string
}

// readEntity reads an object entering the view.
func readEntity(p []byte) (id uint32, e entity, ok bool) {
	if len(p) < 25 {
		return 0, e, false
	}
	nameAt, classAt := 0, 23
	switch binary.LittleEndian.Uint16(p) {
	case 0x09fd, 0x0915:
		nameAt = 90
	case 0x09fe:
		nameAt = 83
	case 0x09ff:
		nameAt = 84
	case 0x0857:
		nameAt, classAt = 69, 19
	default:
		return 0, e, false
	}
	e.kind = p[4]
	e.class = int(binary.LittleEndian.Uint16(p[classAt:]))
	if end := min(int(binary.LittleEndian.Uint16(p[2:])), len(p)); nameAt < end {
		e.name = entityName(p[nameAt:end])
	}
	return binary.LittleEndian.Uint32(p[5:]), e, true
}

// entityName is a zero-terminated name, skipping leading padding; UTF-8
// when valid, else Latin-1 (LATAM names are Latin).
func entityName(b []byte) string {
	for len(b) > 0 && (b[0] == 0 || b[0] == 0xff) {
		b = b[1:]
	}
	if i := strings.IndexByte(string(b), 0); i >= 0 {
		b = b[:i]
	}
	if utf8.Valid(b) {
		return strings.TrimSpace(string(b))
	}
	r := make([]rune, len(b))
	for i, c := range b {
		r[i] = rune(c)
	}
	return strings.TrimSpace(string(r))
}

// summarizeDamage sums the hits on monsters by every character (the
// recording one, aid, and the players in view). The recording character
// comes first, then the others by damage dealt.
func summarizeDamage(aid uint32, self string, initial, stream [][]byte) []*DamageSummary {
	u32 := func(p []byte, o int) uint32 { return binary.LittleEndian.Uint32(p[o:]) }
	ents := map[uint32]entity{}
	type acc struct {
		per  map[[2]int]*SkillDamage // {mob, skill}
		mobs map[int]*MonsterDamage
		hit  map[uint32]bool // monsters it hit (alive)
	}
	chars := map[uint32]*acc{}
	add := func(src, dst uint32, skill int, dmg int64) {
		mob := ents[dst]
		if mob.kind != objMonster || mob.class == 0 || dmg <= 0 {
			return
		}
		if src != aid && ents[src].kind != objPC {
			return // pets, homunculi, monsters, unknown sources
		}
		if src != aid {
			if _, ok := ents[src]; !ok {
				return
			}
		}
		a := chars[src]
		if a == nil {
			a = &acc{per: map[[2]int]*SkillDamage{}, mobs: map[int]*MonsterDamage{}, hit: map[uint32]bool{}}
			chars[src] = a
		}
		m := a.mobs[mob.class]
		if m == nil {
			m = &MonsterDamage{Mob: mob.class}
			a.mobs[mob.class] = m
		}
		if !a.hit[dst] {
			a.hit[dst] = true
			m.Count++
		}
		m.Total += dmg
		m.Hits++
		k := [2]int{mob.class, skill}
		sd := a.per[k]
		if sd == nil {
			sd = &SkillDamage{Skill: skill, Min: dmg}
			a.per[k] = sd
		}
		sd.Total += dmg
		sd.Hits++
		sd.Max = max(sd.Max, dmg)
		sd.Min = min(sd.Min, dmg)
	}
	handle := func(p []byte) {
		if len(p) < 2 {
			return
		}
		if id, e, ok := readEntity(p); ok {
			if old, seen := ents[id]; seen && e.name == "" {
				e.name = old.name
			}
			ents[id] = e
			return
		}
		switch binary.LittleEndian.Uint16(p) {
		case 0x01de:
			if len(p) >= 33 {
				add(u32(p, 4), u32(p, 8), int(binary.LittleEndian.Uint16(p[2:])), int64(int32(u32(p, 24))))
			}
		case 0x08c8:
			if len(p) >= 34 {
				add(u32(p, 2), u32(p, 6), 0, int64(int32(u32(p, 22))))
			}
		case 0x02e1:
			if len(p) >= 33 {
				add(u32(p, 2), u32(p, 6), 0, int64(int32(u32(p, 22))))
			}
		case 0x0080:
			if len(p) >= 7 && p[6] == 1 {
				gid := u32(p, 2)
				for _, a := range chars {
					if a.hit[gid] {
						a.mobs[ents[gid].class].Kills++
						delete(a.hit, gid) // a monster dies once
					}
				}
				delete(ents, gid)
			}
		}
	}
	for _, p := range initial {
		handle(p)
	}
	for _, p := range stream {
		handle(p)
	}

	var out []*DamageSummary
	for src, a := range chars {
		s := &DamageSummary{AID: src, Name: ents[src].name, Job: ents[src].class, Self: src == aid}
		if s.Self {
			s.Name = self
		}
		skills := map[int]*SkillDamage{}
		for k, sd := range a.per {
			a.mobs[k[0]].Skill = append(a.mobs[k[0]].Skill, *sd)
			t := skills[k[1]]
			if t == nil {
				t = &SkillDamage{Skill: k[1], Min: sd.Min}
				skills[k[1]] = t
			}
			t.Total += sd.Total
			t.Hits += sd.Hits
			t.Max = max(t.Max, sd.Max)
			t.Min = min(t.Min, sd.Min)
			t.Monsters++
		}
		for _, m := range a.mobs {
			byTotal(m.Skill)
			s.Monsters = append(s.Monsters, *m)
			s.Total += m.Total
			s.Hits += m.Hits
			s.Kills += m.Kills
		}
		sort.Slice(s.Monsters, func(i, j int) bool {
			if s.Monsters[i].Total != s.Monsters[j].Total {
				return s.Monsters[i].Total > s.Monsters[j].Total
			}
			return s.Monsters[i].Mob < s.Monsters[j].Mob
		})
		for _, t := range skills {
			s.Skills = append(s.Skills, *t)
		}
		byTotal(s.Skills)
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Self != out[j].Self {
			return out[i].Self
		}
		if out[i].Total != out[j].Total {
			return out[i].Total > out[j].Total
		}
		return out[i].AID < out[j].AID
	})
	return out
}

func byTotal(v []SkillDamage) {
	sort.Slice(v, func(i, j int) bool {
		if v[i].Total != v[j].Total {
			return v[i].Total > v[j].Total
		}
		return v[i].Skill < v[j].Skill
	})
}
