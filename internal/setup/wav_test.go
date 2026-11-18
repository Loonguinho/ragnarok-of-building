package setup

import (
	"encoding/binary"
	"testing"
)

func TestPadWAV(t *testing.T) {
	// 100 ms of a constant 1000 at 1000 Hz mono 16-bit.
	data := make([]byte, 200)
	for i := 0; i < 100; i++ {
		binary.LittleEndian.PutUint16(data[i*2:], 1000)
	}
	b := make([]byte, 44)
	copy(b, "RIFF")
	binary.LittleEndian.PutUint32(b[4:], uint32(36+len(data)))
	copy(b[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(b[16:], 16)
	binary.LittleEndian.PutUint16(b[20:], 1)
	binary.LittleEndian.PutUint16(b[22:], 1)
	binary.LittleEndian.PutUint32(b[24:], 1000)
	binary.LittleEndian.PutUint32(b[28:], 2000)
	binary.LittleEndian.PutUint16(b[32:], 2)
	binary.LittleEndian.PutUint16(b[34:], 16)
	copy(b[36:], "data")
	binary.LittleEndian.PutUint32(b[40:], uint32(len(data)))
	b = append(b, data...)

	out := padWAV(b)
	n := int(binary.LittleEndian.Uint32(out[40:]))
	if n != 200+600 { // + 300 ms of silence
		t.Fatalf("data size %d, want 800", n)
	}
	sample := func(i int) int16 { return int16(binary.LittleEndian.Uint16(out[44+i*2:])) }
	if sample(0) != 1000 || sample(79) != 1000 {
		t.Error("the sound itself changed")
	}
	if sample(99) >= 100 || sample(90) >= 1000 {
		t.Errorf("no fade at the end: %d %d", sample(90), sample(99))
	}
	if sample(150) != 0 || sample(399) != 0 {
		t.Error("padding is not silence")
	}
	if got := padWAV([]byte("not a wav")); string(got) != "not a wav" {
		t.Error("non-WAV input changed")
	}
}
