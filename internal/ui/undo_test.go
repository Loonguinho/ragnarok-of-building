package ui

import (
	"testing"

	"gioui.org/io/key"

	"ragnarok-of-building/internal/calc"
)

func TestUndo(t *testing.T) {
	h := newHarness(t)
	str := func(n int) {
		h.a.Edit(func(b *calc.Build) { b.Stats[0] = n })
		h.waitReady()
	}
	want := func(n int) {
		t.Helper()
		h.waitReady()
		if got := h.a.Build().Stats[0]; got != n {
			t.Fatalf("STR = %d, want %d", got, n)
		}
	}
	press := func(n key.Name, m key.Modifiers) {
		h.frame()
		h.router.Queue(key.Event{Name: n, Modifiers: m, State: key.Press})
		h.frame()
		h.frame()
	}
	start := h.a.Build().Stats[0]
	press("Z", key.ModShortcut) // nothing to undo yet
	want(start)
	str(50)
	str(50) // no change: not a step
	str(60)
	press("Z", key.ModShortcut)
	want(50)
	press("Z", key.ModShortcut)
	want(start)
	press("Y", key.ModShortcut)
	want(50)
	press("Z", key.ModShortcut|key.ModShift)
	want(60)
	press("Z", key.ModShortcut)
	want(50)
	str(70) // a new change drops the redo
	press("Y", key.ModShortcut)
	want(70)
	// Another build starts the history over.
	b := h.a.Build()
	b.Stats[0] = 33
	h.a.OpenBuild(b)
	want(33)
	press("Z", key.ModShortcut)
	want(33)
}
