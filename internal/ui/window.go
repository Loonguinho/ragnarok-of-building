package ui

import (
	"image"

	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
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
	// Min: minimized to the title bar (the ⊖ button, as in the client).
	Min bool
	// Float: drawn above the columns and dragged by its title even in the
	// fixed layout (first shown in the middle of the screen).
	Float bool
	// AutoClose: a press outside it (and outside other floating windows
	// and popups) closes it, as a dropdown.
	AutoClose bool
	// Dropdown: drawn as a dropdown's list (no title bar, a plain white
	// box) hanging from what opened it; Escape closes it.
	Dropdown bool

	// Groups: a window whose Members are drawn as its side tabs (one at a
	// time); a member is not drawn on its own. Icon is the member's tab
	// icon (an item icon path).
	Members []*Window
	Icon    string
	parent  *Window
	cur     *Window
	side    SideTabs
	// wasShowing: the group was open last frame (to tell a member closed
	// from its own body from one never opened).
	wasShowing bool

	// The body scrolls inside the window when it is taller than the room
	// the desk gives it, unless FullHeight (the window keeps its size).
	scroll     ScrollList
	FullHeight bool
	// natural is the height the window wanted last frame (title, whole
	// body and footer): the desk shares a column's height out with it.
	natural int

	z     int
	drag  gesture.Drag
	press image.Point
	drawn image.Point // where the last frame drew it: pointer positions are relative to it
	block bool        // tag of the area that keeps the pointer off what is behind
	close widget.Clickable
	mini  widget.Clickable
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
	if w.mini.Clicked(gtx) {
		w.Min = !w.Min
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
				// The pointer is where the last frame drew the window plus
				// this position, however many events came since.
				w.Pos = w.drawn.Add(e.Position.Round().Sub(w.press))
				w.Pos.X = max(w.Pos.X, -width+gtx.Dp(60))
				w.Pos.Y = max(w.Pos.Y, 0)
				d.changed = true
				gtx.Execute(op.InvalidateCmd{})
			}
		}
	}

	if w.Dropdown {
		return w.layoutDropdown(gtx, d, width)
	}
	titleH := gtx.Dp(TitleH)
	// Body and footer.
	bodyM := op.Record(gtx.Ops)
	g := gtx
	g.Constraints.Min.X = width
	// Room for the body: what the desk gives (a finite height) less the
	// title; the body then scrolls inside.
	room := 1 << 20
	if gtx.Constraints.Max.Y < 1<<19 && !w.FullHeight {
		room = gtx.Constraints.Max.Y - titleH - 2
		if w.Footer != nil && !w.Min {
			room -= gtx.Dp(30)
		}
	}
	g.Constraints.Max.Y = 1 << 20
	body := func(gtx C) D {
		return Inset(6, 8, 6, 8).Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			if w.Body == nil || w.Min {
				return D{}
			}
			return w.Body(gtx)
		})
	}
	natBody, natFoot := 0, 0
	bd := layout.Flex{Axis: layout.Vertical}.Layout(g,
		layout.Rigid(func(gtx C) D {
			if room >= 1<<19 || w.Min {
				d := body(gtx)
				natBody = d.Size.Y
				return d
			}
			// Measure; only a body taller than the room scrolls.
			m := op.Record(gtx.Ops)
			fd := body(gtx)
			call := m.Stop()
			natBody = fd.Size.Y
			if fd.Size.Y <= room && w.scroll.List.Position.First == 0 && w.scroll.List.Position.Offset == 0 {
				call.Add(gtx.Ops)
				return fd
			}
			gtx.Constraints.Max.Y = max(room, gtx.Dp(80))
			gtx.Constraints.Min.Y = 0
			return w.scroll.Layout(gtx, 1, func(gtx C, _ int) D { return body(gtx) })
		}),
		layout.Rigid(func(gtx C) D {
			if w.Footer == nil || w.Min {
				return D{}
			}
			mm := op.Record(gtx.Ops)
			empty := false
			fd := Inset(4, 6, 4, 6).Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				d := w.Footer(gtx)
				empty = d.Size.Y == 0
				return d
			})
			call := mm.Stop()
			if empty { // a group whose tab has no footer
				return D{}
			}
			r := image.Rectangle{Max: image.Pt(width, fd.Size.Y)}
			natFoot = fd.Size.Y
			FillRound(gtx, r, ColBodyAlt, 0, 0, 3, 3)
			Fill(gtx, image.Rect(0, 0, width, 1), ColSep)
			call.Add(gtx.Ops)
			return D{Size: r.Max}
		}),
	)
	bodyCall := bodyM.Stop()
	h := titleH + bd.Size.Y + 2
	w.natural = titleH + 2 + natBody + natFoot
	if w.Min {
		h = titleH + 2
		w.natural = h
	}
	full := image.Rectangle{Max: image.Pt(width, h)}

	// Frame.
	if floating {
		w.blockPointer(gtx, full)
	}
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
	right -= gtx.Dp(13) + gtx.Dp(3)
	func() {
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(gtx.Dp(13), gtx.Dp(13))}
		st := op.Offset(image.Pt(right-gtx.Dp(13), gtx.Dp(3))).Push(gtx.Ops)
		defer st.Pop()
		sym := "–"
		if w.Min {
			sym = "+"
		}
		SmallBtn(g, th, &w.mini, sym, ColWhite, rgb(0xc9d6ee), ColBorder, false)
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
	if !w.Min {
		st := op.Offset(image.Pt(0, titleH+2)).Push(gtx.Ops)
		bodyCall.Add(gtx.Ops)
		st.Pop()
	}
	Border(gtx, full, ColBorder, 1, gtx.Dp(5))
	// Press anywhere raises.
	area := clip.Rect(full).Push(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &w.tag)
	pass.Pop()
	area.Pop()
	return D{Size: full.Max}
}

// blockPointer keeps presses, hovers and the wheel from reaching what is
// behind the window; its own content, drawn after, still gets them.
func (w *Window) blockPointer(gtx C, r image.Rectangle) {
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: &w.block, Kinds: pointer.Press | pointer.Release | pointer.Move | pointer.Scroll,
			ScrollY: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20}}); !ok {
			break
		}
	}
	st := clip.Rect(r).Push(gtx.Ops)
	event.Op(gtx.Ops, &w.block)
	st.Pop()
}

// layoutDropdown draws a Dropdown window: its body in a white box with the
// dropdown's border, scrolling inside when taller than the room below.
func (w *Window) layoutDropdown(gtx C, d *Desk, width int) D {
	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameEscape})
		if !ok {
			break
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press && d.popup == nil {
			w.Open = false
			d.changed = true
		}
	}
	room := gtx.Constraints.Max.Y - 2
	m := op.Record(gtx.Ops)
	g := gtx
	g.Constraints = layout.Constraints{Min: image.Pt(width-2, 0), Max: image.Pt(width-2, 1<<20)}
	body := func(gtx C) D {
		return Inset(6, 8, 6, 8).Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			if w.Body == nil {
				return D{}
			}
			return w.Body(gtx)
		})
	}
	fm := op.Record(gtx.Ops)
	bd := body(g)
	fcall := fm.Stop()
	w.natural = bd.Size.Y + 2
	if bd.Size.Y <= room && w.scroll.List.Position.First == 0 && w.scroll.List.Position.Offset == 0 {
		fcall.Add(gtx.Ops)
	} else {
		g.Constraints.Max.Y = max(room, gtx.Dp(80))
		bd = w.scroll.Layout(g, 1, func(gtx C, _ int) D { return body(gtx) })
	}
	call := m.Stop()
	full := image.Rectangle{Max: image.Pt(width, bd.Size.Y+2)}
	w.blockPointer(gtx, full)
	Shadow(gtx, full, gtx.Dp(4))
	Fill(gtx, full, ColWhite)
	st := op.Offset(image.Pt(1, 1)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	st.Pop()
	Border(gtx, full, ColListBorder, 1, 0)
	area := clip.Rect(full).Push(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &w.tag)
	pass.Pop()
	area.Pop()
	return D{Size: full.Max}
}
