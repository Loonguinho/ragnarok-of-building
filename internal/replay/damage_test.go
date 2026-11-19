package replay

import (
	"encoding/binary"
	"testing"
)

func entry(gid uint32, mob int) []byte {
	p := make([]byte, 30)
	binary.LittleEndian.PutUint16(p, 0x09fe)
	p[4] = objMonster
	binary.LittleEndian.PutUint32(p[5:], gid)
	binary.LittleEndian.PutUint16(p[23:], uint16(mob))
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

func died(gid uint32) []byte {
	p := make([]byte, 7)
	binary.LittleEndian.PutUint16(p, 0x0080)
	binary.LittleEndian.PutUint32(p[2:], gid)
	p[6] = 1
	return p
}

func TestSummarizeDamage(t *testing.T) {
	const me, other = 100, 200
	stream := [][]byte{
		entry(1, 1002), entry(2, 1002), entry(3, 1039),
		skillHit(5473, me, 1, 1000), skillHit(5473, me, 1, 3000), basicHit(me, 2, 500),
		skillHit(5469, me, 3, 9000), skillHit(5473, other, 3, 99999), // another player's hit
		skillHit(5473, me, 2, 0),  // a miss
		died(1), died(3), died(2), // 2 was hit, so it counts too
	}
	s := summarizeDamage(me, stream)
	if s == nil || s.Total != 13500 || s.Hits != 4 || s.Kills != 3 {
		t.Fatalf("summary %+v", s)
	}
	if s.Monsters[0].Mob != 1039 || s.Monsters[1].Mob != 1002 || s.Monsters[1].Kills != 2 || s.Monsters[1].Count != 2 {
		t.Errorf("monsters %+v", s.Monsters)
	}
	if s.Skills[0].Skill != 5469 || s.Skills[1].Skill != 5473 || s.Skills[1].Total != 4000 || s.Skills[1].Max != 3000 || s.Skills[2].Skill != 0 {
		t.Errorf("skills %+v", s.Skills)
	}
	if summarizeDamage(me, [][]byte{entry(1, 1002)}) != nil {
		t.Error("no hits should give no summary")
	}
}
