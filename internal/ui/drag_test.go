package ui

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"
)

// drag presses at from and moves through the points, batch events per
// frame, then releases. Each frame's drawn position goes to at.
func (h *harness) drag(w *Window, from image.Point, batch int, to ...image.Point) (at []image.Point) {
	ev := func(k pointer.Kind, p image.Point) pointer.Event {
		return pointer.Event{Kind: k, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary,
			Position: f32.Pt(float32(p.X), float32(p.Y)), Time: time.Since(time.Time{})}
	}
	h.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(float32(from.X), float32(from.Y))})
	h.frame()
	h.router.Queue(ev(pointer.Press, from))
	h.frame()
	for i, p := range to {
		h.router.Queue(ev(pointer.Move, p))
		if (i+1)%batch == 0 {
			h.frame()
			at = append(at, w.rect.Min)
		}
	}
	h.frame()
	e := ev(pointer.Release, to[len(to)-1])
	e.Buttons = 0
	h.router.Queue(e)
	h.frame()
	h.frame()
	return at
}

// TestDragFloat: a floating window follows the pointer exactly and without
// going back, with one or several events per frame; its height stays.
func TestDragFloat(t *testing.T) {
	for _, batch := range []int{1, 3} {
		h := newHarness(t)
		w := h.a.wHelp
		w.Open = true
		h.a.Desk.Raise(w)
		h.frame()
		h.frame()
		start, height := w.Pos, w.rect.Dy()
		from := start.Add(image.Pt(60, 8))
		var path []image.Point
		for i := 1; i <= 12; i++ {
			path = append(path, from.Add(image.Pt(i*7, i*20)))
		}
		at := h.drag(w, from, batch, path...)
		for i := 1; i < len(at); i++ {
			if at[i].Y < at[i-1].Y || at[i].X < at[i-1].X {
				t.Errorf("batch %d: jumped back: %v", batch, at)
				break
			}
		}
		if want := start.Add(image.Pt(84, 240)); w.Pos != want {
			t.Errorf("batch %d: at %v, want %v (from %v)", batch, w.Pos, want, start)
		}
		if w.rect.Min != w.Pos {
			t.Errorf("batch %d: drawn at %v, not at %v", batch, w.rect.Min, w.Pos)
		}
		if w.rect.Dy() != height {
			t.Errorf("batch %d: height %d -> %d", batch, height, w.rect.Dy())
		}
		// Far past the corner: the title stays in view.
		from = w.Pos.Add(image.Pt(60, 8))
		h.drag(w, from, batch, from.Add(image.Pt(3000, 3000)))
		if w.rect.Min.Y > h.size.Y-20 || w.rect.Max.X > h.size.X+1 {
			t.Errorf("batch %d: out of view at %v", batch, w.rect)
		}
	}
}

// TestFloatBlocksPointer: a press on an empty part of a floating window does
// not reach what is behind it.
func TestFloatBlocksPointer(t *testing.T) {
	h := newHarness(t)
	before := h.a.Build().Stats[0]
	w := h.a.wHelp
	w.Open = true
	h.a.Desk.Raise(w)
	h.frame()
	w.Pos, w.Placed = image.Pt(150, 120), true // over FOR's "+" (304, 181)
	h.frame()
	h.frame()
	if !image.Pt(304, 181).In(w.rect) {
		t.Skip("the help window does not cover the button")
	}
	h.click(304, 181)
	if got := h.a.Build().Stats[0]; got != before {
		t.Errorf("the press went through: FOR %d -> %d", before, got)
	}
}
