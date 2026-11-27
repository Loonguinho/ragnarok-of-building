package ui

import (
	"testing"
	"time"

	"gioui.org/io/key"

	"ragnarok-of-building/internal/calc"
)

// waitCompare draws frames until the comparison is computed.
func (h *harness) waitCompare() {
	h.t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		h.frame()
		h.a.mu.Lock()
		ready, dirty := h.a.ecmp.ready, h.a.ecmp.dirty
		h.a.mu.Unlock()
		if ready && !dirty {
			time.Sleep(50 * time.Millisecond)
			h.frame()
			h.a.mu.Lock()
			still := !h.a.ecmp.dirty
			h.a.mu.Unlock()
			if still {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	h.t.Fatal("the comparison never computed")
}

func TestEquipmentComparison(t *testing.T) {
	h := newHarness(t)
	a := h.a
	a.Edit(func(b *calc.Build) {
		b.Job = 4252
		if b.Equip == nil {
			b.Equip = map[string]calc.Equip{}
		}
		b.Equip[calc.Weapon] = calc.Equip{Item: 1201, Refine: 9} // Faca
	})
	h.waitReady()
	before := a.Build()

	// Put the lower headgear in comparison and pick an item in its editor.
	a.openCompare(calc.HeadLow)
	if !a.slot.cmp || a.slot.key != calc.HeadLow {
		t.Fatalf("editor on %q (comparison %v), want the lower headgear's comparison", a.slot.key, a.slot.cmp)
	}
	a.editSet(a.V.Swap(a.editBuild(), calc.HeadLow, 410706, -1)) // Bebê Lobo
	if got := a.Build().Equip[calc.HeadLow].Item; got != before.Equip[calc.HeadLow].Item {
		t.Fatalf("the build changed: lower headgear %d", got)
	}
	h.waitCompare()
	cv, ok := a.compareView()
	if !ok || cv.Build.Equip[calc.HeadLow].Item != 410706 {
		t.Fatalf("comparison lower headgear %d (ready %v), want 410706", cv.Build.Equip[calc.HeadLow].Item, ok)
	}

	// The build's other changes count in the comparison too.
	a.Edit(func(b *calc.Build) { b.Stats[0] = 99 })
	h.waitReady()
	h.waitCompare()
	if cv, _ := a.compareView(); cv.Build.Stats[0] != 99 {
		t.Fatalf("comparison STR %d, want the build's 99", cv.Build.Stats[0])
	}

	// Aplicar equips it, and Ctrl+Z takes it back off.
	a.applyCompare()
	h.waitReady()
	if got := a.Build().Equip[calc.HeadLow].Item; got != 410706 {
		t.Fatalf("after Aplicar the lower headgear is %d", got)
	}
	if len(a.ecmp.open) != 0 || a.slot.cmp {
		t.Fatal("the comparison stayed open after Aplicar")
	}
	h.frame()
	h.router.Queue(key.Event{Name: "Z", Modifiers: key.ModShortcut, State: key.Press})
	h.frame()
	h.frame()
	h.waitReady()
	if got := a.Build().Equip[calc.HeadLow].Item; got != before.Equip[calc.HeadLow].Item {
		t.Fatalf("Ctrl+Z left the lower headgear %d", got)
	}

	// Two slots, then one taken out, then Limpar.
	a.openCompare(calc.HeadLow)
	a.editSet(a.V.Swap(a.editBuild(), calc.HeadLow, 410706, -1))
	a.openCompare(calc.Shoes)
	a.editSet(a.V.Swap(a.editBuild(), calc.Shoes, 2402, -1)) // Sandálias [1]
	if len(a.ecmp.over) != 2 {
		t.Fatalf("%d slots differ, want 2: %v", len(a.ecmp.over), a.ecmp.over)
	}
	a.closeCompare(calc.Shoes)
	if a.comparing(calc.Shoes) || a.ecmp.over[calc.Shoes] != nil || !a.comparing(calc.HeadLow) {
		t.Fatalf("after closing the shoes: open %v, over %v", a.ecmp.open, a.ecmp.over)
	}
	a.clearCompare()
	if len(a.ecmp.open) != 0 || len(a.ecmp.over) != 0 {
		t.Fatal("Limpar left a comparison")
	}
	if _, ok := a.compareView(); ok {
		t.Fatal("a comparison is still shown after Limpar")
	}
}

func TestGainText(t *testing.T) {
	for _, c := range []struct {
		now, other float64
		lang, want string
	}{
		{613, 655, "pt", "▲ +6,9%"},
		{613, 590, "pt", "▼ −3,8%"},
		{613, 655, "en", "▲ +6.9%"},
		{613, 613, "pt", "="},
	} {
		if got := gainText(c.now, c.other, c.lang); got != c.want {
			t.Errorf("gainText(%v, %v, %s) = %q, want %q", c.now, c.other, c.lang, got, c.want)
		}
	}
	if betterColor(10, 5, true) != ColUp || betterColor(10, 5, false) != ColDown || betterColor(3, 3, false) != ColText {
		t.Error("betterColor picks the wrong side")
	}
}
