package replay

import (
	"encoding/binary"
	"slices"
	"testing"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

func statusPkt(efst int, aid uint32, on bool) []byte {
	p := make([]byte, 25)
	binary.LittleEndian.PutUint16(p, 0x0983)
	binary.LittleEndian.PutUint16(p[2:], uint16(efst))
	binary.LittleEndian.PutUint32(p[4:], aid)
	if on {
		p[8] = 1
	}
	return p
}

func skillPkt(src, dst uint32, dmg int32) []byte {
	p := make([]byte, 33)
	binary.LittleEndian.PutUint16(p, 0x01de)
	binary.LittleEndian.PutUint32(p[4:], src)
	binary.LittleEndian.PutUint32(p[8:], dst)
	binary.LittleEndian.PutUint32(p[24:], uint32(dmg))
	return p
}

func mobPkt(id uint32) []byte {
	p := make([]byte, 100)
	binary.LittleEndian.PutUint16(p, 0x09fe)
	binary.LittleEndian.PutUint16(p[2:], 100)
	p[4] = objMonster
	binary.LittleEndian.PutUint32(p[5:], id)
	binary.LittleEndian.PutUint16(p[23:], 1002)
	return p
}

// The statuses are the ones up at the biggest hit, not at the start or end.
func TestActiveStatuses(t *testing.T) {
	const me, mob = 7, 9
	stream := [][]byte{
		mobPkt(mob),
		statusPkt(333, me, false), // Cloaking off before the hit
		statusPkt(114, me, true),  // EDP on
		statusPkt(10, 8, true),    // someone else's Blessing
		skillPkt(me, mob, 1000),
		skillPkt(me, mob, 50000), // the biggest hit
		statusPkt(114, me, false),
		skillPkt(me, mob, 2000),
	}
	got := activeStatuses(me, []int{333, 12}, nil, stream)
	if want := []int{12, 114}; !slices.Equal(got, want) {
		t.Errorf("statuses = %v, want %v", got, want)
	}
}

func TestApplyStatuses(t *testing.T) {
	c := &gamedata.Catalog{Skills: map[int]*gamedata.Skill{5290: {ID: 5290, Code: "SHC_POTENT_VENOM"}}}
	s := &Snapshot{Skills: map[int]int{5290: 7}, Statuses: []int{10, 442, 114, 1194, 341, 99999}}
	var b calc.Build
	if n := applyStatuses(&b, s, c); n != 4 {
		t.Errorf("applied %d, want 4", n)
	}
	if b.Buffs["blessing"] != 10 || b.Buffs["rushwindmill"] != 105 {
		t.Errorf("buffs = %v", b.Buffs)
	}
	if b.Atk.States[calc.StateKeyOf("Enchant Deadly Poison")] != 1 || b.Atk.States[calc.StateKeyOf("Potent Venom")] != 7 {
		t.Errorf("states = %v", b.Atk.States)
	}
	// Aplicar Toxina keeps the weapon's element; Encantar com Veneno is a
	// poison endow.
	if b.Atk.Element != "" {
		t.Errorf("element = %q with Aplicar Toxina, want the weapon's", b.Atk.Element)
	}
	b = calc.Build{}
	applyStatuses(&b, &Snapshot{Statuses: []int{6}}, c)
	if b.Atk.Element != "Poison" {
		t.Errorf("element = %q with Encantar com Veneno, want Poison", b.Atk.Element)
	}
}
