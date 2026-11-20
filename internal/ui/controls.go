package ui

import (
	"image"
	"image/color"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
)

func imagePt(x, y int) image.Point { return image.Pt(x, y) }

// pointerCursor shows the hand over a clickable area.
func pointerCursor(gtx C, size image.Point) {
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	pointer.CursorPointer.Add(gtx.Ops)
}

// Button is the RO button (.ro-btn): gradient, border, 18px high.
type Button struct {
	Label    string
	MinWidth unit.Dp
	Disabled bool
	Bold     bool
	Wide     bool // fill the width available
}

// Layout draws the button over the clickable.
func (b Button) Layout(gtx C, th *Theme, cl *widget.Clickable) D {
	if b.Disabled {
		return b.draw(gtx, th, false, false, true)
	}
	return cl.Layout(gtx, func(gtx C) D {
		d := b.draw(gtx, th, cl.Hovered(), cl.Pressed(), false)
		pointerCursor(gtx, d.Size)
		return d
	})
}

func (b Button) draw(gtx C, th *Theme, hover, pressed, disabled bool) D {
	h := gtx.Dp(BtnH)
	t := T(th, b.Label)
	if b.Bold {
		t = t.Bold()
	}
	if disabled {
		t = t.C(color.NRGBA{R: 0x99, G: 0x99, B: 0x99, A: 0xff})
	}
	m := op.Record(gtx.Ops)
	g := gtx
	g.Constraints.Min = image.Point{}
	td := t.Layout(g)
	call := m.Stop()
	w := max(td.Size.X+gtx.Dp(16), gtx.Dp(b.MinWidth))
	if b.Wide {
		w = max(w, gtx.Constraints.Max.X)
	}
	r := image.Rectangle{Max: image.Pt(w, h)}
	top, bot := ColBtnA, ColBtnB
	if pressed {
		top, bot = bot, top
	} else if hover {
		top, bot = lighten(top), lighten(bot)
	}
	rad := gtx.Dp(3)
	VGradient(gtx, r, top, bot, rad, rad, rad, rad)
	Border(gtx, r, ColBorder, 1, rad)
	st := op.Offset(image.Pt((w-td.Size.X)/2, (h-td.Size.Y)/2)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	st.Pop()
	return D{Size: r.Max}
}

func lighten(c color.NRGBA) color.NRGBA {
	f := func(v uint8) uint8 { return uint8(min(255, int(v)*106/100)) }
	return color.NRGBA{R: f(c.R), G: f(c.G), B: f(c.B), A: c.A}
}

// SmallBtn is the 13px "+" / "−" / "×" button (.ro-plus, .ro-minus,
// .ro-window__close).
func SmallBtn(gtx C, th *Theme, cl *widget.Clickable, label string, top, bottom, line color.NRGBA, disabled bool) D {
	draw := func(gtx C) D {
		s := gtx.Dp(13)
		r := image.Rectangle{Max: image.Pt(s, s)}
		t, b := top, bottom
		if !disabled && cl.Hovered() {
			t, b = lighten(t), lighten(b)
		}
		VGradient(gtx, r, t, b, 2, 2, 2, 2)
		Border(gtx, r, line, 1, 2)
		txt := B(th, label).Sz(SmallText).C(ColTitleText)
		if disabled {
			txt = txt.C(color.NRGBA{R: 0x2b, G: 0x2b, B: 0x2b, A: 0x59})
		}
		m := op.Record(gtx.Ops)
		g := gtx
		g.Constraints.Min = image.Point{}
		td := txt.Layout(g)
		call := m.Stop()
		st := op.Offset(image.Pt((s-td.Size.X)/2, (s-td.Size.Y)/2-1)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		st.Pop()
		if !disabled {
			pointerCursor(gtx, r.Max)
		}
		return D{Size: r.Max}
	}
	if disabled {
		return draw(gtx)
	}
	return cl.Layout(gtx, draw)
}

// PlusBtn and MinusBtn are the stat buttons.
func PlusBtn(gtx C, th *Theme, cl *widget.Clickable, disabled bool) D {
	return SmallBtn(gtx, th, cl, "+", ColWhite, rgb(0xf0d98a), rgb(0xa08a3a), disabled)
}
func MinusBtn(gtx C, th *Theme, cl *widget.Clickable, disabled bool) D {
	return SmallBtn(gtx, th, cl, "-", ColWhite, rgb(0xd9dfeb), ColBorder, disabled)
}

// YesNo is the Sim/Não pair of the Buffs window.
type YesNo struct{ Yes, No widget.Clickable }

// Layout draws the pair; on is the current value. It returns the clicked
// value (1 Sim, 0 Não) and true when one was clicked.
func (y *YesNo) Layout(gtx C, th *Theme, on bool, yes, no string) (D, int, bool) {
	v, clicked := 0, false
	if y.Yes.Clicked(gtx) {
		v, clicked = 1, true
	}
	if y.No.Clicked(gtx) {
		v, clicked = 0, true
	}
	half := func(cl *widget.Clickable, label string, active bool, left bool) layout.Widget {
		return func(gtx C) D {
			return cl.Layout(gtx, func(gtx C) D {
				m := op.Record(gtx.Ops)
				g := gtx
				g.Constraints.Min = image.Point{}
				t := T(th, label).Sz(SmallText)
				if active {
					t = t.Bold().C(ColWhite)
				}
				td := t.Layout(g)
				call := m.Stop()
				w, h := max(td.Size.X+gtx.Dp(10), gtx.Dp(30)), gtx.Dp(16)
				r := image.Rectangle{Max: image.Pt(w, h)}
				nw, sw, ne, se := 3, 3, 0, 0
				if !left {
					nw, sw, ne, se = 0, 0, 3, 3
				}
				if active {
					c := ColOn
					if !left {
						c = rgb(0x9a9a9a)
					}
					FillRound(gtx, r, c, nw, ne, se, sw)
				} else {
					VGradient(gtx, r, ColBtnA, ColBtnB, nw, ne, se, sw)
				}
				Border(gtx, r, ColBorder, 1, 0)
				st := op.Offset(image.Pt((w-td.Size.X)/2, (h-td.Size.Y)/2)).Push(gtx.Ops)
				call.Add(gtx.Ops)
				st.Pop()
				pointerCursor(gtx, r.Max)
				return D{Size: r.Max}
			})
		}
	}
	d := layout.Flex{}.Layout(gtx,
		layout.Rigid(half(&y.Yes, yes, on, true)),
		layout.Rigid(half(&y.No, no, !on, false)),
	)
	return d, v, clicked
}

// Checkbox is a small box with a label.
type Checkbox struct{ cl widget.Clickable }

// Layout returns true when toggled.
func (c *Checkbox) Layout(gtx C, th *Theme, checked bool, label string) (D, bool) {
	toggled := c.cl.Clicked(gtx)
	d := c.cl.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				s := gtx.Dp(13)
				r := image.Rectangle{Max: image.Pt(s, s)}
				Fill(gtx, r, ColWhite)
				Border(gtx, r, ColBorder, 1, 2)
				if checked {
					in := gtx.Dp(3)
					FillRound(gtx, image.Rectangle{Min: image.Pt(in, in), Max: image.Pt(s-in, s-in)}, ColLink, 1, 1, 1, 1)
				}
				return D{Size: r.Max}
			}),
			layout.Rigid(Gap(4, 0)),
			layout.Rigid(T(th, label).W()),
		)
	})
	pointerCursor(gtx, d.Size)
	return d, toggled
}
