package ui

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"
)

// TestTallTip: a description taller than the screen stays in place, keeps
// showing with the mouse on it, scrolls with the wheel to its end, and goes
// when the mouse leaves; the list behind does not scroll.
func TestTallTip(t *testing.T) {
	h := newHarness(t)
	a := h.a
	move := func(p image.Point) {
		h.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(float32(p.X), float32(p.Y))})
		h.frame()
		h.frame()
	}
	wheel := func(p image.Point, dy float32) {
		h.router.Queue(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: f32.Pt(float32(p.X), float32(p.Y)), Scroll: f32.Pt(0, dy)})
		h.frame()
		h.frame()
	}
	a.shellSelect(a.wItems)
	a.items.q.Ed.SetText("dom infernal")
	for range 4 {
		h.frame()
	}
	if len(a.items.results) == 0 || a.items.results[0].ID != 312021 {
		t.Fatalf("results %+v", a.items.results)
	}
	tall := &a.Desk.tall
	// Its row: the first point of the list where the tooltip shows.
	var src image.Point
	for y := 60; y < 140 && !tall.on; y += 6 {
		src = image.Pt(700, y)
		move(src)
	}
	if !tall.on {
		t.Fatal("no tall tooltip over the row")
	}
	if tall.rect.Min.Y < 0 || tall.rect.Max.Y > h.size.Y || tall.h <= tall.rect.Dy() {
		t.Fatalf("rect %v, height %d", tall.rect, tall.h)
	}
	in := tall.rect.Min.Add(image.Pt(30, 300))
	move(in)
	time.Sleep(tipGrace + 100*time.Millisecond)
	h.frame()
	h.frame()
	if !tall.on {
		t.Fatal("the tooltip went away with the mouse on it")
	}
	first := a.items.scroll.List.Position
	wheel(in, 120)
	if tall.scroll != 120 {
		t.Errorf("scrolled %d, want 120", tall.scroll)
	}
	wheel(in, 100000)
	if want := tall.h - tall.rect.Dy(); tall.scroll != want {
		t.Errorf("scrolled to %d, the end is %d", tall.scroll, want)
	}
	wheel(in, -100000)
	if tall.scroll != 0 {
		t.Errorf("scrolled back to %d", tall.scroll)
	}
	if a.items.scroll.List.Position != first {
		t.Error("the list behind scrolled")
	}
	move(image.Pt(5, h.size.Y-5))
	time.Sleep(tipGrace + 100*time.Millisecond)
	h.frame()
	h.frame()
	if tall.on {
		t.Error("the tooltip stayed after the mouse left")
	}
}
