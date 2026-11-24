package ui

import (
	"image"
	"strings"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"

	"ragnarok-of-building/internal/gamedata"
)

// Popup is a box drawn above every window (a dropdown list, the settings
// menu): it closes on Esc or a click outside.
type Popup struct {
	At      image.Point // desk coordinates of its top-left corner
	Width   int         // px; 0 = its content's width
	Content layout.Widget
	OnClose func()
	catch   bool
}

// OpenPopup shows p (closing any other).
func (d *Desk) OpenPopup(p *Popup) {
	if d.popup != nil && d.popup != p && d.popup.OnClose != nil {
		d.popup.OnClose()
	}
	d.popup = p
}

// ClosePopup closes the open popup.
func (d *Desk) ClosePopup() {
	if d.popup != nil && d.popup.OnClose != nil {
		d.popup.OnClose()
	}
	d.popup = nil
}

// PopupOpen reports whether p is the open popup.
func (d *Desk) PopupOpen(p *Popup) bool { return d.popup == p }

// ShowTip sets this frame's tooltip, drawn next to the mouse.
func (d *Desk) ShowTip(w layout.Widget) { d.tip = w }

// Mouse is the pointer position on the desk.
func (d *Desk) Mouse() image.Point { return d.mouse }

func (d *Desk) layoutOverlay(gtx C, size image.Point) {
	if p := d.popup; p != nil {
		// A transparent catcher: a click outside closes the popup.
		for {
			ev, ok := gtx.Event(pointer.Filter{Target: &p.catch, Kinds: pointer.Press})
			if !ok {
				break
			}
			if _, ok := ev.(pointer.Event); ok {
				d.ClosePopup()
			}
		}
		for {
			ev, ok := gtx.Event(key.Filter{Name: key.NameEscape})
			if !ok {
				break
			}
			if e, ok := ev.(key.Event); ok && e.State == key.Press {
				d.ClosePopup()
			}
		}
		if d.popup == p {
			st := clip.Rect{Max: size}.Push(gtx.Ops)
			event.Op(gtx.Ops, &p.catch)
			st.Pop()
			m := op.Record(gtx.Ops)
			g := gtx
			g.Constraints = layout.Constraints{Max: image.Pt(size.X, size.Y)}
			if p.Width > 0 {
				g.Constraints.Min.X, g.Constraints.Max.X = p.Width, p.Width
			}
			pd := Box{Bg: ColPanel, Line: rgb(0x7f9db9)}.Layout(g, p.Content)
			call := m.Stop()
			at := p.At
			// Keep it on screen.
			if at.X+pd.Size.X > size.X {
				at.X = max(0, size.X-pd.Size.X)
			}
			if at.Y+pd.Size.Y > size.Y {
				at.Y = max(0, size.Y-pd.Size.Y)
			}
			off := op.Offset(at).Push(gtx.Ops)
			Shadow(gtx, image.Rectangle{Max: pd.Size}, 0)
			// Clicks on the popup itself don't reach the catcher. This
			// area goes under the content: above it, it would swallow the
			// clicks meant for the options.
			area := clip.Rect{Max: pd.Size}.Push(gtx.Ops)
			event.Op(gtx.Ops, p)
			area.Pop()
			call.Add(gtx.Ops)
			off.Pop()
		}
	}
	d.layoutTip(gtx, size)
}

// tallTip is a tooltip taller than the screen: it stays where it showed,
// the mouse can go onto it, and the wheel scrolls it.
type tallTip struct {
	on     bool
	w      layout.Widget // what it shows, while the mouse is on it
	at     image.Point
	h      int // its whole height
	rect   image.Rectangle
	scroll int
	until  time.Time // kept until then, for the mouse to reach it
	tag    bool
}

// tipGrace is how long a tall tooltip waits for the mouse.
const tipGrace = 350 * time.Millisecond

func (d *Desk) layoutTip(gtx C, size image.Point) {
	t := &d.tall
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &t.tag, Kinds: pointer.Scroll | pointer.Move | pointer.Press | pointer.Release | pointer.Enter | pointer.Leave,
			ScrollY: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20}})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok && e.Kind == pointer.Scroll {
			t.scroll += int(e.Scroll.Y)
		}
	}
	tip := d.tip
	if tip == nil && t.on {
		in := d.mouse.In(t.rect)
		if in || gtx.Now.Before(t.until) {
			tip = t.w
			if in {
				t.until = gtx.Now.Add(tipGrace)
			} else {
				gtx.Execute(op.InvalidateCmd{At: t.until})
			}
		}
	} else if tip != nil {
		t.w, t.until = tip, gtx.Now.Add(tipGrace)
	}
	if tip == nil {
		*t = tallTip{}
		return
	}
	m := op.Record(gtx.Ops)
	g := gtx
	g.Constraints = layout.Constraints{Max: image.Pt(gtx.Dp(340), 1<<20)}
	td := tip(g)
	call := m.Stop()
	pad := gtx.Dp(8)
	maxH := size.Y - 2*pad
	at := d.mouse.Add(image.Pt(gtx.Dp(14), gtx.Dp(16)))
	if at.X+td.Size.X > size.X {
		at.X = max(0, d.mouse.X-td.Size.X-pad)
	}
	if td.Size.Y <= maxH {
		t.on = false
		if at.Y+td.Size.Y > size.Y {
			at.Y = max(0, d.mouse.Y-td.Size.Y-pad)
		}
		off := op.Offset(at).Push(gtx.Ops)
		Shadow(gtx, image.Rectangle{Max: td.Size}, gtx.Dp(3))
		call.Add(gtx.Ops)
		off.Pop()
		return
	}
	if !t.on || t.h != td.Size.Y {
		t.on, t.h, t.scroll, t.at = true, td.Size.Y, 0, image.Pt(at.X, pad)
	}
	t.scroll = max(0, min(t.scroll, td.Size.Y-maxH))
	r := image.Rectangle{Max: image.Pt(td.Size.X, maxH)}
	t.rect = r.Add(t.at)
	off := op.Offset(t.at).Push(gtx.Ops)
	defer off.Pop()
	Shadow(gtx, r, gtx.Dp(3))
	cl := clip.Rect(r).Push(gtx.Ops)
	// Over what is behind, so nothing there takes the mouse.
	event.Op(gtx.Ops, &t.tag)
	in := op.Offset(image.Pt(0, -t.scroll)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	in.Pop()
	// The scroll bar.
	bw := gtx.Dp(5)
	th := max(gtx.Dp(24), maxH*maxH/td.Size.Y)
	y := (maxH - th) * t.scroll / (td.Size.Y - maxH)
	Fill(gtx, image.Rect(r.Max.X-bw, 0, r.Max.X, maxH), colScrollBg)
	Fill(gtx, image.Rect(r.Max.X-bw, y, r.Max.X, y+th), ColBorder)
	cl.Pop()
	Border(gtx, r, ColBorder, 1, gtx.Dp(3))
}

// TipBox is the tooltip frame (.ro-tooltip).
func TipBox(gtx C, w layout.Widget) D {
	return Box{Bg: ColTipBG, Line: ColBorder, Radius: 3, Pad: Inset(6, 8, 6, 8)}.Layout(gtx, w)
}

// Hoverable reports hovering over an area, for tooltips.
type Hoverable struct{ cl widget.Clickable }

// Layout lays w and returns whether the mouse is over it.
func (h *Hoverable) Layout(gtx C, w layout.Widget) (D, bool) {
	d := h.cl.Layout(gtx, w)
	return d, h.cl.Hovered()
}

// Select is a dropdown (.ro-select): a box with the current choice and a ▾
// that opens the list of options in a popup.
type Select struct {
	// Tip, when set, is the tooltip of an option (by value) shown while the
	// mouse is over it in the list, or over the box for the current one;
	// nil for none.
	Tip func(value string) layout.Widget
	// SearchText, when set, is more text of an option (by value) the
	// search looks in besides its label: an item's description.
	SearchText func(value string) string

	popup  Popup
	btn    widget.Clickable
	items  []widget.Clickable
	list   widget.List
	anchor Anchor

	// Search, in lists longer than selectSearchMin.
	q       Input
	wasOpen bool
	norm    []string // each option's label and Search, normalized
	shown   []int    // the options matching q
	enter   string   // the first match, picked with Enter
}

// selectSearchMin is the list length from which a dropdown gets a search
// box.
const selectSearchMin = 13

// SearchHint is the dropdowns' search box hint, in the interface language.
var SearchHint = "Buscar…"

// Anchor finds where a widget is on the desk: the desk sees the mouse in
// desk coordinates, the widget in its own; the difference is its origin.
type Anchor struct {
	tag    bool
	Origin image.Point
}

// Track registers the widget's area (size) without taking its events.
func (a *Anchor) Track(gtx C, d *Desk, size image.Point) {
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &a.tag, Kinds: pointer.Move | pointer.Press | pointer.Enter})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok {
			a.Origin = d.mouse.Sub(e.Position.Round())
		}
	}
	area := clip.Rect{Max: size}.Push(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &a.tag)
	pass.Pop()
	area.Pop()
}

// Option is one choice: value and label.
type Option struct {
	Value string
	Label string
}

// matches reports whether every word of the normalized query is in text.
func matches(text, q string) bool {
	for _, w := range strings.Fields(q) {
		if !strings.Contains(text, w) {
			return false
		}
	}
	return true
}

// Layout draws the select; it returns the chosen value and true when the
// user picked one.
func (s *Select) Layout(gtx C, th *Theme, d *Desk, opts []Option, cur string, width unit.Dp) (D, string, bool) {
	picked, ok := "", false
	if len(s.items) < len(opts) {
		s.items = append(s.items, make([]widget.Clickable, len(opts)-len(s.items))...)
	}
	open := d.PopupOpen(&s.popup)
	search := len(opts) >= selectSearchMin
	if open && !s.wasOpen && search {
		// Opening: an empty search with the focus, and the options' text.
		s.q.Ed.SetText("")
		gtx.Execute(key.FocusCmd{Tag: &s.q.Ed})
		s.norm = s.norm[:0]
		for _, o := range opts {
			txt := o.Label
			if s.SearchText != nil {
				txt += " " + s.SearchText(o.Value)
			}
			s.norm = append(s.norm, gamedata.Normalize(txt))
		}
	}
	s.wasOpen = open
	s.shown = s.shown[:0]
	q := ""
	if open && search {
		q = gamedata.Normalize(s.q.Text())
	}
	for i := range opts {
		if q == "" || (i < len(s.norm) && matches(s.norm[i], q)) {
			s.shown = append(s.shown, i)
		}
	}
	for i := range opts {
		if !d.PopupOpen(&s.popup) {
			break // the options are only on screen while the list is open
		}
		if s.items[i].Clicked(gtx) {
			picked, ok = opts[i].Value, true
			d.ClosePopup()
		}
	}
	label := ""
	for _, o := range opts {
		if o.Value == cur {
			label = o.Label
		}
	}
	if s.btn.Clicked(gtx) {
		if d.PopupOpen(&s.popup) {
			d.ClosePopup()
		} else {
			d.OpenPopup(&s.popup)
		}
	}
	w := gtx.Dp(width)
	if w == 0 {
		w = gtx.Constraints.Max.X
	}
	w = min(w, gtx.Constraints.Max.X)
	h := gtx.Dp(BtnH)
	dims := s.btn.Layout(gtx, func(gtx C) D {
		r := image.Rectangle{Max: image.Pt(w, h)}
		Fill(gtx, r, ColPanel)
		Border(gtx, r, ColBorder, 1, 2)
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(w-gtx.Dp(18), h)}
		st := op.Offset(image.Pt(gtx.Dp(4), (h-gtx.Sp(TextSize)*135/100)/2)).Push(gtx.Ops)
		T(th, label).Layout(g)
		st.Pop()
		st = op.Offset(image.Pt(w-gtx.Dp(13), (h-gtx.Dp(9))/2+1)).Push(gtx.Ops)
		Caret(gtx, ColTitleText)
		st.Pop()
		pointerCursor(gtx, r.Max)
		return D{Size: r.Max}
	})
	if s.Tip != nil && s.btn.Hovered() && !d.PopupOpen(&s.popup) {
		if w := s.Tip(cur); w != nil {
			d.ShowTip(w)
		}
	}
	// The popup opens under the box.
	s.anchor.Track(gtx, d, dims.Size)
	s.popup.Width = max(w, gtx.Dp(180))
	s.popup.At = s.anchor.Origin.Add(image.Pt(0, dims.Size.Y))
	s.list.Axis = layout.Vertical
	shown := s.shown
	s.popup.Content = func(gtx C) D {
		gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(300))
		list := func(gtx C) D {
			if len(shown) == 0 {
				return Inset(4, 6, 4, 6).Layout(gtx, T(th, "—").C(ColMuted).W())
			}
			return s.list.Layout(gtx, len(shown), func(gtx C, k int) D {
				i := shown[k]
				return s.items[i].Layout(gtx, func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					m := op.Record(gtx.Ops)
					td := Inset(2, 6, 2, 6).Layout(gtx, T(th, opts[i].Label).W())
					call := m.Stop()
					r := image.Rectangle{Max: image.Pt(gtx.Constraints.Max.X, td.Size.Y)}
					if opts[i].Value == cur {
						Fill(gtx, r, ColSelect)
					} else if s.items[i].Hovered() {
						Fill(gtx, r, ColHover)
					}
					call.Add(gtx.Ops)
					pointerCursor(gtx, r.Max)
					if s.Tip != nil && s.items[i].Hovered() {
						if w := s.Tip(opts[i].Value); w != nil {
							d.ShowTip(w)
						}
					}
					return D{Size: r.Max}
				})
			})
		}
		if !search {
			return list(gtx)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return Inset(3, 3, 3, 3).Layout(gtx, func(gtx C) D {
					s.q.Hint = SearchHint
					_, _, enter := s.q.Layout(gtx, th, 0)
					if enter && len(shown) > 0 {
						s.enter = opts[shown[0]].Value
					}
					return D{Size: image.Pt(gtx.Constraints.Max.X, gtx.Dp(InputH))}
				})
			}),
			layout.Rigid(list), // as tall as the matches, up to what is left
		)
	}
	if s.enter != "" {
		picked, ok = s.enter, true
		s.enter = ""
		d.ClosePopup()
	}
	return dims, picked, ok
}
