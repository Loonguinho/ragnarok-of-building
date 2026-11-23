package ui

import (
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

// Input is a text box (.ro-input, 20px).
type Input struct {
	Ed      widget.Editor
	Hint    string
	changed bool
	// Compact is the 16px box that fits a table row (stats, traits).
	Compact bool
	// SelectOnFocus selects all the text when the box gets the focus, so
	// typing replaces it.
	SelectOnFocus bool
	focused       bool
}

// Layout draws the box; it returns true when the text changed (submitted
// is true on Enter).
func (in *Input) Layout(gtx C, th *Theme, width unit.Dp) (d D, changed, submitted bool) {
	in.Ed.SingleLine = true
	in.Ed.Submit = true
	for {
		e, ok := in.Ed.Update(gtx)
		if !ok {
			break
		}
		switch e.(type) {
		case widget.ChangeEvent:
			changed = true
		case widget.SubmitEvent:
			submitted = true
		}
	}
	w := gtx.Dp(width)
	if w == 0 || w > gtx.Constraints.Max.X {
		w = gtx.Constraints.Max.X
	}
	h, padX, padY := gtx.Dp(InputH), gtx.Dp(6), gtx.Dp(3)
	if in.Compact {
		h, padX, padY = gtx.Dp(16), gtx.Dp(3), gtx.Dp(1)
	}
	focused := gtx.Focused(&in.Ed)
	if focused && !in.focused && in.SelectOnFocus {
		in.Ed.SetCaret(in.Ed.Len(), 0)
	}
	in.focused = focused
	r := image.Rectangle{Max: image.Pt(w, h)}
	Fill(gtx, r, ColWhite)
	line := ColBorder
	if focused {
		line = rgb(0x7f9fd9)
	}
	Border(gtx, r, line, 1, 2)
	g := gtx
	g.Constraints = layout.Exact(image.Pt(w-2*padX, h-padY-1))
	st := op.Offset(image.Pt(padX, padY)).Push(gtx.Ops)
	if in.Ed.Len() == 0 && in.Hint != "" {
		T(th, in.Hint).C(rgb(0x999999)).Layout(g)
	}
	// Gio's editor text is unhinted (blurry): while the box isn't being
	// typed in, the editor draws invisible text and the hinted text goes
	// on top.
	textCol := ColText
	if !focused {
		textCol = color.NRGBA{}
	}
	in.Ed.Layout(g, th.Shaper, th.Font, TextSize, colorMat(gtx, textCol), colorMat(gtx, ColSelect))
	if !focused && in.Ed.Len() > 0 {
		T(th, in.Ed.Text()).Layout(g)
	}
	st.Pop()
	return D{Size: r.Max}, changed, submitted
}

// Text is the box's text.
func (in *Input) Text() string { return in.Ed.Text() }

// NumInput is a number box showing the build's value. While the user types,
// the box keeps the text: the build is recomputed in the background, so the
// value can still be the old one for a frame. A value that changes to
// something the user did not type (the +/- buttons) replaces the text even
// with the box focused.
type NumInput struct {
	Input
	last int  // what the box shows or last sent
	seen int  // the build's value at the previous frame
	set  bool
}

// Layout shows v; it returns the typed value and true when it changed.
func (n *NumInput) Layout(gtx C, th *Theme, v int, width unit.Dp) (D, int, bool) {
	focused := gtx.Focused(&n.Ed)
	external := n.set && v != n.seen && v != n.last
	if !n.set || (!focused && n.last != v) || external {
		n.Ed.SetText(strconv.Itoa(v))
		if focused {
			n.Ed.SetCaret(n.Ed.Len(), n.Ed.Len())
		}
		n.last, n.set = v, true
	}
	n.seen = v
	n.Ed.Filter = "0123456789-"
	d, changed, submitted := n.Input.Layout(gtx, th, width)
	if changed || submitted {
		txt := strings.TrimSpace(n.Ed.Text())
		if txt == "" { // a cleared box is 0, and shows it
			txt = "0"
		}
		x, err := strconv.Atoi(txt)
		if err == nil {
			// "0" then a digit reads "05": show the number itself.
			if s := strconv.Itoa(x); s != n.Ed.Text() && txt != "-" {
				n.Ed.SetText(s)
				n.Ed.SetCaret(n.Ed.Len(), n.Ed.Len())
			}
			if x != v {
				n.last = x
				return d, x, true
			}
		}
	}
	return d, v, false
}

// Tabs are the .ro-tabs strip; Layout returns the clicked index.
type Tabs struct{ btns []widget.Clickable }

func (t *Tabs) Layout(gtx C, th *Theme, labels []string, active int) (D, int, bool) {
	if len(t.btns) < len(labels) {
		t.btns = append(t.btns, make([]widget.Clickable, len(labels)-len(t.btns))...)
	}
	picked, ok := active, false
	for i := range labels {
		if t.btns[i].Clicked(gtx) {
			picked, ok = i, true
		}
	}
	m := op.Record(gtx.Ops)
	d := Inset(3, 6, 0, 6).Layout(gtx, func(gtx C) D {
		return layout.Flex{}.Layout(gtx, flexMap(len(labels), func(i int) layout.FlexChild {
			return layout.Rigid(func(gtx C) D {
				return Inset(0, 1, 0, 0).Layout(gtx, func(gtx C) D {
					return t.btns[i].Layout(gtx, func(gtx C) D {
						tx := T(th, labels[i])
						if i == active {
							tx = tx.Bold()
						}
						mm := op.Record(gtx.Ops)
						td := Inset(1, 10, 2, 10).Layout(gtx, tx.W())
						call := mm.Stop()
						r := image.Rectangle{Max: td.Size}
						if i == active {
							FillRound(gtx, image.Rectangle{Max: image.Pt(r.Max.X, r.Max.Y+1)}, ColBody, 4, 4, 0, 0)
						} else {
							VGradient(gtx, r, rgb(0xeef1f6), rgb(0xc8d0de), 4, 4, 0, 0)
						}
						Border(gtx, image.Rectangle{Max: image.Pt(r.Max.X, r.Max.Y+gtx.Dp(4))}, ColBorder, 1, 4)
						call.Add(gtx.Ops)
						pointerCursor(gtx, r.Max)
						return D{Size: r.Max}
					})
				})
			})
		})...)
	})
	call := m.Stop()
	full := image.Rectangle{Max: image.Pt(gtx.Constraints.Max.X, d.Size.Y)}
	Fill(gtx, full, ColBodyAlt)
	call.Add(gtx.Ops)
	Fill(gtx, image.Rect(0, d.Size.Y-1, full.Max.X, d.Size.Y), ColBorder)
	return D{Size: full.Max}, picked, ok
}

func flexMap(n int, f func(i int) layout.FlexChild) []layout.FlexChild {
	out := make([]layout.FlexChild, n)
	for i := range out {
		out[i] = f(i)
	}
	return out
}

// Bar is the HP/SP/EXP bar (.ro-bar): pct 0..100 with a centered caption.
func Bar(gtx C, th *Theme, pct int, top, bottom color.NRGBA, caption string) D {
	w, h := gtx.Constraints.Max.X, gtx.Dp(12)
	r := image.Rectangle{Max: image.Pt(w, h)}
	FillRound(gtx, r, rgb(0x555555), 2, 2, 2, 2)
	fw := w * min(max(pct, 0), 100) / 100
	if fw > 0 {
		VGradient(gtx, image.Rect(1, 1, fw-1, h-1), top, bottom, 1, 1, 1, 1)
	}
	Border(gtx, r, rgb(0x333333), 1, 2)
	m := op.Record(gtx.Ops)
	g := gtx
	g.Constraints = layout.Constraints{Max: image.Pt(w, h)}
	td := T(th, caption).Sz(TinyText).C(ColWhite).Layout(g)
	call := m.Stop()
	st := op.Offset(image.Pt((w-td.Size.X)/2, (h-td.Size.Y)/2)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	st.Pop()
	return D{Size: r.Max}
}

// Bar colors.
var (
	BarHP  = [2]color.NRGBA{rgb(0x7fe07f), ColHP}
	BarSP  = [2]color.NRGBA{rgb(0x7f9fff), ColSP}
	BarEXP = [2]color.NRGBA{rgb(0xffe38a), ColEXP}
	BarAP  = [2]color.NRGBA{rgb(0xf5b07a), ColAP}
)

// Sep is a 1px separator line with margins (border-top: 1px solid #c4c8d0).
func Sep(gtx C) D {
	h := gtx.Dp(7)
	Fill(gtx, image.Rect(0, h/2, gtx.Constraints.Max.X, h/2+1), ColSep)
	return D{Size: image.Pt(gtx.Constraints.Max.X, h)}
}

// Row lays children horizontally, centered vertically, with a gap.
func Row(gap unit.Dp, ws ...layout.Widget) layout.Widget {
	return func(gtx C) D {
		var kids []layout.FlexChild
		for i, w := range ws {
			if i > 0 && gap > 0 {
				kids = append(kids, layout.Rigid(Gap(gap, 0)))
			}
			kids = append(kids, layout.Rigid(w))
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
	}
}

// Col lays children vertically with a gap.
func Col(gap unit.Dp, ws ...layout.Widget) layout.Widget {
	return func(gtx C) D {
		var kids []layout.FlexChild
		for i, w := range ws {
			if i > 0 && gap > 0 {
				kids = append(kids, layout.Rigid(Gap(0, gap)))
			}
			kids = append(kids, layout.Rigid(w))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
	}
}

// Fixed lays w in exactly width dp (a table cell).
func Fixed(width unit.Dp, w layout.Widget) layout.Widget {
	return func(gtx C) D {
		px := gtx.Dp(width)
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = px, px
		d := w(gtx)
		d.Size.X = px
		return d
	}
}

// RowTop is Row aligned at the top.
func RowTop(gap unit.Dp, ws ...layout.Widget) layout.Widget {
	return func(gtx C) D {
		var kids []layout.FlexChild
		for i, w := range ws {
			if i > 0 && gap > 0 {
				kids = append(kids, layout.Rigid(Gap(gap, 0)))
			}
			kids = append(kids, layout.Rigid(w))
		}
		return layout.Flex{Alignment: layout.Start}.Layout(gtx, kids...)
	}
}

// Caret is the ▾ of dropdowns (drawn: the interface font lacks it).
func Caret(gtx C, c color.NRGBA) D {
	s := gtx.Dp(9)
	var p clip.Path
	p.Begin(gtx.Ops)
	y := float32(s) * 0.35
	p.MoveTo(f32.Pt(float32(s)*0.15, y))
	p.LineTo(f32.Pt(float32(s)*0.85, y))
	p.LineTo(f32.Pt(float32(s)*0.5, y+float32(s)*0.4))
	p.Close()
	paint.FillShape(gtx.Ops, c, clip.Outline{Path: p.End()}.Op())
	return D{Size: image.Pt(s, s)}
}

// Gear is the settings icon (eight teeth around a ring), size dp.
func Gear(gtx C, c color.NRGBA, size unit.Dp) D {
	s := float32(gtx.Dp(size))
	cx, cy := s/2, s/2
	var p clip.Path
	p.Begin(gtx.Ops)
	for i := 0; i < 8; i++ {
		a := float64(i) * math.Pi / 4
		sin, cos := float32(math.Sin(a)), float32(math.Cos(a))
		// A tooth: a small rectangle from radius 0.3 to 0.5 of the size.
		w, r0, r1 := s*0.09, s*0.28, s*0.48
		pt := func(r, t float32) f32.Point { return f32.Pt(cx+cos*r-sin*t, cy+sin*r+cos*t) }
		p.MoveTo(pt(r0, -w))
		p.LineTo(pt(r1, -w))
		p.LineTo(pt(r1, w))
		p.LineTo(pt(r0, w))
		p.Close()
	}
	paint.FillShape(gtx.Ops, c, clip.Outline{Path: p.End()}.Op())
	ring := clip.Ellipse{Min: image.Pt(int(s*0.2), int(s*0.2)), Max: image.Pt(int(s*0.8), int(s*0.8))}
	var rp clip.Path
	rp.Begin(gtx.Ops)
	_ = ring
	const k = 0.55228475
	r := s * 0.26
	rp.MoveTo(f32.Pt(cx+r, cy))
	rp.CubeTo(f32.Pt(cx+r, cy+r*k), f32.Pt(cx+r*k, cy+r), f32.Pt(cx, cy+r))
	rp.CubeTo(f32.Pt(cx-r*k, cy+r), f32.Pt(cx-r, cy+r*k), f32.Pt(cx-r, cy))
	rp.CubeTo(f32.Pt(cx-r, cy-r*k), f32.Pt(cx-r*k, cy-r), f32.Pt(cx, cy-r))
	rp.CubeTo(f32.Pt(cx+r*k, cy-r), f32.Pt(cx+r, cy-r*k), f32.Pt(cx+r, cy))
	rp.Close()
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: rp.End(), Width: s * 0.16}.Op())
	return D{Size: image.Pt(int(s), int(s))}
}

// Pad is an inset around w, as a widget.
func Pad(in layout.Inset, w layout.Widget) layout.Widget {
	return func(gtx C) D { return in.Layout(gtx, w) }
}
