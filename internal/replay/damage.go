package replay

import (
	"encoding/binary"
	"sort"
)

// DamageSummary is the damage the recording character dealt to monsters
// during the recording: per monster type (with kills) and per skill.
// Skill 0 is the basic attack.
type DamageSummary struct {
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
//	0x09fd / 0x09fe / 0x09ff an object enters: type u8 @4, GID u32 @5, class u16 @23
//	0x0080 an object leaves: GID u32 @2, type u8 @6 (1 = died)
const objMonster = 5

func summarizeDamage(aid uint32, stream [][]byte) *DamageSummary {
	if aid == 0 {
		return nil
	}
	u32 := func(p []byte, o int) uint32 { return binary.LittleEndian.Uint32(p[o:]) }
	mobOf := map[uint32]int{} // GID -> monster ID
	hit := map[uint32]bool{}  // GIDs the character hit
	type key struct{ mob, skill int }
	per := map[key]*SkillDamage{}
	mobs := map[int]*MonsterDamage{}
	add := func(dst uint32, skill int, dmg int64) {
		mob := mobOf[dst]
		if mob == 0 || dmg <= 0 {
			return
		}
		m := mobs[mob]
		if m == nil {
			m = &MonsterDamage{Mob: mob}
			mobs[mob] = m
		}
		if !hit[dst] {
			hit[dst] = true
			m.Count++
		}
		m.Total += dmg
		m.Hits++
		sd := per[key{mob, skill}]
		if sd == nil {
			sd = &SkillDamage{Skill: skill, Min: dmg}
			per[key{mob, skill}] = sd
		}
		sd.Total += dmg
		sd.Hits++
		sd.Max = max(sd.Max, dmg)
		sd.Min = min(sd.Min, dmg)
	}
	for _, p := range stream {
		if len(p) < 2 {
			continue
		}
		switch binary.LittleEndian.Uint16(p) {
		case 0x09fd, 0x09fe, 0x09ff:
			if len(p) >= 25 && p[4] == objMonster {
				mobOf[u32(p, 5)] = int(binary.LittleEndian.Uint16(p[23:]))
			}
		case 0x01de:
			if len(p) >= 33 && u32(p, 4) == aid {
				add(u32(p, 8), int(binary.LittleEndian.Uint16(p[2:])), int64(int32(u32(p, 24))))
			}
		case 0x08c8:
			if len(p) >= 34 && u32(p, 2) == aid {
				add(u32(p, 6), 0, int64(int32(u32(p, 22))))
			}
		case 0x02e1:
			if len(p) >= 33 && u32(p, 2) == aid {
				add(u32(p, 6), 0, int64(int32(u32(p, 22))))
			}
		case 0x0080:
			if len(p) >= 7 && p[6] == 1 {
				gid := u32(p, 2)
				if hit[gid] {
					mobs[mobOf[gid]].Kills++
					delete(hit, gid) // a GID dies once
					delete(mobOf, gid)
				}
			}
		}
	}
	if len(mobs) == 0 {
		return nil
	}
	s := &DamageSummary{}
	skills := map[int]*SkillDamage{}
	for k, sd := range per {
		mobs[k.mob].Skill = append(mobs[k.mob].Skill, *sd)
		t := skills[k.skill]
		if t == nil {
			t = &SkillDamage{Skill: k.skill, Min: sd.Min}
			skills[k.skill] = t
		}
		t.Total += sd.Total
		t.Hits += sd.Hits
		t.Max = max(t.Max, sd.Max)
		t.Min = min(t.Min, sd.Min)
		t.Monsters++
	}
	byTotal := func(v []SkillDamage) {
		sort.Slice(v, func(i, j int) bool {
			if v[i].Total != v[j].Total {
				return v[i].Total > v[j].Total
			}
			return v[i].Skill < v[j].Skill
		})
	}
	for _, m := range mobs {
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
	return s
}
