package ui

import (
	"image"
	"slices"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
)

// Desk holds the windows (rowindow.js): fixed columns by default, or
// floating (Alt+W) where they are dragged freely. Popups (dropdowns,
// menus) and tooltips are drawn above everything.
type Desk struct {
	Windows  []*Window
	Floating bool
	ColW     [3]unit.Dp // fixed layout column widths (360, 420, 560)
	// Below is drawn under the first column (the chat log); floating, at
	// the bottom left.
	Below layout.Widget

	z       int
	scroll  widget.List
	changed bool

	popup    *Popup        // the open dropdown/menu (one at a time)
	tip      layout.Widget // tooltip of this frame
	mouse    image.Point
	mouseTag bool
}

// NewDesk makes an empty desk with the web page's column widths.
func NewDesk() *Desk {
	d := &Desk{ColW: [3]unit.Dp{360, 420, 560}}
	d.scroll.Axis = layout.Vertical
	return d
}

// Add appends a window.
func (d *Desk) Add(w *Window) *Window { d.Windows = append(d.Windows, w); return w }

// Find returns the window with an ID.
func (d *Desk) Find(id string) *Window {
	for _, w := range d.Windows {
		if w.ID == id {
			return w
		}
	}
	return nil
}

// Changed reports (and clears) whether the layout changed since the last
// call: windows moved, opened or closed (time to save the state).
func (d *Desk) Changed() bool { c := d.changed; d.changed = false; return c }

// Raise brings a window to the front.
func (d *Desk) Raise(w *Window) {
	d.z++
	w.z = d.z
}

func (d *Desk) top() *Window {
	var t *Window
	for _, w := range d.Windows {
		if w.Open && (t == nil || w.z > t.z) {
			t = w
		}
	}
	return t
}

// Toggle is the client's window toggle: a closed window opens; an open one
// covered by another comes to the front; the front one closes. In the
// fixed layout an open window closes.
func (d *Desk) Toggle(w *Window) {
	d.changed = true
	switch {
	case !w.Open:
		w.Open = true
		d.Raise(w)
	case d.Floating && d.top() != w:
		d.Raise(w)
	default:
		w.Open = false
	}
}

// ShowAll opens every window (except NoShowAll ones).
func (d *Desk) ShowAll() {
	for _, w := range d.Windows {
		if !w.NoShowAll {
			w.Open = true
		}
	}
	d.changed = true
}

// ResetLayout forgets the floating positions (they cascade again).
func (d *Desk) ResetLayout() {
	for _, w := range d.Windows {
		w.Placed = false
	}
	d.changed = true
}

// Layout draws the backdrop and the windows, then popups and the tooltip.
func (d *Desk) Layout(gtx C, th *Theme) D {
	size := gtx.Constraints.Max
	backdrop(gtx, size)
	// Track the mouse for tooltips.
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &d.mouseTag, Kinds: pointer.Move | pointer.Drag | pointer.Enter})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok {
			d.mouse = e.Position.Round()
		}
	}
	d.tip = nil
	if d.Floating {
		d.layoutFloating(gtx, th, size)
	} else {
		d.layoutFixed(gtx, th)
	}
	d.layoutOverlay(gtx, size)
	// On top of everything, passing events through: the mouse position
	// for tooltips.
	area := clip.Rect{Max: size}.Push(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &d.mouseTag)
	pass.Pop()
	area.Pop()
	return D{Size: size}
}

func (d *Desk) layoutFixed(gtx C, th *Theme) {
	pad := gtx.Dp(16)
	gapPx := gtx.Dp(12)
	cols := [3][]*Window{}
	for _, w := range d.Windows {
		if w.Open {
			c := min(max(w.Col, 0), 2)
			cols[c] = append(cols[c], w)
		}
	}
	d.scroll.Layout(gtx, 1, func(gtx C, _ int) D {
		x := pad
		bottom := 0
		for ci := range cols {
			cw := gtx.Dp(d.ColW[ci])
			y := pad
			for _, w := range cols[ci] {
				g := gtx
				g.Constraints = layout.Constraints{Max: image.Pt(cw, 1<<20)}
				st := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
				wd := w.layout(g, th, d, false)
				st.Pop()
				w.rect = image.Rectangle{Min: image.Pt(x, y), Max: image.Pt(x+cw, y+wd.Size.Y)}
				y += wd.Size.Y + gapPx
			}
			if ci == 0 && d.Below != nil {
				g := gtx
				g.Constraints = layout.Constraints{Max: image.Pt(cw, gtx.Dp(140))}
				st := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
				bd := d.Below(g)
				st.Pop()
				y += bd.Size.Y + gapPx
			}
			bottom = max(bottom, y)
			x += cw + gapPx
		}
		return D{Size: image.Pt(gtx.Constraints.Max.X, bottom+gtx.Dp(140))}
	})
}

func (d *Desk) layoutFloating(gtx C, th *Theme, size image.Point) {
	ws := slices.Clone(d.Windows)
	slices.SortStableFunc(ws, func(a, b *Window) int { return a.z - b.z })
	cascade := 0
	for _, w := range ws {
		if !w.Open {
			continue
		}
		if !w.Placed {
			// Start where the fixed layout had it, else cascade.
			if !w.rect.Empty() {
				w.Pos = w.rect.Min
			} else {
				w.Pos = image.Pt(gtx.Dp(40)+cascade, gtx.Dp(40)+cascade)
				cascade += gtx.Dp(24)
			}
			w.Placed = true
		}
		cw := gtx.Dp(d.ColW[min(max(w.Col, 0), 2)])
		if w.Width > 0 {
			cw = gtx.Dp(w.Width)
		}
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(cw, 1<<20)}
		st := op.Offset(w.Pos).Push(gtx.Ops)
		wd := w.layout(g, th, d, true)
		st.Pop()
		w.rect = image.Rectangle{Min: w.Pos, Max: w.Pos.Add(wd.Size)}
	}
	if d.Below != nil {
		m := op.Record(gtx.Ops)
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(gtx.Dp(360), gtx.Dp(140))}
		bd := d.Below(g)
		call := m.Stop()
		st := op.Offset(image.Pt(gtx.Dp(8), size.Y-bd.Size.Y-gtx.Dp(8))).Push(gtx.Ops)
		call.Add(gtx.Ops)
		st.Pop()
	}
}

// Shortcut is an Alt+letter filter for the window hotkeys.
func (d *Desk) Hotkeys(gtx C) {
	for _, w := range d.Windows {
		if w.Hotkey == "" {
			continue
		}
		for {
			ev, ok := gtx.Event(key.Filter{Name: key.Name(w.Hotkey), Required: key.ModAlt})
			if !ok {
				break
			}
			if e, ok := ev.(key.Event); ok && e.State == key.Press {
				d.Toggle(w)
			}
		}
	}
}

// backdrop is the "field map" sky-to-grass gradient.
func backdrop(gtx C, size image.Point) {
	mid := size.Y * 45 / 100
	VGradient(gtx, image.Rect(0, 0, size.X, mid), ColSkyA, ColSkyB, 0, 0, 0, 0)
	VGradient(gtx, image.Rect(0, mid, size.X, size.Y), ColSkyB, ColSkyC, 0, 0, 0, 0)
}
