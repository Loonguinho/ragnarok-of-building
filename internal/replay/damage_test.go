package replay

import (
	"encoding/binary"
	"testing"
)

// entry is an object coming into view (0x09fe) with a trailing name.
func entry(id uint32, kind byte, class int, name string) []byte {
	p := make([]byte, 83+len(name)+1)
	binary.LittleEndian.PutUint16(p, 0x09fe)
	binary.LittleEndian.PutUint16(p[2:], uint16(len(p)))
	p[4] = kind
	binary.LittleEndian.PutUint32(p[5:], id)
	binary.LittleEndian.PutUint16(p[23:], uint16(class))
	copy(p[83:], name)
	return p
}

// startObject is an object already in view when the recording started (0x0857).
func startObject(id uint32, kind byte, class int) []byte {
	p := make([]byte, 70)
	binary.LittleEndian.PutUint16(p, 0x0857)
	binary.LittleEndian.PutUint16(p[2:], uint16(len(p)))
	p[4] = kind
	binary.LittleEndian.PutUint32(p[5:], id)
	binary.LittleEndian.PutUint16(p[19:], uint16(class))
	return p
}

func skillHit(skill int, src, dst uint32, dmg int32) []byte {
	p := make([]byte, 33)
	binary.LittleEndian.PutUint16(p, 0x01de)
	binary.LittleEndian.PutUint16(p[2:], uint16(skill))
	binary.LittleEndian.PutUint32(p[4:], src)
	binary.LittleEndian.PutUint32(p[8:], dst)
	binary.LittleEndian.PutUint32(p[24:], uint32(dmg))
	return p
}

func basicHit(src, dst uint32, dmg int32) []byte {
	p := make([]byte, 34)
	binary.LittleEndian.PutUint16(p, 0x08c8)
	binary.LittleEndian.PutUint32(p[2:], src)
	binary.LittleEndian.PutUint32(p[6:], dst)
	binary.LittleEndian.PutUint32(p[22:], uint32(dmg))
	return p
}

func died(id uint32) []byte {
	p := make([]byte, 7)
	binary.LittleEndian.PutUint16(p, 0x0080)
	binary.LittleEndian.PutUint32(p[2:], id)
	p[6] = 1
	return p
}

func TestSummarizeDamage(t *testing.T) {
	const me, other, pet = 100, 200, 300
	initial := [][]byte{startObject(9, objMonster, 21077)} // a dummy in view from the start
	stream := [][]byte{
		entry(1, objMonster, 1002, ""), entry(2, objMonster, 1002, ""), entry(3, objMonster, 1039, ""),
		entry(other, objPC, 4302, "Thzinn"), entry(pet, 7, 1002, "Poring"),
		skillHit(5473, me, 1, 1000), skillHit(5473, me, 1, 3000), basicHit(me, 2, 500),
		skillHit(5469, me, 3, 9000), skillHit(5473, other, 3, 99999),
		basicHit(pet, 3, 77),      // a pet: not a character
		skillHit(5473, me, 2, 0),  // a miss
		skillHit(5470, me, 9, 40), // on the dummy
		died(1), died(3), died(2), // 2 was hit, so it counts too
	}
	all := summarizeDamage(me, "Eu", initial, stream)
	if len(all) != 2 || !all[0].Self || all[0].Name != "Eu" || all[1].Name != "Thzinn" || all[1].Job != 4302 {
		t.Fatalf("characters: %+v", all)
	}
	s := all[0]
	if s.Total != 13540 || s.Hits != 5 || s.Kills != 3 {
		t.Fatalf("me: total %d hits %d kills %d", s.Total, s.Hits, s.Kills)
	}
	if s.Monsters[0].Mob != 1039 || s.Monsters[1].Mob != 1002 || s.Monsters[1].Kills != 2 || s.Monsters[1].Count != 2 || s.Monsters[2].Mob != 21077 {
		t.Errorf("monsters %+v", s.Monsters)
	}
	if s.Skills[0].Skill != 5469 || s.Skills[1].Skill != 5473 || s.Skills[1].Total != 4000 || s.Skills[1].Max != 3000 {
		t.Errorf("skills %+v", s.Skills)
	}
	if o := all[1]; o.Total != 99999 || o.Kills != 1 {
		t.Errorf("other player: %+v", o)
	}
	if summarizeDamage(me, "", nil, [][]byte{entry(1, objMonster, 1002, "")}) != nil {
		t.Error("no hits should give no summary")
	}
}

func TestEntityName(t *testing.T) {
	if got := entityName([]byte("\x00Jo\xe3o\x00junk")); got != "João" {
		t.Errorf("latin-1 name = %q", got)
	}
	if got := entityName([]byte("Ação")); got != "Ação" {
		t.Errorf("utf-8 name = %q", got)
	}
}
