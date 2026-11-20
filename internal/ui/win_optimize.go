package ui

import (
	"context"
	"fmt"
	"image"
	"strconv"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/edit"
	"ragnarok-of-building/internal/view"
)

// ---- Otimizador (win-optimize) ----

type optimizeState struct {
	goal              Select
	ref, gra          Select
	items, cards, enc Checkbox
	// Unchecked boxes (the zero value tests everything).
	noItems, noCards, noEnch bool
	refine, grade            int
	goalVal                  string

	run    widget.Clickable
	busy   bool
	res    *view.OptResult
	apply  map[string]*widget.Clickable
	rows   map[string]*widget.Clickable
	expand map[string]*widget.Clickable
	open   map[string]bool
}

func (a *App) runOptimizer() {
	s := &a.opt
	a.mu.Lock()
	if s.busy {
		a.mu.Unlock()
		return
	}
	s.busy = true
	b, lang := a.b, a.Lang
	o := view.OptOptions{Goal: s.goalVal, Items: !s.noItems, Cards: !s.noCards, Enchants: !s.noEnch, Refine: s.refine, Grade: s.grade}
	a.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		res := a.V.Optimize(ctx, b, o, lang)
		a.mu.Lock()
		s.res, s.busy = &res, false
		a.mu.Unlock()
		if a.Invalidate != nil {
			a.Invalidate()
		}
	}()
}

// optKey names an option's widgets.
func optKey(o view.OptOption) string { return fmt.Sprintf("%s/%d/%d", o.Slot, o.Index, o.Item.ID) }

func (a *App) optimizeBody(gtx C) D {
	th, s := a.Th, &a.opt
	if s.run.Clicked(gtx) {
		a.runOptimizer()
	}
	a.mu.Lock()
	res, busy := s.res, s.busy
	a.mu.Unlock()
	if s.apply == nil {
		s.apply = map[string]*widget.Clickable{}
		s.rows = map[string]*widget.Clickable{}
		s.expand = map[string]*widget.Clickable{}
		s.open = map[string]bool{}
	}
	if res != nil {
		for _, g := range res.Groups {
			for _, o := range g.Options {
				if cl := s.apply[optKey(o)]; cl != nil && cl.Clicked(gtx) {
					a.applyOpt(o)
				}
			}
		}
	}
	goals := []Option{{view.GoalDamage, a.T("Dano")}, {view.GoalPhysDef, a.T("Defesa física")}, {view.GoalMagDef, a.T("Defesa mágica")}}
	var refs []Option
	for r := 0; r <= 20; r++ {
		refs = append(refs, Option{strconv.Itoa(r), "+" + strconv.Itoa(r)})
	}
	grades := []Option{{"0", "—"}, {"1", "D"}, {"2", "C"}, {"3", "B"}, {"4", "A"}}
	check := func(c *Checkbox, off *bool, label string) layout.Widget {
		return func(gtx C) D {
			d, toggled := c.Layout(gtx, th, !*off, label)
			if toggled {
				*off = !*off
			}
			return d
		}
	}
	if s.goalVal == "" {
		s.goalVal = view.GoalDamage
	}
	kids := []layout.Widget{
		T(th, a.T("Testa uma troca por vez (item, carta ou encantamento) contra o alvo da janela de dano e mostra a melhor de cada slot.")).Sz(SmallText).C(ColMuted).Wrapping().W(),
		Row(6, T(th, a.T("Objetivo")).W(), func(gtx C) D {
			d, v, ok := s.goal.Layout(gtx, th, a.Desk, goals, s.goalVal, 130)
			if ok {
				s.goalVal = v
			}
			return d
		}),
		Row(6,
			check(&s.items, &s.noItems, a.T("Itens")),
			T(th, a.T("Refino")).W(),
			func(gtx C) D {
				d, v, ok := s.ref.Layout(gtx, th, a.Desk, refs, strconv.Itoa(s.refine), 56)
				if ok {
					s.refine, _ = strconv.Atoi(v)
				}
				return d
			},
			T(th, a.T("Grau")).W(),
			func(gtx C) D {
				d, v, ok := s.gra.Layout(gtx, th, a.Desk, grades, strconv.Itoa(s.grade), 56)
				if ok {
					s.grade, _ = strconv.Atoi(v)
				}
				return d
			},
		),
		Row(10, check(&s.cards, &s.noCards, a.T("Cartas")), check(&s.enc, &s.noEnch, a.T("Encantamentos (os que o cliente lista para cada item)"))),
		func(gtx C) D {
			label := a.T("Calcular")
			if busy {
				label = a.T("Calculando…")
			}
			return Button{Label: label, Disabled: busy, MinWidth: 80}.Layout(gtx, th, &s.run)
		},
	}
	if res != nil {
		kids = append(kids, a.optResult(*res))
	}
	return Col(6, kids...)(gtx)
}

// applyOpt puts a suggestion in the build and optimizes again.
func (a *App) applyOpt(o view.OptOption) {
	b := a.Build()
	if o.Kind == "item" {
		a.SetBuild(a.V.SwapAt(b, o.Slot, o.Item.ID, o.Refine, o.Grade))
		a.Say(a.T("Item equipado."), false)
	} else {
		idx, id := o.Index, o.Item.ID
		a.SetBuild(edit.EditLinked(b, o.Slot, func(e *calc.Equip) { e.Cards[idx] = id }))
		a.Say(a.T("Carta aplicada."), false)
	}
	// Optimize again on the new build once it is computed.
	go func() {
		time.Sleep(300 * time.Millisecond)
		a.runOptimizer()
	}()
}

// optName is a suggestion's name with the refine and grade it was measured at.
func optName(o view.OptOption) string {
	n := itemTitle(&o.Item, o.Refine, o.Grade)
	if o.Item.MVP {
		n += " (MVP)"
	}
	return n
}

func (a *App) optResult(r view.OptResult) layout.Widget {
	th, s := a.Th, &a.opt
	note := func(t string) layout.Widget { return T(th, t).Sz(SmallText).C(ColMuted).Wrapping().W() }
	switch {
	case r.NoTarget:
		return note(a.T("Escolha um alvo na janela de dano (Alt+D) primeiro."))
	case r.NoItems:
		return note(a.T("Nada para testar: marque itens, ou equipe itens com slot de carta."))
	}
	measure := a.T("Dano esperado atual")
	switch r.Goal {
	case view.GoalPhysDef:
		measure = a.T("HP efetivo atual vs golpes físicos do alvo")
	case view.GoalMagDef:
		measure = a.T("HP efetivo atual vs magias do alvo")
	}
	head := []layout.Widget{Row(4, T(th, a.T("Alvo")+":").W(), B(th, r.Target).W())}
	if r.Goal == view.GoalDamage || r.Goal == "" {
		head[0] = Row(4, T(th, a.T("Alvo")+":").W(), B(th, r.Target).W(), T(th, "· "+r.Attack).W())
	}
	head = append(head, Row(4, T(th, measure+":").W(), B(th, r.Base).W()))
	kids := []layout.Widget{Col(1, head...)}
	if r.Partial {
		kids = append(kids, note(a.T("Tempo esgotado: alguns slots não foram testados.")))
	}
	var none []string
	for _, g := range r.Groups {
		g := g
		if len(g.Options) == 0 {
			none = append(none, g.Label)
			continue
		}
		gk := fmt.Sprintf("%s/%d", g.Slot, g.Index)
		if s.expand[gk] == nil {
			s.expand[gk] = &widget.Clickable{}
		}
		kids = append(kids, a.optGroupRows(g, gk))
	}
	if len(none) > 0 {
		kids = append(kids, note(a.T("Sem melhora")+": "+strings.Join(none, ", ")))
	}
	if len(r.NoEnch) > 0 {
		kids = append(kids, note(a.T("Sem lista de encantamentos no cliente (encantados por NPC), não testados")+": "+strings.Join(r.NoEnch, ", ")))
	}
	kids = append(kids, note(fmt.Sprintf(a.T("%d combinações testadas. Bônus condicionais entram pela média, como na janela de dano."), r.Tested)))
	return Col(4, kids...)
}

// optGroupRows is a slot's line: its best swap, gain and Aplicar; ▸ shows
// what is there now and the other options.
func (a *App) optGroupRows(g view.OptGroup, gk string) layout.Widget {
	th, s := a.Th, &a.opt
	ex := s.expand[gk]
	cur := 0
	curName := a.T("vazio")
	if g.Current != nil {
		cur, curName = g.Current.ID, g.Current.Name
	}
	const labelW = 112
	line := func(o view.OptOption, label layout.Widget, more bool) layout.Widget {
		k := optKey(o)
		if s.apply[k] == nil {
			s.apply[k] = &widget.Clickable{}
			s.rows[k] = &widget.Clickable{}
		}
		ap, hv := s.apply[k], s.rows[k]
		return func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(Fixed(labelW, label)),
				layout.Flexed(1, func(gtx C) D {
					d := hv.Layout(gtx, func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx C) D { return a.Icons.Icon(gtx, view.ItemIcon(o.Item.ID), 20) }),
							layout.Rigid(Gap(5, 0)),
							layout.Flexed(1, func(gtx C) D {
								ws := []layout.Widget{T(th, optName(o)).W()}
								if o.Skill != "" {
									ws = append(ws, T(th, a.T("em")+" "+o.Skill).Sz(SmallText).C(ColMuted).W())
								}
								return Col(0, ws...)(gtx)
							}),
							layout.Rigid(B(th, o.Gain).C(ColUp).W()),
						)
					})
					if hv.Hovered() {
						id := o.Item.ID
						if o.Kind == "item" {
							a.Desk.ShowTip(func(gtx C) D { return a.compareTip(gtx, cur, id, nil) })
						} else {
							a.Desk.ShowTip(func(gtx C) D { return a.itemTip(gtx, id, nil) })
						}
					}
					return d
				}),
				layout.Rigid(Gap(6, 0)),
				layout.Rigid(func(gtx C) D { return Button{Label: a.T("Aplicar")}.Layout(gtx, th, ap) }),
				layout.Rigid(func(gtx C) D {
					if !more {
						return Gap(22, 0)(gtx)
					}
					arrow := "+"
					if s.open[gk] {
						arrow = "−"
					}
					return Inset(0, 0, 0, 3).Layout(gtx, func(gtx C) D { return Button{Label: arrow, MinWidth: 18}.Layout(gtx, th, ex) })
				}),
			)
		}
	}
	return func(gtx C) D {
		if ex.Clicked(gtx) {
			s.open[gk] = !s.open[gk]
		}
		rows := []layout.Widget{line(g.Options[0], B(th, g.Label).Sz(SmallText).C(ColTitleText).Wrapping().W(), true)}
		if s.open[gk] {
			rows = append(rows, Pad(Inset(0, labelW, 0, 0), T(th, a.T("atual")+": "+curName).Sz(SmallText).C(ColMuted).W()))
			for _, o := range g.Options[1:] {
				rows = append(rows, line(o, func(gtx C) D { return D{} }, false))
			}
		}
		m := op.Record(gtx.Ops)
		d := Inset(3, 0, 3, 0).Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return Col(2, rows...)(gtx)
		})
		call := m.Stop()
		call.Add(gtx.Ops)
		Fill(gtx, image.Rect(0, d.Size.Y-1, d.Size.X, d.Size.Y), rgb(0xe3e6ec))
		return d
	}
}

// ---- Banco de Itens (win-items) ----

type itemsState struct {
	q       Input
	typ     Select
	typVal  string
	lastKey string
	results []view.ItemView
	rows    map[int]*widget.Clickable
	list    widget.List
	shown   int // item whose description is shown below
}

func (a *App) itemsBody(gtx C) D {
	th, s := a.Th, &a.items
	var types []Option
	for _, t := range view.ItemTypes() {
		types = append(types, Option{t[0], a.T(t[1])})
	}
	key := s.q.Text() + "\x00" + s.typVal + "\x00" + a.Lang
	if key != s.lastKey {
		s.lastKey = key
		s.results = a.V.SearchItems(strings.TrimSpace(s.q.Text()), s.typVal, a.Lang)
	}
	if s.rows == nil {
		s.rows = map[int]*widget.Clickable{}
	}
	for _, it := range s.results {
		if cl := s.rows[it.ID]; cl != nil && cl.Clicked(gtx) {
			s.shown = it.ID
		}
	}
	kids := []layout.Widget{
		func(gtx C) D {
			s.q.Hint = a.T("Nome ou ID…")
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D { d, _, _ := s.q.Layout(gtx, th, 0); return d }),
				layout.Rigid(Gap(4, 0)),
				layout.Rigid(func(gtx C) D {
					d, val, ok := s.typ.Layout(gtx, th, a.Desk, types, s.typVal, 130)
					if ok {
						s.typVal = val
					}
					return d
				}),
			)
		},
		func(gtx C) D {
			h := gtx.Dp(300)
			gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
			return Box{Bg: ColWhite, Line: ColListBorder, Pad: Inset(2, 2, 2, 2)}.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min = gtx.Constraints.Max
				if len(s.results) == 0 {
					msg := a.T("Digite o nome de um item (PT, ES ou EN) ou o ID, ou escolha um tipo.")
					if s.q.Text() != "" || s.typVal != "" {
						msg = a.T("Nenhum item encontrado.")
					}
					return Inset(8, 8, 8, 8).Layout(gtx, T(th, msg).C(rgb(0x777777)).Wrapping().W())
				}
				s.list.Axis = layout.Vertical
				return s.list.Layout(gtx, len(s.results), func(gtx C, i int) D {
					it := s.results[i]
					cl := s.rows[it.ID]
					if cl == nil {
						cl = &widget.Clickable{}
						s.rows[it.ID] = cl
					}
					d := cl.Layout(gtx, func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						m := op.Record(gtx.Ops)
						d := Inset(2, 6, 2, 6).Layout(gtx, func(gtx C) D {
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
								layout.Rigid(T(th, fmt.Sprintf("#%d", it.ID)).Sz(SmallText).C(rgb(0x888888)).W()),
							)
						})
						call := m.Stop()
						switch {
						case it.ID == s.shown:
							Fill(gtx, image.Rectangle{Max: d.Size}, ColSelect)
						case cl.Hovered():
							Fill(gtx, image.Rectangle{Max: d.Size}, ColHover)
						}
						call.Add(gtx.Ops)
						pointerCursor(gtx, d.Size)
						return d
					})
					if cl.Hovered() && it.ID != s.shown {
						id := it.ID
						a.Desk.ShowTip(func(gtx C) D { return a.itemTip(gtx, id, nil) })
					}
					return d
				})
			})
		},
	}
	if it := a.V.Item(s.shown, a.Lang); it != nil {
		kids = append(kids, func(gtx C) D {
			return RowTop(8,
				func(gtx C) D {
					return Box{Bg: ColWhite, Line: ColListBorder, Pad: Inset(4, 4, 4, 4)}.Layout(gtx, func(gtx C) D {
						return a.Icons.Icon(gtx, view.ItemIcon(it.ID), 48)
					})
				},
				func(gtx C) D {
					gtx.Constraints.Max.X -= gtx.Dp(70)
					return Box{Bg: ColWhite, Line: ColListBorder, Pad: Inset(5, 6, 5, 6)}.Layout(gtx, func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						name := it.Name
						if it.Slots > 0 {
							name += fmt.Sprintf(" [%d]", it.Slots)
						}
						return Col(4, B(th, name).W(), RichText(th, it.Desc, rgb(0x000000)), T(th, fmt.Sprintf("ID %d", it.ID)).Sz(SmallText).C(ColMuted).W())(gtx)
					})
				},
			)(gtx)
		})
	}
	return Col(6, kids...)(gtx)
}
