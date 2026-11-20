package ui

import (
	"image"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/view"
)

// harness drives the app without a window: frames on an input router,
// with simulated pointer events.
type harness struct {
	t      *testing.T
	a      *App
	router input.Router
	ops    op.Ops
	size   image.Point
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	cat, err := gamedata.Load(filepath.Join("..", "..", "data", "gen", "catalog.json.gz"))
	if err != nil {
		t.Skip("no generated catalog:", err)
	}
	v, err := view.New(view.Options{Catalog: cat})
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, size: image.Pt(1440, 900)}
	h.a = NewApp(v, NewTheme(), "")
	h.waitReady()
	h.frame()
	return h
}

func (h *harness) waitReady() {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		h.a.mu.Lock()
		ready, pending := h.a.ready, h.a.rev
		h.a.mu.Unlock()
		if ready {
			// Let a running compute land.
			time.Sleep(50 * time.Millisecond)
			h.a.mu.Lock()
			same := pending == h.a.rev
			h.a.mu.Unlock()
			if same {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	h.t.Fatal("the build never computed")
}

func (h *harness) frame() {
	h.ops.Reset()
	gtx := layout.Context{
		Ops:         &h.ops,
		Now:         time.Now(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(h.size),
		Source:      h.router.Source(),
	}
	h.a.Frame(gtx)
	h.router.Frame(&h.ops)
}

// click presses and releases at (x, y) and draws the frames that follow.
func (h *harness) click(x, y float32, mods ...func(*pointer.Event)) {
	pos := f32.Pt(x, y)
	ev := func(k pointer.Kind) pointer.Event {
		e := pointer.Event{Kind: k, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: pos, Time: time.Since(time.Time{})}
		for _, m := range mods {
			m(&e)
		}
		return e
	}
	h.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: pos})
	h.frame()
	h.router.Queue(ev(pointer.Press))
	h.frame()
	h.router.Queue(ev(pointer.Release))
	h.frame()
	h.waitReady()
	h.frame()
}

func TestStatPlus(t *testing.T) {
	h := newHarness(t)
	before := h.a.Build().Stats[0]
	// FOR's "+" in the Atributos window (fixed layout, as in the screenshot).
	h.click(135, 402)
	if got := h.a.Build().Stats[0]; got != before+1 {
		t.Fatalf("FOR %d -> %d, want +1", before, got)
	}
}
