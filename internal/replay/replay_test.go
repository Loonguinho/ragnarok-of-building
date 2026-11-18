package replay

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"ragnarok-of-building/internal/config"
	"testing"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

func le16(v int) []byte { b := make([]byte, 2); binary.LittleEndian.PutUint16(b, uint16(v)); return b }
func le32(v int) []byte { b := make([]byte, 4); binary.LittleEndian.PutUint32(b, uint32(v)); return b }

// genericBody encodes chunks as [i16 id][i32 len][data].
func genericBody(ids []int, data map[int][]byte) []byte {
	var out []byte
	for _, id := range ids {
		out = append(out, le16(id)...)
		out = append(out, le32(len(data[id]))...)
		out = append(out, data[id]...)
	}
	return out
}

// buildRRF writes a minimal encrypted .rrf (Session, Items, InitialPackets),
// mirroring the layout Decode reads, so CI tests need no real recordings.
func buildRRF(t *testing.T) []byte {
	t.Helper()
	const year, month, day, hour, minute, second = 2026, 11, 8, 14, 38, 5
	k1, k2 := keys(year, month, day, hour, minute, second)

	header := make([]byte, 112)
	copy(header, magic+" 0.05 >>")
	header[100] = 5
	copy(header[101:], "RRF")
	binary.LittleEndian.PutUint16(header[104:], year)
	header[106], header[107] = month, day
	header[109], header[110], header[111] = hour, minute, second

	session := genericBody([]int{1014, 1016, 1019, 1024, 1025, 1026, 1027, 1028, 1029}, map[int][]byte{
		1014: le32(4252), 1016: le32(250), 1019: le32(50),
		1024: le32(120), 1025: le32(90), 1026: le32(100), 1027: le32(1), 1028: le32(80), 1029: le32(40),
	})

	// One 221-byte TLV item record: +9 weapon with two cards, worn in the right hand.
	var rec []byte
	tlv := func(tag int, v []byte) {
		rec = append(rec, le16(tag)...)
		rec = append(rec, le32(len(v))...)
		rec = append(rec, v...)
	}
	cards := append(append(append(le32(4001), le32(4002)...), le32(0)...), le32(0)...)
	tlv(tagPos, le16(2))
	tlv(tagWear, le32(0x2))
	tlv(tagQty, le16(1))
	tlv(tagCards, cards)
	tlv(tagNameID, le32(1201))
	tlv(tagRefine, []byte{9})
	tlv(tagGrade, []byte{2})
	// Random options: ATQ +50 and FOR -2 (5 entries of id u16, value i16, param u8).
	opts := make([]byte, 25)
	binary.LittleEndian.PutUint16(opts[0:], 17)
	binary.LittleEndian.PutUint16(opts[2:], 50)
	binary.LittleEndian.PutUint16(opts[5:], 3)
	binary.LittleEndian.PutUint16(opts[7:], uint16(0xFFFE))
	tlv(tagOptions, opts)
	tlv(999, make([]byte, 221-len(rec)-6)) // pad to the record size
	// The record-size probe reads a plausible item id at offset 104 (inside the padding).
	binary.LittleEndian.PutUint32(rec[nameIDOffset:], 1201)
	items := genericBody([]int{4601}, map[int][]byte{4601: rec})

	// 0x010f with two 37-byte skill records: Bash 10, Bowling Bash 7.
	sk := make([]byte, 4+2*37)
	binary.LittleEndian.PutUint16(sk, 0x010f)
	binary.LittleEndian.PutUint16(sk[2:], uint16(len(sk)))
	binary.LittleEndian.PutUint16(sk[4:], 5)
	binary.LittleEndian.PutUint16(sk[4+6:], 10)
	binary.LittleEndian.PutUint16(sk[4+37:], 62)
	binary.LittleEndian.PutUint16(sk[4+37+6:], 7)
	initial := genericBody([]int{1}, map[int][]byte{1: sk})

	conts := []struct {
		typ  int
		body []byte
	}{{ctSession, session}, {ctItems, items}, {ctInitialPackets, initial}}
	file := append(header, make([]byte, containerCount*descriptorBytes)...)
	for i, ct := range conts {
		enc := decrypt(ct.body, len(ct.body), k1, k2) // XOR keystream: encrypt == decrypt
		d := file[112+i*descriptorBytes:]
		binary.LittleEndian.PutUint16(d, uint16(ct.typ))
		binary.LittleEndian.PutUint32(d[2:], uint32(len(enc)))
		binary.LittleEndian.PutUint32(d[6:], uint32(len(file)))
		file = append(file, enc...)
	}
	return file
}

func TestDecodeSynthetic(t *testing.T) {
	s, err := Decode(buildRRF(t))
	if err != nil {
		t.Fatal(err)
	}
	if s.Job != 4252 || s.BaseLv != 250 || s.JobLv != 50 || s.Stats != [6]int{120, 90, 100, 1, 80, 40} {
		t.Errorf("session = %+v", s)
	}
	if len(s.Equipped) != 1 || s.Equipped[0].ItemID != 1201 || s.Equipped[0].Refine != 9 ||
		s.Equipped[0].Cards != [4]int{4001, 4002, 0, 0} || s.Equipped[0].Equipped != 0x2 || s.Equipped[0].Grade != 2 ||
		len(s.Equipped[0].Opts) != 2 || s.Equipped[0].Opts[0] != [2]int{17, 50} || s.Equipped[0].Opts[1] != [2]int{3, -2} {
		t.Errorf("equipped = %+v", s.Equipped)
	}
	if s.Skills[5] != 10 || s.Skills[62] != 7 {
		t.Errorf("skills = %v", s.Skills)
	}

	c := &gamedata.Catalog{Jobs: map[int]*gamedata.Job{4252: {ID: 4252}}, Items: map[int]*gamedata.Item{1201: {ID: 1201}}}
	b, _ := ToBuild(s, c)
	if e := b.Equip[calc.Weapon]; e.Item != 1201 || e.Refine != 9 || e.Cards[1] != 4002 || e.Grade != 2 || len(e.Opts) != 2 {
		t.Errorf("build weapon = %+v", b.Equip)
	}
}

func TestDecodeRejectsGarbage(t *testing.T) {
	good := buildRRF(t)
	for _, bad := range [][]byte{nil, []byte("hello"), good[:111], good[:200]} {
		if _, err := Decode(bad); err == nil {
			t.Errorf("accepted %d bytes", len(bad))
		}
	}
	for i := 112; i < len(good); i += 7 { // corrupt bytes anywhere: must not panic
		b := append([]byte(nil), good...)
		b[i] ^= 0xff
		Decode(b)
	}
}

func FuzzDecode(f *testing.F) {
	f.Add([]byte(magic))
	f.Fuzz(func(t *testing.T, b []byte) { Decode(b) })
}

// Real recordings when present: the user's client folder and the MIT
// fixtures of latam-ro-calc (cloned under tmp/, not committed).
func TestRealReplays(t *testing.T) {
	var files []string
	for _, pat := range []string{filepath.Join(config.ClientDir(), "Replay", "*.rrf"), "../../tmp/latam-ro-calc/src/app/replay/__tests__/fixtures/*.rrf"} {
		m, _ := filepath.Glob(pat)
		files = append(files, m...)
	}
	if len(files) == 0 {
		t.Skip("no replays available")
	}
	c, _ := gamedata.Load("../../data/gen/catalog.json.gz")
	for _, f := range files {
		b, _ := os.ReadFile(f)
		s, err := Decode(b)
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(f), err)
			continue
		}
		if s.BaseLv < 1 || s.BaseLv > 275 || s.JobLv < 1 || s.JobLv > 70 {
			t.Errorf("%s: implausible levels %d/%d", filepath.Base(f), s.BaseLv, s.JobLv)
		}
		if c != nil && c.Jobs[s.Job] == nil {
			t.Errorf("%s: unknown job %d", filepath.Base(f), s.Job)
		}
	}
}

// testdata/synthetic.rrf is buildRRF's output, committed so other packages'
// tests (the upload endpoint) can use a real file. Regenerate with
// UPDATE_SYNTHETIC=1 go test ./internal/replay.
func TestSyntheticFixtureInSync(t *testing.T) {
	want := buildRRF(t)
	if os.Getenv("UPDATE_SYNTHETIC") == "1" {
		if err := os.WriteFile("testdata/synthetic.rrf", want, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile("testdata/synthetic.rrf")
	if err != nil || string(got) != string(want) {
		t.Fatal("testdata/synthetic.rrf is stale: run UPDATE_SYNTHETIC=1 go test ./internal/replay")
	}
}
