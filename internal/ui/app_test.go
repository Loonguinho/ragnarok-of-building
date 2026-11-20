package ui

import (
	"image"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"ragnarok-of-building/internal/calc"
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

// waitSearch waits for the item picker's results.
func (h *harness) waitSearch() {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		h.a.mu.Lock()
		done := !h.a.slot.pending
		h.a.mu.Unlock()
		if done {
			h.frame()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	h.t.Fatal("the picker never answered")
}

func TestEquipFlow(t *testing.T) {
	h := newHarness(t)
	a := h.a
	a.openSlot(calc.Weapon)
	h.waitSearch()
	h.frame()
	a.mu.Lock()
	results := a.slot.results
	a.mu.Unlock()
	if len(results) == 0 {
		t.Fatal("no weapons offered")
	}
	var weapon view.ItemView
	for _, it := range results {
		if it.Slots > 0 {
			weapon = it
			break
		}
	}
	if weapon.ID == 0 {
		t.Skip("no slotted weapon in the first results")
	}
	a.pick(weapon.ID)
	h.waitReady()
	h.waitSearch()
	if got := a.Build().Equip[calc.Weapon].Item; got != weapon.ID {
		t.Fatalf("weapon %d, want %d", got, weapon.ID)
	}
	// A card in its first slot.
	a.slot.card = 0
	a.search(0)
	h.waitSearch()
	a.mu.Lock()
	cards := a.slot.results
	a.mu.Unlock()
	if len(cards) == 0 {
		t.Fatal("no cards offered")
	}
	a.pick(cards[0].ID)
	h.waitReady()
	h.frame()
	if got := a.Build().Equip[calc.Weapon].Cards[0]; got != cards[0].ID {
		t.Errorf("card %d, want %d", got, cards[0].ID)
	}
	// The tooltip with its comparison draws.
	a.Desk.ShowTip(func(gtx C) D {
		return a.itemTip(gtx, weapon.ID, &view.SwapView{Current: "x", Lines: []view.StatLine{{Label: "ATQ", Old: "1", New: "2", Diff: "+1", Better: true}}})
	})
	h.frame()
}

// TestSelectPick opens dropdowns and picks an option: the popup used to
// lay a click catcher over its own content, so options ignored clicks.
func TestSelectPick(t *testing.T) {
	h := newHarness(t)
	// Atributos ▸ Classe (fixed layout, as in the screenshot).
	before := h.a.Build().Job
	h.click(135, 373)
	if h.a.Desk.popup == nil {
		t.Fatal("the class select did not open")
	}
	h.click(135, 373+40)
	if h.a.Desk.popup != nil {
		t.Error("the popup stayed open after a pick")
	}
	if got := h.a.Build().Job; got == before {
		t.Errorf("class still %d after picking another", got)
	}
	// Informações Básicas ▸ the class name opens the grouped class list.
	h.click(115, 214)
	if h.a.Desk.popup == nil {
		t.Fatal("the class list did not open")
	}
	job := h.a.Build().Job
	h.click(115, 214+60)
	if h.a.Desk.popup != nil {
		t.Error("the class list stayed open after a pick")
	}
	if h.a.Build().Job == job {
		t.Error("picking in the class list changed nothing")
	}
}

// TestStatTyped types a value into FOR's box.
func TestStatTyped(t *testing.T) {
	h := newHarness(t)
	in := &h.a.status.stat[0]
	h.click(72, 404) // FOR's box (fixed layout)
	if !in.focused {
		t.Fatal("FOR's box did not take the focus")
	}
	n := in.Ed.Len()
	h.router.Queue(key.EditEvent{Range: key.Range{Start: 0, End: n}, Text: "77"})
	h.frame()
	h.waitReady()
	h.frame()
	if got := h.a.Build().Stats[0]; got != 77 {
		t.Fatalf("FOR = %d after typing 77", got)
	}
	// Past the cap: clamped.
	h.router.Queue(key.EditEvent{Range: key.Range{Start: 0, End: in.Ed.Len()}, Text: "999"})
	h.frame()
	h.waitReady()
	h.frame()
	if got := h.a.Build().Stats[0]; got != h.a.View().R.MaxStat {
		t.Fatalf("FOR = %d after typing 999, want the cap %d", got, h.a.View().R.MaxStat)
	}
}
