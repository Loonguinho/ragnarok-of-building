// Package replay reads the character snapshot stored at the start of a
// Ragnarok Online .rrf replay: class, levels, allocated stats, equipped and
// inventory items (refine, grade, cards) and learned skills.
//
// Format reference: rrfparser by Adson Leal (MIT),
// https://github.com/adsonpleal/rrfparser. This is an independent Go
// implementation of the snapshot part; the packet stream is only scanned for
// the skill list and 4th-job traits.
//
// Input is untrusted: every read is bounds-checked and malformed files return
// an error instead of panicking.
package replay

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

const (
	magic           = "<< Ragnarok Replay File Version"
	headerPrefix    = 100
	containerCount  = 24
	descriptorBytes = 10
	maxFileSize     = 64 << 20
)

// Container types.
const (
	ctPacketStream   = 1
	ctReplayData     = 2
	ctSession        = 3
	ctItems          = 8
	ctInitialPackets = 14
)

// Item is one item record from the snapshot.
type Item struct {
	ItemID   int
	Qty      int
	Refine   int
	Grade    int
	Cards    [4]int
	Opts     [][2]int // random options: [ID, value]
	Equipped uint32   // rAthena e_equip_pos bitmask; 0 = not worn
}

// Snapshot is the character as it was when recording started.
type Snapshot struct {
	RecordedAt time.Time
	Player     string
	Job        int
	BaseLv     int
	JobLv      int
	Stats      [6]int      // allocated STR AGI VIT INT DEX LUK (no gear)
	Traits     map[int]int // allocated POW..CRT (0-5); only those the recording carried
	Equipped   []Item
	Inventory  []Item
	Skills     map[int]int // learned skill ID -> level
}

type chunk struct {
	id   int
	data []byte
}

// Decode reads a .rrf file.
func Decode(buf []byte) (s *Snapshot, err error) {
	defer func() { // belt and braces: malformed input must never crash the server
		if r := recover(); r != nil {
			s, err = nil, fmt.Errorf("replay: malformed file: %v", r)
		}
	}()
	if len(buf) < 112 || len(buf) > maxFileSize {
		return nil, errors.New("replay: file too small or too large")
	}
	if !bytes.HasPrefix(buf, []byte(magic)) {
		return nil, errors.New("replay: not a Ragnarok replay file")
	}
	if v := buf[headerPrefix]; v != 5 {
		return nil, fmt.Errorf("replay: unsupported version %d", v)
	}
	o := headerPrefix + 4
	year := int(int16(binary.LittleEndian.Uint16(buf[o:])))
	month, day := buf[o+2], buf[o+3]
	hour, minute, second := buf[o+5], buf[o+6], buf[o+7]
	tableOffset := o + 8
	k1, k2 := keys(year, month, day, hour, minute, second)

	s = &Snapshot{
		RecordedAt: time.Date(year, time.Month(month), int(day), int(hour), int(minute), int(second), 0, time.UTC),
		Skills:     map[int]int{},
		Traits:     map[int]int{},
	}

	if tableOffset+containerCount*descriptorBytes > len(buf) {
		return nil, errors.New("replay: truncated container table")
	}
	var session, items, initial, replayData []chunk
	var stream [][]byte
	for i := 0; i < containerCount; i++ {
		d := buf[tableOffset+i*descriptorBytes:]
		typ := binary.LittleEndian.Uint16(d)
		declared := int(int32(binary.LittleEndian.Uint32(d[2:])))
		offset := int(int32(binary.LittleEndian.Uint32(d[6:])))
		if offset == 0 && declared == 0 {
			continue
		}
		real := declared
		if real == 0 && offset > 0 {
			real = len(buf) - offset
		}
		if offset < 0 || real < 0 || offset >= len(buf) || offset+real > len(buf) {
			return nil, fmt.Errorf("replay: container %d out of bounds", i)
		}
		body := buf[offset : offset+real]
		switch typ {
		case ctPacketStream:
			stream = append(stream, packetStream(body, k1, k2)...)
		case ctSession:
			session = generic(body, declared, k1, k2)
		case ctItems:
			items = generic(body, declared, k1, k2)
		case ctInitialPackets:
			initial = generic(body, declared, k1, k2)
		case ctReplayData:
			replayData = generic(body, declared, k1, k2)
		}
	}

	if len(replayData) > 4 {
		s.Player = cstring(replayData[4].data)
	}
	u32 := func(cs []chunk, id int) int {
		for _, c := range cs {
			if c.id == id && len(c.data) >= 4 {
				return int(binary.LittleEndian.Uint32(c.data))
			}
		}
		return 0
	}
	s.Job, s.BaseLv, s.JobLv = u32(session, 1014), u32(session, 1016), u32(session, 1019)
	for i := range s.Stats {
		s.Stats[i] = u32(session, 1024+i)
	}
	if s.Job == 0 && s.BaseLv == 0 {
		return nil, errors.New("replay: no character snapshot in this file")
	}

	for _, c := range items {
		switch c.id {
		case 4601:
			s.Equipped = append(s.Equipped, readItems(c.data)...)
		case 4510:
			s.Inventory = append(s.Inventory, readItems(c.data)...)
		}
	}
	// Some recordings keep worn gear only as flagged inventory records.
	if len(s.Equipped) == 0 {
		for _, it := range s.Inventory {
			if it.Equipped != 0 {
				s.Equipped = append(s.Equipped, it)
			}
		}
	}

	// Skills (0x010f) from the initial packets, then the stream; max level wins.
	// Traits (0x0141, statuses 219-224): last value in the stream wins.
	handle := func(p []byte) {
		if len(p) < 4 {
			return
		}
		switch binary.LittleEndian.Uint16(p) {
		case 0x010f:
			end := min(int(binary.LittleEndian.Uint16(p[2:])), len(p))
			for q := 4; q+37 <= end; q += 37 {
				id, lv := int(binary.LittleEndian.Uint16(p[q:])), int(binary.LittleEndian.Uint16(p[q+6:]))
				if id > 0 && lv > 0 && lv > s.Skills[id] {
					s.Skills[id] = lv
				}
			}
		case 0x0141:
			if len(p) >= 14 {
				st := int(int32(binary.LittleEndian.Uint32(p[2:])))
				if st >= 219 && st <= 224 {
					s.Traits[st-219] = int(int32(binary.LittleEndian.Uint32(p[6:])))
				}
			}
		}
	}
	for _, c := range initial {
		handle(c.data)
	}
	for _, p := range stream {
		handle(p)
	}
	return s, nil
}

// keys derives the XOR keystream parameters from the recording timestamp.
func keys(year int, month, day, hour, minute, second byte) (int32, int32) {
	var b [4]byte
	binary.LittleEndian.PutUint16(b[:], uint16(int16(year)))
	b[2], b[3] = month, day
	k1 := int32(binary.LittleEndian.Uint32(b[:])) >> 5
	b = [4]byte{0, hour, minute, second}
	k2 := int32(binary.LittleEndian.Uint32(b[:])) >> 3
	return k1, k2
}

// decrypt XORs whole 32-bit words of data[:size] with (k1 + i + 1) * k2.
func decrypt(data []byte, size int, k1, k2 int32) []byte {
	out := append([]byte(nil), data...)
	words := min(size, len(out)) / 4
	for i := 0; i < words; i++ {
		w := int32(binary.LittleEndian.Uint32(out[i*4:]))
		w ^= (k1 + int32(i) + 1) * k2
		binary.LittleEndian.PutUint32(out[i*4:], uint32(w))
	}
	return out
}

func generic(body []byte, declared int, k1, k2 int32) []chunk {
	if declared <= 0 {
		return nil
	}
	d := decrypt(body, declared, k1, k2)
	var out []chunk
	for p := 0; p+6 <= declared && p+6 <= len(d); {
		id := int(int16(binary.LittleEndian.Uint16(d[p:])))
		n := int(int32(binary.LittleEndian.Uint32(d[p+2:])))
		if n < 0 || p+6+n > len(d) {
			break
		}
		out = append(out, chunk{id: id, data: d[p+6 : p+6+n]})
		p += 6 + n
	}
	return out
}

func packetStream(body []byte, k1, k2 int32) [][]byte {
	var out [][]byte
	for p := 0; p+10 <= len(body); {
		n := int(binary.LittleEndian.Uint16(body[p+8:]))
		if p+10+n > len(body) {
			break
		}
		out = append(out, decrypt(body[p+10:p+10+n], n, k1, k2))
		p += 10 + n
	}
	return out
}

func cstring(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return string(b)
}

// Item records are 221 or 172 bytes, either TLV-tagged or (older) fixed layout.
var recordSizes = []int{221, 172}

const (
	nameIDOffset = 104
	tagPos       = 285
	tagWear      = 286
	tagQty       = 287
	tagCards     = 290
	tagNameID    = 291
	tagRefine    = 295
	tagGrade     = 299
	tagOptions   = 301 // 5 x {u16 id, i16 value, u8 param}
)

func readItems(data []byte) []Item {
	size := 0
	for _, sz := range recordSizes {
		if len(data) < sz || len(data)%sz != 0 {
			continue
		}
		ok, any := true, false
		for r := 0; r+sz <= len(data); r += sz {
			id := int32(binary.LittleEndian.Uint32(data[r+nameIDOffset:]))
			if id == 0 {
				continue
			}
			if id < 0 || id >= 5_000_000 {
				ok = false
				break
			}
			any = true
		}
		if ok && any {
			size = sz
			break
		}
	}
	if size == 0 {
		return nil
	}
	bySlot := map[int]bool{}
	var out []Item
	for r := 0; r+size <= len(data); r += size {
		pos, it := readRecord(data[r : r+size])
		slot := pos - 2
		if it.ItemID <= 0 || it.Qty <= 0 || slot < 0 || bySlot[slot] {
			continue
		}
		bySlot[slot] = true
		out = append(out, it)
	}
	return out
}

func readRecord(rec []byte) (pos int, it Item) {
	type field struct{ off, n int }
	fields := map[int]field{}
	ok := true
	o := 0
	for o+6 <= len(rec) {
		tag := int(binary.LittleEndian.Uint16(rec[o:]))
		n := int(binary.LittleEndian.Uint32(rec[o+2:]))
		if n < 0 || o+6+n > len(rec) {
			ok = false
			break
		}
		fields[tag] = field{o + 6, n}
		o += 6 + n
	}
	if !ok || o != len(rec) {
		// Older fixed layout.
		pos = int(int16(binary.LittleEndian.Uint16(rec[22:])))
		it.ItemID = int(int32(binary.LittleEndian.Uint32(rec[nameIDOffset:])))
		it.Qty = int(int16(binary.LittleEndian.Uint16(rec[52:])))
		it.Equipped = binary.LittleEndian.Uint32(rec[42:])
		it.Refine = int(rec[134])
		for i, off := range []int{82, 86, 90, 94} {
			it.Cards[i] = int(int32(binary.LittleEndian.Uint32(rec[off:])))
		}
		return pos, it
	}
	num := func(tag int) int {
		f, ok := fields[tag]
		if !ok {
			return 0
		}
		switch f.n {
		case 1:
			return int(rec[f.off])
		case 2:
			return int(binary.LittleEndian.Uint16(rec[f.off:]))
		case 4:
			return int(int32(binary.LittleEndian.Uint32(rec[f.off:])))
		}
		return 0
	}
	pos = num(tagPos)
	it.ItemID, it.Qty = num(tagNameID), num(tagQty)
	it.Equipped = uint32(num(tagWear))
	it.Refine, it.Grade = num(tagRefine), num(tagGrade)
	if f, ok := fields[tagOptions]; ok && f.n == 25 {
		for i := 0; i < 5; i++ {
			o := f.off + i*5
			if id := int(binary.LittleEndian.Uint16(rec[o:])); id != 0 {
				it.Opts = append(it.Opts, [2]int{id, int(int16(binary.LittleEndian.Uint16(rec[o+2:])))})
			}
		}
	}
	if f, ok := fields[tagCards]; ok && f.n == 16 {
		for i := range it.Cards {
			it.Cards[i] = int(int32(binary.LittleEndian.Uint32(rec[f.off+i*4:])))
		}
	}
	return pos, it
}
