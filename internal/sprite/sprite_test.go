package sprite

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"testing"
)

// spr21 is a version 2.1 .spr with one 2×2 paletted picture: index 1
// top-left, then a run of two transparent pixels, then index 2.
func spr21() []byte {
	var b bytes.Buffer
	b.WriteString("SP")
	b.Write([]byte{0x01, 0x02}) // 2.1
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint16(0))
	rle := []byte{1, 0, 2, 2}
	binary.Write(&b, binary.LittleEndian, []uint16{2, 2, uint16(len(rle))})
	b.Write(rle)
	pal := make([]byte, 1024)
	copy(pal[4:], []byte{255, 0, 0, 0, 0, 0, 255, 0}) // 1 red, 2 blue
	b.Write(pal)
	return b.Bytes()
}

// act25 is a version 2.5 .act with one action, one frame, one layer at
// (x, y) and one anchor.
func act25(x, y, ax, ay int32) []byte {
	var b bytes.Buffer
	w := func(v any) { binary.Write(&b, binary.LittleEndian, v) }
	b.WriteString("AC")
	b.Write([]byte{0x05, 0x02})
	w(uint16(1))
	b.Write(make([]byte, 10))
	w(uint32(1))              // frames
	b.Write(make([]byte, 32)) // ranges
	w(uint32(1))              // layers
	w([]int32{x, y, 0, 0})    // x, y, picture 0, not mirrored
	b.Write([]byte{255, 255, 255, 255})
	w(math.Float32bits(1))
	w(math.Float32bits(1))
	w([]int32{0, 0, 2, 2}) // angle, paletted, width, height
	w(int32(-1))           // sound
	w(uint32(1))           // anchors
	b.Write(make([]byte, 4))
	w([]int32{ax, ay, 0})
	return b.Bytes()
}

func TestDecodeAndRender(t *testing.T) {
	s, err := DecodeSpr(spr21())
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Pal) != 1 || s.Pal[0].Pix[0] != 1 || s.Pal[0].Pix[1] != 0 || s.Pal[0].Pix[2] != 0 || s.Pal[0].Pix[3] != 2 {
		t.Fatalf("pixels %v, want [1 0 0 2]", s.Pal[0].Pix)
	}
	a, err := DecodeAct(act25(10, -20, 3, -40))
	if err != nil {
		t.Fatal(err)
	}
	p, ok := Render(s, a, 0, 0)
	if !ok {
		t.Fatal("nothing drawn")
	}
	if p.Origin != image.Pt(-9, 21) || p.Anchor != image.Pt(3, -40) || !p.HasAnchor {
		t.Errorf("origin %v anchor %v %v", p.Origin, p.Anchor, p.HasAnchor)
	}
	if c := p.Img.NRGBAAt(0, 0); c != (color.NRGBA{255, 0, 0, 255}) {
		t.Errorf("top-left %v, want red", c)
	}
	if c := p.Img.NRGBAAt(1, 0); c.A != 0 {
		t.Errorf("index 0 drawn: %v", c)
	}
	if _, err := DecodeSpr([]byte("SP\x01\x02\x05\x00")); err == nil {
		t.Error("truncated .spr read without error")
	}
	if _, err := DecodeAct([]byte("not an act file at all")); err == nil {
		t.Error("bad .act read without error")
	}
}

func dot(c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, c)
	return img
}

func TestComposeLinesUpAnchors(t *testing.T) {
	body := Picture{Img: dot(color.NRGBA{255, 0, 0, 255}), Origin: image.Pt(0, 0), Anchor: image.Pt(0, -10), HasAnchor: true}
	head := Picture{Img: dot(color.NRGBA{0, 0, 255, 255}), Origin: image.Pt(0, 0), Anchor: image.Pt(2, -4), HasAnchor: true}
	out := Compose(body, head)
	// The head's origin goes 6 above and 2 left of the body's.
	if out.Rect.Size() != image.Pt(3, 7) {
		t.Fatalf("size %v, want 3×7", out.Rect.Size())
	}
	if out.NRGBAAt(2, 6).R != 255 || out.NRGBAAt(0, 0).B != 255 {
		t.Errorf("body at (2,6) %v, head at (0,0) %v", out.NRGBAAt(2, 6), out.NRGBAAt(0, 0))
	}
}

func TestCharacter(t *testing.T) {
	files := map[string][]byte{}
	pngOf := func(c color.NRGBA) string {
		var b bytes.Buffer
		png.Encode(&b, dot(c))
		h := string(rune('a' + len(files)))
		files[h+".png"] = b.Bytes()
		return h
	}
	at := func(f string) Entry { return Entry{File: f, HasAnchor: true} }
	idx := Index{
		Body: map[string]Entry{"20_f": at(pngOf(color.NRGBA{1, 0, 0, 255}))}, // a job with women only
		Head: map[string]Entry{"1_f": at(pngOf(color.NRGBA{2, 0, 0, 255}))},
		Acc:  map[string]Entry{"99_m": at(pngOf(color.NRGBA{3, 0, 0, 255}))}, // drawn for men only
	}
	files[IndexFile], _ = json.Marshal(idx)
	reads := 0
	s := NewServed(func(_ context.Context, name string) ([]byte, error) {
		reads++
		if b, ok := files[name]; ok {
			return b, nil
		}
		return nil, os.ErrNotExist
	})
	ctx := context.Background()
	img, err := s.Character(ctx, Look{Job: 20, Hair: 7, Heads: [3]int{99, 99, 0}})
	if err != nil {
		t.Fatal(err)
	}
	// Every part lands on the same pixel: the last drawn is the headgear.
	if got := img.NRGBAAt(0, 0).R; got != 3 {
		t.Errorf("top pixel from part %d, want the headgear (3)", got)
	}
	if n := s.Hairs(ctx, 20, true); n != 1 {
		t.Errorf("hairs %d, want 1", n)
	}
	before := reads
	s.Character(ctx, Look{Job: 20, Heads: [3]int{99}})
	if reads != before {
		t.Errorf("pictures read again (%d reads)", reads-before)
	}
	if _, err := s.Character(ctx, Look{Job: 1}); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("job without a body: err %v", err)
	}
}
