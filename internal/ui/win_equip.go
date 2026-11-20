package ui

import (
	"context"
	"fmt"
	"image"
	"strconv"
	"strings"
	"sync"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/edit"
	"ragnarok-of-building/internal/view"
)

// ---- Equipamentos (win-equip) and the slot editor (win-slot) ----

type equipState struct {
	tabs  Tabs
	tab   int
	slots map[string]*widget.Clickable
}

type slotState struct {
	key  string // the slot being edited ("" = none)
	card int    // -1: picking the item itself; else the card/enchant position

	q       Input
	lastQ   string
	results []view.ItemView
	gains   map[int]view.StatLine
	seq     int // searches started; older results are dropped
	pending bool
	timer   *time.Timer
	list    widget.List
	rows    map[int]*widget.Clickable
	cmpBtns map[int]*widget.Clickable // ↔: pin to the comparison

	refine, grade Select
	cardBtns      [4]widget.Clickable
	cardDel       [4]widget.Clickable
	optVals       []NumInput
	optDel        []widget.Clickable
	newOpt        Select
	newOptVal     NumInput
	newOptPick    string
	addOpt        widget.Clickable
	pickItem      widget.Clickable
	unequip       widget.Clickable

	// Best item for the slot: every item ranked at bestRefine/bestGrade.
	bestBtn               widget.Clickable
	bestRef, bestGra      Select
	bestRefine, bestGrade int
	bestFor               string // the slot bestRefine/bestGrade were set for
	best                  *view.BestResult
	bestRunning, bestOn   bool
	bestSeq               int

	tipMu sync.Mutex
	tips  map[string]*view.SwapView // swap comparisons by build code + slot + item + card
}

func (a *App) equipBody(gtx C) D {
	th, s := a.Th, &a.equip
	v := a.View()
	if s.slots == nil {
		s.slots = map[string]*widget.Clickable{}
	}
	tabs := []string{a.T("Equip"), a.T("Sombra"), a.T("Visual")}
	panels := [][]view.SlotView{v.Slots, v.Shadow, v.Costume}
	middle := []string{v.JobName, a.T("Sombra"), a.T("Visual")}
	return Col(0,
		func(gtx C) D {
			// The tabs reach the window's edges (the body has 8px sides).
			gtx.Constraints.Max.X += gtx.Dp(16)
			st := op.Offset(image.Pt(-gtx.Dp(8), -gtx.Dp(6))).Push(gtx.Ops)
			d, i, ok := s.tabs.Layout(gtx, th, tabs, s.tab)
			st.Pop()
			if ok {
				s.tab = i
			}
			d.Size.Y -= gtx.Dp(6)
			return d
		},
		Gap(0, 6),
		func(gtx C) D {
			slots := panels[s.tab]
			col := func(side string) layout.Widget {
				return func(gtx C) D {
					var ws []layout.Widget
					for _, sv := range slots {
						if sv.Side == side {
							sv := sv
							ws = append(ws, func(gtx C) D { return a.equipSlot(gtx, sv) })
						}
					}
					return Col(4, ws...)(gtx)
				}
			}
			sprite := func(gtx C) D {
				sz := image.Pt(gtx.Dp(96), max(gtx.Dp(150), gtx.Constraints.Min.Y))
				Fill(gtx, image.Rectangle{Max: sz}, rgb(0xfafafa))
				Border(gtx, image.Rectangle{Max: sz}, ColSep, 1, 0)
				g := gtx
				g.Constraints = layout.Exact(sz)
				layout.Center.Layout(g, T(th, middle[s.tab]).Sz(SmallText).C(ColMuted).W())
				return D{Size: sz}
			}
			return layout.Flex{}.Layout(gtx,
				layout.Flexed(1, col("L")),
				layout.Rigid(Gap(6, 0)),
				layout.Rigid(sprite),
				layout.Rigid(Gap(6, 0)),
				layout.Flexed(1, col("R")),
			)
		},
	)(gtx)
}

// itemTitle is "[A] +7 Name [3]".
func itemTitle(it *view.ItemView, refine, grade int) string {
	var b strings.Builder
	if grade > 0 && grade <= 4 {
		b.WriteString("[" + string("DCBA"[grade-1]) + "] ")
	}
	if refine > 0 {
		b.WriteString("+" + strconv.Itoa(refine) + " ")
	}
	b.WriteString(it.Name)
	if it.Slots > 0 {
		b.WriteString(" [" + strconv.Itoa(it.Slots) + "]")
	}
	return b.String()
}

func (a *App) equipSlot(gtx C, sv view.SlotView) D {
	th, s := a.Th, &a.equip
	cl := s.slots[sv.Key]
	if cl == nil {
		cl = &widget.Clickable{}
		s.slots[sv.Key] = cl
	}
	if cl.Clicked(gtx) {
		a.openSlot(sv.Key)
	}
	d := cl.Layout(gtx, func(gtx C) D {
		w, h := gtx.Constraints.Max.X, gtx.Dp(30)
		r := image.Rectangle{Max: image.Pt(w, h)}
		bg := ColWhite
		if cl.Hovered() || a.slot.key == sv.Key && a.wSlot.Open {
			bg = rgb(0xf3f7ff)
		}
		Fill(gtx, r, bg)
		Border(gtx, r, ColListBorder, 1, 2)
		g := gtx
		g.Constraints = layout.Exact(image.Pt(w-gtx.Dp(8), h))
		st := op.Offset(image.Pt(gtx.Dp(4), 0)).Push(gtx.Ops)
		layout.Flex{Alignment: layout.Middle}.Layout(g,
			layout.Rigid(func(gtx C) D {
				if sv.Item == nil {
					sz := image.Pt(gtx.Dp(24), gtx.Dp(24))
					Fill(gtx, image.Rectangle{Max: sz}, rgb(0xeef0f4))
					return D{Size: sz}
				}
				return a.Icons.Icon(gtx, view.ItemIcon(sv.Item.ID), 24)
			}),
			layout.Rigid(Gap(5, 0)),
			layout.Flexed(1, func(gtx C) D {
				if sv.Item == nil {
					return T(th, a.T(sv.Label)).C(rgb(0x999999)).Layout(gtx)
				}
				t := T(th, itemTitle(sv.Item, sv.Refine, sv.Grade)).Lines(2).Wrapping()
				return t.Layout(gtx)
			}),
		)
		st.Pop()
		pointerCursor(gtx, r.Max)
		return D{Size: r.Max}
	})
	if sv.Item != nil && cl.Hovered() {
		id := sv.Item.ID
		a.Desk.ShowTip(func(gtx C) D { return a.itemTip(gtx, id, nil) })
	}
	return d
}

// openSlot shows the slot editor for a slot (picking the item itself).
func (a *App) openSlot(key string) {
	s := &a.slot
	s.key, s.card = key, -1
	s.bestOn = false
	s.q.Ed.SetText("")
	if !a.wSlot.Open {
		a.Desk.Toggle(a.wSlot)
	}
	a.Desk.Raise(a.wSlot)
	a.search(0)
}

// slotView finds the slot's current contents.
func (a *App) slotView(v view.BuildView, key string) (view.SlotView, string) {
	for _, list := range [][]view.SlotView{v.Slots, v.Shadow, v.Costume} {
		for _, sv := range list {
			if sv.Key == key {
				return sv, sv.Label
			}
		}
	}
	return view.SlotView{Key: key}, key
}

// search runs the picker query in the background after delay.
func (a *App) search(delay time.Duration) {
	s := &a.slot
	a.mu.Lock()
	s.seq++
	seq, slot, card, q, b, lang := s.seq, s.key, s.card, s.q.Text(), a.b, a.Lang
	s.pending = true
	a.mu.Unlock()
	if s.timer != nil {
		s.timer.Stop()
	}
	run := func() {
		items, gains := a.V.PickItems(b, slot, card >= 0, card, q, lang)
		a.mu.Lock()
		if seq == s.seq {
			s.results, s.gains, s.pending = items, gains, false
		}
		a.mu.Unlock()
		if a.Invalidate != nil {
			a.Invalidate()
		}
	}
	if delay == 0 {
		go run()
	} else {
		s.timer = time.AfterFunc(delay, run)
	}
}

func (a *App) slotTitle() string {
	_, label := a.slotView(a.View(), a.slot.key)
	return a.T(label)
}

func (a *App) slotBody(gtx C) D {
	th, s := a.Th, &a.slot
	v := a.View()
	if s.key == "" {
		return T(th, a.T("Escolha um item")).Layout(gtx)
	}
	sv, label := a.slotView(v, s.key)
	b := v.Build
	setBuild := func(nb calc.Build) { a.SetBuild(nb); a.search(0) }

	// Editor actions.
	if s.bestFor != s.key {
		s.bestFor = s.key
		s.bestRefine, s.bestGrade = sv.Refine, sv.Grade
	}
	if s.bestBtn.Clicked(gtx) {
		a.runBest()
	}
	if s.pickItem.Clicked(gtx) {
		s.bestOn = false
		s.card = -1
		s.q.Ed.SetText("")
		a.search(0)
	}
	if s.unequip.Clicked(gtx) {
		setBuild(edit.Unequip(b, s.key))
		a.Say(a.T("Item removido."), false)
	}
	for i := range 4 {
		if s.cardBtns[i].Clicked(gtx) {
			s.bestOn = false
			s.card = i
			s.q.Ed.SetText("")
			a.search(0)
		}
		if s.cardDel[i].Clicked(gtx) {
			setBuild(edit.EditLinked(b, s.key, func(e *calc.Equip) { e.Cards[i] = 0 }))
		}
	}
	for i := range s.optDel {
		if i < len(sv.Opts) && s.optDel[i].Clicked(gtx) {
			k := i
			setBuild(edit.EditLinked(b, s.key, func(e *calc.Equip) {
				e.Opts = append(e.Opts[:k:k], e.Opts[k+1:]...)
			}))
		}
	}
	if s.addOpt.Clicked(gtx) && s.newOptPick != "" {
		id, _ := strconv.Atoi(s.newOptPick)
		val := s.newOptVal.last
		setBuild(edit.EditLinked(b, s.key, func(e *calc.Equip) { e.Opts = append(e.Opts, [2]int{id, val}) }))
		s.newOptPick = ""
	}

	var kids []layout.Widget
	if sv.Item != nil {
		kids = append(kids, a.slotEditor(sv, label))
	} else {
		kids = append(kids, T(th, a.T(label)+" "+a.T("vazio. Escolha um item abaixo.")).C(ColMuted).W())
	}
	kids = append(kids, Sep,
		func(gtx C) D {
			s.q.Hint = a.T("Buscar item…")
			d, changed, _ := s.q.Layout(gtx, th, 0)
			if changed && s.q.Text() != s.lastQ {
				s.bestOn = false
				s.lastQ = s.q.Text()
				a.search(150 * time.Millisecond)
			}
			return d
		},
		func(gtx C) D {
			if s.card >= 0 {
				return D{}
			}
			return a.bestControls(gtx)
		},
		func(gtx C) D { return a.pickList(gtx) },
		func(gtx C) D {
			msg := a.T("Escolha um item para") + " " + a.T(label)
			if s.bestOn {
				msg = a.bestStatus()
			}
			if s.card >= 0 {
				if s.card < sv.CardSlots {
					msg = fmt.Sprintf("%s (slot %d)", a.T("Escolha uma carta"), s.card+1)
				} else {
					msg = a.T("Escolha um encantamento")
				}
			}
			return T(th, msg).C(ColMuted).Layout(gtx)
		},
	)
	return Col(4, kids...)(gtx)
}

func (a *App) slotEditor(sv view.SlotView, label string) layout.Widget {
	th, s := a.Th, &a.slot
	return func(gtx C) D {
		b := a.View().Build
		var kids []layout.Widget
		head := func(gtx C) D {
			return RowTop(8,
				func(gtx C) D {
					return Box{Bg: ColWhite, Line: ColListBorder, Pad: Inset(4, 4, 4, 4)}.Layout(gtx, func(gtx C) D {
						return a.Icons.Icon(gtx, view.ItemIcon(sv.Item.ID), 40)
					})
				},
				func(gtx C) D {
					var rows []layout.Widget
					rows = append(rows, B(th, itemTitle(sv.Item, sv.Refine, sv.Grade)).Wrapping().W())
					var ctl []layout.Widget
					if sv.Refineable {
						var opts []Option
						for r := 0; r <= 20; r++ {
							opts = append(opts, Option{strconv.Itoa(r), "+" + strconv.Itoa(r)})
						}
						ctl = append(ctl, T(th, a.T("Refino")).W(), func(gtx C) D {
							d, val, ok := s.refine.Layout(gtx, th, a.Desk, opts, strconv.Itoa(sv.Refine), 56)
							if ok {
								r, _ := strconv.Atoi(val)
								a.SetBuild(edit.EditLinked(b, s.key, func(e *calc.Equip) { e.Refine = r }))
							}
							return d
						})
					}
					if sv.Gradable {
						opts := []Option{{"0", a.T("nenhum")}, {"1", "D"}, {"2", "C"}, {"3", "B"}, {"4", "A"}}
						ctl = append(ctl, T(th, a.T("Grau")).W(), func(gtx C) D {
							d, val, ok := s.grade.Layout(gtx, th, a.Desk, opts, strconv.Itoa(sv.Grade), 70)
							if ok {
								g, _ := strconv.Atoi(val)
								a.SetBuild(edit.EditLinked(b, s.key, func(e *calc.Equip) { e.Grade = g }))
							}
							return d
						})
					}
					if len(ctl) > 0 {
						rows = append(rows, Row(4, ctl...))
					}
					return Col(4, rows...)(gtx)
				},
			)(gtx)
		}
		kids = append(kids, head)
		if len(sv.Cards) > 0 {
			kids = append(kids, func(gtx C) D {
				return Box{Bg: ColWhite, Line: ColListBorder, Pad: Inset(2, 4, 2, 4)}.Layout(gtx, func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					var rows []layout.Widget
					for i, c := range sv.Cards {
						i, c := i, c
						rows = append(rows, func(gtx C) D { return a.cardRow(gtx, sv, i, c) })
					}
					return Col(2, rows...)(gtx)
				})
			})
		}
		if len(sv.Opts) > 0 || sv.MaxOpts > 0 {
			kids = append(kids, a.randomOpts(sv))
		}
		kids = append(kids, Row(4,
			func(gtx C) D { return Button{Label: a.T("Trocar item")}.Layout(gtx, th, &s.pickItem) },
			func(gtx C) D { return Button{Label: a.T("Remover")}.Layout(gtx, th, &s.unequip) },
		))
		return Col(6, kids...)(gtx)
	}
}

func (a *App) cardRow(gtx C, sv view.SlotView, i int, c *view.ItemView) D {
	th, s := a.Th, &a.slot
	icon := func(gtx C) D {
		if c == nil {
			sz := image.Pt(gtx.Dp(24), gtx.Dp(24))
			Fill(gtx, image.Rectangle{Max: sz}, rgb(0xeef0f4))
			return D{Size: sz}
		}
		return a.Icons.Icon(gtx, view.ItemIcon(c.ID), 24)
	}
	name := func(gtx C) D {
		if c == nil {
			txt := fmt.Sprintf("Slot %d %s", i+1, a.T("vazio"))
			if i >= sv.CardSlots {
				txt = a.T("Encantamento vazio")
			}
			return T(th, txt).C(rgb(0x999999)).Layout(gtx)
		}
		n := c.Name
		if c.MVP {
			n += " (MVP)"
		}
		return T(th, n).Layout(gtx)
	}
	btn := a.T("Trocar")
	if c == nil {
		btn = a.T("Carta…")
		if i >= sv.CardSlots {
			btn = a.T("Encantar…")
		}
	}
	d := layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(icon),
		layout.Rigid(Gap(5, 0)),
		layout.Flexed(1, name),
		layout.Rigid(func(gtx C) D {
			bt := Button{Label: btn}
			if s.card == i {
				bt.Bold = true
			}
			return bt.Layout(gtx, th, &s.cardBtns[i])
		}),
		layout.Rigid(func(gtx C) D {
			if c == nil {
				return D{}
			}
			return Inset(0, 0, 0, 3).Layout(gtx, func(gtx C) D { return Button{Label: "×", MinWidth: 18}.Layout(gtx, th, &s.cardDel[i]) })
		}),
	)
	return d
}

func (a *App) randomOpts(sv view.SlotView) layout.Widget {
	th, s := a.Th, &a.slot
	return func(gtx C) D {
		b := a.View().Build
		if len(s.optVals) < len(sv.Opts) {
			s.optVals = append(s.optVals, make([]NumInput, len(sv.Opts)-len(s.optVals))...)
			s.optDel = append(s.optDel, make([]widget.Clickable, len(sv.Opts)-len(s.optDel))...)
		}
		title := a.T("Bônus aleatórios")
		if sv.MaxOpts > 0 {
			title += fmt.Sprintf(" (%d/%d)", len(sv.Opts), sv.MaxOpts)
		}
		rows := []layout.Widget{B(th, title).C(ColTitleText).W()}
		for i, o := range sv.Opts {
			i, o := i, o
			rows = append(rows, func(gtx C) D {
				lbl := o.Label
				if o.Max > 0 {
					lbl += fmt.Sprintf(" (%d–%d)", o.Min, o.Max)
				}
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, T(th, lbl).W()),
					layout.Rigid(func(gtx C) D {
						d, x, ok := s.optVals[i].Layout(gtx, th, o.Value, 60)
						if ok {
							a.SetBuild(edit.EditLinked(b, s.key, func(e *calc.Equip) {
								if i < len(e.Opts) {
									e.Opts[i][1] = x
								}
							}))
						}
						return d
					}),
					layout.Rigid(Gap(3, 0)),
					layout.Rigid(func(gtx C) D { return Button{Label: "×", MinWidth: 18}.Layout(gtx, th, &s.optDel[i]) }),
				)
			})
		}
		if len(sv.Opts) < sv.MaxOpts {
			list := a.V.RandomOpts(sv.Item.ID, len(sv.Opts), a.Lang)
			opts := []Option{{"", a.T("Adicionar bônus…")}}
			for _, o := range list {
				l := o.Label
				if o.Max > 0 {
					l += fmt.Sprintf(" (%d–%d)", o.Min, o.Max)
				}
				opts = append(opts, Option{strconv.Itoa(o.ID), l})
			}
			rows = append(rows, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D {
						d, val, ok := s.newOpt.Layout(gtx, th, a.Desk, opts, s.newOptPick, 0)
						if ok {
							s.newOptPick = val
							// The value box starts at the option's best roll.
							for _, o := range list {
								if strconv.Itoa(o.ID) == val && o.Max > 0 {
									s.newOptVal.last, s.newOptVal.set = o.Max, false
								}
							}
						}
						return d
					}),
					layout.Rigid(Gap(3, 0)),
					layout.Rigid(func(gtx C) D {
						if s.newOptVal.last == 0 && !s.newOptVal.set {
							s.newOptVal.last = 1
						}
						d, x, ok := s.newOptVal.Layout(gtx, th, s.newOptVal.last, 60)
						if ok {
							s.newOptVal.last = x
						}
						return d
					}),
					layout.Rigid(Gap(3, 0)),
					layout.Rigid(func(gtx C) D { return Button{Label: a.T("Adicionar")}.Layout(gtx, th, &s.addOpt) }),
				)
			})
		}
		return Box{Line: ColSep, Pad: Inset(4, 6, 6, 6)}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return Col(3, rows...)(gtx)
		})
	}
}

// pickList is the picker's results: icon, name, damage change, ID.
func (a *App) pickList(gtx C) D {
	th, s := a.Th, &a.slot
	a.mu.Lock()
	results, gains, pending := s.results, s.gains, s.pending
	if s.bestOn {
		results, gains, pending = nil, map[int]view.StatLine{}, s.bestRunning
		if s.best != nil {
			for _, bi := range s.best.Items {
				results = append(results, bi.Item)
				gains[bi.Item.ID] = bi.Gain
			}
		}
	}
	a.mu.Unlock()
	if s.rows == nil {
		s.rows = map[int]*widget.Clickable{}
	}
	if s.cmpBtns == nil {
		s.cmpBtns = map[int]*widget.Clickable{}
	}
	for _, it := range results {
		cl := s.rows[it.ID]
		if cl != nil && cl.Clicked(gtx) {
			a.pick(it.ID)
		}
		if cb := s.cmpBtns[it.ID]; cb != nil && cb.Clicked(gtx) {
			a.addCompare(it.ID)
		}
	}
	h := gtx.Dp(220)
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
	return Box{Bg: ColWhite, Line: ColListBorder, Pad: Inset(2, 2, 2, 2)}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min = gtx.Constraints.Max
		if len(results) == 0 {
			msg := a.T("Nenhum item disponível.")
			if pending {
				msg = a.T("Buscando…")
			} else if s.bestOn && s.best != nil && s.best.NoTarget {
				msg = a.T("Escolha um monstro na janela de dano (Alt+D) para achar o melhor item.")
			} else if s.q.Text() != "" {
				msg = a.T("Nada encontrado para este slot.")
			}
			return Inset(8, 8, 8, 8).Layout(gtx, T(th, msg).C(rgb(0x777777)).W())
		}
		s.list.Axis = layout.Vertical
		return s.list.Layout(gtx, len(results), func(gtx C, i int) D {
			it := results[i]
			cl := s.rows[it.ID]
			if cl == nil {
				cl = &widget.Clickable{}
				s.rows[it.ID] = cl
			}
			row := func(gtx C) D {
				return cl.Layout(gtx, func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					m := op.Record(gtx.Ops)
					d := Inset(2, 6, 2, 6).Layout(gtx, func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						name := it.Name
						if it.Slots > 0 {
							name += fmt.Sprintf(" [%d]", it.Slots)
						}
						if it.MVP {
							name += " (MVP)"
						}
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx C) D { return a.Icons.Icon(gtx, view.ItemIcon(it.ID), 24) }),
							layout.Rigid(Gap(6, 0)),
							layout.Flexed(1, T(th, name).W()),
							layout.Rigid(func(gtx C) D {
								g, ok := gains[it.ID]
								if !ok || g.Diff == "" {
									return D{}
								}
								c := ColMuted
								switch {
								case g.Same:
								case g.Better:
									c = ColUp
								default:
									c = ColDown
								}
								return Inset(0, 6, 0, 6).Layout(gtx, B(th, g.Diff).C(c).W())
							}),
							layout.Rigid(T(th, fmt.Sprintf("#%d", it.ID)).Sz(SmallText).C(rgb(0x888888)).W()),
						)
					})
					call := m.Stop()
					if cl.Hovered() {
						Fill(gtx, image.Rectangle{Max: d.Size}, ColHover)
					}
					call.Add(gtx.Ops)
					pointerCursor(gtx, d.Size)
					return d
				})
			}
			var d D
			if s.card >= 0 {
				d = row(gtx)
			} else {
				cb := s.cmpBtns[it.ID]
				if cb == nil {
					cb = &widget.Clickable{}
					s.cmpBtns[it.ID] = cb
				}
				d = layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, row),
					layout.Rigid(func(gtx C) D {
						d := Inset(0, 2, 0, 2).Layout(gtx, func(gtx C) D { return Button{Label: "↔"}.Layout(gtx, th, cb) })
						if cb.Hovered() {
							a.Desk.ShowTip(func(gtx C) D { return TipBox(gtx, T(th, a.T("Comparar lado a lado (Alt+K)")).W()) })
						}
						return d
					}),
				)
			}
			if cl.Hovered() {
				id := it.ID
				cur := a.slotCurrent()
				a.Desk.ShowTip(func(gtx C) D { return a.compareTip(gtx, cur, id, a.swapTip(id)) })
			}
			return d
		})
	})
}

// pick puts a picker result in the slot (or in the card position).
func (a *App) pick(id int) {
	s := &a.slot
	b := a.Build()
	if s.card >= 0 {
		k := s.card
		a.SetBuild(edit.EditLinked(b, s.key, func(e *calc.Equip) { e.Cards[k] = id }))
		a.Say(a.T("Carta aplicada."), false)
		s.card = -1
	} else if s.bestOn {
		a.SetBuild(a.V.SwapAt(b, s.key, id, s.bestRefine, s.bestGrade))
		a.Say(a.T("Item equipado."), false)
		s.bestOn = false
	} else {
		a.SetBuild(a.V.Swap(b, s.key, id, -1))
		a.Say(a.T("Item equipado."), false)
	}
	s.q.Ed.SetText("")
	s.lastQ = ""
	a.search(0)
}

// swapTip is the tooltip's comparison for a picker result, computed in the
// background once per build, slot and item.
func (a *App) swapTip(id int) *view.SwapView {
	s := &a.slot
	b := a.Build()
	refine, grade := -1, -1
	if s.bestOn {
		refine, grade = s.bestRefine, s.bestGrade
	}
	key := fmt.Sprintf("%s|%s|%d|%d|%d|%d", view.EncodeBuild(b), s.key, id, s.card, refine, grade)
	s.tipMu.Lock()
	defer s.tipMu.Unlock()
	if s.tips == nil {
		s.tips = map[string]*view.SwapView{}
	}
	if sv, ok := s.tips[key]; ok {
		return sv
	}
	s.tips[key] = nil
	if len(s.tips) > 200 {
		s.tips = map[string]*view.SwapView{key: nil}
	}
	slot, card, lang := s.key, s.card, a.Lang
	go func() {
		sv := a.V.SwapCompare(b, slot, id, card, lang)
		if refine >= 0 {
			sv = a.V.SwapCompareAt(b, slot, id, refine, grade, lang)
		}
		s.tipMu.Lock()
		s.tips[key] = &sv
		s.tipMu.Unlock()
		if a.Invalidate != nil {
			a.Invalidate()
		}
	}()
	return nil
}

// itemTip is the item tooltip: header (icon, name, slots, ID), the swap
// comparison when given, and the colored description.
func (a *App) itemTip(gtx C, id int, sw *view.SwapView) D { return a.itemTipCap(gtx, id, sw, "") }

// slotCurrent is what the picker would replace: the slot's item, or the
// card in the position being picked (0 when empty).
func (a *App) slotCurrent() int {
	s := &a.slot
	e := a.Build().Equip[s.key]
	if s.card >= 0 && s.card < 4 {
		return e.Cards[s.card]
	}
	return e.Item
}

// compareTip shows the equipped item's tooltip next to the hovered one's,
// both topped by a caption ("Equipado", "Novo").
func (a *App) compareTip(gtx C, cur, id int, sw *view.SwapView) D {
	if cur == 0 || cur == id {
		return a.itemTip(gtx, id, sw)
	}
	gtx.Constraints.Max.X = gtx.Dp(610)
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return a.itemTipCap(gtx, cur, nil, a.T("Equipado")) }),
		layout.Rigid(Gap(6, 0)),
		layout.Rigid(func(gtx C) D { return a.itemTipCap(gtx, id, sw, a.T("Novo")) }),
	)
}

// itemTipCap is itemTip with a caption strip above the header.
func (a *App) itemTipCap(gtx C, id int, sw *view.SwapView, caption string) D {
	th := a.Th
	it := a.V.Item(id, a.Lang)
	if it == nil {
		return D{}
	}
	gtx.Constraints.Max.X = gtx.Dp(300)
	gtx.Constraints.Min.X = gtx.Dp(300)
	m := op.Record(gtx.Ops)
	d := Col(0,
		func(gtx C) D {
			if caption == "" {
				return D{}
			}
			return Inset(3, 6, 0, 6).Layout(gtx, B(th, caption).Sz(SmallText).C(ColLabel).W())
		},
		func(gtx C) D {
			mm := op.Record(gtx.Ops)
			hd := Inset(3, 6, 3, 6).Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				name := it.Name
				if it.Slots > 0 {
					name += fmt.Sprintf(" [%d]", it.Slots)
				}
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return a.Icons.Icon(gtx, view.ItemIcon(id), 24) }),
					layout.Rigid(Gap(6, 0)),
					layout.Flexed(1, B(th, name).Wrapping().W()),
					layout.Rigid(T(th, fmt.Sprintf("#%d", id)).Sz(SmallText).C(rgb(0x666677)).W()),
				)
			})
			call := mm.Stop()
			r := image.Rectangle{Max: hd.Size}
			VGradient(gtx, r, rgb(0xe9eef8), rgb(0xcfd8ea), 3, 3, 0, 0)
			call.Add(gtx.Ops)
			Fill(gtx, image.Rect(0, r.Max.Y-1, r.Max.X, r.Max.Y), ColListBorder)
			return hd
		},
		func(gtx C) D {
			if sw == nil {
				return D{}
			}
			return a.swapBox(gtx, *sw)
		},
		func(gtx C) D {
			return Inset(5, 6, 5, 6).Layout(gtx, RichText(th, it.Desc, rgb(0x000000)))
		},
	)(gtx)
	call := m.Stop()
	r := image.Rectangle{Max: d.Size}
	FillRound(gtx, r, ColWhite, 3, 3, 3, 3)
	call.Add(gtx.Ops)
	Border(gtx, r, rgb(0x8a94a8), 1, 3)
	return d
}

func (a *App) swapBox(gtx C, sw view.SwapView) D {
	th := a.Th
	head := a.T("Slot vazio hoje")
	if sw.Current != "" {
		head = a.T("Comparado com") + " " + sw.Current
	}
	if sw.Target != "" {
		head += " · " + a.T("alvo") + ": " + sw.Target
	}
	rows := []layout.Widget{T(th, head).C(ColLabel).Wrapping().W()}
	line := func(l view.StatLine, bold bool) layout.Widget {
		c := ColDown
		if l.Better {
			c = ColUp
		}
		if l.Same {
			c = ColMuted
		}
		tx := func(s string) Txt {
			t := T(th, s)
			if bold {
				t = t.Bold()
			}
			return t
		}
		return func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, tx(l.Label).C(ColLabel).W()),
				layout.Rigid(tx(l.Old).W()),
				layout.Rigid(Gap(4, 0)),
				layout.Rigid(tx("→ "+l.New).W()),
				layout.Rigid(Gap(6, 0)),
				layout.Rigid(tx(l.Diff).C(c).W()),
			)
		}
	}
	if sw.Expected != nil {
		rows = append(rows, line(*sw.Expected, true))
	}
	for _, l := range sw.Lines {
		rows = append(rows, line(l, false))
	}
	if sw.Expected == nil && len(sw.Lines) == 0 {
		rows = append(rows, T(th, a.T("Nada muda nos atributos.")).C(rgb(0x666677)).W())
	}
	mm := op.Record(gtx.Ops)
	d := Inset(4, 6, 4, 6).Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return Col(1, rows...)(gtx)
	})
	call := mm.Stop()
	Fill(gtx, image.Rectangle{Max: d.Size}, rgb(0xfbfcff))
	call.Add(gtx.Ops)
	Fill(gtx, image.Rect(0, d.Size.Y-1, d.Size.X, d.Size.Y), ColListBorder)
	return d
}

// bestControls is the "best item" row: the button and the refine and
// grade every item is ranked at.
func (a *App) bestControls(gtx C) D {
	th, s := a.Th, &a.slot
	var refs []Option
	for r := 0; r <= 20; r++ {
		refs = append(refs, Option{strconv.Itoa(r), "+" + strconv.Itoa(r)})
	}
	grades := []Option{{"0", a.T("nenhum")}, {"1", "D"}, {"2", "C"}, {"3", "B"}, {"4", "A"}}
	return Row(4,
		func(gtx C) D { return Button{Label: a.T("Melhor item")}.Layout(gtx, th, &s.bestBtn) },
		T(th, a.T("Refino")).W(),
		func(gtx C) D {
			d, val, ok := s.bestRef.Layout(gtx, th, a.Desk, refs, strconv.Itoa(s.bestRefine), 56)
			if ok {
				s.bestRefine, _ = strconv.Atoi(val)
				if s.bestOn {
					a.runBest()
				}
			}
			return d
		},
		T(th, a.T("Grau")).W(),
		func(gtx C) D {
			d, val, ok := s.bestGra.Layout(gtx, th, a.Desk, grades, strconv.Itoa(s.bestGrade), 70)
			if ok {
				s.bestGrade, _ = strconv.Atoi(val)
				if s.bestOn {
					a.runBest()
				}
			}
			return d
		},
	)(gtx)
}

// runBest ranks the items for the slot in the background.
func (a *App) runBest() {
	s := &a.slot
	a.mu.Lock()
	s.bestSeq++
	seq, slot, b, lang := s.bestSeq, s.key, a.b, a.Lang
	refine, grade := s.bestRefine, s.bestGrade
	s.bestOn, s.bestRunning, s.best = true, true, nil
	a.mu.Unlock()
	go func() {
		res := a.V.BestItems(context.Background(), b, slot, refine, grade, lang)
		a.mu.Lock()
		if seq == s.bestSeq {
			s.best, s.bestRunning = &res, false
		}
		a.mu.Unlock()
		if a.Invalidate != nil {
			a.Invalidate()
		}
	}()
}

// bestStatus is the line under the ranking.
func (a *App) bestStatus() string {
	s := &a.slot
	a.mu.Lock()
	res := s.best
	a.mu.Unlock()
	if res == nil {
		return a.T("Testando todos os itens do slot…")
	}
	g := ""
	if s.bestGrade > 0 {
		g = " " + a.T("grau") + " " + string("DCBA"[s.bestGrade-1])
	}
	msg := fmt.Sprintf(a.T("Melhores de %d itens a +%d%s, pelo dano esperado."), res.Tested, s.bestRefine, g)
	if res.Partial {
		msg += " " + a.T("(parou no limite de tempo)")
	}
	return msg
}
