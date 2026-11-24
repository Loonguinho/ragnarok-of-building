package ui

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/view"
)

// undoMax is how many changes Ctrl+Z goes back.
const undoMax = 100

// history is the builds before and after the current one. Opening another
// build (saved, shared, replay, new) starts it over: going back across one
// could save the old build over the new one's entry.
type history struct {
	undo, redo []calc.Build
	step       bool // SetBuild is applying an undo or redo step
}

// record keeps the build the change nb replaces (a.mu held).
func (a *App) record(nb calc.Build) {
	h := &a.hist
	if !a.ready || h.step || view.EncodeBuild(a.b) == view.EncodeBuild(nb) {
		return
	}
	h.undo = append(h.undo, a.b.Clone())
	if len(h.undo) > undoMax {
		h.undo = h.undo[len(h.undo)-undoMax:]
	}
	h.redo = nil
}

// OpenBuild replaces the build with another one and forgets the changes of
// the one before.
func (a *App) OpenBuild(b calc.Build) {
	a.mu.Lock()
	a.hist.undo, a.hist.redo = nil, nil
	a.hist.step = true
	a.mu.Unlock()
	a.SetBuild(b)
	a.mu.Lock()
	a.hist.step = false
	a.mu.Unlock()
}

// Undo goes back one change of the build; Redo takes it again.
func (a *App) Undo() {
	a.stepHistory(&a.hist.undo, &a.hist.redo, "Mudança desfeita.", "Nada para desfazer.")
}

func (a *App) Redo() {
	a.stepHistory(&a.hist.redo, &a.hist.undo, "Mudança refeita.", "Nada para refazer.")
}

func (a *App) stepHistory(from, to *[]calc.Build, done, none string) {
	a.mu.Lock()
	cur := a.b.Clone()
	code := view.EncodeBuild(cur)
	var nb calc.Build
	found := false
	for len(*from) > 0 && !found {
		nb = (*from)[len(*from)-1]
		*from = (*from)[:len(*from)-1]
		// A change the rules took back (a shield under a two-handed
		// weapon) left the build as it was.
		found = view.EncodeBuild(nb) != code
	}
	if found {
		*to = append(*to, cur)
	}
	a.hist.step = found
	a.mu.Unlock()
	if !found {
		a.Say(a.T(none), false)
		return
	}
	a.SetBuild(nb)
	a.mu.Lock()
	a.hist.step = false
	a.mu.Unlock()
	a.Say(a.T(done), false)
	if a.wSlot.Open && a.slot.key != "" { // the editor may show an item that left
		a.slot.card, a.slot.bestOn = -1, false
		a.search(0)
	}
}

// undoButtons are the undo and redo arrows, dimmed when there is nothing to
// take back.
func (a *App) undoButtons(gtx C, cl *[2]widget.Clickable) D {
	if cl[0].Clicked(gtx) {
		a.Undo()
	}
	if cl[1].Clicked(gtx) {
		a.Redo()
	}
	a.mu.Lock()
	on := [2]bool{len(a.hist.undo) > 0, len(a.hist.redo) > 0}
	a.mu.Unlock()
	tips := [2]string{a.T("Desfazer") + " (Ctrl+Z)", a.T("Refazer") + " (Ctrl+Y)"}
	btn := func(i int) layout.Widget {
		return func(gtx C) D {
			b := Button{MinWidth: 30, Disabled: !on[i]}
			d := b.Layout(gtx, a.Th, &cl[i])
			c := ColTitleText
			if !on[i] {
				c = rgb(0x999999)
			}
			curvedArrow(gtx, d.Size, 14, i == 1, c)
			if on[i] && cl[i].Hovered() {
				a.Desk.ShowTip(func(gtx C) D { return TipBox(gtx, T(a.Th, tips[i]).W()) })
			}
			return d
		}
	}
	return Row(4, btn(0), btn(1))(gtx)
}

// curvedArrow draws the undo arrow in the middle of box (mirrored: redo).
func curvedArrow(gtx C, box image.Point, size unit.Dp, mirror bool, c color.NRGBA) {
	px := gtx.Dp(size)
	defer op.Offset(image.Pt((box.X-px)/2, (box.Y-px)/2)).Push(gtx.Ops).Pop()
	u := float32(px) / 14
	pt := func(x, y float32) f32.Point {
		if mirror {
			x = 14 - x
		}
		return f32.Pt(x*u, y*u)
	}
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(pt(2, 5))
	p.LineTo(pt(8.5, 5))
	p.CubeTo(pt(13, 5), pt(13, 12), pt(8.5, 12))
	p.LineTo(pt(4.5, 12))
	p.MoveTo(pt(5.5, 1.5))
	p.LineTo(pt(2, 5))
	p.LineTo(pt(5.5, 8.5))
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: p.End(), Width: 1.6 * u}.Op())
}
