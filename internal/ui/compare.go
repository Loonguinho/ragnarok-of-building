package ui

import (
	"fmt"
	"image"
	"reflect"
	"slices"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/view"
)

// ---- Comparação no equipamento (Comparação v2) ----
//
// Any slot of the equipment window can open a comparison slot under it
// (↔). What the comparison slots hold goes on a copy of the build, all of
// them at once (so sets count), and the Resumo shows the build's damage
// next to that copy's. The comparison is kept as what differs from the
// build, slot by slot, so the build's other changes (stats, buffs, the
// target) count on both sides. It is not saved with the build.

type cmpState struct {
	open []string               // slots in comparison, in the order opened
	over map[string]*calc.Equip // what the comparison has there (nil: empty)

	btn          map[string]*widget.Clickable // ↔ of each slot
	pick, remove map[string]*widget.Clickable // a comparison slot, its ×
	apply, clear widget.Clickable
	at           map[string]*Anchor // where each comparison slot is (the editor drops under it)

	// The comparison computed (guarded by App.mu).
	seq   int
	key   string // build + comparison it was computed for
	view  view.BuildView
	ready bool
	dirty bool // the build or the comparison changed since
}

func clickable(m *map[string]*widget.Clickable, key string) *widget.Clickable {
	if *m == nil {
		*m = map[string]*widget.Clickable{}
	}
	c, ok := (*m)[key]
	if !ok {
		c = new(widget.Clickable)
		(*m)[key] = c
	}
	return c
}

// comparing reports whether a slot is in comparison.
func (a *App) comparing(slot string) bool { return slices.Contains(a.ecmp.open, slot) }

// compareBuild is the build with the comparison's pieces on.
func (a *App) compareBuild() calc.Build {
	return a.V.Overlay(a.Build(), a.ecmp.over)
}

// openCompare puts a slot in comparison and opens its piece in the slot
// editor.
func (a *App) openCompare(slot string) {
	if !a.comparing(slot) {
		a.ecmp.open = append(a.ecmp.open, slot)
	}
	a.markCompare()
	a.openSlot(slot)
	a.slot.cmp = true
	a.search(0)
}

// closeCompare takes a slot out of the comparison (with what its piece
// changed elsewhere: the shield a two-handed weapon left out).
func (a *App) closeCompare(slot string) {
	s := &a.ecmp
	s.open = slices.DeleteFunc(s.open, func(k string) bool { return k == slot })
	if len(s.open) == 0 {
		a.clearCompare()
		return
	}
	delete(s.over, slot)
	for k, e := range s.over { // emptied by the piece taken out (a shield)
		if e == nil && !a.comparing(k) {
			delete(s.over, k)
		}
	}
	if a.slot.cmp && a.slot.key == slot {
		a.slot.cmp = false
		a.equipDone()
	}
	a.markCompare()
}

// clearCompare ends the comparison.
func (a *App) clearCompare() {
	s := &a.ecmp
	s.open, s.over = nil, nil
	if a.slot.cmp {
		a.slot.cmp = false
		a.equipDone()
	}
	a.mu.Lock()
	s.ready, s.key = false, ""
	a.mu.Unlock()
}

// applyCompare equips the comparison (Ctrl+Z undoes it).
func (a *App) applyCompare() {
	nb := a.compareBuild()
	a.clearCompare()
	a.SetBuild(nb)
	a.Say(a.T("Comparação aplicada."), false)
}

// setCompare takes a build the slot editor made from the comparison and
// keeps what differs from the build, slot by slot.
func (a *App) setCompare(nb calc.Build) {
	base := a.V.Overlay(a.Build(), nil)
	over := map[string]*calc.Equip{}
	for _, slot := range calc.Slots {
		e, ok := nb.Equip[slot]
		be, bok := base.Equip[slot]
		if ok == bok && (!ok || reflect.DeepEqual(e, be)) {
			continue
		}
		if !ok {
			over[slot] = nil
			continue
		}
		c := e
		over[slot] = &c
	}
	a.ecmp.over = over
	a.markCompare()
}

// markCompare asks for the comparison to be computed again.
func (a *App) markCompare() {
	a.mu.Lock()
	a.ecmp.dirty = true
	a.mu.Unlock()
	if a.Invalidate != nil {
		a.Invalidate()
	}
}

// refreshCompare computes the comparison in the background when the build
// or the comparison changed (called by each frame).
func (a *App) refreshCompare() {
	s := &a.ecmp
	a.mu.Lock()
	dirty := s.dirty
	s.dirty = false
	a.mu.Unlock()
	if !dirty || len(s.open) == 0 {
		return
	}
	cb := a.compareBuild()
	key := view.EncodeBuild(cb)
	a.mu.Lock()
	if key == s.key {
		a.mu.Unlock()
		return
	}
	s.seq++
	seq, lang := s.seq, a.Lang
	a.mu.Unlock()
	go func() {
		v := a.V.Build(cb, lang)
		a.mu.Lock()
		if seq == s.seq {
			s.view, s.key, s.ready = v, key, true
		}
		a.mu.Unlock()
		if a.Invalidate != nil {
			a.Invalidate()
		}
	}()
}

// compareView is the comparison's windows, once computed.
func (a *App) compareView() (view.BuildView, bool) {
	if len(a.ecmp.open) == 0 {
		return view.BuildView{}, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ecmp.view, a.ecmp.ready
}

// The slot editor works on the build, or on the comparison when it edits
// a comparison slot.

func (a *App) editView() view.BuildView {
	if a.slot.cmp {
		if v, ok := a.compareView(); ok {
			return v
		}
	}
	return a.View()
}

func (a *App) editBuild() calc.Build {
	if a.slot.cmp {
		return a.compareBuild()
	}
	return a.Build()
}

func (a *App) editSet(nb calc.Build) {
	if a.slot.cmp {
		a.setCompare(nb)
		return
	}
	a.SetBuild(nb)
}

// compareSlotNames are the labels of the slots in comparison.
func (a *App) compareSlotNames(v view.BuildView) []string {
	var out []string
	for _, k := range a.ecmp.open {
		_, label := a.slotView(v, k)
		out = append(out, a.T(label))
	}
	return out
}

// compareSummary is "↔ 2 slots em comparação".
func (a *App) compareSummary() string {
	n := len(a.ecmp.open)
	if n == 1 {
		return "↔ " + a.T("1 slot em comparação")
	}
	return fmt.Sprintf("↔ "+a.T("%d slots em comparação"), n)
}

// equippedMsg is what the chat says when the slot editor puts an item on.
func (a *App) equippedMsg() string {
	if a.slot.cmp {
		return a.T("Peça da comparação escolhida.")
	}
	return a.T("Item equipado.")
}

// compareToggle is the ↔ button in a slot's corner: it puts the slot in
// comparison, or takes it out (amber while it is in).
func (a *App) compareToggle(gtx C, key string, under func() image.Point) D {
	cl := clickable(&a.ecmp.btn, key)
	on := a.comparing(key)
	if cl.Clicked(gtx) {
		if on {
			a.closeCompare(key)
		} else {
			pos := under()
			if len(a.ecmp.open) == 0 { // the bar shows up above the slots now
				pos.Y += gtx.Dp(compareBarH)
			}
			a.openCompare(key)
			a.wSlot.Pos, a.wSlot.Placed = pos, true
		}
		on = !on
	}
	d := cl.Layout(gtx, func(gtx C) D {
		sz := image.Pt(gtx.Dp(22), gtx.Dp(18))
		r := image.Rectangle{Max: sz}
		bg, line, fg := ColBtnB, ColListBorder, ColLabel
		switch {
		case on:
			bg, line, fg = ColCompareBg, ColCompare, ColCompare
		case cl.Hovered():
			bg = ColHover
		}
		FillRound(gtx, r, bg, 3, 3, 3, 3)
		Border(gtx, r, line, 1, 3)
		g := gtx
		g.Constraints = layout.Exact(sz)
		layout.Center.Layout(g, B(a.Th, "↔").C(fg).W())
		pointerCursor(gtx, sz)
		return D{Size: sz}
	})
	if cl.Hovered() {
		tip := a.T("Comparar outro item neste slot")
		if on {
			tip = a.T("Parar de comparar este slot")
		}
		a.Desk.ShowTip(func(gtx C) D { return TipBox(gtx, T(a.Th, tip).W()) })
	}
	return d
}

// compareSlot is the comparison slot under a slot in comparison: what the
// comparison has there; a click edits it, × takes the slot out.
func (a *App) compareSlot(gtx C, key string) D {
	th := a.Th
	cv, ok := a.compareView()
	if !ok {
		cv = a.View()
	}
	sv, _ := a.slotView(cv, key)
	cl := clickable(&a.ecmp.pick, key)
	rm := clickable(&a.ecmp.remove, key)
	if a.ecmp.at == nil {
		a.ecmp.at = map[string]*Anchor{}
	}
	at := a.ecmp.at[key]
	if at == nil {
		at = &Anchor{}
		a.ecmp.at[key] = at
	}
	h := gtx.Dp(equipSlotH)
	if rm.Clicked(gtx) {
		a.closeCompare(key)
		return D{}
	}
	if cl.Clicked(gtx) {
		a.openCompare(key)
		a.wSlot.Pos, a.wSlot.Placed = at.Origin.Add(image.Pt(0, h)), true
	}
	defer func() { at.Track(gtx, a.Desk, image.Pt(gtx.Constraints.Max.X, h)) }()
	w := gtx.Constraints.Max.X
	r := image.Rectangle{Max: image.Pt(w, h)}
	d := cl.Layout(gtx, func(gtx C) D {
		bg := ColCompareBg
		if cl.Hovered() || a.slot.cmp && a.slot.key == key && a.wSlot.Open {
			bg = ColHover
		}
		Fill(gtx, r, bg)
		DashedBorder(gtx, r, ColCompare, gtx.Dp(1), gtx.Dp(4))
		g := gtx
		g.Constraints = layout.Exact(image.Pt(w-gtx.Dp(30), h))
		st := op.Offset(image.Pt(gtx.Dp(4), 0)).Push(gtx.Ops)
		layout.Flex{Alignment: layout.Middle}.Layout(g,
			layout.Rigid(func(gtx C) D {
				if sv.Item == nil {
					sz := image.Pt(gtx.Dp(36), gtx.Dp(36))
					FillRound(gtx, image.Rectangle{Max: sz}, ColBodyAlt, 3, 3, 3, 3)
					return D{Size: sz}
				}
				return a.Icons.Art(gtx, sv.Item.ID, 36)
			}),
			layout.Rigid(Gap(5, 0)),
			layout.Flexed(1, func(gtx C) D {
				if sv.Item == nil {
					return T(th, a.T("Vazio na comparação")).C(ColMuted).Layout(gtx)
				}
				var names []string
				for _, c := range sv.Cards {
					if c != nil {
						names = append(names, c.Name)
					}
				}
				if len(names) == 0 {
					return T(th, itemTitle(sv.Item, sv.Refine, sv.Grade)).Lines(2).Wrapping().Layout(gtx)
				}
				return Col(1, T(th, itemTitle(sv.Item, sv.Refine, sv.Grade)).Lines(1).W(),
					T(th, strings.Join(names, " · ")).Sz(SmallText).C(ColCompare).Lines(1).W())(gtx)
			}),
		)
		st.Pop()
		pointerCursor(gtx, r.Max)
		return D{Size: r.Max}
	})
	st := op.Offset(image.Pt(w-gtx.Dp(24), (h-gtx.Dp(18))/2)).Push(gtx.Ops)
	rm.Layout(gtx, func(gtx C) D {
		sz := image.Pt(gtx.Dp(20), gtx.Dp(18))
		g := gtx
		g.Constraints = layout.Exact(sz)
		layout.Center.Layout(g, B(th, "×").Sz(13).C(ColCompare).W())
		pointerCursor(gtx, sz)
		return D{Size: sz}
	})
	st.Pop()
	if rm.Hovered() {
		a.Desk.ShowTip(func(gtx C) D { return TipBox(gtx, T(th, a.T("Parar de comparar este slot")).W()) })
	}
	return d
}

// compareBarH is the bar's height with its margin below.
const compareBarH = 32

// compareBar sits above the slots while any is in comparison.
func (a *App) compareBar(gtx C) D {
	th, s := a.Th, &a.ecmp
	if s.apply.Clicked(gtx) {
		a.applyCompare()
		return D{}
	}
	if s.clear.Clicked(gtx) {
		a.clearCompare()
		return D{}
	}
	return Box{Bg: ColCompareBg, Line: ColCompare, Radius: 3, Pad: Inset(4, 8, 4, 8)}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(B(th, a.compareSummary()).C(ColCompare).W()),
			layout.Rigid(Gap(8, 0)),
			layout.Flexed(1, T(th, strings.Join(a.compareSlotNames(a.View()), ", ")).C(ColMuted).Lines(1).W()),
			layout.Rigid(func(gtx C) D { return Button{Label: a.T("Aplicar"), Bold: true}.Layout(gtx, th, &s.apply) }),
			layout.Rigid(Gap(4, 0)),
			layout.Rigid(func(gtx C) D { return Button{Label: a.T("Limpar") + " ×"}.Layout(gtx, th, &s.clear) }),
		)
	})
}

// compareFrom puts what nb changes in the equipment window's comparison
// (a set of Comparar conjuntos, an optimizer suggestion), replacing the
// comparison there was, and shows the equipment window. It reports
// whether anything differs from the build.
func (a *App) compareFrom(nb calc.Build) bool {
	a.clearCompare()
	a.setCompare(nb)
	for _, slot := range calc.Slots {
		if e, ok := a.ecmp.over[slot]; ok && e != nil {
			a.ecmp.open = append(a.ecmp.open, slot)
		}
	}
	if len(a.ecmp.open) == 0 {
		a.ecmp.over = nil
		a.Say(a.T("Isso já é o que a build usa."), false)
		return false
	}
	a.markCompare()
	a.shellSelect(a.wEquip)
	a.Say(a.T("Comparando no equipamento: veja o Resumo."), false)
	return true
}
