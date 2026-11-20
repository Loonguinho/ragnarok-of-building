//go:build windows

package ui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/gpu/headless"

	"ragnarok-of-building/internal/calc"
)

// shot draws the current frame into a PNG (Direct3D, no window).
func (h *harness) shot(name string) {
	dir := os.Getenv("RO_SHOTS")
	if dir == "" {
		return
	}
	h.frame()
	w, err := headless.NewWindow(h.size.X, h.size.Y)
	if err != nil {
		h.t.Fatal(err)
	}
	defer w.Release()
	if err := w.Frame(&h.ops); err != nil {
		h.t.Fatal(err)
	}
	img := image.NewRGBA(image.Rectangle{Max: h.size})
	if err := w.Screenshot(img); err != nil {
		h.t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		h.t.Fatal(err)
	}
	defer f.Close()
	png.Encode(f, img)
}

// TestShots renders the windows to RO_SHOTS (skipped without it), to look
// at them next to the old web version.
func TestShots(t *testing.T) {
	if os.Getenv("RO_SHOTS") == "" {
		t.Skip("set RO_SHOTS to a folder")
	}
	h := newHarness(t)
	a := h.a
	a.SetBuild(a.V.NewBuild(4252))
	h.waitReady()
	h.shot("01-start")
	a.openSlot(calc.Weapon)
	h.waitSearch()
	h.shot("02-slot-weapon")
	a.mu.Lock()
	res := a.slot.results
	a.mu.Unlock()
	for _, it := range res {
		if it.Slots >= 2 {
			a.pick(it.ID)
			break
		}
	}
	h.waitReady()
	h.waitSearch()
	h.shot("03-weapon-equipped")
}
