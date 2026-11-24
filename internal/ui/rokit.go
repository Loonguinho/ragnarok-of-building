package ui

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

// RO-client window parts, after the inventory and the Kafra storage:
// vertical icon tabs on the left, the thin blue scrollbar with arrows, a
// footer with a count, a search box and buttons, and a row of icon filters.

var (
	colTabOff     = rgb(0xdfe3ea)
	colTabOffB    = rgb(0xc9ced8)
	colScrollBg   = rgb(0xe9eef6)
	colThumbA     = rgb(0xa9c4ee)
	colThumbB     = rgb(0x5d86c9)
	colThumbLine  = rgb(0x4a6fa8)
	colArrow      = rgb(0x4a6fa8)
	colFilterOn   = rgb(0xffffff)
	colFilterOnB  = rgb(0xdbe6f8)
	colFilterOff  = rgb(0xe9ebef)
	colFilterOffB = rgb(0xd3d7de)
)

// SideTab is one tab of a SideTabs: an icon (an item icon path) and the
// label of its tooltip.
type SideTab struct {
	Icon  string
	Label string
}

// SideTabs are the inventory's vertical tabs on the left of a panel.
type SideTabs struct {
	Cur  int
	btns []widget.Clickable
}

const (
	sideTabW = unit.Dp(26)
	sideTabH = unit.Dp(30)
)

// Layout draws the tabs and content next to them, in a white panel the
// active tab opens into. It reports whether the tab changed this frame.
func (s *SideTabs) Layout(gtx C, a *App, tabs []SideTab, content layout.Widget) (D, bool) {
	if len(s.btns) != len(tabs) {
		s.btns = make([]widget.Clickable, len(tabs))
	}
	changed := false
	for i := range s.btns {
		if s.btns[i].Clicked(gtx) && s.Cur != i {
			s.Cur, changed = i, true
		}
	}
	s.Cur = min(max(s.Cur, 0), len(tabs)-1)
	tw, th := gtx.Dp(sideTabW), gtx.Dp(sideTabH)

	// Content first, to know the height.
	m := op.Record(gtx.Ops)
	g := gtx
	g.Constraints.Max.X -= tw
	g.Constraints.Min.X = g.Constraints.Max.X
	g.Constraints.Min.Y = max(g.Constraints.Min.Y, len(tabs)*th+gtx.Dp(6))
	cd := Box{Bg: ColPanel, Line: ColListBorder, Pad: Inset(3, 3, 3, 3)}.Layout(g, func(gtx C) D {
		gtx.Constraints.Min.Y = max(gtx.Constraints.Min.Y, len(tabs)*th)
		return content(gtx)
	})
	call := m.Stop()
	func() {
		st := op.Offset(image.Pt(tw-1, 0)).Push(gtx.Ops)
		defer st.Pop()
		call.Add(gtx.Ops)
	}()

	// Tabs: the inactive ones grey; the active one white, opening into the panel.
	for i, t := range tabs {
		y := gtx.Dp(3) + i*th
		r := image.Rect(0, y, tw, y+th-gtx.Dp(2))
		cl := &s.btns[i]
		func() {
			st := op.Offset(r.Min).Push(gtx.Ops)
			defer st.Pop()
			sz := r.Size()
			on := i == s.Cur
			rr := image.Rectangle{Max: sz}
			if on {
				FillRound(gtx, image.Rectangle{Max: image.Pt(sz.X+1, sz.Y)}, ColPanel, gtx.Dp(4), 0, 0, gtx.Dp(4))
				Border(gtx, image.Rectangle{Max: image.Pt(sz.X+gtx.Dp(4), sz.Y)}, ColListBorder, 1, gtx.Dp(4))
				Fill(gtx, image.Rect(sz.X-1, 1, sz.X+gtx.Dp(4), sz.Y-1), ColPanel)
			} else {
				top, bot := colTabOff, colTabOffB
				if cl.Hovered() {
					top, bot = ColHover, colTabOff
				}
				VGradient(gtx, image.Rect(gtx.Dp(2), 0, sz.X, sz.Y), top, bot, gtx.Dp(4), 0, 0, gtx.Dp(4))
				Border(gtx, image.Rect(gtx.Dp(2), 0, sz.X+gtx.Dp(4), sz.Y), ColListBorder, 1, gtx.Dp(4))
			}
			g := gtx
			g.Constraints = layout.Exact(sz)
			cl.Layout(g, func(gtx C) D {
				return layout.Center.Layout(gtx, func(gtx C) D { return a.Icons.Icon(gtx, t.Icon, 20) })
			})
			pointerCursor(gtx, sz)
			_ = rr
		}()
		if cl.Hovered() && t.Label != "" {
			lbl := t.Label
			a.Desk.ShowTip(func(gtx C) D { return TipBox(gtx, T(a.Th, lbl).W()) })
		}
	}
	h := max(cd.Size.Y, len(tabs)*th+gtx.Dp(6))
	return D{Size: image.Pt(tw-1+cd.Size.X, h)}, changed
}

// ScrollList is a vertical list with the RO scrollbar: a pale track, a
// rounded blue thumb and arrow buttons at both ends.
type ScrollList struct {
	List     widget.List
	up, down widget.Clickable
}

const scrollW = unit.Dp(12)

// Layout fills the constraints' height with the list and its bar.
func (s *ScrollList) Layout(gtx C, n int, el layout.ListElement) D {
	s.List.Axis = layout.Vertical
	bw := gtx.Dp(scrollW)
	if s.up.Clicked(gtx) {
		s.List.ScrollBy(-1)
	}
	if s.down.Clicked(gtx) {
		s.List.ScrollBy(1)
	}
	full := gtx.Constraints.Max
	g := gtx
	g.Constraints.Max.X = max(0, full.X-bw-gtx.Dp(2))
	g.Constraints.Min.X = g.Constraints.Max.X
	ld := s.List.List.Layout(g, n, el)
	h := max(ld.Size.Y, gtx.Constraints.Min.Y)
	if gtx.Constraints.Max.Y < 1<<20 {
		h = gtx.Constraints.Max.Y
	}

	// The bar.
	x0 := full.X - bw
	st := op.Offset(image.Pt(x0, 0)).Push(gtx.Ops)
	s.bar(gtx, bw, h, n)
	st.Pop()
	if d := s.List.ScrollDistance(); d != 0 {
		s.List.List.ScrollBy(d * float32(n))
	}
	return D{Size: image.Pt(full.X, h)}
}

func (s *ScrollList) bar(gtx C, w, h, n int) {
	arrow := w
	// Arrows.
	s.arrowBtn(gtx, &s.up, image.Rect(0, 0, w, arrow), true)
	s.arrowBtn(gtx, &s.down, image.Rect(0, h-arrow, w, h), false)
	// Track.
	track := image.Rect(0, arrow, w, h-arrow)
	if track.Dy() <= 0 {
		return
	}
	Fill(gtx, track, colScrollBg)
	Fill(gtx, image.Rect(0, track.Min.Y, 1, track.Max.Y), ColListBorder)
	Fill(gtx, image.Rect(w-1, track.Min.Y, w, track.Max.Y), ColListBorder)
	start, end := listView(s.List.Position, n)
	g := gtx
	g.Constraints = layout.Exact(image.Pt(w, track.Dy()))
	so := op.Offset(track.Min).Push(gtx.Ops)
	defer so.Pop()
	s.List.Scrollbar.Update(g, layout.Vertical, start, end)
	area := clip.Rect(image.Rectangle{Max: track.Size()}).Push(gtx.Ops)
	s.List.Scrollbar.AddDrag(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	s.List.Scrollbar.AddTrack(gtx.Ops)
	pass.Pop()
	area.Pop()
	if start <= 0 && end >= 1 {
		return // all in view: no thumb
	}
	tl := track.Dy()
	y0 := int(math.Round(float64(start) * float64(tl)))
	y1 := int(math.Round(float64(end) * float64(tl)))
	if y1-y0 < gtx.Dp(14) {
		y1 = min(tl, y0+gtx.Dp(14))
		y0 = y1 - gtx.Dp(14)
	}
	thumb := image.Rect(1, y0, w-1, y1)
	rad := gtx.Dp(4)
	top, bot := colThumbA, colThumbB
	if s.List.Scrollbar.IndicatorHovered() || s.List.Scrollbar.Dragging() {
		top, bot = lighten(colThumbA), colThumbA
	}
	VGradient(gtx, thumb, top, bot, rad, rad, rad, rad)
	Border(gtx, thumb, colThumbLine, 1, rad)
	ta := clip.Rect(thumb).Push(gtx.Ops)
	pass = pointer.PassOp{}.Push(gtx.Ops)
	s.List.Scrollbar.AddIndicator(gtx.Ops)
	pass.Pop()
	ta.Pop()
}

func (s *ScrollList) arrowBtn(gtx C, cl *widget.Clickable, r image.Rectangle, up bool) {
	st := op.Offset(r.Min).Push(gtx.Ops)
	defer st.Pop()
	sz := r.Size()
	g := gtx
	g.Constraints = layout.Exact(sz)
	cl.Layout(g, func(gtx C) D {
		top, bot := ColBtnA, ColBtnB
		if cl.Hovered() {
			top = ColHover
		}
		VGradient(gtx, image.Rectangle{Max: sz}, top, bot, gtx.Dp(2), gtx.Dp(2), gtx.Dp(2), gtx.Dp(2))
		Border(gtx, image.Rectangle{Max: sz}, ColListBorder, 1, gtx.Dp(2))
		triangle(gtx, sz, up, colArrow)
		return D{Size: sz}
	})
}

// triangle draws a small arrow head centred in a box.
func triangle(gtx C, sz image.Point, up bool, c color.NRGBA) {
	cx, cy := float32(sz.X)/2, float32(sz.Y)/2
	hw := float32(sz.X) * 0.28
	hh := float32(sz.Y) * 0.18
	var p clip.Path
	p.Begin(gtx.Ops)
	if up {
		p.MoveTo(f32.Pt(cx-hw, cy+hh))
		p.LineTo(f32.Pt(cx+hw, cy+hh))
		p.LineTo(f32.Pt(cx, cy-hh))
	} else {
		p.MoveTo(f32.Pt(cx-hw, cy-hh))
		p.LineTo(f32.Pt(cx+hw, cy-hh))
		p.LineTo(f32.Pt(cx, cy+hh))
	}
	p.Close()
	paint.FillShape(gtx.Ops, c, clip.Outline{Path: p.End()}.Op())
}

// listView is the part of a list in view, 0-1 (as Gio's material list
// estimates it from the laid-out elements).
func listView(lp layout.Position, n int) (start, end float32) {
	if n <= 0 || lp.Length <= 0 {
		return 0, 1
	}
	elem := float32(lp.Length) / float32(n)
	clamp := func(v float32) float32 { return max(0, min(1, v)) }
	start = clamp((float32(lp.First)*elem + float32(lp.Offset)) / float32(lp.Length))
	end = clamp((float32(lp.First+lp.Count)*elem + float32(lp.OffsetLast)) / float32(lp.Length))
	return start, end
}

// Magnifier draws the search glass of the RO search boxes.
func Magnifier(gtx C, size unit.Dp, c color.NRGBA) D {
	px := gtx.Dp(size)
	r := float32(px) * 0.3
	cx, cy := float32(px)*0.42, float32(px)*0.42
	w := max(1.2, float32(px)*0.11)
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: circlePath(gtx, cx, cy, r), Width: w}.Op())
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(cx+r*0.7, cy+r*0.7))
	p.LineTo(f32.Pt(float32(px)*0.92, float32(px)*0.92))
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: p.End(), Width: w * 1.4}.Op())
	return D{Size: image.Pt(px, px)}
}

func circlePath(gtx C, cx, cy, r float32) clip.PathSpec {
	var p clip.Path
	p.Begin(gtx.Ops)
	const n = 24
	for i := 0; i <= n; i++ {
		a := float64(i) / n * 2 * math.Pi
		pt := f32.Pt(cx+r*float32(math.Cos(a)), cy+r*float32(math.Sin(a)))
		if i == 0 {
			p.MoveTo(pt)
		} else {
			p.LineTo(pt)
		}
	}
	return p.End()
}

// SearchBox is the storage's search field with the glass on its right.
func SearchBox(gtx C, th *Theme, in *Input, hint string, width unit.Dp) (D, bool) {
	in.Hint = hint
	field := func(gtx C) D { d, _, _ := in.Layout(gtx, th, width); return d }
	glass := layout.Rigid(func(gtx C) D {
		return layout.Inset{Left: 3}.Layout(gtx, func(gtx C) D { return Magnifier(gtx, 13, colArrow) })
	})
	if width == 0 { // fill the row
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, layout.Flexed(1, field), glass), false
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, layout.Rigid(field), glass), false
}

// IconFilter is a row of icon toggles (the storage's type filters); the
// one picked is On (-1 = none, all shown). Clicking the picked one clears it.
type IconFilter struct {
	On   int
	btns []widget.Clickable
}

// Layout draws the toggles; it reports whether the pick changed.
func (f *IconFilter) Layout(gtx C, a *App, tabs []SideTab) (D, bool) {
	if len(f.btns) != len(tabs) {
		f.btns = make([]widget.Clickable, len(tabs))
		f.On = -1
	}
	changed := false
	for i := range f.btns {
		if f.btns[i].Clicked(gtx) {
			if f.On == i {
				f.On = -1
			} else {
				f.On = i
			}
			changed = true
		}
	}
	sz := gtx.Dp(22)
	var ws []layout.Widget
	for i, t := range tabs {
		i, t := i, t
		ws = append(ws, func(gtx C) D {
			cl := &f.btns[i]
			g := gtx
			g.Constraints = layout.Exact(image.Pt(sz, sz))
			d := cl.Layout(g, func(gtx C) D {
				r := image.Rectangle{Max: image.Pt(sz, sz)}
				top, bot := colFilterOff, colFilterOffB
				if f.On == i {
					top, bot = colFilterOn, colFilterOnB
				} else if cl.Hovered() {
					top = ColHover
				}
				VGradient(gtx, r, top, bot, gtx.Dp(3), gtx.Dp(3), gtx.Dp(3), gtx.Dp(3))
				line := ColListBorder
				if f.On == i {
					line = colThumbLine
				}
				Border(gtx, r, line, 1, gtx.Dp(3))
				return layout.Center.Layout(gtx, func(gtx C) D { return a.Icons.Icon(gtx, t.Icon, 18) })
			})
			pointerCursor(gtx, d.Size)
			if cl.Hovered() && t.Label != "" {
				lbl := t.Label
				a.Desk.ShowTip(func(gtx C) D { return TipBox(gtx, T(a.Th, lbl).W()) })
			}
			return d
		})
	}
	return Row(2, ws...)(gtx), changed
}
