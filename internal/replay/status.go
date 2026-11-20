package replay

import (
	"encoding/binary"
	"sort"
)

// activeStatuses follows the character's statuses through the recording:
// the ones up at the start (EfstList), then 0x0983/0x043f/0x0196 status
// changes (index u16 @2, ID u32 @4, on u8 @8). It returns the set as it was
// at the character's biggest hit on a monster, the moment a damage check
// compares against, or at the end when it hit nothing.
func activeStatuses(aid uint32, start []int, initial, stream [][]byte) []int {
	on := map[int]bool{}
	for _, id := range start {
		on[id] = true
	}
	mobs := map[uint32]bool{}
	var best int64 = -1
	var at map[int]bool
	snap := func() map[int]bool {
		m := make(map[int]bool, len(on))
		for k, v := range on {
			if v {
				m[k] = true
			}
		}
		return m
	}
	handle := func(p []byte) {
		if len(p) < 2 {
			return
		}
		if id, e, ok := readEntity(p); ok {
			mobs[id] = e.kind == objMonster
			return
		}
		u32 := func(o int) uint32 { return binary.LittleEndian.Uint32(p[o:]) }
		switch binary.LittleEndian.Uint16(p) {
		case 0x0983, 0x043f, 0x0196:
			if len(p) >= 9 && u32(4) == aid {
				on[int(binary.LittleEndian.Uint16(p[2:]))] = p[8] != 0
			}
		case 0x01de, 0x08c8, 0x02e1:
			src, dst, at0 := 0, 0, 0
			switch binary.LittleEndian.Uint16(p) {
			case 0x01de:
				if len(p) < 33 {
					return
				}
				src, dst, at0 = 4, 8, 24
			default:
				if len(p) < 33 {
					return
				}
				src, dst, at0 = 2, 6, 22
			}
			if u32(src) != aid || !mobs[u32(dst)] {
				return
			}
			if d := int64(int32(u32(at0))); d > best {
				best, at = d, snap()
			}
		}
	}
	for _, p := range initial {
		handle(p)
	}
	for _, p := range stream {
		handle(p)
	}
	if at == nil {
		at = snap()
	}
	out := make([]int, 0, len(at))
	for id := range at {
		out = append(out, id)
	}
	sort.Ints(out)
	return out
}
