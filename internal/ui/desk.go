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
	ColW     [deskCols]unit.Dp // fixed layout column widths
	// Below is drawn under the first column (the chat log); floating, at
	// the bottom left.
	Below layout.Widget
	// Main, when set, is drawn instead of the columns (the single window);
	// Float windows stay above it.
	Main layout.Widget

	z       int
	scroll  widget.List
	changed bool

	popup    *Popup        // the open dropdown/menu (one at a time)
	tip      layout.Widget // tooltip of this frame
	mouse    image.Point
	mouseTag bool
}

// deskCols is how many columns the fixed layout has; with these widths
// they fit a 1920 px wide screen.
const deskCols = 4

// NewDesk makes an empty desk.
func NewDesk() *Desk {
	d := &Desk{ColW: [deskCols]unit.Dp{390, 430, 600, 380}}
	d.scroll.Axis = layout.Vertical
	return d
}

// Add appends a window.
func (d *Desk) Add(w *Window) *Window { d.Windows = append(d.Windows, w); return w }

// Group makes members the side tabs of g (added to the desk too, so their
// hotkeys and saved state keep working); the first is the current one.
func (d *Desk) Group(g *Window, members ...*Window) *Window {
	d.Add(g)
	g.Members = members
	for i, m := range members {
		m.parent = g
		m.Open = i == 0 && g.Open // the first tab shows (saved state may pick another)
	}
	if len(members) > 0 {
		g.cur = members[0]
	}
	return g
}

// Current is the tab a group shows.
func (w *Window) Current() *Window { return w.cur }

// selectTab shows member m in its group.
func (d *Desk) selectTab(m *Window) {
	g := m.parent
	if g.cur != m {
		g.cur = m
		d.changed = true
	}
	for i, x := range g.Members {
		x.Open = x == m && g.Open
		if x == m {
			g.side.Cur = i
		}
	}
}

// syncGroups keeps each group and its members agreeing: a member opened
// from elsewhere (its Open set) opens the group on it, a group closed
// closes its members, and a tab picked in the group becomes the member shown.
func (d *Desk) syncGroups() {
	for _, g := range d.Windows {
		if len(g.Members) == 0 {
			continue
		}
		if g.side.Cur >= 0 && g.side.Cur < len(g.Members) && g.Members[g.side.Cur] != g.cur && g.Open {
			d.selectTab(g.Members[g.side.Cur])
		}
		for _, m := range g.Members {
			if m.Open && (m != g.cur || !g.Open) {
				g.Open = true
				d.selectTab(m)
				d.changed = true
			}
		}
		if g.Open && g.cur != nil && !g.cur.Open {
			// The member was closed from its own body (a Fechar button).
			if g.wasShowing {
				g.Open = false
				d.changed = true
			}
		}
		for _, m := range g.Members {
			m.Open = g.Open && m == g.cur
		}
		g.wasShowing = g.Open
	}
}

// autoClose closes the AutoClose windows a press at p is outside of
// (unless it lands in another floating window or a popup is open).
func (d *Desk) autoClose(p image.Point) {
	if d.popup != nil {
		return
	}
	for _, w := range d.Windows {
		if w.Open && (w.Float || d.Floating) && p.In(w.rect) && !w.AutoClose {
			return
		}
	}
	for _, w := range d.Windows {
		if w.AutoClose && w.Open && !p.In(w.rect) {
			w.Open = false
			d.changed = true
		}
	}
}

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
	if w.parent != nil {
		d.selectTab(w)
		w = w.parent
	}
	d.z++
	w.z = d.z
}

func (d *Desk) top() *Window {
	var t *Window
	for _, w := range d.Windows {
		if w.Open && w.parent == nil && (t == nil || w.z > t.z) {
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
	if g := w.parent; g != nil {
		// A member: open its group on it; the tab shown closes the group.
		switch {
		case !g.Open:
			g.Open = true
			d.selectTab(w)
			d.Raise(g)
		case g.cur != w:
			d.selectTab(w)
			d.Raise(g)
		case d.Floating && d.top() != g:
			d.Raise(g)
		default:
			g.Open = false
			d.selectTab(w)
		}
		return
	}
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
		if !w.NoShowAll && w.parent == nil {
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
		ev, ok := gtx.Event(pointer.Filter{Target: &d.mouseTag, Kinds: pointer.Move | pointer.Drag | pointer.Enter | pointer.Press})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok {
			d.mouse = e.Position.Round()
			if e.Kind == pointer.Press {
				d.autoClose(d.mouse)
			}
		}
	}
	d.tip = nil
	d.syncGroups()
	if d.Floating {
		d.layoutFloating(gtx, th, size)
	} else if d.Main != nil {
		g := gtx
		g.Constraints = layout.Exact(size)
		d.Main(g)
		var floats []*Window
		for _, w := range d.Windows {
			if w.Open && w.parent == nil && w.Float {
				floats = append(floats, w)
			}
		}
		d.layoutFloats(gtx, th, size, floats)
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
	cols := [deskCols][]*Window{}
	var floats []*Window
	// Columns that do not fit the window's width (a smaller screen, display
	// scaling) narrow a little; if that is not enough, the last column joins
	// the one before, so no window opens off screen.
	fit := deskCols
	var colW [deskCols]int
	for {
		need := 2*pad + gapPx*(fit-1)
		for i := range fit {
			need += gtx.Dp(d.ColW[i])
		}
		scale := min(1, float32(gtx.Constraints.Max.X)/float32(need))
		if scale >= 0.85 || fit == 1 {
			for i := range fit {
				colW[i] = int(float32(gtx.Dp(d.ColW[i])) * scale)
			}
			break
		}
		fit--
	}
	for _, w := range d.Windows {
		if w.Open && w.parent == nil && w.Float {
			floats = append(floats, w)
			continue
		}
		if w.Open && w.parent == nil {
			c := min(max(w.Col, 0), fit-1)
			cols[c] = append(cols[c], w)
		}
	}
	viewH := gtx.Constraints.Max.Y
	d.scroll.Layout(gtx, 1, func(gtx C, _ int) D {
		x := pad
		bottom := 0
		for ci := range cols {
			if ci >= fit {
				break
			}
			cw := colW[ci]
			y := pad
			// Share the column's height out: windows that fit keep their
			// size, the tall ones split the rest and scroll inside.
			avail := viewH - 2*pad - gapPx*max(0, len(cols[ci])-1)
			if ci == 0 && d.Below != nil {
				avail -= gtx.Dp(140) + gapPx
			}
			rooms := shareHeight(cols[ci], max(avail, gtx.Dp(160)*len(cols[ci])), gtx.Dp(160))
			for i, w := range cols[ci] {
				room := rooms[i]
				g := gtx
				g.Constraints = layout.Constraints{Max: image.Pt(cw, room)}
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
	d.layoutFloats(gtx, th, gtx.Constraints.Max, floats)
}

// shareHeight splits a column's height among its windows: a window that
// wants less than an equal share keeps its size (and FullHeight or
// minimized ones always do); the rest split what is left evenly. The first
// frame, before a window was measured, it gets an equal share.
func shareHeight(ws []*Window, avail, minH int) []int {
	rooms := make([]int, len(ws))
	fixed := make([]bool, len(ws))
	left, open := avail, len(ws)
	for i, w := range ws {
		if w.FullHeight || w.Min {
			rooms[i], fixed[i] = 1<<20, true
			left -= w.natural
			open--
		}
	}
	for changed := true; changed && open > 0; {
		changed = false
		share := left / open
		for i, w := range ws {
			if !fixed[i] && w.natural > 0 && w.natural <= share {
				rooms[i], fixed[i] = 1<<20, true
				left -= w.natural
				open--
				changed = true
			}
		}
	}
	if open > 0 {
		share := max(left/open, minH)
		for i := range ws {
			if !fixed[i] {
				rooms[i] = share
			}
		}
	}
	return rooms
}

// layoutFloats draws floating windows over the columns, front one last; a
// window not placed yet starts in the middle of the screen.
func (d *Desk) layoutFloats(gtx C, th *Theme, size image.Point, ws []*Window) {
	slices.SortStableFunc(ws, func(a, b *Window) int { return a.z - b.z })
	for _, w := range ws {
		cw := gtx.Dp(w.Width)
		if cw == 0 {
			cw = gtx.Dp(d.ColW[min(max(w.Col, 0), deskCols-1)])
		}
		cw = min(cw, size.X)
		if !w.Placed {
			w.Pos = image.Pt(max(0, (size.X-cw)/2), gtx.Dp(60))
			w.Placed = true
		}
		w.Pos.X = max(0, min(w.Pos.X, size.X-cw))
		d.layoutFloat(gtx, th, size, w, cw)
	}
}

// layoutFloat draws a floating window at its position. Its height does not
// depend on where it is (it would change while dragged), except a
// dropdown's, which ends at the bottom of the screen; the title bar always
// stays in view.
func (d *Desk) layoutFloat(gtx C, th *Theme, size image.Point, w *Window, cw int) {
	w.Pos.Y = max(0, min(w.Pos.Y, size.Y-gtx.Dp(TitleH)-gtx.Dp(8)))
	room := size.Y - gtx.Dp(16)
	if w.Dropdown {
		room = size.Y - w.Pos.Y - gtx.Dp(8)
	}
	g := gtx
	g.Constraints = layout.Constraints{Max: image.Pt(cw, max(room, gtx.Dp(160)))}
	at := w.Pos // layout moves w.Pos when dragged: for the next frame
	st := op.Offset(at).Push(gtx.Ops)
	wd := w.layout(g, th, d, true)
	st.Pop()
	w.drawn = at
	w.rect = image.Rectangle{Min: at, Max: at.Add(wd.Size)}
}

func (d *Desk) layoutFloating(gtx C, th *Theme, size image.Point) {
	ws := slices.Clone(d.Windows)
	slices.SortStableFunc(ws, func(a, b *Window) int { return a.z - b.z })
	cascade := 0
	for _, w := range ws {
		if !w.Open || w.parent != nil {
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
		cw := gtx.Dp(d.ColW[min(max(w.Col, 0), deskCols-1)])
		if w.Width > 0 {
			cw = gtx.Dp(w.Width)
		}
		d.layoutFloat(gtx, th, size, w, cw)
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
