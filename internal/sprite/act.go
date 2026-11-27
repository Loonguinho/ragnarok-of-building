package sprite

import (
	"errors"
	"fmt"
	"math"
)

func float32frombits(u uint32) float32 { return math.Float32frombits(u) }

// Act is a sprite's animations: actions (8 directions each for
// characters), frames, and the layers each frame draws.
type Act struct {
	Actions [][]Frame
}

// Frame is one picture of an action.
type Frame struct {
	Layers []Layer
	// Anchors line a head or headgear up with the body: the body's anchor
	// and the head's anchor go on the same point.
	Anchors [][2]int
}

// Layer is one sprite picture placed in a frame.
type Layer struct {
	X, Y           int  // the picture's centre, from the frame's origin
	Index          int  // picture number in the .spr (-1: none)
	Mirror         bool // flipped left to right
	RGBA           bool // a true-colour picture rather than a paletted one
	Color          [4]uint8
	ScaleX, ScaleY float32
	Angle          int
}

// DecodeAct reads a .act file.
func DecodeAct(b []byte) (*Act, error) {
	if len(b) < 16 || b[0] != 'A' || b[1] != 'C' {
		return nil, errors.New("sprite: not a .act file")
	}
	ver := int(b[3])<<8 | int(b[2]) // 0x205 = 2.5
	r := reader{b: b, at: 4}
	n := int(r.u16())
	r.bytes(10)
	a := &Act{}
	for range n {
		nf := int(r.u32())
		if nf > 1000 {
			return nil, fmt.Errorf("sprite: implausible frame count %d", nf)
		}
		frames := make([]Frame, 0, nf)
		for range nf {
			r.bytes(32) // unused ranges
			nl := int(r.u32())
			if nl > 1000 {
				return nil, fmt.Errorf("sprite: implausible layer count %d", nl)
			}
			var f Frame
			for range nl {
				l := Layer{X: int(r.i32()), Y: int(r.i32()), Index: int(r.i32()), Mirror: r.i32() != 0,
					Color: [4]uint8{255, 255, 255, 255}, ScaleX: 1, ScaleY: 1}
				if ver >= 0x200 {
					c := r.bytes(4)
					l.Color = [4]uint8{c[0], c[1], c[2], c[3]}
					l.ScaleX = r.f32()
					l.ScaleY = l.ScaleX
					if ver >= 0x204 {
						l.ScaleY = r.f32()
					}
					l.Angle = int(r.i32())
					l.RGBA = r.i32() == 1
					if ver >= 0x205 {
						r.bytes(8) // width, height
					}
				}
				f.Layers = append(f.Layers, l)
			}
			if ver >= 0x200 {
				r.i32() // sound event
			}
			if ver >= 0x203 {
				na := int(r.u32())
				if na > 100 {
					return nil, fmt.Errorf("sprite: implausible anchor count %d", na)
				}
				for range na {
					r.bytes(4)
					x, y := int(r.i32()), int(r.i32())
					r.i32()
					f.Anchors = append(f.Anchors, [2]int{x, y})
				}
			}
			frames = append(frames, f)
		}
		a.Actions = append(a.Actions, frames)
	}
	if r.err != nil {
		return nil, fmt.Errorf("sprite: %w", r.err)
	}
	return a, nil
}
