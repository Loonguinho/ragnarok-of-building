// Package sprite reads the game client's character sprites (.spr pictures
// and .act animations) and draws a frame of them, so the equipment window
// can show the character with its headgears and costumes.
package sprite

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
)

// Spr is a sprite sheet: paletted pictures (index 0 is transparent) and,
// from version 2.0, true-colour ones.
type Spr struct {
	Pal     []*image.Paletted
	RGBA    []*image.NRGBA
	Palette color.Palette
}

// DecodeSpr reads a .spr file.
func DecodeSpr(b []byte) (*Spr, error) {
	if len(b) < 6 || b[0] != 'S' || b[1] != 'P' {
		return nil, errors.New("sprite: not a .spr file")
	}
	ver := int(b[3])<<8 | int(b[2]) // 0x201 = 2.1
	r := reader{b: b, at: 4}
	nPal := int(r.u16())
	nRGBA := 0
	if ver >= 0x200 {
		nRGBA = int(r.u16())
	}
	s := &Spr{}
	pal := make(color.Palette, 256)
	if ver >= 0x101 && len(b) >= 1024 {
		p := b[len(b)-1024:]
		for i := range 256 {
			a := uint8(0xFF)
			if i == 0 {
				a = 0
			}
			pal[i] = color.NRGBA{p[i*4], p[i*4+1], p[i*4+2], a}
		}
	}
	s.Palette = pal
	for range nPal {
		w, h := int(r.u16()), int(r.u16())
		px := make([]uint8, w*h)
		if ver >= 0x201 { // run-length: 0 is followed by how many zeros
			n := int(r.u16())
			data := r.bytes(n)
			at := 0
			for i := 0; i < len(data) && at < len(px); i++ {
				if data[i] == 0 {
					run := 1
					if i+1 < len(data) {
						i++
						run = int(data[i])
					}
					at += max(run, 1)
					continue
				}
				px[at] = data[i]
				at++
			}
		} else {
			copy(px, r.bytes(w*h))
		}
		s.Pal = append(s.Pal, &image.Paletted{Pix: px, Stride: w, Rect: image.Rect(0, 0, w, h), Palette: pal})
	}
	for range nRGBA {
		w, h := int(r.u16()), int(r.u16())
		raw := r.bytes(w * h * 4)
		img := image.NewNRGBA(image.Rect(0, 0, w, h))
		for y := range h { // stored bottom-up, as ABGR
			for x := range w {
				o := ((h-1-y)*w + x) * 4
				if o+3 >= len(raw) {
					continue
				}
				img.SetNRGBA(x, y, color.NRGBA{raw[o+3], raw[o+2], raw[o+1], raw[o]})
			}
		}
		s.RGBA = append(s.RGBA, img)
	}
	if r.err != nil {
		return nil, fmt.Errorf("sprite: %w", r.err)
	}
	return s, nil
}

// reader reads little-endian values, remembering the first overrun.
type reader struct {
	b   []byte
	at  int
	err error
}

func (r *reader) bytes(n int) []byte {
	if r.err != nil || n < 0 || r.at+n > len(r.b) {
		if r.err == nil {
			r.err = errors.New("truncated")
		}
		return make([]byte, max(n, 0))
	}
	out := r.b[r.at : r.at+n]
	r.at += n
	return out
}

func (r *reader) u16() uint16 { return binary.LittleEndian.Uint16(r.bytes(2)) }
func (r *reader) u32() uint32 { return binary.LittleEndian.Uint32(r.bytes(4)) }
func (r *reader) i32() int32  { return int32(r.u32()) }
func (r *reader) f32() float32 {
	u := r.u32()
	return float32frombits(u)
}
