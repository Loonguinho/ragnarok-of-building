package replay

import "encoding/binary"

const equipAmmo = 0x8000 // rAthena EQP_AMMO

// wornAmmo picks the ammunition that was loaded among the stacks the
// snapshot flags as worn: the one the recording spends (0x07fa, 0x00af item
// removed), then the one it equips (0x013c), then a lone flagged stack.
func wornAmmo(inv []Item, stream [][]byte) int {
	byIndex := map[int]int{} // packet index -> item
	last, n := 0, 0
	for _, it := range inv {
		if it.Equipped&equipAmmo != 0 {
			byIndex[it.Index] = it.ItemID
			last = it.ItemID
			n++
		}
	}
	if n == 0 {
		return 0
	}
	u16 := func(p []byte, o int) int { return int(binary.LittleEndian.Uint16(p[o:])) }
	spent := map[int]int{}
	equipped := 0
	for _, p := range stream {
		if len(p) < 4 {
			continue
		}
		switch u16(p, 0) {
		case 0x07fa: // reason u16 @2, index u16 @4, count u16 @6
			if len(p) >= 8 {
				spent[u16(p, 4)] += u16(p, 6)
			}
		case 0x00af: // index u16 @2, count u16 @4
			if len(p) >= 6 {
				spent[u16(p, 2)] += u16(p, 4)
			}
		case 0x013c:
			if id := byIndex[u16(p, 2)]; id != 0 {
				equipped = id
			}
		}
	}
	best, most := 0, 0
	for idx, id := range byIndex {
		if c := spent[idx]; c > most {
			best, most = id, c
		}
	}
	switch {
	case best != 0:
		return best
	case equipped != 0:
		return equipped
	case n == 1:
		return last
	}
	return 0
}
