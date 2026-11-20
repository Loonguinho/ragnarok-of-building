package ui

import (
	"image"

	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
)

// Window is an RO window (.ro-window): a title bar with the × button, a
// body and an optional footer. In the fixed layout the Desk stacks it in
// its column; floating, it is dragged by the title bar.
type Window struct {
	ID, Title string
	Hotkey    string // Alt+letter that toggles it ("" = none)
	Col       int    // column in the fixed layout (0, 1, 2)
	Width     unit.Dp
	Open      bool
	NoShowAll bool // left out of "Abrir todas as janelas"

	Body       layout.Widget
	Footer     layout.Widget // nil = none
	TitleExtra layout.Widget // drawn at the right of the title (the gear)

	// Floating position (px); Placed false = not placed yet (cascade).
	Pos    image.Point
	Placed bool

	z     int
	drag  gesture.Drag
	press image.Point
	close widget.Clickable
	rect  image.Rectangle // last drawn, desk coordinates
	tag   bool
}

// Layout draws the window at the current offset; floating moves it with
// the title bar's drag.
func (w *Window) layout(gtx C, th *Theme, d *Desk, floating bool) D {
	if w.close.Clicked(gtx) {
		w.Open = false
		d.changed = true
	}
	width := gtx.Dp(w.Width)
	if width == 0 || width > gtx.Constraints.Max.X {
		width = gtx.Constraints.Max.X
	}
	gtx.Constraints = layout.Exact(image.Pt(width, gtx.Constraints.Max.Y))
	gtx.Constraints.Min.Y = 0

	// Raise on any press inside the window.
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &w.tag, Kinds: pointer.Press})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok && e.Kind == pointer.Press {
			d.Raise(w)
		}
	}
	if floating {
		for {
			e, ok := w.drag.Update(gtx.Metric, gtx.Source, gesture.Both)
			if !ok {
				break
			}
			switch e.Kind {
			case pointer.Press:
				w.press = e.Position.Round()
				d.Raise(w)
			case pointer.Drag:
				delta := e.Position.Round().Sub(w.press)
				w.Pos = w.Pos.Add(delta)
				w.Pos.X = max(w.Pos.X, -width+gtx.Dp(60))
				w.Pos.Y = max(w.Pos.Y, 0)
				d.changed = true
			}
		}
	}

	titleH := gtx.Dp(TitleH)
	// Body and footer.
	bodyM := op.Record(gtx.Ops)
	g := gtx
	g.Constraints.Min.X = width
	g.Constraints.Max.Y = 1 << 20
	bd := layout.Flex{Axis: layout.Vertical}.Layout(g,
		layout.Rigid(func(gtx C) D {
			return Inset(6, 8, 6, 8).Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				if w.Body == nil {
					return D{}
				}
				return w.Body(gtx)
			})
		}),
		layout.Rigid(func(gtx C) D {
			if w.Footer == nil {
				return D{}
			}
			mm := op.Record(gtx.Ops)
			fd := Inset(4, 6, 4, 6).Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return w.Footer(gtx)
			})
			call := mm.Stop()
			r := image.Rectangle{Max: image.Pt(width, fd.Size.Y)}
			FillRound(gtx, r, ColBodyAlt, 0, 0, 3, 3)
			Fill(gtx, image.Rect(0, 0, width, 1), ColSep)
			call.Add(gtx.Ops)
			return D{Size: r.Max}
		}),
	)
	bodyCall := bodyM.Stop()
	h := titleH + bd.Size.Y + 2
	full := image.Rectangle{Max: image.Pt(width, h)}

	// Frame.
	Shadow(gtx, full, gtx.Dp(6))
	FillRound(gtx, full, ColBody, gtx.Dp(6), gtx.Dp(6), gtx.Dp(3), gtx.Dp(3))
	// Title bar.
	tr := image.Rectangle{Min: image.Pt(1, 1), Max: image.Pt(width-1, titleH+1)}
	VGradient(gtx, tr, ColTitleA, ColTitleB, gtx.Dp(5), gtx.Dp(5), 0, 0)
	Fill(gtx, image.Rect(1, titleH+1, width-1, titleH+2), ColBorder)
	func() {
		st := op.Offset(image.Pt(gtx.Dp(8), gtx.Dp(3))).Push(gtx.Ops)
		defer st.Pop()
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(width-gtx.Dp(30), titleH)}
		B(th, w.Title).C(ColTitleText).Layout(g)
	}()
	if floating {
		st := clip.Rect(tr).Push(gtx.Ops)
		w.drag.Add(gtx.Ops)
		pointer.CursorAllScroll.Add(gtx.Ops)
		st.Pop()
	}
	// Title extras and the × button.
	right := width - gtx.Dp(3)
	func() {
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(gtx.Dp(13), gtx.Dp(13))}
		st := op.Offset(image.Pt(right-gtx.Dp(13), gtx.Dp(3))).Push(gtx.Ops)
		defer st.Pop()
		SmallBtn(g, th, &w.close, "×", ColWhite, rgb(0xc9d6ee), ColBorder, false)
	}()
	right -= gtx.Dp(13) + gtx.Dp(4)
	if w.TitleExtra != nil {
		mm := op.Record(gtx.Ops)
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(right, titleH)}
		ed := w.TitleExtra(g)
		c := mm.Stop()
		st := op.Offset(image.Pt(right-ed.Size.X, (titleH-ed.Size.Y)/2+1)).Push(gtx.Ops)
		c.Add(gtx.Ops)
		st.Pop()
	}
	// Body.
	st := op.Offset(image.Pt(0, titleH+2)).Push(gtx.Ops)
	bodyCall.Add(gtx.Ops)
	st.Pop()
	Border(gtx, full, ColBorder, 1, gtx.Dp(5))
	// Press anywhere raises.
	area := clip.Rect(full).Push(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &w.tag)
	pass.Pop()
	area.Pop()
	return D{Size: full.Max}
}
