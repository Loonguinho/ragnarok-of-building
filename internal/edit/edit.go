// Package edit holds the build editing rules the interfaces share: what
// happens when a slot is emptied, a refine or card is set on a headgear
// that covers several slots, a buff of an exclusive group is turned on, or
// a skill is raised in the tree. The web pages had them in JavaScript
// (build.js, skilltree.js); the native app uses these.
package edit

import (
	"maps"
	"slices"

	"ragnarok-of-building/internal/calc"
)

var (
	headSlots    = []string{calc.HeadTop, calc.HeadMid, calc.HeadLow}
	costumeHeads = []string{calc.CosTop, calc.CosMid, calc.CosLow}
)

// HeadGroup is the group of head slots one item can cover together (nil
// for other slots).
func HeadGroup(slot string) []string {
	switch {
	case slices.Contains(headSlots, slot):
		return headSlots
	case slices.Contains(costumeHeads, slot):
		return costumeHeads
	}
	return nil
}

// Linked is the slot plus every slot of its head group holding the same item.
func Linked(b calc.Build, slot string) []string {
	cur, ok := b.Equip[slot]
	if !ok {
		return nil
	}
	out := []string{slot}
	for _, h := range HeadGroup(slot) {
		if h != slot && b.Equip[h].Item == cur.Item {
			if _, ok := b.Equip[h]; ok {
				out = append(out, h)
			}
		}
	}
	return out
}

// clone copies the maps an edit writes to.
func clone(b calc.Build) calc.Build {
	b.Equip = maps.Clone(b.Equip)
	b.Skills = maps.Clone(b.Skills)
	b.Buffs = maps.Clone(b.Buffs)
	b.Atk.States = maps.Clone(b.Atk.States)
	return b
}

// Unequip empties a slot and every linked head slot holding the same item.
func Unequip(b calc.Build, slot string) calc.Build {
	b = clone(b)
	for _, k := range Linked(b, slot) {
		delete(b.Equip, k)
	}
	return b
}

// EditLinked applies fn to the slot's equipment and its linked head slots
// (refine, grade, cards and random options stay in sync).
func EditLinked(b calc.Build, slot string, fn func(*calc.Equip)) calc.Build {
	b = clone(b)
	for _, k := range Linked(b, slot) {
		e := b.Equip[k]
		fn(&e)
		b.Equip[k] = e
	}
	return b
}

// SetBuff sets a buff (0 removes it). Turning one on removes the others of
// its exclusive group (one bard song, one ASPD potion...): group lists the
// keys of that group (nil: not exclusive).
func SetBuff(b calc.Build, key string, v int, group []string) calc.Build {
	b = clone(b)
	if b.Buffs == nil {
		b.Buffs = map[string]int{}
	}
	if v > 0 {
		b.Buffs[key] = v
		for _, k := range group {
			if k != key {
				delete(b.Buffs, k)
			}
		}
	} else {
		delete(b.Buffs, key)
	}
	return b
}

// SetState sets a class state of the attack (formula states, class buffs);
// 0 removes it. group, as in SetBuff, lists the exclusive group's keys.
func SetState(b calc.Build, key string, v int, group []string) calc.Build {
	b = clone(b)
	if b.Atk.States == nil {
		b.Atk.States = map[string]int{}
	}
	if v != 0 {
		b.Atk.States[key] = v
		for _, k := range group {
			if k != key {
				delete(b.Atk.States, k)
			}
		}
	} else {
		delete(b.Atk.States, key)
	}
	return b
}
